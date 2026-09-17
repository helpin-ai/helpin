#!/usr/bin/env python3
"""CI selection and required-status policy, independent of GitHub credentials."""
import argparse
import json
import os
import sys

GROUPS = ('server', 'frontend', 'admin', 'packages', 'helpcenter', 'desktop', 'mobile', 'eventpipeline')
JOBS = {
    'server': ('server', 'server-tests', 'community-backend', 'ai-profile-postgres', 'schema'),
    'frontend': ('frontend', 'frontend-tests', 'community-frontend'),
    'admin': ('admin',), 'packages': ('packages-tests',), 'helpcenter': ('helpcenter',),
    'desktop': ('desktop',), 'mobile': ('mobile-check',), 'eventpipeline': ('eventpipeline',),
}
ROOT_CONFIG = {'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml', 'turbo.json', 'tsconfig.base.json', '.node-version', '.go-version', '.dockerignore', '.npmrc', '.pnpmfile.cjs', '.github/actionlint.yaml'}


def select(paths, all_checks=False):
    result = dict.fromkeys(GROUPS, all_checks)
    for path in paths:
        if path in ROOT_CONFIG or path.startswith(('.github/workflows/', '.github/actions/', 'scripts/ci/')):
            return dict.fromkeys(GROUPS, True)
        if path.endswith('.md') and path.startswith(('docs/', 'community/')):
            continue
        server = path.startswith(('server/', 'scripts/check-task-canonical.sh', 'scripts/check-community-backend.sh'))
        frontend = path.startswith(('frontend/', 'packages/shared/', 'packages/widget-core/', 'scripts/check-community-frontend.sh'))
        matches = {
            'server': server,
            'frontend': frontend,
            'admin': path.startswith(('apps/admin/', 'frontend/src/index.css')),
            'packages': path.startswith(('packages/', 'ops/widget-smoke/', 'scripts/monitoring/')),
            'helpcenter': path.startswith(('help-center/', 'packages/shared/', 'packages/widget-core/')),
            'desktop': path.startswith(('apps/support-desktop/', 'frontend/', 'packages/')),
            'mobile': path.startswith(('apps/support-mobile/', 'frontend/', 'packages/support-core/', 'packages/shared/', 'packages/widget-core/')),
            'eventpipeline': path.startswith('events-pipeline/'),
        }
        for group, changed in matches.items():
            result[group] |= changed
    return result


def failures(needs):
    problems = []
    for job in ('changes', 'workflow-checks'):
        if needs.get(job, {}).get('result') != 'success':
            problems.append(job)
    outputs = needs.get('changes', {}).get('outputs', {})
    for group, jobs in JOBS.items():
        if outputs.get(group) not in ('true', 'false'):
            problems.append('missing selection: ' + group)
        if outputs.get(group) == 'true':
            for job in jobs:
                if needs.get(job, {}).get('result') != 'success':
                    problems.append(job)
    # Even an unexpectedly executed job cannot fail unnoticed.
    for job, state in needs.items():
        if state.get('result') in ('failure', 'cancelled') and job not in problems:
            problems.append(job)
    return problems


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('changes', 'required'))
    parser.add_argument('--all', action='store_true')
    args = parser.parse_args()
    if args.command == 'changes':
        selected = select(sys.stdin.read().splitlines(), args.all)
        output = '\n'.join(f'{k}={str(v).lower()}' for k, v in selected.items()) + '\n'
        if os.environ.get('GITHUB_OUTPUT'):
            with open(os.environ['GITHUB_OUTPUT'], 'a') as stream:
                stream.write(output)
        print(output, end='')
    else:
        failed = failures(json.loads(os.environ['CI_NEEDS']))
        if failed:
            raise SystemExit('Required CI did not pass: ' + ', '.join(failed))
        print('All selected required checks passed.')
