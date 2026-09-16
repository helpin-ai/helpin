#!/usr/bin/env bash
# Read-only image audit; never publishes or prints a potential secret match.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
architecture=${COMMUNITY_ARCH:-$(docker info --format '{{.Architecture}}')}
case "$architecture" in x86_64) architecture=amd64;; aarch64) architecture=arm64;; esac
[[ $architecture == amd64 || $architecture == arm64 ]] || exit 1
mkdir -p release-artifacts
private_report=$(mktemp)
trap 'rm -f -- "$private_report"' EXIT
while IFS= read -r image; do
  name=${image%%@*}; name=${name##*/}; name=${name%%:*}
  trivy image --scanners vuln --format cyclonedx --output "release-artifacts/$name-$architecture.cdx.json" "$image"
  trivy image --scanners vuln --ignore-unfixed --severity HIGH,CRITICAL --format json --output "$private_report" "$image"
  python3 verify-image-findings.py "$image" "$private_report"
  if ! trivy image --scanners secret --format json --output "$private_report" --exit-code 1 "$image"; then
    echo "Secret scan requires private review for $name; no findings were uploaded." >&2
    exit 1
  fi
  echo "PASS: $name vulnerability and secret scans"
done < <(docker compose config --images | sort -u)
