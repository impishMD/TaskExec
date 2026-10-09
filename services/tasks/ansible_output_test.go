package tasks

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnsibleOutput_PersistsRecapErrorsStagesAndLogs(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	f.pool.ansibleTaskRepo = f.store
	runner := &TaskRunner{Task: f.task, Template: db.Template{App: db.AppAnsible}}
	at := time.Now().UTC().Truncate(time.Second)
	lines := []string{
		"Preparing task",
		"\x1b[1;34mPLAY [All hosts] ************************\x1b[0m",
		"TASK [Install package] ************************",
		`fatal: [broken -> localhost]: FAILED! => {`,
		`    "changed": false,`,
		`    "msg": "Package not found"`,
		`}`,
		"TASK [Hidden operation] ************************",
		`failed: [hidden] (item=None) => {"censored": "output hidden because no_log is set", "changed": false}`,
		"...ignoring",
		"TASK [Connect] ************************",
		`fatal: [offline]: UNREACHABLE! => {"msg": "Connection refused", "unreachable": true}`,
		"PLAY RECAP ************************",
		"healthy : ok=3 changed=1 unreachable=0 failed=0 skipped=2 rescued=0 ignored=0",
		"broken : ok=1 changed=0 unreachable=0 failed=1 skipped=0 rescued=0 ignored=0",
		"hidden : ok=2 changed=0 unreachable=0 failed=0 skipped=0 rescued=0 ignored=1",
		"offline : ok=0 changed=0 unreachable=1 failed=0 skipped=0 rescued=0 ignored=0",
	}
	for i, line := range lines {
		f.pool.writeLogs([]logRecord{{task: runner, output: line, time: at.Add(time.Duration(i) * time.Second)}})
	}
	end := at.Add(time.Minute)
	require.NoError(t, runner.ansibleOutput.finish(end))
	require.NoError(t, runner.ansibleOutput.finish(end), "completion is idempotent")
	hosts, err := f.store.GetAnsibleTaskHosts(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, hosts, 4)
	assert.Equal(t, "broken", hosts[0].Host)
	assert.Equal(t, 1, hosts[0].Failed)
	assert.Equal(t, 3, hosts[1].Ok)
	assert.Equal(t, 1, hosts[1].Changed)
	assert.Equal(t, 1, hosts[2].Ignored)
	errors, err := f.store.GetAnsibleTaskErrors(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, errors, 3)
	assert.Equal(t, "broken", errors[0].Host)
	assert.Equal(t, "Install package", errors[0].Task)
	assert.Contains(t, errors[0].Error, "Package not found")
	assert.Contains(t, errors[1].Error, "censored")
	assert.Equal(t, "offline", errors[2].Host)
	stages, err := f.store.GetTaskStages(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, stages, 3)
	for i, kind := range []db.TaskStageType{db.TaskStageInit, db.TaskStageRunning, db.TaskStagePrintResult} {
		assert.Equal(t, kind, stages[i].Type)
		require.NotNil(t, stages[i].End)
		output, err := f.store.GetTaskStageOutputs(f.task.ProjectID, f.task.ID, stages[i].ID)
		require.NoError(t, err)
		assert.NotEmpty(t, output)
	}
	output, err := f.store.GetTaskOutputs(f.task.ProjectID, f.task.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	assert.Len(t, output, len(lines))
	assert.JSONEq(t, `{"hosts":4,"ok":6,"changed":1,"failed":1,"unreachable":1,"skipped":2,"rescued":0,"ignored":1}`, stages[2].JSON)
}

func TestAnsibleOutput_DefaultYAMLCallback(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	p := ansibleOutput{store: f.store, repo: f.store, projectID: f.task.ProjectID, taskID: f.task.ID}
	at := time.Now().UTC()
	// Current Ansible versions print diagnostic context before the fatal line,
	// and put the censored result on indented YAML lines rather than inline JSON.
	for _, line := range []string{
		"TASK [Hidden failure] *****************",
		"[ERROR]: Task failed: Module failed: non-zero return code",
		"Origin: /tmp/playbook.yml:5:7",
		"fatal: [host]: FAILED! => ",
		"    censored: 'the output has been hidden due to the fact that ''no_log: true'' was specified",
		"        for this result'",
		"    changed: false",
		"...ignoring",
	} {
		require.NoError(t, p.consume(db.TaskOutput{Time: at, Output: line}))
	}
	require.NoError(t, p.finish(at))
	items, err := f.store.GetAnsibleTaskErrors(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Hidden failure", items[0].Task)
	assert.Contains(t, items[0].Error, "censored:")
	assert.Contains(t, items[0].Error, "for this result'")
	assert.NotContains(t, items[0].Error, "Origin:")
}

func TestAnsibleOutput_OtherAppsDoNotCreateSummary(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	f.pool.ansibleTaskRepo = f.store
	runner := &TaskRunner{Task: f.task, Template: db.Template{App: db.AppBash}}
	f.pool.writeLogs([]logRecord{{task: runner, output: "PLAY RECAP ********", time: time.Now().UTC()}})
	assert.Nil(t, runner.ansibleOutput)
	stages, err := f.store.GetTaskStages(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	assert.Empty(t, stages)
}

func TestParseAnsibleRecap(t *testing.T) {
	for _, tc := range []struct {
		line string
		ok   bool
	}{
		{"host : ok=3 changed=1 unreachable=0 failed=0", true},
		{"2001:db8::1 : ok=0 changed=0 unreachable=1 failed=0 skipped=0 rescued=0 ignored=0", true},
		{"host : ok=1 changed=0 unreachable=0 failed=0 ignored=0 rescued=2", true},
		{"host : ok=1 changed=0", false},
		{"host : ok=1 ok=2 changed=0 unreachable=0 failed=0", false},
		{"host : ok=99999999999999999999 changed=0 unreachable=0 failed=0", false},
		{"host : ok=-1 changed=0 unreachable=0 failed=0", false},
		{"not a recap", false},
	} {
		t.Run(tc.line, func(t *testing.T) { _, ok := parseAnsibleRecap(tc.line); assert.Equal(t, tc.ok, ok) })
	}
}

func TestAnsibleOutput_FailureBeforeRecapAndTruncation(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	p := ansibleOutput{store: f.store, repo: f.store, projectID: f.task.ProjectID, taskID: f.task.ID}
	at := time.Now().UTC()
	require.NoError(t, p.consume(db.TaskOutput{Time: at, Output: "TASK [Connect] ********\nfailed: [host] => " + strings.Repeat("Ошибка", 400)}))
	require.NoError(t, p.finish(at))
	items, err := f.store.GetAnsibleTaskErrors(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Len(t, []rune(items[0].Error), 1000)
	assert.True(t, strings.HasSuffix(items[0].Error, "…"))
	hosts, err := f.store.GetAnsibleTaskHosts(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.AnsibleTaskHost{}, hosts)
}

type failingAnsibleRepository struct{ db.AnsibleTaskRepository }

func (f failingAnsibleRepository) CreateAnsibleTaskHost(db.AnsibleTaskHost) error {
	return errors.New("summary storage unavailable")
}

func TestAnsibleOutput_ParserFailureDoesNotDiscardLogBatch(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	f.pool.ansibleTaskRepo = failingAnsibleRepository{f.store}
	runner := &TaskRunner{Task: f.task, Template: db.Template{App: db.AppAnsible}}
	var logs []logRecord
	for i, line := range []string{"PLAY RECAP ********", "host : ok=1 changed=0 unreachable=0 failed=0", "The rest of the batch"} {
		logs = append(logs, logRecord{task: runner, time: time.Unix(int64(i), 0).UTC(), output: line})
	}
	f.pool.writeLogs(logs)
	output, err := f.store.GetTaskOutputs(f.task.ProjectID, f.task.ID, db.RetrieveQueryParams{})
	require.NoError(t, err)
	assert.Len(t, output, 3)
}

func TestAnsibleOutput_FinalizationBarrier(t *testing.T) {
	f := newTaskRunnerRunFixture(t)
	defer f.store.Close()
	f.pool.ansibleTaskRepo = f.store
	runner := &TaskRunner{Task: f.task, Template: db.Template{App: db.AppAnsible}, pool: &f.pool}
	go f.pool.handleLogs()
	<-f.pool.logsReady
	t.Cleanup(func() { close(f.pool.stop); <-f.pool.logsDone })
	runner.Log("TASK [Last task] ********")
	runner.Log(`fatal: [host]: FAILED! => {"msg":"Last line"}`)
	f.pool.finishTaskLogs(runner, time.Now().UTC())
	items, err := f.store.GetAnsibleTaskErrors(f.task.ProjectID, f.task.ID)
	require.NoError(t, err)
	require.Len(t, items, 1, "the last failure is stored before completion is published")
	assert.Contains(t, items[0].Error, "Last line")
}
