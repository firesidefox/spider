package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// dingTalkSender sends messages to a DingTalk group robot webhook.
type dingTalkSender struct {
	webhookURL string
	secret     string // optional signing secret
}

type dingTalkPayload struct {
	MsgType string          `json:"msgtype"`
	Text    dingTalkText    `json:"text"`
}

type dingTalkText struct {
	Content string `json:"content"`
}

func (d *dingTalkSender) Send(ctx context.Context, msg string) error {
	url := d.webhookURL
	if d.secret != "" {
		url = d.signedURL(url)
	}

	payload := dingTalkPayload{
		MsgType: "text",
		Text:    dingTalkText{Content: msg},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// signedURL appends timestamp + sign query params per DingTalk signing spec.
func (d *dingTalkSender) signedURL(base string) string {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	strToSign := ts + "\n" + d.secret
	mac := hmac.New(sha256.New, []byte(d.secret))
	mac.Write([]byte(strToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s&timestamp=%s&sign=%s", base, ts, sign)
}
