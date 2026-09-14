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
global key unset. The September 13 test-system run
`run_9b1445790f13e52c710f2c3d` (Helpin run
`086a992f-a4ca-479b-8923-67923861fa9d`) used `openai_chatgpt` / `gpt-5.6-terra`
with app-owned OAuth credentials and produced 10 model responses and 37 tool calls
before a Git tool stalled. This later observation supersedes the September 12
blocked inference test. Live token refresh, reconnect, and revocation remain
separate release gates; successful tool continuation does not prove lossless
ChatGPT response replay.

Helpin and Runtime pin the published SDK release `v0.6.0-alpha.1`; builds and
integration tests were verified without Go workspace substitution. The SDK adds
typed run model controls and shared provider/control validation. An explicit empty
controls object clears inherited model controls while preserving agent execution
limits. No production deployment or credential-policy activation is implied by
this dependency update.
