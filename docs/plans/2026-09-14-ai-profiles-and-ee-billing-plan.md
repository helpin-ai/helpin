# Named AI profiles, Helpin-owned credentials, and optional EE billing

## Summary

Make **named AI profiles** the common configuration mechanism in both editions. Helpin owns connections, model selection, personal preferences, and launch-time fallback. Agent Runtime receives the resolved model and credentials for every run.

SaaS supplies managed profiles named **Small, Medium, Large, and Flagship** through EE. These names have no special meaning in the open-source core.

This implementation covers all agent-run paths. Other Helpin AI features retain their current model routing for now, but their billing also moves behind the EE boundary.

## 1. Connections, profiles, and settings

- Add **Personal settings → AI connections** for account-wide API-key and ChatGPT connections, reconnect/disconnect actions, personal model profiles, and the user’s preferred profile for manual runs.
- Add **Workspace settings → AI** for shared API-key connections, shared profiles, and the workspace default profile. Workspace settings permissions control management; members may select permitted shared profiles without seeing credentials.
- A personal profile selects one personal connection, model, and model-specific controls. Personal ChatGPT connections cannot be assigned to shared agents or unattended automations.
- A shared profile contains a primary connection/model configuration and **one optional fallback configuration**. Store fallback configuration directly rather than linking profiles into arbitrary chains.
- Profiles own model selection, reasoning effort, and provider service tier. Agents retain tools, approvals, and execution limits.
- Replace model-size controls in Forge, Lens, other presets, and custom agents with a named-profile picker. Agents may select a shared profile or inherit the workspace default.
- Separate model capability metadata from prices. Core model configuration must not require an entry in the EE pricing catalog.
- Keep the currently supported providers; adding new provider protocols is outside this implementation.

## 2. Selection and fallback rules

Use one Helpin resolver for every launch path:

| Launch | Selection |
|---|---|
| Manual run with an explicit selection | Selected personal or shared profile |
| Manual run without an override | User’s preferred personal profile, if enabled; otherwise agent default |
| Scheduled, triggered, or customer-facing background run | Agent’s shared profile, or workspace default |
| Continuation or resume | Original resolved selection |
| Agent-created child run | Inherit the parent’s personal selection when explicitly carried through the trusted parent relationship; otherwise resolve the child agent’s shared default |

- Users explicitly enable “use the agent default when my personal connection is unavailable.” Default this preference off for existing users.
- Shared-profile fallback is also opt-in. Both fallback choices are visible in settings and launch summaries.
- Fallback happens **before execution only**, based on connection configuration/status and OAuth refresh results. Do not make paid model requests merely to probe availability.
- Invalid selections, permission failures, and EE billing rejection do not trigger fallback. A provider error after execution starts follows normal retry/error handling; authentication failure requires reconnection.
- Persist the selected profile ID/revision, actual connection, provider, model, controls, selection source, and fallback reason with the run. Profile edits cannot change an existing run.
- Show the effective profile, model, connection scope, and any fallback in the launch UI and run details. Replace “Runtime default” with “Agent default.”
- Recheck personal ownership and workspace membership at launch, refresh, and resume. Losing access must not silently switch the run to shared credentials.

## 3. APIs and Agent Runtime

- Add account-scoped connection/profile/preferences APIs beneath `/api/auth/me`, and workspace-scoped profile/connection management using the existing workspace authorization conventions.
- Add `ai_profile_id` to agent configuration and launch requests. Omitted launch selection means “apply the resolver”; provide an explicit “Agent default” selection that bypasses the personal preference.
- Retain the existing connection/model override fields for one compatibility release. Translate them into an explicit run selection and reject requests that also supply a profile selection.
- Extend the shared runtime SDK’s `RunModel` with typed model controls. An explicitly supplied controls object replaces legacy agent-level model controls, including when empty.
- Helpin sends a concrete `RunModel` and request-only `ModelCredential` for **every agent run**, including automation and child runs. Update supported SDKs and clients together.
- Remove model-provider API-key environment lookup and implicit provider/model fallback from Agent Runtime. Missing run credentials produce an actionable admission error.
- Runtime capabilities report supported providers/authentication modes and readiness to accept run credentials, rather than global provider-key availability.
- Retain encrypted run credential storage, terminal cleanup, and the existing refresh callback. OAuth refresh tokens stay in Helpin. Runtime service credentials and encryption keys remain necessary.
- Shared-connection refresh/replacement uses the persisted run-to-connection binding; personal connections additionally require the originating user’s authorization.

## 4. Extract all commercial billing into EE

- Put backend billing implementation under `server/ee` and frontend commercial implementation under `frontend/src/ee`.
- Use an explicit Go `ee` build tag and frontend edition entrypoint. Community is the default build; EE registration supplies commercial services and UI.
- Move Stripe, subscriptions, plans, seat charging, payment-based workspace locks, feature/plan gates, AI pricing, credit reservations, settlement, invoices, billing jobs, and upgrade/payment UI into EE.
- Replace core dependencies on concrete billing services with narrow interfaces for commercial admission, feature/limit decisions, and usage lifecycle notifications.
- Core retains permissions, operational resource limits, execution identity, and raw usage telemetry. Community implementations impose **no commercial limits** and require no pricing catalog, subscription, credits, or Stripe configuration.
- EE preserves existing commercial behavior and public billing APIs. Price the **resolved execution**, with an immutable pricing snapshot; a profile’s display name cannot determine its charge.
- EE registers managed Small/Medium/Large/Flagship profiles and their SaaS-owned connections. Customer-created profiles use the same core mechanism.
- Preserve existing billing records and historical migration checksums. Existing commercial tables may remain inert in community installations; new billing-specific migrations belong to EE.
- Verify community builds without either EE source directory present. Exclude commercial routes, navigation, jobs, and frontend bundles rather than merely hiding them with a runtime flag.

## 5. Migration, implementation order, and acceptance

Implement in four reviewable stages:

1. **Separate core model metadata and commercial interfaces**, then extract billing with EE behavior preserved and a working community build.
2. **Add connections, profiles, settings, and the resolver**, initially alongside legacy launch fields.
3. **Migrate agent configuration and wire every agent launch** to resolved, run-scoped credentials.
4. **Remove runtime environment-key execution** after legacy runs and deployment configuration are accounted for.

Migration requirements:

- Preserve existing personal connection IDs and run references when making ownership account-wide. Keep legacy encryption context readable while re-encrypting; do not merge duplicate connections automatically.
- Revoke a disconnected personal connection across all of that user’s bound workspaces/runs.
- Convert existing agent routes into profiles preserving exact provider, model, and model controls. Preserve historical agent versions and completed-run records.
- Provide an idempotent operator bootstrap command that imports existing Helpin-configured provider keys into shared connections. Missing credentials leave profiles visibly unconfigured.
- Inventory nonterminal runs using runtime environment keys. Finish or explicitly cancel them before deploying the credential-only runtime; do not silently change their execution identity.
- Deploy matching Helpin, runtime, SDK, frontend, and edition builds. Remove provider keys from runtime deployments only after credential-based canaries pass.

Acceptance tests:

- Personal connections work across the owner’s workspaces and remain inaccessible to other users.
- Manual PM, Ask Agent, manual automation, scheduled/background agents, child runs, and resume use the expected selection.
- Test primary/fallback selection, explicit overrides, missing defaults, expired OAuth, revocation, and membership loss.
- Profile edits do not affect queued/running/resumed runs; provider failures never cause a mid-run switch.
- With all runtime provider-key variables unset, API-key and ChatGPT runs succeed using supplied credentials. Capture outgoing authentication in test transports to verify the selected credential.
- Community starts and runs agents without EE source, billing configuration, or pricing data; raw usage remains available.
- EE retains billing, subscription, seat, lock, reservation, and settlement behavior, including idempotency under retries.
- Validate fresh community installs, existing EE database upgrades, and legacy connection decryption.
