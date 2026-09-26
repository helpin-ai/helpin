"""Exercise the real bootstrap with a fixture downloader; never fetch/run a remote CLI."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class BootstrapTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.bin = self.root / 'tools'
        self.bin.mkdir()
        self.destination = self.root / 'bin with spaces'
        self.destination.mkdir()
        self.env = {**os.environ, 'PATH': f'{self.bin}:{os.environ["PATH"]}',
                    'HELPIN_BIN_DIR': str(self.destination), 'HELPIN_VERSION': 'community-v0.1.0-rc.1',
                    'FIXTURE': str(self.root), 'TMPDIR': str(self.root)}
        self.cli = b'#!/bin/sh\necho fixture CLI\n'
        for system in ('linux', 'darwin'):
            for arch in ('amd64', 'arm64'):
                name = f'helpin-{system}-{arch}'
                (self.root / name).write_bytes(self.cli)
                (self.root / (name + '.sha256')).write_text(f'{hashlib.sha256(self.cli).hexdigest()}  {name}\n')
        self.tool('curl', '''#!/bin/bash
url="${@: -1}"
case "$url" in
  *api.github.com*) printf '  "tag_name": "cloud-v1",\n  "tag_name": "community-v0.1.0-rc.1",\n' ;;
  *) cat "$FIXTURE/${url##*/}" ;;
esac
''')
        self.platform('Linux', 'x86_64')

    def tool(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def platform(self, system, arch):
        self.tool('uname', f'#!/bin/sh\nif [ "$1" = -s ]; then echo {system}; else echo {arch}; fi\n')

    def run_installer(self):
        # Same stdin arrangement as curl | bash, without executing downloaded code.
        return subprocess.run(['bash'], input=(ROOT / 'community/install.sh').read_text(),
                              env=self.env, capture_output=True, text=True)

    def test_platform_selection_and_atomic_install(self):
        for system, arch in (('Linux', 'x86_64'), ('Linux', 'aarch64'), ('Darwin', 'x86_64'), ('Darwin', 'arm64')):
            with self.subTest(system=system, arch=arch):
                self.platform(system, arch)
                result = self.run_installer()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual((self.destination / 'helpin').read_bytes(), self.cli)
                self.assertEqual((self.destination / 'helpin').stat().st_mode & 0o777, 0o755)
                self.assertIn('install', result.stdout)
                self.assertFalse(list(self.destination.glob('.helpin-download.*')))

    def test_corrupt_download_preserves_existing_cli(self):
        (self.destination / 'helpin').write_bytes(b'previous')
        (self.root / 'helpin-linux-amd64').write_bytes(b'corrupt')
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('checksum mismatch', result.stderr)
        self.assertEqual((self.destination / 'helpin').read_bytes(), b'previous')
        self.assertNotIn('unbound variable', result.stderr)

    def test_missing_asset_fails_without_installing(self):
        (self.root / 'helpin-linux-amd64.sha256').unlink()
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.destination / 'helpin').exists())
        self.assertNotIn('unbound variable', result.stderr)

    def test_community_prerelease_discovery(self):
        self.env.pop('HELPIN_VERSION')
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('community-v0.1.0-rc.1', result.stdout)

    def test_invalid_platform_or_version_fails(self):
        self.platform('Linux', 'i686')
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.platform('Linux', 'x86_64')
        self.env['HELPIN_VERSION'] = '../../bad'
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse((self.destination / 'helpin').exists())

    def test_unavailable_public_releases_explains_failure_and_preserves_cli(self):
        self.env.pop('HELPIN_VERSION')
        (self.destination / 'helpin').write_bytes(b'previous')
        self.tool('curl', '#!/bin/sh\nexit 22\n')
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('public Community releases are unavailable', result.stderr)
        self.assertEqual((self.destination / 'helpin').read_bytes(), b'previous')

    def test_website_serves_identical_bootstrap(self):
        self.assertEqual((ROOT / 'website/public/install.sh').read_bytes(),
                         (ROOT / 'community/install.sh').read_bytes())


if __name__ == '__main__':
    unittest.main()
