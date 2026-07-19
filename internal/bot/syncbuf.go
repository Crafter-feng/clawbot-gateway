package bot

// SyncBufStore 同步缓冲区存储接口（避免循环依赖）
type SyncBufStore interface {
	GetSyncBuf(accountID string) string
	SetSyncBuf(accountID, buf string) error
}
