# Local Agent Runtime Setup

This runbook is for testing delegated Helpin runs against a local `agent-runtime`.
It is intentionally local-only; staging and production app config are managed as
runtime secrets and must merge Helpin with any existing app entries. For the
staging rollout (Doppler secrets, NATS sharing, flag order, rollback), see
`docs/AGENT_RUNTIME_STAGING.md`.

## Helpin Env

Use `127.0.0.1` for the runtime URL. On some machines Go resolves `localhost`
to IPv6 first while the runtime listens on IPv4.

```bash
AGENT_RUNTIME_BASE_URL=http://127.0.0.1:8090
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_ID=helpin
AGENT_RUNTIME_LAUNCH_ENABLED=true
```

`AGENT_RUNTIME_LAUNCH_ENABLED=true` delegates these run surfaces to Agent Runtime:

- marketer preset: workspace targets
- documentation_agent preset: workspace + document targets
- crm_operator preset: workspace + crm_contact / crm_company / crm_deal targets
- custom agents (no preset, `runtime_kind = native_sdk`): workspace / document / crm_* targets

Task, story, epic, repository, support_conversation, and support_coverage_gap
targets — and all other presets (code_builder, review_agent, epic_planner,
task_planner, support_agent, command_agent) — continue through the in-process
Temporal executor. Codex/opencode custom agents also stay local (their auth and
interaction handling live in the local executor). The predicate is the
preset→target map in `server/internal/service/agent.go`
(`agentRuntimePresetDelegatedTargets` / `agentRuntimeCustomAgentDelegatedTargets`).

## Runtime App Config

Create a local app config whose callback URLs point at the running Helpin API
port. If Helpin is on `:8080`, use:

```json
{
  "apps": [
    {
      "app_id": "helpin",
      "context_endpoint": "http://127.0.0.1:8080/api/internal/agent-runtime/target-context",
      "context_token": "dev-token",
      "workspace_provider": {
        "transport": "repository",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/workspace",
        "token": "dev-token",
        "root_dir": ".local/workspaces"
      },
      "command_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/commands",
        "token": "dev-token"
      },
      "skill_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skills",
        "package_base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skill-packages",
        "token": "dev-token"
      }
    }
  ]
}
```

If Helpin is running on another port, update all three callback URLs. The runtime
must send the same bearer token as `AGENT_RUNTIME_SERVICE_TOKEN`.

## Runtime Env

Example `agent-runtime` local env:

```bash
AGENT_RUNTIME_ADDR=:8090
AGENT_RUNTIME_STORE_DRIVER=sqlite
AGENT_RUNTIME_SQLITE_DSN=.local/helpin-agent-runtime.sqlite3
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_CONFIG=@/tmp/helpin-app-config.json
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE_PREFIX=helpin-
AGENT_RUNTIME_EVENT_SINK=log,nats
AGENT_RUNTIME_NATS_URL=nats://127.0.0.1:4222
CODEX_APP_SERVER=true
CODEX_PATH=codex
```

`CODEX_APP_SERVER=true` is required when using the normal Codex CLI binary. Raw
command mode starts the interactive TUI and fails in non-TTY worker execution.

Start the runtime from the `agent-runtime` checkout:

```bash
set -a
source .env.helpin-local
set +a
GOCACHE=/private/tmp/agent-runtime-go-cache go run ./cmd/agent-runtime
```

Start a local NATS server with JetStream before running the Helpin worker and
runtime. Live projection uses the `AGENT_RUNTIME_EVENTS` stream; the
reconciliation sweep is only a backstop.

```bash
nats-server -js
```

## Smoke Check

1. Start NATS with JetStream.
2. Start Helpin API and the Temporal worker with the Helpin env above.
3. Start `agent-runtime` with the app config above.
4. Launch a Mira workspace run.
5. Confirm the Helpin `agent_runs` row has:
   - `external_runtime = 'agent-runtime'`
   - `external_runtime_id` set
   - no Helpin `workflow_id`
6. Confirm projection updates the row from runtime events. The worker creates
   the `AGENT_RUNTIME_EVENTS` stream if it is missing.

For local Codex auth flows, a successful unauthenticated smoke can pause with
`pause_reason = authentication`; that still validates launch delegation,
runtime execution, and Helpin projection.

## Marketer Workspace Parity Row

Current pilot surface: marketer-preset workspace runs only.

| Capability | Status | Notes |
| --- | --- | --- |
| Launch routing | Green | `AGENT_RUNTIME_LAUNCH_ENABLED=true` delegates marketer-preset workspace runs to Agent Runtime and stamps `external_runtime` / `external_runtime_id` at launch. The predicate is preset-based, not name-based. |
| Runtime orchestration | Green | Helpin requests `execution_mode=durable`; Agent Runtime owns the Temporal workflow. Helpin does not create a local run workflow for delegated runs. |
| Projection | Green | NATS projection updates lifecycle, usage, artifacts, messages, and interactions back into Helpin. |
| Codex auth start/cancel | Green | Delegated Codex auth calls Agent Runtime's `/v1/runs/{id}/codex-auth/device-code/*` endpoints so auth is promoted into the runtime Codex home. |
| Codex auth completion | Yellow | Runtime emits `codex_auth.state_changed`; Helpin mirrors the state and forwards `auth_completed` when the runtime reports `connected`. Local smoke requires the active Helpin worker to be running this projection code. |
| Handoff | Deferred | `HandoffRun` still follows the local Temporal path. Attach it to Agent Runtime when the first delegated surface needs cross-agent/user handoff semantics. |
| Post-auth assistant turn | Blocked without browser login | Requires completing the OpenAI device-code login for the runtime-issued code. |
| Product finalizers | Not applicable | Workspace Mira run has no task/support/repository finalizer. |
| Repository delivery | Not applicable | Workspace Mira run does not prepare or push a repository workspace. |

## Delegated Surface Parity Rows

One row per surface delegated beyond the marketer pilot. Columns map to the
launch pipeline stages; evidence is the code path that exists today.

| Surface | Launch routing | Target context | Tools | Write path | Finalizers | Transcript |
| --- | --- | --- | --- | --- | --- | --- |
| documentation_agent / workspace | Green — preset→target map delegates; `startTargetRunWithOptions` workspace case stamps `workspace_id` metadata via `runtimeStartRunRequest`. | Green — host `workspace` case returns workspace context data. | Green — docs read tools cataloged; docs/publish product tools are command-backed via the commands provider endpoint. | Amber — document writes (`write_document_content`, `publish_document_change_proposal`) are command-backed but not yet E2E-smoked from a delegated run. | Green — agent-idle + automation completed-rules finalizers fire on terminal projection. | Green — NATS projection mirrors messages/interactions into Helpin. |
| documentation_agent / document | Green — predicate delegates; `document` launch case validates `doc.WorkspaceID == workspaceID` before `createRun`, so `metadata.workspace_id` is always set. | Green — host `document` case resolves the doc and stamps `workspace_id`; type string `document` matches what Helpin stamps. | Green — same command-backed docs tool set as workspace runs. | Amber — document-target write flows not yet E2E-smoked against a live runtime. | Green — agent-idle + completed-rules; no doc-specific finalizer exists (none needed yet). | Green — same projection path as marketer pilot. |
| crm_operator / workspace | Green — preset→target map delegates workspace runs. | Green — host `workspace` case. | Green — CRM product tools (`add_deal_note`, `update_deal_stage`, `ensure_crm_contact_company`, `enrich_crm_*`) are command-backed. | Amber — CRM command writes exist but no delegated-run smoke has exercised them yet. | Green — agent-idle + completed-rules; `crm.deal_review_actions` flow output is validated by the planning finalizer. | Green — same projection path. |
| crm_operator / crm_contact + crm_deal | Green — predicate delegates; `crm_contact` / `crm_deal` launch cases validate workspace ownership before `createRun`. | Green — host `crm_contact` / `crm_deal` cases; type strings match Helpin's stamps exactly. | Green — command-backed CRM tools with target-aware defaults (e.g. enrich uses run target ID). | Amber — same as above: command-backed, not E2E-smoked. | Green — agent-idle + completed-rules. | Green — same projection path. |
| crm_operator / crm_company | Amber — predicate delegates `crm_company`, but `startTargetRunWithOptions` has no `crm_company` launch case yet (`unsupported target type`), so no Helpin path can create such a run today; predicate is forward-ready only. | Green — host `crm_company` case already resolves company context. | Green — `enrich_crm_company` command supports `crm_company` targets. | Amber — unreachable until the launch case lands. | Green — generic finalizers would apply once reachable. | Green — projection is target-agnostic. |
| custom native_sdk agents / workspace + document + crm_* | Green — custom agents with no preset and `runtime_kind = native_sdk` delegate on these targets; codex/opencode custom agents intentionally stay local. | Green — same host target-context cases as preset runs. | Amber — tool surface is whatever the agent's `allowed_tools` grants; scanners/preview/github built-ins plus command-backed product tools exist, but per-agent tool grants are not preset-curated. | Amber — write commands available but no custom-agent delegated smoke yet. | Green — agent-idle + completed-rules are agent-agnostic. | Green — same projection path. |
