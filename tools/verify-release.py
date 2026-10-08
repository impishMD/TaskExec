#!/usr/bin/env python3
"""Verify release checksums, archive contents and the architecture of each binary."""
from pathlib import Path
import hashlib
import struct
import sys
import tarfile

root = Path(sys.argv[1] if len(sys.argv) > 1 else 'dist')
checksums = {}
for line in (root / 'checksums.txt').read_text().splitlines():
    digest, name = line.split(maxsplit=1)
    checksums[name.lstrip('*')] = digest
assert len(checksums) == 9, f'Expected nine release artifacts, found {len(checksums)}'
for name, digest in checksums.items():
    path = root / name
    assert path.is_file(), f'Missing artifact: {name}'
    assert hashlib.sha256(path.read_bytes()).hexdigest() == digest, name
    if not name.endswith('.tar.gz') or '_source.' in name:
        continue
    with tarfile.open(path) as archive:
        files = {m.name.removeprefix('./'): m for m in archive.getmembers()}
        for required in ['jeh', 'LICENSE', 'NOTICE', 'docs/en/README.md', 'docs/ru/README.md']:
            assert required in files, f'{name}: missing {required}'
        binary = archive.extractfile(files['jeh']).read(64)
        if '_linux_' in name:
            assert binary[:4] == b'\x7fELF', name
            arch = struct.unpack('<H', binary[18:20])[0]
            assert arch == (183 if '_arm64.' in name else 62), name
        else:
            assert binary[:4] == bytes.fromhex('cffaedfe'), name
            arch = struct.unpack('<I', binary[4:8])[0]
            assert arch == (0x0100000c if '_arm64.' in name else 0x01000007), name
assert sum(n.endswith('.deb') for n in checksums) == 2
assert sum(n.endswith('.rpm') for n in checksums) == 2
assert sum('_source.tar.gz' in n for n in checksums) == 1
print('Verified nine artifacts, checksums, bundled documentation and four binary architectures.')
