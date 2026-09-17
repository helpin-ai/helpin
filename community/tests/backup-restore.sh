#!/usr/bin/env bash
# Destructive only to the newly created restore fixture. Source volumes survive.
# Requires an explicitly disposable source stack because its writers are stopped.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
[[ ${COMMUNITY_TEST_ROOT:-} == "$PWD" && -f .acceptance-project ]] || { echo 'Run tests/run.sh full; restore is restricted to its temporary project.' >&2; exit 1; }
[[ ${COMPOSE_PROJECT_NAME:-} == "$(cat .acceptance-project)" && $COMPOSE_PROJECT_NAME == community-test-* ]] || exit 1
umask 077
source_container=$(docker compose ps -q postgres)
[[ -n $source_container ]] || { echo 'Start the test stack first.' >&2; exit 1; }
source_project=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$source_container")
[[ $source_project == "$COMPOSE_PROJECT_NAME" ]] || { echo 'Refusing a different source project.' >&2; exit 1; }
restore_project="${source_project}-restore"
snapshot=$(mktemp -d)
cleanup() {
  rm -rf -- "$snapshot"
}
# The parent acceptance runner owns both projects, including cleanup on failure.
trap cleanup EXIT
# Save keys/config along with the snapshot without printing them.
cp .env apps.json "$snapshot/"
docker compose -p "$source_project" stop helpin-frontend helpin-helpcenter helpin-worker agent-runtime-worker agent-runtime helpin-api
docker compose -p "$source_project" exec -T postgres psql -U postgres -d helpin -At -c \
  'SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM support_messages), (SELECT count(*) FROM schema_migrations)' > "$snapshot/counts"
docker compose -p "$source_project" stop
for volume in postgres_data garage_data redis_data nats_data; do
  docker volume inspect "${source_project}_${volume}" >/dev/null
  docker volume create --label "com.docker.compose.project=$restore_project" --label "com.docker.compose.volume=$volume" "${restore_project}_${volume}" >/dev/null
  docker run --rm --network none -v "${source_project}_${volume}:/source:ro" -v "$snapshot:/backup" alpine:3.21.3 \
    sh -c 'cd /source && tar czf "/backup/$1.tgz" .' sh "$volume"
  docker run --rm --network none -v "${restore_project}_${volume}:/restore" -v "$snapshot:/backup:ro" alpine:3.21.3 \
    sh -c 'cd /restore && tar xzf "/backup/$1.tgz"' sh "$volume"
done
# Same images, same keys, independently restored data volumes. Original ports are free.
docker compose -p "$restore_project" --env-file "$snapshot/.env" up -d --wait --wait-timeout 300
docker compose -p "$restore_project" exec -T postgres psql -U postgres -d helpin -At -c \
  'SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM support_messages), (SELECT count(*) FROM schema_migrations)' > "$snapshot/restored-counts"
cmp "$snapshot/counts" "$snapshot/restored-counts"
docker compose -p "$restore_project" run --rm --no-deps helpin-migrate validate
node tests/http-smoke.mjs
if [[ -n ${COMMUNITY_TEST_PROOF_FILE:-} ]]; then
  docker compose -p "$restore_project" --env-file "$snapshot/.env" -f compose.yaml -f tests/compose.fixture.yaml up -d --wait --wait-timeout 300
  node tests/restore-ai.mjs
fi
echo 'PASS: all four persistent volumes restored; data counts, migration ledger, stack health and support journey verified'
