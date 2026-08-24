#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

set -a
source server/.env
source events-pipeline/.env.events
set +a

services=(
  kafka
  kafka-init
  clickhouse
  event-capture
  event-transformer
  event-sessionizer
  event-replay
)

case "${1:-}" in
  up)
    : "${INTERNAL_API_SECRET:?INTERNAL_API_SECRET is required in server/.env}"
    docker compose --profile events up -d --build "${services[@]}"
    ;;
  down)
    docker compose --profile events stop "${services[@]}"
    docker compose --profile events rm -f "${services[@]}"
    ;;
  logs)
    docker compose --profile events logs -f --tail=200 "${services[@]}"
    ;;
  *)
    echo "usage: $0 {up|down|logs}" >&2
    exit 2
    ;;
esac
