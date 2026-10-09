#!/usr/bin/env bash
# Integration test for a disposable kind cluster only.
set -euo pipefail
context=kind-taskexec-chart
if [[ "$(kubectl config current-context)" != "$context" ]]; then
  echo "Refusing to run: use a disposable kind cluster named taskexec-chart." >&2
  exit 1
fi
namespace=taskexec-chart-test
chart=charts/taskexec
kc() { kubectl --context "$context" --namespace "$namespace" "$@"; }
hc() { helm "$@" --kube-context "$context" --namespace "$namespace"; }
on_exit() {
  code=$?
  if [[ "$code" != 0 ]]; then
    kc get pods,pvc || true
    kc describe pods || true
    kc logs deployment/taskexec-test -c taskexec --tail=150 || true
  fi
  exit "$code"
}
trap on_exit EXIT
kubectl --context "$context" create namespace "$namespace"
kc create secret generic taskexec-test \
  --from-literal=TASKEXEC_ADMIN_PASSWORD=chart-test-only-password \
  --from-literal=TASKEXEC_ACCESS_KEY_ENCRYPTION="$(openssl rand -base64 32)"

# Exercise authentication, API persistence and persisted cookie signing keys.
verify() {
  kc exec -i deployment/taskexec-test -c taskexec -- python3 - "$1" <<'PY'
import http.cookiejar
import json
import os
from pathlib import Path
import sys
import urllib.parse
import urllib.request
base = 'http://127.0.0.1:3000' + urllib.parse.urlparse(os.environ['TASKEXEC_WEB_ROOT']).path.rstrip('/')
state = Path('/var/lib/taskexec/chart-check.json')
assert os.getuid() == 1001
config = Path('/var/lib/taskexec/config/config.json')
assert config.is_file() and Path('/var/lib/taskexec/database.sqlite').is_file()
assert urllib.request.urlopen(base + '/api/ping').status == 200
assert b'TaskExec' in urllib.request.urlopen(base + '/').read()
if sys.argv[1] == 'create':
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
    def request(path, data):
        return client.open(urllib.request.Request(base + path, data=json.dumps(data).encode(), headers={'Content-Type':'application/json'}))
    assert request('/api/auth/login', {'auth':'admin','password':'chart-test-only-password'}).status == 204
    project = json.load(request('/api/projects', {'name':'Helm persistence check','max_parallel_tasks':1}))
    state.write_text(json.dumps({'id':project['id'], 'cookie':'; '.join(f'{c.name}={c.value}' for c in jar), 'config':config.read_text()}))
    state.chmod(0o600)
else:
    saved = json.loads(state.read_text())
    assert config.read_text() == saved['config'], 'Persistent configuration changed'
    req = urllib.request.Request(base + '/api/projects', headers={'Cookie':saved['cookie']})
    projects = json.load(urllib.request.urlopen(req))
    assert any(p['id'] == saved['id'] and p['name'] == 'Helm persistence check' for p in projects)
print('Authentication and persistent data: OK')
PY
}

hc upgrade --install test "$chart" -f "$chart/ci/test-values.yaml" --wait --timeout 5m
hc test test --logs --timeout 2m
verify create
previous_pod=$(kc get pod -l app.kubernetes.io/instance=test,app.kubernetes.io/component=server -o jsonpath='{.items[0].metadata.uid}')
# Config checksum must restart the server; subpath probes and Service access must work.
hc upgrade test "$chart" -f "$chart/ci/test-values.yaml" \
  --set config.webRoot=http://taskexec-test:3000/taskexec --wait --timeout 5m
current_pod=$(kc get pod -l app.kubernetes.io/instance=test,app.kubernetes.io/component=server -o jsonpath='{.items[0].metadata.uid}')
[[ "$previous_pod" != "$current_pod" ]]
verify existing
hc test test --logs --timeout 2m
kc rollout restart deployment/taskexec-test
kc rollout status deployment/taskexec-test --timeout=3m
verify existing
hc uninstall test --wait --timeout 2m
kc get pvc taskexec-test

# Emulate imported data on a volume that does not implement fsGroup.
kc apply -f - <<'YAML'
apiVersion: batch/v1
kind: Job
metadata:
  name: imported-data
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      automountServiceAccountToken: false
      containers:
        - name: seed
          image: taskexec:helm-ci
          imagePullPolicy: Never
          command: [/bin/sh, -ec]
          args:
            - 'printf imported > /data/imported-marker; chown 0:0 /data/imported-marker; chmod 600 /data/imported-marker'
          securityContext:
            runAsUser: 0
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: taskexec-test
YAML
kc wait --for=condition=complete job/imported-data --timeout=2m
kc delete job imported-data --wait=true
hc upgrade --install test "$chart" -f "$chart/ci/test-values.yaml" \
  --set persistence.existingClaim=taskexec-test --set volumePermissions.enabled=true \
  --set podSecurityContext.fsGroup=null --wait --timeout 5m
verify existing
kc exec deployment/taskexec-test -c taskexec -- sh -ec 'test "$(cat /var/lib/taskexec/imported-marker)" = imported; test -w /var/lib/taskexec/imported-marker'
hc uninstall test --wait --timeout 2m
kc get pvc taskexec-test
# The namespace remains available for inspection until the disposable cluster is removed.
echo 'Chart install, upgrade, restart, subpath and retained-PVC checks passed.'
