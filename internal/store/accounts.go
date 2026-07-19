package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ── 账号凭证存储（每个文件一个账号）──
// 分离原因：credentials 是 Web UI 扫码绑定的运行时状态，
// 与 config.yaml 的静态配置分离，且每个账号独立文件便于管理。

type AccountStore struct {
	mu        sync.RWMutex
	dirPath   string
	accounts  map[string]*StoredCredential
}

type StoredCredential struct {
	AccountID   string `json:"account_id"`
	Token       string `json:"token"`
	BaseURL     string `json:"base_url"`
	UserID      string `json:"user_id"`
	AccountName string `json:"account_name"`
	LoginAt     int64  `json:"login_at"`
}

func NewAccountStore(dirPath string) (*AccountStore, error) {
	a := &AccountStore{
		dirPath:  dirPath,
		accounts: make(map[string]*StoredCredential),
	}
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return nil, err
	}
	if err := a.loadAll(); err != nil {
		return nil, err
	}
	return a, nil
}

// accountFilePath 返回某个账号的文件路径
func (a *AccountStore) accountFilePath(accountID string) string {
	// 使用安全文件名，避免路径穿越
	safeName := filepath.Base(accountID)
	if safeName == "." || safeName == "/" {
		safeName = "unknown"
	}
	return filepath.Join(a.dirPath, safeName+".json")
}

func (a *AccountStore) loadAll() error {
	entries, err := os.ReadDir(a.dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(a.dirPath, 0700)
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.dirPath, entry.Name()))
		if err != nil {
			continue
		}
		var cred StoredCredential
		if err := json.Unmarshal(data, &cred); err != nil {
			continue
		}
		if cred.AccountID != "" {
			a.accounts[cred.AccountID] = &cred
		}
	}
	return nil
}

// List 返回所有已持久化的账号凭证
func (a *AccountStore) List() []StoredCredential {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]StoredCredential, 0, len(a.accounts))
	for _, cred := range a.accounts {
		result = append(result, *cred)
	}
	return result
}

// Get 返回指定账号的凭证，不存在返回 nil
func (a *AccountStore) Get(accountID string) *StoredCredential {
	a.mu.RLock()
	defer a.mu.RUnlock()
	cred, ok := a.accounts[accountID]
	if !ok {
		return nil
	}
	copied := *cred
	return &copied
}

// Save 持久化（创建或更新）一个账号凭证
func (a *AccountStore) Save(cred StoredCredential) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.accounts[cred.AccountID] = &cred
	return a.writeFile(cred)
}

func (a *AccountStore) writeFile(cred StoredCredential) error {
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.accountFilePath(cred.AccountID), data, 0600)
}

// Remove 删除一个账号凭证文件
func (a *AccountStore) Remove(accountID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.accounts, accountID)
	path := a.accountFilePath(accountID)
	if err := os.Remove(path); os.IsNotExist(err) {
		return nil
	} else {
		return err
	}
}

// Count 返回账号数量
func (a *AccountStore) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.accounts)
}
