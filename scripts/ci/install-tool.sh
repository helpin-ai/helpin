#!/usr/bin/env bash
# Official release archives; checksums reviewed with the version update.
set -euo pipefail
: "${RUNNER_TEMP:?This installer is for CI runners}"
: "${GITHUB_PATH:?This installer is for GitHub Actions}"
tool=${1:?Specify doppler or trivy}
case "$tool:$(uname -sm)" in
  'doppler:Linux x86_64') arch=amd64; checksum=1b2f412d984920d665daf233ab6c15b364df9339b5c5b5224d5e8ee4e0a70154 ;;
  'doppler:Linux aarch64') arch=arm64; checksum=567f051c4c334b79a37ee44c9373671c451dd8a4945ed49288a8f3fd0b73ec89 ;;
  'trivy:Linux x86_64') arch=64bit; checksum=1816b632dfe529869c740c0913e36bd1629cb7688bd5634f4a858c1d57c88b75 ;;
  'trivy:Linux aarch64') arch=ARM64; checksum=7e3924a974e912e57b4a99f65ece7931f8079584dae12eb7845024f97087bdfd ;;
  *) echo 'CI installer supports doppler/trivy on Linux amd64 and arm64 only' >&2; exit 1 ;;
esac
case "$tool" in
  doppler)
    archive="doppler_3.76.5_linux_${arch}.tar.gz"
    url="https://github.com/DopplerHQ/cli/releases/download/3.76.5/$archive" ;;
  trivy)
    archive="trivy_0.69.3_Linux-${arch}.tar.gz"
    url="https://github.com/aquasecurity/trivy/releases/download/v0.69.3/$archive" ;;
esac
download=$(mktemp -d "$RUNNER_TEMP/tool-download.XXXXXX")
trap 'rm -rf -- "$download"' EXIT
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --retry 3 \
  "$url" -o "$download/$archive"
(cd "$download" && printf '%s  %s\n' "$checksum" "$archive" | sha256sum --check --strict)
bin=$(mktemp -d "$RUNNER_TEMP/tool-bin.XXXXXX")
tar -xzf "$download/$archive" -C "$bin" "$tool"
chmod 755 "$bin/$tool"
"$bin/$tool" --version
printf '%s\n' "$bin" >> "$GITHUB_PATH"
