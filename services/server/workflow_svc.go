package server

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/pkg/tz"
	"github.com/impishMD/taskexec/services/audit"
	log "github.com/sirupsen/logrus"
)

type WorkflowService interface {
	StartWorkflow(db.WorkflowTemplate, *db.User) (db.WorkflowRun, error)
	ProgressWorkflowRun(projectID, runID int, user *db.User) error
	StopWorkflowRun(projectID, runID int, user *db.User) (db.WorkflowRun, error)
	ResolveWorkflowApproval(projectID, workflowID, runID, nodeID int, status db.WorkflowApprovalStatus, user *db.User) (db.WorkflowApproval, error)
	HandleWorkflowTaskCompletion(db.Task) error
	GetWorkflowRunArtifacts(projectID, runID int, currentTaskID *int) (map[string]any, error)
}

// Kept next to the consumer to avoid a dependency from server to tasks.
type WorkflowTaskEnqueuer interface {
	AddTask(db.Task, *int, string, int, bool) (db.Task, error)
	StopTasksByWorkflowRun(projectID, runID int, forceStop bool)
}

type workflowService struct {
	repo     db.WorkflowManager
	store    db.Store
	queue    WorkflowTaskEnqueuer
	recorder audit.Recorder
	// Active-active servers remain unsupported. Within the server, callbacks,
	// API actions and the reconciler serialize progression of the same run.
	locks [64]sync.Mutex
}

func NewWorkflowService(repo db.WorkflowManager, store db.Store, queue WorkflowTaskEnqueuer, recorder audit.Recorder) WorkflowService {
	if recorder == nil {
		recorder = audit.Nop{}
	}
	return &workflowService{repo: repo, store: store, queue: queue, recorder: recorder}
}
func (s *workflowService) lock(runID int) func() {
	mu := &s.locks[uint(runID)%uint(len(s.locks))]
	mu.Lock()
	return mu.Unlock
}

var workflowFinished = []db.WorkflowRunStatus{db.WorkflowRunSuccess, db.WorkflowRunFailed, db.WorkflowRunStopped}

func (s *workflowService) record(kind audit.Kind, run db.WorkflowRun, nodeID int, user *db.User) {
	actor := audit.SystemActor("workflow")
	if user != nil {
		actor = audit.UserActor(user.ID, user.Username, "", "")
	}
	s.recorder.Record(audit.WithActor(context.Background(), actor), audit.Event{
		Kind: kind, ProjectID: run.ProjectID, Target: audit.ResourceTarget(audit.TargetWorkflow, run.WorkflowTemplateID, ""),
		Metadata: audit.WorkflowMetadata{RunID: run.ID, RevisionID: run.RevisionID, NodeID: nodeID, Status: string(run.Status)},
	})
}
func (s *workflowService) StartWorkflow(workflow db.WorkflowTemplate, user *db.User) (db.WorkflowRun, error) {
	// Always load the latest saved graph; clients cannot supply an executable graph.
	workflow, err := s.repo.GetWorkflowTemplate(workflow.ProjectID, workflow.ID)
	if err != nil {
		return db.WorkflowRun{}, err
	}
	if err = db.ValidateWorkflowTemplate(s.store, workflow); err != nil {
		return db.WorkflowRun{}, err
	}
	run := db.WorkflowRun{ProjectID: workflow.ProjectID, WorkflowTemplateID: workflow.ID, RevisionID: workflow.RevisionID}
	if user != nil {
		run.StartedByUserID = &user.ID
	}
	run, err = s.repo.CreateWorkflowRun(run)
	if err != nil {
		return run, err
	}
	s.record(audit.WorkflowStart, run, 0, user)
	if err = s.ProgressWorkflowRun(run.ProjectID, run.ID, user); err != nil {
		return run, err
	}
	return s.repo.GetWorkflowRunByID(run.ProjectID, run.ID)
}

type workflowNodeState int

const (
	workflowPending workflowNodeState = iota
	workflowActive
	workflowSuccess
	workflowFailure
	workflowSkipped
)

func nodeMatches(status workflowNodeState, condition db.WorkflowEdgeCondition) bool {
	if status != workflowSuccess && status != workflowFailure {
		return false
	}
	return condition == db.WorkflowEdgeAlways || condition == db.WorkflowEdgeOnSuccess && status == workflowSuccess || condition == db.WorkflowEdgeOnFailure && status == workflowFailure
}
func workflowReady(node db.WorkflowNode, incoming []db.WorkflowEdge, states map[int]workflowNodeState) (ready, skipped bool) {
	if len(incoming) == 0 {
		return true, false
	}
	matches, finished := 0, 0
	for _, edge := range incoming {
		state := states[edge.SourceNodeID]
		if state >= workflowSuccess {
			finished++
		}
		if nodeMatches(state, edge.Condition) {
			matches++
		}
	}
	if node.EffectiveConvergenceMode() == db.WorkflowConvergenceAny {
		return matches > 0, matches == 0 && finished == len(incoming)
	}
	return matches == len(incoming), finished > matches
}

func (s *workflowService) ProgressWorkflowRun(p, runID int, _ *db.User) error {
	defer s.lock(runID)()
	run, err := s.repo.GetWorkflowRunByID(p, runID)
	if err != nil || run.Status.IsFinished() {
		return err
	}
	workflow, err := s.repo.GetWorkflowRevisionGraph(p, run.RevisionID)
	if err != nil {
		return err
	}
	order, err := db.WorkflowNodeOrder(workflow)
	if err != nil {
		return s.finish(&run, db.WorkflowRunFailed, nil)
	}
	tasks, err := s.repo.GetWorkflowRunTasks(p, runID, db.RetrieveQueryParams{})
	if err != nil {
		return err
	}
	approvals, err := s.repo.GetWorkflowApprovals(p, runID)
	if err != nil {
		return err
	}
	delays, err := s.repo.GetWorkflowDelays(p, runID)
	if err != nil {
		return err
	}
	byTask := map[int]db.Task{}
	for _, task := range tasks {
		if task.WorkflowNodeID != nil {
			if _, seen := byTask[*task.WorkflowNodeID]; !seen {
				byTask[*task.WorkflowNodeID] = task.Task
			}
		}
	}
	byApproval := map[int]db.WorkflowApproval{}
	for _, item := range approvals {
		byApproval[item.WorkflowNodeID] = item
	}
	byDelay := map[int]db.WorkflowDelay{}
	for _, item := range delays {
		byDelay[item.WorkflowNodeID] = item
	}
	incoming := map[int][]db.WorkflowEdge{}
	for _, edge := range workflow.Edges {
		incoming[edge.DestinationNodeID] = append(incoming[edge.DestinationNodeID], edge)
	}
	states := map[int]workflowNodeState{}
	var initiator *db.User
	if run.StartedByUserID != nil {
		user, e := s.store.GetUser(*run.StartedByUserID)
		if e != nil && !errors.Is(e, db.ErrNotFound) {
			return e
		}
		if e == nil {
			initiator = &user
		}
	}
	activeTasks, waitingApprovals, waitingDelays, failed := 0, 0, 0, false
	for _, node := range order {
		if task, exists := byTask[node.ID]; exists {
			states[node.ID] = workflowActive
			if task.Status.IsFinished() && task.End != nil {
				states[node.ID] = workflowFailure
				if task.Status == task_logger.TaskSuccessStatus {
					states[node.ID] = workflowSuccess
				}
			} else {
				activeTasks++
			}
		} else if approval, exists := byApproval[node.ID]; exists {
			if approval.Status == db.WorkflowApprovalPending && node.ApprovalTimeout != nil && !tz.Now().Before(approval.Created.Add(time.Duration(*node.ApprovalTimeout)*time.Second)) {
				approval.Status = db.WorkflowApprovalRejected
				changed, e := s.repo.ResolveWorkflowApprovalIfPending(approval)
				if e != nil {
					return e
				}
				if changed {
					s.record(audit.WorkflowApprovalTimeout, run, node.ID, nil)
				}
			}
			switch approval.Status {
			case db.WorkflowApprovalApproved:
				states[node.ID] = workflowSuccess
			case db.WorkflowApprovalRejected:
				states[node.ID] = workflowFailure
			default:
				states[node.ID] = workflowActive
				waitingApprovals++
			}
		} else if delay, exists := byDelay[node.ID]; exists {
			if delay.Status == db.WorkflowDelayWaiting && !tz.Now().Before(delay.ResumeAt) {
				delay.Status = db.WorkflowDelaySuccess
				if _, err = s.repo.ResolveWorkflowDelayIfWaiting(delay); err != nil {
					return err
				}
			}
			switch delay.Status {
			case db.WorkflowDelaySuccess:
				states[node.ID] = workflowSuccess
			case db.WorkflowDelayStopped:
				states[node.ID] = workflowFailure
			default:
				states[node.ID] = workflowActive
				waitingDelays++
			}
		} else {
			ready, skipped := workflowReady(node, incoming[node.ID], states)
			if skipped {
				states[node.ID] = workflowSkipped
				continue
			}
			if !ready {
				states[node.ID] = workflowPending
				continue
			}
			states[node.ID] = workflowActive
			switch node.EffectiveKind() {
			case db.WorkflowNodeTaskKind:
				template, e := s.store.GetTemplate(p, node.TemplateID)
				if e != nil {
					return s.failLaunch(&run, e)
				}
				params := db.TaskParams{ProjectID: p}
				if node.TaskParams != nil {
					params = *node.TaskParams
				}
				task := params.CreateTask(node.TemplateID)
				task.WorkflowRunID, task.WorkflowNodeID, task.Version = &run.ID, &node.ID, run.Version
				if template.Type == db.TemplateDeploy {
					task.BuildTaskID = run.RootTaskID
				}
				var uid *int
				username := ""
				if initiator != nil {
					uid, username = &initiator.ID, initiator.Username
				}
				task, e = s.queue.AddTask(task, uid, username, p, template.App.NeedTaskAlias())
				if e != nil {
					return s.failLaunch(&run, e)
				}
				if run.RootTaskID == nil {
					if _, err = s.repo.SetWorkflowRunRootTask(p, runID, task.ID); err != nil {
						return err
					}
					run.RootTaskID = &task.ID
				}
				activeTasks++
			case db.WorkflowNodeApprovalKind:
				if _, err = s.repo.CreateWorkflowApproval(db.WorkflowApproval{ProjectID: p, WorkflowRunID: runID, WorkflowNodeID: node.ID}); err != nil {
					return err
				}
				waitingApprovals++
			case db.WorkflowNodeDelayKind:
				if node.DelaySeconds == nil {
					return s.failLaunch(&run, common_errors.NewValidationError("workflow delay is missing"))
				}
				if _, err = s.repo.CreateWorkflowDelay(db.WorkflowDelay{ProjectID: p, WorkflowRunID: runID, WorkflowNodeID: node.ID, ResumeAt: tz.Now().Add(time.Duration(*node.DelaySeconds) * time.Second)}); err != nil {
					return err
				}
				waitingDelays++
			}
		}
		failed = failed || states[node.ID] == workflowFailure
	}
	if activeTasks+waitingApprovals+waitingDelays == 0 {
		status := db.WorkflowRunSuccess
		if failed {
			status = db.WorkflowRunFailed
		}
		return s.finish(&run, status, nil)
	}
	run.Status = db.WorkflowRunRunning
	if activeTasks+waitingDelays == 0 && waitingApprovals > 0 {
		run.Status = db.WorkflowRunApproval
	}
	_, err = s.repo.UpdateWorkflowRunStatusUnless(run, workflowFinished)
	return err
}
func (s *workflowService) finish(run *db.WorkflowRun, status db.WorkflowRunStatus, user *db.User) error {
	run.Status, run.End = status, new(tz.Now())
	changed, err := s.repo.UpdateWorkflowRunStatusUnless(*run, workflowFinished)
	if err == nil && changed {
		s.record(audit.WorkflowComplete, *run, 0, user)
	}
	return err
}
func (s *workflowService) failLaunch(run *db.WorkflowRun, cause error) error {
	if err := s.finish(run, db.WorkflowRunFailed, nil); err != nil {
		return err
	}
	s.queue.StopTasksByWorkflowRun(run.ProjectID, run.ID, false)
	return cause
}
func (s *workflowService) StopWorkflowRun(p, runID int, user *db.User) (db.WorkflowRun, error) {
	defer s.lock(runID)()
	run, err := s.repo.GetWorkflowRunByID(p, runID)
	if err != nil || run.Status.IsFinished() {
		return run, err
	}
	if err = s.finish(&run, db.WorkflowRunStopped, user); err != nil {
		return run, err
	}
	s.queue.StopTasksByWorkflowRun(p, runID, false)
	approvals, err := s.repo.GetWorkflowApprovals(p, runID)
	if err != nil {
		return run, err
	}
	for _, item := range approvals {
		if item.Status == db.WorkflowApprovalPending {
			item.Status = db.WorkflowApprovalRejected
			if user != nil {
				item.ResolvedByUserID = &user.ID
			}
			if _, err = s.repo.ResolveWorkflowApprovalIfPending(item); err != nil {
				return run, err
			}
		}
	}
	delays, err := s.repo.GetWorkflowDelays(p, runID)
	if err != nil {
		return run, err
	}
	for _, item := range delays {
		if item.Status == db.WorkflowDelayWaiting {
			item.Status = db.WorkflowDelayStopped
			if _, err = s.repo.ResolveWorkflowDelayIfWaiting(item); err != nil {
				return run, err
			}
		}
	}
	s.record(audit.WorkflowStop, run, 0, user)
	return run, nil
}
func (s *workflowService) ResolveWorkflowApproval(p, workflowID, runID, nodeID int, status db.WorkflowApprovalStatus, user *db.User) (db.WorkflowApproval, error) {
	unlock := s.lock(runID)
	run, err := s.repo.GetWorkflowRun(p, workflowID, runID)
	if err != nil {
		unlock()
		return db.WorkflowApproval{}, err
	}
	if run.Status.IsFinished() {
		unlock()
		return db.WorkflowApproval{}, db.ErrInvalidOperation
	}
	item, err := s.repo.GetWorkflowApproval(p, runID, nodeID)
	if err != nil {
		unlock()
		return item, err
	}
	graph, err := s.repo.GetWorkflowRevisionGraph(p, run.RevisionID)
	if err != nil {
		unlock()
		return item, err
	}
	for _, node := range graph.Nodes {
		if node.ID == nodeID && node.ApprovalTimeout != nil && !tz.Now().Before(item.Created.Add(time.Duration(*node.ApprovalTimeout)*time.Second)) {
			unlock()
			if err = s.ProgressWorkflowRun(p, runID, nil); err != nil {
				return item, err
			}
			return item, db.ErrInvalidOperation
		}
	}
	item.Status = status
	if user != nil {
		item.ResolvedByUserID = &user.ID
	}
	changed, err := s.repo.ResolveWorkflowApprovalIfPending(item)
	if err == nil && !changed {
		err = db.ErrInvalidOperation
	}
	if err == nil {
		kind := audit.WorkflowApprovalApprove
		if status == db.WorkflowApprovalRejected {
			kind = audit.WorkflowApprovalReject
		}
		s.record(kind, run, nodeID, user)
	}
	unlock()
	if err != nil {
		return item, err
	}
	if err = s.ProgressWorkflowRun(p, runID, user); err != nil {
		return item, err
	}
	return s.repo.GetWorkflowApproval(p, runID, nodeID)
}
func (s *workflowService) HandleWorkflowTaskCompletion(task db.Task) error {
	if task.WorkflowRunID == nil {
		return nil
	}
	return s.ProgressWorkflowRun(task.ProjectID, *task.WorkflowRunID, nil)
}
func mergeWorkflowArtifacts(dest, source map[string]any) {
	for key, value := range source {
		if nested, ok := value.(map[string]any); ok {
			if existing, ok := dest[key].(map[string]any); ok {
				mergeWorkflowArtifacts(existing, nested)
				continue
			}
		}
		dest[key] = value
	}
}
func (s *workflowService) GetWorkflowRunArtifacts(p, runID int, currentTaskID *int) (map[string]any, error) {
	run, err := s.repo.GetWorkflowRunByID(p, runID)
	if err != nil {
		return nil, err
	}
	graph, err := s.repo.GetWorkflowRevisionGraph(p, run.RevisionID)
	if err != nil {
		return nil, err
	}
	ancestors := map[int]bool{}
	if currentTaskID != nil {
		task, e := s.store.GetTask(p, *currentTaskID)
		if e != nil {
			return nil, e
		}
		if task.WorkflowRunID == nil || *task.WorkflowRunID != runID || task.WorkflowNodeID == nil {
			return nil, db.ErrNotFound
		}
		var visit func(int)
		visit = func(nodeID int) {
			for _, edge := range graph.Edges {
				if edge.DestinationNodeID == nodeID && !ancestors[edge.SourceNodeID] {
					ancestors[edge.SourceNodeID] = true
					visit(edge.SourceNodeID)
				}
			}
		}
		visit(*task.WorkflowNodeID)
	}
	tasks, err := s.repo.GetWorkflowRunTasks(p, runID, db.RetrieveQueryParams{})
	if err != nil {
		return nil, err
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	result := map[string]any{}
	for _, task := range tasks {
		if !task.Status.IsFinished() || task.End == nil || task.Artifacts == nil || task.WorkflowNodeID == nil || currentTaskID != nil && !ancestors[*task.WorkflowNodeID] {
			continue
		}
		var artifact map[string]any
		if err = json.Unmarshal([]byte(*task.Artifacts), &artifact); err != nil {
			return nil, common_errors.NewValidationError("workflow task artifacts must be JSON objects")
		}
		mergeWorkflowArtifacts(result, artifact)
	}
	return result, nil
}

type WorkflowReconciler struct {
	repo    db.WorkflowManager
	service WorkflowService
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func NewWorkflowReconciler(repo db.WorkflowManager, service WorkflowService) *WorkflowReconciler {
	return &WorkflowReconciler{repo: repo, service: service, stop: make(chan struct{}), done: make(chan struct{})}
}
func (r *WorkflowReconciler) Start() {
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			runs, err := r.repo.GetActiveWorkflowRuns()
			if err != nil {
				log.WithError(err).Error("cannot load active workflows")
			}
			for _, run := range runs {
				if err = r.service.ProgressWorkflowRun(run.ProjectID, run.ID, nil); err != nil {
					log.WithError(err).WithField("run_id", run.ID).Error("cannot progress workflow")
				}
			}
			select {
			case <-r.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}
func (r *WorkflowReconciler) Stop() { r.once.Do(func() { close(r.stop) }); <-r.done }
