# Helpin

Internal performance-based quarterly bonus system with integrated project management.

## Architecture

- **Backend**: Go 1.24 + Chi router + GORM (PostgreSQL/Neon) + Temporal workflows
- **Frontend**: React 19 + Vite 7 + TypeScript 5.9 + TanStack Router + TanStack Query + Zustand + shadcn/ui
- **Database**: Neon PostgreSQL (pgcrypto for UUIDs)
- **Storage**: AWS S3 / MinIO (presigned URLs + direct upload)
- **Infra**: Kubernetes with Traefik, Doppler secrets, GHCR container registry

## Development

### Backend
```bash
cd server
go run ./cmd/api
```
Requires: `DATABASE_URL`, `JWT_SECRET` env vars. See `server/.env.example` for all options.

### Frontend
```bash
cd frontend
npm install
npm run dev
```
Requires: `VITE_API_URL` (defaults to `http://localhost:8080/api`)

### Docker
```bash
docker compose up
```

### Task Runner
```bash
just dev        # Start both backend + frontend
just backend    # Backend only
just frontend   # Frontend only
just build      # Production build
```

## Project Structure

```
server/                          # Go API server
  cmd/api/main.go                # Entry point, DI wiring
  internal/
    auth/                        # JWT token management
    authorization/               # RBAC engine, permissions, middleware
    config/                      # Environment configuration
    email/                       # Postmark email client
    githubapp/                   # GitHub OAuth + App integration
    handler/                     # HTTP request handlers
    middleware/                  # Auth, logging middleware
    model/                       # GORM struct definitions
    repository/                  # Data access layer (GORM queries)
    router/router.go             # Chi route registration
    service/                     # Business logic layer
    storage/s3.go                # S3/MinIO client
    temporalapp/                 # Temporal workflow setup
    websocket/                   # WebSocket hub + handler
    worker/                      # Background job workers
  migrations/                    # Sequential SQL migrations (001–022+)

frontend/                        # React SPA
  src/
    components/
      ui/                        # shadcn/ui primitives (30+ components)
      layout/                    # Sidebar, Header, WorkspaceSwitcher
      pm/                        # Project Management components
      workspace/                 # Workspace-specific components
    hooks/
      queries/                   # TanStack Query hooks (barrel-exported from index.ts)
    lib/
      api.ts                     # Fetch-based HTTP client with auto-refresh
      types.ts                   # Settings/workspace TypeScript interfaces
      pmTypes.ts                 # PM module TypeScript interfaces
      queryKeys.ts               # Query key factory
      queryClient.ts             # TanStack Query client config
      queryUtils.ts              # unwrap() helper
      services/                  # API service adapters (thin fetch wrappers)
      utils.ts                   # cn(), getInitials()
    pages/                       # Page-level components
    routes/                      # TanStack Router file-based routes
    stores/                      # Zustand stores (auth, workspace, org, etc.)

k8s/                             # Kubernetes manifests
  stage/                         # Staging (stage.helpin.ai)
  prod/                          # Production (helpin.ai)

.github/workflows/               # CI/CD pipelines
  ci.yml                         # PR checks (go vet + build, npm build)
  deploy-staging.yml             # develop → stage
  deploy-prod.yml                # main → prod
```

## Branches
- `develop` → Staging (stage.helpin.ai)
- `main` → Production (helpin.ai)

---

## Backend Patterns

### Handler → Service → Repository
Every feature follows this layering:
- **Handler** (`internal/handler/`): HTTP request/response, decode JSON, call service, write response
- **Service** (`internal/service/`): Business logic, validation, orchestration
- **Repository** (`internal/repository/`): GORM database queries

```go
// Handler
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var req model.UpdateWorkspaceRequest
    if err := decodeJSON(r, &req); err != nil { writeError(w, 400, "..."); return }
    ws, err := h.workspaceService.Update(r.Context(), id, req)
    writeJSON(w, http.StatusOK, ws)
}

// Service
func (s *WorkspaceService) Update(ctx context.Context, id string, req model.UpdateWorkspaceRequest) (*model.Workspace, error) {
    return s.workspaceRepo.Update(ctx, id, req.Name, req.Description, req.LogoURL, req.Timezone)
}

// Repository
func (r *WorkspaceRepository) Update(ctx context.Context, id string, name, description, logoURL, timezone *string) (*model.Workspace, error) {
    updates := map[string]interface{}{}
    if name != nil { updates["name"] = *name }
    // ...
    r.db.WithContext(ctx).Model(&model.Workspace{}).Where("id = ?", id).Updates(updates)
}
```

### GORM Model Conventions
```go
type Workspace struct {
    ID        string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name      string    `json:"name" gorm:"not null"`
    Slug      string    `json:"slug" gorm:"uniqueIndex;not null"`
    CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Workspace) TableName() string { return "workspaces" }
```
- UUIDs generated by PostgreSQL (`gen_random_uuid()`)
- JSON tags use `snake_case`
- Optional fields use `*string` / `*time.Time`
- Each model has an explicit `TableName()` method
- Request/response DTOs live in the same model file

### RBAC Authorization
Package: `server/internal/authorization/`

**Role hierarchy** (additive): `viewer → member → manager → admin → owner`

**Middleware chain**:
1. `RequireAuth` — validates JWT, injects UserID into context
2. `RequireWorkspaceAccess` — resolves Actor (membership + role + team memberships)
3. `RequirePermission(perm)` — checks role has permission via in-memory matrix

**Permission constants**: `PermWorkspaceRead`, `PermPMEdit`, `PermSettingsManage`, etc. (40+)

**Route pattern**:
```go
r.With(requirePerm(authorization.PermWorkspaceUpdate)).Put("/", h.Workspace.Update)
r.With(authorization.RequireOwner(authz)).Delete("/", h.Workspace.Delete)
```

### DI Wiring
All dependencies are wired in `cmd/api/main.go`:
1. Config → DB connection → AutoMigrate
2. Repositories created from `*gorm.DB`
3. Services created from repositories + external clients (S3, email, Temporal)
4. Handlers created from services
5. Router created from handlers + authorization service

### Database Migrations
- GORM `AutoMigrate` runs on startup for struct-level schema
- SQL migrations in `server/migrations/` numbered sequentially (001–022+)
- Migrations are idempotent (`IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`)

### WebSocket
- Endpoint: `GET /api/ws?token=JWT&workspace_id=ID`
- Bypasses Chi middleware stack (Recoverer strips http.Hijacker)
- Hub pattern: register/unregister clients, broadcast per workspace
- Uses `nhooyr.io/websocket` library

### S3 Storage
- Client: `internal/storage/s3.go`
- Supports AWS S3 and MinIO (path-style)
- Methods: `PutObject`, `DeleteObject`, `PublicURL`, `GeneratePresignedPutURL/GetURL`
- Public URLs via `HasPublicURL()` check

### Logging

**Library**: `log/slog` (Go stdlib, available since Go 1.21)

**Why `slog`**: Zero dependencies, structured JSON output, leveled logging, context-aware, and has native integrations for Chi and GORM. No need for `zap` or `zerolog`.

**Setup** (in `cmd/api/main.go`):
```go
import "log/slog"

// Production: JSON handler
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo, // Use LOG_LEVEL env to override
}))
slog.SetDefault(logger)
```

**Usage patterns**:
```go
// Basic structured logging
slog.Info("story created", "story_id", story.ID, "workspace_id", story.WorkspaceID)
slog.Error("failed to save", "error", err, "story_id", id)

// Context-aware (carries request_id, user_id from middleware)
slog.InfoContext(ctx, "comment added", "entity_id", entityID)
slog.ErrorContext(ctx, "notification emit failed", "error", err)

// With persistent attributes (in service constructors)
logger := slog.Default().With("service", "notification")
logger.Info("emitting notification", "event_type", event.EventType)
```

**Where to log**:
- **Handlers**: Log incoming requests with key params at DEBUG, errors at ERROR
- **Services**: Log business operations (create, update, delete) at INFO, failures at ERROR
- **Repository**: Log slow queries or failures at WARN/ERROR (not every query)
- **Middleware**: Log request/response summary (method, path, status, duration) at INFO
- **Background workers**: Log job start/complete at INFO, failures at ERROR
- **External calls**: Log S3, email, Temporal, GitHub API calls at INFO, failures at ERROR

**Levels**:
- `DEBUG`: Detailed diagnostic info (request params, intermediate values)
- `INFO`: Normal operations (story created, email sent, workflow started)
- `WARN`: Recoverable issues (slow query, retry, deprecated usage)
- `ERROR`: Failures requiring attention (DB error, external API failure)

**Rules**:
- Always use structured key-value pairs, never `fmt.Sprintf` in log messages
- Include `workspace_id`, `user_id`, `entity_id` for traceability
- Do NOT log sensitive data (passwords, tokens, email bodies)
- Do NOT silently discard errors with `_ =` — log them at minimum
- Use `slog.ErrorContext` instead of `log.Printf` for new code

---

## Frontend Patterns

### TanStack Router (File-Based)
Routes are defined in `src/routes/` and auto-generated into `routeTree.gen.ts`:
```
src/routes/
  __root.tsx                  # Global layout
  _authenticated.tsx          # Auth guard (redirects to /login)
    w/$slug.tsx               # Workspace layout (loads workspace, settings, access)
      pm/stories/index.tsx    # /w/:slug/pm/stories
      settings/$section.tsx   # /w/:slug/settings/:section
    workspaces.tsx            # /workspaces
```

Route files export:
```tsx
export const Route = createFileRoute('/path')({
  beforeLoad: ({ context }) => { /* auth checks, redirects */ },
  component: MyComponent,
})
```

### TanStack Query
**Query hooks** in `src/hooks/queries/` with barrel export at `index.ts`:
```tsx
export function useWorkspaces(organizationId?: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.all(organizationId),
    queryFn: async () => unwrap(await workspacesService.list(organizationId)),
    staleTime: 60_000,
  });
}
```

**Query key factory** in `src/lib/queryKeys.ts`:
```tsx
queryKeys.workspaces.all(orgId)        // ['workspaces', { orgId }]
queryKeys.workspaces.bySlug(slug)      // ['workspaces', 'slug', slug]
queryKeys.workspaces.settings(wsId)    // ['workspaces', wsId, 'settings']
```

**Query client** defaults (`src/lib/queryClient.ts`):
- `staleTime: 30_000`, `gcTime: 300_000`, `retry: 1`
- Mutations: `retry: 0`

**Error handling**: `unwrap()` in `src/lib/queryUtils.ts` converts `{ data, error }` → throws on error, returns data.

### API Client (`src/lib/api.ts`)
```tsx
const { data, error, status } = await api.get<T>('/path');
const { data, error } = await api.post<T>('/path', body);
```
- Auto-injects `Authorization: Bearer` from localStorage
- Auto-refreshes token on 401 via `/auth/refresh`
- Returns `{ data: T | null, error: string | null }`

### Service Layer (`src/lib/services/`)
Thin adapters wrapping `api.get/post/put/del`:
```tsx
export const workspacesService = {
  list: (orgId?) => api.get<Workspace[]>(`/workspaces?organization_id=${orgId}`),
  update: (id, data) => api.put<Workspace>(`/workspaces/${id}`, data),
  uploadLogo: async (id, file) => { /* multipart FormData POST */ },
};
```

### Zustand Stores (`src/stores/`)
Used for UI state alongside TanStack Query for server state:
```tsx
// workspaceStore.ts
const useWorkspaceStore = create<State>((set) => ({
  currentWorkspace: null,
  setCurrentWorkspace: (ws) => set({ currentWorkspace: ws }),
}));

// Usage
const workspace = useWorkspaceStore((s) => s.currentWorkspace);
```

Key stores: `authStore`, `workspaceStore`, `organizationStore`, `quarterStore`, `globalCreateStore`, `boardDisplayStore`

### UI Components
- **shadcn/ui**: 30+ installed components (Button, Card, Dialog, Popover, Table, etc.)
- **Radix UI**: Headless primitives underlying shadcn
- **lucide-react**: Icon library
- **CVA** (class-variance-authority): Component variants
- **cn()**: `clsx` + `tailwind-merge` for className composition
- **sonner**: Toast notifications
- **TipTap**: Rich text editor
- **dnd-kit**: Drag and drop
- **react-hook-form + zod**: Form validation

### Styling
- **Tailwind CSS v4** with oklch() color system
- CSS custom properties for theming (light + dark via `next-themes`)
- Color tokens: `--background`, `--primary`, `--destructive`, `--sidebar-*`, etc.
- Border radius: `0.625rem`

### TypeScript Types
- Settings/workspace types: `src/lib/types.ts`
- PM module types: `src/lib/pmTypes.ts`
- Permission type: union of 23 permission strings
- All backend JSON responses have matching TS interfaces

### RBAC (Frontend)
```tsx
const { data: access } = useWorkspaceAccess(workspaceId);
const { has, hasAny, canEdit, canManageSettings } = usePermissions(access);

if (has('pm.edit')) { /* show edit button */ }
```

### Real-time Updates
- `useRealtimeSync` hook invalidates TanStack Query cache on WebSocket events
- Also dispatches DOM custom events for legacy Zustand integration

---

## Key Libraries Reference

### Backend (Go)
| Library | Purpose |
|---------|---------|
| `go-chi/chi/v5` | HTTP router |
| `gorm.io/gorm` | ORM (PostgreSQL) |
| `golang-jwt/jwt/v5` | JWT authentication |
| `aws-sdk-go-v2` | S3 storage |
| `nhooyr.io/websocket` | WebSocket |
| `go.temporal.io/sdk` | Workflow engine |
| `go-chi/cors` | CORS middleware |
| `joho/godotenv` | .env loading |
| `golang.org/x/crypto` | Password hashing |

### Frontend (npm)
| Library | Purpose |
|---------|---------|
| `react` 19 | UI framework |
| `vite` 7 | Build tool |
| `@tanstack/react-router` | File-based routing |
| `@tanstack/react-query` | Server state management |
| `zustand` | Client state management |
| `shadcn/ui` + `@radix-ui` | Component library |
| `tailwindcss` 4 | Utility-first CSS |
| `lucide-react` | Icons |
| `sonner` | Toast notifications |
| `@tiptap/react` | Rich text editor |
| `@dnd-kit` | Drag and drop |
| `react-hook-form` + `zod` | Form validation |
| `date-fns` + `dayjs` | Date utilities |
| `@tanstack/react-table` | Data tables |
| `@tanstack/react-virtual` | Virtualized lists |
| `cmdk` | Command palette |
| `next-themes` | Dark mode |
