// Package notifier sends notifications to external services.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Discord は Discord webhook に対して content メッセージを送信します．
// http.Client は外から注入することでテスト時に置き換え可能です．
type Discord struct {
	client *http.Client
}

// NewDiscord は client を必ず受け取ります．nil を渡すとパニックします．
// (http.DefaultClient はタイムアウト無しでブロックし得るため意図的に許容しません)
func NewDiscord(client *http.Client) *Discord {
	if client == nil {
		panic("notifier: http.Client must not be nil")
	}
	return &Discord{client: client}
}

// Send は webhookURL に message を送信します．
// 4xx/5xx は err として返し，呼び出し側で記録/再試行を判断できるようにします．
func (d *Discord) Send(ctx context.Context, webhookURL, message string) error {
	payload, err := json.Marshal(map[string]string{"content": message})
	if err != nil {
		// content は string なので marshal が失敗することは現実的に無いが，
		// 形式上ハンドル
		return fmt.Errorf("payload marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Body を最大 512 byte 読んでエラー文に含める．Discord の rate limit
		// レスポンス等を切り分けやすくする
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
