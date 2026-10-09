package tasks

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/impishMD/taskexec/db"
)

var (
	ansibleANSI    = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansibleBanner  = regexp.MustCompile(`^(PLAY|TASK|RUNNING HANDLER) \[(.*)\] \*+\s*$`)
	ansibleFailure = regexp.MustCompile(`^(?:fatal|failed): \[([^\]]+)\].*? => (.*)$`)
	ansibleCounter = regexp.MustCompile(`\b(ok|changed|unreachable|failed|skipped|rescued|ignored)=(\d+)\b`)
)

// ansibleOutput consumes the default Ansible stdout callback, including its
// JSON/YAML result formats. It only sees text already emitted to the task log;
// Ansible's no_log censorship is preserved. State belongs to handleLogs alone.
type ansibleOutput struct {
	store             db.TaskManager
	repo              db.AnsibleTaskRepository
	projectID, taskID int
	stage             *db.TaskStage
	taskName          string
	recap             bool
	pending           *db.AnsibleTaskError
	hosts             map[string]db.AnsibleTaskHost
	closed            bool
}

func (p *ansibleOutput) consume(output db.TaskOutput) error {
	if p.closed {
		return nil
	}
	var errs []error
	if p.stage == nil {
		errs = append(errs, p.transition(db.TaskStageInit, output.Time))
	}
	// LogCmd supplies complete lines; a remote runner can batch several in a
	// single record. Never carry an incomplete record into the next log line.
	for _, raw := range strings.Split(output.Output, "\n") {
		line := strings.TrimRight(ansibleANSI.ReplaceAllString(raw, ""), "\r")
		if p.pending != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || line == "}") {
			p.pending.Error = summaryText(p.pending.Error+"\n"+line, 1000)
			continue
		}
		errs = append(errs, p.flushError())
		if strings.HasPrefix(line, "PLAY RECAP ") && strings.Contains(line, "***") {
			p.recap = true
			errs = append(errs, p.transition(db.TaskStagePrintResult, output.Time))
			continue
		}
		if match := ansibleBanner.FindStringSubmatch(line); match != nil {
			p.recap = false
			if match[1] != "PLAY" {
				p.taskName = summaryText(match[2], 250)
			}
			errs = append(errs, p.transition(db.TaskStageRunning, output.Time))
			continue
		}
		if p.recap {
			if host, ok := parseAnsibleRecap(line); ok {
				host.TaskID, host.ProjectID, host.Created = p.taskID, p.projectID, &output.Time
				if err := p.repo.CreateAnsibleTaskHost(host); err != nil {
					errs = append(errs, err)
				} else {
					if p.hosts == nil {
						p.hosts = make(map[string]db.AnsibleTaskHost)
					}
					p.hosts[host.Host] = host
				}
			}
			continue
		}
		if match := ansibleFailure.FindStringSubmatch(line); match != nil {
			// Delegation prints [inventory-host -> delegated-host]. Recap and
			// errors must identify the same inventory host.
			host := strings.SplitN(match[1], " -> ", 2)[0]
			p.pending = &db.AnsibleTaskError{
				ProjectID: p.projectID, TaskID: p.taskID, Created: &output.Time,
				Host: summaryText(host, 250), Task: p.taskName, Error: summaryText(match[2], 1000),
			}
		}
	}
	return errors.Join(errs...)
}

func summaryText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return value
}

func parseAnsibleRecap(line string) (db.AnsibleTaskHost, bool) {
	var host db.AnsibleTaskHost
	// Split at the statistics delimiter, so IPv6 inventory host names work.
	i := strings.Index(line, " :")
	if i < 1 {
		return host, false
	}
	host.Host = strings.TrimSpace(line[:i])
	if len([]rune(host.Host)) > 250 {
		return host, false
	}
	counts := map[string]*int{
		"ok": &host.Ok, "changed": &host.Changed, "unreachable": &host.Unreachable,
		"failed": &host.Failed, "skipped": &host.Skipped, "rescued": &host.Rescued, "ignored": &host.Ignored,
	}
	seen := make(map[string]bool)
	for _, match := range ansibleCounter.FindAllStringSubmatch(line[i+2:], -1) {
		value, err := strconv.Atoi(match[2])
		if err != nil || value > 2147483647 || seen[match[1]] {
			return db.AnsibleTaskHost{}, false
		}
		*counts[match[1]], seen[match[1]] = value, true
	}
	return host, seen["ok"] && seen["changed"] && seen["unreachable"] && seen["failed"]
}

func (p *ansibleOutput) flushError() error {
	if p.pending == nil {
		return nil
	}
	err := p.repo.CreateAnsibleTaskError(*p.pending)
	p.pending = nil
	return err
}

func (p *ansibleOutput) transition(kind db.TaskStageType, at time.Time) error {
	if p.stage != nil && p.stage.Type == kind {
		return nil
	}
	if p.stage != nil {
		if err := p.store.EndTaskStage(p.taskID, p.stage.ID, at); err != nil {
			return err
		}
	}
	stage, err := p.store.CreateTaskStage(db.TaskStage{TaskID: p.taskID, Start: &at, Type: kind})
	if err == nil {
		p.stage = &stage
	}
	return err
}

func (p *ansibleOutput) finish(at time.Time) error {
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.flushError()
	if p.stage != nil {
		err = errors.Join(err, p.store.EndTaskStage(p.taskID, p.stage.ID, at))
		if p.stage.Type == db.TaskStagePrintResult && len(p.hosts) > 0 {
			totals := map[string]any{"hosts": len(p.hosts)}
			var sum db.AnsibleTaskHost
			for _, host := range p.hosts {
				sum.Ok += host.Ok
				sum.Changed += host.Changed
				sum.Failed += host.Failed
				sum.Unreachable += host.Unreachable
				sum.Skipped += host.Skipped
				sum.Rescued += host.Rescued
				sum.Ignored += host.Ignored
			}
			totals["ok"], totals["changed"], totals["failed"] = sum.Ok, sum.Changed, sum.Failed
			totals["unreachable"], totals["skipped"] = sum.Unreachable, sum.Skipped
			totals["rescued"], totals["ignored"] = sum.Rescued, sum.Ignored
			err = errors.Join(err, p.store.CreateTaskStageResult(p.taskID, p.stage.ID, totals))
		}
	}
	return err
}
