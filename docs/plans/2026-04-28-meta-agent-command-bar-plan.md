# Meta-Agent Command Bar — PRD Plan

**Date:** 2026-04-28
**Status:** Draft for review
**Altitude:** Product / architecture, grounded in current codebase. Not an implementation spec.

---

## Summary

Add a persistent command bar to the workspace shell that lets users express intent in natural language ("summarize this and create a doc", "run Code Builder then Review Agent"). The bar resolves the current page's entity context, parses intent into a confirmable plan, and dispatches one or more agent runs through the existing agent-run pipeline. A right-rail run surface shows what's executing and where it's paused for approval.

The bar is an *accelerator* for power users, not a replacement for per-page agent buttons. New users keep the buttons; advanced users get composition and ephemeral overrides through the bar.

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
| Tool catalog endpoint | Exists; needs per-agent filtering for picker UX | `server/internal/service/agent.go:1638`, `server/internal/router/router.go:484`, `server/internal/router/router.go:857` |
| Generic target run launch | Solid for `task/story`, `epic`, `repository`, `support_conversation`, `workspace`; missing `document`, `crm_contact`, `crm_deal` | `server/internal/service/agent.go:2228` |
| `cmdk` UI primitives | Scaffolded | `frontend/src/components/ui/command.tsx`, `components/search/` |
| Route-level entity loading via TanStack Query hooks | Solid (no unified abstraction) | `frontend/src/routes/_authenticated/w/$slug.tsx` |
| `@`-mention extraction for comments (notifies users only) | Solid | `server/internal/service/pm_mention.go` |
| Parent/child workflow composition for multi-agent DAGs | Does not exist | — |
| Structured diff proposal entity | Does not exist | — |
| Comment slash-command / agent invocation parser | Does not exist | — |

The headline: the first slice is mostly an assembly job over existing primitives. The greenfield is intent parsing, page-context resolution, the run-stream surface, command-bar-specific trigger metadata, and target expansion beyond the types `StartTargetRun` supports today.

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

Frontend exposes `usePageContext()`. The bar reads it and passes it as the `target` field on `AgentRunInputPayload` — which already carries `Target`, `Event`, and `Output` blocks.

**v1 routes:** story/task detail and epic detail first, plus workspace fallback. Docs and CRM surfaces are sequenced after backend target support exists for `document`, `crm_contact`, and `crm_deal`.

**Naming rule:** the UI may keep saying "story" where the product does, but the backend canonical run target is `task`. Normalize `story → task` before dispatch.

**Architectural rule:** routes that don't register a `PageContext` get a workspace-scoped fallback (`{ entityType: 'workspace', entityId }`). The bar is never invisible — it just has less context to bind.

### Pillar 2 — Intent parser as a stateless LLM call, not a meta-agent workflow

The bar calls `POST /intents/parse` with `{ text, pageContext, availableAgents }` and gets back:

```json
{
  "plan": [
    { "agentKey": "code_builder", "target": {...}, "instructions": "implement..." },
    { "agentKey": "review_agent", "target": {...}, "instructions": "QA the change" }
  ],
  "rationale": "Detected implement-then-review pattern."
}
```

The plan renders as chips in the bar. User confirms. Backend creates N agent runs through the existing `AgentService.startTargetRun` path — sequentially by default, with parallel dispatch only when the backend and confirmation UI explicitly mark steps independent.

**Authorization rule:** parse output is advisory only. The confirm/dispatch endpoint must re-check workspace access, target access, agent runnability, allowed target type, team scope, and tool-subset constraints server-side.

**Why not a Temporal meta-workflow?** `AgentRunWorkflow` is single-run today. Building a parent workflow that fans out to children is real new infra and we don't yet know if we need cross-run signaling, shared context handoff, or saga-style undo. Defer until sequential dispatch demonstrably breaks.

### Pillar 3 — Reuse the same run-start primitive as automation rules

The automation-rule engine already does *trigger → resolve target → start agent run* by calling `AgentService.startTargetRun`. The command bar should use that same service primitive with a new trigger source (`command_bar`), not create fake automation rules and not fork run creation.

In other words: reuse the durable run path and validation machinery, but keep command-bar parsing and confirmation as its own product surface.

### Pillar 4 — Runtime overrides before saved custom agents

`Agent.AllowedTools/Commands/Targets` and `AgentRunInputPayload.AdditionalContext` / `AllowedTools` already exist. Ephemeral overrides are *purely runtime*: pass extra instructions and an optional `tool_subset` (must be ⊆ base agent's allowlist) on the run input. Prefer using existing payload fields for v1 unless the UX needs a separate `extra_instructions` field for auditability.

**Architectural boundary:** running with runtime overrides is cheap; saving an agent is a product surface. v1 should support known-agent runtime overrides only. v2 can add non-persistent one-shot dynamic runs through a product-owned broad preset. Saved custom agents are only introduced through explicit user promotion after a successful one-shot run, not automatically before value is proven.

### Pillar 5 — Run-stream right rail

The bar without a visible run surface is a black hole. `AgentRun` already carries `Status`, `PauseReason`, `OutputSummary`. v1 ships a right-rail "Activity" panel that:

- Streams runs the user kicked off in this session
- Shows current stage and pause reason
- Surfaces the approve/reject affordance when `awaiting_approval`
- Links to the existing run detail page for deep inspection

New UI, existing data.

### Pillar 6 — Approval stays binary in v1

`ApprovalState` is binary; `OutputSummary` is an opaque JSON blob. Structured propose-→diff-→apply requires a new `agent_proposal` entity, per-entity-type diff renderers, and an apply service. That is its own project.

v1 PRD position: "approve the run, see the summary." Set this expectation in the launch comms — do not imply diff review.

### Pillar 7 — `@`-mention parity is v2

Mentions today only notify users. Unifying the bar with `@code_builder do X` in comments is the right end state but adds a comment-side intent parser and changes notification semantics. Park it.

## Version strategy

### V1 — Command bar for known agents

v1 is a safer launcher for existing agents. The parser returns either a confirmable `plan` using available agents or `no_matching_agent`. Confirmed chips create normal `AgentRun` rows and normal `AgentRunWorkflow` executions. Nothing durable is created when the user only opens Cmd+K, types, parses, or dismisses the plan.

v1 does not create custom agents, does not run a generic one-shot executor, and does not choose the closest preset when no preset actually fits. The supported target set remains bounded by `StartTargetRun`: `task/story`, `epic`, and `workspace` for the initial command-bar surfaces.

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

## Sequenced bets (high-level)

1. **`PageContext` + bar wired on story/task detail only.** Single-agent invocation. Reuses `cmdk`, existing `startTargetRun`, existing approval gate.
2. **Run-stream right rail.** Show current-session runs, pause reasons, approval affordances, cancel, and links to existing run detail before expanding orchestration.
3. **Intent parser → single-step plan with chips and confirm.** The parser may choose the agent and instructions, but dispatch stays one run.
4. **Unmet-intent response and logging.** Return `no_matching_agent` instead of choosing the closest preset; log the prompt/context/reason for roadmap review.
5. **Sequential multi-step plan.** Dispatch one run after another; no parent/child workflow and no implicit fan-out.
6. **Runtime instruction overrides + tool-subset picker.** Use the existing catalog endpoint plus per-agent filtering; enforce subset constraints server-side.
7. **Expand supported routes.** Epic can follow quickly because `StartTargetRun` already supports it. Docs and CRM require explicit backend target support first.
8. **(v2)** One-shot dynamic runs through a product-owned broad preset; no saved agent by default.
9. **(v2.5)** Opt-in promotion of successful one-shot runs to saved custom agents.
10. **Future reference only** Automatic saved-agent creation/reuse is not a delivery phase in this plan.

No week estimates here — the point of this doc is direction, not a schedule.

## Open architectural questions to resolve before build

- **Per-agent tool catalog shape.** A broad tool catalog already exists. The picker needs a filtered view for a selected agent or preset, including labels, categories, and disabled reasons for tools outside the base allowlist.
- **Command-bar trigger contract.** Add a `command_bar` trigger source/type and decide what metadata to persist: raw prompt, parsed plan ID/hash, page context, and confirmed run count.
- **Unmet-intent log shape.** Decide whether this is a dedicated `command_bar_unmet_intents` table or an analytics/event stream. Default recommendation: a dedicated table with prompt, workspace, actor, page context, candidate agents, reason, and redaction metadata.
- **Target expansion.** `StartTargetRun` does not currently support `document`, `crm_contact`, or `crm_deal`. Either narrow v1 to supported target types or add resolvers, authorization checks, activity logging, and run context hydration for each new target.
- **Sequential vs parallel dispatch semantics in the plan.** Does the parser declare ordering, or does the bar always serialize? v1 recommendation: serial unless the plan explicitly marks steps independent.
- **Authorization and privacy boundary.** Decide how much entity data is sent to the intent parser. The dispatch endpoint must treat parser output as untrusted and re-validate all target, agent, team, and tool constraints.
- **One-shot preset shape for v2.** Default recommendation: use a product-owned broad preset (`doc_researcher` or `general_researcher`) instead of arbitrary custom-agent creation.
- **Promotion permission for v2.5.** Default recommendation: only users who can create agents may save a one-shot run as reusable.
- **Cost / rate-limit guardrails.** "Run Code Builder on all 12 stories in this epic" can fan out badly. The plan-confirm step should show an estimated run count and cost band; per-user invocation budgets need a decision before launch.
- **Per-page agent buttons vs. bar suggested prompts.** Long-term, contextual suggested prompts in the bar may replace the button soup. v1 keeps both. Decide measurement criteria before sunsetting buttons.
- **Cancellation.** When a user cancels a multi-step plan mid-flight, do in-flight runs continue or get signaled to abort? `AgentRunWorkflow` has signals; need a "cancel" semantic that's distinct from "reject approval."

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
- Cross-agent context handoff (each run is independent)
- `@agent` invocation in comments
- Spawning agents from agents

One-shot dynamic runs are a v2 candidate. Opt-in promotion is v2.5. Automatic saved-agent creation/reuse is future reference only and is not implemented by this plan.
