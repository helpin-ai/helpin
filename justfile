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

build-server:
    cd server && go mod download && go build -o bin/api ./cmd/api
    @echo "✅ server built → server/bin/api"

build-frontend:
    cd frontend && npm install && npm run build
    @echo "✅ frontend built → frontend/dist"

check:
    cd server && go vet ./... && go build ./cmd/api
    @echo "✅ vet + build passed"
