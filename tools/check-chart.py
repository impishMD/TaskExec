#!/usr/bin/env python3
"""Render Helm contracts; no cluster or credentials required. Needs PyYAML."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import tempfile

import yaml

ROOT = Path(__file__).resolve().parents[1]
CHART = ROOT / 'charts/taskexec'

class UniqueLoader(yaml.SafeLoader):
    pass

def mapping(loader, node, deep=False):
    result = {}
    for key, value in node.value:
        key = loader.construct_object(key, deep=deep)
        assert key not in result, f'Duplicate YAML key: {key}'
        result[key] = loader.construct_object(value, deep=deep)
    return result

UniqueLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, mapping)

def render(values, valid=True):
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json') as file:
        json.dump(values, file)
        file.flush()
        proc = subprocess.run(['helm', 'template', 'test', str(CHART), '-f', file.name,
                               '--namespace', 'chart-test'], capture_output=True, text=True)
    if not valid:
        assert proc.returncode, f'Invalid values accepted: {values}'
        return None
    assert proc.returncode == 0, proc.stderr
    return [obj for obj in yaml.load_all(proc.stdout, Loader=UniqueLoader) if obj]

def kind(objects, name):
    return next(obj for obj in objects if obj['kind'] == name)

def pod(objects):
    return kind(objects, 'Deployment')['spec']['template']

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--release-tag')
    parser.add_argument('--output', type=Path, help='Write valid rendered fixtures for kubeconform')
    args = parser.parse_args()
    chart = yaml.safe_load((CHART / 'Chart.yaml').read_text())
    assert chart['apiVersion'] == 'v2' and chart['type'] == 'application'
    assert chart['appVersion'] == (ROOT / 'VERSION').read_text().strip()
    if args.release_tag:
        assert re.fullmatch(r'chart-v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?', args.release_tag)
        assert args.release_tag == f"chart-v{chart['version']}"
        for lang in ['en', 'ru']:
            assert (ROOT / f"release-notes/charts/{lang}/v{chart['version']}.md").is_file()
    base = {'existingSecret': 'fixture-credentials'}
    objects = render(base)
    template = pod(objects)
    spec = template['spec']
    container = spec['containers'][0]
    assert kind(objects, 'Deployment')['spec']['replicas'] == 1
    assert kind(objects, 'Deployment')['spec']['strategy'] == {'type': 'Recreate'}
    assert container['image'] == f"ghcr.io/impishmd/taskexec:v{chart['appVersion']}"
    assert spec['automountServiceAccountToken'] is False
    assert spec['securityContext']['runAsUser'] == 1001
    assert container['securityContext']['capabilities']['drop'] == ['ALL']
    assert container['readinessProbe']['httpGet']['path'] == '/api/ping'
    assert not any(obj['kind'] == 'Secret' for obj in objects)
    selector = kind(objects, 'Service')['spec']['selector']
    assert all(template['metadata']['labels'].get(k) == v for k, v in selector.items())
    hook = kind(objects, 'Pod')
    assert any(hook['metadata']['labels'].get(k) != v for k, v in selector.items()), 'Service must not route requests to test pods'
    assert 'hook-succeeded' not in hook['metadata']['annotations']['helm.sh/hook-delete-policy'], 'Keep logs available for helm test --logs'
    assert container['envFrom'][-1]['configMapRef']['name'] == 'test-taskexec'
    for entry in container['env']:
        assert entry['valueFrom']['secretKeyRef']['name'] == base['existingSecret']
    config = kind(objects, 'ConfigMap')['data']
    assert config['TASKEXEC_DB_DIALECT'] == 'sqlite'
    assert config['TASKEXEC_DB_HOST'] == '/var/lib/taskexec/database.sqlite'
    assert config['TASKEXEC_CONFIG_PATH'] == '/var/lib/taskexec/config'
    pvc = kind(objects, 'PersistentVolumeClaim')
    assert 'storageClassName' not in pvc['spec']
    assert pvc['metadata']['annotations']['helm.sh/resource-policy'] == 'keep'
    assert 'Prune=false' in pvc['metadata']['annotations']['argocd.argoproj.io/sync-options']
    assert not any(obj['kind'] in ['Ingress', 'Role', 'ClusterRole'] for obj in objects)
    fixtures = {'default': objects}
    for dialect, port in [('postgres', '5432'), ('mysql', '3306')]:
        values = {**base, 'config': {'database': {'dialect': dialect, 'host': 'database.internal', 'options': {'sslmode': 'require'} if dialect == 'postgres' else {}}}}
        rendered = render(values)
        data = kind(rendered, 'ConfigMap')['data']
        assert data['TASKEXEC_DB_PORT'] == port
        assert data['TASKEXEC_DB_HOST'] == 'database.internal'
        assert data['TASKEXEC_DB'] == 'taskexec'
        assert 'TASKEXEC_DB_USER' in [e['name'] for e in pod(rendered)['spec']['containers'][0]['env']]
        fixtures[dialect] = rendered
    advanced = {**base, 'fullnameOverride':'taskexec-custom',
                'config': {'webRoot': 'https://tasks.example.com/taskexec/'},
                'image': {'digest':'sha256:' + 'a'*64, 'tag':'ignored'},
                'service': {'port':80}, 'ingress': {'enabled':True, 'className':'nginx',
                'hosts':[{'host':'tasks.example.com','paths':[{'path':'/taskexec','pathType':'Prefix'}]}],
                'tls':[{'secretName':'taskexec-tls','hosts':['tasks.example.com']}]},
                'persistence': {'existingClaim':'imported-data'},
                'extraEnv': {'TASKEXEC_MAX_PARALLEL_TASKS':'4'},
                'extraVolumes':[{'name':'certificates','secret':{'secretName':'vault-ca'}}],
                'extraVolumeMounts':[{'name':'certificates','mountPath':'/certificates','readOnly':True}],
                'volumePermissions':{'enabled':True}}
    rendered = render(advanced)
    assert not any(obj['kind'] == 'PersistentVolumeClaim' for obj in rendered)
    p = pod(rendered)
    assert p['spec']['volumes'][0]['persistentVolumeClaim']['claimName'] == 'imported-data'
    c = p['spec']['containers'][0]
    assert c['image'].endswith('@sha256:' + 'a'*64)
    assert c['readinessProbe']['httpGet']['path'] == '/taskexec/api/ping'
    assert c['volumeMounts'][-1]['readOnly']
    init = p['spec']['initContainers'][0]
    assert init['securityContext']['runAsUser'] == 0 and 'envFrom' not in init and 'env' not in init
    assert kind(rendered, 'Ingress')['spec']['rules'][0]['http']['paths'][0]['backend']['service']['port']['number'] == 80
    assert p['metadata']['annotations']['checksum/config'] != template['metadata']['annotations']['checksum/config']
    fixtures['ingress-external-pvc'] = rendered
    for storage_class in ['', 'fast-storage']:
        rendered = render({**base, 'persistence':{'storageClass':storage_class, 'retain':False}})
        pvc = kind(rendered, 'PersistentVolumeClaim')
        assert pvc['spec']['storageClassName'] == storage_class
        assert not pvc['metadata'].get('annotations')
        fixtures['storage-' + (storage_class or 'none')] = rendered
    for invalid in [{}, {**base,'replicaCount':2}, {**base,'existingSecret':''},
                    {**base,'config':{'database':{'dialect':'postgres'}}},
                    {**base,'config':{'webRoot':'not-a-url'}},
                    {**base,'image':{'digest':'not-a-digest'}},
                    {**base,'service':{'port':0}}, {**base,'ingress':{'hosts':[]}},
                    {**base,'persistence':{'accessModes':['ReadWriteMany']}},
                    {**base,'extraEnv':{'TASKEXEC_PORT':'8080'}},
                    {**base,'extraEnv':{'TASKEXEC_ADMIN_PASSWORD':'unsafe'}},
                    {**base,'podAnnotations':{'checksum/config':'override'}},
                    {**base,'startupProbe':{'httpGet':{'path':'/wrong'}}}]:
        render(invalid, valid=False)
    if args.output:
        args.output.mkdir(parents=True, exist_ok=True)
        for name, rendered in fixtures.items():
            (args.output / f'{name}.yaml').write_text(yaml.safe_dump_all(rendered))
    print(f'Chart {chart["version"]}: {len(fixtures)} render scenarios and invalid-input checks passed.')

if __name__ == '__main__':
    main()
