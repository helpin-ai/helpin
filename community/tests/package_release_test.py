import hashlib
import os
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
            shutil.copytree(source / 'community', root / 'community', ignore=shutil.ignore_patterns('.env', 'apps.json', 'backups', '__pycache__'))
            shutil.copytree(source / 'docs/community', root / 'docs/community')
            license_files = ('LICENSE', 'LICENSE-AGPL-3.0', 'LICENSE-APACHE-2.0', 'ee/LICENSE')
            for name in (*license_files, 'ROADMAP.md', 'SECURITY.md'):
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source / name, root / name)
            artifacts = root / 'release-artifacts'
            artifacts.mkdir()
            (artifacts / '.env').write_text('MUST_NOT_SHIP=fixture-only\n')
            (root / 'community/.env').write_text('MUST_NOT_SHIP=fixture-only\n')
            for arch in ('amd64', 'arm64'):
                (artifacts / f'images-{arch}.txt').write_text('ghcr.io/helpin-ai/helpin-community-api@sha256:' + 'a' * 64 + '\n')
            (root / 'bin').mkdir()
            docker = root / 'bin/docker'
            docker.write_text('#!/bin/sh\nif [ "$3" = inspect ]; then printf "sha256:%s\\n" "' + 'b' * 64 + '"; fi\n')
            docker.chmod(0o755)
            env = {**os.environ, 'PATH': f'{root}/bin:{os.environ["PATH"]}', 'COMMUNITY_RELEASE_TAG': 'community-v0.1.0-rc.1'}
            subprocess.run(['python3', str(root / 'community/package-release.py')], env=env, check=True, capture_output=True)
            archive = next(artifacts.glob('*.tar.gz'))
            with tarfile.open(archive) as package:
                names = package.getnames()
                for name in license_files:
                    self.assertEqual(package.extractfile('helpin-community/' + name).read(),
                                     (source / name).read_bytes())
                self.assertFalse(any(Path(name).name in ('.env', 'apps.json') for name in names))
                compose = package.extractfile('helpin-community/community/compose.yaml').read().decode()
                images = [line for line in compose.splitlines() if line.startswith('    image:')]
                self.assertGreater(len(images), 10)
                self.assertTrue(all('@sha256:' in line for line in images))
                sums = package.extractfile('helpin-community/SHA256SUMS').read().decode().splitlines()
                for line in sums:
                    digest, name = line.split('  ', 1)
                    self.assertEqual(hashlib.sha256(package.extractfile('helpin-community/' + name).read()).hexdigest(), digest)


if __name__ == '__main__':
    unittest.main()
