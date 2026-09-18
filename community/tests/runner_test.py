import importlib.util
import os
import time
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
                        (cwd / 'apps.json').write_text((cwd / 'apps.example.json').read_text())
                    if args == ['node', 'tests/http-smoke.mjs']:
                        raise failure
                    return subprocess.CompletedProcess(args, 0)
                with patch.object(runner, 'execute', side_effect=invoke):
                    with self.assertRaises(type(failure)):
                        runner.run('smoke')
                cleanups = [call for call in calls if 'down' in call[0]]
                self.assertEqual(len(cleanups), 2)
                for args, cwd, env in cleanups:
                    project = args[args.index('-p') + 1]
                    self.assertIn(project, (env['COMPOSE_PROJECT_NAME'], env['COMPOSE_PROJECT_NAME'] + '-restore'))
                    self.assertFalse(cwd.exists())

    def test_cleanup_failure_cannot_report_success(self):
        def invoke(args, cwd, env, check):
            if args == ['./setup.sh', 'install']:
                (cwd / '.env').write_text('fixture\n')
                (cwd / 'apps.json').write_text((cwd / 'apps.example.json').read_text())
            return subprocess.CompletedProcess(args, 1 if 'down' in args else 0)
        with patch.object(runner, 'execute', side_effect=invoke):
            with self.assertRaisesRegex(RuntimeError, 'cleanup'):
                runner.run('smoke')

    def test_interruption_terminates_child_before_cleanup(self):
        with patch.object(runner.subprocess, 'Popen') as popen, patch.object(runner.os, 'killpg') as kill:
            process = popen.return_value
            process.pid = 123
            process.wait.side_effect = [KeyboardInterrupt(), 0]
            with self.assertRaises(KeyboardInterrupt):
                runner.execute(['fixture'], runner.SOURCE, {})
            kill.assert_called_once_with(123, runner.signal.SIGTERM)

    def test_cli_cancellation_stops_child_and_cleans_owned_projects(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            docker = root / 'docker'
            calls = root / 'calls'
            ready = root / 'ready'
            docker.write_text("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CI_TEST_CALLS\"\ncase \" $* \" in *\" up \"*) touch \"$CI_TEST_READY\"; sleep 60;; esac\n")
            docker.chmod(0o755)
            env = {**os.environ, 'PATH': str(root) + os.pathsep + os.environ['PATH'],
                   'CI_TEST_CALLS': str(calls), 'CI_TEST_READY': str(ready)}
            with (root / 'output').open('w') as output:
                process = subprocess.Popen(['bash', str(runner.SOURCE / 'tests/run.sh'), 'smoke'], env=env, stdout=output, stderr=output)
                try:
                    deadline = time.monotonic() + 10
                    while not ready.exists() and time.monotonic() < deadline and process.poll() is None:
                        time.sleep(0.05)
                    self.assertTrue(ready.exists(), 'fixture did not reach startup')
                    process.terminate()
                    self.assertEqual(process.wait(timeout=15), 143)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
            cleanups = [line for line in calls.read_text().splitlines() if ' down ' in line]
            self.assertEqual(len(cleanups), 2)
            self.assertTrue(all('-p community-test-' in line for line in cleanups))

    def test_failed_install_does_not_call_compose(self):
        with patch.object(runner, 'execute', side_effect=subprocess.CalledProcessError(1, 'install')) as invoke:
            with self.assertRaises(subprocess.CalledProcessError):
                runner.run('smoke')
            self.assertEqual(invoke.call_count, 1)


if __name__ == '__main__':
    unittest.main()
