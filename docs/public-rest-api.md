# Helpin public REST API

**Base URL (Helpin Cloud):** `https://api.helpin.ai/public/v1`
**Self-hosted:** your API host followed by `/public/v1` (set `PUBLIC_API_BASE_URL` so the OpenAPI document advertises it).
**Contract:** [`docs/api/openapi.json`](api/openapi.json), also served live at `/public/v1/openapi.json` (no authentication).

The REST API is a curated, documented view of Helpin for developers. It is not a wrapper around the
web app's internal routes: every operation is an adapter over one public MCP tool, so it inherits the
same security model as [Helpin MCP](public-mcp-server.md).

## What that means

| Concern | Behavior |
| --- | --- |
| Identity | A workspace-bound **automation account** (service principal) token, `hmp_…`. OAuth user tokens issued for MCP also work. |
| Scopes and toolsets | Each operation requires one scope (`x-helpin-scope` in the spec). Tokens, and the workspace policy, limit which scopes and toolsets exist. |
| Read-only | Read-only tokens and read-only workspace policy reject every write with `403 forbidden`. |
| Authorization | Workspace role (RBAC), product-module access, and rollout flags are re-checked on every call. |
| Idempotency | `Idempotency-Key` header on writes; same key and body replays the stored result for 24 hours. |
| Audit | Calls appear in **Settings → MCP access → Activity** with hashed request and result, never raw payloads. |
| Rate limits | Shared per-principal limits (`AUTHENTICATED_RATE_LIMIT_PER_MINUTE`, `EXPENSIVE_RATE_LIMIT_PER_MINUTE`); writes and search count as expensive. `429` carries `Retry-After`. |

## Getting a token

1. A workspace manager opens **Settings → MCP access → Permissions**, enables AI tool access and **Allow automation accounts**, and picks the allowed product areas and scopes.
2. Under **Service accounts**, choose **Create automation account**, pick scopes, product areas, and read-only or read-write, then create a token. It is shown once.
3. Send it as `Authorization: Bearer hmp_…`.

```bash
curl https://api.helpin.ai/public/v1/me -H "Authorization: Bearer $HELPIN_TOKEN"

curl -X POST https://api.helpin.ai/public/v1/tasks \
  -H "Authorization: Bearer $HELPIN_TOKEN" \
  -H "Idempotency-Key: import-2026-09-29-0001" \
  -H "Content-Type: application/json" \
  -d '{"team_id":"…","name":"Investigate slow inbox load","priority":"high"}'
```

## Conventions

- **Envelope.** Success: `{ "data": …, "summary": "…", "links": { … } }`. Error: `{ "error": { "code": "…", "message": "…" } }`.
- **Reads** take filters as query parameters; arrays are comma separated (`owner_member_ids=a,b`). Unknown parameters are rejected.
- **Writes** take a JSON object body. IDs that appear in the URL must not be repeated in the body.
- **Statuses.** `201` for creates, `202` for starting an agent run, `400` invalid input, `401` bad token, `403` not allowed, `404` not found, `409` conflicts (including idempotency reuse and locked or published documents), `422` valid but not applicable, `429` rate limited, `504` 30 second deadline.
- Agent runs are asynchronous: `POST /agent-runs`, then poll `GET /agent-runs/{run_id}`.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `PUBLIC_API_ENABLED` | `true` | Mount `/public/v1`. Authentication still requires `MCP_SERVER_ENABLED` and `MCP_SERVICE_TOKENS_ENABLED`. |
| `PUBLIC_API_BASE_URL` | unset | Origin advertised in the served OpenAPI document. Unset uses `https://api.helpin.ai`. |
| `MCP_*_ENABLED` | `true` | Per-area rollout flags (PM/docs writes, CRM, support, agent runs) apply to the REST API too. |

## Adding or changing an endpoint

The surface is one table: `server/internal/publicapi/routes.go`.

1. Add a `Route` naming an existing public MCP tool. Path parameters must match the tool's argument names; write tools must use a non-`GET` method. `NewHandler` and the tests fail on a mismatch.
2. Regenerate the contract: `UPDATE_OPENAPI=1 go test ./internal/publicapi -run TestCommittedSpecIsCurrent`.
3. Commit `docs/api/openapi.json`. CI fails if it is stale.

A new capability that no MCP tool covers needs the tool first, with its scope, permission, module, and audit behavior; the REST route then comes for free.

## Publishing the reference in Helpin Docs

Helpin Docs can host an OpenAPI reference. In the Help Center space, add an API reference and upload
`docs/api/openapi.json`, or point it at `https://api.helpin.ai/public/v1/openapi.json` with sync enabled.
The document uses OpenAPI 3.1 with only internal `$ref`s, as the importer requires.
