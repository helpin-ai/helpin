# Local development infrastructure

Use the root [Compose file](../docker-compose.yaml) to run PostgreSQL, Redis,
NATS, and Temporal while developing Helpin on the host. pgAdmin and Temporal UI
are optional administration tools. For the complete self-hosted application, use
the [Community installation guide](../community/README.md).

## Prerequisites

- Docker Engine 24+ with Compose V2 (`docker compose` command)

## Quick Start

```bash
# 1. Start infrastructure services (from repo root)
docker compose up -d postgres redis nats temporal temporal-ui pgadmin

# 2. Check service state and available health checks
docker compose ps

# 3. Copy and configure server env
cp server/.env.example server/.env
```

Edit `server/.env` and set:
```env
DATABASE_URL=postgres://helpin:helpin@localhost:5432/helpin?sslmode=disable
REDIS_URL=redis://:helpin-redis-dev@localhost:6379/0
NATS_URL=nats://localhost:4222
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TLS_ENABLED=false
TEMPORAL_API_KEY=
```

These are local development connection values. If you override `REDIS_PASSWORD`,
use the same password in `REDIS_URL`. Complete the remaining authentication and
application settings, then follow [local development](../docs/development.md)
to start the API, frontend, and Temporal worker in separate terminals.

## Services

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL 17 | `5432` | Primary database |
| Redis 7 | `6379` | WebSocket pub/sub, caching |
| NATS 2.14.5 | `4222` | Event streaming (JetStream) |
| NATS Monitoring | `8222` | NATS HTTP dashboard |
| Temporal | `7233` | Workflow service; uses a separate PostgreSQL container |
| Temporal UI | `8233` | Workflow administration |
| pgAdmin 4 | `5050` | Database admin UI (accessible remotely) |

## pgAdmin Access

- URL: `http://<host-ip>:5050`
- The local PostgreSQL server is auto-registered (no manual setup needed)
- Login: `admin@helpin.ai` / `admin`

## Common Commands

```bash
# Start all infra
docker compose up -d postgres redis nats temporal temporal-ui pgadmin

# View logs
docker compose logs -f postgres
docker compose logs -f redis

# Stop services (data preserved in volumes)
docker compose down

# Stop and delete all data
docker compose down -v

# Restart a single service
docker compose restart redis

# Connect to Redis with the local default password
docker compose exec -e REDISCLI_AUTH=helpin-redis-dev redis redis-cli ping
```

## Port Conflicts

If a service fails to start with "address already in use":

```bash
# Find what's using the port (e.g., 6379)
ss -tlnp | grep 6379

# If it's a system service (e.g., redis-server)
sudo systemctl stop redis-server
sudo systemctl disable redis-server

# Then retry
docker compose up -d redis
```

## Notes

- The root Compose file's API and frontend examples are commented out; they are
  not runnable services. It does define an optional `temporal-worker` container.
- Temporal starts its own PostgreSQL dependency automatically. Its data persists
  in `temporal_postgres_data`, alongside the other services' named volumes.
- These defaults are for local development. Use the Community guide for deployment.
