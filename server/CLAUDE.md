# Backend – Go HTTP Service

## Tech Stack

- **Go 1.24** (latest minor)
- **Router**: go-chi/chi/v5
- **ORM**: GORM (gorm.io/gorm) with PostgreSQL (Neon) / SQLite for tests
- **Logging**: log/slog (Go stdlib)
- **Auth**: golang-jwt/jwt/v5
- **Config**: Environment variables via joho/godotenv
- **Storage**: aws-sdk-go-v2 (S3 / MinIO)
- **Workflows**: go.temporal.io/sdk
- **WebSocket**: nhooyr.io/websocket
- **Password hashing**: golang.org/x/crypto/bcrypt
- **Email**: Postmark client
- **LLM**: Model-agnostic provider (Claude + OpenAI adapters)

---

## Project Structure

```
server/
├── cmd/
│   ├── api/main.go              # HTTP server entry point, DI wiring
│   └── temporal-worker/main.go  # Temporal worker entry point
├── internal/
│   ├── auth/                    # JWT token management (issue, verify, refresh)
│   ├── authorization/           # RBAC engine: permissions, roles, middleware, relations
│   ├── automationcatalog/       # Automation template catalog
│   ├── config/                  # Environment configuration loading
│   ├── crmemail/                # CRM email integration (Gmail sync)
│   ├── crmsignal/               # CRM signal detection types
│   ├── crypto/                  # AES-256-GCM encryption helpers
│   ├── email/                   # Postmark email client
│   ├── githubapp/               # GitHub OAuth + App integration
│   ├── handler/                 # HTTP request handlers (59 files)
│   ├── llm/                     # Model-agnostic LLM provider interface
│   ├── middleware/              # Auth, logging, request-scoped middleware
│   ├── model/                   # GORM struct definitions + DTOs (50+ files)
│   ├── oauth/                   # OAuth2 clients (Gmail)
│   ├── repository/              # Data access layer (86 files, GORM queries)
│   ├── router/router.go         # Chi route registration
│   ├── service/                 # Business logic layer (111 files)
│   ├── storage/s3.go            # S3/MinIO storage client
│   ├── sync/                    # External API sync clients (Gmail)
│   ├── temporalapp/             # Temporal workflow definitions + activities
│   ├── tiptap/                  # TipTap rich-text processing
│   ├── websocket/               # WebSocket hub + handler
│   └── worker/                  # Background job workers
├── migrations/                  # Sequential SQL migrations (001–032+)
├── go.mod
├── go.sum
└── .env.example
```

---

## Architecture: Handler → Service → Repository

Every feature follows strict three-layer separation:

- **Handler** (`internal/handler/`): HTTP request/response only. Decode JSON, extract URL params, call service, write response. No business logic.
- **Service** (`internal/service/`): Business logic, validation, orchestration, cross-cutting concerns. Never touches `http.Request` or `http.ResponseWriter`.
- **Repository** (`internal/repository/`): GORM database queries only. Always accepts `context.Context` as first parameter. Returns `nil` (not error) for not-found.
- **Model** (`internal/model/`): GORM structs + request/response DTOs. DTOs live alongside the model they serve.

**DI Wiring** (`cmd/api/main.go`): Config → DB → Repositories → Services → Handlers → Router. All dependencies are explicit constructor injection — no globals, no service locators.

---

## Agents And Automation Model

Canonical reference: `../docs/AGENTS_AND_AUTOMATION.md`

Use this taxonomy when changing backend agent or automation behavior:

- built-in automations are product-owned backend behavior
- automation rules are user-authored trigger-to-action records
- agents are reusable executors
- `agent_run` is the durable execution primitive
- run input now carries explicit `trigger` / `target` / `event` metadata while preserving legacy fields

The backend already behaves as two practical agent categories:

- `system agents`
  - product-owned
  - preset-bound
  - for `native_sdk`, share the same core run machinery as custom agents
  - differ mainly in preset/default ownership plus some target-aware launch and context-loading paths
- `custom agents`
  - generic executors
  - current product direction is `native_sdk` only
  - should gather most context through tools after receiving a minimal trigger payload

Current trigger surfaces in code:

- manual run actions
- agent `trigger_mode`
- agent `schedule`
- automation-rule triggers: `story.state_entered`, `agent_run.approved`, `cron`

Current limitation:

- execution is generic, but launch paths are still partially target-specific
- native planning instructions are selected from effective tools plus target
- generic target launching exists for direct runs and automation-rule `start_agent_run`
- automation-rule `start_agent_run` now uses the generic target contract, with event-target defaulting and explicit targets required for cron

Proposed direction for custom agents:

- keep genuine special-case orchestration only for real product exceptions like support flow
- keep automation rules as the event and cron trigger layer
- make custom agents triggerable by `manual`, automation-rule `event`, and automation-rule `cron`
- pass a minimal structured trigger payload into `agent_run.input`
- let custom agents gather additional context with tools instead of depending on bespoke backend entrypoints

---

## Core Rules

### Style & Structure

- Follow **Uber Go Style Guide** + **Effective Go**
- Package names: short, lowercase, domain-oriented — never `util`, `helper`, `common`, `misc`
- Every exported type, function, and constant MUST have a doc comment
- Import grouping: stdlib → blank line → third-party → blank line → internal packages. Alphabetical within groups
- Function ordering: exported first (after type definition), `NewXxx()` constructor immediately after type, utility functions at end of file
- Group related `const`, `var`, `type` declarations together
- Line length: ~99 characters soft limit
- Reduce nesting: handle errors/edge cases first, return early, keep happy path left-aligned
- Use named struct fields always — never positional initialization
- Nil is a valid slice: prefer `var t []string` over `t := []string{}` unless non-nil needed for JSON

### Error Handling

- **Wrap with context**: `fmt.Errorf("get user %q: %w", id, err)` — avoid redundant "failed to" prefixes
- **Handle errors once**: either log OR return, never both. If you log and return, every caller up the stack logs again
- **Use `errors.Is` / `errors.As`** instead of `==` or type assertions for error matching
- **Sentinel errors** for known conditions: `var ErrNotFound = errors.New("not found")`
- **Custom error types** suffix with `Error`: `ValidationError`, `ConflictError`
- **Sanitize at API boundary**: handlers translate internal errors to user-safe HTTP responses — never expose raw DB errors, file paths, or stack traces
- **Never discard errors** with `_ =` on error-returning functions — log at minimum

### Context

- `context.Context` is always the **first parameter** in all I/O, handler, service, and repository functions
- All GORM queries use `db.WithContext(ctx)`
- Use `slog.InfoContext(ctx, ...)` / `slog.ErrorContext(ctx, ...)` to carry request-scoped fields
- Always `defer cancel()` after creating cancellable contexts

### Naming

- Interfaces: end with `-er` when single-method / behavioral (`Reader`, `Storer`)
- Exported sentinel errors: `ErrBrokenLink` — unexported: `errNotFound`
- Unexported package-level globals: prefix with `_` (e.g., `_defaultPort`) — except error vars which use `err` prefix
- JSON tags: always `snake_case`
- URL params: always `snake_case`

### Interfaces

- Define interfaces where they are **consumed**, not where they are implemented
- Keep interfaces small — prefer single-method interfaces
- Verify interface compliance at compile time: `var _ http.Handler = (*Handler)(nil)`

---

## File & Function Size

- **File** (non-test): < 600 LOC soft limit, < 1000 LOC hard limit → split by responsibility (`handler.go`, `service.go`, `repository.go`, `types.go`)
- **Function/method**: < 60 logical lines ideal, < 100 warning
- **Cyclomatic complexity**: ≤ 15 per function

---

## GORM Patterns

### Model Conventions

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

- UUIDs auto-generated by PostgreSQL (`gen_random_uuid()`)
- Every model has an explicit `TableName()` method
- Optional fields use pointer types: `*string`, `*time.Time`
- Request/response DTOs live in the same model file

### Query Safety

**Safe** — parameterized:
```go
db.Where("name = ?", userInput).First(&user)
```

**VULNERABLE** — never pass user input directly to these without `?` placeholders:
- `db.Select()`, `db.Distinct()`, `db.Group()`, `db.Having()`
- `db.Order()`, `db.Table()`, `db.Pluck()`
- `db.Raw()`, `db.Exec()`

**Safe dynamic ordering**:
```go
allowedColumns := map[string]bool{"name": true, "created_at": true}
if !allowedColumns[userInput] {
    return errors.New("invalid sort column")
}
db.Order(userInput + " ASC").Find(&users)
```

### Repository Conventions

- Repository returns `nil` (not error) for not-found cases — let the service layer decide how to handle
- For rule-builder style filtering, use a shared query-builder package around GORM instead of screen-specific condition code scattered across handlers and repositories
- Keep entity field mappings in separate files from the generic query-builder implementation so future lists can reuse the same operator engine
- Canonical operators:
  `is`, `is_not`, `contains`, `not_contains`, `starts_with`, `ends_with`, `is_empty`, `is_not_empty`, `on`, `before`, `after`, `on_or_before`, `on_or_after`, `between`
- Operator conventions:
  text comparisons are case-insensitive
  enum/id/member fields allow only exact-match and empty operators
  date fields allow date comparisons and empty operators
  `between` requires exactly two values
  `is_empty` and `is_not_empty` carry no value payload
  date inputs should default to `YYYY-MM-DD`
- Always use `db.WithContext(ctx)` on every query
- Use `db.Transaction(func(tx *gorm.DB) error { ... })` for multi-step writes (auto-rollback on error)
- Prefer `Preload` over manual joins to avoid N+1 queries
- Use `Select("id", "name")` to fetch only needed fields on list endpoints
- Use `FindInBatches` for processing large datasets
- Use Scopes for reusable query conditions

### Migrations

**Two migration systems** (both active):

1. **GORM AutoMigrate** — runs on startup, handles struct-level schema creation (add tables/columns). Cannot drop columns, change types, or do data migrations.
2. **dbmigrate** (`internal/dbmigrate/`) — versioned SQL migrations for everything AutoMigrate cannot do: data migrations, table/column drops, cutover tasks, constraint changes, backfills, renames.

**dbmigrate CLI** (`cmd/migrate/`):
```bash
go run ./cmd/migrate up              # Apply all pending migrations
go run ./cmd/migrate status          # Show all migrations (applied/pending)
go run ./cmd/migrate head            # Show latest applied migration
go run ./cmd/migrate pending         # List only unapplied migrations
go run ./cmd/migrate validate        # CI check — exit 1 if issues found
go run ./cmd/migrate repair          # Fix checksums after post-apply file edits
go run ./cmd/migrate create <name>   # Scaffold new migration file
```

**Migration files**: `internal/dbmigrate/sql/YYYYMMDDNNNN_name.sql` (embedded via `//go:embed`)

**When to use which**:
| Task | System |
|------|--------|
| Add new model/table | AutoMigrate (add GORM struct) |
| Add column to existing table | AutoMigrate (add struct field) |
| Drop table or column | dbmigrate |
| Data backfill or transformation | dbmigrate |
| Add/change constraints or indexes | dbmigrate |
| Rename table or column | dbmigrate |
| Cutover / schema reconciliation | dbmigrate |

**Rules**:
- Migrations MUST be idempotent (`IF NOT EXISTS`, `DROP TABLE IF EXISTS`)
- Never edit an already-applied migration file — create a new one (or run `migrate repair` if unavoidable)
- Legacy SQL migrations in `migrations/` are reference docs only — all new migrations go in `internal/dbmigrate/sql/`
- Never use AutoMigrate to drop columns or change types in production

---

## Chi Router Patterns

### Route Organization

```go
// Global middleware: r.Use() — applied to ALL routes in group
r.Use(middleware.RequestID)
r.Use(middleware.Recoverer)

// Route-specific middleware: r.With() — applied to single route
r.With(requirePerm(authorization.PermWorkspaceUpdate)).Put("/", h.Workspace.Update)
r.With(authorization.RequireOwner(authz)).Delete("/", h.Workspace.Delete)

// Sub-router: r.Route() — shared path prefix
r.Route("/tasks", func(r chi.Router) {
    r.Get("/", h.Task.List)
    r.Post("/", h.Task.Create)
})
```

### Middleware Stack (order matters)

1. `RequestID` — assigns unique request ID
2. `RealIP` — extracts client IP
3. `RequestLogger` (slog) — logs request/response
4. `Recoverer` — catches panics, returns 500
5. `CORS` — cross-origin headers
6. `RequireAuth` — validates JWT, injects UserID into context
7. `RequireWorkspaceAccess` — resolves Actor (membership + role + teams)
8. `RequirePermission(perm)` — checks role has permission

### URL Parameters

```go
id := chi.URLParam(r, "id")
slug := chi.URLParam(r, "slug")
```

### WebSocket Caveat

WebSocket handlers bypass Chi's `Recoverer` (it strips `http.Hijacker` interface). WebSocket routes are handled separately.

---

## Logging (slog)

### Usage

```go
// Basic structured logging
slog.Info("task created", "task_id", task.ID, "workspace_id", task.WorkspaceID)
slog.Error("failed to save", "error", err, "task_id", id)

// Context-aware (carries request_id, user_id from middleware)
slog.InfoContext(ctx, "comment added", "entity_id", entityID)
slog.ErrorContext(ctx, "notification emit failed", "error", err)

// With persistent attributes (in service constructors)
logger := slog.Default().With("service", "notification")
```

### Where & What Level

| Layer | Level | What to log |
|-------|-------|-------------|
| Handler | DEBUG | Incoming request params |
| Handler | ERROR | Request failures |
| Service | INFO | Business operations (create, update, delete) |
| Service | ERROR | Business logic failures |
| Repository | WARN/ERROR | Slow queries, DB failures (not every query) |
| Middleware | INFO | Request summary (method, path, status, duration) |
| Workers | INFO | Job start/complete |
| Workers | ERROR | Job failures |
| External calls | INFO | S3, email, Temporal, GitHub API calls |
| External calls | ERROR | External API failures |

### Rules

- Always use structured key-value pairs — never `fmt.Sprintf` in log messages
- Include `workspace_id`, `user_id`, `entity_id` for traceability
- Do NOT log sensitive data (passwords, tokens, email bodies, API keys)
- Do NOT silently discard errors — log at minimum
- Use `slog.ErrorContext` instead of `log.Printf` for all new code

---

## Concurrency

- **Never fire-and-forget goroutines** — every goroutine must have a stop condition and a way to wait for it
- Use `context.WithTimeout` / `context.WithCancel` + `defer cancel()` for bounded operations
- Use `errgroup.Group` for concurrent operations with error handling and first-error cancellation
- Use `errgroup.SetLimit(n)` to bound concurrency
- Use `sync.WaitGroup` only when you don't need error propagation
- Buffered channels: size 0 or 1 only — any other size requires strong justification
- Mutex: embed in unexported struct, never expose
- **Recover panics in goroutines** — a panic in a goroutine crashes the whole process
- Check `ctx.Done()` in long-running loops via `select`

---

## RBAC Authorization

Package: `server/internal/authorization/`

- **Role hierarchy** (additive): `viewer → member → manager → admin → owner`
- **Permission constants**: `PermWorkspaceRead`, `PermPMEdit`, `PermSettingsManage`, etc. (40+)
- **Middleware chain**: `RequireAuth` → `RequireWorkspaceAccess` → `RequirePermission(perm)`
- Object-level access via `authorization_relations` table
- Route-level: use `r.With(requirePerm(...))` for permission checks

---

## Testing

### Framework & Environment

- **Test runner**: Go stdlib `testing` package
- **Database**: In-memory SQLite (`gorm.io/driver/sqlite`) for fast isolated tests
- **No external test frameworks** (no testify in current codebase) — use stdlib assertions
- Tests live alongside source: `foo_test.go` next to `foo.go`

### Test Database Setup Pattern

Every test file that needs a database follows this pattern:

```go
func setupXxxTestDB(t *testing.T) *gorm.DB {
    t.Helper()

    dbName := fmt.Sprintf("file:xxx_%d?mode=memory&cache=shared", time.Now().UnixNano())
    db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
    if err != nil {
        t.Fatalf("open sqlite db: %v", err)
    }

    if err := db.Exec(`CREATE TABLE xxx (...)`).Error; err != nil {
        t.Fatalf("create table: %v", err)
    }

    return db
}
```

Key conventions:
- Use `t.Helper()` so errors report the caller's line, not the helper's
- Unique DB name per test run using `time.Now().UnixNano()` to prevent cross-test pollution
- Use `mode=memory&cache=shared` for in-memory SQLite
- Create tables with raw SQL matching the PostgreSQL schema (adapt syntax for SQLite)
- Use `t.Fatalf` for setup failures — don't return errors from setup helpers

### Table-Driven Tests

Use the slice-of-structs pattern with `t.Run()` for all multi-scenario tests:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:  "valid input",
            input: InputType{Field: "value"},
            want:  OutputType{Result: "expected"},
        },
        {
            name:    "empty input returns error",
            input:   InputType{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Naming

- Test functions: `TestFunctionName` or `TestFunctionName_Scenario`
- Subtests: descriptive names in `t.Run("descriptive scenario", ...)`
- Setup helpers: `setupXxxTestDB` — named after the domain they set up

### What to Test

| Layer | What to test | How |
|-------|-------------|-----|
| Repository | CRUD operations, query filters, edge cases | In-memory SQLite, assert row state |
| Service | Business logic, validation, error paths | Mock repository (interface), or in-memory SQLite |
| Handler | Request parsing, response format, status codes | `httptest.NewRequest` + `httptest.NewRecorder` |
| Middleware | Context injection, auth rejection, permission checks | `httptest` with mock handlers |
| Model | JSON marshaling, validation methods | Direct struct tests |
| Worker | Job execution, error handling | In-memory SQLite + mock dependencies |

### Test Rules

1. **Cover both happy path AND all error cases** — every `if err != nil` branch should be tested
2. **Each test tests one behavior** — if a test name needs "and", split it
3. **Use `t.Helper()`** in ALL test helper functions
4. **Use `t.Fatalf`** for setup failures that make the test meaningless
5. **Use `t.Errorf`** for assertion failures where remaining checks still add value
6. **Never test implementation details** — test behavior through the public API
7. **Use interfaces for dependencies** — enables swapping real implementations for test doubles
8. **Don't use `t.Parallel()` with shared SQLite databases** — SQLite doesn't handle concurrent writes well
9. **Avoid conditional assertions** in table-driven tests — if logic varies, split into separate `TestXxx` functions
10. **Always run with `-race` flag in CI**: `go test -race ./...`

### Coverage

```bash
go test ./...                          # Run all tests
go test -race ./...                    # With race detector (CI)
go test -cover ./...                   # Coverage summary
go test -coverprofile=coverage.out ./... # Coverage profile
go tool cover -html=coverage.out       # Visual HTML report
go test -bench=. -benchmem             # Benchmarks with memory stats
```

### Integration Tests

For tests that require real PostgreSQL or external services:

```go
//go:build integration

package mypackage_test
```

Run with: `go test -tags=integration ./...`

---

## Anti-Patterns (never do)

- `init()` for non-registration logic — use explicit initialization
- `panic` in library/business code — return errors
- Global mutable variables (except `errors.New` sentinels)
- Ignore errors with `_ =` on error-returning functions
- Raw string interpolation in SQL — always use `?` placeholders
- God structs / god services with 10+ dependencies — split by responsibility
- `fmt.Sprintf` inside log messages — use structured key-value pairs
- Returning raw `err.Error()` in HTTP responses — sanitize at API boundary
- `log.Printf` / `log.Fatal` outside `main()` — use `slog` everywhere
- Fire-and-forget goroutines without stop conditions
- Embedding types in exported structs — prefer explicit delegation
- Shadowing built-in names (`error`, `string`, `len`, `make`, `close`)
- Both logging AND returning the same error — handle once

---

## Security

- **Input validation**: validate all user input at the handler layer before passing to services
- **SQL injection**: never concatenate user input into queries — always use GORM's `?` placeholders
- **Secrets**: environment variables via Doppler — never in source code
- **Passwords**: `golang.org/x/crypto/bcrypt` for hashing
- **Encryption**: AES-256-GCM via `internal/crypto/` for sensitive data at rest (OAuth tokens)
- **Error exposure**: never return internal error details, SQL queries, file paths, or dependency names in API responses
- **Logging**: never log passwords, tokens, email bodies, API keys, or encryption keys
- **CORS**: configured in router middleware — review allowed origins when adding new endpoints

---

## Performance

- **Profile first** — never optimize without `pprof` or benchmark data
- **Pre-allocate slices**: `make([]T, 0, expectedLen)` when size is known or estimable
- **Prefer `strconv`** over `fmt` for number ↔ string conversions
- **Use `strings.Builder`** for multi-step string concatenation
- **Reuse `http.Client`** — never create per-request clients
- **Connection pooling**: DB connections are pooled via GORM's underlying `sql.DB`
- **Select only needed fields** on list endpoints: `db.Select("id", "name")`

---

## Workflow

1. **Understand**: read existing code in the area you're changing
2. **Layer**: handler → service → repository → model
3. **Implement**: keep functions small and focused
4. **Test**: write table-driven tests covering happy path + error cases
5. **Validate**: `go vet ./...` and `go build ./...`
6. **Tidy**: `go mod tidy`

Before creating large files: split by responsibility — `handler.go`, `service.go`, `repository.go`, `types.go`.
