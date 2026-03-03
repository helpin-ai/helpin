# Teampulse

Internal performance-based quarterly bonus system.

## Architecture
- **Backend**: Go + Chi router, pgx for PostgreSQL (Neon)
- **Frontend**: React 18 + Vite + TypeScript + shadcn/ui
- **Database**: Neon PostgreSQL
- **Infra**: Kubernetes with Traefik, Doppler secrets

## Development

### Backend
```bash
cd server
go run ./cmd/api
```
Requires: DATABASE_URL, JWT_SECRET env vars

### Frontend
```bash
cd frontend
npm install
npm run dev
```

### Docker
```bash
docker compose up
```

## Project Structure
- `server/` — Go API server
- `frontend/` — React SPA
- `k8s/` — Kubernetes manifests (stage + prod)
- `.github/workflows/` — CI/CD pipelines

## Branches
- `develop` → Staging (stage.teampulse.d4interactive.io)
- `main` → Production (teampulse.d4interactive.io)
