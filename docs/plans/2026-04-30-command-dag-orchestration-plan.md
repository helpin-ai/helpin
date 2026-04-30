# Command DAG Orchestration Plan

**Date:** 2026-04-30  
**Status:** Proposed follow-on to meta-agent command bar and task-pipeline Temporal orchestration  
**Owner area:** Agents, command bar, Temporal automation, Ask Agents dock

---

## Summary

Generalize command-bar plans from single runs, linear plans, and bounded fan-out into a durable DAG execution model. Users should be able to ask for orchestration in natural language, such as:

- "Complete this epic, do blockers first, fan out independent work, and run Forge then Lens for each task."
- "Prepare this release: finish blockers, review risky changes, update release notes, then summarize ship readiness."
- "Audit stale docs in this collection, update the affected pages in parallel, review them, then publish a summary."

The backend should parse these requests into a validated step graph, preview the graph in the Ask Agents dock, and execute it through a Temporal parent workflow that schedules child `AgentRunWorkflow` executions as dependencies become satisfied.

The first production slice already exists for `task_pipeline_fan_out`: an epic task pipeline with Forge then Lens pairs and `blocks` dependencies. This plan expands that slice into a generic command DAG system.

## Goals

- Let users express multi-agent orchestration naturally without manually starting each run.
- Support dependency-aware fan-out and fan-in across saved agents and one-shot command-agent steps.
- Use Temporal as the durable parent scheduler for DAG plans.
- Keep `agent_run` as the durable child execution primitive.
- Preserve confirm-before-dispatch with visible plan preview, run count, tool scope, and guardrails.
- Make LLM-produced plans advisory; validate every step, target, tool, and dependency server-side.
- Show DAG progress inside the Ask Agents dock, not as a separate rail.

## Non-Goals

- Agents recursively spawning arbitrary agents mid-run.
- Silent execution without user confirmation.
- Cross-workspace DAGs.
- Automatic creation of reusable custom agents.
- Full structured diff/proposal approval. Existing run approval remains separate.
- Arbitrary unbounded DAGs. The system must cap nodes, fan-out width, and tool scope.

## Current State

Implemented:

- `task_pipeline_fan_out` plan kind.
- `CommandBarPlanStep.depends_on_step_indexes`.
- Temporal parent workflow for command-bar task pipelines.
- Step readiness activity that schedules newly unblocked child runs.
- Child run completion signaling back to the parent workflow.
- Epic parser that builds Forge then Lens pairs, skips completed tasks, and maps task `blocks` edges.
- Frontend types for `task_pipeline_fan_out` and dependency indexes.

Limitations:

- Only the deterministic epic Forge/Lens path creates a DAG.
- The LLM router does not return generic DAG plans.
- One-shot command-agent plans are still single-step or conventional linear plans.
- Ask Agents dock needs a DAG-specific preview and progress presentation.
- Retry of DAG plans is intentionally blocked until DAG retry semantics are designed.

## Product Scenarios

### 1. Epic Execution

User says: "Complete all tasks in this epic. Do blockers first. Fan out independent tasks. For every task run Forge then Lens."

Plan:

- Load active tasks in the epic.
- Load task dependency edges.
- Build Forge and Lens steps per task.
- Lens depends on that task's Forge.
- Blocked task Forge depends on blocking task Lens.

Status: first slice implemented.

### 2. Release Readiness

User says: "Prepare this release. Finish open blockers, review risky tasks, update release notes, then summarize ship/no-ship."

Plan:

- Analyze release context and open blockers.
- Fan out Forge over fixable blockers.
- Run Lens on completed fixes.
- Run release-notes/document agent after reviews.
- Run final summary one-shot after release notes.

### 3. Workspace Triage

User says: "How many engineering tasks need attention? Group them by team, assign owners where missing, and notify leads."

Plan:

- One-shot read step lists and classifies tasks.
- Optional parallel per-team mutation steps.
- Final summary step reports counts and changes.

### 4. Docs Refresh

User says: "Audit this docs collection for stale API references, update stale pages, then review the updates."

Plan:

- One-shot discovery step finds stale docs.
- Parallel document update steps.
- Parallel review steps.
- Final summary step.

### 5. Customer Escalation Resolution

User says: "Resolve everything blocking Acme before the QBR."

Plan:

- Support agent summarizes open tickets.
- CRM agent loads account/deal context.
- Task agent creates/fixes work items.
- Draft support/customer update after task work completes.

## Backend Design

### Plan Kinds

Keep `task_pipeline_fan_out` for the deterministic epic pipeline.

Add a generic DAG plan kind:

```go
const CommandBarPlanKindDAG = "dag"
```

Optional naming for one-shot-heavy plans:

```go
const CommandBarPlanKindOneShotDAG = "one_shot_dag"
```

Recommendation: start with one generic `dag` kind. A plan may contain saved-agent steps and Command Agent one-shot steps. The step's `agent_id`, `plan_kind`, and `allowed_tools` describe the executor.

### Step Contract

Existing `CommandBarPlanStep` is close:

```go
type CommandBarPlanStep struct {
    AgentID              string
    AgentKey             string
    AgentName            string
    PlanKind             string
    Target               CommandBarPageContext
    Instructions         string
    AllowedTools         []string
    DependsOnStepIndexes []int
}
```

Add optional display metadata only if the UI needs it:

```go
GroupKey   string `json:"group_key,omitempty"`   // e.g. task ID, team ID, doc ID
GroupLabel string `json:"group_label,omitempty"` // e.g. "HLP-123 Fix billing bug"
```

Avoid storing LLM-only hidden reasoning in the step.

### LLM Router Schema

Extend the router output with `route_kind: "dag"`:

```json
{
  "status": "plan",
  "route_kind": "dag",
  "rationale": "The request contains independent task execution plus review and final summarization.",
  "confidence": 0.82,
  "steps": [
    {
      "agent_id": "command-agent-id",
      "target": { "entity_type": "workspace", "entity_id": "..." },
      "instructions": "Find engineering tasks needing attention and group by team.",
      "allowed_tools": ["list_tasks", "list_workspace_teams"],
      "depends_on_step_indexes": []
    },
    {
      "agent_id": "lens-id",
      "target": { "entity_type": "task", "entity_id": "..." },
      "instructions": "Review the completed implementation.",
      "depends_on_step_indexes": [0]
    }
  ],
  "guardrails": []
}
```

Rules:

- The LLM may propose a graph, but the backend owns validation.
- The LLM must choose from server-provided agents, targets, and tools only.
- One-shot steps must use the Command Agent and a minimal allowed tool subset.
- Mutation tools require explicit preview guardrails.
- Discovery steps that produce dynamic targets are not v1 generic DAG. They require a later "expanding step" design.

### Validation

Before dispatch:

- Validate all step indexes are in range.
- Reject self-dependencies.
- Detect cycles.
- Cap total steps, initially 50.
- Cap initially runnable fan-out width, initially 10.
- Validate target types via existing command-bar target validation.
- Validate agent existence and allowed target.
- Validate one-shot tool subsets are non-empty and within the Command Agent allowlist.
- Validate saved-agent tool subsets are within the selected agent allowlist.
- Reject unsupported dynamic placeholders in targets or instructions.
- Add guardrails when mutation tools are present.

### Temporal Execution

Reuse the existing parent workflow shape:

- Parent workflow loops:
  - call activity to schedule ready steps
  - wait for child completion signal or periodic timer
  - terminate when plan is completed, failed, or cancelled
- Child runs remain normal `AgentRunWorkflow` executions.
- `command_bar_plans.run_ids_by_step` remains the read model for UI.
- `agent_run.input.trigger.context` continues to carry `plan_id`, `step_index`, and step list.

Generalize current task-pipeline dispatch:

```go
if commandBarPlanKindForSteps(steps) == CommandBarPlanKindTaskPipeline ||
   commandBarPlanKindForSteps(steps) == CommandBarPlanKindDAG {
    StartCommandBarPlan(...)
}
```

### Completion Semantics

Initial policy:

- If any required step fails or is cancelled, mark the plan failed.
- Dependent steps do not start after a failed dependency.
- Independent already-running steps are not automatically cancelled in v1.
- Whole-plan cancel cancels active children and marks plan cancelled.

Later policy:

- Add `continue_on_failure` per step only after product need is clear.
- Add partial retry from failed node after DAG retry semantics are designed.

### Retry Semantics

Do not reuse the linear retry path.

Design a DAG-specific retry API:

```http
POST /api/command-bar/plans/{planID}/retry-dag
{
  "from_step_indexes": [3],
  "mode": "failed_subgraph"
}
```

Supported v1 retry modes:

- `failed_only`: rerun failed/cancelled steps whose dependencies are completed.
- `failed_subgraph`: rerun failed steps and all downstream dependent steps.

Until then, keep generic DAG retry blocked with a clear error.

## Frontend Design

### Ask Agents Dock Preview

For DAG plans, the confirm preview should show:

- Plan kind: `DAG` or a friendlier label like `Orchestrated plan`.
- Total step count.
- Estimated initial fan-out.
- Mutation-tool guardrails.
- Grouped steps when group metadata exists.

Examples:

- Task pipeline preview:
  - `HLP-123: Forge -> Lens`
  - `HLP-124: waiting on HLP-123`
- Generic DAG preview:
  - `Discover tasks`
  - `Team A updates` and `Team B updates` in parallel
  - `Final summary` after both finish

### Progress View

Add a DAG-specific rail inside the dock:

- Do not render DAGs as a simple left-to-right linear pipeline.
- Show counts: `3 running · 7 waiting · 4 done · 1 failed`.
- Show grouped rows when possible.
- Show each row's step status from `run_ids_by_step`.
- Waiting steps should show "Waiting on N dependencies".

### Store Changes

Current store can hold:

- plan kind
- steps
- run IDs by step
- runs by ID

Likely additions:

- helper selectors for dependency state
- derived counts for waiting/runnable/running/completed/failed
- optional grouping by `group_key`

Avoid duplicating backend scheduler state in the store.

## API Changes

Potential additions:

- `CommandBarPlanKindDAG`.
- Optional `group_key` and `group_label` on `CommandBarPlanStep`.
- Optional plan-level graph summary:

```json
{
  "max_parallelism": 10,
  "dependency_count": 12,
  "group_count": 6
}
```

Not required for the first generic backend slice if the frontend derives it from steps.

## Rollout Plan

### Phase 1 — Stabilize Current Task Pipeline

- Finish Ask Agents dock rendering for `task_pipeline_fan_out`.
- Show task-grouped Forge then Lens rows.
- Show waiting dependency state.
- Keep DAG retry blocked.
- Add tests around plan hydration with zero initial runs.

### Phase 2 — Generic DAG Backend

- Add `dag` plan kind.
- Generalize Temporal dispatch branch to all DAG plan kinds.
- Add DAG validation: range, cycle, fan-out cap, tool/target checks.
- Add tests for validation and ready-step scheduling.
- Keep parser deterministic for known patterns.

### Phase 3 — LLM DAG Planning

- Extend command-router JSON schema with `route_kind: "dag"` and step dependencies.
- Prompt the router with available agents, command-agent tools, and page context.
- Require confidence threshold for DAG plans.
- Fall back to one-shot or no-match when confidence is low.
- Add structured parse tests using scripted LLM responses.

### Phase 4 — One-Shot DAGs

- Allow multiple Command Agent steps in a DAG when the request has clear phases.
- Require narrow tool subsets per one-shot step.
- Add mutation guardrails for any one-shot mutation step.
- Preview the plan before dispatch.

### Phase 5 — DAG Retry

- Add `retry-dag` API.
- Implement `failed_only`.
- Implement `failed_subgraph`.
- Add UI affordance only for failed/cancelled DAG plans.

## Test Plan

Backend:

- Parse deterministic epic pipeline.
- Validate generic DAG accepts parallel branches and fan-in.
- Reject cycles.
- Reject invalid dependency indexes.
- Reject one-shot steps without tools.
- Reject tool subsets outside agent allowlist.
- Scheduler starts all initially ready steps.
- Scheduler starts dependent steps after parent completion.
- Scheduler marks plan failed on required child failure.
- Cancel marks plan cancelled and cancels active runs.

Frontend:

- Preview DAG with grouped task pipeline.
- Hydrate DAG plan with no initial runs.
- Update DAG plan as child runs are created.
- Show waiting/running/completed counts.
- Do not render DAG as a misleading linear rail.

Verification:

- `go test ./internal/service -run 'CommandBar|DAG|ParseEpicTaskPipeline'`
- `go test ./internal/temporalapp -run CommandBar`
- `go build ./cmd/api ./cmd/temporal-worker`
- `npm run build`

## Open Questions

- Should generic DAG plans use only existing concrete targets, or can a discovery step expand new targets into later steps?
- Should a failed independent branch fail the whole plan immediately, or allow other branches to finish first?
- Should user approval be required again before mutation-heavy downstream steps start?
- Should DAG plans have a max concurrency setting per plan, or use a global command-bar cap?
- What UI label is clearest: `DAG`, `Orchestrated plan`, `Task pipeline`, or `Workflow`?

