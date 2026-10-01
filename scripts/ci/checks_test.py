import unittest
from pathlib import Path
import re
import json
import os
import tempfile
import textwrap
import subprocess
from unittest.mock import patch
from checks import GROUPS, JOBS, failures, select


class ChecksTest(unittest.TestCase):
    def test_node_is_ready_before_pnpm_bootstrap(self):
        root = Path(__file__).resolve().parents[2]
        for workflow in (root / '.github/workflows').glob('*.yml'):
            # Keep this policy check dependency-free, like the rest of CI checks.
            for job in re.split(r'(?m)^  [\w-]+:\n', workflow.read_text()):
                node_ready = False
                for step in re.split(r'(?m)^      - ', job):
                    if re.search(r'uses: actions/setup-node@', step):
                        node_ready = True
                        self.assertNotRegex(step, r'(?m)^          cache:', workflow)
                        self.assertIn('package-manager-cache: false', step, workflow)
                    if re.search(r'uses: pnpm/action-setup@', step):
                        # v6 otherwise auto-selects @pnpm/exe on older runners,
                        # which requires libatomic even with standalone=false.
                        self.assertTrue(node_ready, workflow)
                        if 'cache: true' in step:
                            self.assertIn('cache_dependency_path:', step, workflow)
        community = (root / '.github/workflows/community-test.yml').read_text()
        pnpm = next(step for step in re.split(r'(?m)^      - ', community)
                    if 'uses: pnpm/action-setup@' in step)
        self.assertIn("if: inputs.level == 'full'", pnpm)
        self.assertIn('cache_dependency_path: helpin/pnpm-lock.yaml', pnpm)

    def test_external_actions_are_immutable(self):
        root = Path(__file__).resolve().parents[2]
        for workflow in (root / '.github/workflows').glob('*.yml'):
            text = workflow.read_text()
            self.assertNotIn('ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION', text, workflow)
            self.assertRegex(text, r'(?m)^permissions:\n  contents: read$', workflow)
            for reference in re.findall(r'^\s*(?:- )?uses: ([^\s]+)', text, re.M):
                if reference.startswith('./'):
                    continue
                self.assertRegex(reference, r'^[\w.-]+/[\w./-]+@[a-f0-9]{40}$', workflow)

    def test_tool_installer_rejects_bad_checksum_before_extraction(self):
        installer = Path(__file__).with_name('install-tool.sh')
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            # Emulate a corrupted download, without downloading or running a tool.
            curl = directory / 'curl'
            curl.write_text('#!/bin/bash\nprintf corrupt > "${@: -1}"\n')
            curl.chmod(0o755)
            output = directory / 'github-path'
            for tool in ('doppler', 'trivy'):
                with self.subTest(tool=tool):
                    result = subprocess.run(['bash', str(installer), tool],
                        env={**os.environ, 'PATH': temporary + os.pathsep + os.environ['PATH'],
                             'RUNNER_TEMP': temporary, 'GITHUB_PATH': str(output)},
                        capture_output=True, text=True)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('FAILED', result.stdout)
                    self.assertFalse(output.exists())
                    self.assertEqual(list(directory.glob('tool-*')), [])

    def test_dependency_selection(self):
        for path, groups in {
            'frontend/src/lib/supportInboxFilters.ts': ('mobile', 'desktop', 'frontend'),
            'packages/shared/src/types.ts': ('packages', 'mobile', 'frontend', 'helpcenter'),
            'packages/widget-core/src/index.ts': ('mobile', 'frontend', 'helpcenter'),
            'server/internal/model/user.go': ('server',),
            'server/skills/support.md': ('server',),
        }.items():
            with self.subTest(path=path):
                selected = select([path])
                for group in groups:
                    self.assertTrue(selected[group], group)
        self.assertFalse(any(select(['docs/design.md', 'community/README.md', '.github/ISSUE_TEMPLATE/bug.yml']).values()))
        for path in ('pnpm-lock.yaml', 'tsconfig.base.json', '.dockerignore', '.npmrc', '.github/workflows/community-test.yml', 'scripts/ci/checks.py'):
            self.assertTrue(all(select([path]).values()))
        self.assertTrue(all(select([], all_checks=True).values()))

    def test_unrelated_jobs_are_not_selected(self):
        for paths, expected in (
            (['docs/community/development.md'], set()),
            (['apps/admin/src/App.tsx'], {'admin'}),
            (['apps/support-mobile/src/App.tsx'], {'mobile'}),
            (['events-pipeline/src/main.rs'], {'eventpipeline'}),
            (['server/internal/model/user.go'], {'server'}),
            (['community/compose.yaml'], set()),
            (['frontend/src/pages/support/Inbox.tsx'], {'frontend', 'desktop', 'mobile'}),
            (['server/internal/model/user.go', 'apps/admin/src/App.tsx'], {'server', 'admin'}),
        ):
            with self.subTest(paths=paths):
                self.assertEqual({group for group, selected in select(paths).items() if selected}, expected)

    def test_pr_checks_do_not_build_community_images(self):
        root = Path(__file__).resolve().parents[2]
        workflow = (root / '.github/workflows/ci.yml').read_text()
        self.assertNotIn('community-test.yml', workflow)
        self.assertNotIn('docker/bake-action@', workflow)
        self.assertIn('node --test community/tests/installer.test.mjs', workflow)
        self.assertIn("python3 -m unittest discover -s community/tests", workflow)
        for group, job in (('server', 'community-backend'), ('frontend', 'community-frontend')):
            self.assertIn(job, JOBS[group])
        scheduled = (root / '.github/workflows/community-bundle.yml').read_text()
        self.assertIn('schedule:', scheduled)
        self.assertIn('workflow_dispatch:', scheduled)
        self.assertIn('uses: ./.github/workflows/community-test.yml', scheduled)
        release = (root / '.github/workflows/community-release.yml').read_text()
        self.assertIn('arch: [amd64, arm64]', release)
        self.assertIn('export-images: true', release)
        self.assertIn('uses: ./.github/workflows/community-test.yml', release)

    def test_toolchain_pins_match_community_images(self):
        root = Path(__file__).resolve().parents[2]
        dockerfile = (root / 'community/images/Dockerfile').read_text()
        self.assertIn('golang:' + (root / '.go-version').read_text().strip() + '-bookworm', dockerfile)
        node = (root / '.node-version').read_text().strip()
        for filename in ('community/images/Dockerfile', 'frontend/Dockerfile',
                         'help-center/Dockerfile', 'packages/sdk-js/docker/Dockerfile',
                         'community/tests/compose.fixture.yaml'):
            images = re.findall(r'node:([^\s]+)', (root / filename).read_text())
            self.assertTrue(images, filename)
            for image in images:
                self.assertRegex(image, '^' + re.escape(node) + r'-[\w.-]+@sha256:[a-f0-9]{64}$', filename)
        # Artifact-only deploy jobs cannot read a checkout's .node-version.
        for workflow in (root / '.github/workflows').glob('*.yml'):
            for version in re.findall(r'node-version: [\'"]?([\d.]+)', workflow.read_text()):
                self.assertEqual(version, node, workflow)

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
                for export in ('true', 'false'):
                    output = directory / f'{trusted}-{export}'
                    with patch.dict(os.environ, {'RUNNER_TEMP': temporary, 'GITHUB_OUTPUT': str(output), 'COMMUNITY_ARCH': 'amd64',
                                                 'TRUSTED': trusted, 'EXPORT_IMAGES': export}):
                        exec(compile(body, 'cache-policy', 'exec'), {})
                    lines = output.read_text().splitlines()
                    self.assertEqual(lines[0], 'cache<<CACHE')
                    self.assertEqual(lines[-1], 'CACHE')
                    # OS package layers key on the date; release builds also pull fresh base images.
                    self.assertEqual(sum('.cache-from=' in line for line in lines), 2)
                    self.assertEqual(sum(line.endswith('.no-cache=true') for line in lines), 0)
                    self.assertEqual(sum(re.search(r'\.args\.OS_PACKAGES_DATE=\d{4}-\d{2}-\d{2}$', line) is not None for line in lines), 2)
                    self.assertEqual(sum(line.endswith('.pull=true') for line in lines), 2 if export == 'true' else 0)
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
