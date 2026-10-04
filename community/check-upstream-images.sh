#!/usr/bin/env bash
# Scan the digest-pinned upstream images (PostgreSQL, Redis, NATS, Garage,
# Temporal, Caddy) for one architecture against the reviewed exceptions, and
# flag exceptions that expire within a week. Run daily so new upstream CVEs
# surface before a release scan stops on them. Needs Trivy and Docker.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
architecture=${COMMUNITY_ARCH:-amd64}
[[ $architecture == amd64 || $architecture == arm64 ]] || { echo "unsupported architecture: $architecture" >&2; exit 2; }
report=$(mktemp)
trap 'rm -f -- "$report"' EXIT
status=0
while IFS= read -r image; do
  name=${image%%@*}; name=${name##*/}; name=${name%%:*}
  trivy image --quiet --platform "linux/$architecture" --scanners vuln --ignore-unfixed \
    --severity HIGH,CRITICAL --format json --output "$report" "$image"
  if python3 verify-image-findings.py "$image" "$report"; then
    echo "PASS: $name ($architecture)"
  else
    echo "FAIL: $name ($architecture) has unreviewed findings"
    status=1
  fi
done < <(grep -hoE '^ +image: [a-z0-9./_-]+:[^@ ]+@sha256:[a-f0-9]{64}' compose.yaml compose.proxy.yaml | awk '{print $2}' | sort -u)
if ! python3 - <<'PY'
import datetime, json, sys
from pathlib import Path
soon = (datetime.date.today() + datetime.timedelta(days=7)).isoformat()
expiring = {image: entry['expires'] for image, entry in json.loads(Path('upstream-image-exceptions.json').read_text()).items()
            if entry.get('expires', '') <= soon}
for image, expires in sorted(expiring.items()):
    print(f'EXPIRING: exception for {image.split("@")[0]} expires {expires}; pin a patched image or review it again')
sys.exit(1 if expiring else 0)
PY
then
  status=1
fi
exit "$status"
