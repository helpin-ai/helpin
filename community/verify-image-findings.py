#!/usr/bin/env python3
"""Only exact pinned upstream findings may have time-limited beta exceptions."""
import datetime
import json
from pathlib import Path
import sys


def unreviewed(image, report, exceptions, today):
    entry = exceptions.get(image, {}) if '@sha256:' in image else {}
    allowed = set()
    if entry.get('expires', '') >= today.isoformat():
        allowed = {(v['id'], v['package'], v['installed']) for v in entry.get('findings', [])}
    findings = {(v['VulnerabilityID'], v['PkgName'], v['InstalledVersion'])
                for result in report.get('Results', []) for v in result.get('Vulnerabilities', [])
                if v.get('Severity') in ('HIGH', 'CRITICAL') and v.get('FixedVersion')}
    return sorted(findings - allowed), len(findings & allowed)


if __name__ == '__main__':
    image, filename = sys.argv[1:]
    exceptions = json.loads(Path(__file__).with_name('upstream-image-exceptions.json').read_text())
    remaining, accepted = unreviewed(image, json.loads(Path(filename).read_text()), exceptions, datetime.date.today())
    for finding in remaining:
        print('Unreviewed vulnerability:', *finding, file=sys.stderr)
    if accepted:
        print(f'{accepted} pinned upstream findings covered by an unexpired beta exception; see upstream-images.md.')
    sys.exit(1 if remaining else 0)
