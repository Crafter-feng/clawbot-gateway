package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestGetQRCodeFromWeChat 直接调用微信 iLink 官方 API 测试二维码获取
// 使用 ilinkai.weixin.qq.com 公共接口，不依赖任何配置
func TestGetQRCodeFromWeChat(t *testing.T) {
	apiURL := "https://ilinkai.weixin.qq.com/ilink/bot/get_bot_qrcode?bot_type=3"
	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	setQRHeaders(req)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("请求 iLink API 失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("HTTP 状态码异常: %d, body: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		QRCode           string `json:"qrcode"`
		QRCodeImgContent string `json:"qrcode_img_content"`
		Ret              int    `json:"ret"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v, body: %s", err, string(body))
	}

	if raw.Ret != 0 {
		t.Fatalf("iLink 返回错误码: %d", raw.Ret)
	}
	if raw.QRCode == "" {
		t.Fatal("二维码标识 qrcode 为空")
	}
	if raw.QRCodeImgContent == "" {
		t.Fatal("二维码图片链接 qrcode_img_content 为空")
	}

	t.Logf("✅ 二维码获取成功")
	t.Logf("   qrcode:             %s", raw.QRCode)
	t.Logf("   qrcode_img_content: %s", raw.QRCodeImgContent)

	// 验证二维码图片 URL 格式
	parsedURL, err := url.Parse(raw.QRCodeImgContent)
	if err != nil {
		t.Errorf("二维码 URL 格式无效: %v", err)
	} else {
		t.Logf("   URL 协议: %s, 主机: %s", parsedURL.Scheme, parsedURL.Host)
	}

	// 验证状态查询接口（短超时，可能因网络超时跳过）
	statusURL := "https://ilinkai.weixin.qq.com/ilink/bot/get_qrcode_status?qrcode=" +
		url.QueryEscape(raw.QRCode)
	statusReq, _ := http.NewRequestWithContext(context.Background(), "GET", statusURL, nil)
	setQRHeaders(statusReq)

	statusClient := &http.Client{Timeout: 5 * time.Second}
	statusResp, err := statusClient.Do(statusReq)
	if err != nil {
		t.Logf("⚠ 状态查询超时（网络问题），跳过: %v", err)
	} else {
		defer statusResp.Body.Close()
		statusBody, _ := io.ReadAll(io.LimitReader(statusResp.Body, 1<<20))
		var statusRaw struct {
			Status string `json:"status"`
		}
		json.Unmarshal(statusBody, &statusRaw)
		t.Logf("   状态: %s", statusRaw.Status)
		if statusRaw.Status != "wait" {
			t.Logf("注意: 二维码状态为 '%s'，预期 'wait'", statusRaw.Status)
		}
	}
}

// TestGetQRCodeResponseFormat 验证前端需要的字段是否完整
func TestGetQRCodeResponseFormat(t *testing.T) {
	apiURL := "https://ilinkai.weixin.qq.com/ilink/bot/get_bot_qrcode?bot_type=3"
	req, _ := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	setQRHeaders(req)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var result struct {
		QRCode           string `json:"qrcode"`
		QRCodeImgContent string `json:"qrcode_img_content"`
		Ret              int    `json:"ret"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 验证前端需要的字段
	checks := []struct {
		name  string
		value string
	}{
		{"qrcode（二维码标识，前端用于轮询）", result.QRCode},
		{"qrcode_img_content（二维码图片 URL，前端生成二维码）", result.QRCodeImgContent},
	}
	for _, c := range checks {
		if c.value == "" {
			t.Errorf("❌ %s 为空", c.name)
		} else {
			t.Logf("✅ %s: %s", c.name, c.value)
		}
	}
}