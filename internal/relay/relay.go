package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"clawbot-gateway/internal/log"
)

// FileRelay 文件中转服务接口
type FileRelay interface {
	// Forward 转发文件到目标服务
	Forward(ctx context.Context, file *FileMessage) error
	// Name 返回适配器名称
	Name() string
}

// FileMessage 文件消息
type FileMessage struct {
	FromUser  string            `json:"from_user"`
	FileName  string            `json:"file_name"`
	FileType  string            `json:"file_type"` // "image", "file", "audio", "video"
	FileData  []byte            `json:"file_data,omitempty"`
	FileURL   string            `json:"file_url,omitempty"`
	MimeType  string            `json:"mime_type,omitempty"`
	Size      int64             `json:"size"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// RelayManager 文件中转管理器
type RelayManager struct {
	mu       sync.RWMutex
	relays   []FileRelay
	log      *log.Logger
}

func NewRelayManager() *RelayManager {
	return &RelayManager{
		relays: make([]FileRelay, 0),
		log:    log.Default().WithComponent("relay"),
	}
}

func (m *RelayManager) SetLogger(l *log.Logger) {
	m.log = l.WithComponent("relay")
}

// Register 注册文件中转适配器
func (m *RelayManager) Register(relay FileRelay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.relays = append(m.relays, relay)
	m.log.Info("relay registered", "name", relay.Name())
}

// Forward 转发文件到所有已注册的中转服务
func (m *RelayManager) Forward(ctx context.Context, file *FileMessage) error {
	m.mu.RLock()
	relays := make([]FileRelay, len(m.relays))
	copy(relays, m.relays)
	m.mu.RUnlock()

	if len(relays) == 0 {
		return fmt.Errorf("no relay adapters registered")
	}

	var lastErr error
	for _, relay := range relays {
		if err := relay.Forward(ctx, file); err != nil {
			m.log.Warn("relay forward failed", "name", relay.Name(), "error", err)
			lastErr = err
		} else {
			m.log.Info("relay forward success", "name", relay.Name(), "file", file.FileName)
		}
	}

	return lastErr
}

// List 列出所有已注册的中转适配器
func (m *RelayManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, len(m.relays))
	for i, relay := range m.relays {
		names[i] = relay.Name()
	}
	return names
}
