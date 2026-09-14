# AI profiles, community BYOK, and optional SaaS BYOK

Status: implementation in progress. The checkpoints below record completed work; unchecked plan sections remain outstanding.

Split delivery into a standalone catalog prerequisite, **Plan A: AI configuration and execution**, and **Plan B: commercial billing extraction**. Plan A can use an adapter to the existing commercial implementation while Plan B proceeds separately. A clean community distribution requires both plans.

## Product decisions

Named AI profiles are the common configuration mechanism. Helpin owns its connections, model selection, authorization, and fallback before launch. For Helpin runs, Agent Runtime receives the resolved model configuration and run-scoped credentials. Runtime remains independently usable with its existing provider keys and defaults; Usermaven and other apps retain that behavior unless their trusted app configuration explicitly disables it.

| Edition and route | Availability | Helpin charging |
|---|---|---|
| Community, customer credentials or local endpoint | Core functionality | No credits, subscription, or Helpin usage charges |
| SaaS, Helpin-managed credentials | Default; managed Small, Medium, Large, and Flagship profiles | Existing managed usage charges |
| SaaS, customer API key or personal ChatGPT connection | Implemented in the first release; workspace flag defaults off; a configured, versioned flat token tariff is required before activation or launch | Flat fee per million tokens, independent of provider/model prices; paid tools billed separately |

- Small/Medium/Large/Flagship are managed profile names supplied by the commercial integration, not special routing concepts in core.
- SaaS BYOK includes connection management, explicit profile selection, and one optional fallback from the start. Enforce its workspace flag in both backend APIs and UI.
- SaaS BYOK uses a **flat fee per million tokens**, with the same configured rate across BYOK providers and models. Count normalized input plus output tokens, including cached/reasoning categories once where they contribute to those totals; do not add subsets a second time. Prorate actual token usage rather than rounding usage up to whole millions. Paid tools use their existing separate tool tariffs, without an additional BYOK surcharge. Community has no Helpin model or tool charges.
- Enabling or launching SaaS BYOK requires a configured flat token rate, currency, and immutable tariff version, plus the applicable paid-tool tariff versions. Freeze these at launch. A missing flat tariff blocks admission; it must never produce an empty pricing snapshot. This rule also covers custom models with no published equivalent: no provider price record is needed. The monetary rate remains configurable; an unset rate is distinct from an explicitly configured zero. Do not inherit existing percentage-based or full-equivalent charging modes.
- Charge the route actually selected: customer credentials use the configured SaaS BYOK policy; fallback to Helpin credentials uses normal managed pricing. Disclose both outcomes before selection and persist the actual policy with the run. Community charges neither route.
- Personal connections remain owned by a user **within a workspace** in this release. Account-wide ownership migration and automatic personal-profile preferences are deferred. Manual runs use an explicit override or the agent/workspace default.
- Shared workspace connections support unattended agents. Personal ChatGPT connections are available for manual execution and authorized descendants, not unattended schedules or shared agent defaults.
- Scope covers every agent-run launch path. Other Helpin AI features retain their current routing, but receive the community metering implementation and are included in the later billing extraction.
- Add a Chat Completions-compatible runtime transport for local agent execution. This does not claim that every Helpin AI feature can run locally.

The implemented contract will supersede [AI connections](../ai-connections.md) where it specifies Helpin using runtime environment-key defaults and full-rate charging for customer credentials. Runtime default-key support for other apps remains unchanged. Preserve its personal workspace scope and restrictions on unattended personal execution. Update that document in the implementation PRs; do not treat the existing database encryption context as account-wide.

## Prerequisite PR: separate capabilities from pricing

Do this first, in isolation, preserving current SaaS routes and charges.

- Split the current `aiusage.RouteDefinition` concerns into neutral model capabilities and commercial prices. Context limits, supported controls, cache support, protocols, and authentication modes must be usable without a price record.
- Update tier resolution, selectable models, startup validation in both Helpin binaries, and frontend catalog generation. Core configuration and custom models must not depend on the generated commercial pricing catalog.
- Put shared structural validation in the SDK and have Helpin and Runtime consume it. Runtime remains authoritative for supported execution capabilities. Remove the mirrored provider/control validation rules as consumers migrate.
- Distinguish ordinary transcript continuation from lossless provider-specific reasoning/state replay. Reject incompatible profiles for agents that require the latter; do not advertise a provider as supporting it merely because another provider or execution path does. Specifically, Runtime's ChatGPT transport removes `previous_response_id` (`internal/runtime/native_chatgpt.go`), so `openai_chatgpt` must not inherit lossless response-chain replay capability from `openai`. Successful transcript/tool continuation does not establish lossless replay.
- Define which execution fields are model controls. Applying a profile updates those fields without replacing unrelated tools, approval settings, native-context configuration, or execution limits. Preserve reasoning effort when migrating an existing tier snapshot.

Acceptance: existing managed model selection and prices remain equivalent; a custom unpriced model can be represented and validated without inventing a price; provider/control validation agrees across SDK, Helpin, and Runtime.

## Plan A: AI configuration and execution

### A1. Supply a complete community usage lifecycle

Deliver A1 as its own PR series. Its first deliverable is a reviewed call-site inventory covering required context, lifecycle methods, failure behavior, API/worker wiring, and test coverage. Use that inventory to divide the adapters and consumer migrations into independently reviewable changes; do not assume the reported 77-site count is the refactor boundary.

Community needs a zero-financial implementation, not a nil billing service scattered through callers.

- Define neutral interfaces for admission and the existing usage lifecycle: preflight, checkpoints, heartbeat, suspension/resume, settlement, failure, and release. Preserve stable operation IDs and retry/idempotency behavior.
- Implement community admission without financial reservations or credit checks. Retain execution identity, raw usage, audit records, and operational safeguards. Unknown or unpriced models must work without a fabricated catalog entry.
- Wire the implementation through API and worker startup and all metered consumers, including embeddings and reranking. These consumers still require their metering context and audit information; absence of a commercial meter must not remove those requirements.
- Adapt current SaaS billing to the same contract. Keep managed pricing unchanged; supply the token-based fee policy for flagged SaaS BYOK. The neutral lifecycle carries raw model usage by token category, reported-versus-estimated attribution, and separately identified paid-tool usage with stable idempotency keys. Keep monetary calculation in the commercial adapter.
- Persist an immutable EE snapshot containing the fee basis (`flat_per_million_tokens`), configured rate per million tokens, currency, flat tariff version, token-accounting version, paid-tool tariff references/versions, and actual resolved funding route. BYOK settlement is `normalized_tokens × flat_rate / 1,000,000 + separate_tool_charges`; it does not read provider model prices. Normalize provider token categories without double counting and preserve reported-versus-estimated attribution. Use the existing precise monetary representation and settlement rounding rules. Model and tool retries cannot charge either component twice.
- Do not use existing customer-funded constants as a shortcut for free community execution: current modes include percentage or full equivalent charges.

Inventory actual call sites and required methods before changing the interface. Some gateways already permit a nil commercial meter while still requiring context; preserve that distinction during migration.

### A2. Connections, profiles, and settings

- Add **Personal settings → AI connections** with API-key and ChatGPT creation, status, reconnect/disconnect, and personal profiles. Clearly show the active workspace scope.
- Add **Workspace settings → AI** for shared connections, shared profiles, and the workspace default. Workspace permissions govern management; selecting a shared profile never exposes its secret.
- A profile owns a primary connection, provider/model, protocol where applicable, and model controls. Allow one optional fallback route stored directly. Do not resolve arbitrary chains of linked fallback profiles.
- Personal profiles may use the owner's personal connection and an authorized shared fallback. Shared profiles may reference only shared connections or permitted SaaS-managed routes. Shared agent defaults cannot reference personal profiles.
- Replace agent model-size selectors in Forge, Lens, other presets, and custom agents with a profile picker or workspace-default inheritance. Keep agent tools, approvals, and execution limits separate.
- Community supports custom model identifiers and approved local endpoints without price records. SaaS BYOK also permits models without published prices, but requires its configured, versioned flat token tariff before activation and launch. Managed SaaS routes continue to require their existing managed pricing.
- Keep connection and profile APIs workspace-authorized, with additional owner checks for personal resources. Do not add account-scoped ownership or preferred-profile APIs in this release.

### A3. Resolve every launch once

Use one Helpin resolver, including paths that bypass the ordinary agent launcher.

| Launch | Selection |
|---|---|
| Manual PM, Ask Agent, or manual automation with an override | Explicit personal or shared profile |
| Manual run without an override | Agent shared profile, then workspace default |
| Scheduled, triggered, or other unattended execution | Agent shared profile, then workspace default |
| CRM playbook launch | Reviewed, frozen runtime-agent selection, with authorized credentials resolved for that selection |
| Continuation, approval resume, or worker recovery | Original resolved execution snapshot |
| Trusted child of a manual personal run | Inherit the originating selection and owner through the authorized parent relationship |
| Other agent-created child | Resolve the child agent's shared default |

- Include CRM's separate start-request construction and metering preflight in the launch inventory. Do not overwrite its reviewed snapshot with the latest profile; a material route change requires the appropriate review again.
- Fallback is opt-in and occurs **before execution only**, from configuration/readiness checks and OAuth refresh results. Do not send paid model probes. If both eligible routes are unavailable, fail with an actionable connection error.
- Invalid selections, authorization failures, unsupported capabilities, and billing rejection do not trigger fallback. Once execution starts, provider errors follow normal retry/error handling; they never switch credentials or models.
- Persist profile ID/revision, actual connection, provider, model, controls, protocol/endpoint binding, selection source, fallback reason, funding classification, and applicable fee/pricing snapshot. Do not persist plaintext secrets in this metadata.
- Freeze selection when the launch is accepted, before it is queued. Profile edits cannot change queued, running, or resumed runs. Credential refresh can replace tokens for the same authorized connection without selecting a different route.
- Recheck personal ownership, active membership, and connection status at launch, refresh, and resume. Shared execution requires the corresponding workspace authorization. Access loss or disconnect must not silently switch a running execution to another credential.
- Show the effective route, connection scope, fallback policy, and SaaS charging policy in selection and run details. Replace “Runtime default” with “Agent default.”
- Disabling the SaaS BYOK flag blocks new customer-funded selections. Existing accepted runs retain their snapshot and refresh authorization; emergency revocation uses the explicit connection/run revocation mechanism.

### A4. SDK and app-scoped Runtime credential policy

- Add `ai_profile_id` to agent configuration and launch requests. An omitted override uses the resolver. Retain legacy connection/model overrides for one compatibility release, translate them into an explicit selection, and reject conflicting profile and legacy inputs.
- Extend SDK `RunModel` with typed model controls and the transport/endpoint configuration needed for compatible endpoints. Explicit model controls replace legacy model-control values, including when empty; unrelated agent execution fields remain intact.
- Helpin supplies a concrete model and request-only credential specification for every launch, including automation, CRM, and children. Local endpoints without authentication require an explicit supported no-auth mode, not an absent credential that invokes defaults.
- Preserve Runtime model-provider environment-key lookup and existing provider/model defaults for standalone execution and apps such as Usermaven. Add a trusted per-app policy, proposed as `require_run_model_credentials` with a backward-compatible default of `false`. Helpin enables it after its launch migration. When enabled, require an explicit model and credential specification and reject missing values before queueing, even if Runtime environment keys are present. Explicit local no-auth is permitted only for a compatible configured route.
- Enforce the strict policy in the Runtime engine admission path, `Engine.StartRun` in `internal/engine/engine.go`, using authenticated app configuration. Reject missing model/credential specifications before persisting a new queued run or dispatching execution. Cover existing-run admission (`admitExistingRun`) and idempotent retry/redispatch using the persisted selection, without requiring clients to resend secrets for an accepted run. The provider factory is a later execution guard, not the admission enforcement point; a caller-supplied run flag cannot weaken app policy.
- Persist the accepted credential source and prevent workers, summaries, children, and resumed runs from silently using Runtime defaults. Explicit run credentials must never fall back to environment credentials on failure, including for apps that retain default-key support.
- Preserve existing capability fields for global provider-key availability and add app-effective credential requirements/readiness alongside protocols, authentication modes, and controls. Helpin checks run-credential readiness; standalone and Usermaven consumers can still discover default-key availability. Do not change the meaning of existing capability fields.
- Global per-provider readiness is a startup snapshot: `cmd/agent-runtime/main.go` computes `NativeProviderCapabilities()` when constructing the API capabilities. Derive app-effective policy per request from the loaded app configuration, but do not describe global key readiness as live. Environment-key rotation requires restarting affected API/worker processes to reload keys and refresh their startup configuration; until restart, the API snapshot can be stale. This plan does not add hot reload or provider-health probing.
- Retain encrypted run credential storage, terminal cleanup, and the app refresh callback. OAuth refresh tokens stay in Helpin. Runtime service credentials and encryption keys remain required.
- Refresh must honor the persisted run/connection binding, workspace, originating owner where applicable, and revocation state. Preserve request/log redaction and never send model credentials to host tool callbacks.
- Publish a fetchable SDK release and update Helpin and Runtime together. Validate clean builds without local module replacements and verify the release can be fetched by a fresh installation.

### A5. Chat Completions-compatible local execution

Make A5 independently mergeable and the last Plan A delivery. First prove the resolver and Helpin app credential policy with existing OpenAI, Anthropic, and OpenRouter API-key routes. Community API-key BYOK can ship without A5 once its other release gates pass; local-agent support is advertised only after A5 passes.

This is a new transport: Runtime currently uses the Claude chat model and OpenAI Responses model, and does not yet depend on the Eino Chat Completions component. Include dependency selection, SDK compatibility, and adapter integration in its scope.

- Add a distinct Chat Completions transport. Existing Responses support is not sufficient for an endpoint that only implements Chat Completions.
- Cover streaming text, tool calls and results, stable call IDs, multi-turn execution, cancellation, continuation/recovery, provider errors, and usage reporting. Advertise only capabilities the transport implements.
- Permit an explicitly configured no-auth local connection. Other routes continue to require their declared authentication mode.
- Bind endpoints to administrator-approved connection configuration. A browser launch request cannot supply an arbitrary destination for a stored credential. Allow private/local HTTP only through explicit configuration; redirects must not leak authorization to another origin.
- Validate against a real compatible local server and a model that supports the required tool behavior, alongside protocol fixtures. Record the tested server/model and capability limits. A separate native Ollama protocol is not required for this release.

### A6. Migration and deployment

- Preserve existing personal connection IDs, workspace/user ownership, run references, and encryption AAD. There is no account-wide re-encryption migration in this release.
- Convert existing agent routes to profiles while preserving exact model controls and unrelated execution settings. Preserve historical agent versions, reviewed CRM snapshots, and completed-run records.
- Provide an idempotent Helpin operator bootstrap that imports explicitly supplied provider credentials into shared or managed connections as appropriate. Missing credentials leave a route visibly unconfigured; do not depend on private Doppler access or scraping process environments.
- Inventory nonterminal Helpin runs that still use Runtime environment credentials. Finish or explicitly cancel those runs under the rollout procedure before enabling Helpin's strict app policy; never silently change their identity. Other apps' runs and Runtime's default credential path remain supported.
- Update fresh-install configuration across **Helpin, agent-runtime, and agent-runtime-go**. Configure the Runtime API and every worker with the model-credential encryption key, durable state, app authorization, and callback wiring. Cover host development and compose deployment.
- Enable Helpin's strict app policy only after credential-based canaries pass. A dedicated Helpin Runtime deployment can omit provider keys; a shared deployment may retain keys needed by Usermaven or other apps. Verify strict Helpin runs cannot consume those keys. A fresh community installation must work with documented configuration and fetchable dependencies, without private prebuilt binaries.
- Track ChatGPT inference and live refresh as separate gates. The September 13 test confirmed inference and tool execution using ChatGPT credentials; see the evidence record below. Expired-token refresh, restart, and revocation still require explicit end-to-end validation before rollout. Update the stale validation statements in Helpin's `docs/ai-connections.md` and Runtime's `docs/2026-09-12-run-credentials.md`, and document the app policy in Runtime's app configuration contract. Those Runtime documents must continue to describe standalone and Usermaven default-key support.
- Keep SaaS BYOK off until connection, refresh, fallback, fee, and authorization acceptance tests pass and an explicit tariff is configured. Implement the complete path before enabling the flag.

### ChatGPT inference evidence: September 13, 2026

Recorded from the earlier test-system investigation in this working session and reconfirmed by the user on September 14:

- Runtime run: `run_9b1445790f13e52c710f2c3d`; Helpin run: `086a992f-a4ca-479b-8923-67923861fa9d`.
- Selected route: `openai_chatgpt`, model `gpt-5.6-terra`, app-owned OAuth credential. The earlier inspection confirmed that credential source rather than the API-key route.
- Observed progress: 10 successful model responses and 37 tool calls before the run stalled in `commit_and_push`. The stall was traced to a Git authentication/process-handling issue, after model inference and tool execution had succeeded. It was not a completed end-to-end run.
- This record supports ChatGPT inference and tool execution, not expired-token refresh, reconnect/revocation, or lossless provider-state replay. It records the prior investigation; it is not a new live validation or an archived raw-log artifact.

## Plan B: extract commercial billing into EE

This is a separate project, dependent on the catalog split and neutral usage lifecycle. It is not a prerequisite for developing profiles or the local transport, but it is required for the clean open-source distribution.

### B1. Verify and fix billing correctness before moving code

- Reproduce the June audit findings against current handlers. Some lifecycle handlers already use transactional event processing, while the subscription-update path still warrants checking for event insertion before the business write commits.
- Fix remaining reproducible webhook retry, duplicate-charge, and credit/overage transaction failures in focused changes before extraction. Test retries and failures at transaction boundaries.
- Preserve existing fixes: legacy fixed-block overage charging is already disabled, and actual settlement runs through its worker. Do not restore obsolete behavior while moving code.
- Remove only proven dead free-plan and deferred-plan runtime branches. Preserve historical migrations, records, and compatibility needed to read existing data.
- “Preserve commercial behavior” means intended current managed pricing and product behavior, not preservation of confirmed charging bugs.

### B2. Make migration sources extensible

- Refactor the migrator to register SQL sources before moving future migrations into EE. Preserve the existing ledger and strict checksums for historical SQL.
- Support core and optional EE sources with deterministic ordering and duplicate-version rejection. Keep historical migration files and checksums intact; new EE-specific SQL can then live with EE and register only in that edition.
- Existing EE ledger entries and tables must remain valid when opening the database with a community build. Do not delete records, rewrite applied migrations, or require EE files merely to recognize previously applied history.
- A second ledger is not required by this design. Test fresh installs, existing upgrades, and edition transitions against the selected registration scheme. Add an explicit regression test in which a community binary opens a database containing applied EE migration rows without the EE sources: startup, status, validation, and migration application must not reject those rows as unknown migrations, and core checksum validation must still detect real mismatches.

### B3. Extract implementation and commercial policy

- Put backend commercial implementation under `server/ee` and frontend commercial implementation under `frontend/src/ee`. Use an explicit Go `ee` build tag and frontend edition entrypoint; community is the default build.
- Move Stripe, subscriptions, plans, seats, payment-based workspace locks, commercial feature gates, prices, credits, reservations, settlement, invoices, billing jobs, and payment/upgrade UI into EE.
- EE also supplies managed Small/Medium/Large/Flagship profiles, SaaS-owned connections, the workspace SaaS BYOK availability policy, and its configured usage fee.
- Core retains connection/profile functionality, authorization, operational limits, raw telemetry, the complete community usage lifecycle, and neutral extension interfaces. Community has no commercial limits or payment dependency.
- Preserve existing managed billing APIs and intended charges. Freeze the resolved execution's policy and rates for settlement; profile names must never determine charges, and later fee edits must not retroactively price accepted runs.
- Build and test community with both EE source directories absent. Commercial imports, routes, navigation, assets, and jobs must be absent, rather than hidden by a runtime feature flag. No Stripe initialization or price catalog is required for community startup.

## Delivery order and acceptance

1. Merge the standalone capability/catalog split with existing SaaS behavior preserved.
2. Start A1 as a separate PR series with the call-site inventory, then the lifecycle contract, adapters, and consumer migrations. Develop connections/profiles/settings and resolver work against that contract. Deliver SDK/app-policy changes and Helpin migration using existing OpenAI, Anthropic, and OpenRouter transports before A5. Preserve Runtime defaults for other apps throughout.
3. Develop Plan B separately after its interface prerequisites: verified correctness fixes, migrator registration, then backend/frontend extraction. Full EE removal is not bundled into the first catalog PR.
4. Enable SaaS BYOK only for explicitly configured workspaces after its tests pass. Release community API-key BYOK after its agent execution and EE-free build gates pass; A5 is not a prerequisite for that release.
5. Merge and release A5 independently, last in Plan A, after streaming/tool integration and real local-server validation. Only this milestone enables the local-agent claim.

### Plan A acceptance

- Community shared API-key and personal ChatGPT routes launch without credit balances, subscriptions, or commercial price entries; apply the same gate to compatible local routes when A5 lands. Raw usage and audit context survive preflight, interruption, approval pause, heartbeat, resume, failure, and settlement without financial effects.
- Existing SaaS managed routes retain their expected charges. SaaS BYOK is blocked by backend and UI when disabled or lacking a complete flat tariff; when enabled, the configured fee is applied exactly once. Test custom models without published prices, input/output/cache/reasoning accounting, fractional-million usage, separate paid-tool charges, an explicit zero rate versus an unset rate, fallback in each funding direction, and tariff changes after acceptance. Accepted BYOK snapshots always contain the flat rate and version.
- Test personal owner/workspace isolation, shared management permissions, expired OAuth with live refresh, disconnect, membership loss, and trusted-child authorization.
- Exercise every launch path in A3, including CRM's reviewed snapshot, queued runs, children, and recovery. Inspect outgoing model authentication in test transports to prove selection; checking a stored connection ID alone is insufficient.
- Test primary/fallback availability, missing defaults, invalid capabilities, explicit overrides, legacy-input conflicts, and no switch after execution starts. Profile edits must not alter accepted runs.
- Test that profile/tier migration preserves reasoning controls, native context, approvals, and other execution settings. SDK, Helpin, and Runtime agree on supported controls and continuation requirements.
- With Runtime provider-key variables unset, run Helpin API-key and ChatGPT execution under its strict app policy; add explicit no-auth local execution at A5. Cover streaming tool calls, multiple turns, cancellation, worker restart, terminal credential cleanup, and credential redaction.
- In the same Runtime with global keys configured, test that Helpin rejects missing run model/credentials while a Usermaven-style app with the policy omitted retains existing defaults. Assert rejection directly through engine admission, before queued-run persistence, durable dispatch, or provider-factory invocation; cover idempotent redispatch against persisted credentials. Explicit credentials never fall back on expiry, rejection, revocation, summary, child launch, or resume. Test standalone execution and backward-compatible capabilities.
- Test app-effective capability policy independently of the global startup snapshot. Changing provider-key environment configuration does not refresh an already constructed snapshot; restarting with the new configuration does. Readiness metadata is not a successful authentication probe.
- Validate fresh deployment using released dependencies and documented keys/configuration across all three repositories. Record live ChatGPT inference and refresh results separately.

### Plan B acceptance

- Community builds and runs with EE directories absent, without commercial configuration, prices, routes, frontend assets, or billing jobs. Operational authorization and resource safeguards remain active.
- EE regression tests cover managed usage pricing, the configured BYOK fee, subscriptions, seats, workspace locks, reservations, and settlement. Webhook and worker retries cannot duplicate a charge or lose a business update after recording an event.
- Fresh community and EE databases, existing EE upgrades, and supported edition transitions preserve migration history, strict checksums, connection decryption, and historical billing records. A community binary recognizes previously applied EE rows without loading EE SQL and without weakening checksum validation for available core migrations.


## Implementation checkpoints

- `7c6b1c318`: separated the neutral model catalog from commercial prices; migrated tier/connection pickers and both startup paths; added independent frontend model generation. Reviewed aliases, disabled routes, unknown models, persisted pricing shape, and public export parity. Catalog/service/frontend tests, backend build, and relevant vet checks passed. Shared SDK controls and the later edition boundary remain outstanding.

### A1 call-site inventory (September 14)

The concrete lifecycle dependency is concentrated in two consumers (`AIUsageMeter` and `AICompletionService`), rather than requiring each product feature to implement billing. Preserve the existing product wrappers and replace their lifecycle dependency with one consumed interface.

| Boundary | Current paths | Contract and migration checks |
|---|---|---|
| Direct completions | `ai_completion.go`: `Complete`, execution attempt | Preflight per resolved attempt, release on failure, reconcile on success; retain governance and action audit. |
| Legacy governed chat | `ai_usage_meter.go`: `MeteredLLMProvider`, `chatCompletionTokenPriced` | Require metering context; preserve explicitly exempt calls and action audit. Route the active path through the same lifecycle. |
| Agent admission | `ai_usage_meter.go`: `PreflightAgentRunAIUsage`; `agent.go`; `crm_playbook_agent_launcher.go` | Persist immutable context before launch, including the separate CRM reviewed snapshot. Missing financial services cannot block community. |
| Interactive checkpoints | `ai_usage_meter.go`: `checkpointAgentRun`; `agent_runtime_projection.go` | Apply cumulative deltas once; persist the usage watermark with telemetry; suspend on an interaction pause. |
| Resume and heartbeat | `agent.go`; `agent_runtime_projection.go` | Restore the same context via `Heartbeat`; do not resolve a new price or connection on resume. |
| Terminal and late usage | `agent_runtime_usage_settlement.go`; `ai_usage_meter.go`: `reconcileAgentRun` | Keep late-event idempotency and watermarks; financial settlement is optional but raw usage persistence is mandatory. |
| Failure and release | `agent.go`; `crm_playbook_agent_launcher.go`; `ai_usage.go` | Release/fail must be valid with no financial reservation; preserve launch cleanup and retry behavior. |
| Embeddings | `ai_embedding_gateway.go` | Context and audit required; commercial meter already optional. Keep raw estimated tokens even when no charge is made. |
| Reranking | `support_knowledge_reranker.go` | Context, governance, and audit required; no commercial meter to remove. |
| Legacy preflight adapters | `crm_meeting.go`; internal command/support-reply dependencies | Retain wrapper signatures; community uses a real adapter rather than disabling wrappers with nil. |
| Construction | `cmd/api/main.go`; `cmd/temporal-worker/main.go` | Select an edition implementation once; wire the same lifecycle into direct completions, agent services, projections, and CRM launchers. |

The consumed lifecycle needs `ResolveMeteringContext`, `Preflight`, `Checkpoint`, `Reconcile`, `Heartbeat`, `SuspendReservation`, `Fail`, and `Release`. The community implementation must retain identity, normalized raw telemetry, and transactional run watermarks without requiring prices, reservations, periods, or credit balances. Existing agent checkpoint/terminal tests and direct completion tests provide regression coverage; add community interruption, resume, missing-price, and idempotency cases before switching wiring.

- `7d9633eee`: tier changes preserve reasoning effort, native context, and execution limits; targeted regression tests passed. A1 inventory is recorded above.
- A1 lifecycle implementation: introduced the consumed interface and a community recorder with no price/reservation dependency, atomic usage/watermark persistence, stale-event detection, and pause/restart tests. Edition startup selection remains outstanding. Reviewed retries and workspace isolation; focused service/repository tests, vet, and backend build passed.

- `19ff19d84`: workspace-owned AI connections with settings authorization, member selection, unattended access, and unchanged personal encryption AAD. Shared ChatGPT ownership is rejected.
- `79d88d85e`: versioned personal/shared profiles, workspace defaults, one direct pre-execution fallback, conflict detection, and immutable selection restoration. Custom models require no price record in this core resolver.
- Agent integration checkpoint: all ordinary launch surfaces and Temporal construction use profiles; accepted-run reuse precedes new selection, resumes recheck the stored connection, and CRM freezes its model in the reviewed setup. New CRM setup publication fingerprints include the exact shared profile selection; historical snapshots omit unset fields to retain their fingerprints. Older CRM setups require review/publication before launching through profile-enabled wiring. Tests across services, repositories, handlers, templates, and migrations passed; targeted profile/template/CRM tests and backend vet/build passed.
- Deployment remains pending: do not restart into profile-enabled wiring until the shared/managed bootstrap and edition policy wiring are ready. The new migrations have not been applied to the dev database. Helpin and Runtime commits remain local at the user's request; the user will perform migrations and service restarts when asked.

- `bd3c60798`: personal/workspace AI settings, profile editing, and shared/default selectors across agent forms and manual launch/chat surfaces. 115 focused UI tests, TypeScript checking, and production build passed. Live visual checks remain pending the coordinated dev migration/restart.
- B1 correctness checkpoint: reproduced subscription-update event consumption before a failed business write and partial writes when marking the receipt failed. Moved both into the existing lifecycle transaction, including retry of older unprocessed receipts; billing/repository regressions, vet, and backend build passed. No real Stripe calls or live billing data were changed.

- Flat-tariff accounting checkpoint: implemented explicit zero-versus-unset rates, version/currency/accounting snapshots, custom models without published prices, immutable paid-tool tariffs, and cumulative checkpoint rounding. Existing managed and historical percentage modes retain their behavior. Targeted accounting/service tests, vet, and API/worker builds passed. The new tariff is not enabled or wired to SaaS admission yet; workspace availability policy and edition construction remain outstanding.

- Admission policy checkpoint: added trusted connection funding, the core edition-policy hook, and EE workspace BYOK policy with immutable versioned flat tariffs. Policy rejection cannot trigger fallback; the actual fallback route supplies the accepted funding. CRM freezes rates at launch and retries reuse accepted policy rather than admitting or reserving again. The migrate binary now registers EE SQL only under the `ee` build tag. Full service/repository/migrator regressions, EE policy tests, focused retry regressions, vet and both migration builds passed. API/worker edition construction, UI policy disclosure, bootstrap, and live rollout are still pending; no flag or live database was changed.

- Bootstrap checkpoint: added an operator command with transactional preview/apply, explicit provider-to-environment mappings, controlled credential rotation, visibly unconfigured missing routes, and current-agent profile migration. Reruns preserve edited profiles and cleared defaults. The report inventories nonterminal runs still using runtime defaults; historical versions, reviewed setups, and run identities stay unchanged. Preview/rollback, rotation, atomic failure, controls, and idempotency tests passed, as did vet and community/EE command builds. Updated the connections contract and staged rollout commands. API/worker edition construction and live rollout are still pending; the command has not been run against dev.

- EE dependency checkpoint: moved usage lifecycle and shared DTOs out of concrete services, narrowed billing event hooks, separated Customer.io financial projections from core identity syncing, and changed completion startup validation to the neutral model catalog. Billing, usage, identity, analytics, and route regressions passed, as did vet and API/worker/bootstrap builds. Commercial implementations and edition composition remain the next extraction step; this checkpoint does not yet make the default API a community distribution.

- EE source extraction checkpoint: moved commercial services, repositories, Stripe gateway, billing/pricing handlers, catalog and monetary calculator under `server/ee`. Core registers optional edition routes at the existing auth boundaries, keeps shared usage normalization/watermarks, and has no production imports of EE packages. All 18 billing/pricing route methods and paths and the catalog bytes are unchanged. Full EE/host regression suites, pricing export tests, vet and API/worker builds passed. Construction still explicitly wires the commercial edition while the build-tag composition root is being implemented; core financial integration test helpers also still depend on EE. Default community startup, EE-absent build/test validation, frontend extraction, and live rollout are not complete.

- `71eb4fdc8`: community/EE startup is now selected explicitly at build time in both API and worker. Community installs the real zero-charge lifecycle without a price catalog, billing jobs, subscriptions, or provider-key startup requirement. EE installs billing, BYOK admission, managed workspace bootstrap, and joined/cancellable billing jobs. Both startup variants and focused policy/shutdown tests passed; staging/production container builds explicitly choose EE. Live configuration is unchanged.
- `7fb0db650`: separated commercial integration tests behind `ee`, kept core tests in community, and added the CI source-distribution check. A temporary archive without `server/ee` passed API/worker/migrator/bootstrap builds and every core/command test. The archive includes the existing shared reply-time fixture. Financial regression suites also passed with `ee`; historical migrations were not changed.

- Frontend EE extraction checkpoint: billing pages/components, hooks, prices, upgrade presentation, branding policy, and billing navigation guards now live under `frontend/src/ee`. Core uses build-selected extension points; community has no billing requests or upgrade UI, and billing URLs return not found. The model picker uses the neutral model catalog. Web and desktop type checks and both edition production builds passed. The broad EE suite exposed two outdated profile test fixtures; those were updated and all 26 affected cases passed. The community source archive built with `src/ee` absent and recorded 2,607 passing tests; module-loading/runner timeouts under competing builds were resolved by serial retries in a fresh archive (58 passing tests plus all 7 checklist cases, no errors). The CI check includes shared ordering fixtures and runs separately from EE tests. No live migration, restart command, billing flag change, or repository push occurred.

### SaaS policy operator checkpoint

Added the EE-only `ai-byok-policy` operator command with a rolled-back preview,
explicit apply, immutable version/rate validation, and workspace serialization.
It distinguishes explicit zero from an unset rate; disabling affects new
admissions and preserves the tariff reference. In-memory transaction tests cover
preview rollback, idempotency, changed-version rejection, new versions, disable,
and missing workspaces. No live workspace flag or tariff was changed.

### Prospective policy and accepted-run display checkpoint

Connection/profile lists now project edition permission and prospective funding
without persisting display metadata. Policy-store failures remain errors; an
intentional BYOK restriction disables the primary selection even when its
fallback is allowed. Settings and pickers disclose each route's policy. Run
details display the actual route and immutable admission fee, and agent summaries
use profile names instead of legacy size labels. Community has no financial copy.
Focused backend policy/connection tests and UI regressions passed. The EE web type
check caught a run/session union mismatch in Dock details; corrected it to read
the accepted run input and the type check passed. No live configuration changed.

### Admission and personal-origin review checkpoint

Review found that a personal profile using a shared fallback lost personal
inheritance. Added the originating profile owner to immutable selection metadata;
child admission, resume, refresh, and reconnection now retain owner checks even
for that shared fallback. Regression tests prove owner inheritance and rejection
after membership loss. Existing snapshots retain their connection-owner behavior.

Both production Helpin binaries now check Runtime run-credential capabilities at
launch, independently of global environment-key availability. ChatGPT requires
its own provider readiness plus this app's authenticated refresh callback;
capability failures never trigger fallback. CRM reviewed admission, legacy
selection, and restored selections use the same check. Runtime reports the
callback component without exposing its secret or URL. Focused admission,
connection, capability, and startup package tests passed. No current Helpin agent
configuration declares a lossless response-chain requirement; the SDK validation
and Runtime capability still distinguish ChatGPT from direct OpenAI.
