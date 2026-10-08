#!/usr/bin/env bash
set -euo pipefail
binary=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
version=${2:?version required}
"$binary" version | grep -F -- "$version-"
"$binary" --help | grep -F 'Job Executor Hub'
work=$(mktemp -d)
pid=''
cleanup() { if [[ -n "$pid" ]]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi; rm -rf "$work"; }
trap cleanup EXIT
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
export JEH_DB_DIALECT=sqlite JEH_DB_HOST="$work/data.sqlite" JEH_TMP_PATH="$work/tasks" JEH_PORT="$port" JEH_INTERFACE=127.0.0.1
export JEH_COOKIE_HASH
JEH_COOKIE_HASH=$(openssl rand -base64 32)
export JEH_ACCESS_KEY_ENCRYPTION
JEH_ACCESS_KEY_ENCRYPTION=$(openssl rand -base64 32)
"$binary" user add --no-config --admin --login admin --name Admin --email admin@localhost --password jeh-ci-only-password
"$binary" server --no-config > "$work/server.log" 2>&1 &
pid=$!
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/api/ping" > /dev/null 2>&1; then break; fi
  sleep 1
done
curl -fsS "http://127.0.0.1:$port/" | grep -F 'Job Executor Hub'
status=$(curl -sS -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' \
  -d '{"auth":"admin","password":"jeh-ci-only-password"}' "http://127.0.0.1:$port/api/auth/login")
if [[ "$status" != 204 ]]; then cat "$work/server.log"; exit 1; fi
