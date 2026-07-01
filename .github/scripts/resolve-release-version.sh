#!/usr/bin/env bash
set -euo pipefail

prefix="${1:?usage: resolve-release-version.sh <prefix> <changed>}"
changed="${2:?usage: resolve-release-version.sh <prefix> <changed>}"

if [ "$changed" != "true" ]; then
  echo "tag="
  echo "prev="
  exit 0
fi

prev_stable="$(git tag -l "${prefix}-v*" --sort=-v:refname | grep -vE '\-(rc|dev)\.' | head -n1 || true)"
release_tag=""

while IFS= read -r rc_tag; do
  [ -z "$rc_tag" ] && continue
  candidate="$(echo "$rc_tag" | sed 's/-rc\.[0-9]*//')"
  if ! git rev-parse "$candidate" >/dev/null 2>&1; then
    release_tag="$candidate"
    break
  fi
done < <(git tag -l "${prefix}-v*-rc.*" --sort=-v:refname)

if [ -z "$release_tag" ]; then
  if [ -n "$prev_stable" ]; then
    version="${prev_stable#${prefix}-v}"
    IFS='.' read -r major minor patch <<< "$version"
    major="${major:-0}"
    minor="${minor:-0}"
    patch="${patch:-0}"
    patch=$((patch + 1))
    release_tag="${prefix}-v${major}.${minor}.${patch}"
  else
    release_tag="${prefix}-v0.1.0"
  fi
fi

if git rev-parse "$release_tag" >/dev/null 2>&1; then
  echo "tag="
  echo "prev=$prev_stable"
  exit 0
fi

echo "tag=$release_tag"
echo "prev=$prev_stable"
