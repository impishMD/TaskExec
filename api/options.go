package api

import (
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
)

func setOption(w http.ResponseWriter, r *http.Request) {
	currentUser := helpers.GetFromContext(r, "user").(*db.User)

	if !currentUser.Admin {
		helpers.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error": "User must be admin",
		})
		return
	}

	var option db.Option
	if !helpers.Bind(w, r, &option) {
		return
	}

	if option.Key == server.RemoteRunnersEnabledOption && option.Value != "true" && option.Value != "false" {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "remote runner setting must be true or false"})
		return
	}
	err := helpers.Store(r).SetOption(option.Key, option.Value)
	if err != nil {
		helpers.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Can not set option",
		})
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:     audit.SystemSettingsUpdate,
		Metadata: audit.SettingsMetadata{Keys: []string{audit.TruncateName(option.Key, audit.MaxNameBytes)}},
	})

	helpers.WriteJSON(w, http.StatusOK, option)
}

func getOptions(w http.ResponseWriter, r *http.Request) {
	currentUser := helpers.GetFromContext(r, "user").(*db.User)

	if !currentUser.Admin {
		helpers.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error": "User must be admin",
		})
		return
	}

	options, err := helpers.Store(r).GetOptions(db.RetrieveQueryParams{})
	if err != nil {
		helpers.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Can not get options",
		})
		return
	}

	helpers.WriteJSON(w, http.StatusOK, options)
}
