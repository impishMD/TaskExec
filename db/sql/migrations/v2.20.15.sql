alter table access_key add source_mapping text null;

create table project__environment_key (
  environment_id int not null,
  key_id int not null,
  name varchar(255) not null,
  type varchar(10) not null,
  field varchar(255) null,
  primary key (environment_id, type, name),
  foreign key (environment_id) references project__environment(id) on delete cascade,
  foreign key (key_id) references access_key(id) on delete restrict
);
create index environment_key__key_id on project__environment_key(key_id);
