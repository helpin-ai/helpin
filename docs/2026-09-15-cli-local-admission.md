# Agent Runtime CLI: Helpin admission (Phase 3)

## Implemented scope

Helpin exposes the generic `agent-runtime-cli/v1alpha1` discovery, OAuth, admission,
and execution-lease contract. The CLI contains no Helpin task/workspace branches.
It can obtain grants from Helpin and from an independent project-based host using
the same compiled binary.

This phase does not implement Helpin's model gateway, remote tools, event ingestion,
artifact upload, or local-result billing. Helpin advertises only `admission` and
`execution_leases`. The CLI refuses a connected coding run before admission when
`model_gateway` is absent. Standalone local coding remains available.

## Enablement and deployment order

1. Apply additive migration `202609150001_cli_local_admission.sql` using the normal
   migration runner. AutoMigrate also includes these models for development.
2. Deploy the API and frontend consent page with `CLI_ENABLED=false` (the default).
3. Set `CLI_PUBLIC_BASE_URL` to the public **API origin**, e.g.
   `https://api.helpin.ai`; this is required when enabling the feature.
   Set `APP_BASE_URL` to the frontend origin. Both must be origins without paths.
4. Verify the API origin routes `/agent-runtime/cli.json`,
   `/.well-known/oauth-authorization-server/api/cli/oauth`, and `/api/cli/*` to the
   API service. Existing API-origin ingress routes already forward these paths.
5. Enable `CLI_ENABLED=true` in an environment intended for admission testing.
   Enabling admission reserves AI usage and creates visible queued local runs;
   revoke unused admissions afterward. No deployment or flag change is part of
   the source implementation.

For local development use `CLI_PUBLIC_BASE_URL=http://127.0.0.1:8080` and
`APP_BASE_URL=http://localhost:5173`. The CLI connects to the literal loopback API
address. Device flow/SSH forwarding is not implemented.

The SQL uses text IDs matching the new GORM models. `run_id` intentionally has no
foreign key: admission reserves its identity before shared launch creates the
normal run. The migration is additive and repeatable. Test the migration against
an isolated PostgreSQL instance before rollout; this workspace's service tests
use SQLite.

## Identity and authorization

- Public client: `agent-runtime-cli`; exact scope: `agent:local`.
- Separate issuer: `${CLI_PUBLIC_BASE_URL}/api/cli/oauth`.
- Exact resource: `${CLI_PUBLIC_BASE_URL}/api/cli/v1`.
- Browser authorization uses the existing authenticated session and explicit
  workspace choice. Workspace membership, PM edit/module access, and current MFA
  requirements are checked at consent and on protected token/grant operations.
- The trusted browser token supplies MFA evidence. Clients cannot provide an MFA
  claim in the consent payload. Normal JWTs, MCP tokens, provider keys, and worker
  service tokens are not accepted as CLI OAuth credentials.
- Authorization codes expire after two minutes; access tokens after ten minutes;
  refresh tokens after thirty days. All persisted credentials are SHA-256 hashes.
- Codes are single-use and bound to PKCE S256, client, resource, and the exact
  `http://127.0.0.1:PORT/callback` URI. Refresh rotation is atomic; refresh reuse
  revokes the entire connection. Logout revokes the connection and all derived
  authority. Current membership and policy still apply after issuance.

## Admission and execution state

The API admits native agents with a supported local tool subset. Targets are
`task:ID`, `repository:ID`, or `workspace:ID` within the consented workspace.
Existing agent/team and target access checks apply. Unsupported engines and empty
local tool policies fail. The snapshot preserves host approval requirements and
read-only access; `review` can narrow policy further.

`POST /api/cli/v1/runs` accepts only:

```json
{
  "request_id": "local-task-001",
  "agent_id": "AGENT_ID",
  "target": "task:TASK_ID",
  "instructions": "Fix the issue",
  "execution_location": "local",
  "review": false
}
```

The request identity is unique per OAuth connection. Identical retries return the
same normal run and immutable admission; changed payloads return 409. A reserved
request with no persisted run returns 409 instead of risking duplicate launch or
billing preparation. Inspect the failure before choosing a new request ID.

The trusted internal launch option reuses shared agent materialization, target
context, model resolution, transcript creation, and AI usage preflight. It returns
before runtime agent registration or cloud dispatch. Task delivery preparation is
skipped. Run input records server-owned `execution_location=local` and
`local_execution` provenance; the run table shows Local. Cloud resume, projection,
stale-run reconciliation, and delivery finalizers cannot treat the local run as a
cloud worker run.

Admission returns `run_id`, `agent`, `context`, `allowed_tools`, and `execution`.
The grant contains an ID, run ID, epoch, optional local runtime ID, immutable
policy hash, and lease expiration. This is an authenticated identity record, not
a standalone bearer secret.

| Endpoint beneath `/api/cli/v1/runs/{run_id}` | Behavior |
| --- | --- |
| GET `/execution` | Inspect current grant, including expiry |
| POST `/bind` with `epoch`, `local_run_id` | Bind once to a live epoch; reject a different local run |
| POST `/renew` with `epoch` | Renew for 15 minutes; expired lease advances epoch and clears binding |
| POST `/revoke` | Revoke permanently and cancel the undispatched normal run; repeatable |

All operations recheck connection, user, agent/team, and target access. Atomic
updates reject racing renewal, stale epochs, expired binding, and revoked grants.
Explicit cancellation releases the existing AI usage reservation. Lease expiry
alone permits renewal and does not mark the normal run complete. OAuth logout
blocks all derived grant operations but does not cancel the persisted run; cancel
unused runs first, or cancel them through Helpin afterward.

Local events cannot authorize server mutations, fabricate billable provider
usage, or trigger cloud delivery. Automatic lease management and revalidation of
executable policy on model/tool dispatch must be added with Phase 4 gateways.

## Validation and reproduction

Build the CLI in Agent Runtime, then run:

```sh
# agent-runtime/
go build -o /tmp/agent-runtime-cli ./cmd/agent-runtime-cli
AGENT_RUNTIME_CLI_TEST_BINARY=/tmp/agent-runtime-cli go test ./internal/cli

# helpin/server/
AGENT_RUNTIME_CLI_TEST_BINARY=/tmp/agent-runtime-cli go test ./internal/service -run '^TestCLI'
go test ./...
go vet ./...
go build ./...

# helpin/frontend/
NODE_OPTIONS=--max-old-space-size=4096 pnpm exec tsc -b
pnpm exec vitest run src/pages/oauth/CLIAuthorizePage.test.tsx
```

The binary integration fixture uses real Helpin handlers/service/repository code,
a seeded SQLite workspace, and an explicitly simulated authenticated browser
session. It tests discovery, PKCE consent/exchange, identity, agent listing,
idempotent admission, binding/renewal/revocation, remote logout, and zero cloud
starts. Separate tests cover code/refresh replay, expiry, membership/MFA changes,
cross-workspace targets, read-only policy, usage preflight, malformed payloads,
and sanitized errors. Consent component tests cover loading, invalid requests,
empty workspace choices, explicit selection, and authorization failure.

## Rollback and remaining limits

Set `CLI_ENABLED=false` to disable new and existing CLI API operations. Keep the
additive tables and normal run history. Existing local admissions can still be
cancelled through Helpin. This admission-stage switch is not a graceful gateway
drain policy; Phase 4 must define that before managed execution is enabled.

No npm release, live-provider test, production deployment, or staging task-to-local
result scenario has been performed by this phase. The next gate is managed model
execution, fenced events/artifacts, authoritative usage settlement, and a real
Helpin task completed locally with shared results.
