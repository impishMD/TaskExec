package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerSettings_AdminOnlyAndDatabaseBacked(t *testing.T) {
	f := newCoreFixture(t)
	util.Config.Mfa.Email = &util.EmailAuthConfig{}
	util.Config.Schedule = &util.ScheduleConfig{}
	util.Config.JWT = &util.JWTConfig{}
	// The settings endpoint must not expose other option values.
	require.NoError(t, f.store.SetOption("unrelated.option", "private-value"))
	for _, actor := range []string{"owner", "manager", "guest", "outsider", ""} {
		status := 403
		if actor == "" {
			status = 401
		}
		for _, method := range []string{"GET", "HEAD", "PUT"} {
			f.request(t, actor, method, "/settings", `{"use_remote_runner":true}`, status)
		}
	}
	w := f.request(t, "admin", "GET", "/settings", "", 200)
	assert.JSONEq(t, `{"use_remote_runner":false}`, w.Body.String())
	for _, value := range []string{"true", "false"} {
		body := `{"use_remote_runner":` + value + `}`
		w = f.request(t, "admin", "PUT", "/settings", body, 200)
		assert.JSONEq(t, body, w.Body.String())
		stored, err := f.store.GetOption(server.RemoteRunnersEnabledOption)
		require.NoError(t, err)
		assert.Equal(t, value, stored)
		w = f.request(t, "admin", "GET", "/settings", "", 200)
		assert.JSONEq(t, body, w.Body.String())

		w = f.request(t, "guest", "GET", "/info", "", 200)
		var info struct {
			UseRemoteRunner bool `json:"use_remote_runner"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &info))
		assert.Equal(t, value == "true", info.UseRemoteRunner)
		w = f.request(t, "admin", "GET", "/admin/info", "", 200)
		var adminInfo struct {
			Runners server.ServerSettings `json:"runners"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &adminInfo))
		assert.Equal(t, info.UseRemoteRunner, adminInfo.Runners.UseRemoteRunner)
	}
}

func TestServerSettings_InvalidUpdatesPreserveSavedValue(t *testing.T) {
	f := newCoreFixture(t)
	f.request(t, "admin", "PUT", "/settings", `{"use_remote_runner":true}`, 200)
	for _, body := range []string{"", "{", "{}", "null", `{"use_remote_runner":null}`,
		`{"use_remote_runner":"false"}`, `{"use_remote_runner":0}`} {
		f.request(t, "admin", "PUT", "/settings", body, 400)
	}
	f.request(t, "admin", "POST", "/options", `{"key":"`+server.RemoteRunnersEnabledOption+`","value":"invalid"}`, 400)
	w := f.request(t, "admin", "GET", "/settings", "", 200)
	assert.JSONEq(t, `{"use_remote_runner":true}`, w.Body.String())
	// A malformed value must not be reported as a successful disabled state.
	require.NoError(t, f.store.SetOption(server.RemoteRunnersEnabledOption, "invalid"))
	f.request(t, "admin", "GET", "/settings", "", 500)
}

type failingServerSettingsStore struct{ db.Store }

func (failingServerSettingsStore) SetOption(string, string) error { return errors.New("write failed") }

func TestServerSettings_AuditOnlySuccessfulSave(t *testing.T) {
	store := setupSessionTest(t)
	admin := makeAdmin(t, store, createUserOptionsTestUser(t, store, "admin"))
	r, rec := userRequest(store, "PUT", "/api/settings", `{"use_remote_runner":true}`, admin, db.User{})
	w := httptest.NewRecorder()
	serverSettings(w, r)
	require.Equal(t, 200, w.Code)
	assert.Equal(t, audit.SettingsMetadata{Keys: []string{server.RemoteRunnersEnabledOption}},
		onlyEvent(t, rec, audit.SystemSettingsUpdate).Event.Metadata)

	r, rec = userRequest(failingServerSettingsStore{store}, "PUT", "/api/settings",
		`{"use_remote_runner":false}`, admin, db.User{})
	w = httptest.NewRecorder()
	serverSettings(w, r)
	require.Equal(t, 500, w.Code)
	assert.Empty(t, rec.All())
	settings, err := server.GetServerSettings(store)
	require.NoError(t, err)
	assert.True(t, settings.UseRemoteRunner)
}
