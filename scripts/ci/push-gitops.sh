#!/usr/bin/env bash
# Commit manifest changes in a helpin-ai/gitops checkout and push them to main.
# Release workflows in several repositories push to the same branch, so retry
# the rebase-and-push when another bump lands first.
#
# Usage: push-gitops.sh <checkout-dir> <commit-message> <path>...
set -euo pipefail

dir=$1
message=$2
shift 2

cd "$dir"
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

git add -- "$@"
if git diff --cached --quiet; then
  echo "No manifest changes"
  exit 0
fi
git commit -m "$message"

for attempt in 1 2 3 4 5; do
  if git pull --rebase origin main && git push origin HEAD:main; then
    exit 0
  fi
  git rebase --abort 2>/dev/null || true
  echo "Push attempt $attempt failed; retrying"
  sleep $((attempt * 5))
done
echo "Could not push manifest changes to helpin-ai/gitops" >&2
exit 1
