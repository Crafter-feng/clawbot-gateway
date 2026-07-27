package bot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// ── iLink API 常量 ──

const (
	ILinkAppID          = "bot"
	appClientVersionVal = (2 << 16) | (2 << 8) | 0 // 131584
	qrPollDeadlineSecs  = 480
	maxQRRefreshCount   = 3
)

// ── 消息模型 ──

// MediaType 媒体类型常量
const (
	MediaTypeText  = 1
	MediaTypeImage = 2
	MediaTypeVoice = 3
	MediaTypeFile  = 4
	MediaTypeVideo = 5
)

// MessageItem 消息项（从 iLink item_list 解析）
type MessageItem struct {
	Type      int              `json:"type"`
	TextItem  *TextItem        `json:"text_item,omitempty"`
	ImageItem *ImageItem       `json:"image_item,omitempty"`
	VoiceItem *VoiceItem       `json:"voice_item,omitempty"`
	FileItem  *FileItem        `json:"file_item,omitempty"`
	VideoItem *VideoItem       `json:"video_item,omitempty"`
	RefMsg    *RefMessage      `json:"ref_msg,omitempty"`
}

type TextItem struct {
	Text string `json:"text"`
}

type ImageItem struct {
	Media     *CDNMedia `json:"media,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
	AESKey    string    `json:"aeskey,omitempty"`
	URL       string    `json:"url,omitempty"`
	MidSize   int64     `json:"mid_size,omitempty"`
}

type VoiceItem struct {
	Media   *CDNMedia `json:"media,omitempty"`
	Text    string    `json:"text,omitempty"`
	PlayTime int      `json:"playtime,omitempty"`
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

type VideoItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	VideoSize  int64     `json:"video_size,omitempty"`
	VideoMD5   string    `json:"video_md5,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
}

type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

// NormalizedMessage 标准化消息
type NormalizedMessage struct {
	MsgID        string          `json:"msg_id"`
	FromUser     string          `json:"from_user"`
	ToUser       string          `json:"to_user"`
	AccountID    string          `json:"account_id"`
	Type         int             `json:"type"`
	Content      string          `json:"content"`
	Items        []MessageItem   `json:"items,omitempty"` // 原始消息项
	Timestamp    int64           `json:"timestamp"`
	ContextToken string          `json:"context_token,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

func BuildBaseInfo() BaseInfo {
	return BaseInfo{
		ChannelVersion: "1.0.2",
	}
}

// ── 登录凭证 ──

type Credentials struct {
	Token       string `json:"token"`
	BaseURL     string `json:"base_url"`
	AccountID   string `json:"account_id"`
	UserID      string `json:"user_id"`
	AccountName string `json:"account_name,omitempty"`
	LoginAt     int64  `json:"login_at"`
}

type QRCodeResponse struct {
	Success bool   `json:"success"`
	Data    QRData `json:"data"`
}

type QRData struct {
	QRCodeURL string `json:"qrcode_url"`
	QRCode    string `json:"qrcode"`
}

type QRStatusResponse struct {
	Success bool         `json:"success"`
	Data    QRStatusData `json:"data"`
}

type QRStatusData struct {
	Status      string           `json:"status"` // wait, scaned, confirmed, expired
	Credentials *CredentialsData `json:"credentials,omitempty"`
	BaseURL     string           `json:"baseurl,omitempty"`
}

type CredentialsData struct {
	BotToken    string `json:"bot_token"`
	IlinkBotID  string `json:"ilink_bot_id"`
	IlinkUserID string `json:"ilink_user_id"`
}

// ── 获取消息 ──

type GetUpdatesReq struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      BaseInfo `json:"base_info"`
}

type GetUpdatesResp struct {
	Ret           int              `json:"ret"`
	Errcode       int              `json:"errcode,omitempty"`
	Errmsg        string           `json:"errmsg,omitempty"`
	Msgs          []RawMessageItem `json:"msgs"`
	GetUpdatesBuf string           `json:"get_updates_buf"`
}

type RawMessageItem struct {
	MsgID        json.Number           `json:"message_id"`
	FromUserid   string                `json:"from_user_id"`
	ToUserid     string                `json:"to_user_id"`
	MsgType      int                   `json:"msg_type"`
	ItemList     []RawMessageItem_Item `json:"item_list"`
	Timestamp    int64                 `json:"timestamp"`
	ContextToken string                `json:"context_token,omitempty"`
}

type RawMessageItem_Item struct {
	Type      int                    `json:"type"`
	TextItem  *RawMessageItem_TextItem `json:"text_item,omitempty"`
	ImageItem *RawMessageItem_TextItem `json:"image_item,omitempty"`
	VoiceItem *RawMessageItem_TextItem `json:"voice_item,omitempty"`
	FileItem  *RawMessageItem_TextItem `json:"file_item,omitempty"`
	VideoItem *RawMessageItem_TextItem `json:"video_item,omitempty"`
	RefMsg    *RawMessageItem_RefMsg   `json:"ref_msg,omitempty"`
}

type RawMessageItem_TextItem struct {
	Text string `json:"text"`
}

type RawMessageItem_RefMsg struct {
	Title       string              `json:"title,omitempty"`
	MessageItem *RawMessageItem_Item `json:"message_item,omitempty"`
}

// ── 消息解析 ──

// normalize 将原始 iLink 消息转换为标准化格式
func normalize(raw RawMessageItem) NormalizedMessage {
	content := ExtractText(raw.ItemList)
	items := convertItems(raw.ItemList)
	return NormalizedMessage{
		MsgID:        raw.MsgID.String(),
		FromUser:     raw.FromUserid,
		ToUser:       raw.ToUserid,
		Type:         raw.MsgType,
		Content:      content,
		Items:        items,
		Timestamp:    raw.Timestamp,
		ContextToken: raw.ContextToken,
	}
}

// convertItems 将 RawMessageItem_Item 转换为 MessageItem
func convertItems(rawItems []RawMessageItem_Item) []MessageItem {
	items := make([]MessageItem, 0, len(rawItems))
	for _, raw := range rawItems {
		item := MessageItem{Type: raw.Type}
		if raw.TextItem != nil {
			item.TextItem = &TextItem{Text: raw.TextItem.Text}
		}
		if raw.ImageItem != nil {
			item.ImageItem = &ImageItem{URL: raw.ImageItem.Text}
		}
		if raw.VoiceItem != nil {
			item.VoiceItem = &VoiceItem{Text: raw.VoiceItem.Text}
		}
		if raw.FileItem != nil {
			item.FileItem = &FileItem{FileName: raw.FileItem.Text}
		}
		if raw.VideoItem != nil {
			item.VideoItem = &VideoItem{VideoSize: 0}
		}
		if raw.RefMsg != nil {
			ref := &RefMessage{Title: raw.RefMsg.Title}
			if raw.RefMsg.MessageItem != nil {
				converted := convertItems([]RawMessageItem_Item{*raw.RefMsg.MessageItem})
				if len(converted) > 0 {
					ref.MessageItem = &converted[0]
				}
			}
			item.RefMsg = ref
		}
		items = append(items, item)
	}
	return items
}

// ExtractText 从 item_list 提取文本内容（与 Python _extract_text 逻辑对齐）
func ExtractText(items []RawMessageItem_Item) string {
	// 优先提取文本类型 (type=1)
	for _, item := range items {
		if item.Type == 1 && item.TextItem != nil {
			text := item.TextItem.Text
			// 处理引用消息
			if item.RefMsg != nil {
				refText := ""
				if item.RefMsg.MessageItem != nil {
					refText = ExtractText([]RawMessageItem_Item{*item.RefMsg.MessageItem})
				}
				title := item.RefMsg.Title
				if title != "" || refText != "" {
					prefix := ""
					if title != "" && refText != "" {
						prefix = "[引用: " + title + " | " + refText + "]\n"
					} else if title != "" {
						prefix = "[引用: " + title + "]\n"
					} else {
						prefix = "[引用: " + refText + "]\n"
					}
					text = prefix + text
				}
			}
			return strings.TrimSpace(text)
		}
	}
	// 回退：提取语音转文字 (type=3)
	for _, item := range items {
		if item.Type == 3 && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}
	// 回退：对于图片/文件/视频等非文本消息，返回占位描述
	for _, item := range items {
		switch item.Type {
		case 2: // image
			return "[图片]"
		case 4: // file
			if item.FileItem != nil && item.FileItem.Text != "" {
				return "[文件: " + item.FileItem.Text + "]"
			}
			return "[文件]"
		case 5: // video
			return "[视频]"
		}
	}
	return ""
}

// GenerateClientID 生成客户端 ID（格式：openclaw-weixin:<ms>-<rand>）
func GenerateClientID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "openclaw-weixin:" + time.Now().Format("20060102150405.000") + "-fallback"
	}
	return "openclaw-weixin:" + time.Now().Format("20060102150405.000") + "-" + hex.EncodeToString(b)
}
