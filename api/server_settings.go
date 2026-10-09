package api

import (
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	log "github.com/sirupsen/logrus"
)

func serverSettings(w http.ResponseWriter, r *http.Request) {
	if !helpers.GetFromContext(r, "user").(*db.User).Admin {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	store := helpers.Store(r)
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		settings, err := server.GetServerSettings(store)
		if err != nil {
			writeServerSettingsError(w, err)
			return
		}
		helpers.WriteJSON(w, http.StatusOK, settings)
		return
	}
	var update struct {
		UseRemoteRunner *bool `json:"use_remote_runner"`
	}
	if !helpers.Bind(w, r, &update) {
		return
	}
	if update.UseRemoteRunner == nil {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "use_remote_runner must be a boolean"})
		return
	}
	settings := server.ServerSettings{UseRemoteRunner: *update.UseRemoteRunner}
	if err := server.SaveServerSettings(store, settings); err != nil {
		writeServerSettingsError(w, err)
		return
	}
	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:     audit.SystemSettingsUpdate,
		Metadata: audit.SettingsMetadata{Keys: []string{server.RemoteRunnersEnabledOption}},
	})
	helpers.WriteJSON(w, http.StatusOK, settings)
}

func writeServerSettingsError(w http.ResponseWriter, err error) {
	log.WithError(err).Error("Server settings operation failed")
	w.WriteHeader(http.StatusInternalServerError)
}
