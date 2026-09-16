#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
fixture_db="community_migration_test_$$"
binary_dir=$(mktemp -d)
cleanup() {
  docker compose exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS $fixture_db WITH (FORCE)" >/dev/null
  rm -rf -- "$binary_dir"
}
trap cleanup EXIT
(cd ../server && GOWORK=off CGO_ENABLED=0 go test -buildvcs=false -tags integration -c -o "$binary_dir/test" ./internal/dbmigrate)
chmod 755 "$binary_dir" "$binary_dir/test"
docker compose exec -T postgres psql -U postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE $fixture_db OWNER helpin" -c "\\connect $fixture_db" \
  -c 'CREATE EXTENSION vector; CREATE EXTENSION pgcrypto'
docker compose run --rm --no-deps -v "$binary_dir/test:/test:ro" --entrypoint /bin/sh \
  -e "COMMUNITY_FIXTURE_DB=$fixture_db" helpin-migrate -c \
  'export COMMUNITY_TEST_DATABASE_URL="${DATABASE_URL%/*}/$COMMUNITY_FIXTURE_DB?sslmode=disable"; /test -test.run TestFreshCommunityFoundationPostgres -test.v'
