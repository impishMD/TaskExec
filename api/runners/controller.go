package runners

import (
	"net/http"

	"github.com/impishMD/taskexec/api/helpers"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/tz"
	"github.com/impishMD/taskexec/services/audit"
	"github.com/impishMD/taskexec/services/server"
	"github.com/impishMD/taskexec/util"
	log "github.com/sirupsen/logrus"
)

type runnerWithToken struct {
	db.Runner
	Token string `json:"token"`
}

// Controller manages runners in either the administrator or project scope.
type Controller struct {
	projectScoped bool
	runnerService server.RunnerService
}

func NewController(runnerService server.RunnerService, projectScoped bool) *Controller {
	return &Controller{
		runnerService: runnerService,
		projectScoped: projectScoped,
	}
}

func (c *Controller) GetRunners(w http.ResponseWriter, r *http.Request) {
	var runners []db.Runner
	var err error
	if projectID := c.projectID(r); projectID != nil {
		runners, err = helpers.Store(r).GetRunners(*projectID, false, db.RunnerFilterIgnoreTags, nil)
	} else {
		runners, err = helpers.Store(r).GetAllRunners(false, false, db.RunnerFilterIgnoreTags, nil)
	}

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	var result = make([]db.Runner, 0)

	result = append(result, runners...)

	now := tz.Now()
	offlineTimeout := util.Config.RunnersOfflineTimeout()

	for i := range result {
		result[i].Registered = result[i].IsRegistered()
		result[i].FillStatus(now, offlineTimeout)
	}

	helpers.WriteJSON(w, http.StatusOK, result)
}

func (c *Controller) AddRunner(w http.ResponseWriter, r *http.Request) {
	var runner db.Runner
	if !helpers.Bind(w, r, &runner) {
		return
	}

	runner.ProjectID = c.projectID(r)

	newRunner, err := c.runnerService.CreateRunner(runner)

	if err != nil {
		log.Warn("Runner is not created: " + err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.RunnerLifecycleCreate,
		ProjectID: runnerProjectID(newRunner),
		Target:    audit.ResourceTarget(audit.TargetRunner, newRunner.ID, newRunner.Name),
	})

	helpers.WriteJSON(w, http.StatusCreated, runnerWithToken{
		Runner: newRunner,
		Token:  newRunner.Token,
	})
}

func (c *Controller) RunnerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runnerID, ok := helpers.GetIntParamOrAbort("runner_id", w, r)

		if !ok {
			return
		}

		store := helpers.Store(r)

		var runner db.Runner
		var err error
		if projectID := c.projectID(r); projectID != nil {
			runner, err = store.GetRunner(*projectID, runnerID)
		} else {
			runner, err = store.GetGlobalRunner(runnerID)
		}

		if err != nil {
			helpers.WriteJSON(w, http.StatusNotFound, map[string]string{
				"error": "Runner not found",
			})
			return
		}

		r = helpers.SetContextValue(r, "runner", &runner)
		next.ServeHTTP(w, r)
	})
}

func (c *Controller) GetRunner(w http.ResponseWriter, r *http.Request) {
	runner := helpers.GetFromContext(r, "runner").(*db.Runner)

	runner.Registered = runner.IsRegistered()
	runner.FillStatus(tz.Now(), util.Config.RunnersOfflineTimeout())

	helpers.WriteJSON(w, http.StatusOK, runner)
}

func (c *Controller) UpdateRunner(w http.ResponseWriter, r *http.Request) {
	oldRunner := helpers.GetFromContext(r, "runner").(*db.Runner)

	var runner db.Runner
	if !helpers.Bind(w, r, &runner) {
		return
	}

	store := helpers.Store(r)

	runner.ID = oldRunner.ID
	runner.ProjectID = oldRunner.ProjectID

	err := store.UpdateRunner(runner)

	if err != nil {
		helpers.WriteErrorStatus(w, err.Error(), http.StatusBadRequest)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.RunnerLifecycleUpdate,
		ProjectID: runnerProjectID(*oldRunner),
		Target:    audit.ResourceTarget(audit.TargetRunner, oldRunner.ID, runner.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) ClearRunnerCache(w http.ResponseWriter, r *http.Request) {
	runner := helpers.GetFromContext(r, "runner").(*db.Runner)

	store := helpers.Store(r)

	err := store.ClearRunnerCache(*runner)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.RunnerCacheClear,
		ProjectID: runnerProjectID(*runner),
		Target:    audit.ResourceTarget(audit.TargetRunner, runner.ID, runner.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) DeleteRunner(w http.ResponseWriter, r *http.Request) {
	runner := helpers.GetFromContext(r, "runner").(*db.Runner)

	store := helpers.Store(r)

	var err error
	if projectID := c.projectID(r); projectID != nil {
		err = store.DeleteRunner(*projectID, runner.ID)
	} else {
		err = store.DeleteGlobalRunner(runner.ID)
	}

	if err != nil {
		helpers.WriteErrorStatus(w, err.Error(), http.StatusBadRequest)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.RunnerLifecycleDelete,
		ProjectID: runnerProjectID(*runner),
		Target:    audit.ResourceTarget(audit.TargetRunner, runner.ID, runner.Name),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) RegenerateRegistrationToken(w http.ResponseWriter, r *http.Request) {
	runner := helpers.GetFromContext(r, "runner").(*db.Runner)

	token, err := c.runnerService.RegenerateRegistrationToken(*runner)

	if err != nil {
		helpers.WriteErrorStatus(w, err.Error(), http.StatusBadRequest)
		return
	}

	helpers.Audit(r).Record(r.Context(), audit.Event{
		Kind:      audit.RunnerCredentialRotate,
		ProjectID: runnerProjectID(*runner),
		Target:    audit.ResourceTarget(audit.TargetRunner, runner.ID, runner.Name),
	})

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"registration_token": token,
		"runner_id":          runner.ID,
	})
}

func (c *Controller) GetRunnerTags(w http.ResponseWriter, r *http.Request) {
	var tags []db.RunnerTag
	var err error
	if projectID := c.projectID(r); projectID != nil {
		tags, err = helpers.Store(r).GetRunnerTags(*projectID)
	} else {
		tags, err = helpers.Store(r).GetGlobalRunnerTags()
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, tags)
}

func (c *Controller) SetRunnerActive(w http.ResponseWriter, r *http.Request) {
	runner := helpers.GetFromContext(r, "runner").(*db.Runner)

	store := helpers.Store(r)

	var body struct {
		Active bool `json:"active"`
	}

	if !helpers.Bind(w, r, &body) {
		helpers.WriteErrorStatus(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	changed := runner.Active != body.Active
	runner.Active = body.Active

	err := store.UpdateRunner(*runner)

	if err != nil {
		helpers.WriteErrorStatus(w, err.Error(), http.StatusBadRequest)
		return
	}

	if changed {
		kind := audit.RunnerLifecycleDisable
		if body.Active {
			kind = audit.RunnerLifecycleEnable
		}
		helpers.Audit(r).Record(r.Context(), audit.Event{
			Kind:      kind,
			ProjectID: runnerProjectID(*runner),
			Target:    audit.ResourceTarget(audit.TargetRunner, runner.ID, runner.Name),
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) projectID(r *http.Request) *int {
	if !c.projectScoped {
		return nil
	}
	project := helpers.GetFromContext(r, "project").(db.Project)
	return &project.ID
}

func runnerProjectID(runner db.Runner) int {
	if runner.ProjectID == nil {
		return 0
	}
	return *runner.ProjectID
}
