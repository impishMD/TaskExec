package projects

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/random"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/util"
)

type TerraformInventoryController struct{ repo db.TerraformStore }

type publicTerraformAlias struct {
	db.TerraformInventoryAlias
	ID  string `json:"id"`
	URL string `json:"url"`
}

func terraformAliasResponse(alias db.TerraformInventoryAlias) publicTerraformAlias {
	return publicTerraformAlias{TerraformInventoryAlias: alias, ID: alias.Alias, URL: util.GetPublicAliasURL("terraform", alias.Alias)}
}

func NewTerraformInventoryController(repo db.TerraformStore) *TerraformInventoryController {
	return &TerraformInventoryController{repo: repo}
}
func terraformWorkspace(w http.ResponseWriter, r *http.Request) (db.Inventory, bool) {
	inventory := helpers.GetFromContext(r, "inventory").(db.Inventory)
	if !inventory.Type.IsTerraform() {
		http.NotFound(w, r)
		return inventory, false
	}
	return inventory, true
}
func canReadTerraformSecret(w http.ResponseWriter, r *http.Request) bool {
	me := helpers.UserFromContext(r)
	permissions := helpers.GetFromContext(r, "permissions").(db.ProjectUserPermission)
	if !me.Admin && permissions&db.CanManageProjectResources == 0 {
		inventory := helpers.GetFromContext(r, "inventory").(db.Inventory)
		helpers.RecordDenied(r, "manage_resources", inventory.ProjectID)
		w.WriteHeader(http.StatusForbidden)
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}
func (c *TerraformInventoryController) GetTerraformInventoryAliases(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	aliases, err := c.repo.GetTerraformInventoryAliases(inv.ProjectID, inv.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	result := make([]publicTerraformAlias, 0, len(aliases))
	for _, alias := range aliases {
		result = append(result, terraformAliasResponse(alias))
	}
	helpers.WriteJSON(w, http.StatusOK, result)
}
func (c *TerraformInventoryController) GetTerraformInventoryAlias(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	alias, err := c.repo.GetTerraformInventoryAlias(inv.ProjectID, inv.ID, mux.Vars(r)["alias_id"])
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, terraformAliasResponse(alias))
}
func (c *TerraformInventoryController) saveAlias(w http.ResponseWriter, r *http.Request, create bool) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	var body struct {
		AuthKeyID int `json:"auth_key_id"`
	}
	if !helpers.Bind(w, r, &body) {
		return
	}
	alias := db.TerraformInventoryAlias{ProjectID: inv.ProjectID, InventoryID: inv.ID, AuthKeyID: body.AuthKeyID, Alias: mux.Vars(r)["alias_id"]}
	var err error
	kind := audit.TerraformAliasUpdate
	status := http.StatusOK
	if create {
		alias.Alias = random.String(32)
		alias, err = c.repo.CreateTerraformInventoryAlias(alias)
		kind = audit.TerraformAliasCreate
		status = http.StatusCreated
	} else {
		err = c.repo.UpdateTerraformInventoryAlias(alias)
	}
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	c.record(r, kind, inv, audit.TerraformMetadata{AuthKeyID: alias.AuthKeyID})
	helpers.WriteJSON(w, status, terraformAliasResponse(alias))
}
func (c *TerraformInventoryController) AddTerraformInventoryAlias(w http.ResponseWriter, r *http.Request) {
	c.saveAlias(w, r, true)
}
func (c *TerraformInventoryController) SetTerraformInventoryAliasAccessKey(w http.ResponseWriter, r *http.Request) {
	c.saveAlias(w, r, false)
}
func (c *TerraformInventoryController) DeleteTerraformInventoryAlias(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	err := c.repo.DeleteTerraformInventoryAlias(inv.ProjectID, inv.ID, mux.Vars(r)["alias_id"])
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	c.record(r, audit.TerraformAliasDelete, inv, audit.TerraformMetadata{})
	w.WriteHeader(http.StatusNoContent)
}
func (c *TerraformInventoryController) GetTerraformInventoryStates(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	states, err := c.repo.GetTerraformInventoryStates(inv.ProjectID, inv.ID, helpers.QueryParams(r.URL))
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, states)
}
func (c *TerraformInventoryController) GetTerraformInventoryLatestState(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok || !canReadTerraformSecret(w, r) {
		return
	}
	state, err := c.repo.GetLatestTerraformInventoryState(inv.ProjectID, inv.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, state)
}
func (c *TerraformInventoryController) GetTerraformInventoryState(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok || !canReadTerraformSecret(w, r) {
		return
	}
	id, ok := helpers.GetIntParamOrAbort("state_id", w, r)
	if !ok {
		return
	}
	state, err := c.repo.GetTerraformInventoryState(inv.ProjectID, inv.ID, id)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusOK, state)
}
func (c *TerraformInventoryController) DeleteTerraformInventoryState(w http.ResponseWriter, r *http.Request) {
	inv, ok := terraformWorkspace(w, r)
	if !ok {
		return
	}
	id, ok := helpers.GetIntParamOrAbort("state_id", w, r)
	if !ok {
		return
	}
	err := c.repo.DeleteTerraformInventoryState(inv.ProjectID, inv.ID, id)
	if err != nil {
		var locked *db.TerraformLockError
		if errors.As(err, &locked) {
			helpers.WriteErrorStatus(w, locked.Error(), http.StatusConflict)
		} else {
			helpers.WriteError(w, err)
		}
		return
	}
	c.record(r, audit.TerraformStateDelete, inv, audit.TerraformMetadata{StateID: id})
	w.WriteHeader(http.StatusNoContent)
}
func (c *TerraformInventoryController) record(r *http.Request, kind audit.Kind, inv db.Inventory, metadata audit.TerraformMetadata) {
	helpers.Audit(r).Record(r.Context(), audit.Event{Kind: kind, ProjectID: inv.ProjectID, Target: audit.ResourceTarget(audit.TargetInventory, inv.ID, inv.Name), Metadata: metadata})
}
