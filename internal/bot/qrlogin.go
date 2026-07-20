package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"clawbot-gateway/internal/log"
)

// ── 扫码登录 ──

// QRCodeManager 管理 QR 扫码状态轮询
type QRCodeManager struct {
	connector *Connector
	mu        sync.Mutex
	active    map[string]*QRScanState
}

type QRScanState struct {
	QRCode       string
	Cancel       context.CancelFunc
	Status       string // "wait", "scaned", "scaned_but_redirect", "confirmed", "expired"
	Err          error
	Creds        *Credentials
	UpdatedAt    time.Time
	RedirectHost string // scaned_but_redirect 时 iLink 返回的重定向域名
}

func NewQRCodeManager(conn *Connector) *QRCodeManager {
	return &QRCodeManager{
		connector: conn,
		active:    make(map[string]*QRScanState),
	}
}

func (qm *QRCodeManager) log() *log.Logger {
	return qm.connector.log.WithComponent("qr")
}

func (qm *QRCodeManager) CreateScan(ctx context.Context, qrcode string) error {
	qm.mu.Lock()
	if _, exists := qm.active[qrcode]; exists {
		qm.mu.Unlock()
		return nil // already polling
	}
	ctx, cancel := context.WithCancel(ctx)
	state := &QRScanState{
		QRCode: qrcode,
		Status: "wait",
		Cancel: cancel,
	}
	qm.active[qrcode] = state
	qm.mu.Unlock()

	go func() {
		log.Default().Info("QR polling goroutine started", "qrcode", qrcode)
		log.Default().Info("QR polling baseURL", "baseURL", qm.connector.baseURL)
		defer func() {
			qm.mu.Lock()
			// confirmed 状态保留在 active 中，前端需要通过 CheckStatus 读取凭证
			if state.Status != "confirmed" {
				delete(qm.active, qrcode)
			}
			qm.mu.Unlock()
			cancel()
		}()

		currentBaseURL := qm.connector.baseURL
		refreshCount := 0
		startTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				log.Default().Info("QR polling context cancelled", "qrcode", qrcode)
				return
			default:
			}

			// 480s 总超时
			if time.Since(startTime) > qrPollDeadlineSecs*time.Second {
				qm.mu.Lock()
				state.Status = "expired"
				state.UpdatedAt = time.Now()
				qm.mu.Unlock()
				return
			}

			url := fmt.Sprintf("%s/ilink/bot/get_qrcode_status?qrcode=%s", currentBaseURL, url.QueryEscape(qrcode))
			log.Default().Info("QR polling making request", "url", url)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				qm.mu.Lock()
				state.Err = err
				state.Status = "expired"
				state.UpdatedAt = time.Now()
				qm.mu.Unlock()
				return
			}
			setQRHeaders(req)

			resp, err := qm.connector.client.Do(req)
			if err != nil {
				qm.log().Warn("poll status error", "error", err)
				time.Sleep(2 * time.Second)
				continue
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 限制 1MB
			resp.Body.Close()

			qm.log().Info("poll status response", "status_code", resp.StatusCode, "body", string(body))
			status, creds, redirectHost := parseQRStatusResponse(body, qm.connector.baseURL)
			qm.log().Info("parsed status", "status", status)

			qm.mu.Lock()
			state.Status = status
			state.UpdatedAt = time.Now()
			if creds != nil {
				state.Creds = creds
				qm.connector.token = creds.Token
			}
			if redirectHost != "" {
				state.RedirectHost = redirectHost
			}
			qm.mu.Unlock()

			switch status {
			case "confirmed":
				return
			case "scaned_but_redirect":
				if redirectHost != "" {
					currentBaseURL = fmt.Sprintf("https://%s", redirectHost)
				}
				time.Sleep(1 * time.Second)
			case "expired":
				refreshCount++
				if refreshCount > maxQRRefreshCount {
					return
				}
				// 自动刷新二维码
				newURL := fmt.Sprintf("%s/ilink/bot/get_bot_qrcode?bot_type=%d", qm.connector.baseURL, qm.connector.botType)
				newReq, err := http.NewRequestWithContext(ctx, "GET", newURL, nil)
				if err != nil {
					return
				}
				setQRHeaders(newReq)
				newResp, err := qm.connector.client.Do(newReq)
				if err != nil {
					return
				}
				newBody, _ := io.ReadAll(io.LimitReader(newResp.Body, 1<<20))
				newResp.Body.Close()

				var raw struct {
					QRCode           string `json:"qrcode"`
					QRCodeImgContent string `json:"qrcode_img_content"`
				}
				if err := json.Unmarshal(newBody, &raw); err == nil && raw.QRCode != "" {
					oldQRCode := qrcode
					qrcode = raw.QRCode
					// 清理旧状态，用新 qrcode 注册
					qm.mu.Lock()
					qm.active[raw.QRCode] = state
					delete(qm.active, oldQRCode)
					qm.mu.Unlock()
				}
				currentBaseURL = qm.connector.baseURL // 重置 base URL
				continue
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()

	return nil
}

func (qm *QRCodeManager) CheckStatus(qrcode string) *QRScanState {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	state := qm.active[qrcode]
	if state == nil {
		return &QRScanState{Status: "unknown"}
	}
	// return a copy
	return &QRScanState{
		QRCode:       state.QRCode,
		Status:       state.Status,
		Err:          state.Err,
		Creds:        state.Creds,
		UpdatedAt:    state.UpdatedAt,
		RedirectHost: state.RedirectHost,
	}
}

func (qm *QRCodeManager) StopScan(qrcode string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if state, ok := qm.active[qrcode]; ok {
		if state.Cancel != nil {
			state.Cancel()
		}
		delete(qm.active, qrcode)
	}
}

func parseQRStatusResponse(body []byte, defaultBaseURL string) (status string, creds *Credentials, redirectHost string) {
	var rawStatus struct {
		Status       string `json:"status"`
		BotToken     string `json:"bot_token,omitempty"`
		IlinkBotID   string `json:"ilink_bot_id,omitempty"`
		IlinkUserID  string `json:"ilink_user_id,omitempty"`
		BaseURL      string `json:"baseurl,omitempty"`
		RedirectHost string `json:"redirect_host,omitempty"`
	}
	if err := json.Unmarshal(body, &rawStatus); err != nil {
		var qrStatus QRStatusResponse
		if err2 := json.Unmarshal(body, &qrStatus); err2 != nil {
			return "wait", nil, ""
		}
		if !qrStatus.Success {
			return "wait", nil, ""
		}
		rawStatus.Status = qrStatus.Data.Status
		if qrStatus.Data.Credentials != nil {
			rawStatus.BotToken = qrStatus.Data.Credentials.BotToken
			rawStatus.IlinkBotID = qrStatus.Data.Credentials.IlinkBotID
			rawStatus.IlinkUserID = qrStatus.Data.Credentials.IlinkUserID
			rawStatus.BaseURL = qrStatus.Data.BaseURL
		}
	}

	if rawStatus.Status == "confirmed" {
		baseURL := defaultBaseURL
		if rawStatus.BaseURL != "" {
			baseURL = rawStatus.BaseURL
		}
		creds := &Credentials{
			Token:     rawStatus.BotToken,
			BaseURL:   baseURL,
			AccountID: rawStatus.IlinkBotID,
			UserID:    rawStatus.IlinkUserID,
			LoginAt:   time.Now().Unix(),
		}
		return "confirmed", creds, ""
	}
	return rawStatus.Status, nil, rawStatus.RedirectHost
}

// setQRHeaders 设置 iLink QR API 请求头
func setQRHeaders(req *http.Request) {
	uin := randomUIN()
	req.Header.Set("iLink-App-Id", ILinkAppID)
	req.Header.Set("iLink-App-ClientVersion", fmt.Sprintf("%d", appClientVersionVal))
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", uin)
}

// GetQRCode 获取二维码（非阻塞）
func (c *Connector) GetQRCode(ctx context.Context) (*QRData, error) {
	url := fmt.Sprintf("%s/ilink/bot/get_bot_qrcode?bot_type=%d", c.baseURL, c.botType)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	setQRHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get qrcode request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var raw struct {
		QRCode           string `json:"qrcode"`
		QRCodeImgContent string `json:"qrcode_img_content"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.QRCode == "" {
		var qrResp QRCodeResponse
		if err2 := json.Unmarshal(body, &qrResp); err2 != nil || !qrResp.Success {
			return nil, fmt.Errorf("parse qrcode response failed: %w, body: %s", err, string(body))
		}
		return &qrResp.Data, nil
	}

	return &QRData{
		QRCodeURL: raw.QRCodeImgContent,
		QRCode:    raw.QRCode,
	}, nil
}
