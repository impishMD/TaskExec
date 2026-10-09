package docker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

// Real SDK requests against a test Engine pin pull-stream errors and cancellation
// without accessing a registry or downloading an image in ordinary unit tests.
func TestImagePullFailureAndCancellation(t *testing.T) {
	for _, cancelPull := range []bool{false, true} {
		name := "stream_error"
		if cancelPull {
			name = "cancel"
		}
		t.Run(name, func(t *testing.T) {
			pulling := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/json"):
					http.Error(w, `{"message":"No such image"}`, 404)
				case strings.HasSuffix(r.URL.Path, "/images/create"):
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					if cancelPull {
						w.(http.Flusher).Flush()
						close(pulling)
						<-r.Context().Done()
					} else {
						_, _ = w.Write([]byte("{\"errorDetail\":{\"message\":\"registry denied fixture\"}}\n"))
					}
				default:
					http.Error(w, "unexpected endpoint", 500)
				}
			}))
			defer server.Close()
			cli, err := client.New(client.WithHost(server.URL), client.WithAPIVersion("1.44"))
			require.NoError(t, err)
			defer cli.Close()
			p := &Provider{client: cli, config: util.RunnerDockerConfig{Image: "test/image:latest", PullPolicy: "always"}}
			x, err := p.NewExecutor(db.Task{}, db.Template{}, db.Inventory{}, db.Repository{}, db.Environment{}, "", nil)
			require.NoError(t, err)
			e := x.(*Executor)
			if cancelPull {
				done := make(chan error, 1)
				go func() { done <- e.Run("", nil, "") }()
				select {
				case <-pulling:
				case <-time.After(5 * time.Second):
					t.Fatal("pull did not start")
				}
				e.Kill()
				select {
				case err := <-done:
					require.NoError(t, err)
				case <-time.After(5 * time.Second):
					t.Fatal("pull was not cancelled")
				}
			} else {
				require.ErrorContains(t, e.Run("", nil, ""), "registry denied fixture")
			}
			require.False(t, e.createAttempted)
		})
	}
}
