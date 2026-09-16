import unittest
from pathlib import Path
import re
from checks import GROUPS, JOBS, failures, select


class ChecksTest(unittest.TestCase):
    def test_dependency_selection(self):
        for path, groups in {
            'frontend/src/lib/supportInboxFilters.ts': ('mobile', 'desktop', 'frontend', 'community'),
            'packages/shared/src/types.ts': ('packages', 'mobile', 'frontend', 'helpcenter', 'community'),
            'packages/widget-core/src/index.ts': ('mobile', 'frontend', 'helpcenter', 'community'),
            'server/internal/model/user.go': ('server', 'community'),
            'community/compose.yaml': ('community',),
        }.items():
            with self.subTest(path=path):
                selected = select([path])
                for group in groups:
                    self.assertTrue(selected[group], group)
        self.assertFalse(any(select(['docs/design.md']).values()))
        for path in ('pnpm-lock.yaml', 'tsconfig.base.json', '.github/workflows/community-test.yml', 'scripts/ci/checks.py'):
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
