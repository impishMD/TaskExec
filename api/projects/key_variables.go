package projects

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
)

var expressionFieldName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DescribeKeyValue supplies autocomplete paths without exposing secret values.
func DescribeKeyValue(encryption server.AccessKeyEncryptionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		key := helpers.GetFromContext(r, "accessKey").(db.AccessKey)
		value, err := server.ReadVariableKey(helpers.Store(r), encryption, *key.ProjectID, key.ID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		paths := []string{""}
		var walk func(any, string, int)
		walk = func(value any, path string, depth int) {
			if depth >= 32 || len(paths) >= 1000 {
				return
			}
			switch v := value.(type) {
			case map[string]any:
				names := make([]string, 0, len(v))
				for name := range v {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					if len(paths) >= 1000 {
						break
					}
					part := "." + name
					if !expressionFieldName.MatchString(name) {
						encoded, _ := json.Marshal(name)
						part = "[" + string(encoded) + "]"
					}
					paths = append(paths, path+part)
					walk(v[name], path+part, depth+1)
				}
			case []any:
				for i, child := range v {
					if len(paths) >= 1000 {
						break
					}
					part := path + "[" + strconv.Itoa(i) + "]"
					paths = append(paths, part)
					walk(child, part, depth+1)
				}
			}
		}
		walk(value, "", 0)
		helpers.WriteJSON(w, http.StatusOK, map[string]any{"paths": paths})
	}
}

func PreviewKeyValue(encryption server.AccessKeyEncryptionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		key := helpers.GetFromContext(r, "accessKey").(db.AccessKey)
		value, err := server.ReadVariableKey(helpers.Store(r), encryption, *key.ProjectID, key.ID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		helpers.WriteJSON(w, http.StatusOK, map[string]any{"value": value, "type": key.Type, "read_at": time.Now().UTC()})
	}
}

func (c *SecretStorageController) DescribeSecret(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	storage := helpers.GetFromContext(r, "secretStorage").(db.SecretStorage)
	var body struct {
		Path string `json:"path"`
	}
	if !helpers.Bind(w, r, &body) {
		return
	}
	fields, err := c.secretStorageService.DescribeVaultSecret(r.Context(), storage.ProjectID, storage.ID, body.Path)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"fields": fields})
}

func (c *SecretStorageController) ListSecretPaths(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	storage := helpers.GetFromContext(r, "secretStorage").(db.SecretStorage)
	var body struct {
		Path string `json:"path"`
	}
	if !helpers.Bind(w, r, &body) {
		return
	}
	paths, err := c.secretStorageService.ListVaultSecrets(r.Context(), storage.ProjectID, storage.ID, body.Path)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, map[string]any{"paths": paths})
}
