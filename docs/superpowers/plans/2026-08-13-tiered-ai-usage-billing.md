# Tiered AI Usage Billing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace credits and fixed packs with immediately active, exact token-priced AI usage, a percentage-first customer meter, a $150 Founder soft budget, reservations, and idempotent exact Stripe overage settlement.

**Architecture:** A new `aiusage` domain owns an embedded immutable catalog and integer micro-USD calculations. Period, ledger, reservation, estimate, and settlement repositories provide transactional enforcement; services integrate direct LLM calls, remote runtimes, billing lifecycle, Stripe, APIs, and customer interfaces. One cutover migration starts every workspace fresh, activates pricing version `2026-08-13`, preserves the legacy ledger read-only, and removes credit columns.

**Tech Stack:** Go 1.24, GORM, PostgreSQL/SQLite tests, Chi, Stripe Go v86, React 19, TypeScript 5.9, TanStack Query, Vite 7, Next.js website, Vitest, Node test runner.

## Global Constraints

- Customer copy uses `AI usage`, `included AI usage`, `extra AI usage`, and model sizes Small, Medium, Large, and Flagship.
- Never call normalized value `tokens`; token fields contain actual provider tokens only.
- Store charges in signed `int64` micro-USD and use checked integer arithmetic.
- Round each complete usage event once to the nearest micro-dollar, half-up.
- Round each final Stripe settlement once to cents, half-up, and persist the adjustment.
- The launch rate card in the design is authoritative; do not mutate it from an external lookup.
- Percentage is the primary included-usage presentation; dollars render only for accrued extra usage and invoices.
- Founder has a 150,000,000 micro-USD monthly soft budget, never blocks, and never settles through Stripe.
- Existing workspaces start at zero usage with a full new allowance; legacy usage is not converted or displayed.
- Deployment activates pricing version `2026-08-13` immediately and must fail closed if catalog, migration, Stripe, or runtime-budget capability validation fails.
- Stripe calls never occur inside a database transaction or while a period row is locked.
- Prompts, customer content, credentials, and provider response bodies never enter billing metadata or alerts.

---

## File and Interface Map

### Canonical pricing and arithmetic

- Create `server/internal/aiusage/catalog.json`: sole launch catalog source.
- Create `server/internal/aiusage/catalog.go`: embedded loader, resolver, public snapshot, validation.
- Create `server/internal/aiusage/types.go`: tier, token, route, rate, tool, and charge types.
- Create `server/internal/aiusage/calculator.go`: normalization and checked integer rounding.
- Create `server/internal/aiusage/catalog_test.go` and `calculator_test.go`.
- Create `server/cmd/ai-pricing/main.go`: `validate` and `export` commands.

### Persistence and services

- Create `server/internal/model/ai_usage.go`.
- Create `server/internal/dbmigrate/sql/202608130001_tiered_ai_usage_cutover.sql`.
- Create `server/internal/repository/ai_usage.go` and `ai_usage_test.go`.
- Create `server/internal/service/ai_usage.go`, `ai_usage_estimates.go`, and tests.
- Create `server/internal/service/ai_usage_period_worker.go`,
  `ai_usage_reservation_sweeper.go`, and tests.
- Replace `server/internal/service/ai_usage_meter.go` credit adapter with route-aware reservations and reconciliation.
- Modify `server/internal/model/billing.go`, `repository/billing.go`, `service/billing.go`, `service/billing_org.go`, and their tests.

### Providers and runtime

- Modify `server/internal/llm/provider.go`, `openai.go`, `claude.go`, and tests.
- Modify `server/internal/service/agent_runtime_launch_context.go`, `agent_runtime_client.go`, `agent_runtime_projection.go`, and tests.
- Modify `server/internal/agentcontract/codex_appserver_protocol.go` and protocol tests.

### Stripe and APIs

- Create `server/internal/service/ai_usage_settlement.go` and tests.
- Modify `server/internal/billingstripe/gateway.go` and tests.
- Create `server/internal/handler/ai_usage.go` and tests.
- Modify `server/internal/handler/billing.go`, `billing_org.go`, `router/router.go`, `cmd/api/main.go`, and `cmd/temporal-worker/main.go`.

### Customer interfaces

- Generate `frontend/src/generated/aiPricing.ts` and `website/src/generated/aiPricing.ts`.
- Modify `frontend/src/lib/billingTypes.ts`, `types.ts`, services/hooks/query keys, billing components, billing settings, organization billing, agent configuration, analytics, and tests.
- Modify `website/src/app/pricing/page.tsx`, home/footer copy, and marketing tests.

---

### Task 1: Immutable Pricing Catalog and Validation

**Files:**
- Create: `server/internal/aiusage/types.go`
- Create: `server/internal/aiusage/catalog.go`
- Create: `server/internal/aiusage/catalog.json`
- Create: `server/internal/aiusage/catalog_test.go`
- Create: `server/cmd/ai-pricing/main.go`

**Interfaces:**
- Produces: `LoadCatalog() (*Catalog, error)`
- Produces: `(*Catalog).Resolve(provider, model, route, serviceTier string) (ResolvedRoute, error)`
- Produces: `(*Catalog).Validate() []ValidationIssue`
- Produces: `(*Catalog).PublicSnapshot() PublicPricing`
- Produces errors: `ErrModelUnavailable`, `ErrPricingConfigurationMissing`

- [ ] **Step 1: Write catalog loader and validation tests**

```go
func TestLaunchCatalogValidates(t *testing.T) {
    catalog, err := LoadCatalog()
    if err != nil { t.Fatalf("LoadCatalog() error = %v", err) }
    if issues := catalog.Validate(); len(issues) != 0 {
        t.Fatalf("Validate() issues = %#v", issues)
    }
}

func TestResolveCanonicalizesAliasAndRejectsUnapprovedRoute(t *testing.T) {
    catalog := mustLoadCatalog(t)
    got, err := catalog.Resolve("openrouter", "gpt-5.6-luna", "openai/gpt-5.6-luna", "standard")
    if err != nil { t.Fatalf("Resolve() error = %v", err) }
    if got.Tier != TierSmall || got.CanonicalModel != "gpt-5.6-luna" {
        t.Fatalf("Resolve() = %#v", got)
    }
    if _, err := catalog.Resolve("openrouter", "gpt-5.6-luna", "fallback/unknown", "standard"); !errors.Is(err, ErrModelUnavailable) {
        t.Fatalf("Resolve() error = %v", err)
    }
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `cd server && go test ./internal/aiusage -run 'TestLaunchCatalog|TestResolve'`

Expected: FAIL because package `internal/aiusage` does not exist.

- [ ] **Step 3: Define catalog types and authoritative JSON**

Define `Tier`, `TokenRates`, `Route`, `ToolRate`, `Catalog`, `ResolvedRoute`, and
`PublicPricing`. Store every ceiling/customer rate as integer micro-USD per million tokens.
Include pricing version `2026-08-13`, all four tier rate rows, plan allowances, classifications,
source URLs, and enabled exact routes already recognized by production. Classifications without an
exact current route use `enabled: false` and remain visible.

```go
type TokenRates struct {
    InputMicrousdPerMillion      int64 `json:"input_microusd_per_million"`
    CacheReadMicrousdPerMillion  int64 `json:"cache_read_microusd_per_million"`
    CacheWriteMicrousdPerMillion int64 `json:"cache_write_microusd_per_million"`
    OutputMicrousdPerMillion     int64 `json:"output_microusd_per_million"`
}

type ResolvedRoute struct {
    Provider, CanonicalModel, Route, ServiceTier string
    Tier Tier
    FundingMode FundingMode
    Rates TokenRates
    RateSnapshot json.RawMessage
    ContextWindow, MaximumOutput int64
}

type FundingMode string

const (
    FundingHelpinHosted FundingMode = "helpin_hosted"
    FundingCustomer     FundingMode = "customer_funded"
)

type PaidToolUsage struct { ToolKey string; Calls int64 }
type ValidationIssue struct { Code, Path, Message string }
type PublicTier struct { Key Tier; Label, Description string; Rates TokenRates }
type PublicPlan struct { Plan, BillingInterval string; AllowanceMicrousd int64; Soft bool }
type PublicModel struct { Provider, Model, Route string; Tier Tier; Enabled bool }
type PublicTool struct { Key, Label string; MicrousdPerCall int64 }
type PublicPricing struct {
    PricingVersion, EffectiveDate string
    Tiers []PublicTier
    Plans []PublicPlan
    Models []PublicModel
    Tools []PublicTool
}
```

- [ ] **Step 4: Implement embedded loading, alias resolution, and validation**

Validation must reject duplicate aliases, customer rates not equal to ceiling x 110 / 100,
enabled incomplete cache dimensions, unknown modifiers, unpriced paid tools, unsupported built-in
assignments, and enabled routes above a tier ceiling.

- [ ] **Step 5: Add the validation/export CLI and run tests**

Run: `cd server && go test ./internal/aiusage && go run ./cmd/ai-pricing validate`

Expected: PASS and `pricing 2026-08-13 valid`.

- [ ] **Step 6: Commit the catalog**

```bash
git add server/internal/aiusage server/cmd/ai-pricing
git commit -m "feat: add immutable AI pricing catalog"
```

### Task 2: Exact Token Normalization and Micro-USD Calculation

**Files:**
- Create: `server/internal/aiusage/calculator.go`
- Create: `server/internal/aiusage/calculator_test.go`

**Interfaces:**
- Consumes: `TokenRates`, `ResolvedRoute`, `FundingMode`
- Produces: `NormalizeTokens(TokenTelemetry) (NormalizedTokens, error)`
- Produces: `CalculateCharge(ChargeInput) (Charge, error)`
- Produces: `RoundMicrousdToCents(int64) (cents int64, adjustmentMicrousd int64, err error)`

- [ ] **Step 1: Write table-driven normalization and charge tests**

```go
func TestNormalizeTokensSeparatesReasoning(t *testing.T) {
    got, err := NormalizeTokens(TokenTelemetry{
        InputTokensTotal: 100, CacheReadTokens: 30, CacheWriteTokens: 10,
        CompletionTokensTotal: 50, ReasoningTokens: 20,
        CompletionIncludesReasoning: true,
    })
    if err != nil { t.Fatal(err) }
    want := NormalizedTokens{InputTokensTotal: 100, UncachedInputTokens: 60,
        CacheReadTokens: 30, CacheWriteTokens: 10, OutputTokens: 30, ReasoningTokens: 20}
    if got != want { t.Fatalf("NormalizeTokens() = %#v, want %#v", got, want) }
}

func TestCustomerFundedChargesTenPercent(t *testing.T) {
    got, err := CalculateCharge(ChargeInput{FundingMode: FundingCustomer,
        Tokens: NormalizedTokens{UncachedInputTokens: 1_000_000},
        Rates: TokenRates{InputMicrousdPerMillion: 220_000}})
    if err != nil { t.Fatal(err) }
    if got.PublishedEquivalentMicrousd != 220_000 || got.FinalMicrousd != 22_000 {
        t.Fatalf("charge = %#v", got)
    }
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/aiusage -run 'TestNormalize|TestCustomerFunded|TestRound'`

Expected: FAIL with undefined calculation symbols.

- [ ] **Step 3: Implement checked rational accumulation and half-up rounding**

Define the calculation boundary exactly:

```go
type TokenTelemetry struct {
    InputTokensTotal, CacheReadTokens, CacheWriteTokens int64
    CompletionTokensTotal, OutputTokens, ReasoningTokens int64
    CompletionIncludesReasoning bool
}

type NormalizedTokens struct {
    InputTokensTotal, UncachedInputTokens, CacheReadTokens, CacheWriteTokens int64
    OutputTokens, ReasoningTokens int64
}

type ChargeInput struct {
    FundingMode FundingMode
    Tokens NormalizedTokens
    Rates TokenRates
    PaidToolMicrousd int64
}

type Charge struct {
    PublishedEquivalentMicrousd, HostedMicrousd int64
    OrchestrationMicrousd, FinalMicrousd int64
}
```

Use `math/bits.Mul64` or explicit overflow bounds before multiplication. Sum numerator values in
micro-USD-token units, add paid-tool micro-USD x 1,000,000, and divide once by 1,000,000 using
half-up rounding. Reject negative telemetry and component counts exceeding `math.MaxInt64` bounds.

- [ ] **Step 4: Add all-tier, tool, rounding, overflow, and no-double-count tests**

Cover exact boundary values: 4,999/5,000 micro-USD cent rounding; zero; negative rejection;
Small/Medium/Large/Flagship rates; hosted; customer-funded; and completion totals smaller than
reasoning totals.

- [ ] **Step 5: Run and commit**

Run: `cd server && go test ./internal/aiusage`

```bash
git add server/internal/aiusage/calculator.go server/internal/aiusage/calculator_test.go
git commit -m "feat: calculate exact token-priced AI usage"
```

### Task 3: Atomic Cutover Schema and Models

**Files:**
- Create: `server/internal/model/ai_usage.go`
- Create: `server/internal/dbmigrate/sql/202608130001_tiered_ai_usage_cutover.sql`
- Modify: `server/internal/model/billing.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/dbmigrate/migrator_test.go`

**Interfaces:**
- Produces models: `AIUsagePeriod`, `AIUsageLedgerEntry`, `AIUsageReservation`,
  `AIUsageSettlement`, `AIUsageTaskEstimate`, `AIUsagePricingState`
- Produces constants for statuses, entry kinds, measurement, funding, and enforcement modes.

- [ ] **Step 1: Add migration-content tests**

Assert the migration creates every table/check/index, initializes pricing state, preserves the
extra-usage boolean, opens zero-used periods, renames the legacy ledger, installs immutable
update/delete triggers, and drops credit columns.

- [ ] **Step 2: Run migration tests and verify failure**

Run: `cd server && go test ./internal/dbmigrate -run TieredAIUsage`

Expected: FAIL because migration `202608130001` is absent.

- [ ] **Step 3: Define focused GORM models**

```go
type AIUsagePeriod struct {
    ID, WorkspaceID string
    PeriodStart, PeriodEnd time.Time
    AllowanceMicrousd, UsedMicrousd, OverageMicrousd, ReservedMicrousd int64
    EnforcementMode, Status, PricingVersion string
    CreatedAt, UpdatedAt time.Time
}

func (AIUsagePeriod) TableName() string { return "billing_ai_usage_periods" }
```

Use `JSONBlob` for immutable rate/tool/metadata snapshots and explicit `TableName` methods.

- [ ] **Step 4: Write the idempotent PostgreSQL cutover migration**

Use `CREATE TABLE IF NOT EXISTS`, guarded column rename, partial unique open-period index, positive
value checks, enum-like checks, and one immutable-trigger function. Populate allowances by plan,
interval, and trial status. Monthly paid workspaces end at the existing renewal; annual and Founder
periods use the next monthly anniversary anchored to their billing start, clamped to month end.
Set every new period's used/overage/reserved values to zero.

- [ ] **Step 5: Replace AutoMigrate legacy models with new models**

Remove `BillingCreditLedgerEntry` from `AutoMigrate`; register the five new domain models. Rename
`WorkspaceBilling.OnDemandEnabled` to `ExtraAIUsageEnabled` and remove credit fields.

- [ ] **Step 6: Validate and commit**

Run: `cd server && go test ./internal/dbmigrate && go run ./cmd/migrate pending`

```bash
git add server/internal/model server/internal/dbmigrate/sql/202608130001_tiered_ai_usage_cutover.sql server/cmd/api/main.go
git commit -m "feat: add atomic AI usage billing cutover"
```

### Task 4: Transactional Period, Reservation, and Ledger Repository

**Files:**
- Create: `server/internal/repository/ai_usage.go`
- Create: `server/internal/repository/ai_usage_test.go`

**Interfaces:**
- Produces: `NewAIUsageRepository(db *gorm.DB) *AIUsageRepository`
- Produces: `Reserve(ctx, ReservationRequest) (*model.AIUsageReservation, error)`
- Produces: `ResizeReservation(ctx, id string, targetMicrousd int64, heartbeat time.Time) error`
- Produces: `Reconcile(ctx, ReconcileRequest) (*model.AIUsagePeriod, error)`
- Produces: `Release(ctx, id, reason string) error`
- Produces: `ClosePeriod(ctx, workspaceID string, at time.Time) (*model.AIUsageSettlement, error)`
- Produces: `OpenNextPeriod(ctx, PeriodSchedule) (*model.AIUsagePeriod, error)`
- Produces: `ListDuePeriods(ctx, now time.Time, limit int) ([]model.AIUsagePeriod, error)`
- Produces: `ListStaleReservations(ctx, before time.Time, limit int) ([]model.AIUsageReservation, error)`

- [ ] **Step 1: Write SQLite repository tests for idempotency and limits**

Use these repository request types:

```go
type ReservationRequest struct {
    WorkspaceID, PeriodID, TaskNature, Tier, ExecutionID, IdempotencyKey string
    ReservedMicrousd int64
    ExpiresAt, HeartbeatAt time.Time
}

type ReconcileRequest struct {
    ReservationID string
    Entry model.AIUsageLedgerEntry
    ChargedMicrousd, AbsorbedMicrousd int64
}

type PeriodSchedule struct {
    WorkspaceID, Plan, BillingInterval, PricingVersion, EnforcementMode string
    Anchor, Start, End time.Time
    AllowanceMicrousd int64
}
```

```go
func TestReserveCountsActiveReservations(t *testing.T) {
    repo := setupAIUsageRepository(t, 1_000_000)
    _, err := repo.Reserve(context.Background(), ReservationRequest{
        WorkspaceID: "ws", IdempotencyKey: "run:1", ReservedMicrousd: 600_000})
    if err != nil { t.Fatal(err) }
    _, err = repo.Reserve(context.Background(), ReservationRequest{
        WorkspaceID: "ws", IdempotencyKey: "run:2", ReservedMicrousd: 500_000})
    if !errors.Is(err, model.ErrAIAllowanceExhausted) { t.Fatalf("error = %v", err) }
}
```

Add tests for duplicate reserve/reconcile, soft Founder allowance, enabled extra usage, release,
resize rejection, concurrent attempts, and rollback after ledger failure.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/repository -run AIUsage`

- [ ] **Step 3: Implement row-locking transactions**

PostgreSQL uses `FOR UPDATE`; SQLite tests rely on transaction serialization. Availability is
`allowance - used - reserved`. Founder `soft` never rejects. `extra_allowed` may reserve beyond
allowance. Reconciliation inserts the immutable ledger, updates used/overage/reserved, and marks
the reservation reconciled in one transaction.

- [ ] **Step 4: Implement period closure without Stripe calls**

Closure snapshots exact billable overage into one pending settlement, marks the period closed, and
uses a stable unique key. Founder creates no settlement. The next period opening is a separate
transaction.

- [ ] **Step 5: Implement query primitives for safe recovery workers**

List due periods in deterministic end/ID order. List stale active reservations by heartbeat, but
do not release them in the repository query; the service sweeper must first consult the execution
repository. Add tests proving wall-clock expiry alone leaves a reservation active.

- [ ] **Step 6: Run race-relevant tests and commit**

Run: `cd server && go test ./internal/repository -run AIUsage -count=10`

```bash
git add server/internal/repository/ai_usage.go server/internal/repository/ai_usage_test.go
git commit -m "feat: transact AI usage reservations and ledger"
```

### Task 5: AI Usage Service, Typed Errors, and Reservation Bounds

**Files:**
- Create: `server/internal/service/ai_usage.go`
- Create: `server/internal/service/ai_usage_test.go`
- Modify: `server/internal/model/billing_errors.go`
- Replace: `server/internal/service/ai_usage_meter.go`
- Replace: `server/internal/service/ai_usage_meter_test.go`

**Interfaces:**
- Produces: `ResolveMeteringContext(MeteringRequest) (MeteringContext, error)`
- Produces: `Preflight(ctx, PreflightRequest) (*model.AIUsageReservation, error)`
- Produces: `Reconcile(ctx, CompletionUsage) (*UsageResult, error)`
- Produces: `Fail(ctx, reservationID string) error`
- Produces context helpers: `WithAIUsageMetering`, `AIUsageMeteringFromContext`

- [ ] **Step 1: Write service tests for route resolution and bounds**

Define the service boundary before writing fakes:

```go
type MeteringRequest struct {
    WorkspaceID, TaskNature, FeatureKey, Provider, Model, Route, ServiceTier string
    FundingMode aiusage.FundingMode
    InputTokensEstimate, MaximumOutputTokens int64
    AllowedPaidTools []string
    ExecutionID, IdempotencyKey string
    Promotional bool
}

type MeteringContext struct {
    Route aiusage.ResolvedRoute
    ReservationID, PricingVersion string
    MaxBillableMicrousd int64
    Promotional bool
}

type PreflightRequest struct { Metering MeteringRequest }
type CompletionUsage struct {
    Context MeteringContext
    Telemetry aiusage.TokenTelemetry
    PaidTools []aiusage.PaidToolUsage
    MeasurementStatus string
}
type UsageResult struct { ChargedMicrousd, AbsorbedMicrousd int64 }
```

Test that Large planning uses the catalog Large route, Small support work uses Small, unsupported
custom models return `ErrModelUnavailableUnderPricing`, paid tools enlarge bounds, promotional
features produce telemetry without a charged reservation, and Founder never blocks.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/service -run 'AIUsageService|AIUsageMeter'`

- [ ] **Step 3: Define typed customer-safe errors**

```go
var (
    ErrAIAllowanceExhausted = errors.New("AI allowance exhausted")
    ErrExtraAIUsageDisabled = errors.New("extra AI usage disabled")
    ErrExtraAIUsageUnavailable = errors.New("extra AI usage unavailable")
    ErrModelUnavailableUnderPricing = errors.New("model unavailable under current pricing")
    ErrPricingConfigurationMissing = errors.New("pricing configuration missing")
    ErrExecutionStoppedBeforeAllowance = errors.New("execution stopped before exceeding allowance")
    ErrUsageSettlementFailed = errors.New("usage settlement pending or payment failed")
)
```

- [ ] **Step 4: Implement deterministic bounds and feature policy**

Remove `FloorUnits` and `CalculateAIUsageUnits`. Reserve max(observed P90, deterministic token/tool
bound). Promotional setup creates a promotional ledger entry after completion. Embeddings use a
separate internal-cost recorder and do not reserve allowance.

- [ ] **Step 5: Implement direct-call reconciliation and defect cap**

When enforcement is strict, cap charge at reserved value if actual unexpectedly exceeds it and
record absorbed micro-USD plus a sanitized operator alert. Extra-enabled and Founder entries record
the full charge.

- [ ] **Step 6: Run and commit**

Run: `cd server && go test ./internal/service -run 'AIUsageService|AIUsageMeter'`

```bash
git add server/internal/service/ai_usage* server/internal/model/billing_errors.go
git commit -m "feat: replace credit meter with AI usage service"
```

### Task 6: Provider Telemetry and Exact Metering Context

**Files:**
- Modify: `server/internal/llm/provider.go`
- Modify: `server/internal/llm/openai.go`
- Modify: `server/internal/llm/openai_test.go`
- Modify: `server/internal/llm/claude.go`
- Modify: `server/internal/llm/claude_test.go`
- Modify: `server/internal/llm/router.go`

**Interfaces:**
- Consumes: `service.MeteringContext`
- Produces: `llm.TokenUsage` with total input, cache read, cache write, output, reasoning, and
  `CompletionIncludesReasoning`.

- [ ] **Step 1: Write provider decoder tests**

Add fixtures proving OpenAI completion reasoning is separated once; Anthropic cache creation maps
to cache write; cache reads propagate; absent dimensions remain unsupported rather than zero-priced;
and provider/model/route returned in response metadata match the resolved request.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/llm`

- [ ] **Step 3: Extend request/response telemetry types**

```go
type TokenUsage struct {
    InputTokensTotal, CacheReadTokens, CacheWriteTokens int64
    OutputTokens, ReasoningTokens int64
    CompletionIncludesReasoning bool
}
```

Add resolved provider/model/route/service metadata to `ChatResponse` and reject a response route
that differs from the pinned metering context.

- [ ] **Step 4: Update metered provider wrapper**

Reserve before request, release on provider failure, preserve successful customer output, and
estimate when telemetry is absent. Never replace successful content with a billing persistence
error; return content and enqueue reconciliation repair with an operator alert.

- [ ] **Step 5: Run and commit**

Run: `cd server && go test ./internal/llm ./internal/service -run 'MeteredLLM|TokenUsage'`

```bash
git add server/internal/llm server/internal/service/ai_usage_meter.go
git commit -m "feat: normalize provider token telemetry"
```

### Task 7: Observed Estimates and Missing-Telemetry Fallbacks

**Files:**
- Create: `server/internal/service/ai_usage_estimates.go`
- Create: `server/internal/service/ai_usage_estimates_test.go`
- Extend: `server/internal/repository/ai_usage.go`
- Create: `server/internal/service/ai_usage_period_worker.go`
- Create: `server/internal/service/ai_usage_period_worker_test.go`
- Create: `server/internal/service/ai_usage_reservation_sweeper.go`
- Create: `server/internal/service/ai_usage_reservation_sweeper_test.go`

**Interfaces:**
- Produces: `RecalculateTaskEstimates(ctx, now time.Time) error`
- Produces: `EstimateFor(ctx, taskNature string, tier aiusage.Tier, funding aiusage.FundingMode) (model.AIUsageTaskEstimate, error)`
- Produces: `CloseDuePeriods(ctx, now time.Time, limit int) (int, error)`
- Produces: `SweepStaleReservations(ctx, now time.Time, limit int) (int, error)`

- [ ] **Step 1: Write trimmed-mean tests**

Use 20 charges from 1 through 20 units and assert removal of the lowest/highest two, correct mean,
P50/P90, token averages, actual-only filtering, and fallback for sample size 9.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/service -run TaskEstimate`

- [ ] **Step 3: Implement daily aggregate queries and launch fallbacks**

Read only successful actual non-promotional ledger entries from the latest 30 days. Store sample
size, average six-field token breakdown, mean/P50/P90, source pricing version, and calculated time.
Define explicit fallback micro-USD for every task nature and tier in code.

- [ ] **Step 4: Add recalculation triggers and commit**

Call recalculation after pricing activation and routing changes; schedule daily recalculation in
the worker.

Before committing, implement the due-period worker so monthly, annual, trial, and Founder periods
close on their AI boundary even without a subscription webhook. It must open the next period in a
separate transaction after closure. Implement the reservation sweeper with an execution-status
interface; release only terminal or absent executions and retain active/unknown executions. Add
tests for worker retry after closure-before-open failure and for active stale reservations.

Run: `cd server && go test ./internal/service -run TaskEstimate`

```bash
git add server/internal/service/ai_usage_estimates* server/internal/repository/ai_usage.go
git commit -m "feat: derive observed AI task estimates"
```

### Task 8: Billing Lifecycle and Fresh Anniversary Periods

**Files:**
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/model/billing.go`
- Modify: `server/internal/service/billing_test_scenarios.go`

**Interfaces:**
- Consumes: `AIUsageRepository.OpenNextPeriod`, `ClosePeriod`
- Produces: `NextAIUsageBoundary(anchor, after time.Time) time.Time`
- Produces: `SetExtraAIUsageEnabled(ctx, workspaceID string, enabled bool)`

- [ ] **Step 1: Replace credit lifecycle tests with allowance-period tests**

Cover monthly/annual anchors, January 31 clamping, trial end, immediate upgrade allowance increase,
deferred downgrade, cancellation without refund, Founder $150 soft reset, and launch at zero used.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `cd server && go test ./internal/service -run 'Billing.*AIUsage|NextAIUsageBoundary'`

- [ ] **Step 3: Remove credit mutation and implement period lifecycle**

Delete `PreflightCredits`, `ConsumeCredits`, block charging, reset counters, and included-credit
helpers. Subscription renewal closes/opens usage periods independently. Upgrades update the open
allowance; downgrades store pending plan/interval only.

- [ ] **Step 4: Rename extra usage API behavior and analytics**

Preserve the migrated boolean, reject enabling without an active paid subscription, exclude
Founder, and emit `extra_ai_usage_changed` with plan/tier-safe fields.

- [ ] **Step 5: Run and commit**

Run: `cd server && go test ./internal/service -run Billing`

```bash
git add server/internal/service/billing* server/internal/repository/billing.go server/internal/model/billing.go
git commit -m "feat: align billing lifecycle with AI usage periods"
```

### Task 9: Remote Agent Runtime Budgets and Cumulative Price Slices

**Files:**
- Modify: `server/internal/service/agent_runtime_launch_context.go`
- Modify: `server/internal/service/agent_runtime_client.go`
- Modify: `server/internal/service/agent_runtime_projection.go`
- Modify: `server/internal/service/agent_runtime_projection_test.go`
- Modify: `server/internal/agentcontract/codex_appserver_protocol.go`
- Modify: `server/cmd/agent-contract-report/main.go`

**Interfaces:**
- Sends `StartRunRequest.Metadata["billing"]` containing resolved route, tier, funding mode,
  pricing version, reservation ID, and `max_billable_microusd`.
- Consumes cumulative checkpoint slices with provider/model/route/service/funding, six token counts,
  modifiers, paid tools, semantic, and sequence.
- Requires runtime capability `billing_budget_v1`.

- [ ] **Step 1: Write launch-contract and checkpoint tests**

Assert every durable run sends a positive budget for enforced plans, no hard budget for Founder,
and exact route/version metadata. Assert checkpoint replay is monotonic, resizes one reservation,
rejects changed pricing versions, and terminal events reconcile once.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/service -run 'AgentRuntime.*Usage|AgentRuntime.*Budget'`

- [ ] **Step 3: Replace flat usage payload with immutable slices**

```go
type agentRuntimeUsageSlice struct {
    Provider, Model, Route, ServiceTier, FundingMode, PricingVersion string
    InputTokensTotal, CacheReadTokens, CacheWriteTokens int64
    OutputTokens, ReasoningTokens int64
    LongContext bool
    PaidTools []aiusage.PaidToolUsage
}
```

Aggregate cumulative slices by exact price identity. Grow/shrink the reservation to the newly
priced cumulative bound. Cancel cleanly only if expansion fails on an enforced plan.

- [ ] **Step 4: Add runtime capability activation gate**

At startup, query the configured runtime capability endpoint. If any production runtime lacks
`billing_budget_v1`, return a startup error before pricing activation. Add a fixture-backed test
for supported and unsupported responses. This branch must not claim per-call enforcement from
checkpoint cancellation alone.

- [ ] **Step 5: Update Codex/native telemetry adapters**

Map cache-write and reasoning separately, preserve per-request variants, and attach paid provider
tools. Unknown paid tools return pricing-configuration failure before the next call.

- [ ] **Step 6: Run and commit**

Run: `cd server && go test ./internal/service ./internal/agentcontract -run 'Usage|Budget|Token'`

```bash
git add server/internal/service/agent_runtime* server/internal/agentcontract server/cmd/agent-contract-report
git commit -m "feat: enforce runtime AI usage budgets"
```

### Task 10: Exact Stripe Settlement Worker

**Files:**
- Create: `server/internal/service/ai_usage_settlement.go`
- Create: `server/internal/service/ai_usage_settlement_test.go`
- Modify: `server/internal/billingstripe/gateway.go`
- Create: `server/internal/billingstripe/gateway_test.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`
- Modify: `server/internal/handler/billing.go`
- Modify: `server/cmd/temporal-worker/main.go`

**Interfaces:**
- Replaces `BillCreditBlock` with `SettleAIUsage(ctx, AIUsageSettlementCharge) (StripeSettlementResult, error)`
- Produces: `ProcessPendingSettlements(ctx, limit int) (int, error)`

- [ ] **Step 1: Write gateway and worker idempotency tests**

Use this gateway boundary:

```go
type AIUsageSettlementCharge struct {
    WorkspaceID, PeriodID, PricingVersion, SettlementVersion string
    CustomerID, SubscriptionID, DraftInvoiceID, BillingInterval string
    PeriodStart, PeriodEnd time.Time
    AmountCents int64
    IdempotencyKey string
}

type StripeSettlementResult struct { InvoiceItemID, InvoiceID string }
```

Test exact cent amount, description dates, metadata, stable idempotency, monthly draft renewal item,
annual out-of-cycle invoice, zero-cent skip, retry after timeout, and duplicate webhook result.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/billingstripe ./internal/service -run 'Settlement|SettleAIUsage'`

- [ ] **Step 3: Replace Stripe credit-block gateway**

Remove `creditBlockPriceID`, pricing quantity, block metadata, and credit wording. Monthly settlement
targets the draft renewal invoice; annual settlement creates one invoice item and invoice. Use the
settlement idempotency key for every Stripe create operation with operation suffixes.

- [ ] **Step 4: Implement durable claim/process/update worker**

Claim pending settlements with leases, call Stripe outside the claim transaction, and update
complete/failed state idempotently. Persist invoice item/invoice IDs and sanitized error text.

- [ ] **Step 5: Wire worker schedule and commit**

Extend invoice webhook reconciliation to match both subscription renewal drafts and annual
out-of-cycle invoices by workspace/period/settlement metadata. A monthly renewal draft must not
auto-finalize until its pending settlement item is attached or explicitly marked zero-overage.
Add webhook tests for duplicate delivery and out-of-order item/invoice events.

Run: `cd server && go test ./internal/billingstripe ./internal/service -run Settlement`

```bash
git add server/internal/billingstripe server/internal/service/ai_usage_settlement* server/cmd/temporal-worker/main.go
git commit -m "feat: settle exact extra AI usage in Stripe"
```

### Task 11: Pricing, Summary, Usage History, and Error APIs

**Files:**
- Create: `server/internal/handler/ai_usage.go`
- Create: `server/internal/handler/ai_usage_test.go`
- Modify: `server/internal/handler/billing.go`
- Modify: `server/internal/handler/billing_org.go`
- Modify: `server/internal/handler/billing_test.go`
- Modify: `server/internal/service/billing_org.go`
- Modify: `server/internal/router/router.go`

**Interfaces:**
- Produces: `GET /api/ai-pricing`
- Produces: `GET /api/workspaces/{id}/billing/usage`
- Produces: `PUT /api/workspaces/{id}/billing/extra-usage`
- Produces summary fields named in the approved design, plus display percentages.

- [ ] **Step 1: Write handler contract tests**

Assert credit fields are absent from serialized JSON, micro-USD integer fields are exact, display
percentages use decimal numbers, Founder may exceed 100, promotional/tool/token breakdowns are
separate, and public pricing matches catalog version `2026-08-13`.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/handler -run 'AIUsage|Pricing|Billing'`

- [ ] **Step 3: Define summary/history DTOs and aggregation**

Use `int64` micro-USD fields and `float64` display percentages derived at response time. Daily
series maps feature keys to charged micro-USD and percent of allowance; details include exact token
classes and measurement counts. Remove typical minimum and feature floor cost.

- [ ] **Step 4: Route typed errors through stable reason codes**

Return safe JSON `reason` values such as `ai_allowance_exhausted` and
`model_unavailable_under_pricing`; do not expose internal errors.

- [ ] **Step 5: Run and commit**

Run: `cd server && go test ./internal/handler ./internal/service -run 'AIUsage|Pricing|BillingOrg'`

```bash
git add server/internal/handler server/internal/service/billing_org.go server/internal/router/router.go
git commit -m "feat: expose percentage-based AI usage APIs"
```

### Task 12: Curated Model Selection and Built-in Tier Policy

**Files:**
- Modify: `server/internal/service/agent_presets.go`
- Modify: `server/internal/service/agent_policy.go`
- Modify: `server/internal/service/agent.go`
- Modify: `server/internal/service/agent_policy_test.go`
- Modify: `server/internal/service/agent_create_test.go`
- Modify: `server/internal/model/agent.go`

**Interfaces:**
- Consumes: `aiusage.Catalog.Resolve`
- Produces agent DTO fields: `model_size`, `model_catalog_status`, `available_models`

- [ ] **Step 1: Write policy tests**

Assert planning/coding/review built-ins resolve to Large; other built-ins resolve to Small; custom
agents accept enabled catalog models only; aliases canonicalize; unknown legacy models remain
readable but launch returns the typed pricing error; GPT-5.5 Pro is rejected.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/service -run 'Agent.*Model|Agent.*Tier|Agent.*Pricing'`

- [ ] **Step 3: Replace free-text validation with catalog validation**

Store canonical provider/model/route on successful create/update. Do not silently rewrite an
unknown legacy model. Built-ins ignore customer model mutation and expose assigned size read-only.

- [ ] **Step 4: Run and commit**

Run: `cd server && go test ./internal/service -run 'Agent.*Model|Agent.*Tier|Agent.*Pricing'`

```bash
git add server/internal/service/agent* server/internal/model/agent.go
git commit -m "feat: curate agent models by size"
```

### Task 13: Frontend Contracts and Percentage Meter

**Files:**
- Modify: `frontend/src/lib/billingTypes.ts`
- Modify: `frontend/src/lib/types.ts`
- Modify: `frontend/src/lib/services/billingService.ts`
- Modify: `frontend/src/hooks/queries/useBilling.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/lib/upgradeRequired.ts`
- Modify: `frontend/src/lib/__tests__/upgradeRequired.test.ts`
- Modify: `frontend/src/components/billing/BillingSummaryHeader.tsx`
- Modify: `frontend/src/pages/settings/BillingSettingsPage.tsx`
- Test: existing billing component/page tests

**Interfaces:**
- Consumes backend summary and usage contracts from Task 11.
- Produces `formatAIUsagePercent(value: number): string` and percentage meter presentation.

- [ ] **Step 1: Replace frontend type fixtures and write presentation tests**

Test `72% remaining`, `<0.01%`, reserved segment, 100% included plus `$3.37 before tax`, Founder
`112% used — internal budget exceeded`, reset date, and absence of credit/unit copy.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd frontend && npm test -- --run src/components/billing src/pages/settings/__tests__`

- [ ] **Step 3: Replace DTOs and service endpoint**

Remove all credit fields and on-demand names. Use `number` for JSON int64 values only after the
backend guarantees values stay below JavaScript's safe integer bound; add a boundary assertion in
the API adapter. Rename mutation to `setExtraAIUsage`.

- [ ] **Step 4: Build the three-segment meter and accessible copy**

Render used, reserved, and remaining with textual equivalents and `aria-valuetext`. Included
allowance dollar values must not render. Exact overage and invoice amounts remain currency.

Update `getUpgradeRequiredReason` for every typed backend reason and add a repository-wide test
covering PM task/epic runs, global create/panels, automation agents/flows, support agent/rewrite,
coding handoffs, and command/template launches. Each surface must open `UpgradeRequiredDialog`
instead of sending a raw billing toast.

- [ ] **Step 5: Run and commit**

Run: `cd frontend && npm test -- --run src/components/billing src/pages/settings/__tests__`

```bash
git add frontend/src/lib frontend/src/hooks frontend/src/components/billing frontend/src/pages/settings
git commit -m "feat: show percentage-based AI usage"
```

### Task 14: Agent and Feature Consumption Drill-down

**Files:**
- Modify: `frontend/src/components/billing/UsageDetail.tsx`
- Modify: `frontend/src/components/billing/__tests__/UsageDetail.test.ts`
- Modify: `frontend/src/components/billing/__tests__/UsageDetailLayout.test.tsx`
- Create: `frontend/src/components/billing/UsageBreakdownDialog.tsx`
- Create: `frontend/src/components/billing/__tests__/UsageBreakdownDialog.test.tsx`

**Interfaces:**
- Consumes `UsageResponse` feature aggregates and detailed token rows.
- Produces aggregate table and accessible drill-down dialog.

- [ ] **Step 1: Write UI tests for aggregate and detail behavior**

Assert rows show feature/agent, percent, actions, actual/estimated badge; clicking opens tier and
six-token-class totals; promotion says not counted; paid tools and customer-funded orchestration
are separate; old typical-minimum and unit columns are absent.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd frontend && npm test -- --run UsageDetail UsageBreakdownDialog`

- [ ] **Step 3: Replace credit chart values with allowance percentages**

Daily/cumulative charts use returned charged micro-USD shares; tooltips say `% of included AI
usage`. Keep default aggregate rows and individual activity inside the dialog to avoid near-zero
noise.

- [ ] **Step 4: Run and commit**

Run: `cd frontend && npm test -- --run UsageDetail UsageBreakdownDialog`

```bash
git add frontend/src/components/billing
git commit -m "feat: break down AI usage by agent and tokens"
```

### Task 15: Catalog Model Selector

**Files:**
- Modify: `frontend/src/lib/pm-types/agents.ts`
- Modify: `frontend/src/pages/automation/Agents.tsx`
- Modify: `frontend/src/pages/automation/CustomAgentCreatePanel.tsx`
- Modify: `frontend/src/pages/automation/customAgentCreateModel.ts`
- Modify: corresponding `frontend/src/pages/automation/__tests__/*`

**Interfaces:**
- Consumes public pricing/catalog endpoint.
- Produces grouped selector and unsupported legacy state.

- [ ] **Step 1: Write selector tests**

Assert four size groups, badges and rates, disabled unsupported models, no arbitrary input, built-in
read-only size, alias-backed current values, and launch prevention until legacy models are changed.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd frontend && npm test -- --run src/pages/automation/__tests__`

- [ ] **Step 3: Replace provider/free-text controls with catalog options**

Submit canonical provider/model/route. Explain that implementations may change within a size while
capability and published price remain stable. Keep runtime/provider compatibility constraints.

- [ ] **Step 4: Run and commit**

Run: `cd frontend && npm test -- --run src/pages/automation/__tests__`

```bash
git add frontend/src/lib/pm-types/agents.ts frontend/src/pages/automation
git commit -m "feat: select curated agent models by size"
```

### Task 16: Generated Pricing Snapshots and Website Copy

**Files:**
- Generate: `frontend/src/generated/aiPricing.ts`
- Generate: `website/src/generated/aiPricing.ts`
- Modify: `website/src/app/pricing/page.tsx`
- Modify: `website/src/app/page.tsx`
- Modify: `website/src/components/Footer.tsx`
- Modify: `website/tests/marketing-journey.test.mjs`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `go run ./cmd/ai-pricing export --format typescript`
- Produces identical frontend/website snapshots stamped with catalog SHA-256.

- [ ] **Step 1: Add export golden test and marketing failure assertions**

Assert generated files match command output and website contains no `credit`, `usage unit`, pack,
or `$50 / 5,000` copy. Assert four rate tables, cache read/write explanation, typical task examples,
exact extra usage, tax statement, and customer-funded 10% explanation.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./cmd/ai-pricing && cd ../website && npm test`

- [ ] **Step 3: Implement deterministic TypeScript export and consume snapshots**

Do not duplicate numeric pricing constants in React/Next code. Plan cards say included AI usage and
use observed/fallback task examples; they do not show allowance dollars, credits, or fixed token
promises.

- [ ] **Step 4: Add CI drift command and commit**

CI regenerates to a temporary directory and diffs both committed snapshots.

Run: `cd server && go run ./cmd/ai-pricing validate && cd ../frontend && npm run build && cd ../website && npm test && npm run build`

```bash
git add server/cmd/ai-pricing frontend/src/generated website/src/generated website/src .github/workflows/ci.yml
git commit -m "feat: publish canonical AI pricing"
```

### Task 17: Organization Billing, Analytics, and Operational Reporting

**Files:**
- Modify: `server/internal/service/billing_org.go`
- Modify: `server/internal/service/billing_org_test.go`
- Modify: `server/internal/service/billing_analytics.go`
- Modify: `server/internal/service/product_analytics.go`
- Modify: `frontend/src/components/billing/WorkspaceBillingCard.tsx`
- Modify: `frontend/src/components/billing/PlanChangeModal.tsx`
- Modify: `frontend/src/lib/analytics.ts`
- Create: `server/cmd/ai-usage-report/main.go`
- Create: `server/cmd/ai-usage-report/main_test.go`

**Interfaces:**
- Produces organization percentage/charge rollups without credits.
- Produces sanitized margin report grouped by route, tier, task, plan cohort, and workspace.

- [ ] **Step 1: Write aggregation and analytics tests**

Test hosted, customer-funded orchestration, promotion, embeddings, tools, cost, covered amount,
extra invoiced, gross margin, cache ratios, estimates, missing telemetry, and reservation accuracy.
Assert new events contain micro-USD/tier fields and no credit properties.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd server && go test ./internal/service ./cmd/ai-usage-report -run 'BillingOrg|AIUsageReport|ProductAnalytics'`

- [ ] **Step 3: Implement rollups, report command, and Founder alerts**

Founder emits internal warnings once per period at 80% and 100%. Report command supports JSON and
table output and redacts customer content. Negative margins and unknown pricing return non-zero.

- [ ] **Step 4: Update organization UI and commit**

Show percentages and token/action drill-downs; render dollars only for extra invoices. Update plan
change preview to describe the next period allowance without credits.

Run: `cd server && go test ./internal/service ./cmd/ai-usage-report && cd ../frontend && npm test -- --run billing analytics`

```bash
git add server/internal/service server/cmd/ai-usage-report frontend/src/components/billing frontend/src/lib/analytics.ts
git commit -m "feat: report AI usage and margins"
```

### Task 18: Remove Legacy Credits and Activate New Wiring

**Files:**
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/config/config.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`
- Modify: `server/.env.example`
- Delete: credit-block creation script located by `rg -l STRIPE_CREDIT_BLOCK_PRICE_ID`
- Modify/Delete: every application file returned by `rg -l 'included_credits|credits_used|on_demand_blocks_invoiced|BillingCredit|usage units|AI credits' server frontend website`

**Interfaces:**
- Activates pricing version `2026-08-13` on startup only after all validation gates.
- Removes `STRIPE_CREDIT_BLOCK_PRICE_ID` and all runtime legacy ledger access.

- [ ] **Step 1: Add repository-wide terminology and wiring tests**

Create a script/test that permits credit names only inside the immutable migration and explicit
legacy audit model. Fail for application DTOs, configs, analytics, UI, website, and Stripe code.

- [ ] **Step 2: Run scan and capture expected failures**

Run:

```bash
rg -n -i 'AI credits|usage units|included_credits|credits_used|on_demand_blocks_invoiced|STRIPE_CREDIT_BLOCK' server frontend website
```

Expected: existing application matches remain.

- [ ] **Step 3: Wire repositories/services/handlers/workers explicitly**

Construct catalog, pricing validator, AI usage repository/service, estimator, settlement worker,
and handlers in API/worker DI order. Startup checks active DB pricing version, catalog validity,
runtime capability, and required Stripe settlement configuration before serving traffic.

- [ ] **Step 4: Remove legacy application code/config/tests**

Keep only the renamed legacy table and a read-only internal audit query. Delete block counters,
credit DTOs/services, feature floors, scripts, environment configuration, and new-event analytics
properties.

- [ ] **Step 5: Run scan, tests, and commit**

Run the scan again; expected matches are restricted to migration/audit compatibility files.

Run: `cd server && go test ./...`

```bash
git add -A
git commit -m "feat: activate tiered AI usage billing"
```

### Task 19: Migration, Acceptance, and Full Verification

**Files:**
- Create: `server/internal/service/ai_usage_acceptance_test.go`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/ai-usage-metering.md`

**Interfaces:**
- Verifies the complete immediately active system; produces no new product interface.

- [ ] **Step 1: Add end-to-end acceptance scenarios**

Cover Starter/Growth monthly and annual, trial, Founder, hosted, customer-funded, promotion,
embedding, paid tools, missing telemetry, extra enabled/disabled, upgrade/downgrade, period reset,
Stripe retry, unsupported models, and legacy immutability.

- [ ] **Step 2: Run migration validation and focused acceptance tests**

Run:

```bash
cd server
go run ./cmd/migrate validate
go run ./cmd/ai-pricing validate
go test ./internal/service -run AIUsageAcceptance -count=1
```

Expected: all pass against the configured test database.

- [ ] **Step 3: Run backend verification**

```bash
cd server
go test ./...
go test -race ./internal/aiusage ./internal/repository ./internal/service
go vet ./...
go build ./...
```

- [ ] **Step 4: Run frontend and website verification**

```bash
cd frontend
npm test -- --run
npm run build
cd ../website
npm test
npm run build
```

- [ ] **Step 5: Run repository safety checks**

```bash
git diff --check origin/develop...HEAD
git status --short
rg -n -i 'AI credits|usage units|included_credits|credits_used|on_demand_blocks_invoiced|STRIPE_CREDIT_BLOCK' server frontend website
```

Expected: no whitespace errors; clean worktree after the final commit; terminology matches only
explicit immutable legacy audit/migration locations.

- [ ] **Step 6: Update operating documentation and commit**

Document percentage semantics, exact internal accounting, reset anchors, Founder soft budget,
settlement recovery, catalog changes, runtime capability gate, and operator commands.

```bash
git add docs/ai-usage-metering.md .github/workflows/ci.yml server/internal/service/ai_usage_acceptance_test.go
git commit -m "test: verify tiered AI usage billing cutover"
```
