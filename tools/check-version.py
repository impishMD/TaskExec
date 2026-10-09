#!/usr/bin/env python3
"""Check application version files without needing a YAML dependency."""
import json
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
version = (root / 'VERSION').read_text().strip()
assert re.fullmatch(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?', version), version
package = json.loads((root / 'web/package.json').read_text())
lock = json.loads((root / 'web/package-lock.json').read_text())
assert package['version'] == lock['version'] == lock['packages']['']['version'] == version
chart = (root / 'charts/taskexec/Chart.yaml').read_text()
assert re.search(r'^appVersion: "' + re.escape(version) + '"$', chart, re.M), 'Chart appVersion must match VERSION'
assert f'TASKEXEC_VERSION=v{version}\n' in (root / '.env.example').read_text()
print(f'Application version: v{version}')
