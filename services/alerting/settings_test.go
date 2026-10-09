package alerting

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (Service, int) {
	t.Helper()
	oldConfig := util.Config
	store := sql.InitConfigCreateTestStore()
	t.Cleanup(func() { store.Close(); util.Config = oldConfig })
	util.Config.AccessKeyEncryption = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	project, err := store.CreateProject(db.Project{Name: "alerts"})
	require.NoError(t, err)
	return Service{Store: store}, project.ID
}
func ptr(v string) *string { return &v }

func TestSettingsSecretsAndInheritance(t *testing.T) {
	s, id := setup(t)
	defaults, err := s.GetTelegram(id)
	require.NoError(t, err)
	require.False(t, defaults.Enabled)
	require.False(t, defaults.HasToken)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "-100123"})
	require.ErrorContains(t, err, "Configure a global")
	global, err := s.UpdateTelegram(0, TelegramUpdate{Token: ptr("123:global-token")})
	require.NoError(t, err)
	require.True(t, global.HasToken)
	raw, err := s.Store.GetAlertChannel(0, Telegram)
	require.NoError(t, err)
	require.NotContains(t, raw.Secret, "global-token")
	require.NotEqual(t, base64.StdEncoding.EncodeToString([]byte("123:global-token")), raw.Secret)
	serialized, err := json.Marshal(raw)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), raw.Secret)
	settings, err := s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: " -100123 "})
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.True(t, settings.GlobalTokenConfigured)
	require.False(t, settings.HasToken)
	require.Equal(t, "-100123", settings.ChatID)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "@channel", Token: ptr("456:project-token")})
	require.NoError(t, err)
	before, _ := s.Store.GetAlertChannel(id, Telegram)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: false, ChatID: "@channel"})
	require.NoError(t, err)
	after, _ := s.Store.GetAlertChannel(id, Telegram)
	require.Equal(t, before.Secret, after.Secret, "omitted token must preserve ciphertext")
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "@channel", Token: ptr("")})
	require.NoError(t, err)
	after, _ = s.Store.GetAlertChannel(id, Telegram)
	require.Empty(t, after.Secret, "empty token explicitly returns to global")
	_, err = s.UpdateTelegram(0, TelegramUpdate{Token: ptr("")})
	require.NoError(t, err)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: false, ChatID: "@channel"})
	require.NoError(t, err, "disabling remains possible when the global token was removed")
}

func TestValidationDoesNotReplaceSavedSettings(t *testing.T) {
	s, id := setup(t)
	for _, token := range []string{"123:abc/evil", "123:abc?x=1", "not-a-token"} {
		_, err := s.UpdateTelegram(0, TelegramUpdate{Token: &token})
		require.Error(t, err)
	}
	_, err := s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "\"bad-chat", Token: ptr("456:project-token")})
	require.Error(t, err)
	settings, err := s.GetTelegram(id)
	require.NoError(t, err)
	require.False(t, settings.HasToken)
	require.False(t, settings.Enabled)
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDispatchUsesProjectStateAndLatestToken(t *testing.T) {
	s, id := setup(t)
	calls := 0
	wantToken := "123:global-token"
	wantText := "message with \"quotes\"\nand unicode: привет"
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https://api.telegram.org/bot"+wantToken+"/sendMessage", r.URL.String())
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "-100123", body["chat_id"])
		require.Equal(t, wantText, body["text"])
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	})}
	sent, err := s.SendTelegram(id, wantText, client)
	require.NoError(t, err)
	require.False(t, sent)
	require.Zero(t, calls)
	_, err = s.UpdateTelegram(0, TelegramUpdate{Token: &wantToken})
	require.NoError(t, err)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "-100123"})
	require.NoError(t, err)
	sent, err = s.SendTelegram(id, wantText, client)
	require.NoError(t, err)
	require.True(t, sent)
	wantToken = "456:project-token"
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "-100123", Token: &wantToken})
	require.NoError(t, err)
	sent, err = s.SendTelegram(id, wantText, client)
	require.NoError(t, err)
	require.True(t, sent)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "-100123", Token: ptr("")})
	require.NoError(t, err)
	wantToken = "789:rotated-global"
	_, err = s.UpdateTelegram(0, TelegramUpdate{Token: &wantToken})
	require.NoError(t, err)
	sent, err = s.SendTelegram(id, wantText, client)
	require.NoError(t, err)
	require.True(t, sent)
	require.Equal(t, 3, calls)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: false, ChatID: "-100123"})
	require.NoError(t, err)
	sent, err = s.SendTelegram(id, wantText, client)
	require.NoError(t, err)
	require.False(t, sent)
	require.Equal(t, 3, calls)
}

func TestDispatchFailureDoesNotLeakCredentials(t *testing.T) {
	s, id := setup(t)
	_, err := s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "123", Token: ptr("456:secret-token")})
	require.NoError(t, err)
	for _, tc := range []struct {
		name, body string
		code       int
		err        error
	}{
		{name: "network", err: errors.New("https://api.telegram.org/bot456:secret-token/sendMessage")},
		{name: "http", code: 401, body: `{"description":"456:secret-token"}`},
		{name: "api", code: 200, body: `{"ok":false,"description":"456:secret-token"}`},
		{name: "malformed", code: 200, body: `not json`},
		{name: "redirect", code: 302, body: `redirect`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{"Location": []string{"https://example.invalid"}}}, nil
			})}
			sent, err := s.SendTelegram(id, "test", client)
			require.False(t, sent)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret-token")
		})
	}
}

func TestRekeyIncludesGlobalAndProjectTokens(t *testing.T) {
	s, id := setup(t)
	for _, scope := range []int{0, id} {
		_, err := s.UpdateTelegram(scope, TelegramUpdate{Token: ptr("123:secret")})
		require.NoError(t, err)
	}
	old := util.Config.AccessKeyEncryption
	util.Config.AccessKeyEncryption = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	require.NoError(t, s.Rekey(old))
	for _, scope := range []int{0, id} {
		channel, err := s.Store.GetAlertChannel(scope, Telegram)
		require.NoError(t, err)
		secret, err := util.Config.DecryptAccessSecret(channel.Secret)
		require.NoError(t, err)
		require.Equal(t, "123:secret", string(secret))
	}
}
