alter table task__ansible_host add `created` datetime null;
alter table task__ansible_error add `created` datetime null;
create index task__ansible_host__task on task__ansible_host (task_id, project_id);
create index task__ansible_error__task on task__ansible_error (task_id, project_id);
