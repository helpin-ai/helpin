#!/usr/bin/env bash
set -euo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/build-or-promote-image.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

setup_repo() {
  local repo
  repo="$(mktemp -d)"
  git -C "$repo" init -q
  git -C "$repo" config user.email "test@example.com"
  git -C "$repo" config user.name "Test"
  mkdir -p "$repo/server" "$repo/help-center" "$repo/frontend" "$repo/packages/shared" "$repo/docs"
  echo 'package main' > "$repo/server/main.go"
  echo 'FROM scratch' > "$repo/server/Dockerfile"
  echo 'FROM scratch' > "$repo/help-center/Dockerfile"
  echo '{}' > "$repo/frontend/package.json"
  echo '{}' > "$repo/packages/shared/package.json"
  echo 'lock' > "$repo/pnpm-lock.yaml"
  echo 'notes' > "$repo/docs/notes.md"
  git -C "$repo" add -A
  git -C "$repo" commit -q -m init
  printf '%s\n' "$repo"
}

commit_change() {
  local repo=$1 path=$2
  echo "change $RANDOM" >> "$repo/$path"
  git -C "$repo" add -A
  git -C "$repo" commit -q -m "change $path"
}

test_key_tracks_only_build_inputs() {
  local repo server helpcenter
  repo="$(setup_repo)"
  server=$(bash "$SCRIPT" --key "$repo" server)
  helpcenter=$(bash "$SCRIPT" --key "$repo" helpcenter)
  [ "${#server}" -eq 32 ] || fail "server key should be 32 hex characters, got $server"
  [ "$server" != "$helpcenter" ] || fail "components must not share keys"

  commit_change "$repo" docs/notes.md
  [ "$(bash "$SCRIPT" --key "$repo" server)" = "$server" ] || fail "unrelated change altered the server key"
  [ "$(bash "$SCRIPT" --key "$repo" helpcenter)" = "$helpcenter" ] || fail "unrelated change altered the helpcenter key"

  commit_change "$repo" frontend/package.json
  [ "$(bash "$SCRIPT" --key "$repo" server)" = "$server" ] || fail "frontend change altered the server key"
  [ "$(bash "$SCRIPT" --key "$repo" helpcenter)" != "$helpcenter" ] || fail "frontend change must alter the helpcenter key (portal)"

  commit_change "$repo" server/main.go
  [ "$(bash "$SCRIPT" --key "$repo" server)" != "$server" ] || fail "server change must alter the server key"
}

test_merge_commit_with_same_tree_keeps_key() {
  local repo before
  repo="$(setup_repo)"
  before=$(bash "$SCRIPT" --key "$repo" server)
  # A release merge creates a new commit whose tree matches develop.
  git -C "$repo" commit -q --allow-empty -m "Merge develop into main"
  [ "$(bash "$SCRIPT" --key "$repo" server)" = "$before" ] || fail "new commit with the same tree must keep the key"
}

test_dry_run_builds_when_content_is_new() {
  local repo output
  repo="$(setup_repo)"
  output=$(DRY_RUN=1 IMAGE_PREFIX=ghcr.io/acme/app GITHUB_SHA=abc bash "$SCRIPT" "$repo" prod server server-v1.2.3)
  [ "$(grep -c 'docker buildx build' <<<"$output")" -eq 3 ] || fail "expected three server builds: $output"
  grep -q -- '--target temporal-worker' <<<"$output" || fail "temporal-worker target missing"
  grep -q -- '--build-arg GO_BUILD_TAGS=ee' <<<"$output" || fail "ee build tag missing"
  grep -q 'server-buildcache:prod-migrate' <<<"$output" || fail "environment build cache missing"
  grep -q 'server:content-' <<<"$output" || fail "content tag missing"
  if grep -q 'imagetools create' <<<"$output"; then fail "must not promote unseen content"; fi
}

test_dry_run_promotes_existing_content() {
  local repo output
  repo="$(setup_repo)"
  output=$(DRY_RUN=1 DRY_RUN_EXISTING=1 IMAGE_PREFIX=ghcr.io/acme/app GITHUB_SHA=abc bash "$SCRIPT" "$repo" prod helpcenter helpcenter-v2.0.0)
  grep -q 'imagetools create -t ghcr.io/acme/app/helpcenter:helpcenter-v2.0.0 -t ghcr.io/acme/app/helpcenter:sha-abc -t ghcr.io/acme/app/helpcenter:prod-latest ghcr.io/acme/app/helpcenter:content-' <<<"$output" \
    || fail "expected helpcenter promotion: $output"
  if grep -q 'docker buildx build' <<<"$output"; then fail "must not rebuild existing content"; fi
}

test_rejects_unknown_environment() {
  local repo
  repo="$(setup_repo)"
  if DRY_RUN=1 IMAGE_PREFIX=x GITHUB_SHA=y bash "$SCRIPT" "$repo" qa server t >/dev/null 2>&1; then
    fail "unknown environment must fail"
  fi
}

test_key_tracks_only_build_inputs
test_merge_commit_with_same_tree_keeps_key
test_dry_run_builds_when_content_is_new
test_dry_run_promotes_existing_content
test_rejects_unknown_environment
echo "build-or-promote-image tests passed"
