#!/usr/bin/env bash
# Verify the distributable source, not just linker reachability. Use a temporary
# copy so this check cannot remove a developer's EE files or disturb other tests.
set -euo pipefail
cd "$(dirname "$0")/.."
python3 - <<'PYTAG'
from pathlib import Path
untagged = [str(p) for p in Path('server/ee').rglob('*.go') if not p.read_text().startswith('//go:build ee\n')]
if untagged:
    raise SystemExit('EE sources missing build tags: ' + ', '.join(untagged))
PYTAG
# Even with EE sources present, importing an EE package must fail in Community.
if (cd server && go list ./ee/service >/dev/null 2>&1); then
  echo "EE package is importable without -tags ee" >&2
  exit 1
fi
community_check_dir=$(mktemp -d)
trap 'rm -rf "$community_check_dir"' EXIT
python3 - "$community_check_dir" <<'PY'
from pathlib import Path
import shutil
import subprocess
import sys

destination = Path(sys.argv[1])
paths = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z', 'server', 'packages/shared/test-data', 'frontend/src/generated/aiModels.ts', 'docs/api/openapi.json']).split(b'\0')
for item in set(paths):
    if not item:
        continue
    path = Path(item.decode())
    if path.parts[:2] == ('server', 'ee') or not path.is_file():
        continue
    target = destination / path
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, target)
PY
cd "$community_check_dir/server"
test ! -e ee
# Keep the release build CGO-free, but enable CGO for SQLite-backed tests.
CGO_ENABLED=0 go build -buildvcs=false ./cmd/api ./cmd/temporal-worker ./cmd/migrate
CGO_ENABLED=1 go test -buildvcs=false ./internal/... ./cmd/...
