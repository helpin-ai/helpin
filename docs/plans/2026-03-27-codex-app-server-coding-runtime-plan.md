# Codex App-Server Coding Runtime Plan

## Status

Draft plan for a Helpin-owned coding frontend backed by Codex app-server instead of the current fire-and-forget `codex exec` adapter.

This plan is based on:

- the current Helpin Codex shell-out adapter in [server/internal/worker/codex.go](/root/teampulse/server/internal/worker/codex.go)
- the current agent preset/runtime model in [docs/AGENTS_AND_AUTOMATION.md](/root/teampulse/docs/AGENTS_AND_AUTOMATION.md)
- the existing Codex rollout doc in [docs/plans/2026-03-25-codex-opencode-coder-runtime-plan.md](/root/teampulse/docs/plans/2026-03-25-codex-opencode-coder-runtime-plan.md)
- the locally cloned Codex repo evaluated during planning under `/tmp/codex`

## Goal

Introduce a stateful Codex runtime for coding sessions where:

- Helpin owns the user-facing React experience
- Helpin remains the source of truth for `agent_runs`, approvals, pause/resume, and audit state
- Codex provides the local coding runtime: shell, patching, review, thread memory, sandboxing, and streaming turn state

## Scope

Initial scope:

- `code_builder`
- `review_agent`

Out of scope for this plan:

- replacing `native_sdk` for `epic_planner`, `story_planner`, `crm_operator`, or `support_agent`
- making all Helpin agents run on Codex
- direct browser-to-Codex transport
- committing to Helpin-managed ChatGPT OAuth as the default auth path

## Recommendation

Use Codex app-server as the primary runtime seam for coding sessions.

Do not treat `codex exec --full-auto` as the final architecture for interactive or approval-heavy coding flows.

Keep `opencode` as an optional coder runtime during rollout.

## Recommended Shape

```text
React coding UI
  -> Helpin API / WebSocket
    -> Temporal agent_run workflow
      -> Codex Runtime Manager (Go)
        -> spawn codex-app-server per active coding session
           with isolated CODEX_HOME + repo workdir
        -> JSON-RPC over stdio
        -> thread create/resume + turn start
        <- stream items / plan deltas / terminal / approvals
    <- normalized run events to UI
```

This is the cleanest upgrade from the current shell-out adapter in [server/internal/worker/codex.go](/root/teampulse/server/internal/worker/codex.go). Instead of `codex exec --full-auto`, Helpin runs the stateful app-server from `/tmp/codex/codex-rs/app-server/src/main.rs` and owns the frontend and durable workflow model.

## Backend Split

- Helpin remains system-of-record for `agent_runs`, approvals, pause/resume, messages, and operator-visible audit state.
- Codex remains the local coding runtime for repo-aware execution.
- A new Go component such as `CodexSessionHost` owns process lifecycle and protocol translation.

`CodexSessionHost` responsibilities:

- spawn `codex-app-server --listen stdio://`
- set per-run or per-session `CODEX_HOME`
- set repo workdir and runtime config
- initialize auth, model, sandbox, and thread state
- translate app-server notifications into Helpin stream events, persisted messages, and artifacts
- block on approval and user-input requests until Temporal signals or UI actions arrive
- shut down, resume, or reconnect the Codex session cleanly

## Frontend Contract

The React UI should talk only to Helpin, not directly to Codex.

Primary surfaces:

- transcript
- live terminal and runtime output
- changed files and diff preview
- plan/checklist stream
- approval cards
- run status and resume controls

Codex protocol richness is translated behind the backend boundary. Product surfaces should continue to consume Helpin's own run messages, stream events, artifacts, and pause states.

## Preserve One Interaction Contract

Codex does not need to look like `native_sdk` internally, but it should participate in the same Helpin interaction model:

- paused runs still use `status="paused"`
- `pause_reason` remains `human_input` or `human_approval`
- the run drawer still reads `agent_run_message` rows plus `human_input_request` and `human_approval_request` artifacts
- resume, approve, and request-changes continue to flow through the existing Temporal signals and backend endpoints
- interactive artifacts continue to attach to the assistant message that caused them via `assistant_message_sequence_no`

This keeps the product contract stable even if Codex exposes richer internal request and event types.

## Per-Run Flow

1. User opens a coding session on a story.
2. Helpin creates or resumes an `agent_run`.
3. Worker starts `codex-app-server` with an isolated home directory, not shared `~/.codex`.
4. The Go bridge sends:
   - `initialize`
   - account login
   - thread create or resume
   - turn start
5. Codex streams notifications such as thread started, item completed, plan deltas, terminal output, status changes, and approval requests.
6. Helpin converts those protocol events into:
   - transcript updates
   - live runtime stream events
   - diff and changed-file previews
   - approval cards
   - first-class run artifacts
7. When Codex asks for approval or user input, Helpin pauses the run and waits on Temporal signals or UI actions, then replies back to Codex.
8. On completion, Helpin stores final summary, artifacts, diff metadata, and marks the run complete.

## State Ownership

Codex persists sessions and SQLite-backed metadata in its own runtime layer. That state is useful for local resume and recovery, but Helpin should still mirror essential session metadata.

Mirror into Helpin:

- `codex_thread_id`
- `codex_home`
- rollout path if exposed
- current Codex request id if paused on an approval or user-input request
- current runtime session mode and selected model

Boundary rule:

- Helpin DB is authoritative for product-visible run state
- Codex runtime state is an execution cache and runtime-local resume aid

## Auth

Preferred initial auth:

- API-key/provider auth under Helpin control

Helpin-managed ChatGPT OAuth can be added behind a feature flag through the Codex external token refresh bridge, but it should not be the default launch path because the `chatgptAuthTokens` login path is marked unstable in the locally evaluated Codex protocol.

## Runtime Scope Decision

Use this runtime only for:

- `code_builder`
- `review_agent`

Do not use this plan as the base runtime for:

- `epic_planner`
- `story_planner`
- `crm_operator`
- `support_agent`

Those agents should continue to evolve on `native_sdk`, while borrowing good ideas from Codex:

- thread and turn event model
- approval protocol
- request-user-input flows
- stronger transcript and artifact normalization

## Proposed Components

Backend:

- `server/internal/worker/codex_session_host.go`
- `server/internal/worker/codex_appserver_client.go`
- `server/internal/worker/codex_event_mapper.go`
- `server/internal/worker/codex_approval_bridge.go`
- `server/internal/worker/codex_thread_store.go`

Frontend:

- `frontend/src/components/pm/CodingSession/*`

Likely reuse or extension points:

- [server/internal/worker/codex.go](/root/teampulse/server/internal/worker/codex.go)
- [server/internal/temporalapp/workflow.go](/root/teampulse/server/internal/temporalapp/workflow.go)
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- [frontend/src/components/pm/agentRunInteractions.ts](/root/teampulse/frontend/src/components/pm/agentRunInteractions.ts)

## Interaction Mapping Contract

The initial bridge should preserve Helpin's current workflow and UI contract by mapping Codex requests into the same paused-run semantics and interaction artifacts that `native_sdk` already uses.

| Codex app-server event/request | Helpin run mutation | Helpin artifact/UI contract | Resume path |
| --- | --- | --- | --- |
| `item/tool/requestUserInput` | `status="paused"`, `pause_reason="human_input"` | create `human_input_request` artifact using the current structured-question shape; attach `assistant_message_sequence_no`, `runtime_kind`, `codex_request_id`, and `codex_request_kind` metadata | `ResumeRun(intent="reply")` or `SignalMessage`, mapped back to the Codex request response |
| `item/commandExecution/requestApproval` | `status="paused"`, `pause_reason="human_approval"`, `approval_state="pending"` | create `human_approval_request` artifact with `approval_kind="command_execution"` metadata and command preview fields | `ApproveRun`, `RequestRunChanges`, or `ResumeRun(intent="request_changes")`, mapped to command approval response |
| `item/fileChange/requestApproval` | `status="paused"`, `pause_reason="human_approval"`, `approval_state="pending"` | create `human_approval_request` artifact with `approval_kind="file_change"` metadata and file-change summary or diff context | same approval resume path |
| `item/permissions/requestApproval` | `status="paused"`, `pause_reason="human_approval"`, `approval_state="pending"` | create `human_approval_request` artifact with `approval_kind="permissions"` metadata and requested-permission context | same approval resume path |
| assistant item completion | persist `agent_run_message` assistant turn | transcript remains authoritative; interactive artifacts must reference this message via `assistant_message_sequence_no` | n/a |
| plan deltas, terminal deltas, status notifications | publish `agent_run_stream` events and optional runtime artifacts | existing live run stream UI remains the consumer; richer Codex detail can be additive | n/a |

## Question and Approval Fidelity

Helpin's current interactive question contract is narrower than Codex's.

Current Helpin expectations:

- `human_input_request` question payloads
- single-select options
- `human_approval_request` review cards

Codex can request richer user input than the current drawer supports. Initial rollout should:

- down-map Codex user-input requests into the existing Helpin question shape when possible
- preserve richer Codex details in artifact metadata
- avoid exposing a second UI path during phase 1

If richer Codex prompts become important, extend the Helpin interaction schema later rather than bypassing the current run drawer model.

## Artifact Strategy

Near-term:

- keep existing Helpin interaction artifact types:
  - `human_input_request`
  - `human_approval_request`
- allow Codex-specific runtime artifacts where needed:
  - stdout
  - stderr
  - config
  - final summary
  - diff preview
  - changed file bundle

Longer-term:

- normalize toward runtime-agnostic artifact families with runtime metadata rather than runtime-branded artifact names

## Workstreams

### Workstream 1: Codex Session Host

Build a Go-side long-lived Codex app-server host that owns process lifecycle, initialization, thread resume, and shutdown.

### Workstream 2: Codex Event Mapper

Translate Codex app-server notifications and requests into:

- `agent_run_message`
- `agent_run_stream`
- runtime artifacts
- paused-run state
- Helpin approval and input artifacts

### Workstream 3: Temporal Pause/Resume Integration

Reuse the existing Temporal wait model:

- awaiting approval
- awaiting input
- resume
- approve
- request changes

The bridge should adapt Helpin's signal payloads back into Codex request responses instead of inventing a runtime-specific workflow shape.

### Workstream 4: Coding Session UI

Add a Helpin-native coding session UI with:

- transcript
- terminal stream
- file and diff preview
- approval cards
- resume controls

### Workstream 5: State Mirroring and Recovery

Persist enough Codex session metadata in Helpin to:

- reconnect to the session cleanly
- recover from worker restart
- re-render paused interaction state

### Workstream 6: Auth and Runtime Config

Add clear configuration for:

- Codex path
- model
- base URL if needed
- auth mode
- feature flag for Helpin-managed ChatGPT OAuth

## Rollout Phases

### Phase 1: Backend Session Host and Coder Runtime

Ship:

- app-server-backed Codex runtime manager
- coder/reviewer runtime support
- event mapping into current run messages and artifacts
- approval and question translation into current Helpin interaction contract

Do not ship a new coding UI yet.

### Phase 2: Helpin-Native Interactive Coding UI

Ship:

- interactive coding session UI
- session resume across user visits
- diff, approval, and runtime stream panels

### Phase 3: Optional Runtime Deepening

Evaluate:

- optional Rust-side gateway using Codex's in-process app-server host instead of stdio JSON-RPC
- selective extension of Helpin's interaction schema for richer Codex user-input prompts
- optional ChatGPT OAuth behind a feature flag

## Risks

### 1. Interaction Contract Drift

If the bridge does not translate Codex approvals and user-input requests into Helpin's current pause and artifact contract, the run drawer will show incomplete or misleading state.

### 2. Runtime Artifact Drift

If Codex runtime artifacts are added without UI normalization, runs will look incomplete even when execution succeeded.

### 3. Queue and Worker Mismatch

If Codex gets its own queue but no worker polls it, runs will remain queued indefinitely.

### 4. Protocol Drift

Codex app-server is the right seam, but protocol details may evolve. Product-facing code should not depend directly on raw Codex payloads.

### 5. Recovery Complexity

Worker restart, run resume, and paused approval recovery are more complex for a stateful runtime than for `codex exec`.

## Related References

- current Helpin Codex shell-out adapter: [server/internal/worker/codex.go](/root/teampulse/server/internal/worker/codex.go)
- existing Helpin pause/resume workflow: [server/internal/temporalapp/workflow.go](/root/teampulse/server/internal/temporalapp/workflow.go)
- current Helpin interaction parser: [frontend/src/components/pm/agentRunInteractions.ts](/root/teampulse/frontend/src/components/pm/agentRunInteractions.ts)
- current agent preset model: [docs/AGENTS_AND_AUTOMATION.md](/root/teampulse/docs/AGENTS_AND_AUTOMATION.md)
- related codex runtime plan: [docs/plans/2026-03-25-codex-opencode-coder-runtime-plan.md](/root/teampulse/docs/plans/2026-03-25-codex-opencode-coder-runtime-plan.md)

## Success Criteria

- a `code_builder` or `review_agent` run can execute against Codex app-server rather than only `codex exec`
- Helpin remains the only frontend surface; the browser never talks directly to Codex
- Codex-backed approvals and human questions work through the same Helpin run drawer model used today for `native_sdk`
- paused runs can resume cleanly through existing Temporal signal paths
- coding-session transcript, stream, and artifacts remain comprehensible in Helpin UI
- `native_sdk` remains the default runtime for planner, CRM, and support agents
