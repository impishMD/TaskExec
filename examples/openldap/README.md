# JEH with OpenLDAP example

1. Start stack by command:
   ```
   docker compose up -d
   ```
2. Create new LDAP user:
   1. Open https://localhost:6443
   2. Login as `cn=admin,dc=example,dc=org` with password `admin`
   3. Create new user `john`


3. Create new JEH project:
   1. Open http://localhost:3000
   2. Login as `john`
   3. Create demo project

[JEH documentation EN](../../docs/en/README.md) · [Документация RU](../../docs/ru/README.md).
