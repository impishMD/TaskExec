#!/usr/bin/env bash
set -euo pipefail
image=${1:?image required}
version=${2:?version required}
role=${3:-server}
if [[ "$role" == job || "$role" == helper ]]; then
  docker run --rm "$image" sh -ec 'test "$(id -u)" != 0; git --version; ssh -V'
  if [[ "$role" == job ]]; then
    docker run --rm "$image" sh -ec 'ansible-playbook --version; tofu version; terraform version; terragrunt --version'
  fi
  exit 0
fi
docker run --rm "$image" jeh version | grep -F -- "$version-"
docker run --rm "$image" sh -ec 'test "$(id -u)" = 1001; jeh runner start --help; ansible-playbook --version; tofu version'
if [[ "$role" == runner ]]; then exit 0; fi
name="jeh-smoke-$RANDOM-$$"
cleanup() { result=$?; if [[ "$result" != 0 ]]; then docker logs "$name" || true; fi; docker rm -f "$name" > /dev/null 2>&1 || true; }
trap cleanup EXIT
key=$(openssl rand -base64 32)
docker run -d --name "$name" -p 127.0.0.1::3000 \
  -e JEH_DB_DIALECT=sqlite -e JEH_ADMIN=admin -e JEH_ADMIN_PASSWORD=jeh-ci-only-password \
  -e JEH_ACCESS_KEY_ENCRYPTION="$key" "$image" > /dev/null
port=$(docker port "$name" 3000/tcp | head -1 | sed 's/.*://')
url="http://127.0.0.1:$port"
ready=false
for _ in $(seq 1 90); do
  if curl -fsS "$url/api/ping" > /dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then docker logs "$name"; exit 1; fi
curl -fsS "$url/" | grep -F 'Job Executor Hub'
curl -fsS "$url/favicon.svg" | grep -F 'aria-label="JEH"'
status=$(curl -sS -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' \
  -d '{"auth":"admin","password":"jeh-ci-only-password"}' "$url/api/auth/login")
test "$status" = 204
# Persisted SQLite configuration must survive container restart.
docker restart "$name" > /dev/null
port=$(docker port "$name" 3000/tcp | head -1 | sed 's/.*://')
url="http://127.0.0.1:$port"
for _ in $(seq 1 60); do
  if curl -fsS "$url/api/ping" > /dev/null 2>&1; then break; fi
  sleep 1
done
status=$(curl -sS -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' \
  -d '{"auth":"admin","password":"jeh-ci-only-password"}' "$url/api/auth/login")
test "$status" = 204
