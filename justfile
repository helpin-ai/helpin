set shell := ["bash", "-uc"]

default:
    @just --list

frontend:
    cd frontend && pnpm dev

backend:
    cd server && air

worker:
    cd server && go run ./cmd/temporal-worker

dev:
    # Run frontend and backend together; stop both if either exits.
    bash -c 'set -euo pipefail; \
      (cd frontend && pnpm dev) & frontend_pid=$!; \
      (cd server && air) & backend_pid=$!; \
      trap "kill $frontend_pid $backend_pid 2>/dev/null || true" EXIT INT TERM; \
      wait -n $frontend_pid $backend_pid'

dev-full:
    # Run frontend, backend, and temporal worker together; stop all if any exits.
    bash -c 'set -euo pipefail; \
      (cd frontend && pnpm dev) & frontend_pid=$!; \
      (cd server && air) & backend_pid=$!; \
      (cd server && go run ./cmd/temporal-worker) & worker_pid=$!; \
      trap "kill $frontend_pid $backend_pid $worker_pid 2>/dev/null || true" EXIT INT TERM; \
      wait -n $frontend_pid $backend_pid $worker_pid'

dev-tmux:
    tmux new-session -d -s helpin -n dev 'cd server && air'
    tmux split-window -h -t helpin:dev 'cd frontend && pnpm dev'
    tmux split-window -v -t helpin:dev.1 'cd server && go run ./cmd/temporal-worker'
    tmux split-window -v -t helpin:dev.0 'cd website && pnpm dev'
    tmux new-window -t helpin -n helpcenter 'cd help-center && pnpm dev'
    tmux select-window -t helpin:dev
    tmux attach -t helpin

build-server:
    cd server && go mod download && go build -o bin/api ./cmd/api
    cd server && go build -o bin/temporal-worker ./cmd/temporal-worker
    @echo "✅ server built → server/bin/api, server/bin/temporal-worker"

build-frontend:
    pnpm build --filter frontend
    @echo "✅ frontend built → frontend/dist"

help-center:
    cd help-center && pnpm dev

build-help-center:
    cd help-center && npm install && npm run build
    @echo "✅ help-center built → help-center/dist"

kill-dev:
    -pkill -f 'pnpm dev' 2>/dev/null
    -pkill -f 'air' 2>/dev/null
    -pkill -f 'go run ./cmd/api' 2>/dev/null
    -pkill -f 'vite' 2>/dev/null
    @echo "✅ dev processes killed"

claude:
    tmux new-session -s claude "claude"

check:
    cd server && go vet ./... && go build ./cmd/api
    @echo "✅ vet + build passed"

events-up:
    ./events-pipeline/scripts/stack.sh up

events-down:
    ./events-pipeline/scripts/stack.sh down

events-logs:
    ./events-pipeline/scripts/stack.sh logs

events-status:
    ./events-pipeline/scripts/stack.sh status

events-smoke:
    ./events-pipeline/scripts/smoke.sh

events-e2e:
    ./events-pipeline/scripts/e2e.sh

events-browser-smoke:
    ./events-pipeline/scripts/browser-smoke.sh
