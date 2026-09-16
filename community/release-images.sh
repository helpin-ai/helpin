#!/usr/bin/env bash
# Called only after the native install/restore gate, in the protected release job.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
[[ ${COMMUNITY_RELEASE_TAG:-} =~ ^community-v0\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$ ]] || { echo 'Invalid beta release tag.' >&2; exit 1; }
[[ ${COMMUNITY_ARCH:-} == amd64 || ${COMMUNITY_ARCH:-} == arm64 ]] || exit 1
[[ -s ../LICENSE && -s ../../agent-runtime/LICENSE ]] || { echo 'Choose and add repository licenses before publishing.' >&2; exit 1; }
./check-images.sh
while IFS= read -r image; do
  [[ $image == ghcr.io/helpin-ai/* ]] || continue
  destination="${image%:*}:$COMMUNITY_RELEASE_TAG-$COMMUNITY_ARCH"
  docker tag "$image" "$destination"
  docker push "$destination"
  docker image inspect --format '{{index .RepoDigests 0}}' "$destination" >> "release-artifacts/images-$COMMUNITY_ARCH.txt"
done < <(docker compose config --images | sort -u)
