# AI connections and profiles

This document describes the profile contract being implemented in the
[September 14 rollout plan](plans/2026-09-14-ai-profiles-and-ee-billing-plan.md).
It supersedes the September 12 manual-only, full-rate personal-connection contract.
Edition construction is implemented. Deployment remains gated on migration/bootstrap, canaries,
and live refresh/revocation validation. Do not restart an older installation into
profile-based wiring before completing that coordinated rollout.

Connections hold encrypted credentials; profiles bind a connection to an explicit
provider, model, and model controls. A profile may have one direct fallback.
Selection order is a manual override, an agent's shared default, then the workspace
default. Accepted runs retain their selected route and policy across retries and
resumes. Profile names do not determine charges.

Personal connections and profiles belong to a user within a workspace. They are
available for explicit manual use and trusted descendants. Shared connections
belong to the workspace and support unattended execution. Shared management uses
workspace settings permissions; selecting a shared connection requires current
workspace access. ChatGPT connections remain personal. Existing personal IDs and
encryption AAD are preserved; there is no account-wide credential migration.

Fallback happens before admission when the authorized primary connection is
known to be unavailable. Authorization, capability, policy, billing, and
infrastructure errors do not authorize fallback. Execution never changes routes
mid-run. CRM freezes its route in the reviewed setup and requires another review
to change that route; its financial policy is accepted when the run launches.

Community records normalized usage without Helpin token or tool charges. SaaS
managed routes retain hosted pricing. New SaaS BYOK uses an explicit versioned
flat USD fee per million normalized tokens, equally across providers and models;
paid tools are charged separately. A zero rate is valid; an unset rate is not.
SaaS BYOK defaults off per workspace. The historical percentage and full-equivalent
modes remain readable for older records but do not define new profile pricing.

## Configuration and bootstrap

- Apply core migrations through `cmd/migrate`. An EE build adds its migration
  source with `go run -tags ee ./cmd/migrate up`; historical SQL remains in the
  original ledger with unchanged checksums.
- Set Helpin's stable `AI_CONNECTION_ENCRYPTION_KEY` (32 bytes, raw, hexadecimal,
  or base64), and configure its runtime URL and service token.
- Set the runtime's separate `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY` on
  its API and every worker (32 raw bytes or base64).
- Configure the runtime app's `model_credential_callback` URL to Helpin's
  `/api/internal/agent-runtime/model-credentials/refresh`, with a token environment
  variable holding Helpin's `INTERNAL_API_SECRET`.
- ChatGPT requires Helpin's `CHATGPT_CONNECTIONS_ENABLED` and the runtime's
  `AGENT_RUNTIME_CHATGPT_ENABLED`. Enable them only in validated deployments.
  `CHATGPT_OAUTH_CLIENT_ID` optionally overrides the SDK's public client ID.

The operator bootstrap imports only explicitly named API-key environment
variables. From `server/`, preview a community migration with:

```sh
go run ./cmd/ai-bootstrap -env-file /absolute/path/to/helpin/server/.env -workspace WORKSPACE_UUID \
  -credential openai=OPENAI_API_KEY -credential openrouter=OPENROUTER_API_KEY
```

Omit mappings for keys that are not configured. Add `-apply` after reviewing the
JSON report. For SaaS-managed imports use `go run -tags ee ./cmd/ai-bootstrap`
with `-funding managed`. A changed existing key additionally requires
`-rotate-credentials`; this supports managed-key rotation without exposing those
connections to workspace credential editing.

Preview writes roll back. Apply runs in one workspace transaction. Repeating it
preserves edited profiles, cleared defaults, and current credentials. Missing
keys create visibly unconfigured routes. The established tier routes remain
unchanged; configure a usable workspace default if the Small route is unavailable.
Each migrated agent with an explicit model preserves its provider, model,
reasoning, routing controls, and independent execution configuration. Agents
without a model use their configured size, or the Small route when no size is
configured. The report counts these Small defaults as `agents_defaulted_to_small`.
Agents reuse an equivalent active shared profile, preferring the four standard
size profiles. Distinct custom routes get reusable profiles named for their model
and controls, never for an agent. Equivalence includes the connection, model,
endpoint and controls; a profile with a fallback is not a single-route match.
Migration `202609140010` adds a workspace bootstrap completion marker. After the
first successful apply, reruns can import or rotate credentials but do not
reassign agents, including agents reset to workspace inheritance. Explicit models
still require a provider; unknown sizes and invalid controls fail the preview. Historical
versions, reviewed CRM setups, completed runs, and active run identities are not
rewritten. Older CRM setups need review/publication to acquire an explicit profile.

The report lists nonterminal runs still using runtime defaults. Finish or
explicitly cancel them before enabling Helpin's trusted runtime app policy
`require_run_model_credentials`. The runtime enforces this policy at engine
admission, before queueing. Other apps and standalone runtime installations keep
their existing environment keys and defaults. Provider readiness remains a startup
snapshot: restart after changing runtime provider-key configuration.

## Refresh and validation

The callback checks connection/run ownership, active access, provider/account,
and runtime mapping. Row locks serialize refresh; rejected-token fingerprints
avoid repeated rotation by concurrent workers. Reconnect updates active run
credentials and resumes matching authentication interactions. Disconnect clears
the secret and revokes bound credentials. A post-launch check covers disconnects
racing admission. Already in-flight requests may finish; revocation prevents
subsequent model calls.

The September 13 test-system run `run_9b1445790f13e52c710f2c3d` (Helpin run
`086a992f-a4ca-479b-8923-67923861fa9d`) used `openai_chatgpt` / `gpt-5.6-terra`
with app-owned OAuth credentials and produced 10 model responses and 37 tool calls
before a Git tool stalled. This supports inference and tool execution; live
expired-token refresh, reconnect, and revocation are separate release gates.
ChatGPT strips the previous-response identifier and does not support lossless
provider-state replay, although ordinary transcript continuation is supported.

Helpin and Runtime pin the published SDK `v0.6.0-alpha.2`, verified without Go
workspace substitution. Explicit empty model controls clear inherited controls
while preserving execution limits. Chat Completions and the real local-model
adapter validation are implemented; deployed Helpin canaries remain pending.

## Build editions

Community is the default Go build (`go run ./cmd/api` and `go run ./cmd/temporal-worker`). It records usage with no financial policy, price catalog, subscription gate, or billing jobs. Missing deployment provider keys do not prevent startup; a feature still needs a configured provider when invoked.

SaaS uses `go run -tags ee ./cmd/api` and `go run -tags ee ./cmd/temporal-worker`, plus `go run -tags ee ./cmd/migrate up` for registered EE migrations. Its API and every worker require `AI_CONNECTION_ENCRYPTION_KEY`, even when personal ChatGPT is disabled. Existing workspaces must run the managed profile bootstrap before cutover. Newly created SaaS workspaces receive managed connections/profiles from the explicitly configured Helpin provider keys after their agent defaults are seeded. Missing providers remain unconfigured.

Container builds default to community too. SaaS builds pass `--build-arg GO_BUILD_TAGS=ee`; the staging and production workflows declare that choice explicitly. Both binaries must use the same edition. No live migration or service restart command was run as part of this change.

The frontend also defaults to community: `pnpm --dir frontend dev` and `pnpm --dir frontend build`. SaaS development uses `pnpm --dir frontend dev:ee`; production uses `pnpm --dir frontend build:ee`. The build command fixes the edition for both TypeScript and Vite, so an inherited environment value cannot select mismatched implementations. Staging and production workflows explicitly build EE. Community omits billing navigation, payment requests, upgrade UI, and price assets; old billing URLs return not found. Desktop builds that reuse frontend components default to the same community extension points.

### SaaS BYOK operator policy

The EE-only operator command previews a transactional policy change by default:

```bash
cd server
go run -tags ee ./cmd/ai-byok-policy \
  -workspace "$WORKSPACE_ID" -mode enable \
  -tariff-version "$TARIFF_VERSION" \
  -microusd-per-million-tokens "$RATE_MICROUSD"
```

Supply an explicit nonnegative integer rate: `1000000` represents USD 1 per
million normalized tokens; `0` explicitly configures no Helpin token fee.
An omitted rate is invalid. Review the JSON result, then repeat with `-apply`.
The command loads `DATABASE_URL` from the environment or `server/.env` and
requires the core and EE migrations to have been applied. It does not restart
services, test provider credentials, or change accepted executions.

A tariff version can be reused only with identical values. A different rate
requires a new version. Paid tools retain the separate tool tariffs frozen at
admission. To block new BYOK executions while preserving existing run snapshots
and refresh authorization, preview `-workspace "$WORKSPACE_ID" -mode disable`,
then repeat with `-apply`. Disabling preserves the workspace's tariff reference.
Use connection/run revocation when accepted executions must also stop.

Launch readiness is checked against Runtime's capabilities before acquiring a
credential or choosing a fallback. Helpin uses `run_credentials_configured` and
supported authentication modes, not global provider-key availability. ChatGPT
also requires this app's `model_credentials` callback component to report
configured authentication. Upgrade Runtime before restarting Helpin with this
check. These are configuration checks, not provider authentication probes.

A personal profile's originating owner is frozen separately from its actual
connection owner. When a personal route falls back to a shared connection, its
children, resume, and refresh still require the originating member. Profile
edits or deletion do not rewrite the accepted selection.

## Compatible/local agent models

Use the Runtime `openai_compatible` provider for Chat Completions endpoints. The
Runtime administrator approves each endpoint's ID, canonical URL and authentication
mode in the trusted Helpin app entry. Local HTTP requires explicit `allow_http`.
Helpin's connection form selects an approved ID; it cannot send an arbitrary URL.
No-auth endpoints omit API keys and still have a revocable run credential record.
A profile can name an unpriced model and freezes the connection's endpoint binding.
Changes to that binding require a new connection/profile; accepted runs never
redirect their credentials. Compatible routes have transcript continuation, not
lossless Responses replay or provider-specific reasoning/service-tier controls.

See Runtime `docs/2026-09-14-compatible-models.md` for the real local Qwen test and
`docs/2026-09-14-helpin-deployment.md` for fresh host/compose templates. SDK
`v0.6.0-alpha.2` is released and pinned in both consumers.

Host SaaS development commands are `just backend-ee`, `just worker-ee`, and
`just frontend-ee` in separate terminals. Community uses the existing commands
without the `-ee` suffix. Do not switch a live EE deployment to Community while
accepted EE runs are outstanding; drain them first to preserve their frozen policy.

Bootstrap reads only the process environment unless `-env-file` names an explicit
dotenv file. It never guesses a file from the working directory. Existing process
environment values take precedence; each imported key still requires an explicit
`-credential provider=ENVIRONMENT_VARIABLE` mapping.
