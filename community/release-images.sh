#!/usr/bin/env bash
# Publish only the image archive produced by full native acceptance and scans.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
[[ ${COMMUNITY_RELEASE_TAG:-} =~ ^community-v0\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$ ]] || exit 1
[[ ${COMMUNITY_ARCH:-} == amd64 || ${COMMUNITY_ARCH:-} == arm64 ]] || exit 1
[[ $(cat "release-artifacts/source-$COMMUNITY_ARCH.txt") == "$(git rev-parse HEAD)" ]] || exit 1
cmp runtime-revision.txt "release-artifacts/runtime-$COMMUNITY_ARCH.txt"
(cd release-artifacts && sha256sum -c tested-images.tar.gz.sha256)
gzip -dc release-artifacts/tested-images.tar.gz | docker load
# Validate the entire inventory before the first registry write.
python3 - <<'PY'
import json, os, re, subprocess
from pathlib import Path
expected = set(re.findall(r'^    image: (ghcr.io/helpin-ai/[a-z-]+):', Path('compose.yaml').read_text(), re.M))
inventory = json.loads(Path('release-artifacts/tested-applications.json').read_text())
images = Path('release-artifacts/application-images.txt').read_text().splitlines()
if set(inventory) != set(images) or len(images) != len(set(images)):
    raise SystemExit('Tested image inventories differ')
if {i.split(':', 1)[0] for i in images} != expected:
    raise SystemExit('Incomplete application image inventory')
for image, tested in inventory.items():
    if not re.fullmatch(r'ghcr.io/helpin-ai/[a-z-]+:[a-zA-Z0-9_.-]+', image):
        raise SystemExit('Invalid application image reference')
    loaded = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image]))[0]
    if loaded['Id'] != tested['id'] or loaded['Architecture'] != tested['architecture'] or tested['architecture'] != os.environ['COMMUNITY_ARCH']:
        raise SystemExit('Loaded image differs from tested identity or architecture')
PY
: > "release-artifacts/images-$COMMUNITY_ARCH.txt"
while IFS= read -r image; do
  destination="${image%:*}:$COMMUNITY_RELEASE_TAG-$COMMUNITY_ARCH"
  docker tag "$image" "$destination"
  docker push "$destination"
  docker image inspect --format '{{index .RepoDigests 0}}' "$destination" >> "release-artifacts/images-$COMMUNITY_ARCH.txt"
done < release-artifacts/application-images.txt
