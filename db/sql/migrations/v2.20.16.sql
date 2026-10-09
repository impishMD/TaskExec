-- Source aliases use the existing environment-key relation and its key FK.
create table project__environment_expression (
  environment_id int not null,
  name varchar(255) not null,
  type varchar(10) not null,
  expression text not null,
  primary key (environment_id, type, name),
  foreign key (environment_id) references project__environment(id) on delete cascade
);

-- Existing access-key references remain intact; only discovery is retired.
drop table project__secret_sync_path;
drop table project__secret_sync;

-- Each existing external key already owns its Vault reference.
update project__environment set secret_storage_id = null, secret_storage_key_prefix = null;
