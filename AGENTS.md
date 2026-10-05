# Helpin contributor instructions

Unified platform for project management, CRM, customer support, knowledge, and AI-assisted execution. The Community 0.1 beta ships the support, docs, and agents modules; see [ROADMAP.md](ROADMAP.md).

## Community and cloud boundaries

Helpin ships both open-source Community and hosted cloud editions. Before every
change, classify code, configuration, migrations, docs, fixtures, and assets as
shared Community, Enterprise/cloud-only, or private operational material. Follow
[edition architecture](ARCHITECTURE.md#editions-and-module-availability),
[LICENSE](LICENSE), and the [publication gate](community/PUBLICATION.md).

- Keep shared product behavior usable in both editions. Put subscription billing,
  commercial pricing/charging, payment UI, and hosted entitlements in the existing
  `server/ee/` and `frontend/src/ee/` implementations behind edition contracts.
  Community must work without EE source, billing tables, or Helpin-managed credentials.
- Build tags, feature flags, hidden UI, and Enterprise licenses do not make tracked
  source private. A push to any branch of a public repository exposes its files
  and reachable history, including docs, screenshots, fixtures, and workflow data.
  Keep credentials, customer data, private strategy, and private cloud operations
  out of this repository; do not copy them into plans or review evidence.
- If feature ownership or public visibility is unclear, ask the owner before
  implementing the disputed boundary or committing/pushing the material. Do not
  infer open-source approval from a cloud deployment request or a prior push.
  Continue independent, clearly classified work while the decision is pending.
- Before pushing, inspect the full outgoing commit range, not only the last diff.
  Verify affected behavior in both editions; for boundary changes run
  `scripts/check-community-backend.sh`, `scripts/check-community-frontend.sh`,
  relevant `go test -tags ee` checks, and the hosted frontend build/tests.
  Run `python3 scripts/ci/check_secrets.py`; its passing result is not a complete
  history, personal-data, or confidentiality review. Record scope and limitations.
- Never change repository visibility, rewrite published history, move EE source
  into the open-source scope, or publish unresolved material without an explicit
  owner decision. Follow the publication gate for releases and public exports.

## Small Fix Workflow

For small, well-scoped fixes, do not create or modify plan, specification, or design documents unless the user explicitly requests them. Inspect the issue, implement the fix, verify it, and commit it directly. Reserve planning, specification, and design documents for substantial multi-step work or explicit user requests.

Do not write or run tests for small, low-risk changes such as copy, capitalization, spacing, colors, or minor styling. Use judgment to test when behavior, complexity, or regression risk warrants it, or when explicitly requested. For simple edits, reviewing the source or diff is sufficient verification; avoid repetitive browser checks, screenshots, or test suites. Build only when needed to update the preview or deliver the change.

## Documentation

For documentation creation, explanation, naming, or reorganization, use the
repo-scoped `$helpin-documentation` skill in `.agents/skills/helpin-documentation/`.
The shared convention is `docs/documentation-guide.md`. Keep guide names lowercase
and hyphenated, update incoming references on renames, and validate documentation
with the checks described there.

## Website

For the marketing site in `website/` (helpin.ai), use the three website skills:

- [`helpin-website-copy`](.agents/skills/helpin-website-copy/SKILL.md) for
  voice, house style, positioning, claims, and copy drafts, audits, or edits.
- [`helpin-website-pages`](.agents/skills/helpin-website-pages/SKILL.md) for
  page structure, shared components, SEO metadata, QA, and release.
- [`helpin-website-visuals`](.agents/skills/helpin-website-visuals/SKILL.md)
  for product imagery, heroes, videos, and motion.

Preserve the existing page structure and distinguish editorial direction from
release-specific product evidence. A draft request does not authorize website
edits. Product facts and owner decisions the site may state live in
[`docs/website-copy/product-truth.yaml`](docs/website-copy/product-truth.yaml);
code sources such as `website/src/app/pricing/pricing-data.ts` win when they
disagree.

## Architecture

- **Backend**: Go 1.26.7 toolchain (`.go-version`; module minimum 1.25.0) + Chi router + GORM (PostgreSQL) + Temporal workflows
- **Frontend**: React 19 + Vite 7 + TypeScript 5.9 + TanStack Router + TanStack Query + Zustand + shadcn/ui
- **Database**: PostgreSQL with pgvector; see Community Compose for self-hosted defaults
- **Storage**: S3-compatible object storage (Garage in the Community bundle; AWS S3 or MinIO elsewhere) with presigned URLs and direct upload
- **Infra**: Docker Compose for Community; Kubernetes with Traefik for the hosted service; GHCR container registry

## Development

### Backend
```bash
cd server
go run ./cmd/api
```
Requires `DATABASE_URL` and `JWT_SECRET`, a migrated PostgreSQL database, and reachable NATS. `NATS_URL` defaults to `nats://localhost:4222`. See [backend configuration](server/.env.example) and [local development](docs/development.md) for setup and edition-specific dependencies.

### Temporal Worker
```bash
cd server
go run ./cmd/temporal-worker
```
Runs Temporal-backed workflows. Agent execution also depends on the configured Agent Runtime; running this worker alone does not provision that runtime.

### Frontend
```bash
# From the repository root
pnpm install --frozen-lockfile
pnpm --filter @helpin-ai/widget-core build
pnpm --dir frontend dev
```
Requires: `VITE_API_URL` (defaults to `http://localhost:8080/api`)

### Local Browser Preview
When starting the frontend for someone who will open it through the machine/network URL
(for example `http://dev.example.test:5173`), do not leave `VITE_API_URL` pointed at
`localhost`. In that browser, `localhost` means the user's computer, not this dev box,
and the app will show "unable to reach the server".

Use the same reachable host for the API:
```bash
cd server
set -a && . ./.env && set +a && GOCACHE=/tmp/go-build-cache go run ./cmd/api

cd frontend
VITE_API_URL=http://dev.example.test:8080/api pnpm dev --host 0.0.0.0 --port 5173
```

If the user opens `http://localhost:5173` from the same machine running the server,
`VITE_API_URL=http://localhost:8080/api` is fine. Before handing off a preview, check
both `http://<host>:5173/` and `http://<host>:8080/api/health` from the same host the
user will use.

### Docker
```bash
docker compose up
```

### Task Runner
```bash
just dev        # Start both backend + frontend
just backend    # Backend only; requires air
just frontend   # Frontend only
just dev-full   # Backend, frontend, and Temporal worker
just build-server    # API and worker build
just build-frontend  # Frontend production build
```

## Refactors and Migrations

When replacing or moving an existing feature, do a capability parity audit before implementation: entry points, required/optional inputs, generated defaults, editable fields before save, backend side effects, validation, permissions, navigation, and tests. If any old capability is removed or changed, call it out as an explicit product decision before coding.

## Project Structure

```
server/                          # Go API server
  cmd/api/main.go                # Entry point, DI wiring
  internal/
    auth/                        # JWT token management
    authorization/               # RBAC engine, permissions, middleware
    config/                      # Environment configuration
    crypto/                      # AES-256-GCM encryption helpers
    email/                       # Application email abstraction, SMTP, and Postmark
    githubapp/                   # GitHub OAuth + App integration
    handler/                     # HTTP request handlers
    llm/                         # Model-agnostic LLM provider (Claude + OpenAI)
    middleware/                  # Auth, logging middleware
    model/                       # GORM struct definitions
    oauth/                       # OAuth2 clients (Gmail)
    repository/                  # Data access layer (GORM queries)
    router/router.go             # Chi route registration
    service/                     # Business logic layer
    storage/s3.go                # S3/MinIO client
    sync/                        # External API sync clients (Gmail)
    temporalapp/                 # Temporal workflow setup
    websocket/                   # WebSocket hub + handler
  migrations/                    # Legacy SQL references; use internal/dbmigrate/sql for new migrations

frontend/                        # React SPA
  src/
    components/
      ui/                        # shadcn/ui primitives (30+ components)
      layout/                    # Sidebar, Header, WorkspaceSwitcher
      crm/                       # CRM components (SuggestionCard, DealCard, etc.)
      pm/                        # Project Management components
      workspace/                 # Workspace-specific components
    hooks/
      queries/                   # TanStack Query hooks (barrel-exported from index.ts)
    lib/
      api.ts                     # Fetch-based HTTP client with auto-refresh
      types.ts                   # Settings/workspace TypeScript interfaces
      pmTypes.ts                 # PM module TypeScript interfaces
      crmTypes.ts                # CRM module TypeScript interfaces
      queryKeys.ts               # Query key factory
      queryClient.ts             # TanStack Query client config
      queryUtils.ts              # unwrap() helper
      services/                  # API service adapters (thin fetch wrappers)
      utils.ts                   # cn(), getInitials()
    pages/                       # Page-level components
    routes/                      # TanStack Router file-based routes
    stores/                      # Zustand stores (auth, workspace, org, etc.)

helpin-ai/gitops                 # Hosted Kubernetes manifests (separate private repo)

.github/workflows/               # CI/CD pipelines
  ci.yml                         # PR checks (Go and pnpm validation)
  deploy-*.yml                   # Hosted-service deployments
```

## Branches
- `develop`: integration branch; open pull requests against it
- `main`: release branch for the hosted service

---

## Backend Patterns

### Handler → Service → Repository
Use this layering for feature changes:
- **Handler** (`internal/handler/`): HTTP request/response, decode JSON, call service, write response
- **Service** (`internal/service/`): Business logic, validation, orchestration
- **Repository** (`internal/repository/`): GORM database queries

```go
// Schematic example: consult current Workspace signatures before copying.
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

**Role hierarchy** (additive): `viewer → member → admin → owner`

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
The API composition root is `cmd/api/main.go`; Temporal workers also have their own wiring in `cmd/temporal-worker/main.go`:
1. Config → DB connection → AutoMigrate when enabled
2. Repositories created from `*gorm.DB`
3. Services created from repositories + external clients (S3, email, Temporal)
4. Handlers created from services
5. Router created from handlers + authorization service

### Database Migrations

**Two migration paths**:

Community Compose disables AutoMigrate and applies versioned SQL through its
migration service. Ship versioned SQL for schema additions as well as destructive
or data changes; model changes alone do not migrate those installations.

1. **GORM AutoMigrate** — runs on startup only when `RUN_AUTO_MIGRATE=true`, handles struct-level schema creation (add tables/columns). Cannot drop columns or tables.
2. **dbmigrate** (`server/internal/dbmigrate/`) — versioned SQL migrations for everything AutoMigrate cannot do: data migrations, table drops, cutover tasks, constraint changes, backfills.

**dbmigrate CLI** (`server/cmd/migrate/`):
```bash
go run ./cmd/migrate up              # Apply all pending migrations
go run ./cmd/migrate status          # Show all migrations (applied/pending)
go run ./cmd/migrate head            # Show latest applied migration
go run ./cmd/migrate pending         # List only unapplied migrations
go run ./cmd/migrate validate        # CI check — exit 1 if issues found
go run ./cmd/migrate repair          # Fix checksums after post-apply file edits
go run ./cmd/migrate create <name>   # Scaffold new migration file
```

**Migration files**: `server/internal/dbmigrate/sql/YYYYMMDDNNNN_name.sql` (embedded via `//go:embed`)

**When to use which**:
- **AutoMigrate**: Development schema additions when enabled; also provide versioned SQL for installations where it is disabled
- **dbmigrate**: Dropping tables/columns, data backfills, constraint changes, renaming, cutover tasks, any DDL that AutoMigrate cannot express

**Rules**:
- Migrations MUST be idempotent (`IF NOT EXISTS`, `IF EXISTS`)
- Never edit an already-applied migration file — create a new one instead (use `migrate repair` only for a reviewed checksum recovery; it does not execute changed SQL)
- Legacy SQL migrations in `server/migrations/` are reference docs only — new migrations go in `server/internal/dbmigrate/sql/`

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

### CRM Intelligence and CRM Signals

Canonical reference: `docs/crm-signals.md`

The CRM intelligence layer combines verified conversation extraction with
versioned deterministic rules over CRM, support, PM, calendar, and behavioral
evidence. Signals are stored in Postgres, scored separately from extraction
confidence, and remain activation-gated by rule version, identity trust,
business priority, deduplication, and routing policy.

Evidence and meaning are separate concerns:

- **observation** — immutable, motion-agnostic evidence from one detector
  (`crm_signal_observations`)
- **commercial motion** — which commercial context the account is in, resolved
  per entity and concurrent by design: `prospecting`, `conversion`,
  `onboarding`, `adoption`, `expansion`, `renewal`, `retention`
- **interpretation** — a versioned `(rule_key, rule_version, motion)` mapping to
  signal type, polarity, weight, half-life, and recommended action
  (`crm_signal_interpretation_configs`)

One observation produces one signal per applicable motion, so the same evidence
can mean expansion intent in one lane and retention risk in another. Meaning is
snapshotted at detection and is immutable; read-time rescoring may change
magnitude only. An observation with no mapping deliberately produces no signal.

Deals carry a `commercial_motion` inherited from
`crm_pipelines.default_commercial_motion` unless overridden per deal.

**Key packages:**
- `internal/crypto/` — AES-256-GCM token encryption (`CRM_ENCRYPTION_KEY` env)
- `internal/oauth/` — Gmail OAuth2 flow (scopes: `gmail.readonly`, `gmail.send`, `gmail.modify`, `calendar.readonly`)
- `internal/sync/` — Gmail REST API client (message sync, send, token auto-refresh)
- `internal/llm/` — Model-agnostic LLM interface (`Provider` interface with Claude + OpenAI adapters)
- `internal/service/crm_signal_detection.go` — verified LLM signal extraction from emails/calendar/support
- `internal/service/crm_signal_rule_evaluator.go` — daily and behavioral deterministic rules
- `internal/repository/crm_signal_interpretation.go` — motion resolver, observations, interpretation, motion-exit supersession
- `internal/service/crm_signal_score.go` — explainable ranking and `(entity, motion)` lane composition
- `internal/service/crm_signal_activation.go` — versioned activation, routing, rollout, and feedback
- `internal/service/crm_commercial_state.go` / `crm_commercial_state_sync.go` — server-authenticated company state patches and materialization
- `internal/service/crm_usage_baseline_sync.go` — daily workspace-local weekday baselines
- `internal/eventcatalog/` — shared server-only event catalog; generated into Go and Rust by `scripts/generate-commercial-event-catalog.go`
- `internal/service/crm_deal_automation.go` — Auto-create/progress deals based on signal confidence

**Temporal Workflows:**
| Workflow | Schedule | Purpose |
|----------|----------|---------|
| `EmailSyncWorkflow` | Long-running per account (5min poll) | Gmail backfill + incremental sync |
| `SignalDetectionWorkflow` | Event-driven (child of sync/support) | Extract CRM signals via LLM |
| `DealManagementCronWorkflow` | Hourly cron | Evaluate deal progression |

Deterministic rule evaluation is backend-owned rather than Temporal-owned: the
API runs daily Postgres rules and ten-minute behavioral rules with global
watermarks and leases.

**Autonomy Thresholds** (`CRMAutonomySettings` model):
- `auto_execute_threshold` (default 0.9) — signals above this auto-create/advance deals
- `review_threshold` (default 0.7) — signals between review and auto-execute create pending suggestions
- Below review threshold: low-priority suggestions

**Signal Types** (7): `buying_intent`, `objection`, `competitor_mention`,
`budget_signal`, `timeline_signal`, `champion_signal`, `risk_signal`

**Signal Sources**: conversation records, support, CRM, PM, web behavior,
instrumented product usage, and normalized external evidence.

**Two independent gates** — do not conflate them:

- **Workspace rollout** (`crm_signal_rollout_settings.mode`) controls whether
  motion lanes and routing are visible. Workspaces are **live by default**;
  `shadow` is an explicit per-workspace opt-out. Detection, commercial-state
  materialization, and baseline building run regardless of mode so history is
  warm.
- **Per-rule policy** (`shadow_mode` / `activation_eligible` on
  `crm_signal_rule_configs`) controls whether a rule version may create
  downstream work. All seeded deterministic rules start in rule-level shadow
  mode. Do not describe signals as autonomously actionable unless the exact rule
  version and routing policy have been promoted through the activation gates.

Activating a rule version requires an enabled interpretation for that exact
`(rule_key, rule_version)`; activation is refused otherwise, because the lookup
matches the version exactly and an unmapped version would silently go quiet.

**Support → CRM Bridge**: Support tickets auto-match to CRM contacts by email. If no contact exists, one is auto-created with `lifecycle_stage=subscriber`, `source=support`.

**Environment Variables** (CRM intelligence):
| Variable | Required | Description |
|----------|----------|-------------|
| `CRM_ENCRYPTION_KEY` | Yes (if Gmail) | 32-byte hex-encoded AES key |
| `GMAIL_CLIENT_ID` | Yes (if Gmail) | Google OAuth client ID |
| `GMAIL_CLIENT_SECRET` | Yes (if Gmail) | Google OAuth client secret |
| `GMAIL_OAUTH_REDIRECT_URL` | Yes (if Gmail) | OAuth redirect URL |
| `CRM_LLM_PROVIDER` / `CRM_LLM_MODEL` | No | Optional CRM primary-route override; empty preserves OpenRouter GLM 5.3 Flash Exacto |
| `CRM_LLM_OPENROUTER_PROVIDER` | No | Optional OpenRouter infrastructure provider for the primary route |
| `CRM_LLM_FALLBACK_PROVIDER` / `CRM_LLM_FALLBACK_MODEL` | No | Optional general CRM fallback override; empty preserves OpenRouter GPT-5.6 Luna |
| `CRM_LLM_FALLBACK_OPENROUTER_PROVIDER` | No | Optional OpenRouter infrastructure provider for the general fallback |
| `CRM_MEETING_LLM_FALLBACK_PROVIDER` / `CRM_MEETING_LLM_FALLBACK_MODEL` | No | Optional meeting fallback override; empty preserves OpenRouter Gemini 3.7 Flash |
| `CRM_MEETING_LLM_FALLBACK_OPENROUTER_PROVIDER` | No | Optional OpenRouter infrastructure provider for the meeting fallback |

### Agents And Automation Model

Canonical reference: `docs/agents-and-automation.md`

Use this taxonomy when working on backend agent features:

- built-in automations are product-owned backend behavior
- automation rules are user-authored trigger-to-action records
- agents are reusable executors
- `agent_run` is the durable execution primitive
- run input now carries explicit `trigger` / `target` / `event` metadata while preserving legacy fields

The backend records two agent ownership styles. They are not separate execution
paths:

- `system agents`
  - `is_system = true`
  - product-owned
  - usually preset-bound
  - differ mainly in backend-owned defaults, prompt/skill bundles, allowed tools, and product launch surfaces
- `custom agents`
  - `is_system = false`
  - workspace-managed, versioned generic executors
  - use the same `agent_run` executor path as system agents
  - should gather most context through tools after receiving a minimal trigger payload

Current executor model:

- `agent_run` is the durable execution primitive for system agents, custom agents, and one-shot command agents
- Agent Runtime is the only agent executor; Helpin owns workspace tenancy,
  configuration, authorization, triggers, launch surfaces, and product finalizers
- `runtime_kind` selects the backend adapter (`native_sdk`, `codex`, or `opencode` where configured), not a separate product behavior path
- planner/review/support behavior is expressed through prompt, skills, allowed tools, targets, and artifact contracts
- Helpin product and interaction tools use bare canonical names such as `update_plan` and `request_user_input`. Write-side configuration normalization accepts historical aliases; provider calls require exact canonical names

Current trigger surfaces in code:

- manual run actions
- agent `trigger_mode`
- automation-rule event triggers such as `task.state_entered` and `agent_run.approved`
- automation-rule `cron`

Billing and AI usage rules for agent launch surfaces:

- Any UI or backend route that can start an agent, run an automation-backed agent, continue/handoff a coding session, run a support agent, or invoke AI assistance must go through AI usage preflight/metering.
- Frontend launch surfaces must classify billing/usage failures with `getUpgradeRequiredReason` before showing a toast. Do not show raw messages like `AI usage exhausted` to users; open `UpgradeRequiredDialog` instead.
- When an action creates an entity and also starts an agent (`Create & run agent`, `Save & run agent`), handle `agent_run_error` separately. If it is a billing/usage error, keep the create modal open and show the upgrade dialog; otherwise show a normal "created, but agent did not start" warning/error.
- When adding a new agent launch entry point, audit the matching backend handler/service and frontend error path together. Covered surfaces should include PM task runs, PM epic planner runs, global task/epic create modals, global task/epic panels, automation agents, automation flows, support agent runs, support AI rewrite, coding-session handoffs, and any command-bar or template flow that starts an `agent_run`.

Current limitation to keep in mind:

- agent execution is generic, but some launch paths are still target-specific
- active planning/review/support instructions are selected from effective skills, tools, target, and durable run state
- generic target launching exists for direct runs and automation-rule `start_agent_run`
- automation-rule `start_agent_run` now uses the generic target contract, with event-target defaulting and explicit targets required for cron

Direction:

- keep genuine special-case orchestration only for real product exceptions like support flow
- keep automation rules as the event and cron trigger layer
- make custom agents triggerable by `manual`, automation-rule `event`, and automation-rule `cron`
- pass a minimal structured trigger payload into `agent_run.input`
- let custom agents gather additional context with tools instead of relying on bespoke backend entrypoints

### Logging

`InfoContext` and `ErrorContext` pass context to the configured handler; they do not automatically attach every request field. Add required safe identifiers explicitly or through the relevant context-aware handler.

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
slog.Info("task created", "task_id", task.ID, "workspace_id", task.WorkspaceID)
slog.Error("failed to save", "error", err, "task_id", id)

// Context-aware; attach required fields explicitly when not supplied by the handler
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
- `INFO`: Normal operations (task created, email sent, workflow started)
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

### Helpin Design System

For product UI work under `frontend/`, use the repo-scoped `$helpin-design-system` skill in `.agents/skills/helpin-design-system/`. Quiet Hairline is the visual authority; centralized Helpin components remain the behavior and accessibility authority.

### TanStack Router (File-Based)
Routes are defined in `src/routes/` and auto-generated into `routeTree.gen.ts`:
```
src/routes/
  __root.tsx                  # Global layout
  _authenticated.tsx          # Auth guard (redirects to /login)
    w/$slug.tsx               # Workspace layout (loads workspace, settings, access)
      pm/tasks/index.tsx      # /w/:slug/pm/tasks
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
- Uses the shared `@helpin-ai/support-core` client and session adapter; the browser adapter stores tokens in localStorage and sends `Authorization: Bearer`
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

### Query Builder Conventions
- For CRM, PM, or other list filtering UIs, prefer the shared query-builder abstractions instead of page-specific filter dropdowns.
- Frontend query-builder metadata must live in a separate config file per entity and feed a reusable builder component. Do not hardcode field/operator/value controls directly in page components.
- Backend query-builder behavior must live in a reusable package that wraps the ORM/query layer. Do not concatenate raw SQL from handlers or embed one-off operator logic in route files.
- Canonical operator set:
  `is`, `is_not`, `contains`, `not_contains`, `starts_with`, `ends_with`, `is_empty`, `is_not_empty`, `on`, `before`, `after`, `on_or_before`, `on_or_after`, `between`
- Operator rules:
  text fields use case-insensitive matching
  enum/member/id fields only use exact-match or empty operators
  date fields use date operators plus empty operators
  `is_empty` and `is_not_empty` render no value input
  `between` requires exactly two values
  date values should be passed in `YYYY-MM-DD` unless a route explicitly requires a timestamp
- Query-builder payloads should be structured, serializable rule groups such as `{ logic, rules[] }`. Avoid bespoke query-param names for every filterable field once a screen adopts the query builder.

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

Key stores: `authStore`, `workspaceStore`, `organizationStore`, `globalCreateStore`, `boardDisplayStore`

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
- CRM module types: `src/lib/crmTypes.ts`
- Permission types: follow the current definitions in `src/lib/types.ts` and backend `internal/authorization/permissions.go`
- Keep TypeScript response interfaces synchronized with the backend DTOs you change

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
