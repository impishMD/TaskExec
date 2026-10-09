package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/util"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type testLogger struct {
	task_logger.NopLogger
	mu       sync.Mutex
	logs     []string
	statuses []task_logger.TaskStatus
	commit   string
	onStatus func(task_logger.TaskStatus)
	ready    chan struct{}
	once     sync.Once
}

func (l *testLogger) Log(s string)            { l.LogWithTime(time.Now(), s) }
func (l *testLogger) Logf(f string, a ...any) { l.Log(fmt.Sprintf(f, a...)) } // lifecycle diagnostics aren't fixture output
func (l *testLogger) LogWithTime(_ time.Time, s string) {
	l.mu.Lock()
	l.logs = append(l.logs, s)
	l.mu.Unlock()
	if l.ready != nil && strings.Contains(s, "WAITING_FOR_STOP") {
		l.once.Do(func() { close(l.ready) })
	}
}
func (l *testLogger) SetCommit(hash, _ string) { l.mu.Lock(); defer l.mu.Unlock(); l.commit = hash }
func (l *testLogger) SetStatus(s task_logger.TaskStatus) {
	l.mu.Lock()
	l.statuses = append(l.statuses, s)
	l.mu.Unlock()
	if l.onStatus != nil {
		l.onStatus(s)
	}
}

func TestProviderValidation(t *testing.T) {
	for _, cfg := range []util.RunnerDockerConfig{{PullPolicy: "maybe"}, {Image: "bad image"}, {CPULimit: -1}, {MemoryLimit: "oops"}, {CleanupGraceSeconds: -1}} {
		_, err := NewProvider(cfg)
		require.Error(t, err)
	}
}
func TestDockerPayloadPreservesPrivateRelations(t *testing.T) {
	p, err := NewProvider(util.RunnerDockerConfig{})
	require.NoError(t, err)
	defer p.Close()
	key := db.AccessKey{ID: 5, Type: db.AccessKeySSH, SshKey: db.SshKey{PrivateKey: "fixture-key"}}
	tpl := db.Template{Vaults: []db.TemplateVault{{Vault: &key}}}
	inv := db.Inventory{SSHKey: key, BecomeKey: key, Repository: &db.Repository{SSHKey: key}}
	x, err := p.NewExecutor(db.Task{Secret: `{"secret":"fixture"}`}, tpl, inv, db.Repository{SSHKey: key}, db.Environment{}, "task-jwt", []db.HostConfig{{SSHKey: key}})
	require.NoError(t, err)
	e := x.(*Executor)
	defer e.Cleanup()
	data, err := json.Marshal(e.request)
	require.NoError(t, err)
	req := e.request
	require.NoError(t, json.Unmarshal(data, &req))
	req.Hydrate()
	require.Equal(t, key, req.Repository.SSHKey)
	require.Equal(t, key, req.Inventory.SSHKey)
	require.Equal(t, key, req.Inventory.BecomeKey)
	require.Equal(t, key, req.Inventory.Repository.SSHKey)
	require.Equal(t, key, *req.Template.Vaults[0].Vault)
	require.Equal(t, key, req.HostConfigs[0].SSHKey)
	require.Equal(t, "task-jwt", req.JWT)
	require.Equal(t, `{"secret":"fixture"}`, req.Task.Secret)
}

// Opt in with TASKEXEC_DOCKER_TEST_IMAGE built from testdata/Dockerfile.
// No production containers, repositories or database are used.
func TestDockerRealLifecycle(t *testing.T) {
	image := os.Getenv("TASKEXEC_DOCKER_TEST_IMAGE")
	if image == "" {
		t.Skip("set TASKEXEC_DOCKER_TEST_IMAGE to the fixture image")
	}
	p, err := NewProvider(util.RunnerDockerConfig{Image: image, PullPolicy: "never", CPULimit: 1, MemoryLimit: "512m", CleanupGraceSeconds: 2})
	require.NoError(t, err)
	defer p.Close()
	create := func(t *testing.T, app db.TemplateApp, playbook string) (*Executor, *testLogger) {
		t.Helper()
		env := `{"CONTAINER_TEST_SECRET":"fixture-only-secret"}`
		task := db.Task{ID: 101, ProjectID: 202, TemplateID: 303}
		template := db.Template{ID: 303, ProjectID: 202, App: app, Type: db.TemplateTask, Playbook: playbook}
		inv := db.Inventory{Type: db.InventoryStatic, Inventory: "localhost ansible_connection=local", SSHKey: db.AccessKey{Type: db.AccessKeyNone}}
		if app.IsTerraform() {
			inv.Type = db.InventoryTerraformWorkspace
			inv.Inventory = "default"
		}
		repo := db.Repository{ID: 404, ProjectID: 202, GitURL: "file:///opt/taskexec-test-repo", GitBranch: "main", SSHKey: db.AccessKey{Type: db.AccessKeyNone}}
		x, err := p.NewExecutor(task, template, inv, repo, db.Environment{JSON: "{}", ENV: &env}, "", nil)
		require.NoError(t, err)
		l := &testLogger{}
		x.SetLogger(l)
		e := x.(*Executor)
		t.Cleanup(func() {
			e.Kill()
			e.Cleanup()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := p.client.ContainerInspect(ctx, e.name, client.ContainerInspectOptions{})
			require.True(t, errdefs.IsNotFound(err), "container must be removed: %v", err)
		})
		return e, l
	}
	t.Run("shell_success_and_metadata", func(t *testing.T) {
		e, l := create(t, db.AppBash, "success.sh")
		require.NoError(t, e.Prepare("test", nil, ""))
		require.NoError(t, e.Prepare("test", nil, ""))
		inspect, err := p.client.ContainerInspect(context.Background(), e.id, client.ContainerInspectOptions{})
		require.NoError(t, err)
		data, err := json.Marshal(inspect)
		require.NoError(t, err)
		require.NotContains(t, string(data), "fixture-only-secret")
		require.Equal(t, int64(1e9), inspect.Container.HostConfig.NanoCPUs)
		require.Equal(t, int64(512*1024*1024), inspect.Container.HostConfig.Memory)
		require.Empty(t, inspect.Container.Mounts)
		require.NoError(t, e.Run("test", nil, ""))
		require.Contains(t, strings.Join(l.logs, "\n"), "CONTAINER_SUCCESS")
		require.NotEmpty(t, l.commit)
	})
	t.Run("template_image_override", func(t *testing.T) {
		copyProvider := *p
		copyProvider.config.Image = "taskexec-test-missing:never"
		template := db.Template{ExecutorImage: &image}
		x, err := copyProvider.NewExecutor(db.Task{}, template, db.Inventory{}, db.Repository{}, db.Environment{}, "", nil)
		require.NoError(t, err)
		e := x.(*Executor)
		defer e.Cleanup()
		require.Equal(t, image, e.image)
		require.NoError(t, e.Prepare("test", nil, ""))
	})
	t.Run("connection_loss", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "stop.sh")
		require.NoError(t, e.Prepare("test", nil, ""))
		attach, err := p.client.ContainerAttach(context.Background(), e.id, client.ContainerAttachOptions{Stream: true, Stdin: true, Stdout: true, Stderr: true})
		require.NoError(t, err)
		_, err = p.client.ContainerStart(context.Background(), e.id, client.ContainerStartOptions{})
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(attach.Conn).Encode(e.request))
		// Closing only the input simulates a lost runner. No stop/remove call is
		// allowed until the worker has exited and Docker has auto-removed it.
		require.NoError(t, attach.CloseWrite())
		defer attach.Close()
		require.Eventually(t, func() bool {
			_, err := p.client.ContainerInspect(context.Background(), e.id, client.ContainerInspectOptions{})
			return errdefs.IsNotFound(err)
		}, 8*time.Second, 100*time.Millisecond)
	})
	t.Run("start_failure_cleanup", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "success.sh")
		copyProvider := *p
		copyProvider.config.Network = "taskexec-test-missing-network"
		e.provider = &copyProvider
		require.Error(t, e.Run("test", nil, ""))
	})
	t.Run("failure", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "fail.sh")
		require.ErrorContains(t, e.Run("test", nil, ""), "17")
	})
	t.Run("ansible_vault", func(t *testing.T) {
		e, l := create(t, db.AppAnsible, "vault-playbook.yml")
		key := db.AccessKey{Type: db.AccessKeyLoginPassword, LoginPassword: db.LoginPassword{Password: "fixture-vault-password"}}
		e.request.Template.Vaults = []db.TemplateVault{{Type: db.TemplateVaultPassword}}
		e.request.VaultKeys = []*db.AccessKey{&key}
		require.NoError(t, e.Run("test", nil, ""))
		output := strings.Join(l.logs, "\n")
		require.Contains(t, output, "PLAY RECAP")
		require.NotContains(t, output, "fixture-vault-password")
	})
	t.Run("ansible", func(t *testing.T) {
		e, l := create(t, db.AppAnsible, "ansible.yml")
		require.NoError(t, e.Run("test", nil, ""))
		require.Contains(t, strings.Join(l.logs, "\n"), "PLAY RECAP")
	})
	for _, decision := range []task_logger.TaskStatus{task_logger.TaskConfirmed, task_logger.TaskRejected} {
		t.Run("terraform_"+string(decision), func(t *testing.T) {
			e, l := create(t, db.AppTerraform, "terraform")
			l.onStatus = func(s task_logger.TaskStatus) {
				if s == task_logger.TaskWaitingConfirmation {
					e.SetStatus(decision)
				}
			}
			err := e.Run("test", nil, "")
			require.Contains(t, l.statuses, task_logger.TaskWaitingConfirmation)
			if decision == task_logger.TaskConfirmed {
				require.NoError(t, err)
				require.Contains(t, strings.Join(l.logs, "\n"), "Apply complete!")
			} else {
				require.Error(t, err)
				require.NotContains(t, strings.Join(l.logs, "\n"), "Apply complete!")
			}
		})
	}
	t.Run("stop", func(t *testing.T) {
		e, l := create(t, db.AppBash, "stop.sh")
		l.ready = make(chan struct{})
		done := make(chan error, 1)
		go func() { done <- e.Run("test", nil, "") }()
		select {
		case <-l.ready:
		case <-time.After(30 * time.Second):
			t.Fatal("task never started")
		}
		e.Kill()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("stop did not finish")
		}
		require.Contains(t, l.statuses, task_logger.TaskStoppedStatus)
		require.NotContains(t, strings.Join(l.logs, "\n"), "SHOULD_NOT_RUN")
	})
	t.Run("stop_before_start", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "stop.sh")
		e.Kill()
		require.NoError(t, e.Run("test", nil, ""))
		require.Empty(t, e.id)
	})
	t.Run("invalid_image", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "success.sh")
		e.image = "invalid image"
		require.Error(t, e.Run("test", nil, ""))
		require.Empty(t, e.id)
	})
	t.Run("missing_image", func(t *testing.T) {
		e, _ := create(t, db.AppBash, "success.sh")
		e.image = "taskexec-test-missing:never"
		require.ErrorContains(t, e.Run("test", nil, ""), "pull_policy=never")
	})
	t.Run("parallel", func(t *testing.T) {
		first, _ := create(t, db.AppBash, "success.sh")
		second, _ := create(t, db.AppBash, "success.sh")
		results := make(chan error, 2)
		go func() { results <- first.Run("test", nil, "") }()
		go func() { results <- second.Run("test", nil, "") }()
		require.NoError(t, <-results)
		require.NoError(t, <-results)
		require.NotEqual(t, first.id, second.id)
	})
}
