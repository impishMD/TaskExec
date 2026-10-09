create table `alert_channel` (
  `channel` varchar(32) not null primary key,
  `enabled` boolean not null default false,
  `settings` text not null,
  `secret` text not null
){{if .Mysql}} default charset=utf8mb4{{end}};

create table `project__alert_channel` (
  `project_id` int not null,
  `channel` varchar(32) not null,
  `enabled` boolean not null default false,
  `settings` text not null,
  `secret` text not null,
  primary key (`project_id`, `channel`),
  foreign key (`project_id`) references `project` (`id`) on delete cascade
){{if .Mysql}} default charset=utf8mb4{{end}};
