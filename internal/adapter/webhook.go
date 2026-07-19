package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookAdapter Webhook 通知适配器（单向推送，不进行交互）
type WebhookAdapter struct {
	id      string
	name    string
	url     string
	headers map[string]string
	client  *http.Client
}

// WebhookMessage Webhook 消息格式
type WebhookMessage struct {
	FromUser string            `json:"from_user"`
	Content  string            `json:"content"`
	Type     string            `json:"type"` // "text", "image", "file", "audio"
	Time     time.Time         `json:"time"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func NewWebhookAdapter(id, name, url string, headers map[string]string) *WebhookAdapter {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &WebhookAdapter{
		id:      id,
		name:    name,
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (w *WebhookAdapter) ID() string   { return w.id }
func (w *WebhookAdapter) Name() string { return w.name }
func (w *WebhookAdapter) Type() string { return "webhook" }

func (w *WebhookAdapter) HealthCheck(ctx context.Context) bool {
	// Webhook 不支持健康检查，始终返回 true
	return true
}

// Handle 发送消息到 Webhook URL（单向推送）
func (w *WebhookAdapter) Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	msg := WebhookMessage{
		FromUser: req.UserID,
		Content:  req.Message,
		Type:     "text",
		Time:     time.Now(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal webhook message: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create webhook request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := w.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return &ChatResponse{
		Text:    fmt.Sprintf("Webhook 通知已发送 (HTTP %d)", resp.StatusCode),
		Backend: w.id,
	}, nil
}

// HandleStream Webhook 不支持 streaming
func (w *WebhookAdapter) HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error {
	defer close(ch)
	resp, err := w.Handle(ctx, req)
	if err != nil {
		return err
	}
	ch <- resp.Text
	return nil
}
