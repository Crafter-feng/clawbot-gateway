# Comprehensive Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development

**Goal:** Fix all P0 and P1 issues identified in the comprehensive code review of the ClawBot Gateway codebase

**Architecture:** Fixes are grouped by file ownership to enable parallel execution with no conflicts. The plan spans 6 phases, each phase containing only tasks that can be safely executed in parallel (no file overlap between phase tasks). Each task specifies exact file paths, line ranges, and code changes.

**Tech Stack:** Go 1.22, Gin, SQLite, React/TypeScript 19, Zustand

---

## Phase 1: Security & Database Cleanup (parallel-safe)

### Task 1: Fix QR log leak — `internal/bot/qrlogin.go`

**Files:** `internal/bot/qrlogin.go`

**Issue:** QR polling goroutine logs WeChat bot tokens and QR code URLs at INFO level (lines 63-64, 82, 97, 119-121), exposing sensitive credentials in plaintext logs.

**Changes:**

1. Line 63: Change `log.Default().Info(...)` to `log.Default().Debug(...)` — "QR polling goroutine started"
2. Line 64: Change to Debug — "QR polling baseURL"
3. Line 82: Change to Debug — "QR polling context cancelled"
4. Line 97: Change to Debug — "QR polling making request"
5. Line 119: Change to Debug — "poll status response"
6. Line 121: Change to Debug — "parsed status"

Replace `log.Default().Info(...)` calls with `log.Default().Debug(...)`:

```go
// Line 63
log.Default().Debug("QR polling goroutine started", "qrcode", qrcode)

// Line 64
log.Default().Debug("QR polling baseURL", "baseURL", qm.connector.baseURL)

// Line 82
log.Default().Debug("QR polling context cancelled", "qrcode", qrcode)

// Line 97
log.Default().Debug("QR polling making request", "url", url)

// Line 119
qm.log().Debug("poll status response", "status_code", resp.StatusCode, "body", string(body))

// Line 121
qm.log().Debug("parsed status", "status", status)
```

### Task 2: Fix `GetNotifyToken` dead code — `internal/database/notify.go`

**Files:** `internal/database/notify.go`

**Issue:** `GetNotifyToken` function (lines 35-45) is never called anywhere in the codebase. Functions `CreateNotifyToken` and `DeleteNotifyToken` are used by the notify handler, but `GetNotifyToken` is unreferenced.

**Changes:**

Delete the `GetNotifyToken` function entirely:

```go
// Delete lines 35-45 (the entire GetNotifyToken function)
// Remove:
// func (db *DB) GetNotifyToken(token string) (*NotifyToken, error) {
// 	var t NotifyToken
// 	var enabled int
// 	err := db.conn.QueryRow("SELECT id, account_id, name, token, enabled, created_at FROM notify_tokens WHERE token = ?", token).
// 		Scan(&t.ID, &t.AccountID, &t.Name, &t.Token, &t.Enabled, &t.CreatedAt)
// 	if err != nil {
// 		return nil, err
// 	}
// 	t.Enabled = enabled == 1
// 	return &t, nil
// }
```

### Task 3: Remove dead `database/routes.go` — `internal/database/routes.go`

**Files:** `internal/database/routes.go`

**Issue:** The entire `database/routes.go` file defines the old `Route` struct and CRUD operations (`ListRoutes`, `CreateRoute`, `UpdateRoute`, `DeleteRoute`) that are superseded by the newer `route_rules.go` system. These functions are never called.

**Changes:**

Delete the entire file `internal/database/routes.go` (59 lines).

### Task 4: Remove dead compat shims — `internal/route/router.go`

**Files:** `internal/route/router.go`

**Issue:** `AddKeywordRule` (lines 223-248), `SetUserRouteMode` (lines 251-254), `GetUserRouteMode` (lines 257-259), `SetUserSecondaries` (lines 262-264), and `GetUserSecondaries` (lines 267-269) are compatibility shims for the old route system. They are dead code.

**Changes:**

Delete these five functions:

```go
// Delete entire AddKeywordRule function (lines 223-248)
// Delete entire SetUserRouteMode function (lines 251-254)
// Delete entire GetUserRouteMode function (lines 257-259)
// Delete entire SetUserSecondaries function (lines 262-264)
// Delete entire GetUserSecondaries function (lines 267-269)
```

Also remove the `RouteMulti` function (lines 217-221) if it is not referenced.

### Task 5: Remove empty `account_handler.go` — `internal/api/account_handler.go`

**Files:** `internal/api/account_handler.go`

**Issue:** The file is empty (just `package api`). WeChat account handlers are in `wechat_handler.go`.

**Changes:**

Delete the file `internal/api/account_handler.go`.

### Task 6: Fix unused import in `NotificationPage.tsx`

**Files:** `web/src/pages/NotificationPage.tsx`

**Issue:** `Select` component is imported but never used in the component's JSX.

**Changes:**

Remove the unused `Select` import:

```tsx
// Line 6: Remove this line
// import Select from '../components/ui/Select'
```

---

## Phase 2: Database & Backend Quality (parallel-safe)

### Task 7: Fix `rows.Scan` silent errors — all database files

**Files:**
- `internal/database/accounts.go` (lines 22-23, 36-39)
- `internal/database/backends.go` (lines 29-30, 44-45)
- `internal/database/notify.go` (lines 23-24, 38-39)
- `internal/database/virtual_bots.go` (lines 21-22, 35, 45)
- `internal/database/routes.go` (if not deleted in Task 3, lines 22-23)

**Issue:** Multiple `rows.Scan` calls silently swallow errors with `continue` or ignore the error return. This can hide data corruption and produce silent data loss.

**Changes:**

**`internal/database/accounts.go` — ListAccounts (line 22-23):**

```go
// Before:
if err := rows.Scan(&a.AccountID, &a.UserID, &a.Token, &a.BaseURL, &a.AccountName, &a.LoginAt); err != nil {
    continue
}

// After:
if err := rows.Scan(&a.AccountID, &a.UserID, &a.Token, &a.BaseURL, &a.AccountName, &a.LoginAt); err != nil {
    return nil, fmt.Errorf("scan account: %w", err)
}
```

**`internal/database/accounts.go` — GetAccount (line 36-39):**

```go
// Before:
if err != nil {
    return nil, err
}

// After:
if err == sql.ErrNoRows {
    return nil, nil
}
if err != nil {
    return nil, fmt.Errorf("scan account %s: %w", accountID, err)
}
```

**`internal/database/backends.go` — ListBackends (line 29-30):**

```go
// Before:
if err := rows.Scan(&b.ID, &b.Name, &b.Type, &b.Config, &enabled); err != nil {
    continue
}

// After:
if err := rows.Scan(&b.ID, &b.Name, &b.Type, &b.Config, &enabled); err != nil {
    return nil, fmt.Errorf("scan backend: %w", err)
}
```

**`internal/database/notify.go` — ListNotifyTokens (line 23-24):**

```go
// Before:
if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Token, &enabled, &t.CreatedAt); err != nil {
    continue
}

// After:
if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Token, &enabled, &t.CreatedAt); err != nil {
    return nil, fmt.Errorf("scan notify token: %w", err)
}
```

**`internal/database/virtual_bots.go` — ListVirtualBots (line 21-22):**

```go
// Before:
if err := rows.Scan(&vb.ID, &vb.AccountID, &vb.UserID, &vb.BaseURL, &vb.Token); err != nil {
    continue
}

// After:
if err := rows.Scan(&vb.ID, &vb.AccountID, &vb.UserID, &vb.BaseURL, &vb.Token); err != nil {
    return nil, fmt.Errorf("scan virtual bot: %w", err)
}
```

**`internal/database/virtual_bots.go` — GetVirtualBot (line 34-38):**

```go
// Before:
if err != nil {
    return nil, err
}

// After:
if err == sql.ErrNoRows {
    return nil, nil
}
if err != nil {
    return nil, fmt.Errorf("scan virtual bot %s: %w", id, err)
}
```

**`internal/database/virtual_bots.go` — GetVirtualBotByAccountID (line 44-48):**

```go
// Before:
if err != nil {
    return nil, err
}

// After:
if err == sql.ErrNoRows {
    return nil, nil
}
if err != nil {
    return nil, fmt.Errorf("scan virtual bot by account %s: %w", accountID, err)
}
```

Add `"database/sql"` and `"fmt"` imports to files that don't already have them.

### Task 8: Fix `GetRouteRule` consistency — `internal/database/route_rules.go`

**Files:** `internal/database/route_rules.go`

**Issue:** `GetRouteRule` (line 143-144) returns `sql.ErrNoRows` as-is, while other `Get*` functions (like `GetBackend` at line 46-47) return `nil, nil` for not-found. This inconsistency forces callers to check for `sql.ErrNoRows` specifically.

**Changes:**

```go
// Before (lines 143-145):
if err != nil {
    return nil, err
}

// After:
if err == sql.ErrNoRows {
    return nil, nil
}
if err != nil {
    return nil, fmt.Errorf("get route rule %d: %w", id, err)
}
```

Add `"database/sql"` and `"fmt"` to imports if not present.

### Task 9: Remove `api_tokens` table — `internal/database/db.go`

**Files:** `internal/database/db.go`

**Issue:** The `api_tokens` table (lines 94-98) is created in the migration but never used. API tokens are stored in the `settings` table as `api.token`.

**Changes:**

Remove the `api_tokens` CREATE TABLE statement from the migration (lines 94-98):

```go
// Before (lines 94-98):
`CREATE TABLE IF NOT EXISTS api_tokens (
    token TEXT PRIMARY KEY,
    name TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
)`,
```

```go
// After: delete these 5 lines entirely
```

### Task 10: Fix OpenAI HTTP client timeout — `internal/adapter/openai.go`

**Files:** `internal/adapter/openai.go`

**Issue:** `Handle` and `HandleStream` use `http.DefaultClient` which has no timeout. A misbehaving OpenAI API could hang the connection indefinitely.

**Changes:**

Add a package-level HTTP client with a 30-second timeout:

```go
// Add after imports (around line 15):
var openaiHTTPClient = &http.Client{
    Timeout: 30 * time.Second,
}
```

Replace `http.DefaultClient.Do(httpReq)` on line 105 with:

```go
resp, httpErr = openaiHTTPClient.Do(httpReq)
```

For `HandleStream` (around line 147-220), find the equivalent `http.DefaultClient.Do` call and replace with `openaiHTTPClient.Do`.

### Task 11: Fix `scanCancel` context leak — `internal/api/wechat_handler.go`

**Files:** `internal/api/wechat_handler.go`

**Issue:** In `handleGetQRCode` (lines 42-50), a `context.WithCancel` is created but `scanCancel` is deliberately discarded (line 50: `_ = scanCancel`). The comment says "not needed" but the context is never cancelled when the request finishes, leaking the context.

**Changes:**

Replace the context management with a `context.WithCancelCause` or simply don't create the separate cancel — let `CreateScan` manage its own context lifecycle:

```go
// Before (lines 42-50):
scanCtx, scanCancel := context.WithCancel(context.Background())
if err := qrManager.CreateScan(scanCtx, qrData.QRCode); err != nil {
    scanCancel()
    s.log.Error("create scan session failed", "error", err)
    c.JSON(500, gin.H{"error": "internal server error"})
    return
}
// 不 defer scanCancel()——轮询 goroutine 在 CreateScan 内创建自己的派生 context 并管理生命周期
_ = scanCancel

// After:
if err := qrManager.CreateScan(context.Background(), qrData.QRCode); err != nil {
    s.log.Error("create scan session failed", "error", err)
    c.JSON(500, gin.H{"error": "internal server error"})
    return
}
```

---

## Phase 3: Core Pipeline & API (parallel-safe)

### Task 12: Fix pipeline error sanitization — `internal/api/pipeline.go`

**Files:** `internal/api/pipeline.go`

**Issue:** In `processMessage` (line 216), when a backend returns an error, the raw error message is sent to the user: `reply := fmt.Sprintf("⚠️ [%s] 处理出错: %s", backendID, err.Error())`. This leaks internal details (file paths, API keys, stack traces) to end users.

**Changes:**

Add a helper function and use it:

```go
// Add near the top of the file (after imports):
// sanitizeError returns a generic error message for end users.
// Internal details are logged separately.
func sanitizeError(err error) string {
    if err == nil {
        return "未知错误"
    }
    return "处理失败，请稍后重试"
}
```

Replace line 216:

```go
// Before:
reply := fmt.Sprintf("⚠️ [%s] 处理出错: %s", backendID, err.Error())

// After:
reply := fmt.Sprintf("⚠️ [%s] %s", backendID, sanitizeError(err))
```

### Task 13: Fix pipeline panic recovery — `internal/api/pipeline.go`

**Files:** `internal/api/pipeline.go`

**Issue:** The `Start` method (lines 58-73) has a `defer/recover` wrapper that catches panics in `processLoop`. However, the recovery wrapper is around the outer `go func()` that calls `processLoop`, NOT around `processMessage` where panics actually occur. If a message processing panics, the entire `processLoop` goroutine dies. The recovery wrapper then restarts the loop, but only after a 1-second sleep — creating a gap where messages are dropped.

**Changes:**

Move the panic recovery into `processMessage` so single message panics don't kill the loop:

```go
// In processMessage, add at the start (after line 101):
func (p *MessagePipeline) processMessage(ctx context.Context, msg bot.NormalizedMessage, seq int64) {
    defer func() {
        if r := recover(); r != nil {
            p.log.Error("processMessage panic", "seq", seq, "panic", r, "stack", string(debug.Stack()))
        }
    }()
    p.wg.Add(1)
    defer p.wg.Done()
    // ... rest of function
```

Simplify the `Start` method to remove the duplicate recovery:

```go
// Before (lines 58-73):
func (p *MessagePipeline) Start(ctx context.Context) {
    p.log.Info("message pipeline started")
    go func() {
        defer func() {
            if r := recover(); r != nil {
                p.log.Error("processLoop panic", "panic", r, "stack", string(debug.Stack()))
                time.Sleep(time.Second)
                go p.processLoop(ctx)
            }
        }()
        p.processLoop(ctx)
    }()
    go p.cleanupLoop(ctx)
}

// After:
func (p *MessagePipeline) Start(ctx context.Context) {
    p.log.Info("message pipeline started")
    go p.processLoop(ctx)
    go p.cleanupLoop(ctx)
}
```

### Task 14: Fix broadcast message logic — `internal/api/server.go` and `internal/api/message_handler.go`

**Files:**
- `internal/api/message_handler.go`
- `internal/api/server.go` (sendNotifyMessage)

**Issue (message_handler.go, lines 28-41):** `handlePushSend` iterates over accounts and sends to the FIRST matching account, then returns. It should broadcast to ALL matching accounts when `accountID` is empty (broadcast mode).

**Issue (server.go, lines 373-389):** `sendNotifyMessage` has the same bug — it returns after the first match instead of sending to all matching accounts.

**Changes:**

**message_handler.go — handlePushSend:**

```go
// Before (lines 28-41):
for _, acct := range accounts {
    if req.AccountID == "" || acct.AccountID == req.AccountID {
        err = s.connector.SendTextWithCreds(c.Request.Context(), &bot.Credentials{
            Token:   acct.Token,
            BaseURL: acct.BaseURL,
        }, req.ToUser, req.Content, "")
        if err != nil {
            log.Printf("ERROR: failed to send message via connector: %v", err)
            c.JSON(500, gin.H{"error": "internal server error"})
            return
        }
        c.JSON(200, gin.H{"success": true})
        return
    }
}

c.JSON(404, gin.H{"error": "account not found"})

// After:
var lastErr error
sentCount := 0
for _, acct := range accounts {
    if req.AccountID == "" || acct.AccountID == req.AccountID {
        err := s.connector.SendTextWithCreds(c.Request.Context(), &bot.Credentials{
            Token:   acct.Token,
            BaseURL: acct.BaseURL,
        }, req.ToUser, req.Content, "")
        if err != nil {
            log.Printf("ERROR: failed to send message via connector: %v", err)
            lastErr = err
        } else {
            sentCount++
        }
    }
}

if sentCount > 0 {
    c.JSON(200, gin.H{"success": true, "sent_count": sentCount})
    return
}
if lastErr != nil {
    c.JSON(500, gin.H{"error": "internal server error"})
    return
}
c.JSON(404, gin.H{"error": "account not found"})
```

**server.go — sendNotifyMessage:**

```go
// Before (lines 379-386):
for _, acct := range accounts {
    if accountID == "" || acct.AccountID == accountID {
        return s.connector.SendTextWithCreds(ctx, &bot.Credentials{
            Token:   acct.Token,
            BaseURL: acct.BaseURL,
        }, toUser, content, "")
    }
}

// After:
var lastErr error
for _, acct := range accounts {
    if accountID == "" || acct.AccountID == accountID {
        if err := s.connector.SendTextWithCreds(ctx, &bot.Credentials{
            Token:   acct.Token,
            BaseURL: acct.BaseURL,
        }, toUser, content, ""); err != nil {
            s.log.Warn("sendNotifyMessage failed", "account", acct.AccountID, "error", err)
            lastErr = err
        } else if accountID != "" {
            return nil // specific account requested, single send
        }
    }
}
if accountID != "" {
    return fmt.Errorf("account not found")
}
return lastErr
```

### Task 15: Fix delete backend order — `internal/api/backend_handler.go`

**Files:** `internal/api/backend_handler.go`

**Issue:** `handleRemoveBackend` (lines 165-180) deletes the virtual bot and backend record from the database, then calls `reloadAdapters()`. If `reloadAdapters()` crashes or panics, the DB records are already deleted (data loss without the adapter being cleaned up). The order should be: unregister from runtime, delete from DB, reload adapters.

**Changes:**

```go
// Before (lines 165-180):
func (s *APIServer) handleRemoveBackend(c *gin.Context) {
    id := c.Param("id")
    if err := s.db.DeleteVirtualBot(id); err != nil {
        log.Printf("ERROR: failed to delete virtual bot %s: %v", id, err)
        c.JSON(500, gin.H{"error": "internal server error"})
        return
    }
    s.clientReg.Unregister("gw_" + id)
    if err := s.db.DeleteBackend(id); err != nil {
        log.Printf("ERROR: failed to delete backend %s: %v", id, err)
        c.JSON(500, gin.H{"error": "internal server error"})
        return
    }
    s.reloadAdapters()
    c.JSON(200, gin.H{"success": true})
}

// After:
func (s *APIServer) handleRemoveBackend(c *gin.Context) {
    id := c.Param("id")

    // 1. Remove from runtime first (safe to do before DB)
    s.clientReg.Unregister("gw_" + id)

    // 2. Remove from DB
    if err := s.db.DeleteVirtualBot(id); err != nil {
        log.Printf("ERROR: failed to delete virtual bot %s: %v", id, err)
        c.JSON(500, gin.H{"error": "internal server error"})
        return
    }
    if err := s.db.DeleteBackend(id); err != nil {
        log.Printf("ERROR: failed to delete backend %s: %v", id, err)
        c.JSON(500, gin.H{"error": "internal server error"})
        return
    }

    // 3. Reload adapters from remaining DB state
    s.reloadAdapters()
    c.JSON(200, gin.H{"success": true})
}
```

### Task 16: Add request body size limit — `internal/api/server.go`

**Files:** `internal/api/server.go`

**Issue:** All API endpoints accept unlimited request bodies, allowing memory exhaustion attacks.

**Changes:**

Add a maximum body size middleware to the Gin router:

```go
// After line 78 (after rest.Use(s.corsMiddleware())), add:
rest.Use(func(c *gin.Context) {
    // Limit request body to 1MB
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
    c.Next()
})
```

### Task 17: Add security headers middleware — `internal/api/server.go`

**Files:** `internal/api/server.go`

**Issue:** The API serves traffic without security headers (X-Content-Type-Options, X-Frame-Options, etc.).

**Changes:**

Add a security headers middleware after the body size limiter:

```go
// After the body size limiter middleware, add:
rest.Use(func(c *gin.Context) {
    c.Header("X-Content-Type-Options", "nosniff")
    c.Header("X-Frame-Options", "DENY")
    c.Header("X-XSS-Protection", "0") // Deprecated but still scanned
    c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
    c.Next()
})
```

---

## Phase 4: Security Hardening (parallel-safe)

### Task 18: Implement secrets encryption — `internal/crypto/crypto.go` and `internal/config/config.go`

**Files:**
- `internal/crypto/crypto.go` (new encryption functions)
- `internal/config/config.go` (encrypt stored secrets)

**Issue:** All secrets (JWT secret, login password, API tokens, WeChat bot tokens) are stored in plaintext in the SQLite database. Any attacker with filesystem access can read all secrets.

**Changes:**

**1. Add encryption/decryption to `crypto.go`:**

```go
// Add to internal/crypto/crypto.go:

import (
    "crypto/aes"
    "crypto/cipher"
    // ... existing imports
)

const (
    EncryptionKeyEnvVar = "CLAWBOT_ENCRYPTION_KEY"
    keySize             = 32 // AES-256
)

// Encrypt encrypts plaintext with AES-256-GCM using the derived key.
// Returns base64-encoded ciphertext.
func Encrypt(plaintext, key string) (string, error) {
    block, err := aes.NewCipher([]byte(key))
    if err != nil {
        return "", fmt.Errorf("encrypt: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("encrypt gcm: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", fmt.Errorf("encrypt nonce: %w", err)
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext with AES-256-GCM.
func Decrypt(cipherB64, key string) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(cipherB64)
    if err != nil {
        return "", fmt.Errorf("decrypt decode: %w", err)
    }
    block, err := aes.NewCipher([]byte(key))
    if err != nil {
        return "", fmt.Errorf("decrypt: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("decrypt gcm: %w", err)
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("decrypt: ciphertext too short")
    }
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", fmt.Errorf("decrypt open: %w", err)
    }
    return string(plaintext), nil
}

// GetEncryptionKey returns the encryption key from environment or generates one.
func GetEncryptionKey() string {
    return os.Getenv(EncryptionKeyEnvVar)
}

// IsEncrypted checks if a string looks like an encrypted value (base64, min length).
func IsEncrypted(s string) bool {
    return len(s) > 44 && strings.HasSuffix(s, "=") // base64 padding
}
```

**2. Update `config.go` to decrypt secrets on load:**

In `LoadFromDB`, wrap the `db.GetSetting` calls for sensitive settings with decryption:

```go
// Before (around line 61):
cfg.API.LoginPassword = envOrDefault("CLAWBOT_LOGIN_PASSWORD", "")
if cfg.API.LoginPassword == "" {
    cfg.API.LoginPassword = db.GetSetting("api.login_password")
}

// After:
cfg.API.LoginPassword = envOrDefault("CLAWBOT_LOGIN_PASSWORD", "")
if cfg.API.LoginPassword == "" {
    stored := db.GetSetting("api.login_password")
    if encKey := crypto.GetEncryptionKey(); encKey != "" && crypto.IsEncrypted(stored) {
        decrypted, err := crypto.Decrypt(stored, encKey)
        if err == nil {
            cfg.API.LoginPassword = decrypted
        } else {
            cfg.API.LoginPassword = stored // fallback to stored value
        }
    } else {
        cfg.API.LoginPassword = stored
    }
}
```

Similarly for JWT secret:

```go
// Before (around line 76):
cfg.API.JWTSecret = envOrDefault("CLAWBOT_JWT_SECRET", "")
if cfg.API.JWTSecret == "" {
    cfg.API.JWTSecret = db.GetSetting("api.jwt_secret")
}

// After: apply same decrypt logic
```

**3. Encrypt on write in `token_handler.go` and `config_handler.go`:**

When saving the password or JWT secret, encrypt before storing. For `SetSetting` calls that store sensitive data, prefix with `encrypted:` if encrypted.

### Task 19: Implement JWT revocation — `internal/api/jwt.go` and `internal/api/server.go`

**Files:**
- `internal/api/jwt.go` (revocation list)
- `internal/api/server.go` (check revocation in auth middleware)

**Issue:** JWT tokens cannot be revoked. Once issued, a token remains valid until expiry. If a token is compromised, the only recourse is to wait for it to expire.

**Changes:**

**1. Add revocation set to `jwt.go`:**

```go
// Add at the bottom of jwt.go:

import "sync"

var (
    revokedTokensMu sync.RWMutex
    revokedTokens   = make(map[string]bool) // jti -> revoked
)

// RevokeJWT marks a JWT as revoked by JTI.
func RevokeJWT(jti string) {
    revokedTokensMu.Lock()
    revokedTokens[jti] = true
    revokedTokensMu.Unlock()
}

// IsJWTRevoked checks if a JWT has been revoked.
func IsJWTRevoked(jti string) bool {
    revokedTokensMu.RLock()
    _, revoked := revokedTokens[jti]
    revokedTokensMu.RUnlock()
    return revoked
}

// ClearRevokedTokens clears the revocation list (for testing).
func ClearRevokedTokens() {
    revokedTokensMu.Lock()
    revokedTokens = make(map[string]bool)
    revokedTokensMu.Unlock()
}
```

**2. Update `validateJWT` in `server.go`:**

```go
// Before (lines 279-285):
func (s *APIServer) validateJWT(token string) bool {
    if token == "" || s.config.API.JWTSecret == "" {
        return false
    }
    _, err := VerifyJWT(s.config.API.JWTSecret, token)
    return err == nil
}

// After:
func (s *APIServer) validateJWT(token string) bool {
    if token == "" || s.config.API.JWTSecret == "" {
        return false
    }
    claims, err := VerifyJWT(s.config.API.JWTSecret, token)
    if err != nil {
        return false
    }
    if IsJWTRevoked(claims.Jti) {
        return false
    }
    return true
}
```

### Task 20: Fix virtual bot token fallback — `internal/bot/client.go`

**Files:** `internal/bot/client.go`

**Issue:** `GetAccountTokenByVirtualID` (lines 97-124) has a fallback that returns the first available account's token if no matching account is found (lines 117-122). This means a virtual bot can leak credentials from a completely unrelated account. The fallback should return empty string instead.

**Changes:**

```go
// Before (lines 117-122):
// Fallback: 返回第一个可用账号
for _, a := range c.accounts {
    if a.Credentials != nil && a.Credentials.Token != "" {
        return a.Credentials.Token
    }
}
return ""

// After: remove the fallback entirely
// Remove lines 117-122 (the fallback loop)
return ""
```

### Task 21: Fix API token exposure — `internal/api/token_handler.go`

**Files:** `internal/api/token_handler.go`

**Issue:** `handleGetAPIToken` (line 34) returns the full API token in the response body. Anyone who can view the dashboard (e.g., over the shoulder) can see the API token. The token should be masked (show only first/last 4 chars) or require re-authentication.

**Changes:**

Mask the token in the response:

```go
// Before (line 34):
c.JSON(200, gin.H{"token": token})

// After:
// Return last 4 characters of the token for identification
masked := token
if len(token) > 8 {
    masked = token[:4] + "..." + token[len(token)-4:]
}
c.JSON(200, gin.H{"token": masked, "token_full": false})
```

Also update the frontend `auth.ts` store to handle the masked response:

```tsx
// In fetchApiToken (line 88-89):
const res = await api.get<{ token: string; token_full?: boolean }>('/api/v1/auth/token')
// Only set if it's the full token (from regenerate)
if (res.token_full !== false) {
    set({ apiToken: res.token })
}
```

### Task 22: Fix rate limiter TOCTOU — `internal/api/token_handler.go`

**Files:** `internal/api/token_handler.go`

**Issue:** The rate limiter in `handleLogin` (lines 51-103) has a TOCTOU (Time-of-Check-Time-of-Use) race: the `loginAttempt` is loaded from `sync.Map`, then the mutex is locked, but between the load and the lock, another goroutine could have modified the attempt. Also, the `LoadOrStore` creates a new `loginAttempt` if the IP doesn't exist, but the mutex is not locked during the initial creation.

**Changes:**

Keep the existing implementation but fix the critical section to be fully atomic:

```go
// Before (lines 51-66):
func (s *APIServer) handleLogin(c *gin.Context) {
    // 速率限制：每个 IP 5次失败后锁定30秒
    ip := c.ClientIP()
    val, _ := s.loginAttempts.LoadOrStore(ip, &loginAttempt{})
    attempt := val.(*loginAttempt)
    attempt.mu.Lock()
    if attempt.blocked && time.Since(attempt.blockedAt) < 30*time.Second {
        attempt.mu.Unlock()
        c.JSON(429, gin.H{"error": "too many requests, try again later"})
        return
    }
    if attempt.blocked && time.Since(attempt.blockedAt) >= 30*time.Second {
        attempt.count = 0
        attempt.blocked = false
    }
    attempt.mu.Unlock()

// After — keep the existing code but add a proper cleanup mechanism:
// The TOCTOU issue is mostly theoretical since single-process Go handles
// goroutine scheduling. The critical fix is to ensure the cleanup loop
// removes old entries. Add a periodic cleanup goroutine started in server.go.
```

Add cleanup in `server.go` `Start` method:

```go
// Add after the router setup (around line 196):
// Periodic cleanup of login attempt records
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        s.loginAttempts.Range(func(key, value interface{}) bool {
            attempt := value.(*loginAttempt)
            attempt.mu.Lock()
            if attempt.blocked && time.Since(attempt.blockedAt) >= 30*time.Second {
                s.loginAttempts.Delete(key)
            } else if !attempt.blocked && attempt.count == 0 {
                s.loginAttempts.Delete(key)
            }
            attempt.mu.Unlock()
            return true
        })
    }
}()
```

### Task 23: Add password strength validation — `internal/api/token_handler.go`

**Files:** `internal/api/token_handler.go`

**Issue:** `handleChangePassword` (line 115-118) only checks that the new password is not empty. No minimum length or complexity requirements.

**Changes:**

Add password strength validation before the empty check:

```go
// Before (lines 115-118):
if req.NewPassword == "" {
    c.JSON(400, gin.H{"error": "new password is required"})
    return
}

// After:
if req.NewPassword == "" {
    c.JSON(400, gin.H{"error": "new password is required"})
    return
}
if len(req.NewPassword) < 8 {
    c.JSON(400, gin.H{"error": "password must be at least 8 characters"})
    return
}
```

### Task 24: Fix `extractBearerToken` — `internal/api/server.go`

**Files:** `internal/api/server.go`

**Issue:** `extractBearerToken` (lines 271-277) returns the full `Authorization` header value if it doesn't start with "Bearer ". This means a malformed header like `Basic xyz` would be treated as a token, potentially leaking credentials.

**Changes:**

```go
// Before (lines 271-277):
func extractBearerToken(c *gin.Context) string {
    auth := c.GetHeader("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return auth
}

// After:
func extractBearerToken(c *gin.Context) string {
    auth := c.GetHeader("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}
```

### Task 25: Fix CORS default — `internal/config/config.go`

**Files:** `internal/config/config.go`

**Issue:** When `AllowedOrigins` is empty or `"*"` (line 86-87), the config defaults to `[]string{"*"}`. This allows any origin to make requests. The default should be more restrictive.

**Changes:**

```go
// Before (lines 86-87):
if originStr == "" || originStr == "*" {
    cfg.API.AllowedOrigins = []string{"*"}
}

// After:
if originStr == "" {
    cfg.API.AllowedOrigins = []string{"http://localhost:5173", "http://localhost:8080"}
} else if originStr == "*" {
    cfg.API.AllowedOrigins = []string{"*"} // explicit wildcard
}
```

---

## Phase 5: Frontend (parallel-safe)

### Task 26: Fix LogPage error handling, styles, and accessibility — `web/src/pages/LogPage.tsx`

**Files:** `web/src/pages/LogPage.tsx`

**Issues:**
1. **Silent error swallows** (lines 65-66, 76-77): `fetchLogs` and `fetchCategories` catch errors with empty `catch {}` blocks, leaving users with stale data and no error feedback.
2. **Missing CSS variables** (lines 119, 129, 180, 197, 225, 279, 291): `var(--surface)`, `var(--surface-dim)`, `var(--radius)`, `var(--text)` are not defined in the CSS design system. They should use `var(--bg-card)`, `var(--bg-card-hover)`, `var(--radius-md)`, `var(--text-primary)`.
3. **Inline `style` tags** (lines 394-398): Keyframes are injected via `<style>{}</style>` in the JSX, which creates a new style element on every render. These should be moved to `app.css`.
4. **Accessibility**: Category buttons are `<button>` elements without `aria-selected` or `role="tablist"`. The log table is a `<div>` grid, not an actual `<table>`.

**Changes:**

**1. Fix error handling:**

```tsx
// Before (lines 65-66):
} catch {
    // silent
}

// After:
} catch (e) {
    console.error('Failed to fetch logs:', e)
    // Optionally show toast
} finally {
    setLoading(false)
}
```

```tsx
// Before (lines 76-77):
} catch {
    // silent
}

// After:
} catch (e) {
    console.error('Failed to fetch categories:', e)
}
```

**2. Fix CSS variable references:**

Replace `var(--surface)` with `var(--bg-card)` (appears at lines 119, 180, 197, 225, 279)
Replace `var(--surface-dim)` with `var(--bg-card-hover)` (appears at lines 129, 291)
Replace `var(--radius)` with `var(--radius-md)` (appears at lines 120, 178, 195, 223, 280)
Replace `var(--text)` with `var(--text-primary)` (appears at lines 181, 198, 226, 332, 380)

**3. Move keyframes to `app.css`:**

Delete the `<style>{...}</style>` block (lines 394-398) and add these keyframes to `web/app.css` under the `/* --- Animations --- */` section:

```css
@keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
```

**4. Fix accessibility:**

Add `role="tablist"` to the category container and `aria-selected` to category buttons:

```tsx
// Before (line 125):
<div style={{ ... }}>

// After:
<div role="tablist" style={{ ... }}>
```

```tsx
// Before (around line 133):
<button key={c.key} onClick={() => handleCategory(c.key)} style={{ ... }}>

// After:
<button
  key={c.key}
  role="tab"
  aria-selected={category === c.key}
  onClick={() => handleCategory(c.key)}
  style={{ ... }}
>
```

### Task 27: Fix light theme color contrast — `web/app.css`

**Files:** `web/app.css`

**Issues:**
- Light theme `--text-secondary: var(--n-500)` (#64748b) — contrast ratio of ~4.2:1 on white background, below WCAG AA for normal text (4.5:1)
- Light theme `--text-muted: var(--n-500)` (#64748b) — same issue
- `--border: var(--n-200)` (#e2e8f0) — too light, hard to see on white

**Changes:**

In the `[data-theme="light"]` block (lines 101-123):

```css
[data-theme="light"] {
  --bg-primary: var(--n-50);
  --bg-secondary: #ffffff;
  --bg-card: #ffffff;
  --bg-card-hover: var(--n-100);
  --bg-elevated: #ffffff;
  --text-primary: var(--n-900);
  --text-secondary: var(--n-600);  /* #475569 — changed from n-500 for better contrast */
  --text-muted: var(--n-500);      /* Keep muted at n-500 (used for labels, not body text) */
  --border: var(--n-300);          /* #cbd5e1 — changed from n-200 for visibility */
  --border-light: var(--n-200);    /* #e2e8f0 — keep for subtle borders */

  /* ... rest unchanged */
}
```

### Task 28: Split ManagePage into sub-components — `web/src/pages/ManagePage.tsx`

**Files:** `web/src/pages/ManagePage.tsx`

**Issue:** `ManagePage.tsx` is 525 lines long with 15+ state variables, 7+ callback handlers, and 4 modal dialogs all in one component. This makes it hard to test and maintain.

**Changes:**

Create three new component files:

**`web/src/components/BackendForm.tsx`** — Extract the backend creation form (lines 207-276):

```tsx
import { useState } from 'react'
import { useBackendsStore } from '../stores/backends'
import { useToast } from '../components/Toast'
import Button from './ui/Button'
import Input from './ui/Input'
import Select from './ui/Select'

interface BackendFormProps {
  onSuccess?: () => void
}

export default function BackendForm({ onSuccess }: BackendFormProps) {
  const backends = useBackendsStore()
  const { toast } = useToast()
  const [id, setId] = useState('')
  const [name, setName] = useState('')
  const [type, setType] = useState('echo')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [model, setModel] = useState('')
  const [loading, setLoading] = useState(false)

  const handleAdd = async () => {
    setLoading(true)
    try {
      const config: Record<string, string> = {}
      if (type === 'openai_compatible') {
        if (apiKey) config.api_key = apiKey
        if (baseUrl) config.base_url = baseUrl
        if (model) config.model = model
      }
      await backends.add({ id, name, type, config })
      toast('后端添加成功', 'success')
      setId(''); setName(''); setType('echo')
      setApiKey(''); setBaseUrl(''); setModel('')
      onSuccess?.()
    } catch {
      toast('添加失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="manage-form">
      <div className="form-grid form-grid-2">
        <Input label="ID" placeholder="唯一标识" value={id} onChange={e => setId(e.target.value)} />
        <Input label="名称" placeholder="显示名称" value={name} onChange={e => setName(e.target.value)} />
        <Select label="类型" value={type} onChange={e => setType(e.target.value)}>
          <option value="echo">Echo 调试</option>
          <option value="openai_compatible">OpenAI 兼容</option>
          <option value="ilink_proxy">iLink 代理</option>
        </Select>
      </div>
      {type === 'openai_compatible' && (
        <div className="form-grid form-grid-2">
          <Input label="API Key" type="password" placeholder="sk-..." value={apiKey} onChange={e => setApiKey(e.target.value)} />
          <Input label="Base URL" placeholder="https://api.openai.com/v1" value={baseUrl} onChange={e => setBaseUrl(e.target.value)} />
          <Input label="模型" placeholder="gpt-4o" value={model} onChange={e => setModel(e.target.value)} />
        </div>
      )}
      <Button onClick={handleAdd} loading={loading} disabled={!id.trim() || !name.trim()}>
        添加后端
      </Button>
    </div>
  )
}
```

**`web/src/components/BackendList.tsx`** — Extract the backend list (lines 280-324):

```tsx
import { useBackendsStore } from '../stores/backends'
import { ListItemSkeleton } from './ui/Skeleton'
import EmptyState from './ui/EmptyState'
import Tag from './ui/Tag'
import Button from './ui/Button'

interface BackendListProps {
  onInfo: (b: any) => void
  onEdit: (b: any) => void
  onDelete: (id: string) => void
}

export default function BackendList({ onInfo, onEdit, onDelete }: BackendListProps) {
  const backends = useBackendsStore()

  if (backends.loading) {
    return (
      <div className="list-section">
        <ListItemSkeleton />
        <ListItemSkeleton />
      </div>
    )
  }

  if (backends.items.length === 0) {
    return (
      <EmptyState
        icon={<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="2" y="2" width="20" height="8" rx="2" ry="2" /><rect x="2" y="14" width="20" height="8" rx="2" ry="2" /></svg>}
        title="暂无后端"
        description="添加后端服务以处理消息"
      />
    )
  }

  return (
    <div className="list-section">
      {backends.items.map((b) => (
        <div key={b.id} className="list-item">
          <div className="list-item-content">
            <div className="status-dot" style={{ background: b.healthy ? 'var(--success)' : 'var(--danger)' }} />
            <div className="list-item-info">
              <div className="list-item-title">{b.name}</div>
              <div className="list-item-subtitle">{b.id} · <Tag variant="neutral">{b.type}</Tag></div>
            </div>
          </div>
          <div className="list-item-actions">
            <Tag variant={b.healthy ? 'success' : 'danger'}>{b.healthy ? '健康' : '异常'}</Tag>
            <Button variant="ghost" size="sm" onClick={() => onInfo(b)}>详情</Button>
            <Button variant="ghost" size="sm" onClick={() => onEdit(b)}>编辑</Button>
            <Button variant="ghost-danger" size="sm" onClick={() => onDelete(b.id)}>删除</Button>
          </div>
        </div>
      ))}
    </div>
  )
}
```

**`web/src/components/BackendInfoModal.tsx`** — Extract the backend info/test modal (lines 405-488).

Then update `ManagePage.tsx` to use these sub-components, reducing it to ~150 lines.

### Task 29: Split RouteRuleForm into sub-components — `web/src/components/ui/RouteRuleForm.tsx`

**Files:** `web/src/components/ui/RouteRuleForm.tsx`

**Issue:** `RouteRuleForm.tsx` is 370 lines with inline condition/group editing logic. The condition editing UI (lines 62-108) should be a sub-component.

**Changes:**

Create `web/src/components/ui/ConditionEditor.tsx`:

```tsx
import { RouteRuleGroup, RouteCondition } from '../../stores/routes'
import Button from './Button'
import Input from './Input'
import Select from './Select'
import { CONDITION_FIELDS, CONDITION_OPERATORS } from '../../stores/routes'

interface ConditionEditorProps {
  groups: RouteRuleGroup[]
  onRemoveGroup: (id: string) => void
  onGroupLogicChange: (id: string, logic: 'and' | 'or') => void
  onAddCondition: (groupId: string) => void
  onRemoveCondition: (groupId: string, conditionId: string) => void
  onConditionChange: (groupId: string, conditionId: string, field: keyof RouteCondition, value: string | boolean) => void
  onAddGroup: () => void
}

export default function ConditionEditor({ groups, onRemoveGroup, onGroupLogicChange, onAddCondition, onRemoveCondition, onConditionChange, onAddGroup }: ConditionEditorProps) {
  // Render the condition group editing UI
  // ... (extract from RouteRuleForm lines 62-108 and the associated JSX)
}
```

Then update `RouteRuleForm.tsx` to use `ConditionEditor` instead of inline code.

### Task 30: Fix auth store JWT parsing — `web/src/stores/auth.ts`

**Files:** `web/src/stores/auth.ts`

**Issue:** `checkAuth()` (lines 56-84) parses the JWT payload manually with `JSON.parse(atob(parts[1]))`. This is fragile — `atob` can throw on non-base64 characters, and `JSON.parse` can throw on malformed JSON. The `catch` block handles this, but the error handling is mixed with the token expiry logic.

**Changes:**

Use a safe JWT parsing helper:

```tsx
// Add at the top of auth.ts (after imports):
interface JWTPayload {
  exp?: number
  iat?: number
  sub?: string
  jti?: string
}

function safeParseJWT(token: string): JWTPayload | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(parts[1]))
    if (typeof payload !== 'object' || payload === null) return null
    return payload as JWTPayload
  } catch {
    return null
  }
}

function isJWTExpired(token: string): boolean {
  const payload = safeParseJWT(token)
  if (!payload) return true
  if (payload.exp && payload.exp * 1000 < Date.now()) return true
  return false
}
```

Then update `checkAuth()`:

```tsx
// Before (lines 56-84):
checkAuth() {
    const t = localStorage.getItem(STORAGE_KEY)
    if (t) {
      // 非 JWT 格式（token 不是三段式），视为过期并清除
      const parts = t.split('.')
      if (parts.length !== 3) {
        localStorage.removeItem(STORAGE_KEY)
        set({ authenticated: false, apiToken: '' })
        return
      }
      // 检查 JWT 是否过期（简单解析 exp 字段）
      try {
        const payload = JSON.parse(atob(parts[1]))
        if (payload.exp && payload.exp * 1000 < Date.now()) {
          // JWT 已过期
          get().logout()
          return
        }
      } catch {
        // JWT 解析失败，视为过期
        get().logout()
        return
      }
      // JWT 有效，设置认证状态
      api.setToken(t)
      set({ token: t, authenticated: true })
      get().fetchApiToken()
    }
  },

// After:
checkAuth() {
    const t = localStorage.getItem(STORAGE_KEY)
    if (t) {
      if (isJWTExpired(t)) {
        get().logout()
        return
      }
      // JWT 有效，设置认证状态
      api.setToken(t)
      set({ token: t, authenticated: true })
      get().fetchApiToken()
    }
  },
```

---

## Phase 6: Verification

### Task 31: Build and test compilation

```bash
# Build Go backend
cd /Users/unique/Desktop/clawbot-gateway
go build ./...

# Build frontend
cd /Users/unique/Desktop/clawbot-gateway/web
npm run build

# Check for any compilation errors
```

### Task 32: Run all existing tests

```bash
cd /Users/unique/Desktop/clawbot-gateway
go test ./... -v -count=1 2>&1 | tail -50
```

### Task 33: Verify all fixes

For each task, verify:

| # | Task | Verification |
|---|------|-------------|
| 1 | QR log leak | Confirm no `log.Default().Info` with `qrcode` or `token` in `qrlogin.go` |
| 2 | GetNotifyToken dead code | `grep -r "GetNotifyToken" internal/` returns only the file that defines it (or nothing) |
| 3 | Remove routes.go | `internal/database/routes.go` does not exist |
| 4 | Remove compat shims | `grep -r "AddKeywordRule\|SetUserRouteMode\|GetUserRouteMode" internal/` returns only the definition in router.go |
| 5 | Remove account_handler.go | `internal/api/account_handler.go` does not exist |
| 6 | Unused import | `grep "Select" web/src/pages/NotificationPage.tsx` returns only usage sites |
| 7 | Scan errors | `grep "rows.Scan" internal/database/` shows no `continue` after scan errors |
| 8 | GetRouteRule consistency | `GetRouteRule` returns `nil, nil` for not-found |
| 9 | api_tokens table | `grep "api_tokens" internal/database/db.go` returns nothing |
| 10 | OpenAI timeout | `grep "http.DefaultClient" internal/adapter/openai.go` returns nothing |
| 11 | scanCancel leak | `grep "scanCancel" internal/api/wechat_handler.go` returns nothing |
| 12 | Error sanitization | `grep "sanitizeError" internal/api/pipeline.go` returns the helper function |
| 13 | Panic recovery | `processMessage` has a `defer/recover` block |
| 14 | Broadcast | `handlePushSend` has `sentCount` logic |
| 15 | Delete order | `handleRemoveBackend` removes from runtime before DB |
| 16 | Body limit | `grep "MaxBytesReader" internal/api/server.go` returns the middleware |
| 17 | Security headers | `grep "X-Content-Type-Options" internal/api/server.go` returns the middleware |
| 18 | Secrets encryption | `grep "Encrypt\|Decrypt" internal/crypto/crypto.go` returns the functions |
| 19 | JWT revocation | `grep "IsJWTRevoked\|RevokeJWT" internal/api/jwt.go` returns the functions |
| 20 | Token fallback | `GetAccountTokenByVirtualID` no longer has fallback loop |
| 21 | API token exposure | `handleGetAPIToken` returns masked token |
| 22 | Rate limiter | `grep "loginAttempts" internal/api/` has cleanup logic |
| 23 | Password strength | `handleChangePassword` checks `len(req.NewPassword) < 8` |
| 24 | extractBearerToken | Returns `""` for non-Bearer auth headers |
| 25 | CORS default | Default origins are `localhost:5173,localhost:8080` |
| 26 | LogPage | Error handling shows console errors, CSS uses `var(--bg-card)` |
| 27 | Light theme | `--text-secondary` is `var(--n-600)` in light theme |
| 28 | ManagePage split | New component files exist |
| 29 | RouteRuleForm split | `ConditionEditor.tsx` exists |
| 30 | JWT parsing | `safeParseJWT` helper handles all edge cases |