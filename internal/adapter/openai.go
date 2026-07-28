package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"clawbot-gateway/internal/database"
)


var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}
// ── OpenAI 兼容适配器 ──

type OpenAICompatibleAdapter struct {
	id      string
	name    string
	apiKey  string
	baseURL string
	model   string
}

func NewOpenAICompatibleAdapter(id, name, apiKey, baseURL, model string) *OpenAICompatibleAdapter {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OpenAICompatibleAdapter{
		id:      id,
		name:    name,
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}

func (o *OpenAICompatibleAdapter) ID() string   { return o.id }
func (o *OpenAICompatibleAdapter) Name() string { return o.name }
func (o *OpenAICompatibleAdapter) Type() string { return "openai_compatible" }

func (o *OpenAICompatibleAdapter) HealthCheck(ctx context.Context) bool {
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(healthCtx, "GET", o.baseURL+"/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain body
	return resp.StatusCode == 200
}

func sanitizeErrorBody(body string) string {
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	// Remove non-printable characters that could break message format
	body = strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 {
			return r
		}
		return -1
	}, body)
	return body
}

func (o *OpenAICompatibleAdapter) Handle(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	messages := o.buildMessages(req)
	payload := map[string]interface{}{
		"model":    o.model,
		"messages": messages,
		"stream":   false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	const maxRetries = 2
	var resp *http.Response
	var httpErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

		resp, httpErr = httpClient.Do(httpReq)
		if httpErr != nil {
			return nil, fmt.Errorf("api call: %w", httpErr)
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt < maxRetries {
				resp.Body.Close()
				continue
			}
		}
		break
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		bodyStr := sanitizeErrorBody(string(respBody))
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, bodyStr)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ChatResponse{
		Text:    strings.TrimSpace(result.Choices[0].Message.Content),
		Backend: o.id,
	}, nil
}

func (o *OpenAICompatibleAdapter) HandleStream(ctx context.Context, req *ChatRequest, ch chan<- string) error {
	defer close(ch)

	messages := o.buildMessages(req)
	payload := map[string]interface{}{
		"model":    o.model,
		"messages": messages,
		"stream":   true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("stream api call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		bodyStr := sanitizeErrorBody(string(respBody))
		return fmt.Errorf("stream api error %d: %s", resp.StatusCode, bodyStr)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		resp.Body.Close()
		return fmt.Errorf("unexpected content type: %s", contentType)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 4MB max line
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				select {
				case ch <- content:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
	return scanner.Err()
}

func (o *OpenAICompatibleAdapter) buildMessages(req *ChatRequest) []map[string]string {
	messages := make([]map[string]string, 0)
	if len(req.History) > 0 {
		for _, h := range req.History {
			messages = append(messages, map[string]string{
				"role":    h.Role,
				"content": h.Content,
			})
		}
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": req.Message,
	})
	return messages
}

// 自注册
func init() {
	RegisterAdapter("openai_compatible", func(b database.Backend) BackendAdapter {
		apiKey := GetJSONString(b.Config, "api_key")
		baseURL := GetJSONString(b.Config, "base_url")
		model := GetJSONString(b.Config, "model")
		if model == "" {
			model = "gpt-4o"
		}
		return NewOpenAICompatibleAdapter(b.ID, b.Name, apiKey, baseURL, model)
	})
}
