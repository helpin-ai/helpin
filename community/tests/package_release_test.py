import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest


class ReleasePackageTest(unittest.TestCase):
    def test_digest_lock_checksums_and_secret_exclusion(self):
        source = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            shutil.copytree(source / 'community', root / 'community', ignore=shutil.ignore_patterns('.env', 'apps.json', 'backups', '__pycache__', 'release-artifacts'))
            shutil.copytree(source / 'docs/community', root / 'docs/community')
            license_files = ('LICENSE', 'LICENSE-AGPL-3.0', 'LICENSE-APACHE-2.0', 'ee/LICENSE')
            for name in (*license_files, 'ROADMAP.md', 'SECURITY.md'):
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source / name, root / name)
            artifacts = root / 'release-artifacts'
            artifacts.mkdir()
            assets = {}
            for name in [f'helpin-{system}-{arch}' for system in ('linux', 'darwin') for arch in ('amd64', 'arm64')] + ['install.sh']:
                data = ('fixture ' + name).encode()
                (artifacts / name).write_bytes(data)
                assets[name] = hashlib.sha256(data).hexdigest()
                (artifacts / (name + '.sha256')).write_text(f'{assets[name]}  {name}\n')
            (artifacts / 'cli-assets.json').write_text(json.dumps(assets))
            (artifacts / '.env').write_text('MUST_NOT_SHIP=fixture-only\n')
            (root / 'community/.env').write_text('MUST_NOT_SHIP=fixture-only\n')
            expected = set(re.findall(r'^    image: (ghcr.io/helpin-ai/[a-z-]+):', (root / 'community/compose.yaml').read_text(), re.M))
            for arch in ('amd64', 'arm64'):
                (artifacts / f'images-{arch}.txt').write_text(''.join(image + '@sha256:' + 'a' * 64 + '\n' for image in sorted(expected)))
                (artifacts / f'source-{arch}.txt').write_text('c' * 40)
                (artifacts / f'runtime-{arch}.txt').write_text((root / 'community/runtime-revision.txt').read_text())
            (root / 'bin').mkdir()
            git = root / 'bin/git'
            git.write_text('#!/bin/sh\necho ' + 'c' * 40 + '\n')
            git.chmod(0o755)
            docker = root / 'bin/docker'
            docker.write_text('#!/bin/sh\nif [ "$3" = inspect ]; then printf "sha256:%s\\n" "' + 'b' * 64 + '"; fi\n')
            docker.chmod(0o755)
            env = {**os.environ, 'PATH': f'{root}/bin:{os.environ["PATH"]}', 'COMMUNITY_RELEASE_TAG': 'community-v0.1.0-rc.1', 'PUBLIC_HOST_EVIDENCE': 'https://example.test/acceptance',
                   'COMMUNITY_UPGRADE_FROM': 'community-v0.0.9', 'UPGRADE_EVIDENCE': 'https://example.test/upgrade-recovery'}
            subprocess.run(['python3', str(root / 'community/package-release.py')], env=env, check=True, capture_output=True)
            archive = next(artifacts.glob('*.tar.gz'))
            with tarfile.open(archive) as package:
                metadata = json.load(package.extractfile('helpin-community/release-evidence/release.json'))
                self.assertEqual(metadata['upgrade_from'], ['community-v0.0.9'])
                self.assertEqual(metadata['upgrade_evidence'], 'https://example.test/upgrade-recovery')
                names = package.getnames()
                self.assertIn('helpin-community/docs/community/troubleshooting.md', names)
                for name in license_files:
                    self.assertEqual(package.extractfile('helpin-community/' + name).read(),
                                     (source / name).read_bytes())
                self.assertFalse(any(Path(name).name in ('.env', 'apps.json', 'PUBLICATION.md', 'implementation-status.md', 'development.md') for name in names))
                self.assertFalse(any('/tests/' in name for name in names))
                compose = package.extractfile('helpin-community/community/compose.yaml').read().decode()
                images = [line for line in compose.splitlines() if line.startswith('    image:')]
                self.assertGreater(len(images), 10)
                self.assertTrue(all('@sha256:' in line for line in images))
                sums = package.extractfile('helpin-community/SHA256SUMS').read().decode().splitlines()
                for line in sums:
                    digest, name = line.split('  ', 1)
                    self.assertEqual(hashlib.sha256(package.extractfile('helpin-community/' + name).read()).hexdigest(), digest)
            for changes in ({'UPGRADE_EVIDENCE': ''}, {'COMMUNITY_UPGRADE_FROM': '../bad'},
                            {'COMMUNITY_UPGRADE_FROM': 'community-v0.1.0-rc.1'},
                            {'COMMUNITY_UPGRADE_FROM': 'community-v0.0.9,community-v0.0.9'}):
                rejected = subprocess.run(['python3', str(root / 'community/package-release.py')],
                                          env={**env, **changes}, capture_output=True, text=True)
                self.assertNotEqual(rejected.returncode, 0)
                self.assertIn('upgrade', rejected.stderr.lower())
            # Reject incomplete inventories before any manifest can be published.
            (artifacts / 'images-arm64.txt').write_text(next(iter(expected)) + '@sha256:' + 'a' * 64 + '\n')
            rejected = subprocess.run(['python3', str(root / 'community/package-release.py')], env=env, capture_output=True, text=True)
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn('Incomplete', rejected.stderr)


if __name__ == '__main__':
    unittest.main()
