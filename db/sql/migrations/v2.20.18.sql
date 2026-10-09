alter table `user` drop column pro;
update project__secret_storage set type = 'vault' where type = 'openbao';
