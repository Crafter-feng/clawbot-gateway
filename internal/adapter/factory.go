package adapter

import (
	"fmt"
	"sync"

	"clawbot-gateway/internal/database"
)

// ── 适配器工厂（注册中心） ──

// AdapterFactory 管理所有已创建的适配器实例
type AdapterFactory struct {
	mu       sync.RWMutex
	adapters map[string]BackendAdapter    // 后端适配器
	conns    map[string]ConnectionAdapter // 连接适配器
}

func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		adapters: make(map[string]BackendAdapter),
		conns:    make(map[string]ConnectionAdapter),
	}
}

// ── 后端适配器管理 ──

func (f *AdapterFactory) Register(adapter BackendAdapter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adapters[adapter.ID()] = adapter
}

func (f *AdapterFactory) Get(id string) (BackendAdapter, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	a, ok := f.adapters[id]
	return a, ok
}

func (f *AdapterFactory) Remove(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.adapters, id)
}

func (f *AdapterFactory) ListIDs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	ids := make([]string, 0, len(f.adapters))
	for id := range f.adapters {
		ids = append(ids, id)
	}
	return ids
}

func (f *AdapterFactory) List() []BackendAdapter {
	f.mu.RLock()
	defer f.mu.RUnlock()
	list := make([]BackendAdapter, 0, len(f.adapters))
	for _, a := range f.adapters {
		list = append(list, a)
	}
	return list
}

func (f *AdapterFactory) HealthyList() []BackendAdapter {
	f.mu.RLock()
	defer f.mu.RUnlock()
	list := make([]BackendAdapter, 0, len(f.adapters))
	for _, a := range f.adapters {
		if a.HealthCheck(nil) {
			list = append(list, a)
		}
	}
	return list
}

// ── 连接适配器管理 ──

func (f *AdapterFactory) RegisterConnection(adapter ConnectionAdapter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conns[adapter.ID()] = adapter
}

func (f *AdapterFactory) GetConnection(id string) (ConnectionAdapter, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.conns[id]
	return c, ok
}

func (f *AdapterFactory) RemoveConnection(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.conns, id)
}

func (f *AdapterFactory) ListConnections() []ConnectionAdapter {
	f.mu.RLock()
	defer f.mu.RUnlock()
	list := make([]ConnectionAdapter, 0, len(f.conns))
	for _, c := range f.conns {
		list = append(list, c)
	}
	return list
}

// ── 兼容入口：从数据库记录创建适配器 ──

// CreateAdapterFromDB 从数据库后端记录创建适配器实例（委托给注册表）
func CreateAdapterFromDB(b database.Backend) BackendAdapter {
	return CreateAdapter(b)
}

func IsConnectionAdapter(adapterType string) bool {
	return adapterType == "ilink_proxy"
}

func (f *AdapterFactory) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adapters = make(map[string]BackendAdapter)
	f.conns = make(map[string]ConnectionAdapter)
}

func (f *AdapterFactory) RegisterFromDB(b database.Backend) error {
	adapt := CreateAdapterFromDB(b)
	if adapt == nil {
		return fmt.Errorf("cannot create adapter for backend %s", b.ID)
	}
	f.Register(adapt)
	if IsConnectionAdapter(adapt.Type()) {
		connAdapter := NewILinkProxyAdapter(b.ID, b.Name, "gw_"+b.ID, "gw_"+b.ID+"@im.wechat", "https://ilinkai.weixin.qq.com")
		f.RegisterConnection(connAdapter)
	}
	return nil
}
