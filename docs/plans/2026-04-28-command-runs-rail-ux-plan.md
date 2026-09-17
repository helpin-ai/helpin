# Command Runs Rail UX Plan

**Date:** 2026-04-28
**Status:** Implemented in the command runs rail
**Scope:** Redesign the command runs rail around a spine/branch execution model while staying faithful to the current backend semantics.

## Goal

Make command-bar execution easy to understand at a glance:

- what is running now
- what needs attention
- what is queued
- what already completed
- which runs belong to the same command-bar plan
- whether a plan is sequential, one-shot, or fan-out
- which actions are available now: open, approve, cancel, retry, save as agent

The rail should feel like an execution monitor, not a stack of generic cards.

## Current Backend Semantics

The UI must reflect what the backend actually supports today.

Supported:

- `known_agent` sequential plans
  - step 1 starts on dispatch
  - step N+1 starts after step N completes
  - later runs are linked with `parent_run_id`
- `one_shot_command` plans
  - one Command Agent run
  - runtime instructions and narrowed `allowed_tools`
  - may be promoted after successful completion
- `fan_out` plans
  - concrete related targets only
  - all target runs start together
  - capped at 5 targets
  - plan completes only after all target runs complete
  - retry restarts failed/cancelled targets only

Not supported yet:

- mixed DAGs such as `Atlas -> fan-out Forge branches -> Lens join`
- arbitrary joins
- approval gates as command-bar plan steps
- a parent command-bar Temporal workflow

The UI may use branch visuals for fan-out, but it must not imply unsupported join semantics.

## Design Direction

Adopt a compact execution spine:

- a vertical 1px spine connects sequential steps
- fan-out targets render as indented branch rows
- agent identity is anchored in the node/dot
- state is encoded with shape, fill, motion, and text
- sections prioritize operational state over pure chronology

This replaces large stacked cards with a scannable execution graph.

## Information Architecture

Rail sections:

1. **Needs Attention**
   - failed plans/runs
   - cancelled plans/runs with retry affordance
   - paused/waiting-for-approval runs
2. **Running**
   - active plans/runs
3. **Queued**
   - plans waiting for current step completion
   - pending fan-out targets if any are queued
4. **Completed**
   - recently completed plans/runs

Filters:

- All
- Needs attention
- Running
- Queued
- Completed

Counts should be derived from visible plan/run state, not hardcoded.

## Visual Model

### Node States

| State | Shape | Text |
|---|---|---|
| Queued | hollow circle | `queued` |
| Running | filled circle with subtle pulse | `running` or elapsed time |
| Completed | filled circle | duration or `done` |
| Failed | red filled circle | `failed` |
| Cancelled | muted outlined circle with slash/icon | `cancelled` |
| Approval needed | inline diamond marker inside the run row | `approval needed` + action |

Color helps scanning, but state must not rely on color alone.

### Agent Colors

Use stable, restrained agent accent colors:

- Forge / Code Builder: coral
- Lens / Review Agent: teal
- Task Planner / Atlas-style planning: purple
- Command Agent: neutral/blue-gray
- CRM Operator: green
- Support Agent: amber

If an agent does not have a mapped preset, derive a stable color from its ID/name.

### Sequential Plan Rendering

Example:

```text
Forge -> Lens

│  Forge · implement acceptance criteria     done · 1m 47s
│  Lens  · QA and verify tests               queued
```

Rules:

- show the plan title as agent chain or user prompt preview
- show total step count in the header
- render every step on the spine
- pending future steps should use persisted plan steps even before an `AgentRun` exists
- show `parent_run_id` sequencing only as UI state, not as technical text

### Fan-Out Rendering

Example:

```text
Lens across 3 child tasks

│
├─ Lens · TASK-1 · Review auth flow          running
├─ Lens · TASK-2 · Review API flow           done
└─ Lens · TASK-3 · Review UI flow            failed
```

Rules:

- show fan-out count in the header
- show aggregate status: `1 running · 1 done · 1 failed`
- branch rows are indented from the main spine
- do not render a fake join step
- `Retry failed` restarts failed/cancelled targets only
- successful target rows remain visibly completed after retry starts

### One-Shot Rendering

Example:

```text
Command Agent · one-shot

│  Command Agent · update stale doc sections   running
```

Rules:

- label as `one-shot`
- show enabled tool count or compact tool summary
- show target type/title
- show `Save as agent` only after successful completion
- do not show promotion for normal known-agent runs by default

### Approval State

Approval is part of an `AgentRunWorkflow`, not an independent command-bar step.

Render approval inline on the affected run row:

```text
Forge · update migration
approval needed        [Approve] [Reject]
```

Do not render approval gates as standalone nodes until the backend has explicit command-bar gate steps.

## Actions

Plan-level actions:

- Open latest/active run
- Cancel active plan
- Retry failed/cancelled plan
- Retry failed fan-out targets
- Collapse/expand plan

Run-level actions:

- Open run
- Approve/reject if waiting for approval
- Save as agent if completed one-shot command run

Action visibility:

- primary action should be visible for Needs Attention rows
- secondary actions may live behind hover/overflow
- do not hide approval actions behind hover
- do not hide one-shot `Save as agent` behind hover on completed one-shot runs

## Component Plan

Create small components instead of growing `CommandBarRunRail.tsx` further:

- `CommandRunsRail`
  - owns sectioning, filters, rail open/peek state
- `CommandRunSection`
  - renders one section with count and collapsed state
- `CommandPlanTimeline`
  - shared plan shell and spine
- `SequentialPlanTimeline`
  - known-agent sequential steps
- `FanOutPlanTimeline`
  - branch rows and aggregate state
- `OneShotPlanTimeline`
  - one-shot Command Agent presentation
- `CommandRunNode`
  - node dot, agent label, description, status text, inline actions
- `CommandRunActions`
  - open/cancel/retry/save/approve affordances

Keep existing store/API contracts unless a real data gap appears.

## Data Requirements

Existing data is mostly enough:

- `CommandBarPlanSummary.plan_kind`
- `CommandBarPlanSummary.steps`
- `CommandBarPlanSummary.run_ids_by_step`
- linked runs from plan list/detail responses
- run status
- run target info
- run trigger payload for plan metadata

Potential backend follow-ups:

- add `started_at`/`completed_at` display helpers if frontend duration math is messy
- expose active approval state in a compact run summary if not already available
- expose parsed tool count/selected tools for one-shot display if frontend cannot read from input

Do not add backend fields just for styling until the frontend proves the current data is insufficient.

## Implementation Steps

1. [x] **Extract Timeline Components**
   - Split the existing rail into smaller renderer components.
   - Preserve current behavior and tests/typecheck.

2. [x] **Add Sectioning and Filters**
   - Replace flat plan/run list with operational sections.
   - Add counts for All, Needs Attention, Running, Queued, Completed.

3. [x] **Implement Spine Renderer**
   - Render sequential and one-shot plans on a vertical spine.
   - Include pending steps even before a run exists.

4. [x] **Implement Fan-Out Branch Renderer**
   - Render fan-out steps as branches.
   - Show aggregate status and target count.
   - Wire `Retry failed` to existing fan-out retry response.

5. [x] **Inline Approval and Promotion**
   - Show approval actions inline on paused approval runs.
   - Show `Save as agent` only for completed one-shot Command Agent runs.

6. [x] **Polish Empty, Overflow, and Responsive States**
   - Keep rail useful at narrow widths.
   - Ensure long prompts/target names truncate cleanly.
   - Respect reduced-motion preferences for pulse animation.

7. [ ] **QA**
   - Sequential: `run forge and then lens`
   - Fan-out: `run lens across all child tasks`
   - One-shot: doc update/create command
   - Failed sequential retry
   - Failed fan-out retry
   - Cancel active sequential plan
   - Cancel active fan-out plan
   - Page refresh while plan is active
   - Waiting-for-approval run

## Non-Goals For This Pass

- mixed DAG rendering with join nodes
- arbitrary branch labels beyond target metadata
- standalone approval-gate nodes
- a new backend parent workflow
- new command-bar scheduling semantics
- redesigning the Cmd+K parse confirmation card

## Acceptance Criteria

- Sequential plans read visually as ordered execution.
- Fan-out plans read visually as one grouped plan with multiple concrete target branches.
- The rail clearly separates actionable failures/approvals from passive history.
- One-shot Command Agent runs are visibly different from saved-agent runs.
- Promotion is available for completed one-shot runs without cluttering normal runs.
- Retry/cancel behavior remains identical to the backend contract.
- No UI state implies unsupported DAG/join behavior.
- TypeScript build passes.
