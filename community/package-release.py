#!/usr/bin/env python3
"""Assemble manifests from tested digests; never include an operator environment."""
import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile

root = Path(__file__).resolve().parent.parent
tag = os.environ.get('COMMUNITY_RELEASE_TAG', '')
if not re.fullmatch(r'community-v0\.\d+\.\d+(?:-[a-z0-9.]+)?', tag):
    raise SystemExit('Invalid Community beta tag')
if not (root / 'LICENSE').is_file() or not (root / 'LICENSE').read_text().strip():
    raise SystemExit('Choose and add a license before assembling release manifests')
artifacts = root / 'release-artifacts'
by_arch = {}
for arch in ('amd64', 'arm64'):
    entries = (artifacts / f'images-{arch}.txt').read_text().splitlines()
    if not entries or any(not re.fullmatch(r'ghcr\.io/helpin-ai/[a-z-]+@sha256:[a-f0-9]{64}', entry) for entry in entries):
        raise SystemExit(f'Invalid tested digest inventory: {arch}')
    by_arch[arch] = dict(entry.split('@', 1) for entry in entries)
if by_arch['amd64'].keys() != by_arch['arm64'].keys():
    raise SystemExit('Architecture inventories differ')
for repository in by_arch['amd64']:
    subprocess.run(['docker', 'buildx', 'imagetools', 'create', '--tag', f'{repository}:{tag}',
                    *[f'{repository}@{by_arch[arch][repository]}' for arch in ('amd64', 'arm64')]], check=True)

def pinned(match):
    original = match.group(1).strip()
    repository = original.split(':', 1)[0]
    reference = f'{repository}:{tag}' if repository in by_arch['amd64'] else original.split('@', 1)[0]
    digest = subprocess.check_output(['docker', 'buildx', 'imagetools', 'inspect', reference, '--format', '{{.Manifest.Digest}}'], text=True).strip()
    if not re.fullmatch(r'sha256:[a-f0-9]{64}', digest):
        raise SystemExit(f'Invalid registry digest for {repository}')
    return f'    image: {reference}@{digest}'

with tempfile.TemporaryDirectory(prefix='community-bundle-') as temporary:
    bundle = Path(temporary) / 'helpin-community'
    community = bundle / 'community'
    community.mkdir(parents=True)
    for name in ('setup.sh', '.env.example', 'apps.example.json', 'README.md', 'PUBLICATION.md', 'Caddyfile.example',
                 'storage-init.sh', 'storage-public-policy.json', 'temporal-schema.sh', 'temporal.yaml', 'temporal-namespace.sh', 'runtime-revision.txt'):
        shutil.copy2(root / 'community' / name, community / name)
    shutil.copytree(root / 'community/postgres', community / 'postgres')
    (community / 'tests').mkdir()
    for name in ('backup-restore.sh', 'http-smoke.mjs'):
        shutil.copy2(root / 'community/tests' / name, community / 'tests' / name)
    shutil.copytree(root / 'docs/community', bundle / 'docs/community')
    for name in ('LICENSE', 'ROADMAP.md', 'SECURITY.md'):
        shutil.copy2(root / name, bundle / name)
    (bundle / 'release-evidence').mkdir()
    for pattern in ('*.cdx.json', 'images-*.txt'):
        for path in artifacts.glob(pattern):
            shutil.copy2(path, bundle / 'release-evidence' / path.name)
    compose = (root / 'community/compose.yaml').read_text()
    (community / 'compose.yaml').write_text(re.sub(r'^    image: (.+)$', pinned, compose, flags=re.MULTILINE))
    checksum_lines = []
    for path in sorted(bundle.rglob('*')):
        if path.is_file():
            checksum_lines.append(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(bundle)}')
    (bundle / 'SHA256SUMS').write_text('\n'.join(checksum_lines) + '\n')
    archive = artifacts / f'helpin-community-{tag}.tar.gz'
    with tarfile.open(archive, 'w:gz') as output:
        output.add(bundle, arcname=bundle.name)
    archive.with_suffix('.gz.sha256').write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
    print(f'Created {archive.name}; every service image is digest-pinned')
