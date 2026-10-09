# Workflows

**English** | [Русский](../ru/workflows.md)

Open **Project → Workflows**, create a graph, save it and run it. A graph must have
one executable starting node, no cycles and at most 500 nodes / 2,000 connections.
Notes are annotations and cannot have connections.

| Node | Behavior |
| --- | --- |
| Task | Runs a task template from this project with the node's saved parameters |
| Approval | Waits for approval/rejection; an optional timeout in seconds rejects it |
| Delay | Waits the configured positive number of seconds |
| Note | Displays a comment without running anything |

Connections run on success, failure, or either result. **All** requires every incoming
condition to match; **Any** starts after the first matching result. Unreachable branches
are skipped. Each node runs at most once per run on a single server. The run is failed
if any executed task fails or an approval is rejected, even when a failure branch succeeds.
Stopping the run stops active/queued tasks and cancels pending approvals and delays.

Saving creates an immutable graph revision. A run keeps its original revision when the
graph is edited. Task template definitions themselves remain shared: a node uses the
current definition when it launches. An optional starting version increments per run
and is shared by its nodes. The first task becomes the build reference for deploy nodes.

Project members can view graphs and runs. Editing/deleting requires **Manage project
resources**; running, stopping and resolving approvals requires **Run project tasks**.
Individual template grants do not grant workflow control. Actions are recorded in the audit log.

## Ansible values between nodes

Use [`ansible.builtin.set_stats`](https://docs.ansible.com/projects/ansible/latest/collections/ansible/builtin/set_stats_module.html)
with `per_host: false` to publish JSON values. TaskExec adds a callback alongside the
configured stdout callback. `aggregate` follows Ansible's accumulation rules within a task.
`no_log` results and per-host stats are excluded. Artifacts are public project data,
visible in task logs/details; do not use them to pass credentials.

```yaml
- ansible.builtin.set_stats:
    data:
      release_revision: "{{ revision }}"
    per_host: false
    aggregate: false
```

Downstream nodes receive the completed ancestors' values as variables. Nested objects
merge; later task IDs win on conflicts. Explicit node variables take precedence, followed
by the normal secret-variable handling. Unrelated sibling branches are excluded. This works
with local tasks and remote runners, including Docker execution. Keep server and
runners on the same version. A task's artifacts must be a JSON object no larger than 1 MiB.

## Restart, history and backups

Approvals and delay deadlines are stored in the database and resume after server restart.
Tasks assigned to remote runners use normal runner reconciliation. Interrupted local tasks
(including tasks not yet assigned before restart) are stopped; their workflows are stopped
without replaying commands. Start a new run after checking any partial external changes.
Use one TaskExec server to coordinate workflows.

Automatic task retention preserves workflow task records. Individual workflow tasks cannot
be deleted while their run exists, and templates referenced by saved graph revisions cannot
be deleted. Delete a workflow only after its active runs finish/stop: this removes its graphs
and run history while retaining ordinary task logs. Project export/import includes the latest
graph and node parameters with references remapped to the restored project. Use a full database
backup to preserve historical revisions, runs, approvals and timers.
