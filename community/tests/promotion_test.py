import hashlib
import io
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('promotion', Path(__file__).resolve().parents[1] / 'promote-release.py')
promotion = importlib.util.module_from_spec(spec)
spec.loader.exec_module(promotion)


class PromotionTest(unittest.TestCase):
    def setUp(self):
        public = patch.object(promotion, 'assert_public_repository')
        public.start()
        self.addCleanup(public.stop)
        assets = patch.object(promotion, 'verify_public_assets')
        assets.start()
        self.addCleanup(assets.stop)
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.metadata = {'tag': 'community-v0.1.0-rc.1', 'source_revision': 'a' * 40,
                         'runtime_revision': 'b' * 40, 'architectures': ['amd64', 'arm64'],
                         'public_host_evidence': 'https://example.test/acceptance',
                         'archive': 'helpin-community-v0.1.0-rc.1.tar.gz'}
        self.run = {'conclusion': 'success', 'event': 'workflow_dispatch',
                    'path': '.github/workflows/community-release.yml',
                    'repository': {'full_name': 'helpin-ai/helpin'},
                    'head_repository': {'full_name': 'helpin-ai/helpin'}, 'head_sha': 'a' * 40}
        self.metadata['cli_assets'] = {}
        for name in [f'helpin-{system}-{arch}' for system in ('linux', 'darwin') for arch in ('amd64', 'arm64')] + ['install.sh']:
            data = ('fixture ' + name).encode()
            (self.root / name).write_bytes(data)
            digest = hashlib.sha256(data).hexdigest()
            self.metadata['cli_assets'][name] = digest
            (self.root / (name + '.sha256')).write_text(f'{digest}  {name}\n')
        (self.root / 'release.json').write_text(json.dumps(self.metadata))
        archive = self.root / self.metadata['archive']
        archive.write_bytes(b'exact tested artifact')
        archive.with_suffix('.gz.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive.name + '\n')

    def test_accepts_matching_successful_candidate(self):
        metadata, archive, _ = promotion.validate(self.root, self.run, 'helpin-ai/helpin')
        self.assertEqual(metadata, self.metadata)
        self.assertEqual(archive.read_bytes(), b'exact tested artifact')

    def test_rejects_wrong_workflow_revision_fork_or_failure(self):
        for key, value in (('path', '.github/workflows/ci.yml'), ('head_sha', 'c' * 40),
                           ('conclusion', 'failure'), ('head_repository', {'full_name': 'fork/helpin'})):
            with self.subTest(key=key), self.assertRaises(ValueError):
                promotion.validate(self.root, {**self.run, key: value}, 'helpin-ai/helpin')

    def test_rejects_modified_archive(self):
        (self.root / self.metadata['archive']).write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'checksum'):
            promotion.validate(self.root, self.run, 'helpin-ai/helpin')

    def test_rejects_modified_cli_before_publication(self):
        (self.root / 'helpin-linux-amd64').write_bytes(b'tampered')
        with self.assertRaisesRegex(ValueError, 'CLI checksum'):
            promotion.validate(self.root, self.run, 'helpin-ai/helpin')

    def test_existing_release_or_network_failure_never_publishes(self):
        for result in (subprocess.CompletedProcess([], 0, '{}'), subprocess.CompletedProcess([], 1, 'unavailable')):
            with patch.object(promotion.subprocess, 'check_output', return_value=json.dumps(self.run).encode()):
                with patch.object(promotion.subprocess, 'run', return_value=result) as call:
                    with self.assertRaises(ValueError):
                        promotion.promote(self.root, '123', 'helpin-ai/helpin')
                    self.assertTrue(all(args[0][:2] == ['gh', 'api'] for args, _ in call.call_args_list))

    def test_publication_uses_exact_assets_and_commit(self):
        def invoke(args, **kwargs):
            return subprocess.CompletedProcess(args, 1 if args[1] == 'api' else 0, '{"status":"404"}')
        with patch.object(promotion.subprocess, 'check_output', return_value=json.dumps(self.run).encode()):
            with patch.object(promotion.subprocess, 'run', side_effect=invoke) as call:
                promotion.promote(self.root, '123', 'helpin-ai/helpin')
                command = call.call_args.args[0]
                self.assertEqual(command[:3], ['gh', 'release', 'create'])
                self.assertIn(str(self.root / self.metadata['archive']), command)
                self.assertEqual(command[command.index('--target') + 1], 'a' * 40)
                self.assertIn(str(self.root / 'helpin-linux-arm64.sha256'), command)


class PublicDistributionTest(unittest.TestCase):
    def test_private_or_unreachable_repository_is_rejected(self):
        with patch.object(promotion.urllib.request, 'urlopen', side_effect=OSError('HTTP 404')):
            with self.assertRaisesRegex(ValueError, 'Public release downloads'):
                promotion.assert_public_repository('helpin-ai/helpin')
        for value in ({'private': True, 'full_name': 'helpin-ai/helpin'}, {'private': False, 'full_name': 'someone/else'}):
            with patch.object(promotion.urllib.request, 'urlopen', return_value=io.BytesIO(json.dumps(value).encode())):
                with self.assertRaises(ValueError):
                    promotion.assert_public_repository('helpin-ai/helpin')

    def test_public_repository_uses_anonymous_request(self):
        with patch.object(promotion.urllib.request, 'urlopen', return_value=io.BytesIO(b'{"private":false,"full_name":"helpin-ai/helpin"}')) as call:
            promotion.assert_public_repository('helpin-ai/helpin')
            self.assertEqual(call.call_args.args, ('https://api.github.com/repos/helpin-ai/helpin',))

    def test_public_assets_are_downloaded_and_verified(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            metadata = {'tag': 'community-v0.2.0', 'archive': 'bundle.tar.gz', 'cli_assets': {'helpin-linux-amd64': 'fixture'}}
            for name in ('bundle.tar.gz', 'bundle.tar.gz.sha256', 'release.json', 'helpin-linux-amd64', 'helpin-linux-amd64.sha256'):
                (root / name).write_bytes(name.encode())
            with patch.object(promotion.urllib.request, 'urlopen', side_effect=lambda url, **kw: io.BytesIO(url.rsplit('/', 1)[1].encode())):
                promotion.verify_public_assets(root, metadata, 'helpin-ai/helpin')
            with patch.object(promotion.urllib.request, 'urlopen', return_value=io.BytesIO(b'corrupt')):
                with self.assertRaisesRegex(ValueError, 'checksum'):
                    promotion.verify_public_assets(root, metadata, 'helpin-ai/helpin')


if __name__ == '__main__':
    unittest.main()
