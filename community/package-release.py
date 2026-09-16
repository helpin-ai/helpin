#!/usr/bin/env python3
"""Assemble manifests from tested digests; never include an operator environment."""
import hashlib
import json
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
license_files = ('LICENSE', 'LICENSE-AGPL-3.0', 'LICENSE-APACHE-2.0', 'ee/LICENSE')
for name in license_files:
    if not (root / name).is_file() or not (root / name).read_text().strip():
        raise SystemExit(f'Missing release license: {name}')
artifacts = root / 'release-artifacts'
compose_source = (root / 'community/compose.yaml').read_text()
expected = set(re.findall(r'^    image: (ghcr.io/helpin-ai/[a-z-]+):', compose_source, re.MULTILINE))
source_revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
runtime_revision = (root / 'community/runtime-revision.txt').read_text().strip()
evidence = os.environ.get('PUBLIC_HOST_EVIDENCE', '')
if not evidence.startswith('https://') or any(c.isspace() for c in evidence):
    raise SystemExit('A reviewed HTTPS acceptance record is required')
by_arch = {}
for arch in ('amd64', 'arm64'):
    entries = (artifacts / f'images-{arch}.txt').read_text().splitlines()
    if not entries or any(not re.fullmatch(r'ghcr\.io/helpin-ai/[a-z-]+@sha256:[a-f0-9]{64}', entry) for entry in entries):
        raise SystemExit(f'Invalid tested digest inventory: {arch}')
    by_arch[arch] = dict(entry.split('@', 1) for entry in entries)
    if len(entries) != len(by_arch[arch]) or set(by_arch[arch]) != expected:
        raise SystemExit(f'Incomplete or duplicate tested digest inventory: {arch}')
    if (artifacts / f'source-{arch}.txt').read_text().strip() != source_revision:
        raise SystemExit(f'Source revision mismatch: {arch}')
    if (artifacts / f'runtime-{arch}.txt').read_text().strip() != runtime_revision:
        raise SystemExit(f'Runtime revision mismatch: {arch}')
if by_arch['amd64'].keys() != by_arch['arm64'].keys():
    raise SystemExit('Architecture inventories differ')
for repository in by_arch['amd64']:
    subprocess.run(['docker', 'buildx', 'imagetools', 'create', '--tag', f'{repository}:{tag}',
                    *[f'{repository}@{by_arch[arch][repository]}' for arch in ('amd64', 'arm64')]], check=True)

def pinned(match):
    original = match.group(1).strip()
    repository = original.split(':', 1)[0]
    if repository not in by_arch['amd64']:
        if not re.fullmatch(r'[^ ]+@sha256:[a-f0-9]{64}', original):
            raise SystemExit(f'Upstream image must already be digest-pinned: {original}')
        return f'    image: {original}'
    reference = f'{repository}:{tag}' if repository in by_arch['amd64'] else original.split('@', 1)[0]
    digest = subprocess.check_output(['docker', 'buildx', 'imagetools', 'inspect', reference, '--format', '{{.Manifest.Digest}}'], text=True).strip()
    if not re.fullmatch(r'sha256:[a-f0-9]{64}', digest):
        raise SystemExit(f'Invalid registry digest for {repository}')
    return f'    image: {reference}@{digest}'

with tempfile.TemporaryDirectory(prefix='community-bundle-') as temporary:
    bundle = Path(temporary) / 'helpin-community'
    community = bundle / 'community'
    community.mkdir(parents=True)
    for name in ('setup.sh', '.env.example', 'apps.example.json', 'README.md', 'Caddyfile.example',
                 'garage.toml', 'upstream-image-exceptions.json', 'temporal-schema.sh', 'temporal.yaml', 'temporal-namespace.sh', 'runtime-revision.txt'):
        shutil.copy2(root / 'community' / name, community / name)
    shutil.copytree(root / 'community/postgres', community / 'postgres')
    (bundle / 'docs/community').mkdir(parents=True)
    for name in ('configuration.md', 'deployment.md', 'backups.md', 'upstream-images.md', 'widget-identity.md'):
        shutil.copy2(root / 'docs/community' / name, bundle / 'docs/community' / name)
    # Image references in a release bundle are immutable. Version variables are
    # useful for source builds only and cannot override these digest locks.
    template = (community / '.env.example').read_text()
    (community / '.env.example').write_text(re.sub(r'^(HELPIN_VERSION|AGENT_RUNTIME_VERSION)=.*\n', '', template, flags=re.M))
    for name in (*license_files, 'ROADMAP.md', 'SECURITY.md'):
        (bundle / name).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(root / name, bundle / name)
    (bundle / 'release-evidence').mkdir()
    for pattern in ('*.cdx.json', 'images-*.txt'):
        for path in artifacts.glob(pattern):
            shutil.copy2(path, bundle / 'release-evidence' / path.name)
    compose = compose_source
    (community / 'compose.yaml').write_text(re.sub(r'^    image: (.+)$', pinned, compose, flags=re.MULTILINE))
    metadata = {'tag': tag, 'source_revision': source_revision, 'runtime_revision': runtime_revision,
                'public_host_evidence': evidence, 'architectures': ['amd64', 'arm64'],
                'archive': f'helpin-{tag}.tar.gz'}
    (artifacts / 'release.json').write_text(json.dumps(metadata, indent=2) + '\n')
    (bundle / 'release-evidence/release.json').write_text(json.dumps(metadata, indent=2) + '\n')
    # The archive is self-contained: no source-only relative documentation links.
    for document in bundle.rglob('*.md'):
        for link in re.findall(r'\[[^\]]*\]\(([^)]+)\)', document.read_text()):
            path = link.split('#', 1)[0]
            if not path or '://' in path or path.startswith(('mailto:', '/')):
                continue
            target = (document.parent / path).resolve()
            if not target.is_relative_to(bundle.resolve()) or not target.exists():
                raise SystemExit(f'Broken operator documentation link: {document.name}: {path}')
    checksum_lines = []
    for path in sorted(bundle.rglob('*')):
        if path.is_file():
            checksum_lines.append(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.relative_to(bundle)}')
    (bundle / 'SHA256SUMS').write_text('\n'.join(checksum_lines) + '\n')
    archive = artifacts / metadata['archive']
    with tarfile.open(archive, 'w:gz') as output:
        output.add(bundle, arcname=bundle.name)
    archive.with_suffix('.gz.sha256').write_text(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
    print(f'Created {archive.name}; every service image is digest-pinned')
