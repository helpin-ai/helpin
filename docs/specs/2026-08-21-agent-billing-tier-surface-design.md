# Agent billing-tier interface design


This design records the intended customer interface for model sizes. Use the
implementation notes below when maintaining tier selection or investigating
legacy records; the acceptance criteria are not a release-verification report.

## Current implementation and limits

Source-compared on 2026-09-18. These are repository configuration observations,
not verification of external model availability, vendor prices, or deployed data.

- [AgentModelTierResolver](../../server/internal/service/agent_model_tiers.go)
  implements catalog-validated tier resolution. Its policy is a Go map validated
  against the catalog, not a route chosen dynamically from any enabled catalog
  entry. Medium currently maps to OpenRouter `google/gemini-3.8-flash`, Large to
  direct OpenAI `gpt-5.6-terra`, and Flagship to direct Anthropic `claude-sonnet-5`.
  Small uses the shared fast OpenRouter model constant. The original table below
  is a dated proposal, not the current mapping.
- `ResolveCustom` returns `native_sdk` for every supported tier. It does not take
  target/capability inputs to select different adapters as the runtime policy
  proposes. Existing route derivation and new custom resolution are separate
  operations; derivation may return an empty tier for unresolved legacy data.
- The [frontend tier helper](../../frontend/src/lib/agentModelTier.ts) reads labels
  and descriptions from generated `AI_MODELS` data. Unknown/missing tier labels
  display “Managed.” It does not derive display labels from provider names.
- The [model definitions](../../server/internal/model/agent.go) contain tier fields,
  and [agent service](../../server/internal/service/agent.go) run construction
  copies the billing agent's tier. This does not by itself verify every historical
  record or every mutation/read surface against all acceptance criteria below.
- The [versioned migration](../../server/internal/dbmigrate/sql/202608210001_agent_model_tiers.sql)
  adds the four tier columns and fills empty agent/version tiers using model-name
  substring rules. Runs fall back through agent versions and owning agents.
  It does not implement the proposed priority of durable usage snapshots or
  exact provider/model/service-tier catalog resolution. Unmatched records may
  retain an empty tier. The file's existence does not prove it ran in an installation.
- Both [API startup](../../server/cmd/api/main.go) and
  [worker startup](../../server/cmd/temporal-worker/main.go) call selectable-tier
  validation, but enforce its issues only when the edition requires configured
  providers. Do not describe this as an unconditional startup rejection.

The universal provider-label hiding, snapshot immutability, public mutation
restrictions, and source-guard requirements remain acceptance criteria to check
at their respective boundaries. This review did not run application tests or
perform a browser/public-share audit, and does not claim those universal checks
passed.

## Original design

## Objective

Make Helpin’s customer-facing agent experience describe AI capacity using the billing names **Small**, **Medium**, **Large**, and **Flagship**. Remove provider names, exact model names, and execution-engine names such as Native SDK, Codex, and OpenCode from agent creation, editing, version management, agent lists, and run surfaces.

Technical provider, model, route, and runtime values remain internal execution state. This work does not remove any runtime adapter or model integration.

## Product contract

### Built-in agents

- Show one read-only billing tier using the catalog label and description.
- Do not expose provider, exact model, route, runtime, provider service tier, reasoning effort, or native tool-step settings.
- Workspace versions can still be created, duplicated, edited, and activated for prompts, skills, tools, modes, and other supported behavior.
- Workspace versions cannot change the built-in agent’s billing tier.
- A new Helpin-owned preset version may resolve to a different internal route. Existing versions retain their stored route for reproducibility.

### Custom agents

- Require the user to choose Small, Medium, Large, or Flagship during creation.
- Allow the billing tier to change when creating or editing a custom-agent version.
- Do not allow selection of a provider, exact model, route, or runtime.
- Resolve the selected tier to Helpin’s current approved internal route when saving the version.
- Store both the public billing tier and the resolved technical route on that immutable version.
- Activating a version affects only future runs. Running, paused, and completed runs keep their launch-time route.

### Runs and sessions

- Capture `model_tier` on the run when it is launched.
- Show the billing label, for example “Large model,” where a model indicator is useful.
- Remove Native SDK, Codex, OpenCode, OpenAI, Anthropic, OpenRouter, and exact model identifiers from customer-facing run and session UI.
- Keep technical route/runtime values in backend records, internal logs, telemetry, support diagnostics, and provider requests.
- Publicly shared run/session views follow the same hiding rules.

## Tier-to-route policy

The feature-route work and the agent UI must consume the same catalog-backed resolver. Initial customer-selectable defaults are:

| Billing tier | Internal default route |
|---|---|
| Small | OpenRouter DeepSeek V4 Flash |
| Medium | OpenRouter Gemini 3.7 Flash |
| Large | OpenRouter GPT-5.6 Terra |
| Flagship | OpenRouter Claude Sonnet 5 |

These are internal defaults, not customer-visible promises. Helpin may replace a tier’s default with another enabled catalog route without renaming the tier. A saved agent version and launched run retain their resolved route so later mapping changes do not rewrite history.

Built-in presets may use a different exact route within their Helpin-managed tier. The preset/version definition remains authoritative.

## Runtime policy

Hiding runtime selection does not mean forcing every agent onto one adapter.

- Existing agents and versions preserve their current runtime.
- Existing version duplication copies its internal runtime.
- New built-in agents use the runtime declared by the Helpin preset.
- New custom agents use a backend-owned runtime policy based on their target/capability contract. Repository coding/review work uses the coding-capable adapter selected by Helpin; general Helpin workspace work uses the product-tool-capable adapter selected by Helpin.
- The selected runtime is stored on the version but omitted from customer-editable requests and customer-facing presentation.
- Runtime compatibility checks move behind the backend resolver. The frontend no longer normalizes providers based on a runtime the user chose.

## Data model

Add `ModelTier`/`model_tier` to:

- `Agent`
- `WorkspaceAgentPresetVersion`
- `AgentVersion`
- `AgentRun`
- relevant agent draft, create, update, version, preset, and read DTOs

Technical `RuntimeKind`, `Provider`, `Model`, and `ExecutionConfig` remain persisted. They are execution snapshots, not the public configuration contract.

For customer-facing mutation DTOs:

- custom-agent creation/version requests accept `model_tier`;
- built-in preset-version requests do not accept a tier override;
- provider/model/runtime inputs are removed or rejected at the public service boundary;
- internal seed, migration, template, and preset paths use separate internal configuration structures where technical routing is required.

For read DTOs, expose `model_tier`. Existing technical fields may remain temporarily for backward-compatible transport during migration, but customer frontend code must stop rendering or relying on them. A later API-versioned cleanup may remove them entirely.

## Resolution and validation

Create one catalog-backed agent tier resolver with these responsibilities:

1. Validate that the requested tier is one of the enabled public billing tiers.
2. Resolve the current default exact route for a custom-agent tier.
3. Resolve the backend-owned runtime compatible with the agent’s targets and capabilities.
4. Validate built-in preset routes and derive their public tier from the pricing catalog.
5. Return a complete internal execution configuration: tier, provider, model/route, service tier, and runtime.
6. Reject configuration before persistence if no enabled route/provider/runtime can satisfy the tier.

The resolver does not use task categories to prohibit catalogued model tiers. The selected tier itself is the customer’s cost/capability choice for custom agents.

## Version semantics

- Agent versions remain the durable configuration boundary.
- Creating a custom version with a new tier resolves and stores a new technical route.
- Editing a custom version may change its tier only while that version is editable under current version rules.
- Activating a version projects its `model_tier` and technical execution fields onto the agent record.
- Launching a run copies the tier to `AgentRun` and preserves the resolved metering context already stored with the run.
- No active run changes route or tier after launch.
- Built-in workspace versions copy the preset-managed tier and cannot accept a user tier override.

## Backfill

Backfill existing records without changing execution behavior:

1. Resolve each existing provider/model/service-tier tuple through the pricing catalog.
2. Store the resolved catalog tier on agents and versions.
3. Derive missing version route values from the owning agent only when legacy data lacks a snapshot.
4. Backfill historical runs from their durable AI-usage metering snapshot where available, then from their agent version, then from the owning agent.
5. Preserve unresolved legacy routes and flag them for internal diagnostics rather than silently changing their model.

The migration must be idempotent. New writes must always populate `model_tier` before the backfill is considered complete.

## Frontend changes

### Agent fleet and details

- Replace `AgentModelPill` with a billing-tier badge driven by generated pricing labels.
- Remove provider icons, model identifiers, and runtime labels.
- Replace “Runtime” and “Model” summary fields with one “Model size” field.

### Built-in version drawer

- Remove the customer-facing Execution section.
- Show a compact read-only Model size row.
- Keep run mode, prompts, skills, tools, targets, version actions, and other nontechnical settings.

### Custom creation and version editing

- Replace execution engine, provider, model, reasoning effort, service tier, and native tool-step controls with one Model size selector.
- Populate options from the generated public pricing tiers, not hard-coded frontend strings.
- Show each tier’s public description and a concise pricing/capability explanation.
- Submit only `model_tier`; backend resolution supplies technical execution fields.

### Run surfaces

Update at minimum:

- agent fleet last-run/detail displays;
- agent run tables;
- Ask agent dock execution strips;
- coding-session header and related session surfaces;
- command-run timeline;
- automation activity views;
- public shared run views;
- token-usage labels where runtime names currently control presentation.

Runtime may still drive frontend behavior internally, such as whether a coding-session component is supported. It must not be rendered as customer-facing text.

## API compatibility

- Add `model_tier` before removing frontend dependence on technical fields.
- Continue reading legacy provider/model/runtime fields from old persisted records.
- Stop accepting those fields as authoritative in public custom-agent mutations after tier resolution is live.
- Ensure templates, automations, workspace preset versions, agent drafts, and SDK types are migrated together.
- Generated pricing data remains the canonical source for customer tier labels and descriptions.

## Error handling

- An unavailable tier mapping fails agent/version save with a stable “model size temporarily unavailable” error, not a provider/model identifier.
- Startup validation verifies every selectable tier has an enabled catalog route and configured provider.
- Built-in preset validation names internal routes only in server logs; customer errors name the built-in agent and billing tier.
- A provider outage during an agent run follows the separate agent-runtime failure policy. This design does not change a running agent’s route or add an implicit mid-run model switch.

## Testing

### Backend

- Every selectable tier resolves to an enabled catalog route.
- Built-in preset routes derive the expected read-only tier.
- Custom create/version requests resolve tier to technical configuration.
- Public mutations cannot override provider/model/runtime.
- Version activation projects tier and technical snapshot atomically.
- Run creation snapshots tier and does not change when the agent/version changes later.
- Existing direct, OpenRouter, legacy alias, and unresolved records backfill correctly.
- API/worker startup validation catches an unavailable selectable tier.

### Frontend

- Creation exposes only Model size and submits only `model_tier` for execution selection.
- Custom versions can change tier.
- Built-in versions display tier but cannot change it.
- No customer agent or run surface renders provider, exact model, Native SDK, Codex, or OpenCode.
- Generated billing labels and descriptions are used for all tier presentation.
- Existing runtime-dependent behavior still selects the correct internal component without displaying runtime names.

### Regression audit

Add a source-level guard over customer frontend files that fails on forbidden technical labels and direct rendering of `provider`, `model`, or `runtime_kind` in agent/run presentation components. Allow explicit exceptions for internal behavior modules and tests that validate transport compatibility.

## Acceptance criteria

- Users choose a billing tier, not a provider or exact model, for custom agents.
- Built-in agents show a read-only Helpin-managed tier.
- Users can create and activate custom versions with a different tier.
- Existing runs never change tier/model/runtime mid-execution.
- Provider, exact model, and runtime-engine names are absent from customer agent creation, editing, lists, details, versions, runs, sessions, and public shares.
- Billing, pricing, provider execution, and internal diagnostics retain exact route information.
- The first-class completion-routing plan and this agent-tier contract share one pricing catalog and one route-resolution policy.
