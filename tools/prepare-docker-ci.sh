#!/usr/bin/env bash
set -euo pipefail

# Only configure the disposable CI daemon, never a developer's local Docker.
if [[ "${GITHUB_ACTIONS:-}" != true ]]; then
  echo 'This helper is only intended for GitHub Actions runners.' >&2
  exit 1
fi

# Google's public Docker Hub cache also serves pulls when Hub authentication
# is unavailable or the runner has exhausted its anonymous pull quota.
# Preserve the runner's existing daemon settings and registry credentials.
sudo python3 - <<'PYTHON'
import json
from pathlib import Path

path = Path('/etc/docker/daemon.json')
config = json.loads(path.read_text()) if path.exists() else {}
mirror = 'https://mirror.gcr.io'
config['registry-mirrors'] = [mirror] + [
    value for value in config.get('registry-mirrors', []) if value != mirror
]
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(config) + '\n')
PYTHON
sudo systemctl restart docker
