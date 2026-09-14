#!/usr/bin/env bash
# Build and test a frontend source archive without commercial code or prices.
set -euo pipefail
cd "$(dirname "$0")/.."
community_frontend_dir=$(mktemp -d)
trap 'rm -rf "$community_frontend_dir"' EXIT
python3 - "$community_frontend_dir" <<'PY'
from pathlib import Path
import shutil
import subprocess
import sys

root = Path.cwd()
destination = Path(sys.argv[1])
paths = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z', 'frontend', 'server/internal/ordering/testdata']).split(b'\0')
for item in set(paths):
    if not item:
        continue
    path = Path(item.decode())
    if path.parts[:3] == ('frontend', 'src', 'ee') or not path.is_file():
        continue
    target = destination / path
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, target)
# Dependencies and shared product packages are already installed/built by CI.
for name in ['node_modules', 'packages', 'help-center']:
    (destination / name).symlink_to(root / name, target_is_directory=True)
(destination / 'frontend/node_modules').symlink_to(root / 'frontend/node_modules', target_is_directory=True)
PY
cd "$community_frontend_dir/frontend"
test ! -e src/ee
export VITE_EDITION=community
export NODE_OPTIONS=--max-old-space-size=4096
node scripts/build.mjs
node_modules/.bin/vitest run --maxWorkers=4
