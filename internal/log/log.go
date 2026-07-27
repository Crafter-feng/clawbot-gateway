package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Level wraps slog.Level for convenience.
type Level slog.Level

const (
	LevelDebug = Level(slog.LevelDebug)
	LevelInfo  = Level(slog.LevelInfo)
	LevelWarn  = Level(slog.LevelWarn)
	LevelError = Level(slog.LevelError)
)

// Logger wraps slog.Logger with structured logging helpers.
type Logger struct {
	*slog.Logger
}

var defaultLogger *Logger

func init() {
	defaultLogger = New("info")
}

// New creates a new Logger with the given level string ("debug", "info", "warn", "error").
// Output goes to stderr. Includes source file:line.
func New(level string) *Logger {
	lvl := &slog.LevelVar{}
	switch strings.ToLower(level) {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "info":
		lvl.Set(slog.LevelInfo)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as ISO8601 with milliseconds
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: a.Key, Value: slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))}
			}
			return a
		},
	})
	return &Logger{slog.New(h)}
}

// NewWriter creates a Logger that writes to an arbitrary writer (useful for tests).
func NewWriter(w io.Writer, level string) *Logger {
	lvl := &slog.LevelVar{}
	switch strings.ToLower(level) {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "info":
		lvl.Set(slog.LevelInfo)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: a.Key, Value: slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))}
			}
			return a
		},
	})
	return &Logger{slog.New(h)}
}

// SetDefault sets the package-level default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
	slog.SetDefault(l.Logger)
}

// Default returns the package-level default logger.
func Default() *Logger {
	return defaultLogger
}

// WithComponent returns a child logger tagged with the named component.
func (l *Logger) WithComponent(name string) *Logger {
	return &Logger{l.With("cmp", name)}
}

// WithField adds a key-value attribute.
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{l.With(key, value)}
}

// GinMiddleware returns a Gin middleware that logs every request with
// status, method, path, latency, and client IP.
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
			// Mask sensitive params in query string
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
