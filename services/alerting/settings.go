package alerting

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/util"
)

const Telegram = "telegram"

// TelegramSettings is the public, secret-free representation of a channel.
type TelegramSettings struct {
	Channel               string `json:"channel"`
	Enabled               bool   `json:"enabled"`
	ChatID                string `json:"chat_id"`
	HasToken              bool   `json:"has_token"`
	GlobalTokenConfigured bool   `json:"global_token_configured"`
}

type TelegramUpdate struct {
	Enabled bool   `json:"enabled"`
	ChatID  string `json:"chat_id"`
	// Omitted/null preserves the secret; an empty string explicitly removes it.
	Token *string `json:"token"`
}

type telegramConfig struct {
	ChatID string `json:"chat_id"`
}

type Service struct{ Store db.AlertChannelRepository }

func (s Service) GetTelegram(projectID int) (TelegramSettings, error) {
	c, err := s.Store.GetAlertChannel(projectID, Telegram)
	if err != nil {
		return TelegramSettings{}, err
	}
	var config telegramConfig
	if err = json.Unmarshal([]byte(c.Settings), &config); err != nil {
		return TelegramSettings{}, err
	}
	result := TelegramSettings{Channel: Telegram, Enabled: c.Enabled, ChatID: config.ChatID, HasToken: c.Secret != ""}
	if projectID != 0 {
		global, err := s.Store.GetAlertChannel(0, Telegram)
		if err != nil {
			return result, err
		}
		result.GlobalTokenConfigured = global.Secret != ""
	}
	return result, nil
}

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
var chatPattern = regexp.MustCompile(`^(-?[1-9][0-9]*|@[A-Za-z][A-Za-z0-9_]{4,})$`)

func (s Service) UpdateTelegram(projectID int, update TelegramUpdate) (TelegramSettings, error) {
	c, err := s.Store.GetAlertChannel(projectID, Telegram)
	if err != nil {
		return TelegramSettings{}, err
	}
	chatID := strings.TrimSpace(update.ChatID)
	if projectID == 0 {
		chatID = ""
	}
	if update.Token != nil {
		token := strings.TrimSpace(*update.Token)
		if token == "" {
			c.Secret = ""
		} else {
			if len(token) > 256 || !tokenPattern.MatchString(token) {
				return TelegramSettings{}, common_errors.NewValidationError("Invalid Telegram bot token")
			}
			c.Secret, err = util.Config.EncryptAccessSecret([]byte(token))
			if err != nil {
				return TelegramSettings{}, errors.New("could not protect Telegram bot token")
			}
		}
	}
	if projectID != 0 && update.Enabled {
		if len(chatID) > 128 || !chatPattern.MatchString(chatID) {
			return TelegramSettings{}, common_errors.NewValidationError("Enter a Telegram chat ID or channel @username")
		}
		if c.Secret == "" {
			global, err := s.Store.GetAlertChannel(0, Telegram)
			if err != nil {
				return TelegramSettings{}, err
			}
			if global.Secret == "" {
				return TelegramSettings{}, common_errors.NewValidationError("Configure a global Telegram token or provide a project token")
			}
		}
	}
	if len(chatID) > 128 {
		return TelegramSettings{}, common_errors.NewValidationError("Telegram chat ID is too long")
	}
	config, _ := json.Marshal(telegramConfig{ChatID: chatID})
	c.Settings = string(config)
	c.Enabled = projectID != 0 && update.Enabled
	if err = s.Store.SetAlertChannel(c); err != nil {
		return TelegramSettings{}, err
	}
	return s.GetTelegram(projectID)
}

// Rekey includes both global credentials and per-project overrides in vault rekey.
func (s Service) Rekey(oldKey string) error {
	channels, err := s.Store.GetAlertChannelsWithSecrets()
	if err != nil {
		return err
	}
	for _, c := range channels {
		var secret []byte
		if oldKey == "" {
			secret, err = util.Config.DecryptAccessSecret(c.Secret)
		} else {
			secret, err = util.Config.DecryptAccessSecretWithKey(c.Secret, oldKey)
		}
		if err != nil {
			return errors.New("could not decrypt alert channel credentials")
		}
		c.Secret, err = util.Config.EncryptAccessSecret(secret)
		if err != nil {
			return errors.New("could not encrypt alert channel credentials")
		}
		if err = s.Store.SetAlertChannel(c); err != nil {
			return err
		}
	}
	return nil
}
