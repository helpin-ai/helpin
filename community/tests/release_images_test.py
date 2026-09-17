import gzip
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


class ReleaseImagesTest(unittest.TestCase):
    def test_only_matching_complete_tested_images_can_be_pushed(self):
        source = Path(__file__).resolve().parents[1]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifacts = root / 'release-artifacts'
            artifacts.mkdir()
            for name in ('compose.yaml', 'release-images.sh', 'runtime-revision.txt'):
                shutil.copy2(source / name, root / name)
            images = sorted(set(re.findall(r'^    image: (ghcr.io/helpin-ai/[a-z-]+):', (root / 'compose.yaml').read_text(), re.M)))
            refs = [image + ':community-v0.1.0-rc.1' for image in images]
            inventory = {ref: {'id': 'sha256:' + 'a' * 64, 'architecture': 'amd64'} for ref in refs}
            (artifacts / 'application-images.txt').write_text('\n'.join(refs) + '\n')
            (artifacts / 'source-amd64.txt').write_text('b' * 40)
            (artifacts / 'runtime-amd64.txt').write_text((root / 'runtime-revision.txt').read_text())
            archive = artifacts / 'tested-images.tar.gz'
            archive.write_bytes(gzip.compress(b'tested archive'))
            import hashlib
            (artifacts / 'tested-images.tar.gz.sha256').write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  tested-images.tar.gz\n')
            binary = root / 'bin'
            binary.mkdir()
            git = binary / 'git'
            git.write_text('#!/bin/sh\necho ' + 'b' * 40 + '\n')
            git.chmod(0o755)
            docker = binary / 'docker'
            docker.write_text('''#!/usr/bin/env python3
import json, sys
from pathlib import Path
args = sys.argv[1:]
if args == ['load']:
    sys.stdin.buffer.read()
elif args[:2] == ['image', 'inspect']:
    if '--format' in args:
        print(args[-1].split(':')[0] + '@sha256:' + 'a' * 64)
    else:
        print(json.dumps([{'Id': 'sha256:' + 'a' * 64, 'Architecture': 'amd64'}]))
elif args[0] == 'push':
    with Path('pushes').open('a') as output:
        output.write(args[1] + '\\n')
''')
            docker.chmod(0o755)
            env = {**os.environ, 'PATH': str(binary) + os.pathsep + os.environ['PATH'],
                   'COMMUNITY_ARCH': 'amd64', 'COMMUNITY_RELEASE_TAG': 'community-v0.1.0-rc.1'}
            tested = artifacts / 'tested-applications.json'
            for bad in ({refs[0]: inventory[refs[0]]}, {**inventory, refs[0]: {'id': 'different', 'architecture': 'amd64'}},
                        {**inventory, refs[0]: {'id': 'sha256:' + 'a' * 64, 'architecture': 'arm64'}}):
                tested.write_text(json.dumps(bad))
                rejected = subprocess.run(['bash', str(root / 'release-images.sh')], cwd=root, env=env, capture_output=True)
                self.assertNotEqual(rejected.returncode, 0)
                self.assertFalse((root / 'pushes').exists())
            tested.write_text(json.dumps(inventory))
            subprocess.run(['bash', str(root / 'release-images.sh')], cwd=root, env=env, capture_output=True, check=True)
            self.assertEqual(len((root / 'pushes').read_text().splitlines()), len(refs))
            self.assertEqual(len((artifacts / 'images-amd64.txt').read_text().splitlines()), len(refs))


if __name__ == '__main__':
    unittest.main()
