# AI profiles, community BYOK, and optional SaaS BYOK

Status: agreed implementation plan. This document replaces the earlier combined proposal; it does not record completed implementation.

Split delivery into a standalone catalog prerequisite, **Plan A: AI configuration and execution**, and **Plan B: commercial billing extraction**. Plan A can use an adapter to the existing commercial implementation while Plan B proceeds separately. A clean community distribution requires both plans.

## Product decisions

Named AI profiles are the common configuration mechanism. Helpin owns connections, model selection, authorization, and fallback before launch. Agent Runtime receives the resolved model configuration and run-scoped credentials.

| Edition and route | Availability | Helpin charging |
|---|---|---|
| Community, customer credentials or local endpoint | Core functionality | No credits, subscription, or Helpin usage charges |
| SaaS, Helpin-managed credentials | Default; managed Small, Medium, Large, and Flagship profiles | Existing managed usage charges |
| SaaS, customer API key or personal ChatGPT connection | Implemented in the first release; workspace feature flag defaults off | Explicitly configured, versioned usage-based platform fee |

- Small/Medium/Large/Flagship are managed profile names supplied by the commercial integration, not special routing concepts in core.
- SaaS BYOK includes connection management, explicit profile selection, and one optional fallback from the start. Enforce its workspace flag in both backend APIs and UI.
- Enabling SaaS BYOK requires a complete fee policy. Do not inherit the existing 10% customer-funded mode or the full-rate customer-funded-platform mode. This plan chooses no monetary rate; configuration must specify the applicable usage basis and treatment of paid tools before activation.
- Charge the route actually selected: customer credentials use the configured SaaS BYOK policy; fallback to Helpin credentials uses normal managed pricing. Disclose both outcomes before selection and persist the actual policy with the run. Community charges neither route.
- Personal connections remain owned by a user **within a workspace** in this release. Account-wide ownership migration and automatic personal-profile preferences are deferred. Manual runs use an explicit override or the agent/workspace default.
- Shared workspace connections support unattended agents. Personal ChatGPT connections are available for manual execution and authorized descendants, not unattended schedules or shared agent defaults.
- Scope covers every agent-run launch path. Other Helpin AI features retain their current routing, but receive the community metering implementation and are included in the later billing extraction.
- Add a Chat Completions-compatible runtime transport for local agent execution. This does not claim that every Helpin AI feature can run locally.

The implemented contract will supersede [AI connections](../ai-connections.md) where it specifies runtime environment-key defaults and full-rate charging for customer credentials. Preserve its personal workspace scope and restrictions on unattended personal execution. Update that document in the implementation PRs; do not treat the existing database encryption context as account-wide.

## Prerequisite PR: separate capabilities from pricing

Do this first, in isolation, preserving current SaaS routes and charges.

- Split the current `aiusage.RouteDefinition` concerns into neutral model capabilities and commercial prices. Context limits, supported controls, cache support, protocols, and authentication modes must be usable without a price record.
- Update tier resolution, selectable models, startup validation in both Helpin binaries, and frontend catalog generation. Core configuration and custom models must not depend on the generated commercial pricing catalog.
- Put shared structural validation in the SDK and have Helpin and Runtime consume it. Runtime remains authoritative for supported execution capabilities. Remove the mirrored provider/control validation rules as consumers migrate.
- Distinguish ordinary transcript continuation from lossless provider-specific reasoning/state replay. Reject incompatible profiles for agents that require the latter; do not advertise a provider as supporting it merely because another provider or execution path does.
- Define which execution fields are model controls. Applying a profile updates those fields without replacing unrelated tools, approval settings, native-context configuration, or execution limits. Preserve reasoning effort when migrating an existing tier snapshot.

Acceptance: existing managed model selection and prices remain equivalent; a custom unpriced model can be represented and validated without inventing a price; provider/control validation agrees across SDK, Helpin, and Runtime.

## Plan A: AI configuration and execution

### A1. Supply a complete community usage lifecycle

Community needs a zero-financial implementation, not a nil billing service scattered through callers.

- Define neutral interfaces for admission and the existing usage lifecycle: preflight, checkpoints, heartbeat, suspension/resume, settlement, failure, and release. Preserve stable operation IDs and retry/idempotency behavior.
- Implement community admission without financial reservations or credit checks. Retain execution identity, raw usage, audit records, and operational safeguards. Unknown or unpriced models must work without a fabricated catalog entry.
- Wire the implementation through API and worker startup and all metered consumers, including embeddings and reranking. These consumers still require their metering context and audit information; absence of a commercial meter must not remove those requirements.
- Adapt current SaaS billing to the same contract. Keep managed pricing unchanged; supply an explicit fee policy for flagged SaaS BYOK.
- Do not use existing customer-funded constants as a shortcut for free community execution: current modes include percentage or full equivalent charges.

Inventory actual call sites and required methods before changing the interface. Some gateways already permit a nil commercial meter while still requiring context; preserve that distinction during migration.

### A2. Connections, profiles, and settings

- Add **Personal settings → AI connections** with API-key and ChatGPT creation, status, reconnect/disconnect, and personal profiles. Clearly show the active workspace scope.
- Add **Workspace settings → AI** for shared connections, shared profiles, and the workspace default. Workspace permissions govern management; selecting a shared profile never exposes its secret.
- A profile owns a primary connection, provider/model, protocol where applicable, and model controls. Allow one optional fallback route stored directly. Do not resolve arbitrary chains of linked fallback profiles.
- Personal profiles may use the owner's personal connection and an authorized shared fallback. Shared profiles may reference only shared connections or permitted SaaS-managed routes. Shared agent defaults cannot reference personal profiles.
- Replace agent model-size selectors in Forge, Lens, other presets, and custom agents with a profile picker or workspace-default inheritance. Keep agent tools, approvals, and execution limits separate.
- Community supports custom model identifiers and approved local endpoints without price records. SaaS may require a configured tariff before a customer route is eligible for its fee policy.
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

### A4. SDK and credential-only Runtime

- Add `ai_profile_id` to agent configuration and launch requests. An omitted override uses the resolver. Retain legacy connection/model overrides for one compatibility release, translate them into an explicit selection, and reject conflicting profile and legacy inputs.
- Extend SDK `RunModel` with typed model controls and the transport/endpoint configuration needed for compatible endpoints. Explicit model controls replace legacy model-control values, including when empty; unrelated agent execution fields remain intact.
- Helpin supplies a concrete model and request-only credential specification for every launch, including automation, CRM, and children. Local endpoints without authentication require an explicit supported no-auth mode, not an absent credential that invokes defaults.
- Remove Runtime model-provider environment-key lookup and implicit provider/model defaults. Missing or unsupported resolved configuration produces an admission error.
- Runtime capabilities report supported protocols/providers, authentication modes, controls, and readiness to accept run credentials rather than ambient provider-key availability.
- Retain encrypted run credential storage, terminal cleanup, and the app refresh callback. OAuth refresh tokens stay in Helpin. Runtime service credentials and encryption keys remain required.
- Refresh must honor the persisted run/connection binding, workspace, originating owner where applicable, and revocation state. Preserve request/log redaction and never send model credentials to host tool callbacks.
- Publish a fetchable SDK release and update Helpin and Runtime together. Validate clean builds without local module replacements and verify the release can be fetched by a fresh installation.

### A5. Chat Completions-compatible local execution

- Add a distinct Chat Completions transport. Existing Responses support is not sufficient for an endpoint that only implements Chat Completions.
- Cover streaming text, tool calls and results, stable call IDs, multi-turn execution, cancellation, continuation/recovery, provider errors, and usage reporting. Advertise only capabilities the transport implements.
- Permit an explicitly configured no-auth local connection. Other routes continue to require their declared authentication mode.
- Bind endpoints to administrator-approved connection configuration. A browser launch request cannot supply an arbitrary destination for a stored credential. Allow private/local HTTP only through explicit configuration; redirects must not leak authorization to another origin.
- Validate against a real compatible local server and a model that supports the required tool behavior, alongside protocol fixtures. Record the tested server/model and capability limits. A separate native Ollama protocol is not required for this release.

### A6. Migration and deployment

- Preserve existing personal connection IDs, workspace/user ownership, run references, and encryption AAD. There is no account-wide re-encryption migration in this release.
- Convert existing agent routes to profiles while preserving exact model controls and unrelated execution settings. Preserve historical agent versions, reviewed CRM snapshots, and completed-run records.
- Provide an idempotent Helpin operator bootstrap that imports explicitly supplied provider credentials into shared or managed connections as appropriate. Missing credentials leave a route visibly unconfigured; do not depend on private Doppler access or scraping process environments.
- Inventory nonterminal runs that still use Runtime environment credentials. Finish or explicitly cancel them under the rollout procedure before removing that execution path; never silently change their identity.
- Update fresh-install configuration across **Helpin, agent-runtime, and agent-runtime-go**. Configure the Runtime API and every worker with the model-credential encryption key, durable state, app authorization, and callback wiring. Cover host development and compose deployment.
- Remove Runtime provider keys only after credential-based canaries pass. A fresh community installation must work with documented configuration and fetchable dependencies, without private prebuilt binaries.
- Track ChatGPT inference and live refresh as separate gates. Inference has been observed in the test system; expired-token refresh, restart, and revocation still require explicit end-to-end validation before rollout. Update stale validation statements in the existing connections document.
- Keep SaaS BYOK off until connection, refresh, fallback, fee, and authorization acceptance tests pass and an explicit tariff is configured. Implement the complete path before enabling the flag.

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
- A second ledger is not required by this design. Test fresh installs, existing upgrades, and edition transitions against the selected registration scheme.

### B3. Extract implementation and commercial policy

- Put backend commercial implementation under `server/ee` and frontend commercial implementation under `frontend/src/ee`. Use an explicit Go `ee` build tag and frontend edition entrypoint; community is the default build.
- Move Stripe, subscriptions, plans, seats, payment-based workspace locks, commercial feature gates, prices, credits, reservations, settlement, invoices, billing jobs, and payment/upgrade UI into EE.
- EE also supplies managed Small/Medium/Large/Flagship profiles, SaaS-owned connections, the workspace SaaS BYOK availability policy, and its configured usage fee.
- Core retains connection/profile functionality, authorization, operational limits, raw telemetry, the complete community usage lifecycle, and neutral extension interfaces. Community has no commercial limits or payment dependency.
- Preserve existing managed billing APIs and intended charges. Freeze the resolved execution's policy and rates for settlement; profile names must never determine charges, and later fee edits must not retroactively price accepted runs.
- Build and test community with both EE source directories absent. Commercial imports, routes, navigation, assets, and jobs must be absent, rather than hidden by a runtime feature flag. No Stripe initialization or price catalog is required for community startup.

## Delivery order and acceptance

1. Merge the standalone capability/catalog split with existing SaaS behavior preserved.
2. Deliver Plan A in bounded changes: the usage lifecycle and commercial adapter; connections/profiles/settings; resolver and launch coverage; SDK/Runtime transport and credentials; migration and deployment. Keep the old execution path until the replacement and rollout gates pass.
3. Develop Plan B separately after its interface prerequisites: verified correctness fixes, migrator registration, then backend/frontend extraction. Full EE removal is not bundled into the first catalog PR.
4. Enable SaaS BYOK only for explicitly configured workspaces after its tests pass. Release community only after both agent execution and the EE-free build gates pass.

### Plan A acceptance

- Community shared API-key, personal ChatGPT, and compatible local routes launch without credit balances, subscriptions, or commercial price entries. Raw usage and audit context survive preflight, interruption, approval pause, heartbeat, resume, failure, and settlement without financial effects.
- Existing SaaS managed routes retain their expected charges. SaaS BYOK is blocked by backend and UI when disabled or lacking a complete fee policy; when enabled, the configured fee is applied exactly once. Test fallback in each funding direction and changes to policy after acceptance.
- Test personal owner/workspace isolation, shared management permissions, expired OAuth with live refresh, disconnect, membership loss, and trusted-child authorization.
- Exercise every launch path in A3, including CRM's reviewed snapshot, queued runs, children, and recovery. Inspect outgoing model authentication in test transports to prove selection; checking a stored connection ID alone is insufficient.
- Test primary/fallback availability, missing defaults, invalid capabilities, explicit overrides, legacy-input conflicts, and no switch after execution starts. Profile edits must not alter accepted runs.
- Test that profile/tier migration preserves reasoning controls, native context, approvals, and other execution settings. SDK, Helpin, and Runtime agree on supported controls and continuation requirements.
- With all Runtime provider-key variables unset, run API-key, ChatGPT, and explicit no-auth local execution. Cover streaming tool calls, multiple turns, cancellation, worker restart, terminal credential cleanup, and credential redaction.
- Validate fresh deployment using released dependencies and documented keys/configuration across all three repositories. Record live ChatGPT inference and refresh results separately.

### Plan B acceptance

- Community builds and runs with EE directories absent, without commercial configuration, prices, routes, frontend assets, or billing jobs. Operational authorization and resource safeguards remain active.
- EE regression tests cover managed usage pricing, the configured BYOK fee, subscriptions, seats, workspace locks, reservations, and settlement. Webhook and worker retries cannot duplicate a charge or lose a business update after recording an event.
- Fresh community and EE databases, existing EE upgrades, and supported edition transitions preserve migration history, strict checksums, connection decryption, and historical billing records.
