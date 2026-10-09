# TC-014 — Bind HashiCorp Vault secret storage to a project

| Field        | Value                            |
|--------------|----------------------------------|
| Area         | Key Store / Secret Storage       |
| Priority     | Medium                           |
| Type         | Integration                      |
| Automatable  | Partial (Vault dev server)       |

## Objective

A project can use HashiCorp Vault as a secret storage backend. Secrets fetched
from Vault are surfaced as access keys without being persisted in plaintext in
the TaskExec database.

## Preconditions

* Vault dev server reachable from TaskExec at `http://vault:8200`.
* AppRole or token with read access to `secret/data/qa/*`.
* A secret stored at `secret/data/qa/db_password` with key `password = s3cret`.
* Project `Infra QA`.

## Test data

| Field           | Value                  |
|-----------------|------------------------|
| Storage name    | `hcvault-qa`           |
| Storage type    | `vault`                |
| Address         | `http://vault:8200`    |
| Auth method     | `token` (dev mode)     |

## Steps

1. Navigate to **Secret Storages → New Storage** in project `Infra QA`.
2. Select type **HashiCorp Vault**, fill the address and token, save.
3. Open **Key Store → New Key**, choose **Source: Vault**, point at
   `qa/db_password` relative to mount `secret`, choose **String**, load fields and select
   `password`, save as key
   `db-password-vault`.
4. In a variable group, open **Keys**, bind the key as `db_password` targeting extra
   variables, inspect the fresh preview, and attach the group to an Ansible template.
   Use an assertion with `no_log: true` to verify `db_password` without printing it.
5. Disable the Vault server temporarily, re-run the task.

## Expected results

* Step 2: storage saved; the token field is not displayed after save.
* Step 3: the key is sourced from Vault (manual keys are not synchronized);
  the `plain` column in the database is null/empty (verify via SQL on
  `access_key` table).
* Step 4: task succeeds; the value is rendered correctly when used.
* Step 5: task fails fast with a clear "secret storage unavailable" error
  rather than crashing the runner.

## Postconditions

Restore Vault server; the storage and key remain for downstream tests.
