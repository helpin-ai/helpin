#!/usr/bin/env python3
"""Run pinned analyzers and reject findings absent from the reviewed baseline."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
BASELINE = Path(__file__).with_name('baseline.json')
KINDS = ('files', 'exports', 'types', 'dependencies', 'devDependencies', 'unlisted', 'unresolved')


def knip_findings(report):
    return {f"{kind}:{issue['file']}:{item['name']}"
            for issue in report['issues'] for kind in KINDS
            for item in issue.get(kind, [])}


def go_findings(report):
    return {f"function:{fn['Position']['File']}:{fn['Name']}"
            for package in (report or []) for fn in package['Funcs']
            if not fn.get('Generated') and not fn.get('Marker')}


def run_json(command, cwd, env):
    result = subprocess.run(command, cwd=cwd, env=env, text=True, capture_output=True)
    if result.returncode or 'ERROR:' in result.stderr:
        raise RuntimeError(f"Analyzer failed: {' '.join(command)}\n{result.stderr}\n{result.stdout[:2000]}")
    if result.stderr:
        print(result.stderr, file=sys.stderr)
    return json.loads(result.stdout)


def compare(current, baseline):
    return {name: sorted(set(items) - set(baseline.get(name, [])))
            for name, items in current.items() if set(items) - set(baseline.get(name, []))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--update-baseline', action='store_true', help='Explicitly accept current findings; review the diff')
    parser.add_argument('--report-dir', type=Path, help='Save raw reports for investigation')
    args = parser.parse_args()
    if args.report_dir:
        args.report_dir.mkdir(parents=True, exist_ok=True)
    current = {}
    for edition in ('community', 'ee'):
        env = {**os.environ, 'VITE_EDITION': edition}
        commands = {
            'typescript': ([os.environ.get('KNIP', str(ROOT / 'node_modules/.bin/knip')), '--config', 'knip.config.ts', '--reporter', 'json', '--no-exit-code'], ROOT, knip_findings),
            'go': ([os.environ.get('DEADCODE', 'deadcode'), '-json', *(['-tags', 'ee'] if edition == 'ee' else []), './cmd/...'], ROOT / 'server', go_findings),
        }
        for language, (command, cwd, normalize) in commands.items():
            key = f'{language}-{edition}'
            print(f'Analyzing {key}...', flush=True)
            report = run_json(command, cwd, env)
            current[key] = sorted(normalize(report))
            if args.report_dir:
                (args.report_dir / f'{key}.json').write_text(json.dumps(report, indent=2) + '\n')
    if args.update_baseline:
        BASELINE.write_text(json.dumps(current, indent=2) + '\n')
        print('Baseline updated; review newly accepted findings before committing.')
        return 0
    additions = compare(current, json.loads(BASELINE.read_text()))
    for key, findings in additions.items():
        print(f'New unused-code findings ({key}):\n' + '\n'.join(findings))
    if additions:
        return 1
    print('No new unused-code findings in either edition.')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, RuntimeError) as error:
        print(error, file=sys.stderr)
        sys.exit(2)
