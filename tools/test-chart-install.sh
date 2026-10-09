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
kc create secret generic taskexec-oidc --from-literal=client-secret=chart-test-only-oidc-secret
fixture_dir=$(mktemp -d)
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj '/CN=TaskExec Helm Test CA' \
  -keyout "$fixture_dir/key.pem" -out "$fixture_dir/ca.crt" >/dev/null 2>&1
kc create configmap taskexec-ca --from-file="ca.crt=$fixture_dir/ca.crt"
rm -rf "$fixture_dir"

# Exercise authentication, API persistence and persisted cookie signing keys.
verify() {
  kc exec -i deployment/taskexec-test -c taskexec -- python3 - "$1" <<'PY'
import http.cookiejar
import http.server
import json
import os
from pathlib import Path
import ssl
import sys
import threading
import urllib.error
import urllib.parse
import urllib.request
base = 'http://127.0.0.1:3000' + urllib.parse.urlparse(os.environ['TASKEXEC_WEB_ROOT']).path.rstrip('/')
state = Path('/var/lib/taskexec/chart-check.json')
assert os.getuid() == 1001
config = Path('/var/lib/taskexec/config/config.json')
assert config.is_file() and Path('/var/lib/taskexec/database.sqlite').is_file()
assert urllib.request.urlopen(base + '/api/ping').status == 200
assert b'TaskExec' in urllib.request.urlopen(base + '/').read()
if os.environ.get('SSL_CERT_FILE'):
    subjects = [dict(item for rdn in cert['subject'] for item in rdn)
                for cert in ssl.create_default_context().get_ca_certs()]
    assert any(subject.get('commonName') == 'TaskExec Helm Test CA' for subject in subjects)
    assert len(subjects) > 1, 'Custom CA must supplement the default trust store'
    print('Custom CA and default trust store: OK')
metadata = json.load(urllib.request.urlopen(base + '/api/auth/login'))
if sys.argv[1] == 'disabled':
    assert metadata['oidc_providers'] == []
    assert json.loads(os.environ['TASKEXEC_OIDC_PROVIDERS']) == {}
else:
    assert metadata['oidc_providers'][0]['name'] == 'Test SSO'
    provider = json.loads(os.environ['TASKEXEC_OIDC_PROVIDERS'])['test']
    assert 'client_secret' not in provider
    assert Path(provider['client_secret_file']).read_text() == 'chart-test-only-oidc-secret'
    # Exercise actual OIDC discovery and login with the Secret-mounted client
    # credentials. No external identity provider is required for this fixture.
    class Discovery(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            issuer = 'http://127.0.0.1:5556'
            data = json.dumps({'issuer':issuer, 'authorization_endpoint':issuer+'/auth',
                               'token_endpoint':issuer+'/token', 'jwks_uri':issuer+'/keys',
                               'id_token_signing_alg_values_supported':['RS256']}).encode()
            self.send_response(200)
            self.send_header('Content-Type','application/json')
            self.end_headers()
            self.wfile.write(data)
        def log_message(self, *_):
            pass
    class Server(http.server.HTTPServer):
        allow_reuse_address = True
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *_):
            return None
    with Server(('127.0.0.1',5556), Discovery) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            urllib.request.build_opener(NoRedirect()).open(base+'/api/auth/oidc/test/login')
            raise AssertionError('OIDC must redirect to the identity provider')
        except urllib.error.HTTPError as error:
            assert error.code == 307
            query = urllib.parse.parse_qs(urllib.parse.urlparse(error.headers['Location']).query)
            assert query['client_id'] == ['chart-test-client']
            assert query['redirect_uri'] == [os.environ['TASKEXEC_WEB_ROOT'].rstrip('/')+'/api/auth/oidc/test/redirect']
            assert query['scope'] == ['openid profile email']
            assert query['state'][0]
            assert 'client_secret' not in query
        finally:
            server.shutdown()
            thread.join()
    print('OIDC discovery, Secret credentials and callback URL: OK')
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
  --set general.host=http://taskexec-test:3000/taskexec \
  --set customCertificates.enabled=true --set customCertificates.existingConfigMap=taskexec-ca \
  --wait --timeout 5m
current_pod=$(kc get pod -l app.kubernetes.io/instance=test,app.kubernetes.io/component=server -o jsonpath='{.items[0].metadata.uid}')
[[ "$previous_pod" != "$current_pod" ]]
verify existing
hc test test --logs --timeout 2m
hc upgrade test "$chart" -f "$chart/ci/test-values.yaml" \
  --set oidc.enable=false --wait --timeout 5m
verify disabled
hc upgrade test "$chart" -f "$chart/ci/test-values.yaml" --wait --timeout 5m
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
