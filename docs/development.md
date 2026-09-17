# Local development

Run commands from the repository root unless a step says otherwise. Read the
[architecture overview](../ARCHITECTURE.md) for service ownership and code entry points.
For the complete Community stack, including Agent Runtime, use the
[Compose source-build guide](community/development.md#source-builds-and-acceptance).
The steps below are for editing the API and frontend as host processes.

## Prerequisites

- Go 1.26.7, pinned in [`.go-version`](../.go-version).
- Node.js 24.21.0, pinned in [`.node-version`](../.node-version).
- pnpm 10.32.1, pinned in the root `package.json`.
- Docker Engine with Compose V2 for local infrastructure.
- Optional: `just` for task shortcuts and Air for backend hot reload.

Native apps and the Rust event pipeline have additional requirements documented
in their component guides.

## Install and configure

```bash
pnpm install --frozen-lockfile
cp server/.env.example server/.env
```

Configure `server/.env` using the comments in the example file. At minimum, set the
database connection and authentication secrets, and configure the infrastructure
URLs for the services you run. Keep real credentials out of version control.

Start the local infrastructure:

```bash
docker compose up -d postgres redis nats temporal
```

See the [infrastructure guide](../docker/README.md) for database and Redis
connection values. Temporal listens on port `7233`. AI execution also requires
[Agent Runtime configuration](AGENT_RUNTIME_LOCAL.md) and
[AI connections and profiles](ai-connections.md).

## Run the application

Start the API:

```bash
cd server
go run ./cmd/api
```

In another terminal, from the repository root, build the shared widget dependency
and start the frontend:

```bash
pnpm --filter @helpin-ai/widget-core build
pnpm --dir frontend dev
```

The frontend normally runs at `http://localhost:5173` and uses the API at
`http://localhost:8080/api`. For a remote browser, set `VITE_API_URL` to an API
address reachable from that browser and configure backend CORS accordingly.

For Temporal-backed features, start the worker in a separate terminal:

```bash
cd server
go run ./cmd/temporal-worker
```

The [justfile](../justfile) provides shortcuts, including `just dev-full` for the
API, frontend, and worker. Its backend shortcut requires Air.

## Validate changes

Run checks appropriate to the components you changed:

```bash
# Backend (from server/)
go vet ./...
go test ./...
go build ./cmd/api ./cmd/temporal-worker

# Frontend (from the repository root)
pnpm --dir frontend test
pnpm --dir frontend build
```

Some integration tests need configured services. Component test instructions and
[CI](../.github/workflows/ci.yml) describe additional checks, including EE builds.
For documentation-only changes, verify relative links and run `git diff --check`.

## Further guides

- [Database migrations](ops/database-migrations.md)
- [Widget architecture and builds](widget-architecture.md)
- [Event pipeline](../events-pipeline/README.md)
- [Mobile support app](../apps/support-mobile/README.md)
- [Native release runbook](ops/native-release-runbook.md)
