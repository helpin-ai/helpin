#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

setup_repo() {
  local repo
  repo="$(mktemp -d)"
  git -C "$repo" init -q
  git -C "$repo" config user.email "test@example.com"
  git -C "$repo" config user.name "Test"
  printf 'init\n' > "$repo/README.md"
  git -C "$repo" add README.md
  git -C "$repo" commit -q -m init
  printf '%s\n' "$repo"
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL: %s\nexpected: %s\nactual:   %s\n' "$message" "$expected" "$actual" >&2
    exit 1
  fi
}

test_stale_rc_does_not_block_next_patch_release() {
  local repo output tag prev
  repo="$(setup_repo)"
  git -C "$repo" tag helpcenter-v0.95.86
  git -C "$repo" tag helpcenter-v0.95.86-rc.1

  output="$(cd "$repo" && "$SCRIPT_DIR/resolve-release-version.sh" helpcenter true)"
  tag="$(printf '%s\n' "$output" | awk -F= '$1 == "tag" { print $2 }')"
  prev="$(printf '%s\n' "$output" | awk -F= '$1 == "prev" { print $2 }')"

  assert_eq "$tag" "helpcenter-v0.95.87" "stale RC should fall back to next stable patch"
  assert_eq "$prev" "helpcenter-v0.95.86" "previous stable should remain latest stable"
}

test_promotes_unreleased_rc() {
  local repo output tag prev
  repo="$(setup_repo)"
  git -C "$repo" tag helpcenter-v0.95.86
  git -C "$repo" tag helpcenter-v0.95.87-rc.2

  output="$(cd "$repo" && "$SCRIPT_DIR/resolve-release-version.sh" helpcenter true)"
  tag="$(printf '%s\n' "$output" | awk -F= '$1 == "tag" { print $2 }')"
  prev="$(printf '%s\n' "$output" | awk -F= '$1 == "prev" { print $2 }')"

  assert_eq "$tag" "helpcenter-v0.95.87" "unreleased RC should promote to stable"
  assert_eq "$prev" "helpcenter-v0.95.86" "previous stable should be reported"
}

test_unchanged_service_returns_empty_outputs() {
  local repo output tag prev
  repo="$(setup_repo)"
  git -C "$repo" tag helpcenter-v0.95.86

  output="$(cd "$repo" && "$SCRIPT_DIR/resolve-release-version.sh" helpcenter false)"
  tag="$(printf '%s\n' "$output" | awk -F= '$1 == "tag" { print $2 }')"
  prev="$(printf '%s\n' "$output" | awk -F= '$1 == "prev" { print $2 }')"

  assert_eq "$tag" "" "unchanged service should not produce a release tag"
  assert_eq "$prev" "" "unchanged service should not produce a previous tag"
}

test_stale_rc_does_not_block_next_patch_release
test_promotes_unreleased_rc
test_unchanged_service_returns_empty_outputs

printf 'release-version tests passed\n'
