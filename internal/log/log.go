package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 日志级别 ──

type Level slog.Level

const (
	LevelDebug = Level(slog.LevelDebug)
	LevelInfo  = Level(slog.LevelInfo)
	LevelWarn  = Level(slog.LevelWarn)
	LevelError = Level(slog.LevelError)
)

// ── 日志条目 ──

type Entry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	Attrs   any    `json:"attrs,omitempty"`
}

// ── 日志缓冲区 ──

type Buffer struct {
	mu       sync.RWMutex
	entries  []Entry
	capacity int
}

func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		entries:  make([]Entry, 0, capacity),
		capacity: capacity,
	}
}

func (b *Buffer) Append(e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= b.capacity {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, e)
}
func (b *Buffer) Entries(limit int, level string, component string, backend string) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.entries) == 0 {
		return nil
	}

	filtered := make([]Entry, 0, len(b.entries))
	for i := len(b.entries) - 1; i >= 0; i-- {
		e := b.entries[i]
		if level != "" && e.Level != level {
			continue
		}
		if component != "" {
			attrs, ok := e.Attrs.(map[string]interface{})
			if !ok {
				continue
			}
			cmp, _ := attrs["cmp"].(string)
			if cmp != component {
				continue
			}
		}
		if backend != "" {
			attrs, ok := e.Attrs.(map[string]interface{})
			if !ok {
				continue
			}
			bid, _ := attrs["backend"].(string)
			if bid != backend {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	// reverse to chronological order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

func (b *Buffer) GetComponents() (components []string, backends []string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cmpSet := make(map[string]bool)
	bkSet := make(map[string]bool)
	for _, e := range b.entries {
		if attrs, ok := e.Attrs.(map[string]interface{}); ok {
			if c, ok := attrs["cmp"].(string); ok && c != "" {
				cmpSet[c] = true
			}
			if b, ok := attrs["backend"].(string); ok && b != "" {
				bkSet[b] = true
			}
		}
	}
	components = make([]string, 0, len(cmpSet))
	for c := range cmpSet {
		components = append(components, c)
	}
	backends = make([]string, 0, len(bkSet))
	for b := range bkSet {
		backends = append(backends, b)
	}
	return
}

// ── Logger ──

type Logger struct {
	*slog.Logger
	buffer *Buffer
}

var defaultLogger *Logger

func init() {
	SetDefault(New("info"))
}

func New(level string) *Logger {
	return NewWriter(os.Stderr, level)
}

func NewWriter(w io.Writer, level string) *Logger {
	l := &slog.LevelVar{}
	switch level {
	case "debug":
		l.Set(slog.LevelDebug)
	case "info":
		l.Set(slog.LevelInfo)
	case "warn":
		l.Set(slog.LevelWarn)
	case "error":
		l.Set(slog.LevelError)
	default:
		l.Set(slog.LevelInfo)
	}
	opts := &slog.HandlerOptions{
		Level: l,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	}
	buf := NewBuffer(500)
	h := &bufferHandler{
		handler: slog.NewJSONHandler(w, opts),
		buffer:  buf,
	}
	return &Logger{slog.New(h), buf}
}

func SetDefault(l *Logger) {
	defaultLogger = l
	slog.SetDefault(l.Logger)
}

func Default() *Logger {
	return defaultLogger
}

func GetBuffer() *Buffer {
	if defaultLogger == nil {
		return nil
	}
	return defaultLogger.buffer
}

func (l *Logger) WithComponent(name string) *Logger {
	return &Logger{l.With("cmp", name), l.buffer}
}

func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{l.With(key, value), l.buffer}
}

// ── bufferHandler ──

type bufferHandler struct {
	handler slog.Handler
	buffer  *Buffer
}

func (h *bufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *bufferHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.handler.Handle(ctx, r); err != nil {
		return err
	}
	entry := Entry{
		Time:    r.Time.Format(time.RFC3339),
		Level:   r.Level.String(),
		Message: r.Message,
	}
	if r.PC != 0 {
		entry.Source = fmt.Sprintf("0x%x", r.PC)
	}
	attrs := make(map[string]interface{})
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	if len(attrs) > 0 {
		entry.Attrs = attrs
	}
	h.buffer.Append(entry)
	return nil
}

func (h *bufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &bufferHandler{h.handler.WithAttrs(attrs), h.buffer}
}

func (h *bufferHandler) WithGroup(name string) slog.Handler {
	return &bufferHandler{h.handler.WithGroup(name), h.buffer}
}

// ── Gin 中间件 ──

func GinMiddleware(log *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Duration("latency", latency),
			slog.String("ip", c.ClientIP()),
		}
		if query != "" {
			safeQuery := c.Request.URL.Query()
			sensitiveParams := []string{"token", "password", "secret", "key"}
			for _, p := range sensitiveParams {
				if safeQuery.Has(p) {
					safeQuery.Set(p, "***")
				}
			}
			attrs = append(attrs, slog.String("query", safeQuery.Encode()))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("gin_errors", c.Errors.String()))
		}

		switch {
		case status >= 500:
			log.LogAttrs(nil, slog.LevelError, "api request", attrs...)
		case status >= 400:
			log.LogAttrs(nil, slog.LevelWarn, "api request", attrs...)
		default:
			log.LogAttrs(nil, slog.LevelInfo, "api request", attrs...)
		}
	}
}