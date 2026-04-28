# Meta-Agent Command Bar — PRD Plan

**Date:** 2026-04-28
**Status:** Implemented v1 foundation; follow-up items remain
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
- `command_bar` trigger metadata on `AgentRunInputPayload`.
- `command_bar_unmet_intents` model, repository, and migration.
- `PageContextProvider`, `usePageContext()`, and route/page registrations for workspace fallback, task panel, and epic detail.
- Cmd+K command palette integration: "Ask agents", visible plan rows, confirm action, and `no_matching_agent` messaging.
- Right-side command-run rail with current-session runs, status, approve, cancel, and existing run detail drawer.
- Parser behavior for known available agents only, with explicit `no_matching_agent` logging.
- Explicit named-agent plans preserve request order. Example: `run Forge and then Lens` returns Forge then Lens.
- Dispatch creates one normal `AgentRun` per plan step through `AgentService.startTargetRun`.
- Later command-bar runs are linked to the previous run via `parent_run_id`.

Current caveat:

- Multi-step command-bar dispatch creates the ordered run rows immediately. It does **not** yet wait for step 1 to complete before starting step 2, and it is not a durable parent/child Temporal workflow. The plan is visible and linked, but not truly scheduled sequentially.

Verification completed:

- `go test ./internal/service -run 'TestParseExplicitNamedAgents|TestCommandBar'`
- `go test ./cmd/api ./internal/model ./internal/handler ./internal/router`
- `go build ./cmd/api`
- `npm run build`
- `git diff --check`

## Goals

- Single keystroke (Cmd+K) entry point to invoke any agent against the current page's entity.
- Natural-language intent → visible multi-step plan → confirm → dispatch. No silent fan-out.
- Ephemeral instruction overrides and tool-subset narrowing without creating a saved agent.
- Reuse the existing `AgentRun` start path used by manual runs and automation rules. No parallel orchestrator.

## Non-Goals (v1)

- Saved custom agents with their own registry, versioning, and editor UI.
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
| Six seeded system agents (Epic Planner, Task Planner, CRM Operator, Support Agent, Code Builder, Review Agent) | Solid | `server/internal/service/agent_presets.go:25` |
| `AgentRunInputPayload` with `Trigger` / `Target` / `Event` / `Output` fields | Solid | `server/internal/model/agent.go:536` |
| Single canonical `AgentRunWorkflow` with approve/message/resume signals | Solid | `server/internal/temporalapp/workflow.go:38` |
| Automation-rule engine that resolves a target and starts an agent run through `AgentService.startTargetRun` | Solid | `server/internal/service/automation_rule_engine.go:395`, `server/internal/service/agent.go:2228` |
| Approval gate (`ApprovalState`, `awaiting_approval`, `PauseReason`) | Solid (binary) | `server/internal/model/agent.go:130`, `temporalapp/workflow.go:98` |
| Per-agent tool allowlists | Solid | `server/internal/model/agent.go:59`, `server/internal/service/agent_presets.go` |
| Tool catalog endpoint | Exists; per-agent picker UX still not implemented | `server/internal/service/agent.go:1638`, `server/internal/router/router.go:484`, `server/internal/router/router.go:857` |
| Generic target run launch | Solid for `task/story`, `epic`, `repository`, `support_conversation`, `workspace`; missing `document`, `crm_contact`, `crm_deal` | `server/internal/service/agent.go:2228` |
| `cmdk` UI primitives | Implemented for search + agent plan entry | `frontend/src/components/ui/command.tsx`, `frontend/src/components/search/SearchCommandPalette.tsx` |
| Route-level entity loading via TanStack Query hooks | Solid; command-bar page context implemented for workspace/task/epic | `frontend/src/components/command-bar/pageContext.tsx` |
| `@`-mention extraction for comments (notifies users only) | Solid | `server/internal/service/pm_mention.go` |
| Parent/child workflow composition for multi-agent DAGs | Does not exist | — |
| Structured diff proposal entity | Does not exist | — |
| Comment slash-command / agent invocation parser | Does not exist | — |

The headline: the v1 assembly work is now in place for supported targets. The remaining greenfield work is true sequential orchestration, runtime tool-subset UX, and target expansion beyond the types `StartTargetRun` supports today.

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

**v1 routes:** story/task detail and epic detail first, plus workspace fallback. This is implemented. Docs and CRM surfaces are sequenced after backend target support exists for `document`, `crm_contact`, and `crm_deal`.

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

The plan renders as ordered rows in the bar. User confirms. Backend creates N agent runs through the existing `AgentService.startTargetRun` path.

Current implementation note: N-step plans create N runs immediately and link each later run to the previous run via `parent_run_id`. This is ordered metadata, not a durable sequential scheduler.

**Authorization rule:** parse output is advisory only. The confirm/dispatch endpoint must re-check workspace access, target access, agent runnability, allowed target type, team scope, and tool-subset constraints server-side.

**Why not a Temporal meta-workflow?** `AgentRunWorkflow` is single-run today. Building a parent workflow that fans out to children is real new infra and we don't yet know if we need cross-run signaling, shared context handoff, or saga-style undo. Defer until sequential dispatch demonstrably breaks.

### Pillar 3 — Reuse the same run-start primitive as automation rules

The automation-rule engine already does *trigger → resolve target → start agent run* by calling `AgentService.startTargetRun`. The command bar should use that same service primitive with a new trigger source (`command_bar`), not create fake automation rules and not fork run creation.

In other words: reuse the durable run path and validation machinery, but keep command-bar parsing and confirmation as its own product surface. This is implemented.

### Pillar 4 — Runtime overrides before saved custom agents

`Agent.AllowedTools/Commands/Targets` and `AgentRunInputPayload.AdditionalContext` / `AllowedTools` already exist. Ephemeral overrides are *purely runtime*: pass extra instructions and an optional `tool_subset` (must be ⊆ base agent's allowlist) on the run input. Prefer using existing payload fields for v1 unless the UX needs a separate `extra_instructions` field for auditability.

Current implementation note: parsed instructions are passed as additional context. Tool-subset picker UX is not implemented yet.

**Architectural boundary:** running with runtime overrides is cheap; saving an agent is a product surface. v1 should support known-agent runtime overrides only. v2 can add non-persistent one-shot dynamic runs through a product-owned broad preset. Saved custom agents are only introduced through explicit user promotion after a successful one-shot run, not automatically before value is proven.

### Pillar 5 — Run-stream right rail

The bar without a visible run surface is a black hole. `AgentRun` already carries `Status`, `PauseReason`, `OutputSummary`. v1 ships a right-rail "Activity" panel that:

- Streams runs the user kicked off in this session
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

v1 does not create custom agents, does not run a generic one-shot executor, and does not choose the closest preset when no preset actually fits. The supported target set remains bounded by `StartTargetRun`: `task/story`, `epic`, and `workspace` for the initial command-bar surfaces.

Status: implemented.

### V2 — One-shot dynamic runs

v2 adds a product-owned broad preset, likely `doc_researcher` or `general_researcher`, for recurring unmet categories that do not fit the six seeded agents. The parser may return a `one_shot_plan` with proposed tools, target, and instructions. The user confirms the tool subset and instructions before dispatch.

The backend still starts a normal `AgentRun` against a real agent ID. The run uses runtime instructions and `AllowedTools` on the run input. Nothing is saved as a reusable custom agent by default.

The canonical motivating example is "check the web and update stale doc sections." That category should wait for document target support before being exposed as a first-class command target.

### V2.5 — Promote one-shot to saved agent

After a successful one-shot run, the right rail can offer "Save as reusable agent" to users with agent-create permission. Promotion requires an explicit name, description, tool set, target set, and provenance from the source run/prompt. The saved agent then appears in future parser `availableAgents`.

Promotion is opt-in and post-run. The user is the dedupe mechanism for v2.5: only runs that proved useful get persisted.

### Future reference only — auto-create / auto-reuse saved agents

Automatic "no agent matched, so create and save the right one, then run it" is not implemented in this plan. It is a future-reference direction only.

Before this should be considered, the product needs semantic dedupe against existing agents, naming rules, registry quality controls, ownership and permission policy, archive/cleanup of unused generated agents, eval/test-run support, visible versioning/provenance, and a clear policy for reusing an existing saved agent versus creating a new one.

## Handling unmet intents

v1 must be honest when the available roster cannot do the job. The intent parser returns `no_matching_agent` with a short reason instead of silently dispatching the closest preset.

Unmet intents should be logged with the raw prompt, normalized page context, matched candidates, reason, and a redaction policy for sensitive entity text. A review loop uses this data to decide whether to add a tool to an existing preset, add a narrow new preset, or graduate a recurring category into v2 one-shot dynamic runs.

The UI response should be explicit: "No available agent can do that yet." It can suggest supported alternatives, but it should not imply that the current roster can perform the requested action.

Status: implemented, except redaction policy and review tooling are still follow-up work.

## Sequenced bets (high-level)

- [x] **`PageContext` + bar wired on story/task detail, epic detail, and workspace fallback.** Reuses `cmdk`, existing `startTargetRun`, existing approval gate.
- [x] **Run-stream right rail.** Shows current-session runs, status, approval affordance, cancel, and existing run detail drawer.
- [x] **Intent parser → visible plan with confirm.** Parser can choose the agent and instructions; explicit named-agent requests preserve order.
- [x] **Unmet-intent response and logging.** Returns `no_matching_agent` instead of choosing the closest preset; logs prompt/context/candidates/reason.
- [~] **Sequential multi-step plan.** Implemented as ordered multi-run dispatch with `parent_run_id` links and per-step metadata. Not yet a durable sequential scheduler; steps currently start immediately.
- [~] **Runtime instruction overrides + tool-subset picker.** Additional instructions are passed through run context. Tool-subset picker and server-side subset field enforcement are still pending.
- [~] **Expand supported routes.** Epic is implemented. Docs and CRM still require explicit backend target support first.
- [ ] **(v2)** One-shot dynamic runs through a product-owned broad preset; no saved agent by default.
- [ ] **(v2.5)** Opt-in promotion of successful one-shot runs to saved custom agents.
- [ ] **Future reference only** Automatic saved-agent creation/reuse is not a delivery phase in this plan.

No week estimates here — the point of this doc is direction, not a schedule.

## Open architectural questions to resolve before build

- **Per-agent tool catalog shape.** Still open. A broad tool catalog already exists. The picker needs a filtered view for a selected agent or preset, including labels, categories, and disabled reasons for tools outside the base allowlist.
- **Command-bar trigger contract.** Implemented baseline: `command_bar` trigger source/type, raw prompt, page context, full steps list, run count, and step index. Still open: parsed plan ID/hash if we later persist plans separately.
- **Unmet-intent log shape.** Implemented as dedicated `command_bar_unmet_intents` table with prompt, workspace, actor, page context, candidate agents, and reason. Still open: redaction metadata/policy.
- **Target expansion.** Still open beyond v1. `StartTargetRun` does not currently support `document`, `crm_contact`, or `crm_deal`. Either narrow v1 to supported target types or add resolvers, authorization checks, activity logging, and run context hydration for each new target.
- **Sequential vs parallel dispatch semantics in the plan.** Partially resolved. Explicit named-agent plans are ordered, and created runs are linked. Still open: whether to build a durable scheduler that waits for step completion before starting the next step.
- **Authorization and privacy boundary.** Decide how much entity data is sent to the intent parser. The dispatch endpoint must treat parser output as untrusted and re-validate all target, agent, team, and tool constraints.
- **One-shot preset shape for v2.** Default recommendation: use a product-owned broad preset (`doc_researcher` or `general_researcher`) instead of arbitrary custom-agent creation.
- **Promotion permission for v2.5.** Default recommendation: only users who can create agents may save a one-shot run as reusable.
- **Cost / rate-limit guardrails.** "Run Code Builder on all 12 stories in this epic" can fan out badly. The plan-confirm step should show an estimated run count and cost band; per-user invocation budgets need a decision before launch.
- **Per-page agent buttons vs. bar suggested prompts.** Long-term, contextual suggested prompts in the bar may replace the button soup. v1 keeps both. Decide measurement criteria before sunsetting buttons.
- **Cancellation.** Partially implemented at individual-run level through the rail. Still open for multi-step plan semantics: cancelling one run does not cancel the whole linked plan.

## Risks

- **Wrong-context bug erodes trust fast.** If `PageContext` resolves to the wrong entity even occasionally, users stop trusting the bar within days. Invest in the resolver before the parser.
- **Plan parsing ambiguity.** Natural language → DAG is the part most likely to feel magical-or-broken. The chip-based confirm step is the safety net; do not skip it even when the plan looks obvious.
- **Closest-preset mismatch.** If no agent fits, routing to a vaguely related preset is worse than refusing. v1 should fail honestly with `no_matching_agent`.
- **Unsupported target promise.** Docs and CRM pages will look easy because they can produce `PageContext`, but dispatch will fail until the backend supports those target types. Do not expose them as first-class v1 command targets prematurely.
- **Agent sprawl from auto-save.** Automatically saving agents from command prompts creates overlapping, poorly named registry entries. This is why auto-create/reuse is future-reference only, not a v2 deliverable.
- **Output blob honesty.** Without diffs, "run completed" with a JSON summary will feel underwhelming on mutation-heavy actions. Pick the v1 demo flows carefully (summarize, draft, plan) over (implement, mutate).
- **Agent name vocabulary drift.** Earlier discussion floated Atlas/Forge/Lens; actual presets are Epic Planner / Task Planner / CRM Operator / Support Agent / Code Builder / Review Agent. PRD and UI must use one set. Renaming presets is a separate decision.

## What v1 explicitly does not promise

- Diff review before applying changes
- Saved custom agents
- One-shot dynamic runs outside the existing agent roster
- Automatic saved-agent creation/reuse
- Cross-agent context handoff (runs are linked by `parent_run_id`, but no automatic output handoff exists)
- `@agent` invocation in comments
- Spawning agents from agents

One-shot dynamic runs are a v2 candidate. Opt-in promotion is v2.5. Automatic saved-agent creation/reuse is future reference only and is not implemented by this plan.
