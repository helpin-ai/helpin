#!/usr/bin/env python3
"""Enforce filename conventions for Helpin Markdown under docs/."""
from datetime import date
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]


def check_names(paths):
    errors = []
    for name in sorted(set(paths)):
        path = PurePosixPath(name)
        if not path.parts or path.parts[0] != 'docs' or path.suffix.lower() != '.md':
            continue
        if path.name == 'README.md':
            continue
        if not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*\.md', path.name):
            errors.append(f'{name}: use lowercase words separated by hyphens')
            continue
        if path.stem.startswith('todo-') or ('prds' in path.parts and path.stem.startswith('prd-')):
            errors.append(f'{name}: remove status or redundant document-type prefix')
        dated = re.match(r'^(\d{4}-\d{2}-\d{2})-', path.name)
        if dated:
            try:
                date.fromisoformat(dated[1])
            except ValueError:
                errors.append(f'{name}: invalid YYYY-MM-DD date prefix')
    return errors


def main():
    if len(sys.argv) > 1:
        paths = sys.argv[1:]
    else:
        tracked = subprocess.check_output(
            ['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'docs'],
            cwd=ROOT,
        ).decode().split('\0')
        # Git still lists the old tracked path during an unstaged rename.
        paths = [name for name in tracked if name and (ROOT / name).is_file()]
    errors = check_names(paths)
    for error in errors:
        print(error, file=sys.stderr)
    print(f'Documentation naming: {len(errors)} errors.')
    return bool(errors)


if __name__ == '__main__':
    sys.exit(main())
