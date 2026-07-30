# External MCP servers for Helpin agents

External MCP servers let a Helpin workspace connect systems such as Customer.io and make selected remote tools available to selected Helpin agents. This is the outbound MCP path: Helpin and Agent Runtime are the MCP client.

This is separate from [Helpin Public MCP](./HELPIN_PUBLIC_MCP.md), where Claude, ChatGPT, Cursor, or another outside client connects into Helpin.

## Ownership and trust boundary

| Component | Responsibility |
| --- | --- |
| Helpin app | Workspace installation, settings UI, provider presets, OAuth browser redirects, encrypted refresh/static credentials, tool discovery, workspace policy, notifications, and audit bindings |
| Helpin agent | Stores MCP tool aliases in `allowed_tools`, alongside built-in tools |
| Helpin run launcher | Resolves those aliases against the current workspace, refreshes OAuth when needed, and sends an exact server/tool attachment for the new run |
| Agent Runtime | Validates the attachment, encrypts the run credential with run/server-bound AAD, exposes only the attached tools, applies read/write approval policy, pauses on authentication failure, and destroys the credential when the run terminates |
| Go/Python runtime SDKs | Typed run-scoped MCP request/rotation helpers plus headless MCP OAuth discovery, PKCE, registration, exchange, and refresh clients |

Durable OAuth credentials do not belong in Agent Runtime. Agent Runtime receives only a current access token or static header set for one run. It cannot use a credential from one app/run/server tuple in another tuple.

## Workspace manager flow

1. Open **Settings → Model Context Protocol → External servers**.
2. Select Customer.io US/EU, or an operator-approved custom server when custom servers are enabled.
3. Choose the OAuth scopes. Customer.io starts with `read`; sensitive reads and writes are opt-in.
4. Continue to the provider's browser consent page.
5. After the callback, Helpin exchanges the authorization code using PKCE S256, stores encrypted tokens, connects to the MCP endpoint, and discovers tools.
6. Review each discovered tool. A manager can disable it or classify it as read/write.
7. Open an agent version and select tools from the **External MCP** category under **Allowed tools**.
8. Start an ordinary Helpin run. Only the selected servers and selected tools are attached to that run.

An agent never receives every tool merely because a server is installed in the workspace. Installation, workspace tool enablement, agent `allowed_tools`, and the run attachment are separate gates.

## Tool names and mutation policy

The saved/runtime alias is deterministic:

```text
mcp__<server_name>__<remote_tool_name>
```

For example:

```text
mcp__customer_io_us__cio_read_api
```

The runtime attachment sends the remote name (`cio_read_api`) plus its `read` or `write` classification. Write-classified calls participate in Agent Runtime's mutating-tool approval policy.

Remote MCP annotations are hints, not authority. New custom tools default to `write` unless `readOnlyHint` is explicitly present. Helpin also maintains a reviewed Customer.io classification so `cio_write_api` and `cio_delete_api` remain mutating even if annotations are absent.

## Browser OAuth lifecycle

Helpin implements the MCP authorization discovery sequence:

1. Read `resource_metadata` from a `WWW-Authenticate` challenge when provided.
2. Try path-specific and root OAuth Protected Resource Metadata.
3. Discover OAuth Authorization Server Metadata, with OpenID metadata as a fallback.
4. Require PKCE S256.
5. Use a configured OAuth client or Dynamic Client Registration.
6. Include the MCP resource audience in authorization, code exchange, and refresh requests.
7. Bind a random, hashed, single-use, ten-minute OAuth state to the workspace, user, server, return path, and encrypted PKCE verifier.

Normal access-token expiry is refreshed silently before tool discovery or a run. Refresh rotation is serialized with a database row lock so two Helpin instances do not race a rotating refresh token.

If refresh is rejected or the provider revokes consent:

- the installation becomes `reauthorization_required`;
- the authorizing workspace user receives one deduplicated in-app notification per incident;
- new runs using that server are rejected before launch;
- a running Agent Runtime run pauses with an `authentication` interaction rather than losing its run credential context;
- reconnecting rotates the existing run/server credential and resumes runs paused for that authentication incident.

A provider `403` is not treated as expired OAuth. It becomes `remote_disabled`, because Customer.io uses this response when MCP is unavailable or disabled for the workspace/user.

## Customer.io pilot

The preset follows [Customer.io's current MCP setup documentation](https://docs.customer.io/ai/mcp/get-started/), including regional endpoints, scopes, and the verb-separated tool surface.

| Region | MCP endpoint |
| --- | --- |
| US | `https://mcp.customer.io/mcp` |
| EU | `https://mcp-eu.customer.io/mcp` |

Supported scope choices:

- `read` (default)
- `read:sensitive`
- `write`
- `write:live`
- `configure`

The pilot recognizes these Customer.io tools:

- Read: `cio_prime`, `cio_schema`, `cio_read_api`, `cio_skills_list`, `cio_skills_read`, `cio_auth_status`
- Write: `cio_write_api`, `cio_delete_api`

Provider capabilities can evolve; use **Refresh tools** after Customer.io adds or removes tools, then review the resulting workspace policy before enabling new ones on agents.

## Custom servers

Custom servers require both:

- `EXTERNAL_MCP_CUSTOM_SERVERS_ENABLED=true`; and
- an endpoint host matching `EXTERNAL_MCP_ALLOWED_HOSTS`.

Supported V1 transport/auth combinations are Streamable HTTP with browser OAuth, bearer token, custom credential headers, or no authentication. The settings UI never reads a credential back after creation.

Outbound protections include HTTPS by default, URL userinfo/fragment rejection, an explicit host allowlist, same-site or explicitly allowlisted discovered OAuth URLs, single-resolution dialing to prevent DNS rebinding, checks against private/loopback/link-local/multicast/shared addresses, no environment proxy, bounded redirects, pagination/tool limits, timeouts, and response/body limits. Insecure localhost is a test-only opt-in. Add a separate OAuth authorization host to `EXTERNAL_MCP_ALLOWED_HOSTS` when a custom server intentionally delegates authorization outside its own registrable site.

## Runtime wire contract for apps

Apps using Agent Runtime may implement their own installation UI/storage or use their app SDK. At run creation they send only the servers selected for that run:

The Go SDK exposes `github.com/helpin-ai/agent-runtime-go/mcpauth`; the Python SDK exposes `MCPOAuthClient`. These helpers own protocol mechanics, not product policy. An app must still supply workspace/user authorization, callback routes, encrypted single-use state and refresh-token storage, tool discovery/review, notifications, and run bindings. Helpin imports the Go SDK OAuth helper and typed run/rotation contracts, then supplies all of those application-owned pieces here as the reference architecture.

While developing the SDK and Helpin together before an SDK release, use a local Go workspace instead of committing a filesystem `replace` directive:

```bash
go work init ./helpin/server ./agent-runtime-go
GOWORK="$PWD/go.work" go test ./helpin/server/...
```

The committed `server/go.mod` deliberately does not contain a local filesystem `replace`. Until the SDK version containing `mcpauth` and the run-scoped MCP types is published and pinned, build this Helpin branch through the local workspace override. Publish and bump the SDK before merging or deploying without that override.

```json
{
  "agent_id": "agent-id",
  "target": { "type": "workspace", "id": "workspace-id" },
  "mcp_servers": [
    {
      "server_id": "workspace-installation-id",
      "server_name": "customer_io_us",
      "transport": "streamable_http",
      "url": "https://mcp.customer.io/mcp",
      "tools": [
        { "name": "cio_read_api", "access": "read" }
      ],
      "credential": {
        "type": "bearer_token",
        "access_token": "short-lived-access-token",
        "expires_at": "2026-07-30T12:00:00Z"
      }
    }
  ]
}
```

Do not send a refresh token. When a paused nonterminal run needs a replacement credential, refresh in the app and call:

```text
PUT /v1/runs/{run_id}/mcp-servers/{server_id}/credential?app_id={app_id}
```

with:

```json
{
  "credential": {
    "type": "bearer_token",
    "access_token": "replacement-access-token",
    "expires_at": "2026-07-30T13:00:00Z"
  }
}
```

The endpoint cannot change server identity, URL, transport, or tool policy. Its response contains only run/server/expiry/update metadata and never echoes the credential. Then resume the run with the `auth_completed` intent.

For full validation, limits, SDK examples, and credential cleanup behavior, see Agent Runtime's `docs/run-scoped-mcp.md`.

## Helpin API routes

All management routes use normal Helpin session authentication, workspace membership, and settings permissions.

| Route | Purpose |
| --- | --- |
| `GET /api/external-mcp/providers` | Rollout status and allowed provider presets |
| `GET /api/external-mcp/servers` | Workspace installations and sanitized discovered tools |
| `POST /api/external-mcp/servers` | Create an installation; static credentials are write-only |
| `PUT /api/external-mcp/servers/{id}` | Rename or enable/disable |
| `DELETE /api/external-mcp/servers/{id}` | Delete installation, credential, tools, OAuth states, and run binding rows |
| `POST /api/external-mcp/servers/{id}/oauth/start` | Discover OAuth, persist state, and return the provider authorization URL |
| `GET /api/external-mcp/oauth/callback` | Consume state, exchange code, sync tools, and return to settings |
| `POST /api/external-mcp/servers/{id}/tools/refresh` | Refresh OAuth if required and rediscover tools |
| `PUT /api/external-mcp/servers/{id}/tools` | Replace enabled/read-write workspace policy for discovered tools |

Mutation bodies are strict JSON with unknown-field rejection and a 128 KiB limit. Workspace identifiers come from middleware, not request bodies.

## Operator configuration

| Environment variable | Meaning |
| --- | --- |
| `EXTERNAL_MCP_ENABLED` | Global outbound MCP rollout switch; default `false` |
| `EXTERNAL_MCP_CUSTOM_SERVERS_ENABLED` | Allows custom endpoints; default `false` |
| `EXTERNAL_MCP_ENCRYPTION_KEY` | Required when enabled; 32 raw bytes, 64 hex characters, or base64-encoded 32 bytes |
| `EXTERNAL_MCP_ALLOWED_HOSTS` | Comma-separated exact/wildcard endpoint hosts; defaults to the Customer.io US/EU hosts |
| `EXTERNAL_MCP_OAUTH_REDIRECT_URL` | Public Helpin API callback URL |
| `EXTERNAL_MCP_OAUTH_CLIENT_ID` | Optional pre-registered OAuth client ID; otherwise DCR is used |
| `EXTERNAL_MCP_OAUTH_CLIENT_SECRET` | Optional pre-registered client secret |
| `EXTERNAL_MCP_OAUTH_CLIENT_AUTH_METHOD` | `none`, `client_secret_basic`, or `client_secret_post` |
| `EXTERNAL_MCP_ALLOW_INSECURE_LOCALHOST` | Local automated tests only; never enable in shared environments |

Agent Runtime must independently allow the same MCP hosts and have its run-credential encryption key configured. See its run-scoped MCP documentation for `AGENT_RUNTIME_MCP_*` variables.

The Kubernetes stage/prod manifests explicitly keep outbound MCP and custom servers off. Enabling requires applying migration `202607300001_external_mcp_servers.sql`, setting secrets/configuration, confirming the public callback, configuring Agent Runtime's host allowlist, and changing the reviewed rollout switch.

## End-to-end Customer.io verification

1. Deploy Helpin and Agent Runtime with their independent 32-byte MCP encryption keys and Customer.io host allowlists.
2. Confirm `GET /api/external-mcp/providers` returns `enabled: true`.
3. Add Customer.io US or EU in workspace settings and complete browser consent.
4. Confirm status is `connected`, no token fields appear in network responses, and the expected tools are discovered.
5. Leave only `cio_read_api` enabled and add that External MCP tool to a test agent.
6. Start a run asking the agent for a harmless Customer.io read. Verify Helpin records one `agent_run_external_mcp_binding` and Agent Runtime exposes only the selected alias.
7. Enable `cio_write_api`, add it to the agent, and verify a run follows the mutating-tool approval policy.
8. Revoke Customer.io consent (or invalidate the refresh credential), let the access token expire, and start/use another run. Verify Helpin marks `reauthorization_required`, sends one notification, and Agent Runtime pauses an in-flight run with `pause_reason=authentication`.
9. Reconnect from settings. Verify Helpin rotates the credential without changing the run's URL/tools and resumes the paused run.
10. Disable the installation. Verify new runs using its aliases fail before Agent Runtime launch.

Automated coverage should accompany this manual flow: outbound URL validation, OAuth metadata/PKCE/resource audience, single-use state, encryption AAD, refresh rotation, workspace scoping, strict request bodies, credential non-disclosure, Customer.io tool classification, exact run attachments, runtime 401 pause, rotation, and terminal credential cleanup.
