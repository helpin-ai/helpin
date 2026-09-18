#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
umask 077
fail() { printf '%s\n' "$*" >&2; exit 1; }
command -v docker >/dev/null || fail 'Install Docker Engine and the Docker Compose plugin first.'
docker compose version >/dev/null || fail 'Docker Compose v2 is required.'
# Inline config is inherited only by the agent-runtime worker. Never mount it where commands can read it.
if [[ -f apps.json ]]; then
  export AGENT_RUNTIME_EXECUTION_APP_CONFIG="$(cat apps.json)"
fi
case "${1:-}" in
  install)
    command -v openssl >/dev/null || fail 'Install openssl to generate secrets.'
    if [[ ! -e .env ]]; then
      temporary=$(mktemp .env.XXXXXX)
      trap 'rm -f -- "$temporary"' EXIT
      while IFS= read -r line || [[ -n "$line" ]]; do
        case "$line" in
          *=GENERATE_GARAGE_ACCESS_KEY) printf '%s=GK%s\n' "${line%%=*}" "$(openssl rand -hex 16)" ;;
          *=GENERATE_BASE64) printf '%s=%s\n' "${line%%=*}" "$(openssl rand -base64 32)" ;;
          *=GENERATE) printf '%s=%s\n' "${line%%=*}" "$(openssl rand -hex 32)" ;;
          *) printf '%s\n' "$line" ;;
        esac
      done < .env.example > "$temporary"
      mv -- "$temporary" .env
      trap - EXIT
    fi
    [[ -e apps.json ]] || cp apps.example.json apps.json
    chmod 600 .env
    # Runtime runs as an unprivileged user; this contains env variable names only.
    chmod 644 apps.json
    printf '%s\n' 'Configuration ready. Edit .env public URLs and optional mail/AI settings, then run ./setup.sh start. Existing secrets were preserved.'
    ;;
  start)
    [[ -f .env && -f apps.json ]] || fail 'Run ./setup.sh install first.'
    if awk -F= '$2 == "GENERATE" || $2 == "GENERATE_BASE64" || $2 == "GENERATE_GARAGE_ACCESS_KEY" {found=1} END {exit !found}' .env; then
      fail 'Uninitialized secret placeholders. Use setup.sh install in a new directory or generate the missing secrets.'
    fi
    docker compose --env-file .env -f compose.yaml config --quiet
    docker compose --env-file .env -f compose.yaml up -d --wait --wait-timeout 300
    ;;
  stop) docker compose --env-file .env -f compose.yaml down ;;
  logs) shift; docker compose --env-file .env -f compose.yaml logs --tail=150 -f "$@" ;;
  status)
    docker compose --env-file .env -f compose.yaml ps --all
    docker compose --env-file .env -f compose.yaml images
    if docker compose --env-file .env -f compose.yaml exec -T postgres pg_isready -U postgres -d helpin >/dev/null 2>&1; then
      docker compose --env-file .env -f compose.yaml exec -T postgres psql -U postgres -d helpin -At -c \
        "SELECT 'Helpin migration: ' || max(version) FROM schema_migrations" || true
      for database in temporal temporal_visibility; do
        docker compose --env-file .env -f compose.yaml exec -T postgres psql -U postgres -d "$database" -At -c \
          "SELECT '$database schema: ' || curr_version FROM schema_version" || true
      done
    fi
    printf '%s\n' 'Runtime schema has no ledger head; record the Runtime image ID above.'
    ;;
  *) fail 'Usage: ./setup.sh install|start|stop|logs [service...]|status' ;;
esac
