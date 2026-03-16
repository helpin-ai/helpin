set shell := ["bash", "-uc"]

default:
    @just --list

frontend:
    cd frontend && pnpm dev

backend:
    cd server && air

dev:
    # Run frontend and backend together; stop both if either exits.
    bash -c 'set -euo pipefail; \
      (cd frontend && pnpm dev) & frontend_pid=$!; \
      (cd server && air) & backend_pid=$!; \
      trap "kill $frontend_pid $backend_pid 2>/dev/null || true" EXIT INT TERM; \
      wait -n $frontend_pid $backend_pid'

dev-tmux:
    tmux new-session -d -s helpin -n dev 'cd server && air'
    tmux split-window -h -t helpin:dev 'cd frontend && pnpm dev'
    tmux attach -t helpin

build-server:
    cd server && go mod download && go build -o bin/api ./cmd/api
    @echo "✅ server built → server/bin/api"

build-frontend:
    pnpm build --filter frontend
    @echo "✅ frontend built → frontend/dist"

help-center:
    cd help-center && npm run dev

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
