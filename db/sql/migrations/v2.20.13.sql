create table project__terraform_inventory_lock (
  inventory_id int primary key,
  project_id int not null,
  lock_data text not null,
  foreign key (inventory_id) references project__inventory(id) on delete cascade,
  foreign key (project_id) references project(id) on delete cascade
);
create index project__terraform_inventory_state__latest on project__terraform_inventory_state (project_id, inventory_id, id);
