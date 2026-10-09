package alerting

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTelegramPreviewUsesSubmittedSettingsWithoutSaving(t *testing.T) {
	s, id := setup(t)
	_, err := s.UpdateTelegram(0, TelegramUpdate{Token: ptr("123:global-token")})
	require.NoError(t, err)
	_, err = s.UpdateTelegram(id, TelegramUpdate{Enabled: true, ChatID: "-100123", Token: ptr("456:project-token")})
	require.NoError(t, err)
	before, err := s.Store.GetAlertChannel(id, Telegram)
	require.NoError(t, err)
	globalBefore, err := s.Store.GetAlertChannel(0, Telegram)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, wantToken, locale string
		scope                   int
		token                   *string
	}{
		{"saved project token", "456:project-token", "en", id, nil},
		{"inherit global", "123:global-token", "ru", id, ptr("")},
		{"unsaved override", "789:new-token", "ru-RU", id, ptr(" 789:new-token ")},
		{"saved global token", "123:global-token", "en", 0, nil},
		{"unsaved global token", "789:new-token", "en", 0, ptr("789:new-token")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "/bot"+tc.wantToken+"/sendMessage", r.URL.Path)
				require.Equal(t, "POST", r.Method)
				var body map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "@preview", body["chat_id"])
				if strings.HasPrefix(tc.locale, "ru") {
					require.Contains(t, body["text"], "тестовое оповещение")
				} else {
					require.Contains(t, body["text"], "test notification")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
			})}
			require.NoError(t, s.TestTelegram(context.Background(), tc.scope, TelegramUpdate{ChatID: " @preview ", Token: tc.token}, tc.locale, client))
			require.Equal(t, 1, calls)
			after, err := s.Store.GetAlertChannel(id, Telegram)
			require.NoError(t, err)
			require.Equal(t, before, after)
			globalAfter, err := s.Store.GetAlertChannel(0, Telegram)
			require.NoError(t, err)
			require.Equal(t, globalBefore, globalAfter)
		})
	}
}

func TestTelegramPreviewValidationAndSafeFailures(t *testing.T) {
	s, id := setup(t)
	calls := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New(r.URL.String())
	})}
	for _, update := range []TelegramUpdate{
		{ChatID: "", Token: ptr("123:private-token")},
		{ChatID: "@preview", Token: ptr("123:private/token")},
		{ChatID: "@preview"},
	} {
		err := s.TestTelegram(context.Background(), id, update, "en", client)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "private")
	}
	require.Zero(t, calls)
	err := s.TestTelegram(context.Background(), id, TelegramUpdate{ChatID: "@preview", Token: ptr("123:private-token")}, "en", client)
	require.EqualError(t, err, "Telegram request failed (network error or timeout)")
	require.Equal(t, 1, calls)
	ctx, cancel := context.WithCancel(context.Background())
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		require.ErrorIs(t, r.Context().Err(), context.Canceled)
		return nil, r.Context().Err()
	})
	cancel()
	require.Error(t, s.TestTelegram(ctx, id, TelegramUpdate{ChatID: "@preview", Token: ptr("123:private-token")}, "en", client))
}
