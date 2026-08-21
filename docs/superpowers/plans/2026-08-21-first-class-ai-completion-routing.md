# First-Class AI Completion Routing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans to implement this plan. Do not use subagents for this repository task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every production chat-completion call use one typed, metered, catalog-priced routing boundary, while making billing tiers—not providers, exact models, or runtime engines—the customer-facing agent configuration contract.

**Architecture:** Keep `aiusage.Catalog` as the immutable model/route/rate allowlist. Add a product-owned feature route registry that supplies defaults and fallbacks but does not restrict models by arbitrary task categories. Replace context-hidden direct `llm.Provider.ChatCompletion` usage with an `AICompletionService.Complete` boundary. Add a catalog-backed agent tier resolver that maps customer-selected Small/Medium/Large/Flagship tiers to internal provider/model/runtime snapshots. Built-ins expose a read-only tier; custom-agent versions select a tier; runs snapshot it at launch. Technical execution fields remain internal and disappear from customer-facing agent and run surfaces.

**Tech Stack:** Go 1.24, React 19, TypeScript 5.9, existing `internal/llm` provider adapters/router, embedded `internal/aiusage` catalog, PostgreSQL/GORM migrations, generated frontend pricing data, Go and Vitest tests.

---

## Scope and invariants

**Approved design:** `docs/superpowers/specs/2026-08-21-agent-billing-tier-surface-design.md`

- The pricing catalog answers only: “Is this exact route supported, and what immutable customer rate applies?”
- The feature route registry answers only: “What route should this internal feature use by default, and what fallback is safe?”
- A catalogued explicit model is not rejected because its tier differs from a task category.
- Real capability restrictions remain explicit. The Ask media operation must continue to use an approved vision route.
- Provider/model environment overrides for fixed internal jobs are removed. Internal route choices live in one reviewed registry.
- User-selected agent/support models remain valid explicit preferences when catalogued; the feature fallback protects execution when their provider is unavailable.
- No raw production service receives a metered `llm.Provider`. It receives `AICompleter`, making feature and metering identity mandatory at compile time.
- Agent Runtime remains a separate durable execution path, but its preflight uses catalog eligibility without small/large task-tier rejection.
- Built-in agents retain Helpin-managed routes and show a read-only billing tier.
- Custom-agent versions accept a billing tier and resolve it to a stored internal route/runtime snapshot.
- Provider, exact model, and runtime-engine names remain available to backend execution and diagnostics but are not customer-facing product concepts.
- Activating an agent version affects future runs only; launched runs never change tier, route, or runtime.
- Fallback attempts receive distinct reservation idempotency keys. A failed attempt is released before the next attempt is reserved.
- Workspace balance failures, cancellation, authorization failures, and invalid requests never trigger another provider attempt.
- A successful provider response is reconciled even if later product parsing rejects its content; observed provider usage must still be charged exactly once.

## Final default route matrix

| Feature | Primary | Fallback | Notes |
|---|---|---|---|
| AI routing/triage | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Structured, high volume |
| Coverage analysis | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | All analysis/refinement calls |
| CRM signal detection | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Structured |
| Support reply rewrite | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Remove direct-OpenAI dependency |
| Deal automation inference | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Structured |
| Meeting intelligence | OpenRouter DeepSeek V4 Flash | OpenRouter Gemini 3.7 Flash | Preserve specialized current fallback |
| CRM summary | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Structured |
| Task standing brief | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Structured |
| Support reply/task draft | Configured agent route | OpenRouter DeepSeek V4 Flash | Explicit model remains preferred |
| Docs translation/generation | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Explicit output limits required |
| Docs import conversion | OpenRouter GPT-5.6 Luna | OpenRouter Gemini 3.7 Flash | Needs up to 24k output; DeepSeek catalog maximum is 8,192 |
| Public help-center answer | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Add missing typed metering identity |
| Custom-agent draft | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Currently omits provider/model |
| Company/product context | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Require complete response |
| Dock chat title | OpenRouter DeepSeek V4 Flash | OpenRouter GPT-5.6 Luna | Currently omits provider/model |
| Ask media enrichment | OpenRouter Gemini 3.7 Flash | none until another vision route is catalogued | Preserve explicit capability restriction |

Agent Runtime routes remain driven by the persisted agent/preset configuration. Their exact catalog route is priced without imposing a task-tier match. This makes the current Atlas DeepSeek route valid while preserving Terra, Sonnet, and other catalogued agent choices.

### Task 1: Separate catalog pricing from product task tiers

**Files:**
- Modify: `server/internal/service/ai_usage.go`
- Modify: `server/internal/service/ai_usage_test.go`

- [ ] **Step 1: Replace the wrong-tier regression with catalog-only eligibility tests**

Add table cases proving that small, medium, large, and flagship routes resolve for any task nature when the exact route is enabled in the catalog. Retain rejection tests for unknown/disabled routes.

- [ ] **Step 2: Run the focused test and verify the old implementation fails**

Run: `cd server && go test ./internal/service -run 'TestAIUsageServiceResolvesCataloguedRouteRegardlessOfTaskNature|TestAIUsageServiceRejectsUnknownRoute' -count=1`

Expected: the cross-tier cases fail with `tasks require ...`.

- [ ] **Step 3: Remove `requiredBuiltInTier` and its call from default operation validation**

Make an empty `OperationKey` accept any exact enabled catalog route. Keep `AIUsageOperationMediaEnrichment` validation exact and fail closed for unknown operation keys.

- [ ] **Step 4: Run pricing tests**

Run: `cd server && go test ./internal/aiusage ./internal/service -run 'AIUsage|Catalog|MediaEnrichment' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/ai_usage.go server/internal/service/ai_usage_test.go
git commit -m "fix(ai): decouple catalog pricing from task tiers"
```

### Task 2: Add the feature route registry and validate the entire matrix

**Files:**
- Create: `server/internal/service/ai_completion_routes.go`
- Create: `server/internal/service/ai_completion_routes_test.go`
- Modify: `server/internal/service/ai_usage_meter.go`

- [ ] **Step 1: Write failing registry completeness tests**

Define tests asserting:

- every direct-completion billing feature has one policy;
- every fixed primary/fallback resolves through `aiusage.Catalog`;
- no primary duplicates its fallback;
- operation routes satisfy their explicit capability rule;
- configured output ceilings do not exceed the catalog route maximum;
- promotional/chargeable status continues to come from `AIUsageFeatureDefinition`.

- [ ] **Step 2: Run the registry test and verify it fails because the registry is absent**

Run: `cd server && go test ./internal/service -run TestAICompletionRouteRegistry -count=1`

Expected: FAIL to compile until the registry exists.

- [ ] **Step 3: Implement immutable route-policy types and the matrix above**

Use explicit types such as:

```go
type AICompletionRoute struct {
    Provider    string
    Model       string
    ServiceTier string
}

type AICompletionRoutePolicy struct {
    FeatureKey string
    Primary    AICompletionRoute
    Fallbacks  []AICompletionRoute
}
```

Represent “configured agent route first” as a policy flag rather than copying agent selection logic into the registry. Keep the exact media route under its operation key.

- [ ] **Step 4: Make the registry the sole home for fixed direct-completion model constants**

Move or delete scattered constants from `workspace_context.go`, `crm_meeting_processing.go`, `support_ai.go`, `support_inbox_triage.go`, `docs_import_ai.go`, and `helpcenter_ai_search.go` as those callers migrate. Do not move Agent Runtime preset defaults.

- [ ] **Step 5: Run registry and catalog tests**

Run: `cd server && go test ./internal/aiusage ./internal/service -run 'Catalog|AICompletionRouteRegistry' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/ai_completion_routes.go server/internal/service/ai_completion_routes_test.go server/internal/service/ai_usage_meter.go
git commit -m "feat(ai): add first-class completion route registry"
```

### Task 3: Make provider failures typed and fallback-safe

**Files:**
- Modify: `server/internal/llm/provider.go`
- Modify: `server/internal/llm/openai.go`
- Modify: `server/internal/llm/claude.go`
- Modify: `server/internal/llm/router.go`
- Modify: `server/internal/llm/openai_test.go`
- Modify: `server/internal/llm/claude_test.go`

- [ ] **Step 1: Write tests for retry classification**

Cover network timeout, 408, 429, 5xx, upstream 402, missing configured provider, model unavailable, ordinary 4xx, caller cancellation, and deadline expiration.

- [ ] **Step 2: Run the tests and verify string-only provider errors cannot be classified reliably**

Run: `cd server && go test ./internal/llm -run 'ProviderError|Retryable' -count=1`

Expected: FAIL.

- [ ] **Step 3: Add a typed provider error**

Preserve HTTP status and provider name and expose `Retryable() bool`. Retry only transient transport failures, 408/429/5xx, exhausted upstream provider credits, missing preferred provider, and provider-side model unavailability. Never retry caller cancellation or request-validation 4xx errors.

- [ ] **Step 4: Propagate completion status consistently**

Keep OpenAI/OpenRouter `finish_reason`; add Claude `stop_reason` to `ChatResponse.FinishReason`. This enables product callers to reject `length`/`max_tokens` instead of accepting truncated output.

- [ ] **Step 5: Run provider tests**

Run: `cd server && go test ./internal/llm -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/llm
git commit -m "feat(llm): classify provider failures for safe fallback"
```

### Task 4: Implement the typed completion boundary

**Files:**
- Create: `server/internal/service/ai_completion.go`
- Create: `server/internal/service/ai_completion_test.go`
- Modify: `server/internal/service/ai_usage_meter.go`

- [ ] **Step 1: Write failing lifecycle tests**

Test these exact behaviors:

- missing feature, idempotency key, or required workspace fails before provider execution;
- missing provider/model is supplied by the feature registry;
- an explicit preferred route is used when catalogued;
- unknown preferred route falls back only when the policy allows it;
- primary success reserves and reconciles once;
- retryable primary failure releases its reservation and succeeds on fallback;
- non-retryable primary failure does not call fallback;
- workspace balance/preflight failure does not call fallback;
- each attempt has a route-specific idempotency key;
- maximum output exceeding the resolved catalog route is rejected before execution;
- `finish_reason=length` is returned as a typed incomplete-output error when `RequireComplete` is true;
- promotional calls record usage without reserving customer balance.

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `cd server && go test ./internal/service -run TestAICompletionService -count=1`

Expected: FAIL because the service does not exist.

- [ ] **Step 3: Add `AICompleter` and the explicit request contract**

The request must carry `WorkspaceID`, `FeatureKey`, `OperationKey`, `IdempotencyKey`, `Metadata`, optional `PreferredRoute`, `RequireComplete`, and the `llm.ChatRequest`. Provider/model fields in the nested chat request must not be independently authoritative.

- [ ] **Step 4: Implement one-attempt reserve/call/reconcile semantics**

Resolve the exact route from the pricing catalog before reservation. Set the chat request provider/model from that resolved attempt. On provider failure release the reservation before considering fallback. Once the provider succeeds, reconcile actual telemetry before returning the response.

- [ ] **Step 5: Implement bounded fallback**

Try only registry-declared fallbacks, at most once per distinct route. Preserve the original error chain for logs while returning a stable product error. Do not put fallback inside `llm.Router`, because that layer cannot meter the actual route correctly.

- [ ] **Step 6: Run completion and usage tests**

Run: `cd server && go test ./internal/service -run 'AICompletionService|AIUsage' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/ai_completion.go server/internal/service/ai_completion_test.go server/internal/service/ai_usage_meter.go
git commit -m "feat(ai): centralize routed metered completions"
```

### Task 5: Use one provider router in API and Temporal worker

**Files:**
- Modify: `server/internal/llm/support_router.go`
- Modify: `server/internal/llm/openai_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`
- Modify: `server/internal/config/config.go`
- Modify: `server/.env.example`

- [ ] **Step 1: Add tests for provider inventory**

Verify the router reports whether `anthropic`, `openai`, and `openrouter` are configured and normalizes `openrouter-responses` to `openrouter`.

- [ ] **Step 2: Rename/generalize `NewSupportRouter` as the application chat router**

Keep embeddings separate. Build the same chat-provider inventory in API and worker. Remove the alternate CRM-only provider construction that defaults silently to Claude or `gpt-4o`.

- [ ] **Step 3: Construct one `AICompletionService` per process**

Inject the unified router, usage service, route registry, and provider inventory. Keep Agent Runtime credential wiring unchanged.

- [ ] **Step 4: Add startup validation**

At process startup validate all fixed routes against the embedded catalog and verify that each process has the provider required by its primary or fallback policy. Production AI deployments must fail fast with a feature-specific configuration message if no route for a wired feature is executable.

- [ ] **Step 5: Remove route-selection environment drift**

Deprecate/remove `CRM_LLM_PROVIDER`, `CRM_LLM_MODEL`, `HELPCENTER_ANSWER_PROVIDER`, `HELPCENTER_ANSWER_MODEL`, and docs-import provider/model defaults after their callers migrate. Retain API keys and base URLs. Make `server/.env.example` match code throughout the migration.

- [ ] **Step 6: Run wiring/config tests and builds**

Run: `cd server && go test ./internal/config ./internal/llm ./cmd/api ./cmd/temporal-worker -count=1`

Run: `cd server && go build ./cmd/api ./cmd/temporal-worker`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/llm server/internal/config server/cmd/api server/cmd/temporal-worker server/.env.example
git commit -m "refactor(ai): unify completion provider wiring"
```

### Task 6: Migrate every direct completion caller

**Files:**
- Modify: `server/internal/service/agent_draft.go`
- Modify: `server/internal/service/crm_deal_automation.go`
- Modify: `server/internal/service/crm_meeting_processing.go`
- Modify: `server/internal/service/crm_signal_detection.go`
- Modify: `server/internal/service/crm_summary.go`
- Modify: `server/internal/service/dock_chat_media.go`
- Modify: `server/internal/service/dock_chat_title.go`
- Modify: `server/internal/service/docs_helpcenter_translation.go`
- Modify: `server/internal/service/docs_helpcenter_translation_auto.go`
- Modify: `server/internal/service/docs_import_ai.go`
- Modify: `server/internal/service/helpcenter_ai_search.go`
- Modify: `server/internal/service/pm_task_insights.go`
- Modify: `server/internal/service/support_ai_admin.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_drafts.go`
- Modify: `server/internal/service/support_coverage_enrichment.go`
- Modify: `server/internal/service/support_inbox_triage.go`
- Modify: `server/internal/service/workspace_context.go`
- Modify: corresponding `*_test.go` files for every service above

- [ ] **Step 1: Add a production-call inventory guard**

Add an AST-based test that fails if production files under `internal/service` call `.ChatCompletion` outside the completion implementation. This prevents future unmetered/missing-route call sites.

- [ ] **Step 2: Run the guard and verify it lists every current raw caller**

Run: `cd server && go test ./internal/service -run TestProductionLLMCallsUseAICompleter -count=1`

Expected: FAIL with the current call-site list.

- [ ] **Step 3: Migrate setup and lightweight calls**

Move custom-agent draft, company/product context, and dock title to `AICompleter`. Remove provider/model constants. For company context set a 2,400-token ceiling, require a complete response, and retry once with the compact route policy rather than accepting a `length` response.

- [ ] **Step 4: Migrate support calls**

Move triage, rewrite, task draft, AI reply, public help-center answer, and coverage calls. Pass the assigned support-agent route as `PreferredRoute`; let the registry provide OpenRouter fallback. This explicitly fixes public help-center answers, which currently call the metered provider without `WithAIUsageMetering`.

- [ ] **Step 5: Migrate CRM/project calls**

Move signal detection, deal inference, CRM summary, meeting intelligence, and task standing brief. Remove service-local meeting retry after its output validator is expressed through the completion attempt callback or an equivalent typed validation hook; malformed structured primary output must be eligible for the declared meeting fallback while provider usage remains reconciled.

- [ ] **Step 6: Migrate docs calls**

Move translation, automatic translation, coverage drafts/enrichment, and import conversion. Give every request an explicit maximum output. Keep docs import on the long-output Luna/Gemini policy rather than DeepSeek.

- [ ] **Step 7: Delete `WithAIUsageMetering` from direct-completion call sites**

Retain it only if a non-completion legacy consumer genuinely remains during the migration. Once the inventory test is green, remove `MeteredLLMProvider` and exemption helpers if no production code uses them.

- [ ] **Step 8: Run all migrated service tests**

Run: `cd server && go test ./internal/service -count=1`

Expected: PASS, including the AST guard.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service
git commit -m "refactor(ai): route all direct completions through one boundary"
```

### Task 7: Add durable agent billing tiers and backfill existing execution snapshots

**Files:**
- Modify: `server/internal/model/agent.go`
- Modify: `server/internal/repository/agent.go`
- Modify: `server/internal/service/agent.go`
- Create: `server/internal/dbmigrate/sql/202608210001_agent_model_tiers.sql`
- Modify: `server/internal/dbmigrate/migrator.go`
- Modify: `server/internal/dbmigrate/migrator_test.go`
- Modify: `server/internal/service/agent_create_test.go`
- Modify: `server/internal/service/agent_runtime_projection_test.go`

- [ ] **Step 1: Write failing persistence and snapshot tests**

Assert that `Agent`, `WorkspaceAgentPresetVersion`, `AgentVersion`, and `AgentRun` persist `model_tier`; activating a version projects its tier and technical route atomically; launching a run copies the tier; later agent/version changes do not mutate the run.

- [ ] **Step 2: Run the focused tests and verify the fields are absent**

Run: `cd server && go test ./internal/service -run 'AgentModelTier|AgentRunSnapshotsModelTier|AgentVersionActivationProjectsModelTier' -count=1`

Expected: FAIL to compile until `model_tier` exists.

- [ ] **Step 3: Add `ModelTier` to durable models and DTOs**

Add it to agents, built-in workspace versions, custom versions, and runs. Keep `RuntimeKind`, `Provider`, `Model`, and `ExecutionConfig` persisted as internal execution snapshots. Add tier to read DTOs and custom mutation DTOs; built-in mutation DTOs must not accept a customer tier override.

- [ ] **Step 4: Add the idempotent database migration**

Add columns first. Backfill from exact catalog-compatible provider/model tuples in stable SQL cases, then from owning versions/agents for legacy missing snapshots. Do not rewrite provider, model, runtime, or active execution state. Leave unresolved legacy rows identifiable for internal diagnostics.

- [ ] **Step 5: Validate migration registration**

Run: `cd server && go test ./internal/dbmigrate -count=1`

Expected: PASS and the new migration is registered exactly once.

- [ ] **Step 6: Run model, repository, and service tests**

Run: `cd server && go test ./internal/model ./internal/repository ./internal/service -run 'Agent.*(Tier|Version|Run)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add server/internal/model/agent.go server/internal/repository/agent.go server/internal/service/agent.go server/internal/dbmigrate server/internal/service/*agent*_test.go
git commit -m "feat(agents): persist billing tier snapshots"
```

### Task 8: Resolve custom-agent tiers while preserving built-in and active-run behavior

**Files:**
- Create: `server/internal/service/agent_model_tiers.go`
- Create: `server/internal/service/agent_model_tiers_test.go`
- Modify: `server/internal/service/agent.go`
- Modify: `server/internal/service/agent_policy.go`
- Modify: `server/internal/service/agent_presets.go`
- Modify: `server/internal/service/ai_usage_meter.go`
- Modify: `server/internal/service/ai_usage_meter_test.go`
- Modify: `server/internal/service/agent_policy_test.go`
- Modify: `server/internal/service/agent_draft.go`
- Modify: `server/internal/handler/agent.go`
- Modify: relevant handler/service agent tests

- [ ] **Step 1: Write the tier-resolution contract tests**

Cover Small → OpenRouter DeepSeek, Medium → OpenRouter Gemini, Large → OpenRouter Terra, and Flagship → OpenRouter Sonnet. Assert every route is enabled in the pricing catalog and its provider is available in the validated inventory.

- [ ] **Step 2: Add built-in preset pricing tests**

For every built-in preset, derive its read-only tier from its persisted exact route and preflight successfully. Include Atlas/DeepSeek, Scribe/Terra, Ask/DeepSeek, Quill/DeepSeek alias, and coding/review presets.

- [ ] **Step 3: Verify the current Atlas failure before the task-tier gate is removed**

Run: `cd server && go test ./internal/service -run 'TestAgentModelTierResolver|TestBuiltInAgentPresetPricingRoutes' -count=1`

Expected before Task 1: Atlas fails because planning demands large while its product default is small.

- [ ] **Step 4: Implement the catalog-backed resolver**

Return a complete internal snapshot: public tier, provider, model/route, service tier, and backend-selected runtime. Preserve existing runtime when duplicating an existing version. Use preset runtime for built-ins. For new custom agents, select the runtime from backend-owned target/capability policy—repository coding/review uses the coding-capable adapter; general Helpin workspace work uses the product-tool-capable adapter.

- [ ] **Step 5: Make custom-agent mutations tier-authoritative**

Creation and editable custom versions accept `model_tier`. Ignore/reject customer attempts to override provider, exact model, runtime, service tier, reasoning effort, or native tool-step internals. Resolve and persist technical fields server-side. Custom-agent AI drafting returns a recommended tier, not technical routing fields.

- [ ] **Step 6: Keep built-in tiers Helpin-managed**

Built-in workspace versions copy their source preset tier and technical route. Reject any public tier override. Prompts, skills, tools, run modes, descriptions, duplication, and activation remain unchanged.

- [ ] **Step 7: Keep task nature as metadata only**

Preserve agent pricing reservations, durable metering context, checkpoints, reconciliation, and the exact launch route. Do not use task nature to authorize model tiers and do not add mid-run fallback or model switching.

- [ ] **Step 8: Run agent backend tests**

Run: `cd server && go test ./internal/service ./internal/handler -run 'Agent.*(Tier|Pricing|Route|Policy|Version)|PreflightAgentRunAIUsage|AgentRunUsage' -count=1`

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service server/internal/handler
git commit -m "feat(agents): resolve execution from billing tiers"
```

### Task 9: Replace customer-facing model and runtime controls with billing tiers

**Files:**
- Modify: `frontend/src/generated/aiPricing.ts` through the existing generator
- Modify: `frontend/src/lib/pm-types/agents.ts`
- Create: `frontend/src/lib/agentModelTier.ts`
- Create: `frontend/src/lib/__tests__/agentModelTier.test.ts`
- Modify: `frontend/src/pages/automation/Agents.tsx`
- Modify: `frontend/src/pages/automation/CustomAgentCreatePanel.tsx`
- Modify: `frontend/src/pages/automation/AutomationActivity.tsx` if it renders technical execution identity
- Modify: `frontend/src/components/pm/AgentRunTable.tsx`
- Modify: `frontend/src/components/pm/CodingSession/CodingSessionHeader.tsx`
- Modify: `frontend/src/components/command-bar/CommandRunTimeline.tsx`
- Modify: `frontend/src/pages/PublicSharedView.tsx`
- Modify: customer-visible Ask dock/session components that render technical execution identity
- Modify: corresponding frontend tests

- [ ] **Step 1: Write failing creation/editing presentation tests**

Assert custom creation and custom-version editing show one “Model size” selector with Small/Medium/Large/Flagship, submit `model_tier`, and do not render provider, exact model, execution engine, reasoning effort, provider service tier, or native tool-step controls.

- [ ] **Step 2: Write failing built-in presentation tests**

Assert built-in agents and versions display one read-only generated tier label, keep version actions and behavioral editing, and expose no tier selector or technical execution fields.

- [ ] **Step 3: Write failing run-surface tests**

Assert fleet cards, agent details, run tables, Ask dock, coding-session header, command timeline, automation activity, and public shares display the run’s billing tier where useful and never render provider/model/runtime names.

- [ ] **Step 4: Run focused frontend tests and verify current technical UI fails them**

Run: `cd frontend && npm test -- --run src/pages/automation/__tests__/Agents.test.tsx src/pages/automation/__tests__/CustomAgentCreatePanel.test.tsx src/components/pm/__tests__/AgentRunTable.test.tsx src/components/pm/CodingSession/__tests__/CodingSessionHeader.test.tsx`

Expected: FAIL because technical controls and labels are currently present.

- [ ] **Step 5: Add generated tier presentation helpers**

Build tier options, labels, and descriptions from `AI_PRICING.tiers`; do not duplicate tier copy in React components. Use “Small model,” “Medium model,” “Large model,” and “Flagship model” consistently.

- [ ] **Step 6: Simplify agent creation and version forms**

Remove runtime/provider/model controls and runtime-dependent advanced execution controls. Add the tier selector only for custom agents. Show a compact read-only tier row for built-ins. Preserve prompts, tools, skills, targets, modes, version duplication/editing/activation, and permissions.

- [ ] **Step 7: Remove technical identity from customer run surfaces**

Runtime may continue to select internal components and token-usage parsing, but it cannot be rendered as text. Replace technical labels with agent name, status, invocation mode, and billing tier as appropriate.

- [ ] **Step 8: Add a customer-surface source guard**

Fail if scoped customer presentation files render Native SDK, Codex, OpenCode, OpenAI, Anthropic, OpenRouter, technical model identifiers, or raw `runtime_kind`/`provider`/`model`. Allow internal behavior utilities and compatibility fixtures explicitly.

- [ ] **Step 9: Run frontend tests, typecheck, and build**

Run: `cd frontend && npm test -- --run`

Run: `cd frontend && npm run build`

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add frontend
git commit -m "feat(agents): expose billing tiers instead of execution internals"
```

### Task 10: Add end-to-end route, tier, and accounting contracts

**Files:**
- Modify: `server/internal/service/ai_completion_routes_test.go`
- Modify: `server/internal/service/ai_completion_test.go`
- Modify: `server/internal/service/agent_model_tiers_test.go`
- Modify: `server/internal/aiusage/catalog_test.go`
- Modify: frontend agent/run regression tests
- Modify: `.github/workflows/ci.yml` only if existing commands do not execute these packages

- [ ] **Step 1: Add the final route-and-tier matrix contract test**

For every direct primary/fallback and every selectable agent tier, assert catalog resolution, provider availability, maximum output compatibility, promotional status, and operation capability. Assert every built-in preset route resolves to one read-only tier.

- [ ] **Step 2: Add fallback accounting integration tests**

Use fake router providers plus the fake usage store to prove failed primary reservations are released, successful fallback usage records the fallback route/rates, and repeated idempotent execution never double-charges.

- [ ] **Step 3: Add version/run immutability integration tests**

Create a custom agent at Small, create and activate a Large version, and prove an already-launched Small run stays Small while the next run snapshots Large. Verify built-in version requests cannot override tier.

- [ ] **Step 4: Run formatting and static checks**

Run `gofmt -w` on changed Go files, then run: `git diff --check`

Expected: no output.

- [ ] **Step 5: Run focused backend packages without cache**

Run: `cd server && go test ./internal/aiusage ./internal/llm ./internal/service ./internal/handler ./internal/dbmigrate -count=1`

Expected: PASS.

- [ ] **Step 6: Run the complete backend suite and build**

Run: `cd server && go test ./... -count=1`

Run: `cd server && go build ./...`

Expected: PASS.

- [ ] **Step 7: Run the complete frontend suite and build**

Run: `cd frontend && npm test -- --run`

Run: `cd frontend && npm run build`

Expected: PASS.

- [ ] **Step 8: Audit raw calls and customer-facing technical labels**

Run: `rg -n '\.ChatCompletion\(' server --glob '*.go' | rg -v '_test.go'`

Expected: only provider/router internals and `ai_completion.go`.

Run the new frontend source guard.

Expected: no forbidden technical execution identity in customer agent/run surfaces.

- [ ] **Step 9: Verify repository state**

Run: `git status --short`

Expected: only intended changes.

- [ ] **Step 10: Commit final contracts**

```bash
git add server frontend .github/workflows/ci.yml
git commit -m "test(ai): enforce completion and agent tier contracts"
```

## Deployment acceptance criteria

- API and Temporal worker start with the same validated catalog and provider inventory.
- Startup names the exact feature and routes when no executable route is configured.
- Company/product context, onboarding, Settings/Knowledge, public help-center answers, support AI, CRM, docs, task insights, and meeting intelligence all execute through `AICompleter`.
- No production direct completion can compile without an explicit feature and idempotency identity.
- Any enabled catalog model can be priced regardless of small/medium/large/flagship category.
- Ask media remains pinned to an approved vision route.
- Atlas DeepSeek and every other built-in agent preset preflight successfully.
- Built-in agents expose a read-only billing tier while preserving prompt, skill, tool, mode, and version workflows.
- Custom-agent creation and versions select Small, Medium, Large, or Flagship and resolve technical execution server-side.
- Agent versions and runs durably snapshot both public tier and internal route; active runs never change after launch.
- Customer-facing agent and run surfaces do not mention providers, exact models, Native SDK, Codex, or OpenCode.
- Existing provider/model/runtime records are backfilled to tiers without changing their execution behavior.
- A retryable primary failure can use only its declared fallback and records the fallback’s actual route and rates.
- Configuration, catalog, and provider-route errors are detected by tests or startup validation rather than by customer actions.
- External provider outages, workspace balance exhaustion, invalid model output, and network cancellation remain possible runtime conditions, but they produce typed errors or intentional degradation rather than model-category/configuration surprises.
