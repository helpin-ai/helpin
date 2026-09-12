# Personal AI connections

Helpin can optionally attach a personal provider credential to a manual agent run
or chat. Existing agents and automations continue using Agent Runtime's keys by
default. The task run panel, Run now dialog, and new chat composer expose the same
connection/model picker. Authentication-paused runs expose a reconnect action.

The Go/Python SDKs handle ChatGPT device login and refresh. Helpin stores API keys,
device sessions, and OAuth tokens encrypted in `ai_connections`, scoped to one user
within one workspace. Agent Runtime receives only a per-run API key/access token;
it does not own permanent ChatGPT connections or refresh tokens. Teammates and
scheduled root runs cannot use another user's personal connection. Descendants
keep the original owner's connection/model. A new independent run/chat is required
to change them.

Helpin AI credits **continue to apply at the usual full rate**, including when
inference is funded by the customer's API key or subscription. The ledger uses
`customer_funded_platform` and records the selected model's pricing identity.
The picker and connection dialog disclose this before launch.

Configuration:

- Apply `server/internal/dbmigrate/sql/202609120002_personal_ai_connections.sql` using the migration command. The earlier retired-run disposition gate remains a prerequisite if it is still pending.
- Set `AI_CONNECTION_ENCRYPTION_KEY` to a stable 32-byte key (raw, hexadecimal, or base64). Without it personal connections remain disabled.
- Set the runtime's separate `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY` on its API and every worker (raw 32 bytes or base64).
- Configure the runtime app's `model_credential_callback` URL to `/api/internal/agent-runtime/model-credentials/refresh` and its token environment variable to Helpin's `INTERNAL_API_SECRET` value.
- Keep `CHATGPT_CONNECTIONS_ENABLED=false` and the runtime's `AGENT_RUNTIME_CHATGPT_ENABLED=false` until separate live account/deployment validation is approved and passes. `CHATGPT_OAUTH_CLIENT_ID` optionally overrides the SDK's public client ID.

The callback checks connection/run ownership, active membership, provider/account,
and runtime mapping. Row locks serialize refresh; rejected-token fingerprints
avoid repeated rotation by concurrent workers. Reconnect updates active run
credentials and resumes matching model-authentication interactions. Disconnect
clears the saved secret and revokes bound run credentials; failures are retryable.
A post-launch check covers disconnects racing admission. Requests already in flight
may finish; revocation prevents subsequent model requests.

Validation includes encrypted storage/ownership tests, concurrent refresh,
revoked membership, manual-only admission, normal and estimated credit charges,
existing chat/run-view tests, TypeScript checks, the production frontend build, and isolated browser checks with
mocked connection endpoints. A real OpenAI per-run credential smoke passed with the
global key unset. The SDK initiated a real device-login request, but consent,
subscription inference, and live token refresh have not been validated. Automatic
approval review requires separate approval for that test and SDK branch publication.

The server pins SDK commit `2e2b4985086b` using
`v0.5.1-0.20260912224056-2e2b4985086b`. Publish that SDK feature branch before deploying
consumers; the committed dependency was validated locally from the exact SDK commit,
with Go workspace substitution disabled. No production deploy or retirement
inventory was performed by this feature.
