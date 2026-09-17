# Codex-Native Interaction Contract Plan

## Status

Draft implementation plan for replacing Helpin's flattened human-input and approval layer with a versioned interaction contract that is isomorphic to Codex where Codex already has a first-class protocol.

This plan is based on:

- Current execution ownership: [Coding Agent Runtime Flow](../CODING_AGENT_RUNTIME_FLOW.md). The retired in-process runtime rollout plan is available in Git history.
- [docs/plans/2026-03-31-coding-session-ui-and-runtime-contract-plan.md](/root/teampulse/docs/plans/2026-03-31-coding-session-ui-and-runtime-contract-plan.md)
- the locally cloned Codex repo at `/tmp/codex`
- the current Helpin bridge code in:
  - [server/internal/worker/codex_approval_bridge.go](/root/teampulse/server/internal/worker/codex_approval_bridge.go)
  - [server/internal/worker/codex_event_mapper.go](/root/teampulse/server/internal/worker/codex_event_mapper.go)
  - [server/internal/worker/tools_interaction.go](/root/teampulse/server/internal/worker/tools_interaction.go)

## Problem

Today Helpin collapses several distinct interaction families into two narrow shapes:

- `request_human_input`
- `request_human_approval`

That causes real data loss:

- Codex `request_user_input` loses `header`, `description`, `is_secret`, and native answer structure
- Codex command approvals lose `available_decisions`, `approval_id`, network context, execpolicy amendments, and session-scoped approval choices
- Codex file-change approvals lose typed decision options
- Codex permissions approvals lose grant-scope choice and structured permission rendering in the UI
- `native_sdk` has no clean target contract to match if we want coding-session parity

The current bridge was acceptable for phase-1 rollout, but it is now the main source of avoidable complexity.

## Goal

Adopt a Helpin-owned interaction contract that:

- preserves Codex interaction payloads losslessly for the Codex-supported request families
- becomes the canonical coding-session interaction model across Codex, OpenCode, and `native_sdk`
- keeps Helpin as system-of-record for session state, persistence, UI, approvals, and resume
- allows Helpin-specific product review checkpoints to coexist cleanly without overloading runtime approval semantics

## Recommendation

Do not make the Helpin product surface a raw copy of every Codex protocol message.

Do make the Helpin interaction layer isomorphic to Codex for the Codex-defined request families.

Concrete rule:

- when Codex already has a native interaction type, Helpin should persist and expose the same shape with no lossy flattening
- when Helpin needs a product-specific checkpoint that Codex does not define, add a Helpin-specific interaction kind instead of overloading Codex kinds

## Scope

In scope:

- coding-session interaction persistence
- coding-session interaction event model
- Codex request and response preservation
- `native_sdk` migration to the new interaction contract
- frontend interaction cards for typed request families
- model-facing tool contract changes for `native_sdk`

Out of scope:

- replacing all historical `agent_run_artifacts` immediately
- removing `agent_runs` as the durable workflow record
- browser-to-Codex direct transport
- adopting all Codex protocol events wholesale outside the interaction layer

## Core Decision

### 1. Replace flattened interaction artifacts with first-class interaction records

Add a dedicated durable model:

- `agent_run_interactions`

Reason:

- interactions have state, identity, payload, response, and lifecycle
- they are not just passive artifacts
- they should not need ad hoc metadata decoding to become actionable UI

Recommended schema:

```sql
CREATE TABLE agent_run_interactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  run_id UUID NOT NULL,
  runtime_kind TEXT NOT NULL,
  interaction_kind TEXT NOT NULL,
  status TEXT NOT NULL,
  request_schema_version TEXT NOT NULL,
  response_schema_version TEXT,
  request_id TEXT,
  thread_id TEXT,
  turn_id TEXT,
  item_id TEXT,
  approval_id TEXT,
  assistant_message_sequence_no INTEGER,
  title TEXT,
  summary TEXT,
  request_payload JSONB NOT NULL,
  response_payload JSONB,
  runtime_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  resolved_by TEXT,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Recommended indexes:

- `(workspace_id, run_id, created_at)`
- `(workspace_id, run_id, status)`
- `(workspace_id, run_id, interaction_kind, status)`
- unique partial index on `(workspace_id, run_id, request_id)` where `request_id IS NOT NULL`

## Canonical Interaction Kinds

These become the canonical coding-session request families:

- `request_user_input`
- `command_execution_approval`
- `file_change_approval`
- `permissions_approval`
- `review_checkpoint`
- `auth_required`

Rules:

- the first four should mirror Codex exactly where possible
- `review_checkpoint` remains Helpin-specific for PRD/story-plan/customer-visible review gates
- `auth_required` is runtime-owned but should be modeled consistently with the same persistence and event flow

## Payload Naming Rule

Use two layers:

- Helpin envelope fields use Helpin naming conventions such as `interaction_kind`, `request_schema_version`, and `assistant_message_sequence_no`
- raw `request_payload` and `response_payload` preserve the native producer shape exactly

That means:

- Codex-native interaction payloads keep Codex field names and JSON variants exactly as received or sent
- Helpin-specific interaction payloads use Helpin conventions

Examples:

- Codex-native payloads keep `threadId`, `turnId`, `itemId`, `approvalId`, `isOther`, `isSecret`, `availableDecisions`
- Helpin-specific `review_checkpoint` payloads use `phase`, `artifact_refs`, and other Helpin fields

This is a deliberate exception to the normal preference for snake_case tool fields because contract fidelity is the point of this migration.

## Target Backend Types

Recommended Go model:

```go
type AgentRunInteraction struct {
    ID                        string
    WorkspaceID               string
    RunID                     string
    RuntimeKind               string
    InteractionKind           string
    Status                    string
    RequestSchemaVersion      string
    ResponseSchemaVersion     *string
    RequestID                 *string
    ThreadID                  *string
    TurnID                    *string
    ItemID                    *string
    ApprovalID                *string
    AssistantMessageSequenceNo *int
    Title                     *string
    Summary                   *string
    RequestPayload            json.RawMessage
    ResponsePayload           json.RawMessage
    RuntimeMetadata           json.RawMessage
    ResolvedBy                *string
    ResolvedAt                *time.Time
    CreatedAt                 time.Time
    UpdatedAt                 time.Time
}
```

Recommended frontend type:

```ts
type CodingSessionInteraction =
  | RequestUserInputInteraction
  | CommandExecutionApprovalInteraction
  | FileChangeApprovalInteraction
  | PermissionsApprovalInteraction
  | ReviewCheckpointInteraction
  | AuthRequiredInteraction
```

## Contract Shape By Kind

### `request_user_input`

This should mirror Codex's question contract.

Canonical request payload:

```json
{
  "questions": [
    {
      "id": "confirm_path",
      "header": "Confirm",
      "question": "Proceed with the plan?",
      "isOther": true,
      "isSecret": false,
      "options": [
        { "label": "Yes (Recommended)", "description": "Continue the current plan." },
        { "label": "No", "description": "Stop and revisit the approach." }
      ]
    }
  ]
}
```

Canonical response payload:

```json
{
  "answers": {
    "confirm_path": { "answers": ["yes"] }
  }
}
```

### `command_execution_approval`

This should mirror Codex's command approval request and response families.

Required request payload fields:

- `threadId`
- `turnId`
- `itemId`

Optional native fields to preserve:

- `approvalId`
- `reason`
- `networkApprovalContext`
- `command`
- `cwd`
- `commandActions`
- `additionalPermissions`
- `proposedExecpolicyAmendment`
- `proposedNetworkPolicyAmendments`
- `availableDecisions`

Response payload must preserve Codex response JSON exactly, including:

- `accept`
- `acceptForSession`
- `acceptWithExecpolicyAmendment`
- `applyNetworkPolicyAmendment`
- `decline`
- `cancel`

### `file_change_approval`

Canonical request payload:

- `threadId`
- `turnId`
- `itemId`
- `reason`
- `grantRoot`

Canonical response payload:

- `accept`
- `acceptForSession`
- `decline`
- `cancel`

### `permissions_approval`

Canonical request payload:

- `threadId`
- `turnId`
- `itemId`
- `reason`
- `permissions`

Canonical response payload:

- `permissions`
- `scope`

Important:

- Helpin must preserve `scope: "turn" | "session"`
- do not hardcode turn-scoped grants in the new contract

### `review_checkpoint`

This replaces the current overloaded meaning of `request_human_approval` for product review gates.

Canonical request payload:

```json
{
  "phase": "prd",
  "title": "Approve PRD",
  "summary": "Review the latest draft.",
  "artifact_refs": [
    { "artifact_type": "run_preview", "panel_key": "prd_draft" }
  ]
}
```

Canonical response payload:

```json
{
  "decision": "approve"
}
```

or

```json
{
  "decision": "request_changes",
  "message": "Tighten the acceptance criteria."
}
```

Rule:

- runtime/sandbox approvals must never use `review_checkpoint`
- product/workflow review gates must never be coerced into Codex command/file/permissions approval kinds

## Tool Contract Changes

### Replace `request_human_input` with `request_user_input`

For model-facing runtime tools in `native_sdk`:

- add `request_user_input`
- make its input contract match Codex question payloads exactly, including native field names inside the tool input object
- keep `request_human_input` only as a temporary alias during migration

Files to update:

- [server/internal/worker/tools.go](/root/teampulse/server/internal/worker/tools.go)
- [server/internal/worker/tools_interaction.go](/root/teampulse/server/internal/worker/tools_interaction.go)
- [server/internal/worker/tool_catalog.go](/root/teampulse/server/internal/worker/tool_catalog.go)
- [server/internal/worker/runtime_profiles.go](/root/teampulse/server/internal/worker/runtime_profiles.go)
- planner prompt docs and examples

### Replace overloaded `request_human_approval`

Split semantics:

- `request_review_checkpoint` for product review gates
- runtime command/file/permissions approvals are emitted by the runtime adapter, not as model-called tools

Compatibility:

- keep `request_human_approval` as a decode alias to `request_review_checkpoint` for one release
- update all planner/support prompts to use the new name

## Coding Session Event Model

Add typed interaction events to the coding-session stream:

- `interaction.requested`
- `interaction.updated`
- `interaction.resolved`
- `interaction.cancelled`

Recommended payload:

```json
{
  "interaction_id": "int_123",
  "interaction_kind": "command_execution_approval",
  "status": "pending",
  "request_schema_version": "codex.v2",
  "request_payload": { "...": "..." },
  "response_payload": null,
  "title": "Approve command execution",
  "summary": "Command: go test ./...",
  "assistant_message_sequence_no": 42
}
```

Rule:

- the event stream should carry both the display fields and the typed request payload
- the frontend should not reconstruct request meaning from generic artifact metadata

## UI Changes

### Coding Session UI

The coding-session surface should render dedicated cards for:

- `request_user_input`
- `command_execution_approval`
- `file_change_approval`
- `permissions_approval`
- `review_checkpoint`
- `auth_required`

Each card should render from typed payloads, not flattened prose.

Examples:

- command approval card shows command, cwd, reason, available decisions
- file-change approval card shows diff summary, grant root, and session-scope option when allowed
- permissions approval card shows requested network/filesystem changes and scope picker
- user-input card shows header, question, description, secret input handling, and other/freeform input correctly

Frontend files likely affected:

- [frontend/src/lib/pm-types/codingSession.ts](/root/teampulse/frontend/src/lib/pm-types/codingSession.ts)
- [frontend/src/components/pm/AgentRunDrawer.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDrawer.tsx)
- new `frontend/src/components/pm/coding-session/interactions/*`
- any coding-session page components introduced from the broader coding-session plan

## Backend Workstreams

### Workstream 1: Persistence

Add:

- migration for `agent_run_interactions`
- model + repository + repository tests

Files:

- `server/migrations/0xx_agent_run_interactions.sql`
- `server/internal/model/agent_run_interaction.go`
- `server/internal/repository/agent_run_interaction.go`
- tests under `server/internal/repository/`

Acceptance criteria:

- interaction requests and responses can be stored and listed without artifacts
- latest pending interaction can be fetched in one query

### Workstream 2: Interaction Service Layer

Add a dedicated service API for:

- create interaction request
- resolve interaction
- list interactions for a run/session
- derive current paused interaction state

Files:

- `server/internal/service/agent_interaction.go`
- `server/internal/service/coding_session.go`
- `server/internal/service/agent.go`

Acceptance criteria:

- resume and approval flows read from interaction records, not only artifacts
- coding-session APIs can return pending interactions directly

### Workstream 3: Codex Bridge Migration

Update the Codex adapter to dual-write:

- typed `agent_run_interactions`
- legacy artifacts during compatibility phase

Files:

- [server/internal/worker/codex_event_mapper.go](/root/teampulse/server/internal/worker/codex_event_mapper.go)
- [server/internal/worker/codex_approval_bridge.go](/root/teampulse/server/internal/worker/codex_approval_bridge.go)
- [server/internal/worker/codex_session_host.go](/root/teampulse/server/internal/worker/codex_session_host.go)
- [server/internal/worker/codex_appserver_protocol.go](/root/teampulse/server/internal/worker/codex_appserver_protocol.go)

Acceptance criteria:

- no Codex request fields are lost in persisted request payloads
- session-scoped decisions and permission scopes round-trip correctly
- request changes / approve / cancel map to native Codex responses without fallback parsing hacks

### Workstream 4: `native_sdk` Contract Migration

Update `native_sdk` to emit the same typed interaction families:

- `request_user_input`
- `review_checkpoint`

Longer term, if `native_sdk` coding agents gain runtime-managed shell/file permissions, they should emit the same approval families too.

Files:

- [server/internal/worker/tools.go](/root/teampulse/server/internal/worker/tools.go)
- [server/internal/worker/tools_interaction.go](/root/teampulse/server/internal/worker/tools_interaction.go)
- [server/internal/service/agent_system_prompts.go](/root/teampulse/server/internal/service/agent_system_prompts.go)
- planner contract docs

Acceptance criteria:

- planner prompts no longer depend on `request_human_input`
- planner/support review gates no longer overload runtime approval semantics

### Workstream 5: Frontend Interaction Cards

Replace generic parsing of `human_input_request` and `human_approval_request` with typed interaction rendering.

Compatibility period:

- if a run has new interaction records, prefer them
- otherwise fall back to legacy artifact parsing for historical runs

Acceptance criteria:

- current Codex flows still work
- richer Codex approval choices are exposed in the UI
- no metadata-only hacks are required for the active coding-session surface

### Workstream 6: Cleanup

After all supported runtimes emit the new contract:

- stop writing new `human_input_request` and `human_approval_request` records for coding-session flows
- keep read-only decode support for historical runs
- remove `request_human_input` from prompts and default runtime profiles
- narrow `request_human_approval` to legacy alias behavior only, then remove later

## Rollout Plan

### Phase 1: Dual-Write Foundation

- add the new table, model, repo, and service
- dual-write Codex interactions into both the new table and legacy artifacts
- expose interaction records through coding-session APIs

Exit criteria:

- no user-visible regression
- interaction records mirror live Codex traffic correctly

### Phase 2: Frontend Read Switch

- coding-session UI reads interaction records first
- legacy artifact parsing remains fallback-only

Exit criteria:

- command/file/permissions approvals render from typed payloads
- `request_user_input` renders from Codex-shaped payloads

### Phase 3: `native_sdk` Tool Migration

- introduce `request_user_input`
- introduce `request_review_checkpoint`
- update prompts, runtime profiles, and tests

Exit criteria:

- planner/support interactive flows use the new contract
- no new coding-session flows depend on flattened `request_human_*` payloads

### Phase 4: Legacy De-emphasis

- stop dual-writing legacy coding-session artifacts for new runs
- keep historical-read compatibility

Exit criteria:

- active coding sessions use only the new interaction model
- artifact parsing remains for historical records only

## Testing Plan

Backend:

- repository tests for create/list/resolve interactions
- Codex bridge tests for each request kind
- resume tests for accept, accept-for-session, request changes, decline, cancel, and permission-scope choices
- service tests for latest pending interaction lookup

Frontend:

- typed-card unit tests for each interaction kind
- coding-session integration tests for pending/resolved lifecycle
- regression tests for historical artifact fallback

End-to-end:

- Codex command approval round trip
- Codex file-change approval round trip
- Codex permissions approval round trip with `scope=session`
- Codex `request_user_input` with `isOther` and `isSecret`
- `native_sdk` planner review checkpoint round trip

## Risks

### Risk 1: Over-coupling to unstable Codex fields

Mitigation:

- store `request_schema_version`
- preserve raw request payloads
- isolate Codex-specific decode logic in the adapter layer

### Risk 2: Mixing product review and runtime approval semantics again

Mitigation:

- separate `review_checkpoint` from command/file/permissions approvals at the schema level
- enforce the distinction in service validation and UI components

### Risk 3: Migration churn in planner prompts and tests

Mitigation:

- keep temporary decode aliases
- migrate prompts and tests in one tracked phase instead of piecemeal edits

## Acceptance Criteria

This plan is complete when:

- Codex interaction requests round-trip without lossy flattening
- Helpin coding-session UI renders typed interaction cards from first-class records
- `native_sdk` can target the same interaction contract
- product review checkpoints remain supported without masquerading as runtime approvals
- forking or upgrading against `codex-app-server` no longer requires rewriting Helpin's flattened bridge semantics

## Recommended First PR Sequence

1. Add `agent_run_interactions` model, migration, repository, and service.
2. Dual-write Codex requests into the new table from the event mapper.
3. Add coding-session API responses and event payloads for typed interactions.
4. Build typed frontend cards and switch the coding-session UI to prefer the new contract.
5. Add `request_user_input` and `request_review_checkpoint` to `native_sdk`, keeping legacy aliases temporarily.
6. Remove legacy interaction writes for new coding-session flows after one stable release.
