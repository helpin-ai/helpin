# Connect external A2A agents

This guide is for administrators and contributors who connect remote agents
that speak the [Agent2Agent (A2A) protocol](https://a2a-protocol.org/) to a
Helpin workspace. It explains the setup, the server configuration, how a task
flows to the remote agent and back, and the security boundaries. It describes
the behavior implemented on the `feat/a2a-external-agents` branch; availability
in a release depends on that branch shipping.

An external agent appears in Helpin as an ordinary agent that can be assigned
tasks, mentioned in comments, and started from a task. Its runs do not use a
model, AI profile, tools, or skills: Agent Runtime forwards the task to the
remote agent and reports its progress back.

## Components and responsibilities

| Component | Responsibility |
| --- | --- |
| Helpin API | Stores the connection (`external_a2a_agents`) with the bearer token encrypted, creates the linked agent (`runtime_kind: a2a`), hands per-turn connection details to Agent Runtime, projects remote task state onto the task, and receives uploaded files |
| Agent Runtime | Runs the A2A exchange: sends the task as an A2A message, follows the remote task until it completes or asks for input, and emits `a2a.task` events |
| Remote agent | Any A2A v1.0 (or v0.3) agent that publishes an agent card |

The same run lifecycle as other agents applies: `agent_runs` records the run,
Agent Runtime is the executor, and events are projected through the normal
run-event path.

## Configure the server

| Variable | Required | Purpose |
| --- | --- | --- |
| `EXTERNAL_A2A_ENCRYPTION_KEY` | Yes | 32 raw bytes, 64 hex characters, or base64-encoded 32 bytes. Encrypts agent tokens with AES-256-GCM bound to the workspace and connection. Without it every external agents endpoint returns HTTP 503 “External agents are not configured on this server”. |
| `EXTERNAL_A2A_ALLOWED_PRIVATE_HOSTS` | No | Comma-separated exact hosts or `*.domain` patterns that may resolve to private, loopback, or link-local addresses and may use plain HTTP. Use it for agents on an internal network. Every other host must be public and use HTTPS. |
| `PUBLIC_API_BASE_URL` | Recommended | Externally reachable API origin used in upload links. Falls back to `CLI_PUBLIC_BASE_URL`, then `APP_BASE_URL`. The remote agent must be able to reach `<origin>/api/a2a/uploads`. |

The variables are listed in [server/.env.example](../server/.env.example).
Generate a key with `openssl rand -hex 32`. Changing the key makes stored tokens
unreadable; re-enter each agent's token after a rotation.

Apply the versioned migration `202609300001_external_a2a_agents.sql` (for
example with `go run ./cmd/migrate up` from `server/`). It creates the
connection, task-context, upload-token, and projection tables and adds
`pm_attachments.uploaded_by_agent_id`.

## Connect an agent

Workspace members with settings management permission can connect agents.

1. Open **Settings → External agents** and choose to add an agent.
2. Enter the agent card URL. A base URL such as `https://hermes.example.com`
   is accepted; Helpin appends `/.well-known/agent-card.json`.
3. Enter the bearer token the remote agent expects, if it requires one.
4. Preview the card. Helpin fetches it, validates the name and interface, and
   shows the skills and capabilities without saving anything.
5. Optionally restrict the agent to teams, then save.

Helpin prefers the card's `JSONRPC` interface and otherwise uses the first
listed interface. v0.3 cards with a top-level `url` and `preferredTransport`
are accepted. Saving creates a Helpin agent with the card's name, trigger mode
`auto_on_assignment`, and `execution_config.external_a2a_agent_id` pointing at
the connection. **Refresh card** re-reads the card; a failure marks the
connection `error` and keeps the last good card. **Disable** stops new
connection details from being issued. **Delete** deletes the linked agent,
which removes the connection, its task contexts, and upload links.

The token is never returned by the API; responses include only `token_hint`
(the last four characters). The generic agent editor cannot change an external
agent's model, tools, skills, instructions, or runtime.

### REST API

All routes are under `/api/workspaces/{workspaceID}/external-agents`. Reads need
settings read permission; writes need settings management permission.

| Method and path | Result |
| --- | --- |
| `GET /` | `{ "items": [ExternalAgent] }` |
| `POST /preview` with `{card_url, token}` | `{ "card": AgentCardSummary }`; nothing is saved |
| `POST /` with `{card_url, token, allowed_team_ids?}` | `201` with the `ExternalAgent` |
| `PATCH /{id}` with `{name?, token?, allowed_team_ids?, status?}` | Updated `ExternalAgent`; `status` is `active` or `disabled`, and an empty `token` clears it |
| `POST /{id}/refresh-card` | Updated `ExternalAgent` |
| `DELETE /{id}` | `204` |

Errors use `{ "error": "message" }`.

## How a task flows

1. **Start.** Assigning a task to an agent whose trigger mode is
   `auto_on_assignment` starts a task run, unless that agent already has an
   active run on the task. This applies to every agent with that trigger mode.
   A run can also be started manually or by an automation rule. The task needs
   no repository: external agents get no Helpin tools. A disabled connection, or
   one whose card refresh failed, starts no run; assigning or starting one is
   refused with "external agent <name> is disabled; enable it in Settings →
   External agents".
2. **Connection details.** At the start of every turn Agent Runtime asks Helpin
   for the target context. For a non-terminal run of an active external agent
   in the same workspace, Helpin adds `data.a2a`: the connection, the decrypted
   token, the cached card, the remote `context_id` from earlier runs on the same
   task, the upload instructions, whether private networks are allowed, and a
   110-minute turn limit. The runtime removes it from the context before
   anything else sees it. A run cancelled within the last ten minutes still
   receives the connection (without upload instructions) so Agent Runtime can
   cancel the remote task.
3. **Message.** The first message is the task launch context plus the upload
   instructions. Later messages are the human replies.
4. **Progress.** Agent Runtime emits an `a2a.task` event on each remote state
   change. Helpin stores the remote `context_id`, posts the agent's question
   (`input_required`, `auth_required`) or final answer (`completed`) as a task
   comment attributed to the agent, and imports files the agent publishes as
   URL artifacts as task attachments attributed to the agent.
5. **Replies.** A human comment on the task reaches the external agent when the
   task is assigned to it, the comment mentions it by name (for example
   `@Hermes`), or the comment replies to one of its comments. If the agent's
   latest run on the task is waiting for input, the comment text resumes that
   run. If the comment mentions the agent and it has no active run, a new run
   starts and continues the stored remote conversation.
6. **End.** When the run completes, fails, or is cancelled, its upload links are
   revoked.

Event projection is idempotent: a replayed `a2a.task` event does not post a
second comment for the same message or import the same file URL twice.

## Upload link

Each run gets an upload token, derived from the server key so only its SHA-256
is stored. It is valid until the run ends or for 24 hours. The first message
tells the agent how to use it:

```bash
curl -fsS -X POST -H 'Authorization: Bearer <token>' -F 'file=@demo.mp4' \
  https://api.example.com/api/a2a/uploads
```

A successful upload returns `201` with `{ "id", "filename", "size" }` and adds
the file to the run's task. The endpoint accepts one multipart `file` field.

| Limit | Value |
| --- | --- |
| File types | mp4, webm, png, jpeg, gif, pdf, txt, log, json, csv, zip; the content must match the extension |
| Size per file | 200 MB (`413` above it) |
| Total per run | 2 GB |
| Token | `401` when missing, unknown, expired, revoked, or the run has ended |

Uploads are streamed to a temporary file rather than held in memory. The API
server's 5-minute read timeout bounds slow uploads.

## Connect Hermes

Enable the `a2a` gateway platform in Hermes with a per-peer token, for example
`A2A_PEER_TOKENS=helpin:<token>`, `A2A_TRUSTED_PEERS=helpin`, `A2A_HOST=0.0.0.0`
and `A2A_PUBLIC_URL` set to the address Helpin reaches. Add it in Helpin with
the card URL and the same token. Hermes behaves as follows:

- It advertises streaming, so Agent Runtime streams each turn and a cancelled
  run cancels the Hermes task.
- A reply starting with `[INPUT_REQUIRED]` is a question: the run pauses and the
  next human comment on the task answers it. Mention this marker in the task
  description when you want Hermes to ask before it acts.
- It runs tasks in one live session. A task sent while another is working
  interrupts it, and the interrupted task ends with "Hermes finished without a
  reply". Give Hermes one task at a time.
- It rejects a conversation after `A2A_MAX_PINGPONG_TURNS` turns (default 5,
  at most 20) and fails a task after `A2A_REPLY_TIMEOUT` seconds (default 300).
  A task's conversation spans its runs and follow-up comments, so raise the
  turn limit for long threads.
- It returns text only. It attaches files by running the upload command from
  the first message, which needs the upload link to be reachable from Hermes.

## Security

- Card fetches and file downloads use a client that resolves the host itself
  and refuses private, loopback, link-local, and shared (100.64.0.0/10)
  addresses unless the host is in `EXTERNAL_A2A_ALLOWED_PRIVATE_HOSTS`.
  Redirects are limited to three and re-validated; the bearer token is sent only
  to the agent's own interface host.
- Tokens are encrypted at rest, never serialized, and only leave Helpin in the
  target context of a run of that agent, over the internal API secret.
- `runtime_kind: a2a` is reserved for agents created by connecting an external
  agent; the generic agent API rejects it.
- External agent runs reserve no AI usage and use no AI profile or model
  credential.
- Imported and uploaded files use the same MIME allowlist and private storage
  as other task attachments.

## Code entry points

| Area | Path |
| --- | --- |
| Card parsing and SSRF-safe client | `server/internal/externala2a/` |
| Connection management and target context | `server/internal/service/external_a2a.go`, `external_a2a_runtime.go` |
| Uploads and file import | `server/internal/service/external_a2a_upload.go`, `external_a2a_projection.go` |
| Comment routing | `server/internal/service/external_a2a_comments.go` |
| Assignment trigger | `server/internal/service/pm_task_assignment_run.go`, `agent_external_a2a.go` |
| HTTP handlers | `server/internal/handler/external_a2a.go` |
| Schema | `server/internal/dbmigrate/sql/202609300001_external_a2a_agents.sql` |
