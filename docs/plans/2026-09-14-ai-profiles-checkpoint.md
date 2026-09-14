# AI profiles implementation checkpoint — September 14, 2026

Implementation resumed at the user's request. See [Resumed implementation](#resumed-implementation) for the current state. The original pause snapshot below is retained as history; its outstanding-code list is superseded by that update.

Plan: [AI profiles, community BYOK, and optional SaaS BYOK](2026-09-14-ai-profiles-and-ee-billing-plan.md).

## Repository boundaries

All three repositories are on `feat/native-only-runtime`. Working trees were
clean before this checkpoint document was added.

- Helpin implementation head: `1961fc577`.
- Agent Runtime implementation head: `3d838f2`.
- SDK head: `c401c0e`, published as `v0.6.0-alpha.1`; both consumers use that
  fetchable version without local module replacements.
- Keep **Helpin and Agent Runtime commits local. Do not push either repository.**
- The user accepted the earlier SDK publication, including prior branch commits.
  A future SDK release is part of the transport plan, but none was made at this
  checkpoint.
- The user performs live migrations and service restarts. Provide concrete,
  reviewed commands when ready. No live migration, restart, tariff activation,
  or production billing action was performed in this implementation session.
- The user authorized testing at the supplied Helpin dev URL. Credentials are in
  the conversation only; do not copy them into files or test logs. No live login
  or authenticated canary was performed yet.

## Completed implementation

The plan's earlier checkpoints and Git history contain the details. Major pieces:

- Neutral model/capability catalog separated from commercial prices.
- Complete community usage lifecycle and raw usage persistence, preserving
  operation identity, checkpoints, settlement, and retry behavior.
- Shared/personal workspace connections, revisioned profiles, one fallback,
  workspace defaults, agent profile configuration, settings and launch pickers.
- Common resolution across ordinary/Temporal launches and CRM reviewed launches;
  accepted selection and controls are frozen. Legacy overrides remain compatible.
- SDK model controls/shared validation; Runtime app policy enforced in engine
  admission. Standalone/Usermaven defaults remain supported.
- Precise flat-per-million SaaS BYOK fee accounting and separate paid tools;
  immutable tariff snapshots, explicit zero versus unset, workspace flag off by
  default, and managed/customer funding classification.
- Operator profile bootstrap with preview/apply and explicit credential mappings.
- Subscription webhook receipt/business write transaction fix.
- Optional migration source registration using the existing checksum ledger;
  historical SQL remains in place.
- Backend commercial implementations under `server/ee`, CE default build,
  explicit `ee` build tags, edition startup wiring and financial jobs.
- Frontend commercial implementations under `frontend/src/ee`, CE default
  entrypoints and explicit EE builds; production workflows select EE.
- EE-free source archive build/test scripts for backend and frontend.

Latest local commits:

| Repository | Commit | Change |
|---|---|---|
| Helpin | `8dddced3f` | Frontend billing extraction and edition builds |
| Helpin | `c92a7fd02` | EE-only `ai-byok-policy` operator command; transactional preview, immutable versions, explicit rate, disable |
| Helpin | `70f05ae77` | Prospective connection policy, disabled selections, separate primary/fallback fees, immutable accepted-run details, profile labels |
| Helpin | `1961fc577` | Personal profile owner retained through shared fallback/children/refresh; Runtime readiness checked in production launch paths |
| Runtime | `3d838f2` | App-scoped credential refresh capability, without callback secret or URL |

The personal-origin review found and fixed a real issue: using a shared fallback
previously lost the originating personal identity. `profile_owner_id` now persists
separately from the actual connection owner. Membership loss blocks restore and
refresh; another user cannot inherit the selection. Older snapshots retain their
existing connection-owner behavior.

Runtime readiness uses `run_credentials_configured` and supported auth modes,
independently of global provider-key availability. ChatGPT requires its own
provider capability plus this app's authenticated `model_credentials` callback
component. Upgrade Runtime before restarting Helpin with this check. No existing
Helpin agent configuration declares a lossless response-chain requirement;
ChatGPT still must not inherit direct OpenAI's lossless replay capability.

## Validation recorded

Most recent focused checks passed:

- Helpin profile/connection/admission tests, including personal fallback owner,
  membership loss, Runtime capability rejection, and no capability fallback.
- EE tariff operator preview rollback, idempotency, changed-version rejection,
  new version, explicit zero/unset, disable, and missing workspace tests.
- Runtime app configuration and API capability tests.
- 24 frontend tests covering automation summaries, profile selection and fee
  details; earlier 8-test run also covered Dock details.
- EE frontend TypeScript check passed after fixing a run/session union mismatch.

Logs in `/tmp` (temporary evidence, not release artifacts):
`helpin-profile-admission-tests.log`, `helpin-personal-origin-tests.log`,
`helpin-policy-backend-final.log`, `runtime-app-readiness-tests.log`,
`helpin-ai-policy-final-ui.log`, `helpin-ai-policy-ui-tests.log`,
`helpin-ai-policy-types.log`.

Earlier extraction validation:

- Full backend build and core/cmd tests passed in an archive with `server/ee`
  absent. EE backend regression suites passed.
- CE and EE production frontend builds and desktop TypeScript passed.
- Full EE frontend run exposed two outdated automation fixtures; corrected them
  and all 26 affected cases passed.
- CE frontend archive built without EE. The large test invocation had 2,607
  passing tests and runner/module-loading timeouts under host contention.
  All affected suites passed serial reruns, including the checklist suite.
  Do not describe that original full invocation as entirely green.

Serialize heavy tests/builds on this host. Use `/snap/go/current/bin/go` and
`/snap/go/current/bin/gofmt`. Local HTTP fixtures require sandbox escalation;
this authorization does not permit live migrations or restarts. For single-worker
Vitest runs pass **both** `--minWorkers=1 --maxWorkers=1`.

## Remaining work

### A5: compatible Chat Completions transport (not implemented)

Only investigation occurred. No transport code, dependency change, or new SDK
release has been made.

- Add a distinct provider/transport, SDK endpoint binding and explicit no-auth
  mode. Publish a fetchable SDK version and update both consumers together.
- Bind destination to administrator-approved connection configuration. Browser
  run overrides must not choose where a stored credential is sent. Local HTTP
  requires explicit configuration; redirects must not leak authorization.
- Support streaming text and tool calls/results, stable IDs, multi-turn,
  cancellation, errors, usage, and transcript continuation/recovery.
- Integrate Helpin connection storage/configuration, resolver, settings and UI.
- Validate with fixtures **and an actual compatible local server/model** before
  claiming local-agent support. No actual server/model was identified or run.

Investigation notes:

- Runtime already has `EinoChatModelFactory` in
  `internal/runtime/native_eino_model.go` and native/Eino message conversions.
- Published Eino OpenAI Chat Completions adapter metadata resolved to
  `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`; it was **not**
  downloaded into the module or selected as the final dependency. Inspect its
  configuration and streaming behavior before choosing it.
- Inspect interleaved tool-call indexes and empty argument fragments carefully:
  the current generic native streaming path tracks a current call ID, and the
  Eino converter substitutes `{}` for empty arguments. Their compatibility with
  the new transport is not yet validated.
- Runtime credential transport currently always adds Bearer authorization for
  non-Anthropic credentials. Explicit no-auth needs a real transport branch,
  persisted source, admission validation and revocation behavior.
- OpenAI Docs skill was announced/read. Official pages fetched:
  [Function calling](https://developers.openai.com/api/docs/guides/function-calling)
  and [Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create).
  No API inference was performed.

### Integration review and UI completion

- Pickers disclose explicit selection fees. Complete the effective inherited
  agent/workspace-default route and fee disclosure before launch; currently the
  default option does not resolve/display that profile's details. Carry agent
  default context through PM, Run Now, and Dock where available.
- Review managed connection UI actions and final populated/empty/error/keyboard,
  narrow-screen and dark-mode behavior in the actual dev application.
- Check remaining core-only commercial analytics helpers and the legacy optional
  weighted-credit adapter for proven dead code before removing anything. Preserve
  historical billing data/modes and compatibility.
- Review all plan acceptance items against final implementation; do not infer
  full completion from focused test passes.

### A6: deployment and live acceptance

- Finish fresh-install configuration across all three repositories: API and all
  workers' encryption keys, durable state, app authorization and refresh callback,
  host and compose setup, CE/EE commands and edition transition procedure.
- Prepare user-run core+EE migration and explicit managed-profile bootstrap
  commands. Operator commands are documented in `docs/ai-connections.md`.
- Inventory/drain or explicitly cancel nonterminal Helpin runs that still use
  Runtime environment defaults before enabling strict Helpin app policy. Other
  apps retain defaults. Do not silently change accepted execution identities.
- Ask the user to run migrations/restarts only once exact commands/config are
  ready; no live rollout has been performed.
- Run API-key and ChatGPT canaries with explicit run credentials; verify actual
  outgoing auth, durable recovery/restart, cleanup/redaction, both funding
  fallback directions, and authorization. Keep SaaS BYOK off until an explicit
  tariff and acceptance gates are satisfied.
- Expired ChatGPT refresh, reconnect and revocation remain live validation gates.
  The September 13 inference evidence is documented in the plan and does not
  prove these gates. Interactive OAuth may require the user.
- Verify Helpin strict admission cannot consume global keys while a Usermaven-style
  app continues to use its existing defaults.
- Document supported EE-to-CE transitions with accepted EE work drained first;
  historical EE migration rows/data remain intact.

## Resume instructions

Start by reading this checkpoint, the full plan and current repository status.
Continue outstanding work in reviewable local commits, reviewing and correcting
each major step. Do not spawn subagents under the current instructions. The user
explicitly paused execution; wait for their request to continue.


## Resumed implementation

The remaining code is implemented. **Live rollout and acceptance are not complete.**
Helpin and Runtime commits remain local; no live migration, restart, workspace
flag/tariff change, login, or authenticated deployment canary was performed.

- SDK `f5b9dae`, released as `v0.6.0-alpha.2`: approved endpoint bindings and
  protocol/capability metadata. Both consumers pin the fetchable release.
- Runtime `c374065`: compatible provider, admission/recovery endpoint binding,
  API-key/no-auth transport, streamed tool IDs, usage and summaries.
- Runtime `91614d2`, `ffe0215`: durable host/Compose templates and refreshed docs.
- Helpin `f30b463e6`: compatible connections/profiles/UI, endpoint-bound secrets,
  inherited route disclosure, and OAuth outage/fallback correction.
- Helpin `543b51fa2`: removed the dead core weighted-credit adapter and floors;
  moved commercial analytics into EE; tests now use the production lifecycle.
- Helpin `0bd6487f5`: empty edition policies cannot admit a run.
- Helpin `c98453bcf`: Ask Agent default loading, send guard, and UI fixture fixes.

Validation:

- Full runtime app/engine/credential/model suites and vet passed. Actual local
  Qwen3-0.6B on llama.cpp b10955 passed streamed tool use, transcript restoration,
  no-auth and reported usage. The temporary server/downloads were removed. This
  does not claim a deployed Helpin canary or an actual durable-worker restart.
- Full Helpin service and EE suites passed; the final floor-removal, missing
  lifecycle, and empty-policy regressions also passed. Backend vet passed.
- A Community source archive built API/worker/migrator/bootstrap and passed all
  core/command tests with `server/ee` absent. Subsequent small guard/floor changes
  passed the affected service/policy suites.
- The Community frontend archive built with `src/ee` absent. Its broad run had
  436 passing files, one failed Dock file, and a widget pageview-timer teardown
  error. The Dock fixture was updated for the new default query, and the support
  fixture now mocks unrelated widget initialization. A fresh EE-free archive then
  passed all 124 affected Dock/support/profile/analytics tests with no errors.
  The original broad invocation is not claimed as an entirely passing run.
- EE profile/automation checks passed after the Dock correction; 85 Dock tests
  passed separately. EE type checking and the final frontend production build passed, as did EE
  API/worker/migrator/bootstrap/BYOK-operator builds. Runtime Compose validates, and the fresh
  app example constructs strict admission plus an authenticated callback manager.

### Operator steps still needed

The user owns live migrations and restarts. No matching dev services were found
in this host's systemd list/process inspection, so their actual restart commands
must be supplied; do not guess service names or restart unrelated processes.

On the dev host, with its existing private Helpin configuration loaded:

```bash
cd /root/helpin/server
GOWORK=off go run -tags ee ./cmd/migrate pending
GOWORK=off go run -tags ee ./cmd/migrate up
```

Then preview `cmd/ai-bootstrap` for the chosen workspace with `-funding managed`
and explicit `-credential provider=ENV_VAR` mappings; inspect unconfigured routes
and nonterminal legacy runs, then repeat with `-apply`. The exact command and
rate preview depend on the workspace and operator-selected mappings/rate. See
[AI connections](../ai-connections.md) for all command flags.

Use EE consistently across API, worker and frontend. Runtime API and every worker
need the same stable model encryption key and trusted callback config. Preserve
other apps' global defaults. Finish/cancel legacy Helpin runs before enabling its
strict policy, after credential canaries pass. Do not overwrite shared app config
with the fresh-install example.

Two questions were left pending: the dev launch/restart method, and the workspace
plus flat USD/million-token rate for SaaS BYOK canaries (explicit zero is supported).
BYOK stays off until the user applies that configuration. Paid tools remain
separate. No fee choice or elapsed time constitutes permission to activate it.

Still required after deployment: current UI/keyboard/mobile/dark visual checks;
API-key and ChatGPT runs with outgoing credential attribution; live expired OAuth
refresh, reconnect and revocation; durable interruption/worker restart/cleanup;
strict Helpin versus unchanged other-app defaults; and actual database migration
and edition-transition checks. Preserve these as release gates, not completed tests.
