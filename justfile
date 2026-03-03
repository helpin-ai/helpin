set shell := ["bash", "-uc"]

default:
    @just --list

frontend:
    cd frontend && npm run dev

backend:
    cd server && go run ./cmd/api

dev:
    # Run frontend and backend together; stop both if either exits.
    bash -c 'set -euo pipefail; \
      (cd frontend && npm run dev) & frontend_pid=$!; \
      (cd server && go run ./cmd/api) & backend_pid=$!; \
      trap "kill $frontend_pid $backend_pid 2>/dev/null || true" EXIT INT TERM; \
      wait -n $frontend_pid $backend_pid'

check:
    cd server && go vet ./... && go build ./cmd/api
    @echo "✅ vet + build passed"
