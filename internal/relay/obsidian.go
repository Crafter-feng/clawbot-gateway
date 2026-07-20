package relay

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ObsidianRelay Obsidian 文件中转适配器
type ObsidianRelay struct {
	VaultPath   string
	InboxDir    string
	DailyFolder bool
}

func NewObsidianRelay(vaultPath, inboxDir string, dailyFolder bool) *ObsidianRelay {
	return &ObsidianRelay{
		VaultPath:   vaultPath,
		InboxDir:    inboxDir,
		DailyFolder: dailyFolder,
	}
}

func (r *ObsidianRelay) Name() string { return "obsidian" }

func (r *ObsidianRelay) Forward(ctx context.Context, file *FileMessage) error {
	// 1. 确定保存路径
	dir := filepath.Join(r.VaultPath, r.InboxDir)
	if r.DailyFolder {
		dir = filepath.Join(dir, time.Now().Format("2006-01-02"))
	}

	// 2. 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 3. 安全处理文件名（防止路径穿越）
	safeName := filepath.Base(file.FileName)
	if safeName == "." || safeName == "/" {
		safeName = "unknown_file"
	}

	// 4. 保存文件
	filePath := filepath.Join(dir, safeName)
	if err := os.WriteFile(filePath, file.FileData, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	// 5. 生成 Markdown 笔记（可选）
	if file.FileType == "image" || file.FileType == "file" {
		noteName := strings.TrimSuffix(safeName, filepath.Ext(safeName)) + ".md"
		notePath := filepath.Join(dir, noteName)
		note := fmt.Sprintf("---\ntype: %s\nfrom: %s\ndate: %s\n---\n\n# %s\n\n![](%s)\n",
			file.FileType,
			file.FromUser,
			file.Timestamp.Format("2006-01-02 15:04:05"),
			safeName,
			safeName,
		)
		if err := os.WriteFile(notePath, []byte(note), 0644); err != nil {
			return fmt.Errorf("write note: %w", err)
		}
	}

	return nil
}
