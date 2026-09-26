#!/usr/bin/env python3
"""Build versioned standalone CLIs and checksums for the Community candidate."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

root = Path(__file__).resolve().parents[1]
tag = os.environ.get('COMMUNITY_RELEASE_TAG', '')
if not re.fullmatch(r'community-v0\.\d+\.\d+(?:-[a-z0-9.]+)?', tag):
    raise SystemExit('Invalid Community release tag')
output = root / 'release-artifacts'
output.mkdir(exist_ok=True)
assets = {}
for system in ('linux', 'darwin'):
    for arch in ('amd64', 'arm64'):
        name = f'helpin-{system}-{arch}'
        subprocess.run(['go', 'build', '-trimpath', '-ldflags', f'-s -w -X main.version={tag}',
                        '-o', str(output / name), '.'], cwd=root / 'community/cli', check=True,
                       env={**os.environ, 'CGO_ENABLED': '0', 'GOWORK': 'off', 'GOOS': system, 'GOARCH': arch})
        digest = hashlib.sha256((output / name).read_bytes()).hexdigest()
        (output / (name + '.sha256')).write_text(f'{digest}  {name}\n')
        assets[name] = digest
installer = (root / 'community/install.sh').read_bytes()
if installer != (root / 'website/public/install.sh').read_bytes():
    raise SystemExit('Website bootstrap differs from community/install.sh')
(output / 'install.sh').write_bytes(installer)
assets['install.sh'] = hashlib.sha256(installer).hexdigest()
(output / 'install.sh.sha256').write_text(f"{assets['install.sh']}  install.sh\n")
(output / 'cli-assets.json').write_text(json.dumps(assets, indent=2) + '\n')
