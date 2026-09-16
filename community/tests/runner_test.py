import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('acceptance', Path(__file__).with_name('run.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class RunnerTest(unittest.TestCase):
    def test_operator_configuration_is_not_inherited(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            community, env = runner.prepare(runner.SOURCE, root, {
                'PATH': '/bin', 'OPENAI_API_KEY': 'must-not-inherit',
                'COMPOSE_FILE': '/operator/compose.yaml', 'COMPOSE_PROJECT_NAME': 'production',
                'COMMUNITY_URL': 'https://production', 'APP_BASE_URL': 'https://production',
            })
            self.assertFalse((community / '.env').exists())
            self.assertFalse((community / 'apps.json').exists())
            self.assertNotIn('OPENAI_API_KEY', env)
            self.assertEqual(env['PATH'], '/bin')
            self.assertEqual(env['COMPOSE_FILE'], str(community / 'compose.yaml'))
            self.assertTrue(env['COMPOSE_PROJECT_NAME'].startswith('community-test-'))
            self.assertEqual((community / '.acceptance-project').read_text(), env['COMPOSE_PROJECT_NAME'])
            self.assertTrue(env['COMMUNITY_URL'].startswith('http://localhost:'))
            self.assertEqual(env['APP_BASE_URL'], env['COMMUNITY_URL'])
            self.assertEqual(len({env[k] for k in ('DASHBOARD_PORT', 'HELPCENTER_PORT', 'STORAGE_PORT', 'COMMUNITY_PROVIDER_PORT')}), 4)

    def test_failure_and_interruption_clean_only_owned_projects(self):
        for failure in (subprocess.CalledProcessError(1, 'fixture'), SystemExit(143)):
            with self.subTest(failure=failure):
                calls = []
                def invoke(args, cwd, env, check):
                    calls.append((args, cwd, env.copy()))
                    if args == ['./setup.sh', 'install']:
                        (cwd / '.env').write_text('fixture\n')
                    if args == ['node', 'tests/http-smoke.mjs']:
                        raise failure
                    return subprocess.CompletedProcess(args, 0)
                with patch.object(runner.subprocess, 'run', side_effect=invoke):
                    with self.assertRaises(type(failure)):
                        runner.run('smoke')
                cleanups = [call for call in calls if 'down' in call[0]]
                self.assertEqual(len(cleanups), 2)
                for args, cwd, env in cleanups:
                    project = args[args.index('-p') + 1]
                    self.assertIn(project, (env['COMPOSE_PROJECT_NAME'], env['COMPOSE_PROJECT_NAME'] + '-restore'))
                    self.assertFalse(cwd.exists())

    def test_failed_install_does_not_call_compose(self):
        with patch.object(runner.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'install')) as invoke:
            with self.assertRaises(subprocess.CalledProcessError):
                runner.run('smoke')
            self.assertEqual(invoke.call_count, 1)


if __name__ == '__main__':
    unittest.main()
