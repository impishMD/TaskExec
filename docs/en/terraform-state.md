# Terraform / OpenTofu HTTP state

TaskExec stores workspace state and its history in the main database. The backend
supports Terraform, OpenTofu and Terragrunt HTTP clients, including state locking.
Each workspace has independent state; aliases of the same workspace share its lock.

## Tasks in TaskExec

Create a Terraform/OpenTofu template with a workspace. Enable **Override backend**
to generate an empty `backend "http"` block in the checkout. The default filename
is `backend.tf`; choose a different `.tf` filename when needed. An existing file
with this name is replaced in the task checkout. Keep only one backend block in
the resulting configuration. TaskExec removes the generated file after execution.

TaskExec supplies the backend address and LOCK/UNLOCK addresses through environment
variables. Each task gets a random temporary endpoint bound to its workspace,
valid only while the task is active. Local tasks and remote runners use the same
backend. Set `TASKEXEC_WEB_HOST` to a URL reachable by every runner or Docker task
container. State written by a task includes its task ID in the history.

The **State** tab of the template lists workspace history and permanent aliases.
Project members can see history metadata; reading state contents or managing
aliases requires **Manage project resources** or instance administrator access.
Template-only permission grants do not grant access to shared workspace secrets.

## External clients

1. Create a shared **Login with password** key in the project's Key Store.
2. Open the template's **State** tab, add an alias and select that key.
3. Copy its URL and configure the HTTP backend:

```hcl
terraform {
  backend "http" {}
}
```

```sh
export TF_HTTP_ADDRESS='https://taskexec.example.com/api/terraform/ALIAS'
export TF_HTTP_LOCK_ADDRESS="$TF_HTTP_ADDRESS"
export TF_HTTP_UNLOCK_ADDRESS="$TF_HTTP_ADDRESS"
export TF_HTTP_LOCK_METHOD=LOCK
export TF_HTTP_UNLOCK_METHOD=UNLOCK
export TF_HTTP_USERNAME='backend-user'
# Set TF_HTTP_PASSWORD using your local secret manager.
terraform init
terraform apply
# The same configuration works with tofu.
```

Use HTTPS outside local development. Keep credentials in environment variables;
backend configuration and plan files can otherwise retain them. TaskExec checks
the selected key on each request, so key rotation takes effect immediately.
Deleting an alias revokes that URL without deleting workspace history. A key
referenced by an alias cannot be deleted until that reference is removed.

## Locks and history

The backend implements GET, POST, DELETE, LOCK and UNLOCK. A conflicting lock or a
write with an incorrect lock ID returns HTTP 409 with the current lock information.
Locks are stored in SQL and survive server restarts. If a client terminates without
unlocking, first ensure it has stopped, then use `terraform force-unlock LOCK_ID`
(or `tofu force-unlock LOCK_ID`) through a permanent alias of the same workspace.

Every successful write creates a snapshot. HTTP DELETE resets the current state
and preserves previous snapshots; it never makes an older snapshot current again.
The project API can explicitly delete a historical snapshot while the workspace is
unlocked. This operation is separate from resetting the backend. The maximum state
size is 32 MiB. Audit events record operations and IDs, never state bodies or keys.

State can contain passwords and other sensitive values. It is stored as JSON in
the database, including its history, so protect database access and backups.
SQLite, PostgreSQL and MySQL are supported.

Protocol references: [Terraform HTTP backend](https://developer.hashicorp.com/terraform/language/backend/http)
and [OpenTofu HTTP backend](https://opentofu.org/docs/language/settings/backends/http/).
