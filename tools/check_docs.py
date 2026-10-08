#!/usr/bin/env python3
"""Check repository-local Markdown links, including generated documentation."""
from pathlib import Path
import re
import sys
from urllib.parse import unquote

root = Path(__file__).resolve().parents[1]
files = list(root.glob('*.md'))
for folder in ['docs', 'deployment', 'examples', 'test', 'web']:
    files += [p for p in (root / folder).rglob('*.md')
              if not any(x in p.parts for x in ['node_modules', 'public', 'dist'])]
errors = []
for page in files:
    text = re.sub(r'```.*?```', '', page.read_text(), flags=re.S)
    for target in re.findall(r'\]\(([^)]+)\)', text):
        target = target.split(' "', 1)[0].strip('<>')
        if re.match(r'[a-z]+:', target, re.I) or target.startswith('#'):
            continue
        path = unquote(target.split('#', 1)[0])
        resolved = root / path.lstrip('/') if path.startswith('/') else page.parent / path
        if not resolved.exists():
            errors.append(f'{page.relative_to(root)}: missing {target}')
if errors:
    print('\n'.join(errors), file=sys.stderr)
    sys.exit(1)
print(f'Checked local links in {len(files)} Markdown files.')
