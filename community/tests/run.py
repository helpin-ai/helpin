#!/usr/bin/env python3
"""Acceptance owns its configuration, ports and disposable Compose projects."""
import argparse
import os
from pathlib import Path
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import uuid

SOURCE = Path(__file__).resolve().parents[1]


def execute(args, cwd, env, check=True):
    process = subprocess.Popen(args, cwd=cwd, env=env, start_new_session=True)
    try:
        code = process.wait()
    except BaseException:
        # Cancellation must stop the active Compose/browser command before
        # cleanup, rather than allowing it to recreate resources afterwards.
        try:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
        except ProcessLookupError:
            pass
        raise
    if check and code:
        raise subprocess.CalledProcessError(code, args)
    return subprocess.CompletedProcess(args, code)


def clean_environment(source, inherited):
    # Never inherit operator credentials, Compose selection, or test endpoints.
    keys = set(re.findall(r'^([A-Z_]+)=', (source / '.env.example').read_text(), re.M))
    keys.update(re.findall(r'\$\{([A-Z_]+)', (source / 'compose.yaml').read_text()))
    return {k: v for k, v in inherited.items()
            if k not in keys and not k.startswith(('COMPOSE_', 'COMMUNITY_'))}


def prepare(source, root, inherited):
    community = root / 'community'
    community.mkdir()
    for name in ('setup.sh', '.env.example', 'apps.example.json', 'compose.yaml',
                 'garage.toml', 'temporal.yaml', 'temporal-schema.sh', 'temporal-namespace.sh'):
        shutil.copy2(source / name, community / name)
    for name in ('postgres', 'tests'):
        shutil.copytree(source / name, community / name, ignore=shutil.ignore_patterns('__pycache__'))
    project = 'community-test-' + uuid.uuid4().hex[:12]
    (community / '.acceptance-project').write_text(project)
    env = clean_environment(source, inherited)
    env.update(COMPOSE_PROJECT_NAME=project, COMPOSE_FILE=str(community / 'compose.yaml'),
               COMMUNITY_TEST_ROOT=str(community), COMMUNITY_SOURCE_ROOT=str(source.parent),
               COMMUNITY_TEST_PROOF_FILE=str(root / 'restored-profile.json'))
    # Reserve together so every selected port differs. A later collision fails
    # startup and cleans up our project; it can never redirect us to another one.
    sockets = [socket.socket() for _ in range(6)]
    try:
        for sock in sockets:
            sock.bind(('127.0.0.1', 0))
        dashboard, helpcenter, storage, provider, visitor, rejected = [s.getsockname()[1] for s in sockets]
    finally:
        for sock in sockets:
            sock.close()
    base = f'http://localhost:{dashboard}'
    env.update(DASHBOARD_PORT=str(dashboard), HELPCENTER_PORT=str(helpcenter), STORAGE_PORT=str(storage),
               COMMUNITY_PROVIDER_PORT=str(provider), APP_BASE_URL=base, PUBLIC_WIDGET_URL=base,
               PUBLIC_SDK_URL=base + '/sdk/lib.js', PUBLIC_STORAGE_URL=f'http://localhost:{storage}',
               COMMUNITY_URL=base, COMMUNITY_HC_URL=f'http://localhost:{helpcenter}',
               COMMUNITY_PROVIDER_URL=f'http://localhost:{provider}',
               COMMUNITY_VISITOR_ORIGIN=f'http://localhost:{visitor}',
               COMMUNITY_REJECTED_ORIGIN=f'http://localhost:{rejected}', BIND_ADDRESS='127.0.0.1')
    return community, env


def run(mode):
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix='helpin-acceptance-') as temporary:
        community, env = prepare(SOURCE, Path(temporary), os.environ)
        compose = ['docker', 'compose']
        fixture = compose + ['-f', 'compose.yaml', '-f', 'tests/compose.fixture.yaml']

        def command(args, check=True):
            return execute(args, cwd=community, env=env, check=check)

        try:
            command(['./setup.sh', 'install'])
            env['AGENT_RUNTIME_EXECUTION_APP_CONFIG'] = (community / 'apps.json').read_text()
            command(compose + ['config', '--quiet'])
            command(compose + ['up', '-d', '--wait', '--wait-timeout', '300'])
            command(['node', 'tests/http-smoke.mjs'])
            if mode == 'full':
                command(['bash', 'tests/verification-mail.sh'])
                command(['pnpm', '--dir', str(SOURCE.parent / 'packages/sdk-js'), 'exec', 'playwright',
                         'test', '--config=playwright.community.config.ts'])
                env['AGENT_RUNTIME_EXECUTION_APP_CONFIG'] = (community / 'tests/apps.fixture.json').read_text()
                command(fixture + ['up', '-d', '--wait', '--wait-timeout', '300'])
                command(['node', 'tests/ai-mail-smoke.mjs'])
                command(compose + ['up', '-d', '--wait', '--wait-timeout', '300', '--remove-orphans'])
                command(['bash', 'tests/backup-restore.sh'])
        finally:
            # Health/state only: raw application logs and inspect/config output
            # can contain credentials. Never upload the temporary configuration.
            if (community / '.env').exists():
                cleanup_failed = False
                for project in (env['COMPOSE_PROJECT_NAME'] + '-restore', env['COMPOSE_PROJECT_NAME']):
                    command(fixture + ['-p', project, 'ps', '--all', '--format', '{{.Service}} {{.State}} {{.Health}} {{.ExitCode}}'], check=False)
                    result = command(fixture + ['-p', project, 'down', '-v', '--remove-orphans'], check=False)
                    cleanup_failed |= result.returncode != 0
                if cleanup_failed:
                    print('Acceptance resource cleanup failed; inspect the named test projects.', file=sys.stderr)
                    if sys.exc_info()[0] is None:
                        raise RuntimeError('Acceptance cleanup failed')
            print(f'Acceptance {mode} elapsed: {time.monotonic() - started:.1f}s', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=('smoke', 'full'), nargs='?', default='smoke')
    args = parser.parse_args()
    os.umask(0o077)
    def interrupted(signum, _frame):
        raise SystemExit(128 + signum)
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    run(args.mode)
