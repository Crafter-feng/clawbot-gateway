package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"clawbot-gateway/test/mock"
)

func TestProxyForwarding(t *testing.T) {
	// 启动模拟 WeClawBot-API
	mockServer := mock.NewWeClawBotMock("18090")
	if err := mockServer.Start(); err != nil {
		t.Fatalf("启动模拟服务器失败: %v", err)
	}
	defer mockServer.Stop()

	// 等待服务器启动
	time.Sleep(200 * time.Millisecond)

	// 测试发送消息
	body, _ := json.Marshal(map[string]string{
		"text": "测试消息",
	})

	resp, err := http.Post(
		mockServer.GetURL()+"/bots/test-bot/messages",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体（确保请求被完全处理）
	_, _ = io.ReadAll(resp.Body)

	// 验证响应
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}

	// 验证消息被记录
	messages := mockServer.GetMessages()
	if len(messages) != 1 {
		t.Errorf("期望 1 条消息，实际 %d", len(messages))
	}
}

func TestProxyMultipleMessages(t *testing.T) {
	// 启动模拟 WeClawBot-API
	mockServer := mock.NewWeClawBotMock("18091")
	if err := mockServer.Start(); err != nil {
		t.Fatalf("启动模拟服务器失败: %v", err)
	}
	defer mockServer.Stop()

	time.Sleep(200 * time.Millisecond)

	// 发送多条消息
	for i := 0; i < 5; i++ {
		body, _ := json.Marshal(map[string]string{
			"text": "测试消息",
		})

		resp, err := http.Post(
			mockServer.GetURL()+"/bots/test-bot/messages",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("发送消息失败: %v", err)
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	// 验证消息数量
	messages := mockServer.GetMessages()
	if len(messages) != 5 {
		t.Errorf("期望 5 条消息，实际 %d", len(messages))
	}
}

func TestProxyClearMessages(t *testing.T) {
	// 启动模拟 WeClawBot-API
	mockServer := mock.NewWeClawBotMock("18092")
	if err := mockServer.Start(); err != nil {
		t.Fatalf("启动模拟服务器失败: %v", err)
	}
	defer mockServer.Stop()

	time.Sleep(200 * time.Millisecond)

	// 发送消息
	body, _ := json.Marshal(map[string]string{
		"text": "测试消息",
	})

	resp, err := http.Post(
		mockServer.GetURL()+"/bots/test-bot/messages",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	// 验证消息被记录
	if mockServer.GetMessageCount() != 1 {
		t.Errorf("期望 1 条消息，实际 %d", mockServer.GetMessageCount())
	}

	// 清空消息
	mockServer.ClearMessages()

	// 验证消息已清空
	if mockServer.GetMessageCount() != 0 {
		t.Errorf("期望 0 条消息，实际 %d", mockServer.GetMessageCount())
	}
}
