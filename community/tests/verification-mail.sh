#!/usr/bin/env bash
# Required verification must fail startup when no application sender exists.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
umask 077
report=$(mktemp)
trap 'rm -f -- "$report"' EXIT
if timeout 60s docker compose run --rm --no-deps -e AUTH_EMAIL_VERIFICATION_REQUIRED=true \
  -e SMTP_HOST= -e POSTMARK_APP_SERVER_TOKEN= helpin-api >"$report" 2>&1; then
  echo 'FAIL: verification-required API started without a mail sender' >&2
  exit 1
fi
if ! grep -q 'AUTH_EMAIL_VERIFICATION_REQUIRED requires an SMTP or Postmark application email sender' "$report"; then
  echo 'FAIL: startup failed before reaching the required-mail validation; inspect locally without exposing secrets' >&2
  exit 1
fi
echo 'PASS: required verification without application email fails startup clearly'
