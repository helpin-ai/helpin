# Local Infrastructure Setup

Docker Compose manages all infrastructure services: PostgreSQL, Redis, NATS, and pgAdmin.

## Prerequisites

- Docker Engine 24+ with Compose V2 (`docker compose` command)

## Quick Start

```bash
# 1. Start infrastructure services (from repo root)
docker compose up -d postgres redis nats pgadmin

# 2. Verify all services are healthy
docker compose ps

# 3. Copy and configure server env
cp server/.env.example server/.env
```

Edit `server/.env` and set:
```env
DATABASE_URL=postgres://helpin:helpin@localhost:5432/helpin?sslmode=disable
REDIS_URL=redis://:helpin-redis-dev@localhost:6379/0
NATS_URL=nats://localhost:4222
```

Then start the backend and frontend as usual:
```bash
cd server && go run ./cmd/api     # Backend
cd frontend && pnpm dev           # Frontend (separate terminal)
```

## Services

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL 17 | `5432` | Primary database |
| Redis 7 | `6379` | WebSocket pub/sub, caching |
| NATS 2.11 | `4222` | Event streaming (JetStream) |
| NATS Monitoring | `8222` | NATS HTTP dashboard |
| pgAdmin 4 | `5050` | Database admin UI (accessible remotely) |

## pgAdmin Access

- URL: `http://<host-ip>:5050`
- The local PostgreSQL server is auto-registered (no manual setup needed)
- Login: `admin@helpin.ai` / `admin`

## Common Commands

```bash
# Start all infra
docker compose up -d postgres redis nats pgadmin

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
REDISCLI_AUTH=helpin-redis-dev docker compose exec redis redis-cli ping
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

- The `server` and `frontend` services in `docker-compose.yaml` are for full containerized deployment. For local development, run only the infra services (`postgres`, `redis`, `nats`, `pgadmin`) and start the backend/frontend directly.
- All data is persisted in Docker named volumes (`postgres_data`, `redis_data`, `nats_data`, `pgadmin_data`).
- PostgreSQL uses a healthcheck — dependent services wait until it's ready.
