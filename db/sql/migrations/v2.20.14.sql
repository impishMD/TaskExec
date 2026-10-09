{{if .Mysql}}
alter table project__workflow_run add started_by_user_id int null;
alter table project__workflow_run add constraint workflow_run__started_by_user_fk foreign key (started_by_user_id) references `user`(id) on delete set null;
{{else}}
alter table project__workflow_run add started_by_user_id int null references `user`(id) on delete set null;
{{end}}
