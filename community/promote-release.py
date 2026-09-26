#!/usr/bin/env python3
"""Publish an approved candidate's exact assets; never build or replace a release."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time
import urllib.request


def validate(directory, run, repository):
    metadata = json.loads((directory / 'release.json').read_text())
    tag = metadata['tag']
    if not re.fullmatch(r'community-v0\.\d+\.\d+(?:-[a-z0-9.]+)?', tag):
        raise ValueError('Invalid candidate tag')
    if (run.get('conclusion') != 'success' or run.get('event') != 'workflow_dispatch'
            or run.get('path') != '.github/workflows/community-release.yml'
            or run.get('repository', {}).get('full_name') != repository
            or run.get('head_repository', {}).get('full_name') != repository
            or run.get('head_sha') != metadata['source_revision']):
        raise ValueError('Candidate must be a successful release workflow for the same repository and source commit')
    if metadata.get('architectures') != ['amd64', 'arm64']:
        raise ValueError('Both tested architectures are required')
    if not re.fullmatch(r'[a-f0-9]{40}', metadata.get('runtime_revision', '')):
        raise ValueError('Missing Runtime provenance')
    if not re.fullmatch(r'https://\S+', metadata.get('public_host_evidence', '')):
        raise ValueError('Missing external HTTPS acceptance record')
    name = f'helpin-{tag}.tar.gz'
    if metadata['archive'] != name:
        raise ValueError('Archive name does not match candidate tag')
    archive = directory / name
    checksum = directory / (name + '.sha256')
    expected = f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {name}'
    if checksum.read_text().strip() != expected:
        raise ValueError('Candidate archive checksum does not match')
    expected_cli = {'helpin-' + system + '-' + arch for system in ('linux', 'darwin') for arch in ('amd64', 'arm64')} | {'install.sh'}
    assets = metadata.get('cli_assets', {})
    if set(assets) != expected_cli:
        raise ValueError('Incomplete CLI asset inventory')
    for name, digest in assets.items():
        if hashlib.sha256((directory / name).read_bytes()).hexdigest() != digest:
            raise ValueError(f'CLI checksum mismatch: {name}')
        if (directory / (name + '.sha256')).read_text().strip() != f'{digest}  {name}':
            raise ValueError(f'CLI checksum file mismatch: {name}')
    return metadata, archive, checksum


def assert_public_repository(repository):
    """Use no GitHub token: installers must work for unauthenticated operators."""
    try:
        with urllib.request.urlopen(f'https://api.github.com/repos/{repository}', timeout=30) as response:
            metadata = json.load(response)
    except (OSError, ValueError) as error:
        raise ValueError('Public release downloads are unavailable; publish an approved public repository before releasing the installer') from error
    if metadata.get('private') is not False or metadata.get('full_name', '').lower() != repository.lower():
        raise ValueError('Installer requires the intended repository to be publicly readable')


def verify_public_assets(directory, metadata, repository):
    names = [metadata['archive'], metadata['archive'] + '.sha256', 'release.json']
    names += list(metadata['cli_assets'])
    names += [name + '.sha256' for name in metadata['cli_assets']]
    for name in names:
        expected = hashlib.sha256((directory / name).read_bytes()).hexdigest()
        url = f"https://github.com/{repository}/releases/download/{metadata['tag']}/{name}"
        for attempt in range(4):
            try:
                digest = hashlib.sha256()
                with urllib.request.urlopen(url, timeout=60) as response:
                    while chunk := response.read(1024 * 1024):
                        digest.update(chunk)
                if digest.hexdigest() != expected:
                    raise ValueError(f'Public asset checksum mismatch: {name}')
                break
            except OSError as error:
                if attempt == 3:
                    raise ValueError(f'Release was created, but public download failed: {name}; repair access before announcing it') from error
                time.sleep(3)


def assert_unpublished(tag, repository):
    if not re.fullmatch(r'community-v0\.\d+\.\d+(?:-[a-z0-9.]+)?', tag):
        raise ValueError('Invalid candidate tag')
    assert_public_repository(repository)
    # A tag can already point at another commit even without a release. Refuse
    # both cases. gh release create also fails if a release is created concurrently.
    for endpoint in (f'repos/{repository}/releases/tags/{tag}', f'repos/{repository}/git/ref/tags/{tag}'):
        result = subprocess.run(['gh', 'api', endpoint], capture_output=True, text=True)
        if result.returncode == 0:
            raise ValueError('Release or tag already exists; review manually instead of overwriting')
        try:
            status = json.loads(result.stdout).get('status')
        except (ValueError, AttributeError):
            status = None
        if str(status) != '404':
            raise ValueError('Could not confirm release/tag absence; refusing publication')


def promote(directory, run_id, repository):
    if not re.fullmatch(r'[0-9]+', run_id) or not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Invalid candidate run or repository')
    run = json.loads(subprocess.check_output(['gh', 'api', f'repos/{repository}/actions/runs/{run_id}']))
    metadata, archive, checksum = validate(directory, run, repository)
    tag = metadata['tag']
    assert_unpublished(tag, repository)
    notes = directory / 'release-notes.md'
    notes.write_text(f"Community 0.x beta.\n\nSource: `{metadata['source_revision']}`\n\n"
                     f"Runtime: `{metadata['runtime_revision']}`\n\n"
                     f"Tested natively on amd64 and arm64.\n\n"
                     f"HTTPS acceptance: {metadata['public_host_evidence']}\n\n"
                     f"Verify the archive with `sha256sum -c {checksum.name}`, extract it, "
                     "and follow `helpin-community/community/README.md`.\n")
    cli_files = [str(directory / name) for name in metadata['cli_assets']]
    cli_files += [str(directory / (name + '.sha256')) for name in metadata['cli_assets']]
    subprocess.run(['gh', 'release', 'create', tag, str(archive), str(checksum), str(directory / 'release.json'), *cli_files,
                    '--repo', repository, '--target', metadata['source_revision'], '--prerelease',
                    '--title', f'Helpin {tag}', '--notes-file', str(notes)], check=True)
    verify_public_assets(directory, metadata, repository)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('check', 'promote'), nargs='?', default='promote')
    if parser.parse_args().command == 'check':
        assert_unpublished(os.environ['COMMUNITY_RELEASE_TAG'], os.environ['GITHUB_REPOSITORY'])
    else:
        promote(Path('release-artifacts'), os.environ['CANDIDATE_RUN_ID'], os.environ['GITHUB_REPOSITORY'])
