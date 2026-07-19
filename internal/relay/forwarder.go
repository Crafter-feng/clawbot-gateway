package relay

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LocalRelay 本地文件系统中转适配器
type LocalRelay struct {
	BasePath string
}

func NewLocalRelay(basePath string) *LocalRelay {
	return &LocalRelay{BasePath: basePath}
}

func (r *LocalRelay) Name() string { return "local" }

func (r *LocalRelay) Forward(ctx context.Context, file *FileMessage) error {
	// 创建日期目录
	dir := filepath.Join(r.BasePath, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 保存文件
	filePath := filepath.Join(dir, file.FileName)
	if err := os.WriteFile(filePath, file.FileData, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
