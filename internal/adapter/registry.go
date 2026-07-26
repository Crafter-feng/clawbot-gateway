package adapter

import (
	"sync"

	"clawbot-gateway/internal/database"
)

// ── 适配器注册表 ──
// 适配器通过 init() 自注册，新增类型无需修改工厂代码

var (
	registryMu sync.RWMutex
	registry   = make(map[string]AdapterCreator)
)

// RegisterAdapter 注册适配器创建函数
// adapterType: 适配器类型标识（如 "echo", "openai_compatible", "ilink_proxy"）
// creator: 工厂函数，从数据库记录创建适配器实例
func RegisterAdapter(adapterType string, creator AdapterCreator) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[adapterType] = creator
}

// CreateAdapter 从数据库记录创建适配器实例（通过注册表查找）
func CreateAdapter(b database.Backend) BackendAdapter {
	registryMu.RLock()
	creator, ok := registry[b.Type]
	registryMu.RUnlock()
	if !ok {
		return nil
	}
	return creator(b)
}

// RegisteredTypes 返回所有已注册的适配器类型
func RegisteredTypes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}
