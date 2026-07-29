package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
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
	Count   int    `json:"count"`
	Source  string `json:"source,omitempty"`
	Attrs   any    `json:"attrs,omitempty"`
}

// ── 日志缓冲区 ──

// DefaultBufferCapacity 每个组件默认缓冲容量
const DefaultBufferCapacity = 200

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
	// 与最后一条合并（相同消息+级别+属性）
	if n := len(b.entries); n > 0 {
		last := &b.entries[n-1]
		if last.Message == e.Message && last.Level == e.Level && attrsEqual(last.Attrs, e.Attrs) {
			last.Count++
			last.Time = e.Time
			return
		}
	}
	if len(b.entries) >= b.capacity {
		b.entries = b.entries[1:]
	}
	if e.Count == 0 {
		e.Count = 1
	}
	b.entries = append(b.entries, e)
}

// Snap 返回当前所有条目的快照（按时间正序）
func (b *Buffer) Snap() []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.entries) == 0 {
		return nil
	}
	result := make([]Entry, len(b.entries))
	copy(result, b.entries)
	return result
}

// ── 多组件缓冲区 ──
// 每个组件独立缓存，互不挤占

type BufferGroup struct {
	mu       sync.RWMutex
	buffers  map[string]*Buffer
	capacity int
}

func NewBufferGroup(capacity int) *BufferGroup {
	if capacity <= 0 {
		capacity = DefaultBufferCapacity
	}
	return &BufferGroup{
		buffers:  make(map[string]*Buffer),
		capacity: capacity,
	}
}

func (bg *BufferGroup) getOrCreate(cmp string) *Buffer {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if b, ok := bg.buffers[cmp]; ok {
		return b
	}
	b := NewBuffer(bg.capacity)
	bg.buffers[cmp] = b
	return b
}

func (bg *BufferGroup) Append(cmp string, e Entry) {
	bg.getOrCreate(cmp).Append(e)
}

// Entries 返回过滤后的日志条目（按时间正序）
func (bg *BufferGroup) Entries(limit int, level string, component string, backend string) []Entry {
	if component != "" {
		bg.mu.RLock()
		b, ok := bg.buffers[component]
		bg.mu.RUnlock()
		if !ok {
			return nil
		}
		all := b.Snap()
		return filterEntries(all, limit, level, component, backend)
	}

	// 全部：合并所有组件的最新条目
	bg.mu.RLock()
	all := make([]Entry, 0)
	for _, b := range bg.buffers {
		all = append(all, b.Snap()...)
	}
	bg.mu.RUnlock()

	// 按时间倒序
	sort.Slice(all, func(i, j int) bool {
		return all[i].Time > all[j].Time
	})
	// 截取最近的 limit 条
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return filterEntries(all, 0, level, component, backend)
}

// GetComponents 返回日志中出现的所有组件名和后端名
func (bg *BufferGroup) GetComponents() (components []string, backends []string) {
	cmpSet := make(map[string]bool)
	bkSet := make(map[string]bool)

	bg.mu.RLock()
	for cmp, b := range bg.buffers {
		cmpSet[cmp] = true
		for _, e := range b.Snap() {
			attrs, ok := e.Attrs.(map[string]interface{})
			if !ok {
				continue
			}
			if bk, ok := attrs["backend"].(string); ok && bk != "" {
				bkSet[bk] = true
			}
		}
	}
	bg.mu.RUnlock()

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

// filterEntries 过滤并截取日志条目
func filterEntries(entries []Entry, limit int, level string, component string, backend string) []Entry {
	if len(entries) == 0 {
		return nil
	}
	// 从后往前收集，最后反转为正序
	filtered := make([]Entry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
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


// ── Logger ──
type Logger struct {
	*slog.Logger
	buffers *BufferGroup
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
	bg := NewBufferGroup(DefaultBufferCapacity)
	h := &bufferHandler{
		handler:  slog.NewJSONHandler(w, opts),
		buffers:  bg,
		preAttrs: nil,
	}
	return &Logger{slog.New(h), bg, nil}
}

func SetDefault(l *Logger) {
	defaultLogger = l
	slog.SetDefault(l.Logger)
}

func Default() *Logger {
	return defaultLogger
}
func GetBuffer() *BufferGroup {
	if defaultLogger == nil {
		return nil
	}
	return defaultLogger.buffers
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
		buffers:  l.buffers,
		preAttrs: append(cloneAttrs(l.preAttrs), attr),
	}
	return &Logger{
		Logger:    slog.New(newHandler),
		buffers:   l.buffers,
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
type bufferHandler struct {
	handler  slog.Handler
	buffers  *BufferGroup
	preAttrs []slog.Attr
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
	for _, a := range h.preAttrs {
		attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	if len(attrs) > 0 {
		entry.Attrs = attrs
	}

	// 按组件路由到独立缓存
	cmp, _ := attrs["cmp"].(string)
	h.buffers.Append(cmp, entry)
	return nil
}
func (h *bufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &bufferHandler{
		handler:  h.handler.WithAttrs(attrs),
		buffers:  h.buffers,
		preAttrs: append(cloneAttrs(h.preAttrs), attrs...),
	}
}
func (h *bufferHandler) WithGroup(name string) slog.Handler {
	return &bufferHandler{
		handler:  h.handler.WithGroup(name),
		buffers:  h.buffers,
		preAttrs: h.preAttrs,
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
		msg := fmt.Sprintf("%s %s", c.Request.Method, path)
		if query != "" {
			msg += "?" + c.Request.URL.Query().Encode()
		}
		switch {
		case status >= 500:
			logger.LogAttrs(nil, slog.LevelError, msg, attrs...)
		case status >= 400:
			logger.LogAttrs(nil, slog.LevelWarn, msg, attrs...)
		default:
			logger.LogAttrs(nil, slog.LevelInfo, msg, attrs...)
		}
	}
}
// attrsEqual 比较两个 Attrs 是否相等（用于合并重复日志）
func attrsEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	am, aok := a.(map[string]interface{})
	bm, bok := b.(map[string]interface{})
	if !aok || !bok {
		return false
	}
	if len(am) != len(bm) {
		return false
	}
	for k, v := range am {
		if bv, ok := bm[k]; !ok || v != bv {
			return false
		}
	}
	return true
}