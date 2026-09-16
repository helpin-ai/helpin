import unittest
from pathlib import Path
import re
import json
import os
import tempfile
import textwrap
from unittest.mock import patch
from checks import GROUPS, JOBS, failures, select


class ChecksTest(unittest.TestCase):
    def test_dependency_selection(self):
        for path, groups in {
            'frontend/src/lib/supportInboxFilters.ts': ('mobile', 'desktop', 'frontend', 'community'),
            'packages/shared/src/types.ts': ('packages', 'mobile', 'frontend', 'helpcenter', 'community'),
            'packages/widget-core/src/index.ts': ('mobile', 'frontend', 'helpcenter', 'community'),
            'server/internal/model/user.go': ('server', 'community'),
            'server/skills/support.md': ('server', 'community'),
            'community/compose.yaml': ('community',),
        }.items():
            with self.subTest(path=path):
                selected = select([path])
                for group in groups:
                    self.assertTrue(selected[group], group)
        self.assertFalse(any(select(['docs/design.md', 'community/README.md', '.github/ISSUE_TEMPLATE/bug.yml']).values()))
        for path in ('pnpm-lock.yaml', 'tsconfig.base.json', '.dockerignore', '.npmrc', '.github/workflows/community-test.yml', 'scripts/ci/checks.py'):
            self.assertTrue(all(select([path]).values()))
        self.assertTrue(all(select([], all_checks=True).values()))

    def test_toolchain_pins_match_community_images(self):
        root = Path(__file__).resolve().parents[2]
        dockerfile = (root / 'community/images/Dockerfile').read_text()
        self.assertIn('golang:' + (root / '.go-version').read_text().strip() + '-bookworm', dockerfile)
        self.assertIn('node:' + (root / '.node-version').read_text().strip() + '-bookworm-slim', dockerfile)

    def test_every_selected_job_is_in_final_gate(self):
        workflow = (Path(__file__).resolve().parents[2] / '.github/workflows/ci.yml').read_text()
        final = workflow.split('  required:', 1)[1]
        for jobs in JOBS.values():
            for job in jobs:
                self.assertRegex(workflow, r'(?m)^  ' + re.escape(job) + ':$')
                self.assertRegex(final, r'(?<![a-z-])' + re.escape(job) + r'(?![a-z-])')

    def test_forks_only_read_docker_caches(self):
        root = Path(__file__).resolve().parents[2]
        workflow = (root / '.github/workflows/community-test.yml').read_text()
        body = textwrap.dedent(workflow.split("python3 - <<'PY'\n", 1)[1].split('\n          PY', 1)[0])
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / 'community-bake.json').write_text(json.dumps({'target': {'helpin-api': {}, 'agent-runtime': {}}}))
            for trusted in ('true', 'false'):
                output = directory / trusted
                with patch.dict(os.environ, {'RUNNER_TEMP': temporary, 'GITHUB_OUTPUT': str(output), 'COMMUNITY_ARCH': 'amd64', 'TRUSTED': trusted}):
                    exec(compile(body, 'cache-policy', 'exec'), {})
                lines = output.read_text().splitlines()
                self.assertEqual(lines[0], 'cache<<CACHE')
                self.assertEqual(lines[-1], 'CACHE')
                self.assertEqual(sum('.cache-from=' in line for line in lines), 2)
                self.assertEqual(sum('.cache-to=' in line for line in lines), 2 if trusted == 'true' else 0)

    def test_required_status(self):
        needs = {'changes': {'result': 'success', 'outputs': {k: 'false' for k in GROUPS}},
                 'workflow-checks': {'result': 'success'}}
        self.assertEqual(failures(needs), [])
        needs['changes']['outputs']['server'] = 'true'
        for job in JOBS['server']:
            needs[job] = {'result': 'success'}
        self.assertEqual(failures(needs), [])
        for status in ('failure', 'cancelled', 'skipped'):
            needs['server-tests']['result'] = status
            self.assertIn('server-tests', failures(needs))
        needs['server-tests']['result'] = 'success'
        del needs['changes']['outputs']['mobile']
        self.assertIn('missing selection: mobile', failures(needs))


if __name__ == '__main__':
    unittest.main()
