package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer os.Remove(path)

	if s == nil {
		t.Fatal("store is nil")
	}

	rules := s.GetKeywordRules()
	if len(rules) != 0 {
		t.Errorf("empty rules want 0, got %d", len(rules))
	}
}

func TestSaveAndLoadCredential(t *testing.T) {
	accDir := t.TempDir()
	as, err := NewAccountStore(accDir)
	if err != nil {
		t.Fatalf("NewAccountStore: %v", err)
	}

	cred := StoredCredential{
		AccountID:   "acc_001",
		Token:       "tok_abc123",
		BaseURL:     "https://ilinkai.weixin.qq.com",
		UserID:      "user_001",
		AccountName: "Work WeChat",
		LoginAt:     1699999999,
	}

	if err := as.Save(cred); err != nil {
		t.Fatalf("AccountStore.Save: %v", err)
	}

	creds := as.List()
	if len(creds) != 1 {
		t.Fatalf("want 1 credential, got %d", len(creds))
	}
	if creds[0].AccountID != "acc_001" {
		t.Errorf("AccountID want acc_001, got %s", creds[0].AccountID)
	}

	// 重新加载，验证持久化
	as2, err := NewAccountStore(accDir)
	if err != nil {
		t.Fatalf("reload AccountStore: %v", err)
	}
	creds2 := as2.List()
	if len(creds2) != 1 {
		t.Fatalf("reload: want 1 credential, got %d", len(creds2))
	}
	if creds2[0].Token != "tok_abc123" {
		t.Errorf("reload Token want tok_abc123, got %s", creds2[0].Token)
	}
}

func TestSaveCredentialDedup(t *testing.T) {
	accDir := t.TempDir()
	as, _ := NewAccountStore(accDir)

	as.Save(StoredCredential{AccountID: "a1", Token: "tok1"})
	as.Save(StoredCredential{AccountID: "a1", Token: "tok2"}) // 同 ID 应覆盖

	creds := as.List()
	if len(creds) != 1 {
		t.Errorf("dedup want 1 credential, got %d", len(creds))
	}
	if len(creds) > 0 && creds[0].Token != "tok2" {
		t.Errorf("Token should be updated, got %s", creds[0].Token)
	}
}

func TestRemoveCredential(t *testing.T) {
	accDir := t.TempDir()
	as, _ := NewAccountStore(accDir)

	as.Save(StoredCredential{AccountID: "a1", Token: "tok1"})
	as.Save(StoredCredential{AccountID: "a2", Token: "tok2"})

	if err := as.Remove("a1"); err != nil {
		t.Fatalf("AccountStore.Remove: %v", err)
	}

	creds := as.List()
	if len(creds) != 1 {
		t.Fatalf("after remove want 1 credential, got %d", len(creds))
	}
	if creds[0].AccountID != "a2" {
		t.Errorf("remaining want a2, got %s", creds[0].AccountID)
	}

	// 删除不存在的应该无错误
	if err := as.Remove("nonexist"); err != nil {
		t.Errorf("remove nonexist should not error: %v", err)
	}
}

func TestKeywordRulesPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	s, _ := NewStore(path)

	rules := []StoredRule{
		{Keyword: "天气", Backend: "openclaw"},
		{Keyword: "代码", Backend: "claude"},
	}
	if err := s.SetKeywordRules(rules); err != nil {
		t.Fatalf("SetKeywordRules: %v", err)
	}

	// 重载
	s2, _ := NewStore(path)
	rules2 := s2.GetKeywordRules()
	if len(rules2) != 2 {
		t.Fatalf("reload want 2 rules, got %d", len(rules2))
	}
	if rules2[0].Keyword != "天气" || rules2[0].Backend != "openclaw" {
		t.Errorf("rule[0] want 天气/openclaw, got %s/%s", rules2[0].Keyword, rules2[0].Backend)
	}
}

func TestUserBackends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	s, _ := NewStore(path)

	if err := s.SetUserBackend("user1", "claude"); err != nil {
		t.Fatalf("SetUserBackend: %v", err)
	}

	m := s.GetUserBackends()
	if m["user1"] != "claude" {
		t.Errorf("user1 want claude, got %s", m["user1"])
	}

	if err := s.ClearUserBackend("user1"); err != nil {
		t.Fatalf("ClearUserBackend: %v", err)
	}
	m = s.GetUserBackends()
	if _, ok := m["user1"]; ok {
		t.Error("user1 should be cleared")
	}
}
