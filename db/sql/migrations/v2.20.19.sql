create table project__token (
    id varchar(32) primary key,
    project_id int not null,
    name varchar(100) not null,
    secret_hash varchar(64) not null,
    scopes text not null,
    all_templates boolean not null default false,
    template_ids text not null,
    overrides text not null,
    creator_id int not null,
    creator_name varchar(255) not null,
    created datetime not null,
    expires_at datetime null,
    revoked_at datetime null,
    last_used_at datetime null,
    foreign key (project_id) references project(id) on delete cascade
);
create index project__token_project on project__token(project_id);
alter table task add project_token_id varchar(32) null;
alter table task add project_token_name varchar(100) not null default '';
