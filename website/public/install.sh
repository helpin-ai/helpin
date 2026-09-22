#!/usr/bin/env bash
# Download the Helpin operator CLI. This script does not start Docker services.
set -euo pipefail

install_helpin() {
  local release=${HELPIN_VERSION:-} destination=${HELPIN_BIN_DIR:-"$HOME/.local/bin"}
  local os arch asset checksum actual api
  command -v curl >/dev/null || { echo 'Helpin: curl is required.' >&2; return 1; }
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) echo 'Helpin: use Linux or macOS with Docker and Compose v2.' >&2; return 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) echo 'Helpin: supported architectures are amd64 and arm64.' >&2; return 1 ;;
  esac
  if command -v sha256sum >/dev/null; then checksum=sha256sum
  elif command -v shasum >/dev/null; then checksum=shasum
  else echo 'Helpin: sha256sum or shasum is required.' >&2; return 1
  fi
  temporary=$(mktemp -d)
  trap 'rm -rf -- "$temporary"' EXIT
  if [[ -z "$release" ]]; then
    api='https://api.github.com/repos/helpin-ai/helpin/releases?per_page=100'
    if ! curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 60 "$api" > "$temporary/releases.json"; then
      echo 'Helpin: public Community releases are unavailable. Check network access and https://github.com/helpin-ai/helpin/releases. No CLI was installed.' >&2
      return 1
    fi
    # The unauthenticated API returns only published releases, newest first.
    # Community is a prerelease channel; GitHub /releases/latest excludes it.
    release=$(sed -nE 's/^[[:space:]]*"tag_name":[[:space:]]*"(community-v0\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?)",?[[:space:]]*$/\1/p' "$temporary/releases.json" | sed -n '1p')
  fi
  if [[ ! "$release" =~ ^community-v0\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$ ]]; then
    echo 'Helpin: no installable Community release found. Set HELPIN_VERSION to a published Community tag, or use the source-build guide.' >&2
    return 1
  fi
  asset="helpin-$os-$arch"
  echo "Downloading Helpin $release ($os/$arch)…"
  local base="https://github.com/helpin-ai/helpin/releases/download/$release"
  curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 300 "$base/$asset" > "$temporary/$asset"
  curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 60 "$base/$asset.sha256" > "$temporary/checksum"
  local expected filename extra
  read -r expected filename extra < "$temporary/checksum"
  if [[ ! "$expected" =~ ^[a-f0-9]{64}$ || "$filename" != "$asset" || -n "$extra" || $(wc -l < "$temporary/checksum") -ne 1 ]]; then
    echo 'Helpin: invalid release checksum file; nothing installed.' >&2; return 1
  fi
  if [[ "$checksum" == sha256sum ]]; then actual=$(sha256sum "$temporary/$asset")
  else actual=$(shasum -a 256 "$temporary/$asset")
  fi
  if [[ "${actual%% *}" != "$expected" ]]; then
    echo 'Helpin: checksum mismatch; nothing installed.' >&2; return 1
  fi
  if [[ -d "$destination/helpin" ]]; then
    echo 'Helpin: the destination helpin path is a directory; choose another HELPIN_BIN_DIR.' >&2; return 1
  fi
  mkdir -p -- "$destination"
  # Stage on the destination filesystem so replacement is atomic.
  staged=$(mktemp "$destination/.helpin-download.XXXXXX")
  trap 'rm -rf -- "$temporary"; rm -f -- "$staged"' EXIT
  cp -- "$temporary/$asset" "$staged"
  chmod 755 "$staged"
  mv -f -- "$staged" "$destination/helpin"
  echo "✓ Helpin installed at $destination/helpin"
  case ":$PATH:" in
    *":$destination:"*) echo 'Run: helpin install' ;;
    *) printf 'Run: %q install\n' "$destination/helpin"
       printf 'To use helpin from any directory, add this to your shell profile:\n  export PATH=%q:"$PATH"\n' "$destination" ;;
  esac
  # EXIT traps run after this function returns; clean up while locals exist.
  rm -rf -- "$temporary"
  trap - EXIT
}

install_helpin "$@"
