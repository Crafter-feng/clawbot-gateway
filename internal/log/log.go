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

// DefaultBufferCapacity 默认缓冲区容量
const DefaultBufferCapacity = 500

type Buffer struct {
	mu       sync.RWMutex
	entries  []Entry
	capacity int
}

func NewBuffer(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = DefaultBufferCapacity
	}
	return &Buffer{
		entries:  make([]Entry, 0, capacity),
		capacity: capacity,
	}
}

func (b *Buffer) Append(e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= b.capacity {
		// 环形：丢弃最旧的一条
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, e)
}

// Entries 返回过滤后的日志条目（按时间正序）
// level: "DEBUG"/"INFO"/"WARN"/"ERROR"，空字符串表示不过滤
// component: 匹配 attrs["cmp"]，空字符串表示不过滤
// backend: 匹配 attrs["backend"]，空字符串表示不过滤
func (b *Buffer) Entries(limit int, level string, component string, backend string) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.entries) == 0 {
		return nil
	}

	// 从后往前收集，最后再反转为正序
	filtered := make([]Entry, 0, len(b.entries))
	for i := len(b.entries) - 1; i >= 0; i-- {
		e := b.entries[i]
		if level != "" && e.Level != level {
			continue
		}
		attrs, _ := e.Attrs.(map[string]interface{})
		if component != "" {
			if cmp, _ := attrs["cmp"].(string); cmp != component {
				continue
			}
		}
		if backend != "" {
			if bid, _ := attrs["backend"].(string); bid != backend {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	// 反转为时间正序
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// 截取最近的 limit 条
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

// GetComponents 返回日志中出现的所有组件名和后端名
func (b *Buffer) GetComponents() (components []string, backends []string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cmpSet := make(map[string]bool)
	bkSet := make(map[string]bool)
	for _, e := range b.entries {
		attrs, ok := e.Attrs.(map[string]interface{})
		if !ok {
			continue
		}
		if c, ok := attrs["cmp"].(string); ok && c != "" {
			cmpSet[c] = true
		}
		if bk, ok := attrs["backend"].(string); ok && bk != "" {
			bkSet[bk] = true
		}
	}
	components = make([]string, 0, len(cmpSet))
	for c := range cmpSet {
		components = append(components, c)
	}
	backends = make([]string, 0, len(bkSet))
	for bk := range bkSet {
		backends = append(backends, bk)
	}
	return
}

// ── Logger ──

type Logger struct {
	*slog.Logger
	buffer *Buffer
	// preAttrs 存储通过 WithAttrs 预绑定的属性
	// slog 的 Record.Attrs() 只包含本次调用的属性，不含预绑定的
	// bufferHandler 需要自己维护预绑定属性，才能在缓冲区中完整记录
	preAttrs []slog.Attr
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
			// 去掉 slog 自动注入的 time 字段（我们自己格式化）
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	}
	buf := NewBuffer(DefaultBufferCapacity)
	h := &bufferHandler{
		handler:  slog.NewJSONHandler(w, opts),
		buffer:   buf,
		preAttrs: nil,
	}
	return &Logger{slog.New(h), buf, nil}
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

// WithComponent 创建带组件标签的子 logger
// 预绑定 cmp 属性到 bufferHandler，确保缓冲区中能记录组件名
func (l *Logger) WithComponent(name string) *Logger {
	return l.WithField("cmp", name)
}

// WithField 创建带预绑定属性的子 logger
// 预绑定属性会被存入 bufferHandler.preAttrs，在 Handle 时合并到 Entry.Attrs
func (l *Logger) WithField(key string, value any) *Logger {
	attr := slog.Any(key, value)
	newHandler := &bufferHandler{
		handler:  l.Logger.Handler().WithAttrs([]slog.Attr{attr}),
		buffer:   l.buffer,
		preAttrs: append(cloneAttrs(l.preAttrs), attr),
	}
	return &Logger{
		Logger:    slog.New(newHandler),
		buffer:    l.buffer,
		preAttrs:  newHandler.preAttrs,
	}
}

// cloneAttrs 复制属性切片，避免共享底层数组
func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}
	cp := make([]slog.Attr, len(attrs))
	copy(cp, attrs)
	return cp
}

// ── bufferHandler ──

type bufferHandler struct {
	handler  slog.Handler
	buffer   *Buffer
	preAttrs []slog.Attr // 预绑定属性（通过 WithAttrs / WithField 添加）
}

func (h *bufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *bufferHandler) Handle(ctx context.Context, r slog.Record) error {
	// 先写入标准输出
	if err := h.handler.Handle(ctx, r); err != nil {
		return err
	}

	// 构建日志条目
	entry := Entry{
		Time:    r.Time.Format(time.RFC3339),
		Level:   r.Level.String(),
		Message: r.Message,
	}
	if r.PC != 0 {
		entry.Source = fmt.Sprintf("0x%x", r.PC)
	}

	// 合并预绑定属性和本次调用的属性
	attrs := make(map[string]interface{})
	// 预绑定属性（cmp, backend 等）
	for _, a := range h.preAttrs {
		attrs[a.Key] = a.Value.Any()
	}
	// 本次调用的属性（覆盖同名预绑定属性）
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
	return &bufferHandler{
		handler:  h.handler.WithAttrs(attrs),
		buffer:   h.buffer,
		preAttrs: append(cloneAttrs(h.preAttrs), attrs...),
	}
}

func (h *bufferHandler) WithGroup(name string) slog.Handler {
	return &bufferHandler{
		handler:  h.handler.WithGroup(name),
		buffer:   h.buffer,
		preAttrs: h.preAttrs, // group 不影响预绑定属性
	}
}

// ── Gin 中间件 ──

func GinMiddleware(logger *Logger) gin.HandlerFunc {
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
			logger.LogAttrs(nil, slog.LevelError, "api request", attrs...)
		case status >= 400:
			logger.LogAttrs(nil, slog.LevelWarn, "api request", attrs...)
		default:
			logger.LogAttrs(nil, slog.LevelInfo, "api request", attrs...)
		}
	}
}