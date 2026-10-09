package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/services/tasks"
)

type TerraformController struct {
	encryption server.AccessKeyEncryptionService
	repo       db.TerraformStore
	store      db.Store
	pool       *tasks.TaskPool
}

func NewTerraformController(enc server.AccessKeyEncryptionService, repo db.TerraformStore, store db.Store, pool *tasks.TaskPool) *TerraformController {
	return &TerraformController{enc, repo, store, pool}
}
func equalTerraformCredential(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}
func (c *TerraformController) TerraformInventoryAliasMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		name := mux.Vars(r)["alias"]
		var alias db.TerraformInventoryAlias
		var err error
		if c.pool != nil {
			alias, err = c.pool.ResolveTerraformAlias(name)
		} else {
			err = db.ErrNotFound
		}
		if err != nil {
			alias, err = c.repo.GetTerraformInventoryAliasByAlias(name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			user, password, ok := r.BasicAuth()
			key, keyErr := c.store.GetAccessKey(alias.ProjectID, alias.AuthKeyID)
			if !ok || keyErr != nil || key.Type != db.AccessKeyLoginPassword || key.Owner != db.AccessKeyShared || c.encryption == nil || c.encryption.DeserializeSecret(&key) != nil || key.LoginPassword.Password == "" || !equalTerraformCredential(user, key.LoginPassword.Login) || !equalTerraformCredential(password, key.LoginPassword.Password) {
				w.Header().Set("WWW-Authenticate", `Basic realm="TaskExec Terraform state"`)
				helpers.WriteErrorStatus(w, "Invalid backend credentials", http.StatusUnauthorized)
				return
			}
		}
		inventory, err := c.store.GetInventory(alias.ProjectID, alias.InventoryID)
		if err != nil || !inventory.Type.IsTerraform() {
			http.NotFound(w, r)
			return
		}
		r = helpers.SetContextValue(r, "terraformAlias", alias)
		r = r.WithContext(audit.WithActor(r.Context(), audit.SystemActor("terraform_backend")))
		next.ServeHTTP(w, r)
	})
}
func terraformError(w http.ResponseWriter, err error) {
	var locked *db.TerraformLockError
	if errors.As(err, &locked) {
		helpers.WriteJSON(w, http.StatusConflict, locked.Lock)
		return
	}
	helpers.WriteError(w, err)
}
func (c *TerraformController) GetTerraformState(w http.ResponseWriter, r *http.Request) {
	alias := helpers.GetFromContext(r, "terraformAlias").(db.TerraformInventoryAlias)
	state, err := c.repo.GetLatestTerraformInventoryState(alias.ProjectID, alias.InventoryID)
	if err != nil {
		terraformError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, state.State)
}
func (c *TerraformController) AddTerraformState(w http.ResponseWriter, r *http.Request) {
	alias := helpers.GetFromContext(r, "terraformAlias").(db.TerraformInventoryAlias)
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, db.TerraformMaxStateBytes))
	if err != nil {
		helpers.WriteErrorStatus(w, "State is unreadable or exceeds 32 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	state, err := c.repo.SaveTerraformInventoryState(db.TerraformInventoryState{ProjectID: alias.ProjectID, InventoryID: alias.InventoryID, TaskID: alias.TaskID, State: string(body)}, r.URL.Query().Get("ID"))
	if err != nil {
		terraformError(w, err)
		return
	}
	c.record(r, audit.TerraformStateWrite, alias, state.ID)
	w.WriteHeader(http.StatusOK)
}
func (c *TerraformController) DeleteTerraformState(w http.ResponseWriter, r *http.Request) {
	alias := helpers.GetFromContext(r, "terraformAlias").(db.TerraformInventoryAlias)
	if err := c.repo.ResetTerraformInventoryState(alias.ProjectID, alias.InventoryID, r.URL.Query().Get("ID")); err != nil {
		terraformError(w, err)
		return
	}
	c.record(r, audit.TerraformStateReset, alias, 0)
	w.WriteHeader(http.StatusOK)
}
func (c *TerraformController) changeLock(w http.ResponseWriter, r *http.Request, unlock bool) {
	alias := helpers.GetFromContext(r, "terraformAlias").(db.TerraformInventoryAlias)
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	var lock db.TerraformStateLock
	if err != nil || json.Unmarshal(body, &lock) != nil || lock.ID == "" {
		helpers.WriteErrorStatus(w, "Valid lock information with an ID is required", http.StatusBadRequest)
		return
	}
	kind := audit.TerraformStateLock
	if unlock {
		err = c.repo.UnlockTerraformInventoryState(alias.ProjectID, alias.InventoryID, lock.ID)
		kind = audit.TerraformStateUnlock
	} else {
		err = c.repo.LockTerraformInventoryState(alias.ProjectID, alias.InventoryID, lock)
	}
	if err != nil {
		terraformError(w, err)
		return
	}
	c.record(r, kind, alias, 0)
	w.WriteHeader(http.StatusOK)
}
func (c *TerraformController) LockTerraformState(w http.ResponseWriter, r *http.Request) {
	c.changeLock(w, r, false)
}
func (c *TerraformController) UnlockTerraformState(w http.ResponseWriter, r *http.Request) {
	c.changeLock(w, r, true)
}
func (c *TerraformController) record(r *http.Request, kind audit.Kind, alias db.TerraformInventoryAlias, stateID int) {
	helpers.Audit(r).Record(r.Context(), audit.Event{Kind: kind, ProjectID: alias.ProjectID, Target: audit.ResourceTarget(audit.TargetInventory, alias.InventoryID, ""), Metadata: audit.TerraformMetadata{StateID: stateID, AuthKeyID: alias.AuthKeyID, TaskID: alias.TaskID}})
}
