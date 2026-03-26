#!/usr/bin/env bash
set -euo pipefail

PACKAGE_NAME="helpin-temporal-worker"
REPO="helpin-ai/helpin"

log() {
  echo "[helpin-temporal-worker-updater] $*"
}

if ! command -v gh >/dev/null 2>&1; then
  log "gh is not installed; skipping update check"
  exit 0
fi

if ! command -v apt >/dev/null 2>&1; then
  log "apt is not installed; skipping update check"
  exit 0
fi

if ! command -v dpkg-query >/dev/null 2>&1; then
  log "dpkg-query is not installed; skipping update check"
  exit 0
fi

if [[ -z "${GH_TOKEN:-}" ]]; then
  log "GH_TOKEN is not set; skipping update check"
  exit 0
fi

installed_version="$(dpkg-query -W -f='${Version}' "${PACKAGE_NAME}" 2>/dev/null || true)"
if [[ -z "${installed_version}" ]]; then
  log "package ${PACKAGE_NAME} is not installed; skipping update check"
  exit 0
fi

latest_tag="$(gh release view --repo "${REPO}" --json tagName --jq '.tagName' 2>/dev/null || true)"
if [[ -z "${latest_tag}" ]]; then
  log "unable to determine latest release tag; skipping update check"
  exit 0
fi

latest_version="${latest_tag#v}"
if ! dpkg --compare-versions "${latest_version}" gt "${installed_version}"; then
  log "installed version ${installed_version} is current"
  exit 0
fi

tmpdir="$(mktemp -d /tmp/helpin-temporal-worker-update.XXXXXX)"
cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

asset_name="${PACKAGE_NAME}_${latest_version}_amd64.deb"
log "downloading ${asset_name} from ${latest_tag}"
gh release download "${latest_tag}" --repo "${REPO}" --pattern "${asset_name}" --dir "${tmpdir}"

deb_path="${tmpdir}/${asset_name}"
if [[ ! -f "${deb_path}" ]]; then
  log "release asset ${asset_name} was not found"
  exit 1
fi

log "installing ${asset_name}"
DEBIAN_FRONTEND=noninteractive apt install -y "${deb_path}"
log "updated ${PACKAGE_NAME} from ${installed_version} to ${latest_version}"
