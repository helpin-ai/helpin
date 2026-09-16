#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
proof_dir=$(mktemp -d)
trap 'rm -rf -- "$proof_dir"' EXIT
export COMMUNITY_TEST_PROOF_FILE="$proof_dir/restored-profile.json"
node --test tests/installer.test.mjs
python3 tests/package_release_test.py
./setup.sh install
docker compose config --quiet
docker compose up -d --wait --wait-timeout 300 --remove-orphans
tests/migrations.sh
node tests/http-smoke.mjs
pnpm --dir ../packages/sdk-js exec playwright test --config=playwright.community.config.ts
docker compose -f compose.yaml -f tests/compose.fixture.yaml up -d --wait --wait-timeout 300
node tests/ai-mail-smoke.mjs
# Return to the released configuration before taking the same-version backup.
docker compose up -d --wait --wait-timeout 300 --remove-orphans
COMMUNITY_TEST_DISPOSABLE=yes tests/backup-restore.sh
