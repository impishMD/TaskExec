package api

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/tz"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/tasks"
)

func decodeTokenBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		helpers.WriteErrorStatus(w, "Invalid request fields", http.StatusBadRequest)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		helpers.WriteErrorStatus(w, "Expected one JSON object", http.StatusBadRequest)
		return false
	}
	return true
}

func projectTokenFrom(r *http.Request) (db.ProjectToken, bool) {
	t, ok := helpers.GetFromContext(r, "project_token").(db.ProjectToken)
	return t, ok
}

func authenticateProjectToken(w http.ResponseWriter, r *http.Request, secret string) (bool, *http.Request) {
	id := db.ProjectTokenID(secret)
	t, err := helpers.Store(r).GetProjectTokenByID(id)
	if id == "" || err != nil || !t.MatchesSecret(secret) || !t.IsActive(tz.Now()) {
		helpers.Audit(r).Record(r.Context(), audit.Event{Kind: audit.AuthAPITokenReject, Outcome: audit.OutcomeFailure,
			Reason: audit.ReasonTokenUnknown, Target: &audit.Target{Type: audit.TargetProjectToken, ID: audit.TokenFingerprint(secret)}})
		w.WriteHeader(http.StatusUnauthorized)
		return false, r
	}
	// No creator lookup: a project credential lives independently of any account.
	r = helpers.SetContextValue(r, "project_token", t)
	r = r.WithContext(audit.WithActor(r.Context(), audit.ProjectTokenActor(t.ID, t.Name)))
	return true, r
}

// Management is restricted to application admins and the built-in project owner.
// Token permissions themselves never grant access to this endpoint.
func projectTokens(w http.ResponseWriter, r *http.Request) {
	user := helpers.UserFromContext(r)
	project := helpers.GetFromContext(r, "project").(db.Project)
	role := helpers.GetFromContext(r, "projectUserRole").(db.ProjectUserRole)
	if !user.Admin && role != db.ProjectOwner {
		helpers.RecordDenied(r, "manage_project_tokens", project.ID)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	store := helpers.Store(r)
	id := mux.Vars(r)["token_id"]
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		items, err := store.GetProjectTokens(project.ID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		helpers.WriteJSON(w, http.StatusOK, items)
		return
	}
	if r.Method == http.MethodDelete {
		t, err := store.GetProjectToken(project.ID, id)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		if t.RevokedAt == nil {
			if err = store.RevokeProjectToken(project.ID, id); err != nil {
				helpers.WriteError(w, err)
				return
			}
			helpers.Audit(r).Record(r.Context(), audit.Event{Kind: audit.IAMProjectTokenRevoke, ProjectID: project.ID,
				Target: &audit.Target{Type: audit.TargetProjectToken, ID: id, Name: t.Name}})
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		Name         string     `json:"name"`
		Scopes       []string   `json:"scopes"`
		AllTemplates bool       `json:"all_templates"`
		TemplateIDs  []int      `json:"template_ids"`
		Overrides    []string   `json:"overrides"`
		ExpiresAt    *time.Time `json:"expires_at"`
	}
	if !decodeTokenBody(w, r, &body) {
		return
	}
	t := db.ProjectToken{ProjectID: project.ID, Name: strings.TrimSpace(body.Name), Scopes: body.Scopes,
		AllTemplates: body.AllTemplates, TemplateIDs: body.TemplateIDs, Overrides: body.Overrides,
		ExpiresAt: body.ExpiresAt, CreatorID: user.ID, CreatorName: user.Username}
	if err := t.Validate(tz.Now()); err != nil {
		helpers.WriteError(w, err)
		return
	}
	for _, templateID := range t.TemplateIDs {
		if _, err := store.GetTemplate(project.ID, templateID); err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	secret, err := t.IssueSecret()
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	t, err = store.CreateProjectToken(t, id)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	kind := audit.IAMProjectTokenCreate
	if id != "" {
		kind = audit.IAMProjectTokenRotate
	}
	helpers.Audit(r).Record(r.Context(), audit.Event{Kind: kind, ProjectID: project.ID,
		Target:   &audit.Target{Type: audit.TargetProjectToken, ID: t.ID, Name: t.Name},
		Metadata: audit.ProjectTokenMetadata{Scopes: t.Scopes, TemplateIDs: t.TemplateIDs, AllTemplates: t.AllTemplates, Overrides: t.Overrides, PreviousID: id}})
	w.Header().Set("Cache-Control", "no-store")
	helpers.WriteJSON(w, http.StatusCreated, struct {
		db.ProjectToken
		Token string `json:"token"`
	}{t, secret})
}

func denyProjectToken(w http.ResponseWriter, r *http.Request, t db.ProjectToken) {
	helpers.RecordDenied(r, "project_token_scope", t.ProjectID)
	w.WriteHeader(http.StatusForbidden)
}

// Project credentials have their own explicit route allowlist and response DTOs.
// They never enter the user/admin middleware, even when issued by an admin.
func serveProjectToken(w http.ResponseWriter, r *http.Request, t db.ProjectToken) {
	projectID, err := strconv.Atoi(mux.Vars(r)["project_id"])
	if err != nil || projectID != t.ProjectID {
		denyProjectToken(w, r, t)
		return
	}
	if _, err = helpers.Store(r).GetProject(projectID); err != nil {
		helpers.WriteError(w, err)
		return
	}
	prefix := "/project/" + strconv.Itoa(projectID)
	idx := strings.LastIndex(r.URL.Path, prefix)
	if idx < 0 {
		denyProjectToken(w, r, t)
		return
	}
	path := r.URL.Path[idx+len(prefix):]
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	templateID, _ := strconv.Atoi(mux.Vars(r)["template_id"])
	taskID, _ := strconv.Atoi(mux.Vars(r)["task_id"])
	scope := ""
	switch {
	case read && (path == "/templates" || (templateID > 0 && path == "/templates/"+strconv.Itoa(templateID))):
		scope = db.TokenReadTemplates
	case r.Method == http.MethodPost && path == "/tasks":
		scope = db.TokenRunTasks
	case read && (path == "/tasks" || path == "/tasks/last" || (taskID > 0 && path == "/tasks/"+strconv.Itoa(taskID)) || (templateID > 0 && path == "/templates/"+strconv.Itoa(templateID)+"/tasks")):
		scope = db.TokenReadTasks
	case read && taskID > 0 && path == "/tasks/"+strconv.Itoa(taskID)+"/output":
		scope = db.TokenReadLogs
	case r.Method == http.MethodPost && taskID > 0 && path == "/tasks/"+strconv.Itoa(taskID)+"/stop":
		scope = db.TokenStopTasks
	}
	if scope == "" || !t.Can(scope) || (templateID > 0 && !t.AllowsTemplate(templateID)) {
		denyProjectToken(w, r, t)
		return
	}
	store := helpers.Store(r)
	var task db.Task
	if taskID > 0 {
		task, err = store.GetTask(projectID, taskID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		if !t.AllowsTemplate(task.TemplateID) {
			denyProjectToken(w, r, t)
			return
		}
		if scope == db.TokenStopTasks && (task.ProjectTokenID == nil || *task.ProjectTokenID != t.ID) {
			denyProjectToken(w, r, t)
			return
		}
	}
	if err = store.TouchProjectToken(projectID, t.ID); err != nil {
		helpers.WriteError(w, err)
		return
	}
	switch scope {
	case db.TokenReadTemplates:
		if templateID > 0 {
			tpl, e := store.GetTemplate(projectID, templateID)
			if e != nil {
				helpers.WriteError(w, e)
				return
			}
			helpers.WriteJSON(w, http.StatusOK, projectTokenTemplate(tpl))
			return
		}
		items, e := store.GetTemplates(projectID, db.TemplateFilter{}, db.RetrieveQueryParams{})
		if e != nil {
			helpers.WriteError(w, e)
			return
		}
		result := []map[string]any{}
		for _, tpl := range items {
			if t.AllowsTemplate(tpl.ID) {
				result = append(result, projectTokenTemplate(tpl))
			}
		}
		helpers.WriteJSON(w, http.StatusOK, result)
	case db.TokenRunTasks:
		runProjectTokenTask(w, r, t)
	case db.TokenReadTasks:
		if taskID > 0 {
			helpers.WriteJSON(w, http.StatusOK, projectTokenTask(task))
			return
		}
		params := db.RetrieveQueryParams{Count: 100, TaskFilter: &db.TaskFilter{}}
		if before, e := strconv.Atoi(r.URL.Query().Get("before")); e == nil && before > 0 {
			params.BeforeID = before
		}
		if templateID > 0 {
			params.TaskFilter.TemplateIDs = []int{templateID}
		} else if !t.AllTemplates {
			params.TaskFilter.TemplateIDs = t.TemplateIDs
		}
		items, e := store.GetProjectTasks(projectID, params)
		if e != nil {
			helpers.WriteError(w, e)
			return
		}
		result := []map[string]any{}
		for _, item := range items {
			result = append(result, projectTokenTask(item.Task))
		}
		helpers.WriteJSON(w, http.StatusOK, result)
	case db.TokenReadLogs:
		items, e := store.GetTaskOutputs(projectID, taskID, db.RetrieveQueryParams{})
		if e != nil {
			helpers.WriteError(w, e)
			return
		}
		helpers.WriteJSON(w, http.StatusOK, items)
	case db.TokenStopTasks:
		var body struct {
			Force bool `json:"force"`
		}
		if !decodeTokenBody(w, r, &body) {
			return
		}
		pool := helpers.GetFromContext(r, "task_pool").(*tasks.TaskPool)
		changed, e := pool.StopTask(task, body.Force)
		if e != nil {
			helpers.WriteError(w, e)
			return
		}
		if changed {
			kind := audit.TaskControlStop
			if body.Force {
				kind = audit.TaskControlForceStop
			}
			helpers.Audit(r).Record(r.Context(), audit.Event{Kind: kind, ProjectID: projectID,
				Target: audit.ResourceTarget(audit.TargetTask, taskID, ""), Metadata: audit.TaskMetadata{TemplateID: task.TemplateID}})
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func projectTokenTemplate(t db.Template) map[string]any {
	// Deliberately omit repository credentials, environment, arguments and survey defaults.
	vars := make([]map[string]any, 0, len(t.SurveyVars))
	for _, v := range t.SurveyVars {
		vars = append(vars, map[string]any{"name": v.Name, "title": v.Title, "type": v.Type, "required": v.Required, "values": v.Values})
	}
	return map[string]any{"id": t.ID, "project_id": t.ProjectID, "name": t.Name, "app": t.App, "type": t.Type, "survey_vars": vars}
}

func projectTokenTask(t db.Task) map[string]any {
	return map[string]any{"id": t.ID, "project_id": t.ProjectID, "template_id": t.TemplateID,
		"status": t.Status, "created": t.Created, "start": t.Start, "end": t.End, "version": t.Version,
		"project_token_id": t.ProjectTokenID, "project_token_name": t.ProjectTokenName}
}

func runProjectTokenTask(w http.ResponseWriter, r *http.Request, token db.ProjectToken) {
	// Never bind db.Task: caller-controlled source IDs and internal fields are rejected.
	var body struct {
		TemplateID   int                  `json:"template_id"`
		TemplateName string               `json:"template_name"`
		Message      string               `json:"message"`
		Environment  string               `json:"environment"`
		Secret       string               `json:"secret"`
		GitBranch    *string              `json:"git_branch"`
		Playbook     string               `json:"playbook"`
		InventoryID  *int                 `json:"inventory_id"`
		Arguments    *string              `json:"arguments"`
		Params       db.MapStringAnyField `json:"params"`
	}
	if !decodeTokenBody(w, r, &body) {
		return
	}
	store := helpers.Store(r)
	var tpl db.Template
	var err error
	if body.TemplateID > 0 && body.TemplateName == "" {
		tpl, err = store.GetTemplate(token.ProjectID, body.TemplateID)
	} else if body.TemplateID == 0 && body.TemplateName != "" {
		tpl, err = store.GetTemplateByName(token.ProjectID, body.TemplateName)
	} else {
		helpers.WriteErrorStatus(w, "Specify template_id or template_name", http.StatusBadRequest)
		return
	}
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	if !token.AllowsTemplate(tpl.ID) {
		denyProjectToken(w, r, token)
		return
	}
	for _, field := range []struct {
		name string
		set  bool
	}{
		{"git_branch", body.GitBranch != nil}, {"playbook", body.Playbook != ""}, {"inventory_id", body.InventoryID != nil},
		{"arguments", body.Arguments != nil}, {"params", len(body.Params) > 0},
	} {
		if field.set && !slices.Contains(token.Overrides, field.name) {
			denyProjectToken(w, r, token)
			return
		}
	}
	if body.InventoryID != nil {
		if _, err = store.GetInventory(token.ProjectID, *body.InventoryID); err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	body.Environment, body.Secret, err = projectTokenSurveyDefaults(tpl, body.Environment, body.Secret)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	if !slices.Contains(token.Overrides, "environment") {
		if err = validateTokenSurvey(tpl, body.Environment, body.Secret); err != nil {
			helpers.WriteError(w, err)
			return
		}
	}
	task := db.Task{TemplateID: tpl.ID, Message: body.Message, Environment: body.Environment, Secret: body.Secret,
		GitBranch: body.GitBranch, Playbook: body.Playbook, InventoryID: body.InventoryID, Arguments: body.Arguments, Params: body.Params}
	if err = task.ValidateNewTask(tpl); err != nil {
		helpers.WriteError(w, err)
		return
	}
	pool := helpers.GetFromContext(r, "task_pool").(*tasks.TaskPool)
	created, err := pool.AddTaskFrom(r.Context(), audit.TriggerAPI, task, nil, token.Name, token.ProjectID, tpl.App.NeedTaskAlias())
	if err != nil {
		helpers.WriteError(w, err)
		return
	}
	helpers.WriteJSON(w, http.StatusCreated, projectTokenTask(created))
}

// The UI supplies survey defaults itself; API clients cannot read those defaults.
// Apply saved defaults on the server and keep secret fields out of task.environment.
func projectTokenSurveyDefaults(tpl db.Template, values, secrets string) (string, string, error) {
	vars, hidden := map[string]any{}, map[string]any{}
	for _, field := range []struct {
		raw string
		dst *map[string]any
	}{{values, &vars}, {secrets, &hidden}} {
		if field.raw != "" {
			if err := json.Unmarshal([]byte(field.raw), field.dst); err != nil || *field.dst == nil {
				return "", "", common_errors.NewValidationError("Variables must be a JSON object")
			}
		}
	}
	for name := range vars {
		if _, duplicate := hidden[name]; duplicate {
			return "", "", common_errors.NewValidationError("Duplicate survey variable")
		}
	}
	for _, v := range tpl.SurveyVars {
		_, visible := vars[v.Name]
		_, secret := hidden[v.Name]
		if !visible && !secret && v.DefaultValue != nil {
			switch v.Type {
			case db.SurveyVarSelect:
				vars[v.Name] = append([]string{}, v.DefaultValue.Values...)
			case db.SurveyVarInt:
				if v.DefaultValue.String() != "" {
					n, err := strconv.ParseInt(v.DefaultValue.String(), 10, 64)
					if err != nil {
						return "", "", common_errors.NewValidationError("Invalid integer survey default")
					}
					vars[v.Name] = n
				}
			default:
				vars[v.Name] = v.DefaultValue.String()
			}
		}
		if v.Type == "secret" {
			if value, exists := vars[v.Name]; exists {
				hidden[v.Name] = value
				delete(vars, v.Name)
			}
		}
	}
	visibleJSON, err := json.Marshal(vars)
	if err != nil {
		return "", "", err
	}
	secretJSON, err := json.Marshal(hidden)
	return string(visibleJSON), string(secretJSON), err
}

func validateTokenSurvey(tpl db.Template, values, secrets string) error {
	all := map[string]any{}
	for _, raw := range []string{values, secrets} {
		if raw == "" {
			continue
		}
		var vars map[string]any
		if err := json.Unmarshal([]byte(raw), &vars); err != nil || vars == nil {
			return common_errors.NewValidationError("Variables must be a JSON object")
		}
		for k, v := range vars {
			if _, exists := all[k]; exists {
				return common_errors.NewValidationError("Duplicate survey variable")
			}
			all[k] = v
		}
	}
	for name, value := range all {
		idx := slices.IndexFunc(tpl.SurveyVars, func(v db.SurveyVar) bool { return v.Name == name })
		if idx < 0 {
			return common_errors.NewValidationError("Only declared survey variables are allowed")
		}
		v := tpl.SurveyVars[idx]
		if err := validateTokenSurveyValue(v, value); err != nil {
			return err
		}
	}
	for _, v := range tpl.SurveyVars {
		value, set := all[v.Name]
		empty := !set || value == "" || value == nil
		if items, ok := value.([]any); ok && len(items) == 0 {
			empty = true
		}
		if v.Required && empty {
			return common_errors.NewValidationError("A required survey variable is missing")
		}
	}
	return nil
}

func validateTokenSurveyValue(v db.SurveyVar, value any) error {
	invalid := common_errors.NewValidationError("Invalid survey variable value")
	allowed := func(s string) bool {
		return slices.ContainsFunc(v.Values, func(option db.SurveyVarEnumValue) bool { return option.Value == s })
	}
	switch v.Type {
	case db.SurveyVarInt:
		n, ok := value.(float64)
		if !ok || n != float64(int64(n)) {
			return invalid
		}
	case db.SurveyVarEnum:
		s, ok := value.(string)
		if !ok || !allowed(s) {
			return invalid
		}
	case db.SurveyVarSelect:
		items, ok := value.([]any)
		if !ok {
			return invalid
		}
		for _, item := range items {
			s, ok := item.(string)
			if !ok || !allowed(s) {
				return invalid
			}
		}
	default:
		if _, ok := value.(string); !ok {
			return invalid
		}
	}
	return nil
}
