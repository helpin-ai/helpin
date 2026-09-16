#!/usr/bin/env python3
"""Publish an approved candidate's exact assets; never build or replace a release."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess


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
    return metadata, archive, checksum


def promote(directory, run_id, repository):
    if not re.fullmatch(r'[0-9]+', run_id) or not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise ValueError('Invalid candidate run or repository')
    run = json.loads(subprocess.check_output(['gh', 'api', f'repos/{repository}/actions/runs/{run_id}']))
    metadata, archive, checksum = validate(directory, run, repository)
    tag = metadata['tag']
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
    notes = directory / 'release-notes.md'
    notes.write_text(f"Community 0.x beta.\n\nSource: `{metadata['source_revision']}`\n\n"
                     f"Runtime: `{metadata['runtime_revision']}`\n\n"
                     f"Tested natively on amd64 and arm64.\n\n"
                     f"HTTPS acceptance: {metadata['public_host_evidence']}\n\n"
                     f"Verify the archive with `sha256sum -c {checksum.name}`, extract it, "
                     "and follow `helpin-community/community/README.md`.\n")
    subprocess.run(['gh', 'release', 'create', tag, str(archive), str(checksum), str(directory / 'release.json'),
                    '--repo', repository, '--target', metadata['source_revision'], '--prerelease',
                    '--title', f'Helpin {tag}', '--notes-file', str(notes)], check=True)


if __name__ == '__main__':
    promote(Path('release-artifacts'), os.environ['CANDIDATE_RUN_ID'], os.environ['GITHUB_REPOSITORY'])
