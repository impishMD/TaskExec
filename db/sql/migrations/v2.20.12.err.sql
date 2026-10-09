drop index if exists task__ansible_error__task;
drop index if exists task__ansible_host__task;
alter table task__ansible_error drop column `created`;
alter table task__ansible_host drop column `created`;
