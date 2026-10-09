package containerworker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/impishMD/taskexec/pkg/task_logger"
)

type logger struct {
	mu              sync.Mutex
	encoder         *json.Encoder
	status          task_logger.TaskStatus
	statusListeners []task_logger.StatusListener
	logListeners    []task_logger.LogListener
	err             error
}

func (l *logger) emit(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		l.err = l.encoder.Encode(e)
	}
}
func (l *logger) Log(s string)            { l.LogWithTime(time.Now(), s) }
func (l *logger) Logf(f string, a ...any) { l.Log(fmt.Sprintf(f, a...)) }
func (l *logger) LogfWithTime(t time.Time, f string, a ...any) {
	l.LogWithTime(t, fmt.Sprintf(f, a...))
}
func (l *logger) LogWithTime(t time.Time, s string) {
	l.emit(Event{Type: "log", Time: t, Message: s})
	l.mu.Lock()
	listeners := append([]task_logger.LogListener(nil), l.logListeners...)
	l.mu.Unlock()
	for _, f := range listeners {
		f(t, s)
	}
}
func (l *logger) SetCommit(hash, message string) {
	l.emit(Event{Type: "commit", CommitHash: hash, CommitMessage: message})
}
func (l *logger) AddLogListener(f task_logger.LogListener) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logListeners = append(l.logListeners, f)
}
func (l *logger) AddStatusListener(f task_logger.StatusListener) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.statusListeners = append(l.statusListeners, f)
}
func (l *logger) SetStatus(s task_logger.TaskStatus) {
	l.mu.Lock()
	if l.status == s {
		l.mu.Unlock()
		return
	}
	l.status = s
	listeners := append([]task_logger.StatusListener(nil), l.statusListeners...)
	l.mu.Unlock()
	// Set the app's confirmation state before the runner can respond to it.
	for _, f := range listeners {
		f(s)
	}
	l.emit(Event{Type: "status", Status: s})
}
func (l *logger) LogCmd(cmd *exec.Cmd) func() {
	r, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer r.Close()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			l.Log(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			l.Logf("Failed to read command output: %v", err)
		}
	}()
	return sync.OnceFunc(func() { _ = w.Close(); <-done })
}
