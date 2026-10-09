package alerting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/impishMD/taskexec/util"
)

// SendTelegram resolves current database settings for each notification. A disabled
// project never sends, and a project token takes precedence over the global token.
// Errors must not include Telegram's request URL (it contains the bot token).
func (s Service) SendTelegram(projectID int, text string, client *http.Client) (bool, error) {
	if projectID <= 0 {
		return false, errors.New("Telegram alerts require a project")
	}
	channel, err := s.Store.GetAlertChannel(projectID, Telegram)
	if err != nil {
		return false, errors.New("could not load Telegram settings")
	}
	if !channel.Enabled {
		return false, nil
	}
	var config telegramConfig
	if err = json.Unmarshal([]byte(channel.Settings), &config); err != nil || config.ChatID == "" {
		return false, errors.New("Telegram chat is not configured")
	}
	secret := channel.Secret
	if secret == "" {
		global, err := s.Store.GetAlertChannel(0, Telegram)
		if err != nil {
			return false, errors.New("could not load global Telegram settings")
		}
		secret = global.Secret
	}
	if secret == "" {
		return false, errors.New("Telegram bot token is not configured")
	}
	token, err := util.Config.DecryptAccessSecret(secret)
	if err != nil {
		return false, errors.New("could not decrypt Telegram bot token")
	}
	if !tokenPattern.Match(token) {
		return false, errors.New("invalid stored Telegram bot token")
	}
	body, err := json.Marshal(map[string]string{"chat_id": config.ChatID, "parse_mode": "HTML", "text": text})
	if err != nil {
		return false, errors.New("could not encode Telegram alert")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// Never forward a credential-bearing path through a redirect.
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safeClient.Post("https://api.telegram.org/bot"+string(token)+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return false, errors.New("Telegram request failed (network error or timeout)")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Telegram request failed (HTTP %d)", resp.StatusCode)
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil || !result.OK {
		return false, errors.New("Telegram did not accept the alert")
	}
	return true, nil
}
