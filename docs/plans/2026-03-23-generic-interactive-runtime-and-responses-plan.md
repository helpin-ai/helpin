# Generic Interactive Runtime and Responses API Plan

## Why This Exists

The current interactive agent loop works in pieces, but not as one coherent generic system.

Observed problems:

- `awaiting_input` and `awaiting_approval` are still treated as separate transport/control paths.
- Interactive resume depends too much on exact assistant message shapes instead of persisted app state.
- Preview artifacts are not first-class resume context.
- The direct epic planner still carries planner-specific branching in prompt/instruction code.
- The `native_sdk` runtime is not actually aligned with a true Responses-style model/runtime contract for OpenAI/OpenRouter.
- Temporal activity liveness was fragile because the in-process runtime did not send periodic heartbeats.

This plan makes the runtime generic, keeps artifacts app-owned, and moves OpenAI/OpenRouter onto a Responses-style runtime that fits interactive resume better.

## Goals

- One generic paused/resume model for all interactive runs.
- App-owned artifact state is the source of truth.
- Every resumed turn sees the latest preview/doc/story-plan state.
- OpenAI/OpenRouter interactive runs use a Responses-style runtime contract.
- The planner becomes a consumer of the generic substrate, not a custom mini workflow engine.
- Heartbeats and cancellation are reliable for long-running interactive runs.

## Non-Goals

- Removing persisted run messages or artifacts.
- Making provider-side conversation state authoritative.
- Replacing Docs, epic, or story records with model-session state.
- Rewriting the planner prompt from scratch in the first pass.

## Core Rules

1. The database owns workflow truth.
2. Provider runtime state is execution transport, not business state.
3. Artifacts must be replayable without relying on assistant prose.
4. Human input, approval, and change requests should all resume the same transcript loop.
5. Planner behavior should be derived from durable facts and transcript state, not hidden phase machines.

## Current State Summary

### Working Pieces

- Run transcript persistence exists via `agent_run_messages`.
- Artifact persistence exists via `agent_run_artifacts`.
- The Temporal workflow already accepts either an approval signal or a message signal while paused.
- The direct epic planner already uses one transcript instead of separate child runs.

### Structural Gaps

- `SendRunMessage` only resumes `awaiting_input` runs.
- Approval handling still needs special guards and UI affordances.
- Resume context is reconstructed mostly from run messages, not artifact state.
- `run_preview`, `approved_preview`, and `approved_preview_applied` are persisted, but not treated as generic replay inputs.
- `native_sdk` uses chat-model semantics today, while policy/UI already imply Responses-style support for OpenAI/OpenRouter.

## Target Architecture

### 1. Generic Paused Run Model

Keep `status` generic and introduce an explicit pause reason in run state.

Recommended shape:

- `status`: `queued | running | paused | completed | failed | cancelled`
- `pause_reason`: `human_input | human_approval | none`

If changing the DB shape immediately is too disruptive, keep current statuses short-term but implement behavior as if there is one paused state.

Short-term compatibility rule:

- `awaiting_input` and `awaiting_approval` must both be resumable through the same backend path.
- The UI should always allow a transcript reply while paused.
- Approval and request-changes are typed forms of the same resume action.

### 2. App-Owned Artifact State

Treat these as first-class state for interactive runs:

- Latest preview per panel key
- Latest approved preview
- Applied approved preview markers
- Canonical linked PRD document
- Approved spec version
- Created stories from approved story plan

This data remains authoritative even if provider-side response chaining is lost.

### 3. Responses-Style Runtime Contract

For OpenAI/OpenRouter, the runtime should be item/event oriented rather than plain chat-message reconstruction.

The runtime adapter should:

- preserve provider response IDs when available
- preserve assistant output items and tool call items
- preserve tool result items
- emit normalized internal execution events
- still persist the final transcript and artifacts into Teampulse-owned tables

Teampulse should normalize provider events into one internal schema consumed by the UI and activity layer.

## Workstreams

## Workstream A: Unify Pause/Resume Controls

### Backend

Files:

- `server/internal/service/agent.go`
- `server/internal/handler/agent.go`
- `server/internal/temporalapp/workflow.go`
- `server/internal/temporalapp/activities.go`
- `server/internal/model/agent.go`

Changes:

- Add one generic resume service entry point for interactive runs.
- Allow resume when run is paused for either input or approval.
- Model approval and change requests as structured resume intents instead of separate transport behavior.
- Keep `ApproveRun` and `RequestRunChanges` as compatibility endpoints if needed, but route both through the same underlying resume logic.
- Make the Temporal workflow treat “human replied” as one resume path with optional metadata for approval/change intent.

Recommended service shape:

```go
type ResumeAgentRunRequest struct {
    Intent  string `json:"intent"` // "reply" | "approve" | "request_changes"
    Content string `json:"content,omitempty"`
}
```

### Frontend

Files:

- `frontend/src/components/pm/AgentRunDrawer.tsx`
- `frontend/src/lib/services/agentService.ts`
- `frontend/src/components/pm/agentRunConstants.ts`

Changes:

- Always show one reply surface while a run is paused.
- Approval card should prefill or structure intent, not replace transcript reply.
- Keep explicit approve/change buttons if desired, but they should submit the same resume API with different `intent`.
- Status display can still show “Awaiting approval” or “Awaiting input”, but this becomes presentation, not transport behavior.

## Workstream B: Artifact Replay as First-Class Context

### Backend

Files:

- `server/internal/temporalapp/activities.go`
- `server/internal/service/agent.go`
- `server/internal/model/agent_preview.go`
- `server/internal/repository/agent.go`
- worker context/prompt files in `server/internal/worker`

Changes:

- Add a run-context snapshot builder for interactive resume.
- On each execution round, load:
  - latest `run_preview` by `panel_key`
  - latest `approved_preview`
  - latest `approved_preview_applied`
  - relevant doc linkage and epic/story facts
- Inject a compact structured artifact-state section into the runtime context before model execution.

Recommended runtime input shape:

```go
type InteractiveArtifactState struct {
    LatestPreviews          []PreviewState
    LatestApprovedPreview   *model.ApprovedRunPreview
    AppliedApprovedPreviews []model.AppliedApprovedRunPreview
    SpecDocumentID          *string
    ApprovedSpecVersionID   *string
    ExistingStoryIDs        []string
}
```

### Behavior Requirements

- If the human asks for PRD changes, the agent must see the latest PRD preview content explicitly.
- If the human approves a preview, the apply step must use the persisted approved preview artifact, not infer from assistant text.
- If approval and preview were split across assistant turns, replay still works.
- If the right-pane state exists only as artifacts, the model still gets it on resume.

## Workstream C: Responses Runtime for OpenAI/OpenRouter

### Backend

Files:

- `server/internal/worker/runtime_factory.go`
- `server/internal/worker/runtime_adapter.go`
- `server/internal/worker/eino_exec.go`
- `server/internal/worker/eino_executor.go`
- new runtime adapter files under `server/internal/worker`
- `server/internal/service/agent_policy.go`

Changes:

- Introduce a dedicated Responses-style runtime adapter for OpenAI/OpenRouter.
- Preserve provider response identifiers and response-item lineage when supported.
- Normalize provider items into internal execution messages/events so existing artifact/transcript persistence still works.
- Keep the current runtime adapter interface stable if possible.

Recommended internal adapter split:

- `native_sdk_anthropic_chat`
- `native_sdk_openai_responses`
- `native_sdk_openrouter_responses`

If keeping one `native_sdk` label externally, dispatch internally by provider.

### Important Constraint

The Responses runtime should not become the source of truth for run state.

Provider response IDs are useful for:

- continuation efficiency
- event fidelity
- tool sequencing

But Teampulse still owns:

- run status
- pause reason
- preview artifacts
- approval artifacts
- linked docs
- created stories

## Workstream D: Provider/Runtime Contract Cleanup

### Files

- `server/internal/service/agent_policy.go`
- `server/internal/model/agent.go`
- agent settings frontend files under `frontend/src/pages/pm/Agents.tsx` and related types

### Changes

- Decide the supported matrix explicitly.
- Validate only combinations the backend can actually execute.
- Remove provider labels that are not wired through the runtime layer.

Recommended policy:

- `anthropic`: current chat-style native runtime
- `openai`: Responses runtime
- `openrouter`: Responses runtime
- `openrouter-responses`: either alias to `openrouter` or remove if redundant

## Workstream E: Planner Simplification on Top of Generic Runtime

### Files

- `server/internal/temporalapp/activities.go`
- `server/internal/service/agent_system_prompts.go`
- worker prompt builders in `server/internal/worker`

### Changes

- Keep durable facts only:
  - approved spec exists
  - draft spec exists
  - stories already exist
  - approved preview was applied
- Remove prompt-level branch choreography where possible.
- Reduce instructions like “Branch A/B/C/D” once artifact replay is reliable.
- Keep planner-specific tool guidance, but not planner-specific state machines.

### Desired Planner Rule

The planner should decide what to do next from:

- transcript
- current artifact state
- linked docs
- approved spec version
- existing stories

not from a persisted or implied planner phase machine.

## Workstream F: Liveness and Cancellation

### Files

- `server/internal/worker/eino_executor.go`
- `server/internal/temporalapp/workflow.go`
- other runtime adapters as needed

### Changes

- Keep the periodic heartbeat fix for `native_sdk`.
- Require every runtime to heartbeat independently of streamed model events.
- Verify cancellation closes model/tool execution promptly.
- Ensure post-run persistence steps also heartbeat if they can be long-running.

## Workstream G: Regression Coverage

### Tests to Add

Backend integration and unit tests should cover:

- paused run resumed with plain human reply
- paused run resumed with approval intent
- paused run resumed with request-changes intent
- PRD preview approved and persisted to linked document
- PRD preview revised using latest preview artifact after change request
- story plan approved and applied to story creation
- split-turn `publish_preview` and `request_human_approval`
- long quiet model call with no heartbeat timeout
- provider continuation with persisted artifact replay
- resume after page refresh / fresh API fetch

### Primary Test Files

- `server/internal/service/agent_interactive_approval_test.go`
- `server/internal/temporalapp/activities_test.go`
- runtime tests under `server/internal/worker`
- targeted frontend tests around `AgentRunDrawer`

## Recommended Execution Order

### Phase 1: Stabilize Current Generic Interactive Loop

- Unify pause/resume behavior
- make artifact replay first-class
- finish heartbeat/liveness hardening

### Phase 2: Introduce Responses Runtime

- add OpenAI/OpenRouter Responses adapter
- normalize events/items into internal execution schema
- clean up provider/runtime validation

### Phase 3: Simplify Planner on Top

- remove branch-heavy prompt choreography
- rely on transcript + artifact state + DB facts

## Concrete File-Level Starting Checklist

### First Pass

- `server/internal/service/agent.go`
  - add generic resume request path
  - collapse approval/change/reply handling onto shared resume semantics

- `server/internal/temporalapp/workflow.go`
  - simplify paused-state handling around one resume concept

- `server/internal/temporalapp/activities.go`
  - build and inject `InteractiveArtifactState`
  - stop assuming run messages alone are sufficient replay context

- `frontend/src/components/pm/AgentRunDrawer.tsx`
  - keep one transcript reply surface for paused runs
  - submit resume intent instead of bespoke transport paths

### Second Pass

- `server/internal/worker`
  - add Responses runtime adapter
  - preserve provider response lineage
  - keep normalized internal execution events

- `server/internal/service/agent_policy.go`
  - make allowed provider/runtime combinations truthful

### Third Pass

- planner prompt/instruction cleanup in `server/internal/temporalapp/activities.go`
- reduce branch choreography after artifact replay is in place

## Definition of Done

This work is done when all of the following are true:

- A paused interactive run can always be resumed through one generic backend path.
- Approval and request-changes are intent variants, not separate transport modes.
- The model always receives the latest preview/doc/story-plan artifact state on resumed turns.
- OpenAI/OpenRouter interactive runs use a Responses-style runtime contract.
- Provider/runtime options exposed in product settings match what the backend actually supports.
- The planner no longer relies on hidden flow-state logic to recover its next step.
- Long-running or quiet interactive runs do not die from heartbeat timeout.

## Notes for the Next Session

- Start with Phase 1 before touching the Responses adapter.
- Do not try to use provider-side response chaining as a substitute for app-owned artifacts.
- The right pane should be treated as a view over persisted artifacts, not transient assistant output.
- Preserve backward compatibility for existing runs while migrating the control model.
