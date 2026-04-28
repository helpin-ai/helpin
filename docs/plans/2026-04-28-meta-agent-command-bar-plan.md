# Meta-Agent Command Bar — PRD Plan

**Date:** 2026-04-28
**Status:** v1 command-bar launcher plus durable multi-step plan controls implemented; follow-up UX polish remains
**Altitude:** Product / architecture, grounded in current codebase. Also tracks implementation status.

---

## Summary

Add a persistent command bar to the workspace shell that lets users express intent in natural language ("summarize this and create a doc", "run Code Builder then Review Agent"). The bar resolves the current page's entity context, parses intent into a confirmable plan, and dispatches one or more agent runs through the existing agent-run pipeline. A right-rail run surface shows what's executing and where it's paused for approval.

The bar is an *accelerator* for power users, not a replacement for per-page agent buttons. New users keep the buttons; advanced users get composition and ephemeral overrides through the bar.

## Implementation status

Implemented in this branch:

- Command-bar backend endpoints:
  - `POST /api/command-bar/intents/parse`
  - `POST /api/command-bar/plans/dispatch`
  - `GET /api/command-bar/plans`
  - `POST /api/command-bar/plans/{planID}/cancel`
  - `POST /api/command-bar/plans/{planID}/retry`
  - `POST /api/command-bar/runs/{runID}/promote-agent`
  - `GET /api/command-bar/unmet-intents`
  - `POST /api/command-bar/unmet-intents/{intentID}/review`
- `command_bar` trigger metadata on `AgentRunInputPayload`.
- `command_bar_plans` and `command_bar_unmet_intents` models, repositories, and forward dbmigrate migrations.
- `PageContextProvider`, `usePageContext()`, and route/page registrations for workspace fallback, task panel, epic detail, docs document detail, CRM contact detail, and CRM deal detail.
- Cmd+K command palette integration: "Ask agents", visible plan rows, confirm action, and `no_matching_agent` messaging.
- Right-side command-run rail with persisted plan reload, status, approve, cancel, retry, save-agent, and existing run detail drawer.
- Parser behavior for known available agents only, with explicit `no_matching_agent` logging.
- Explicit named-agent plans preserve request order. Example: `run Forge and then Lens` returns Forge then Lens.
- Dispatch starts the first plan step through `AgentService.startTargetRun`; later steps are started after the previous command-bar run completes.
- Later command-bar runs are linked to the previous run via `parent_run_id` and share a command-bar `plan_id`.
- `command_bar_plans` durable ledger with plan status, current step, step list, and `run_ids_by_step`.
- The run rail reloads recent command-bar plans after page refresh and groups runs into a visible plan, including pending steps that have not started yet.
- Whole-plan cancel and retry-from-current-step controls.
- Plan fan-out guardrails: parser/dispatch cap at 5 steps and the confirm action shows the run count.
- Runtime `allowed_tools` plumbing on command-bar steps and run input, validated server-side against the selected agent allowlist.
- Backend target expansion for `document`, `crm_contact`, and `crm_deal`, with page context registered on docs, CRM contact, and CRM deal detail pages.
- Unmet-intent review API (`list` + `review`) with status/notes fields.
- Opt-in promotion of a completed command-bar run to a reusable custom agent.
- New product-owned `Researcher` preset definition for broad research/doc/CRM one-shot-style work.

Current caveats:

- Multi-step command-bar dispatch is sequential, but it is still not a durable parent/child Temporal workflow. The sequencing is implemented as a post-completion activity on each normal `AgentRunWorkflow` that starts the next run if needed.
- Tool-subset picker UI is not implemented. The backend/type plumbing exists, but users cannot yet choose tools from Cmd+K.
- Unmet-intent review has backend APIs, but no dedicated admin UI and no redaction policy metadata yet.
- Promotion UI is intentionally minimal in the rail; it prompts for a name and uses backend defaults for description/tools/targets.

Verification completed:

- `go test ./internal/service -run 'TestParseExplicitNamedAgents|TestCommandBarAdditionalContext'`
- `go test ./internal/temporalapp -run 'TestAgentRunWorkflow'`
- `go build ./cmd/temporal-worker ./cmd/api`
- `npm run build`
- `git diff --check`

Migration note:

- New dbmigrate files are forward-only and do not rename or edit applied production migrations:
  - `202604280004_command_bar_plans.sql`
  - `202604280005_command_bar_crm_contact_targets.sql`
  - `202604280006_command_bar_unmet_intent_review.sql`

Known unrelated verification noise:

- Broader sqlite-backed Go suites currently fail on existing fixture/schema drift, including missing `crm_contacts.email_status` and `pm_workflow_states.position`. Those failures are outside the command-bar changes.

## Goals

- Single keystroke (Cmd+K) entry point to invoke any agent against the current page's entity.
- Natural-language intent → visible multi-step plan → confirm → dispatch. No silent fan-out.
- Ephemeral instruction overrides and tool-subset narrowing without creating a saved agent.
- Reuse the existing `AgentRun` start path used by manual runs and automation rules. No parallel orchestrator.

## Non-Goals (v1)

- Full saved-custom-agent registry, versioning, and editor UI. v1 only includes opt-in promotion from a completed command-bar run.
- Agents spawning other agents mid-flow.
- True multi-agent DAG orchestration via parent/child Temporal workflows.
- Structured diff proposals (propose → diff → approve → apply).
- Unifying the bar with `@agent` mentions in comments.
- Cross-workspace orchestration.
- One-shot dynamic agents that do not map to an existing preset.

These are real and worth doing — they're sequenced after v1 once v1 surfaces what users actually ask for.

## Grounding (what already exists)

Mapped against `/root/helpin/server` and `/root/helpin/frontend` as of 2026-04-28.

| Capability | State | Path |
|---|---|---|
| `Agent` model with `is_system`, `trigger_mode`, `allowed_tools/commands/targets` | Solid | `server/internal/model/agent.go:33` |
| System agent presets (Epic Planner, Task Planner, CRM Operator, Support Agent, Code Builder, Review Agent, Researcher) | Solid | `server/internal/service/agent_presets.go:25` |
| `AgentRunInputPayload` with `Trigger` / `Target` / `Event` / `Output` fields | Solid | `server/internal/model/agent.go:536` |
| Single canonical `AgentRunWorkflow` with approve/message/resume signals | Solid | `server/internal/temporalapp/workflow.go:38` |
| Automation-rule engine that resolves a target and starts an agent run through `AgentService.startTargetRun` | Solid | `server/internal/service/automation_rule_engine.go:395`, `server/internal/service/agent.go:2228` |
| Approval gate (`ApprovalState`, `awaiting_approval`, `PauseReason`) | Solid (binary) | `server/internal/model/agent.go:130`, `temporalapp/workflow.go:98` |
| Per-agent tool allowlists | Solid | `server/internal/model/agent.go:59`, `server/internal/service/agent_presets.go` |
| Tool catalog endpoint | Exists; per-agent picker UX still not implemented | `server/internal/service/agent.go:1638`, `server/internal/router/router.go:484`, `server/internal/router/router.go:857` |
| Generic target run launch | Solid for `task/story`, `epic`, `repository`, `support_conversation`, `workspace`, `document`, `crm_contact`, `crm_deal` | `server/internal/service/agent.go` |
| `cmdk` UI primitives | Implemented for search + agent plan entry | `frontend/src/components/ui/command.tsx`, `frontend/src/components/search/SearchCommandPalette.tsx` |
| Route-level entity loading via TanStack Query hooks | Solid; command-bar page context implemented for workspace/task/epic/docs/CRM detail surfaces | `frontend/src/components/command-bar/pageContext.tsx` |
| `@`-mention extraction for comments (notifies users only) | Solid | `server/internal/service/pm_mention.go` |
| Parent/child workflow composition for multi-agent DAGs | Does not exist | — |
| Structured diff proposal entity | Does not exist | — |
| Comment slash-command / agent invocation parser | Does not exist | — |

The headline: the v1 assembly work is now in place for supported targets, including durable sequential plan state and whole-plan controls. The remaining greenfield work is mostly product polish: tool-subset picker UI, review/admin UX for unmet intents, and a more deliberate promotion dialog.

## Architecture

### Pillar 1 — `PageContext` as a first-class concept

Every workspace route declares the entity it represents:

```ts
type PageContext = {
  entityType: 'task' | 'epic' | 'document' | 'crm_contact' | 'crm_deal' | 'workspace';
  entityId: string;
  displayTitle: string;
  relatedIds?: { storyIds?: string[]; prIds?: string[]; /* ... */ };
};
```

Frontend exposes `usePageContext()`. The bar reads it and passes it as command-bar page context; dispatch revalidates and then starts runs through the existing target-run path, which builds `AgentRunInputPayload.Target`.

**v1 routes:** story/task detail, epic detail, docs document detail, CRM contact detail, CRM deal detail, plus workspace fallback. This is implemented.

**Naming rule:** the UI may keep saying "story" where the product does, but the backend canonical run target is `task`. Normalize `story → task` before dispatch.

**Architectural rule:** routes that don't register a `PageContext` get a workspace-scoped fallback (`{ entityType: 'workspace', entityId }`). The bar is never invisible — it just has less context to bind.

### Pillar 2 — Intent parser as a stateless LLM call, not a meta-agent workflow

The bar calls `POST /api/command-bar/intents/parse` with `{ text, page_context }`. The backend loads available agents server-side, filters them by target, and gets back:

```json
{
  "plan": [
    { "agentKey": "code_builder", "target": {...}, "instructions": "implement..." },
    { "agentKey": "review_agent", "target": {...}, "instructions": "QA the change" }
  ],
  "rationale": "Detected implement-then-review pattern."
}
```

The plan renders as ordered rows in the bar. User confirms. Backend starts the first agent run through the existing `AgentService.startTargetRun` path. Each completed command-bar run triggers a retryable Temporal activity that starts the next run, using the previous run as `parent_run_id`.

Current implementation note: N-step plans do not fan out on confirmation. The raw user prompt and full step list stay in trigger metadata; each run receives only its step-scoped execution instruction in `AdditionalContext`.

**Authorization rule:** parse output is advisory only. The confirm/dispatch endpoint must re-check workspace access, target access, agent runnability, allowed target type, team scope, and tool-subset constraints server-side.

**Why not a Temporal meta-workflow?** `AgentRunWorkflow` is single-run today. Building a parent workflow that fans out to children is real new infra and we don't yet know if we need cross-run signaling, shared context handoff, or saga-style undo. Defer until sequential dispatch demonstrably breaks.

### Pillar 3 — Reuse the same run-start primitive as automation rules

The automation-rule engine already does *trigger → resolve target → start agent run* by calling `AgentService.startTargetRun`. The command bar should use that same service primitive with a new trigger source (`command_bar`), not create fake automation rules and not fork run creation.

In other words: reuse the durable run path and validation machinery, but keep command-bar parsing and confirmation as its own product surface. This is implemented.

### Pillar 4 — Runtime overrides before saved custom agents

`Agent.AllowedTools/Commands/Targets` and `AgentRunInputPayload.AdditionalContext` / `AllowedTools` already exist. Ephemeral overrides are *purely runtime*: pass extra instructions and an optional `tool_subset` (must be ⊆ base agent's allowlist) on the run input. Prefer using existing payload fields for v1 unless the UX needs a separate `extra_instructions` field for auditability.

Current implementation note: parsed instructions are passed as additional context. Runtime `allowed_tools` is wired through the parser shape, dispatch request, run input, and server-side subset validation. Tool-subset picker UX is not implemented yet.

**Architectural boundary:** running with runtime overrides is cheap; saving an agent is a product surface. v1 supports known-agent runtime overrides and explicit post-run promotion only. v2 can add non-persistent one-shot dynamic runs through a product-owned broad preset. Saved custom agents are introduced through explicit user promotion after a successful run, not automatically before value is proven.

### Pillar 5 — Run-stream right rail

The bar without a visible run surface is a black hole. `AgentRun` already carries `Status`, `PauseReason`, `OutputSummary`. v1 ships a right-rail "Activity" panel that:

- Streams runs the user kicked off in this session
- Shows command-bar plans as ordered steps, including pending steps that are waiting for the previous step to complete
- Shows current stage and pause reason
- Surfaces the approve/reject affordance when `awaiting_approval`
- Links to the existing run detail page for deep inspection

New UI, existing data. This is implemented as `CommandBarRunRail`.

### Pillar 6 — Approval stays binary in v1

`ApprovalState` is binary; `OutputSummary` is an opaque JSON blob. Structured propose-→diff-→apply requires a new `agent_proposal` entity, per-entity-type diff renderers, and an apply service. That is its own project.

v1 PRD position: "approve the run, see the summary." Set this expectation in the launch comms — do not imply diff review.

### Pillar 7 — `@`-mention parity is v2

Mentions today only notify users. Unifying the bar with `@code_builder do X` in comments is the right end state but adds a comment-side intent parser and changes notification semantics. Park it.

## Version strategy

### V1 — Command bar for known agents

v1 is a safer launcher for existing agents. The parser returns either a confirmable `plan` using available agents or `no_matching_agent`. Confirmed plan steps create normal `AgentRun` rows and normal `AgentRunWorkflow` executions. Nothing durable is created when the user only opens Cmd+K, types, parses, or dismisses the plan.

v1 does not automatically create custom agents, does not run a generic one-shot executor, and does not choose the closest preset when no preset actually fits. The supported target set remains bounded by `StartTargetRun`: `task/story`, `epic`, `workspace`, `document`, `crm_contact`, and `crm_deal`.

Status: implemented.

### V2 — One-shot dynamic runs

v2 adds a product-owned broad preset for recurring unmet categories that do not fit the original six seeded agents. The `Researcher` preset definition now exists as that broad product-owned executor. The parser may return a `one_shot_plan` with proposed tools, target, and instructions after the UX is ready. The user confirms the tool subset and instructions before dispatch.

The backend still starts a normal `AgentRun` against a real agent ID. The run uses runtime instructions and `AllowedTools` on the run input. Nothing is saved as a reusable custom agent by default.

The canonical motivating example is "check the web and update stale doc sections." Backend document target support now exists; the remaining blocker is the product UX for exposing this as a deliberate one-shot command rather than silently routing arbitrary no-match prompts to a broad executor.

### V2.5 — Promote one-shot to saved agent

After a successful command-bar run, the right rail offers "Save agent" to users who can access the endpoint. Promotion requires an explicit name, copies runtime/provider/model/commands from the source agent, uses the step tool subset or source tools, scopes targets to the source run target, and stores provenance in the generated role text.

Promotion is opt-in and post-run. The user is the dedupe mechanism for v2.5: only runs that proved useful get persisted.

Status: backend implemented with a minimal rail prompt. Follow-up UX should replace `window.prompt` with a dialog for name, description, tool set, target set, and provenance preview.

### Future reference only — auto-create / auto-reuse saved agents

Automatic "no agent matched, so create and save the right one, then run it" is not implemented in this plan. It is a future-reference direction only.

Before this should be considered, the product needs semantic dedupe against existing agents, naming rules, registry quality controls, ownership and permission policy, archive/cleanup of unused generated agents, eval/test-run support, visible versioning/provenance, and a clear policy for reusing an existing saved agent versus creating a new one.

## Handling unmet intents

v1 must be honest when the available roster cannot do the job. The intent parser returns `no_matching_agent` with a short reason instead of silently dispatching the closest preset.

Unmet intents should be logged with the raw prompt, normalized page context, matched candidates, reason, and a redaction policy for sensitive entity text. A review loop uses this data to decide whether to add a tool to an existing preset, add a narrow new preset, or graduate a recurring category into v2 one-shot dynamic runs.

The UI response should be explicit: "No available agent can do that yet." It can suggest supported alternatives, but it should not imply that the current roster can perform the requested action.

Status: implemented, except redaction policy metadata and a dedicated review/admin UI are still follow-up work.

## What is implemented now

- [x] **Known-agent command bar.** Cmd+K can parse supported intents into a confirmable plan for available agents.
- [x] **Supported page context.** Workspace fallback, task detail/panel context, and epic detail context are wired.
- [x] **Honest no-match behavior.** Unsupported requests return `no_matching_agent` and log unmet intent data.
- [x] **Step-scoped agent context.** Raw Cmd+K prompts stay in trigger metadata; each agent receives only its current step instruction in `AdditionalContext`.
- [x] **Sequential multi-step execution.** Confirm starts step 1 only. Completion of step N starts step N+1 through a retryable post-completion Temporal activity.
- [x] **Plan linkage.** Runs share a command-bar `plan_id`, carry the full plan in trigger metadata, and link later steps with `parent_run_id`.
- [x] **Durable plan ledger.** `command_bar_plans` persists prompt, page context, steps, current step, status, and `run_ids_by_step`.
- [x] **Visible run rail.** The right rail reloads recent plans, groups command-bar runs by plan, and shows pending, running, completed, paused, and actionable runs.
- [x] **Whole-plan controls.** Cancel marks the plan cancelled, cancels active runs, and prevents pending steps from starting. Retry restarts a failed/cancelled plan from the current step.
- [x] **Guardrails.** Plans are capped at 5 runs and the confirm action shows the run count before dispatch.
- [x] **Target expansion.** `document`, `crm_contact`, and `crm_deal` targets are supported in backend dispatch and registered on detail pages.
- [x] **Tool-subset backend plumbing.** Step-level `allowed_tools` is carried into run input and validated as a subset of the selected agent's allowlist.
- [x] **Unmet-intent review API.** Unmet intents can be listed and marked `open`, `accepted`, `rejected`, or `deferred` with notes.
- [x] **Opt-in promotion backend.** Completed command-bar runs can be promoted to saved custom agents.
- [x] **Researcher preset definition.** Broad research/doc/CRM preset configuration is present for the v2 one-shot direction.
- [x] **Backend/frontend types.** Dispatch responses include `plan_id`, `steps`, `run_count`, and started runs.

## What is next

1. **Tool-subset picker UX.** Let users inspect and narrow tools per step in Cmd+K. Backend validation already exists.
2. **Promotion dialog.** Replace the rail's minimal name prompt with a proper dialog for name, description, tool set, target set, and provenance preview.
3. **Unmet-intent review UI + redaction.** Add a settings/admin surface for reviewing unmet intents, plus explicit redaction metadata/policy.
4. **Researcher exposure.** Decide when the parser may choose the new Researcher preset directly versus returning `no_matching_agent` and logging the unmet intent.
5. **Plan status realtime polish.** Reload works and run events update runs, but plan status itself should be pushed/refetched after terminal transitions instead of relying on list refresh or local action responses.
6. **Broader fan-out policy.** The hard cap is implemented. Future fan-out across multiple selected targets still needs explicit target enumeration and cost estimates before dispatch.

## Sequenced bets (high-level)

- [x] **`PageContext` + bar wired on story/task detail, epic detail, and workspace fallback.** Reuses `cmdk`, existing `startTargetRun`, existing approval gate.
- [x] **Run-stream right rail.** Shows persisted command-bar plans, loaded runs, status, approval affordance, cancel, retry, save-agent, and existing run detail drawer.
- [x] **Intent parser → visible plan with confirm.** Parser can choose the agent and instructions; explicit named-agent requests preserve order.
- [x] **Unmet-intent response and logging.** Returns `no_matching_agent` instead of choosing the closest preset; logs prompt/context/candidates/reason.
- [x] **Sequential multi-step plan.** Confirm starts step 1 only; each completed command-bar run advances the next step through a retryable post-completion activity. Runs are linked with `parent_run_id` and grouped by command-bar `plan_id`.
- [x] **Step context separation.** Full user prompt and full plan are stored as trigger metadata; per-agent execution context contains only the current step instruction and page context.
- [~] **Runtime instruction overrides + tool-subset picker.** Additional instructions and server-side tool-subset enforcement are implemented. The picker UI is still pending.
- [x] **Expand supported routes.** Docs and CRM detail pages now register command-bar context, backed by `document`, `crm_contact`, and `crm_deal` target support.
- [x] **Whole-plan controls.** Persisted plan state supports reload-after-refresh, cancel-rest, and retry from current failed/cancelled step.
- [~] **(v2)** One-shot dynamic runs through a product-owned broad preset; the Researcher preset exists, but parser/UX exposure is still pending.
- [~] **(v2.5)** Opt-in promotion of successful command-bar runs to saved custom agents is implemented with minimal UI; richer promotion UX is pending.
- [ ] **Future reference only** Automatic saved-agent creation/reuse is not a delivery phase in this plan.

No week estimates here — the point of this doc is direction, not a schedule.

## Open follow-ups

- **Per-agent tool catalog shape.** Still open. A broad tool catalog already exists. The picker needs a filtered view for a selected agent or preset, including labels, categories, and disabled reasons for tools outside the base allowlist.
- **Command-bar trigger contract.** Implemented baseline: `command_bar` trigger source/type, plan ID, raw prompt, page context, full steps list, run count, and step index. Still open: parsed plan hash/version.
- **Unmet-intent log shape.** Implemented as dedicated `command_bar_unmet_intents` table with prompt, workspace, actor, page context, candidate agents, reason, status, review notes, and reviewed timestamp. Still open: redaction metadata/policy.
- **Target expansion.** Implemented for `document`, `crm_contact`, and `crm_deal`. Still open: richer per-target context hydration beyond the baseline target payload.
- **Sequential vs parallel dispatch semantics in the plan.** Resolved for v1: command-bar plans are sequential by default. Confirm starts only the first step, and completion of step N starts step N+1. A full parent/child Temporal plan workflow remains deferred unless this lightweight scheduler proves insufficient.
- **Authorization and privacy boundary.** Decide how much entity data is sent to the intent parser. The dispatch endpoint must treat parser output as untrusted and re-validate all target, agent, team, and tool constraints.
- **One-shot preset shape for v2.** Default recommendation: use a product-owned broad preset (`doc_researcher` or `general_researcher`) instead of arbitrary custom-agent creation.
- **Promotion permission.** Current endpoint is settings-managed. Confirm whether this should become a narrower agent-create permission before broader rollout.
- **Cost / rate-limit guardrails.** Basic hard cap and run-count display are implemented. Future multi-target expansion still needs cost bands and per-user invocation budgets.
- **Per-page agent buttons vs. bar suggested prompts.** Long-term, contextual suggested prompts in the bar may replace the button soup. v1 keeps both. Decide measurement criteria before sunsetting buttons.
- **Cancellation.** Whole-plan cancel is implemented. Still open: decide whether cancelling an individual run inside a plan should implicitly cancel the remaining plan or stay run-scoped.

## Risks

- **Wrong-context bug erodes trust fast.** If `PageContext` resolves to the wrong entity even occasionally, users stop trusting the bar within days. Invest in the resolver before the parser.
- **Plan parsing ambiguity.** Natural language → DAG is the part most likely to feel magical-or-broken. The chip-based confirm step is the safety net; do not skip it even when the plan looks obvious.
- **Closest-preset mismatch.** If no agent fits, routing to a vaguely related preset is worse than refusing. v1 should fail honestly with `no_matching_agent`.
- **Target promise boundary.** Docs and CRM target dispatch is now supported for `document`, `crm_contact`, and `crm_deal`. The remaining risk is over-promising richer target-specific context or mutation behavior beyond the baseline run target payload.
- **Agent sprawl from auto-save.** Automatically saving agents from command prompts creates overlapping, poorly named registry entries. This is why auto-create/reuse is future-reference only, not a v2 deliverable.
- **Output blob honesty.** Without diffs, "run completed" with a JSON summary will feel underwhelming on mutation-heavy actions. Pick the v1 demo flows carefully (summarize, draft, plan) over (implement, mutate).
- **Agent name vocabulary drift.** Earlier discussion floated Atlas/Forge/Lens; actual presets are Epic Planner / Task Planner / CRM Operator / Support Agent / Code Builder / Review Agent. PRD and UI must use one set. Renaming presets is a separate decision.

## What v1 explicitly does not promise

- Diff review before applying changes
- Full saved-custom-agent registry/editor/versioning. Opt-in promotion from completed command-bar runs is implemented.
- One-shot dynamic runs outside the existing agent roster
- Automatic saved-agent creation/reuse
- Cross-agent context handoff (runs are linked by `parent_run_id`, but no automatic output handoff exists)
- `@agent` invocation in comments
- Spawning agents from agents

One-shot dynamic runs remain a v2 candidate. Opt-in promotion is implemented with minimal UI and still needs a proper promotion dialog. Automatic saved-agent creation/reuse is future reference only and is not implemented by this plan.
