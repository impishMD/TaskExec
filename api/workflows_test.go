package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
	"github.com/stretchr/testify/require"
)

type approvalOnlyQueue struct{}

func (approvalOnlyQueue) AddTask(db.Task, *int, string, int, bool) (db.Task, error) {
	panic("approval graph cannot launch tasks")
}
func (approvalOnlyQueue) StopTasksByWorkflowRun(int, int, bool) {}

func TestWorkflowAPIIsolationPermissionsAndPinnedGraph(t *testing.T) {
	f := newCoreFixture(t)
	svc := server.NewWorkflowService(f.store, f.store, approvalOnlyQueue{}, nil)
	f.router = Route(f.store, nil, f.store, f.store, nil, nil, nil, nil, nil, nil, nil, nil, nil, server.NewRunnerService(f.store), svc, nil)
	base := fmt.Sprintf("/project/%d/workflows", f.project.ID)
	body := fmt.Sprintf(`{"project_id":%d,"name":"approval","nodes":[{"id":-1,"kind":"approval","approval_message":"Original"}],"edges":[]}`, f.other.ID)
	f.request(t, "guest", "POST", base, body, 403)
	f.request(t, "outsider", "GET", base, "", 404)
	w := f.request(t, "manager", "POST", base, body, 201)
	var graph db.WorkflowTemplate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &graph))
	require.Equal(t, f.project.ID, graph.ProjectID)
	path := fmt.Sprintf("%s/%d", base, graph.ID)
	f.request(t, "guest", "GET", path, "", 200)
	f.request(t, "guest", "PUT", path, body, 403)
	f.request(t, "guest", "POST", path+"/runs", "", 403)
	w = f.request(t, "manager", "POST", path+"/runs", "", 201)
	var run db.WorkflowRun
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &run))
	require.Equal(t, db.WorkflowRunApproval, run.Status)
	require.NotNil(t, run.StartedByUserID)
	user, err := f.store.GetUser(*run.StartedByUserID)
	require.NoError(t, err)
	require.Equal(t, "manager", user.Username)
	runPath := fmt.Sprintf("%s/runs/%d", path, run.ID)
	f.request(t, "guest", "GET", runPath, "", 200)
	f.request(t, "manager", "DELETE", path, "", 409)
	f.request(t, "manager", "PUT", path, `{"name":"changed","nodes":[{"id":-1,"kind":"approval","approval_message":"New"}],"edges":[]}`, 204)
	w = f.request(t, "guest", "GET", runPath, "", 200)
	require.Contains(t, w.Body.String(), "Original")
	require.NotContains(t, w.Body.String(), `"New"`)
	other, err := f.store.CreateWorkflowTemplate(db.WorkflowTemplate{ProjectID: f.other.ID, Name: "other", Nodes: []db.WorkflowNode{{ID: 1, Kind: db.WorkflowNodeApprovalKind}}})
	require.NoError(t, err)
	f.request(t, "admin", "GET", fmt.Sprintf("%s/revisions/%d", path, other.RevisionID), "", 404)
	f.request(t, "admin", "GET", fmt.Sprintf("/project/%d/workflows/%d/runs/%d", f.other.ID, other.ID, run.ID), "", 404)
	approvalPath := fmt.Sprintf("%s/approvals/%d", runPath, graph.Nodes[0].ID)
	f.request(t, "guest", "POST", approvalPath, `{"status":"approved"}`, 403)
	f.request(t, "manager", "POST", approvalPath, `{"status":"invalid"}`, 400)
	f.request(t, "manager", "POST", approvalPath, `{"status":"approved"}`, 200)
	f.request(t, "manager", "POST", approvalPath, `{"status":"approved"}`, 409)
	current, err := f.store.GetWorkflowRunByID(f.project.ID, run.ID)
	require.NoError(t, err)
	require.Equal(t, db.WorkflowRunSuccess, current.Status)
	f.request(t, "manager", "DELETE", path, "", 204)
	f.request(t, "guest", "GET", runPath, "", 404)
}
