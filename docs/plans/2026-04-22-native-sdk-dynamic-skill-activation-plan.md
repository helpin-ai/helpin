# Native SDK Dynamic Skill Activation Plan

## Status

In progress.

The core selective native planner path has been implemented. Remaining work is now primarily:

- reducing planner-specific Temporal instruction ownership further
- broadening regression and invariant coverage
- validating provider reliability and non-regression using live selective-path run data
- cleaning up stale plan sections and documenting final rollout state

### Implemented

- native selective-path enablement predicate and execution-state threading
- execution-local turn-local instruction transport for native runs
- continuation-safe native turn-local instruction injection
- first-turn and resumed-turn alignment on the same selective native path
- active skill selection from resolved runtime refs
- conservative default activation for workspace/custom skills without applicability metadata
- active per-turn policy derived from the same active skill subset
- Temporal/runtime enforcement aligned to active per-turn policy
- suppression of legacy full `ResolvedSkillInstructions` for selective native planner runs
- suppression/replacement of legacy planner `initialInstructions` for selective native planner runs
- dedicated native phase-guidance assembly
- transition-aware guidance/skill switching coverage for key planner state changes
- normalized native repair-state capture and persistence
- repair-instruction reinjection for completion-policy retries and planner tool failures
- native debug/observability artifacts for phase, skills, policy, continuation mode, and repair state
- rollout-report tooling for comparing native planner run outcomes, selective-path coverage, continuation modes, repair frequency, and applied actions across providers/presets
- substantial Phase 5 extraction of planner rule/context assembly into smaller Temporal helpers
- native planner phase-guidance composition extracted into a dedicated Temporal file
- planner context assembly extracted into a dedicated Temporal file
- approved-preview selection/recovery/application commands extracted into a dedicated Temporal file
- completion-interaction policy enforcement and retry/repair handling extracted into a dedicated Temporal file
- native repair classification and replay selection extracted into a dedicated Temporal file
- native selective-path gating, active-skill selection, and active-policy derivation extracted into a dedicated Temporal file
- native observability artifact persistence extracted into a dedicated Temporal file
- legacy monolithic planner builders are explicitly scoped as fallback-only and covered by tests proving selective native planners bypass them
- explicit regression coverage for:
  - PRD request-changes keeping PRD-focused active skills
  - task-extension guidance after approved plans have already created tasks
  - task-planner and epic-planner spec-context helper behavior
  - review/support preset bundles excluding planner skills
- preservation of existing Codex/OpenCode staged-skill behavior

### Partially implemented

- base-prompt simplification is complete for selective native planner runs, but broader prompt cleanup is still incomplete
- planner guidance extraction is well underway, with native phase-guidance, planner context assembly, approved-preview application, completion-policy enforcement/retry handling, native repair selection, native selective activation, observability persistence, and legacy fallback scoping split out; Temporal still owns backend orchestration paths
- transition coverage is improved, but not yet exhaustive across every apply/resume edge
- regression coverage is materially broader now, but final Phase 5 cleanup can still add more invariant-protection coverage
- live selective-path validation has enough small-sample evidence to close Phase 4
- sampled eligible runs emitted `native_turn_debug` artifacts
- Atlas/OpenRouter epic-planner validation completed successfully on a small sample
- Scribe/OpenAI task-planner validation completed successfully on the post-fix sample
- earlier sampled Scribe failures were traced to approval preview-panel binding edge cases and fixed

### Not yet complete

- final Phase 5-style reduction of Temporal into thin planner orchestration wrappers
- final cleanup of remaining stale phase text and acceptance tracking in this document

### Latest Live Validation Snapshot

Snapshot time: 2026-04-23 09:24 UTC.

Command:

```bash
cd server && go run ./cmd/native-planner-rollout-report --since 2026-04-23T08:40:00Z
```

Observed selective-path data:

- Atlas / `openrouter` / `moonshotai/kimi-k2.6` / `epic_planner`: 3 eligible runs, 0 missing `native_turn_debug`, 2 completed, 1 cancelled, 0 failed; applied actions were `persist_prd=2` and `create_tasks=2`.
- Scribe / `anthropic` / `task_planner`: 6 eligible runs, 0 missing `native_turn_debug`, 2 completed, 1 cancelled, 3 failed; the 2 post-fix completions each produced `approved_preview=1` and `approved_preview_applied=1` with `persist_task_doc`.
- The 3 Scribe failures occurred before the approval panel-key alias fix and had no `approved_preview` artifacts; they matched the fixed bug where `preview_panel_key="publish_task_plan_doc"` did not bind to the canonical `task_plan_doc` preview.

Interpretation:

- selective-path observability is working for sampled eligible runs
- Atlas/OpenRouter is validated on a small sample
- Scribe/Anthropic is validated post-fix on a small sample
- Scribe/OpenAI is validated post-fix on a small sample
- broader provider reliability conclusions should continue to be monitored after rollout, but Phase 4 has enough evidence to proceed

Additional OpenAI Scribe validation:

Command:

```bash
cd server && go run ./cmd/native-planner-rollout-report --since 2026-04-23T09:20:00Z --provider openai --preset task_planner
```

Observed data:

- `openai` / `task_planner`: 5 eligible runs, 0 missing `native_turn_debug`, 4 completed, 1 failed, 0 cancelled; applied actions were `persist_task_doc=4`.
- The single failed run was the pre-fix approval-binding failure where an unknown UUID-style `preview_panel_key` did not bind to the unique same-turn `task_plan_doc` preview.
- The latest successful OpenAI Scribe run completed with `approved_preview=1`, `approved_preview_applied=1`, and no repeated final prose loop.

## Direction

Use the Codex skill model as the architectural reference, but adapt it for `native_sdk`.

The key idea to copy is:

- keep the durable base prompt small
- keep skills modular
- activate only the relevant skills for the current phase, tool, and task
- re-inject contracts when needed instead of depending on turn-0 prompt memory

For `native_sdk`, do this through dynamic prompt assembly. Do not depend on runtime filesystem skill loading.

## Implementation Constraints And Outcomes

These constraints drove the implementation. They are no longer open design questions for the selective native planner path.

### 1. Native planner instructions must not depend on full skill flattening

Before this work, `agent.ResolvedSkillInstructions` compiled all resolved skills into the native system prompt. Selective native planner runs now suppress that full blob and use turn-local active phase guidance plus active skills instead.

### 2. Native turn-local instructions must survive continuation

OpenAI/OpenRouter continuation can omit the system prompt when using `previous_response_id`. The implementation uses execution-local turn instructions so phase guidance, active skill contracts, and repair guidance are available on both first and resumed turns without persisting synthetic user messages.

### 3. Planner routing must reuse durable state, not create a second state engine

Epic/task planner routing continues to derive from existing durable state such as approved spec presence, existing tasks, task plan documents, approved-preview application markers, and latest interactions. That routing now feeds native phase guidance and active skill selection.

### 4. Repair guidance must be normalized enough to reinject safely

Planner tool failures and completion-policy failures now produce normalized native repair-state artifacts or execution-local repair instructions. The next native turn receives targeted repair guidance rather than relying on turn-0 prompt memory.

### 5. Backend-owned domain application remains authoritative

Approved PRD, task-plan, and task-doc previews are still applied by backend commands and repositories. Skills guide the model, but canonical document writes, spec approval, task creation, idempotence, and replay protection remain backend-enforced.

### 6. Active skills must drive both prompt and policy

Selective activation now starts from resolved runtime refs, selects the active subset for the turn, and aggregates the active policy from that same subset. Completion gating, approval-preview validation, runtime interaction behavior, and review parsing use the active turn policy.

### 7. Workspace/custom skills are preserved conservatively

Because skill definitions do not yet expose applicability metadata, unknown workspace/custom skills remain phase-agnostic and active whenever their resolved ref is present. This prevents silent policy or instruction loss until explicit applicability metadata exists.

## Target Architecture

Split instruction delivery for `native_sdk` into four layers:

### 1. Base system prompt

Durable, always-on:

- agent identity
- repo and tool discipline
- execution safety
- general interaction rules
- completion discipline

This layer should stay small.

### 2. Run context

Durable run-specific context:

- task, epic, support, or CRM target context
- linked docs
- transcript summary
- artifact context
- durable planning facts
- repository context

### 3. Active phase guidance

Current-stage guidance only:

- epic planning
- PRD drafting
- task planning
- task planning document drafting
- review
- support reply drafting
- CRM decision support

### 4. Active skill contracts

Only the contracts needed for the current turn:

- phase-relevant skill instructions
- tool-shape reminders
- approval binding rules
- repair instructions after validation failures

This layer should be recomputed every execution turn.

## Generic Agent Boundary

The long-term direction is:

- agents become more generic
- runtime orchestration becomes more generic
- planner behavior moves into active skills plus durable fact injection

But "generic" does not mean "all behavior lives only in prompt text."

The correct boundary is:

- skills and prompt assembly own model behavior
- backend tools and internal commands own real mutations and domain invariants
- Temporal orchestrates generic run lifecycle plus state selection, not a second planner brain

### What should become generic

These parts should move toward generic agent/runtime behavior:

- active phase selection for the current turn
- active skill selection for the current turn
- phase-specific contract reinjection
- repair-instruction reinjection after validation failures
- generic pause/resume handling for human input, approval, and review
- generic preview publishing
- generic interaction artifact persistence
- transcript summarization and replay
- durable run fact injection
- per-turn active skill-policy selection derived from resolved runtime refs

### What must remain backend-owned

These parts should remain backend-enforced because they are product invariants, not model guidance:

- authorization and allowed-tool enforcement
- canonical epic spec document creation and linking
- canonical task planning document creation and linking
- approval of the epic spec
- idempotent application of approved PRD previews
- idempotent application of approved task-plan previews
- task creation from approved task plans
- idempotent application of approved task-doc previews
- race-safe handling of approval and request-changes responses
- prevention of replaying the same approved preview twice

### Why this distinction matters

If we move all planner behavior into skills only, we risk:

- duplicate task creation
- stale or replayed approved artifacts
- spec approval without backend validation
- broken resume semantics across retries and interactive turns
- prompt-only enforcement of rules that must remain authoritative server-side

The target is not "Temporal knows nothing." The target is:

- one generic run engine
- generic interaction and preview primitives
- active skills drive the model
- backend mutations stay authoritative
- Temporal supplies durable state and invokes backend application steps

## Expected End State

After this migration, the intended runtime split is:

### `native_sdk`

- most model-facing flow guidance is defined by:
  - durable base prompt
  - active phase guidance
  - active skills
  - repair instructions
- skills are activated only when relevant for the current turn
- active per-turn selection happens in memory from resolved runtime refs
- active per-turn policy is aggregated from the same active skill subset
- no filesystem skill staging is required

### `codex` and `opencode`

- keep the current staged-skill behavior unchanged
- continue using runtime-visible staged skill packages
- do not adopt the native selective-injection path as part of this rollout
- may later share a higher-level activation abstraction, but not in this plan

### Shared backend behavior across runtimes

The following remain shared backend responsibilities and are not moved into prompt-only behavior:

- approval and review interaction persistence
- approved-preview application
- canonical document persistence and linking
- spec approval
- task creation and related mutations
- authorization and allowed-tool enforcement
- idempotence and replay protection

So the practical end state is:

- flow guidance becomes prompt-and-skill driven
- backend state transitions remain service- and command-enforced
- `native_sdk` becomes modular through selective in-memory activation
- `codex` and `opencode` remain stable on their current staged-skill path

## Always-On vs Dynamic

### Always-on

- general agent behavior
- repo and tool rules
- interaction etiquette
- completion discipline
- safety constraints

### Dynamic

- `prd_authorship`
- `task_decomposition`
- `approval_protocol`
- `epic_state_routing`
- `task_planner_context`
- `review_agent`
- support reply workflow
- CRM-specific contracts

## Design Principles

1. Do not use full `ResolvedSkillInstructions` flattening as the primary mechanism for `native_sdk` planner runs.
2. Do not depend on the initial system prompt carrying all planner semantics for the whole run.
3. Recompute active skills every execution turn from current run state.
4. Re-anchor the model after:
   - approval resolution
   - task or phase transitions
   - tool validation failures
   - request-changes feedback
5. Keep Codex/OpenCode staged skill behavior unchanged.
6. Reuse existing planner-state derivation rather than duplicating it in a new phase machine.
7. Do not move backend-owned approval application and mutation invariants into prompt text.
8. Shrink planner-specific Temporal prompt ownership without removing backend enforcement.
9. Active per-turn selection must drive both prompt content and runtime policy.
10. Replace the current first-turn planner instruction path so first-turn and resumed-turn behavior stay aligned.
11. Do not persist native activation supplements as normal run messages.
12. Start activation from resolved runtime refs and preserve workspace-added skills and policy contracts.
13. Use an explicit conservative default for workspace/custom skills that lack applicability metadata.

## Implemented Native Concept

The native selective planner path now uses the equivalent of an `ActiveInstructionSet` for each execution turn:

- `base`
- `phase_guidance`
- `active_skills`
- `repair_instructions`
- `active_policy`

Inputs include:

- target type
- preset key
- current planning stage
- durable planning facts
- latest pending or approved interaction
- latest normalized planner repair state

Outputs are:

- a compact instruction fragment for the current turn
- the active per-turn skill definitions and aggregated policy subset for that turn

For custom/workspace skills without applicability metadata, the output must preserve them predictably under a documented default rule.

## Native Injection Strategy

Recommendation:

- keep the core system prompt stable and small
- inject active phase guidance and active skill contracts through an execution-local turn supplement
- inject repair instructions in that same execution-local turn supplement
- do not persist that supplement as a normal run message

Do not make correctness depend on updating the system prompt only, because provider continuation may omit it.
Do not inject the supplement by creating a synthetic durable `user` message in run history.

### Execution-local transport for native turn-local instructions

Implemented with dedicated execution-local instruction fields on the native execution context.

Behavior:

- turn-local instructions are passed separately from the durable system prompt and durable run-message history
- they are included in the current execution request only
- they are not persisted as run messages
- continuation mode still receives them even when the provider omits the system prompt

Requirements preserved by this transport:

- it must survive provider-continuation mode where the system prompt is omitted
- it must not be written into durable transcript history
- it must not be summarized as if it were human context
- it must be available on both first-turn and resumed-turn native planner executions

## Concrete Plan Status

The core rollout plan below is implemented for selective native planner runs unless a subsection explicitly says it remains future work.

### 1. Native-only instruction assembly

Implemented through native phase-guidance, active skill selection, repair instruction, and execution-local transport helpers rather than one exported builder API. The effective pieces are:

- `BuildNativeBasePrompt(...)`
- `BuildNativePhaseGuidance(...)`
- `BuildNativeActiveSkillInstructions(...)`
- `BuildNativeRepairInstructions(...)`
- `BuildNativeTurnInstructions(...)`

Implemented across:

- `server/internal/worker/prompt.go`
- planner instruction builders in `server/internal/temporalapp/activities.go`

This assembly layer has absorbed the selective native planner path. Legacy monolithic planner builders are now fallback-only and covered by tests proving selective native planners bypass them.
It must feed both:

- the turn-local instruction text sent to the model
- the active per-turn `SkillPolicy` used by runtime enforcement

### 2. `ResolvedSkillInstructions` compatibility fallback

The compiled skill blob remains available for compatibility and non-selective paths.

Instead:

- keep it available for compatibility and non-native paths
- stop treating it as the primary planner mechanism for `native_sdk`
- gate the new selective behavior to native planner-style runs first
- do not let prompt activation drift from policy activation during the transition

### Legacy injection suppression rule

When the native selective path is enabled for a planner run:

- suppress legacy full `ResolvedSkillInstructions` injection for that run
- suppress legacy planner `initialInstructions` injection for that run
- use only:
  - lean native base prompt
  - execution-local turn instructions from active skills and phase guidance
  - active per-turn policy derived from the same selected subset

When the selective path is not enabled:

- preserve current legacy behavior unchanged

This rule is required to avoid stacking old and new planner instructions in the same native run.

### 3. Start from resolved runtime refs, then select the active subset

Native activation must begin from the runtime's resolved ref set, not from a separate planner-only map.

Flow:

1. resolve effective runtime refs
2. resolve definitions for those refs
3. derive the active subset for this turn from current state
4. aggregate policy from the active subset
5. use that active subset for both prompt assembly and runtime enforcement

This preserves:

- workspace-attached planner skills
- `general_agent_behavior`
- required tools
- interaction contracts
- future workspace skill extensions

### Default rule for skills without applicability metadata

Until the skill schema gains explicit phase/applicability metadata, use this conservative rule:

- built-in skills with known planner/review/support semantics may be selectively activated by phase
- workspace/custom skills without explicit applicability metadata are treated as phase-agnostic and remain active on every turn where their resolved ref is present
- such skills still participate in active policy aggregation on every turn

Why this default:

- it preserves existing behavior and avoids silently dropping workspace-added instructions
- it preserves required tools and interaction contracts attached to those skills
- it avoids unsafe under-enforcement caused by deactivating unknown skills heuristically

Tradeoff:

- unknown/custom skills reduce selectivity until applicability metadata is added in a future phase

Future extension:

- add optional applicability metadata to the skill schema so workspace/custom skills can opt into true selective activation later

### 4. Build phase-based skill selection from current state

Drive active skill selection from current run state, not just preset.

Examples:

- Epic planner, no approved PRD:
  - `epic_state_routing`
  - `prd_authorship`
  - `approval_protocol`
- Epic planner, approved PRD, tasks not created:
  - `epic_state_routing`
  - `task_decomposition`
  - `approval_protocol`
- Task planning doc run:
  - `task_planner_context`
  - `approval_protocol`
- Review run:
  - `review_agent`
- Support draft run:
  - support workflow contract
  - approval contract when required

This selection must be derived from durable state and current run context such as:

- target type
- planning stage
- approved spec presence
- existing task count
- canonical planning-doc presence
- latest pending or resolved interaction
- latest approved preview application state

The selector algorithm should be:

1. resolve the full runtime ref set
2. resolve definitions for that ref set
3. split skills into:
   - known phase-selectable built-ins
   - phase-agnostic/default-active skills
4. choose the active built-in subset for the current turn
5. union that subset with the phase-agnostic/default-active skills
6. aggregate the active policy from that final active set

### 5. Replace the current first-turn planner instruction path

The new native instruction assembly must replace the current first-turn `initialInstructions` path for planner-style runs.

Specifically:

- first-turn planner runs and resumed planner turns must both use the same active-skill selection logic
- do not leave turn 1 on the old monolithic Temporal instruction builders while resumed turns use the new path
- for native planner runs on the selective path, disable the legacy `initialInstructions` prompt contribution entirely

The migration can temporarily preserve the old builders as implementation sources, but not as a separate runtime path.

### 6. Extract existing hardcoded planner guidance into reusable selectors

Refactor existing planner builders so the current hardcoded rules become reusable phase-guidance and skill-selection functions.

Do not maintain two independent sources of planner truth.

Specifically:

- keep existing state derivation as the source of truth
- extract phase and next-step rules from the current epic-planner and task-planner builders
- re-express those rules as:
  - durable planning facts
  - active phase guidance
  - active skill selection

The goal is to remove large prompt blobs from Temporal over time, not to delete the planner state model.

### 7. Normalized repair-state capture

Status: implemented for planner tool failures and completion-policy retries.

When a planner tool fails validation, the runtime captures a normalized repair object for the next turn.

Current fields:

- `tool_name`
- `repair_class`
- `repair_hint`
- `source`
- `error_summary`

Stored as a lightweight run artifact.

### 8. Failure-aware repair reinjection

Status: implemented for known planner preview/tool validation failures and completion-policy failures.

On the next turn after a planner tool failure, inject only the relevant repair contract.

Examples:

- failed `publish_task_plan`:
  - inject task-plan contract only
  - include canonical object shape
  - include exact failure summary
- failed `publish_prd_draft`:
  - inject PRD publishing contract only
- failed `request_approval`:
  - inject approval binding rule only

### 9. Transition-aware reinjection

Status: implemented for the core epic/task planner transitions; additional edge-case coverage remains useful.

When run state changes, switch the active guidance on the next turn.

Important transitions:

- PRD approved -> switch from PRD authorship to task decomposition
- PRD request-changes -> stay in PRD-focused guidance
- task-plan approval -> switch toward apply/create-tasks guidance
- task-plan-doc approval -> switch toward persist-doc guidance

This should work from persisted state and approved-preview application markers, not from prompt memory.

### 10. Lean base prompt

Status: implemented for selective native planner runs; broader non-planner prompt cleanup is outside this rollout.

The selective native planner path keeps planner-heavy details out of the base prompt and injects them through dynamic phase guidance, active skills, and repair instructions.

Move planner-heavy details out of:

- global managed prompt text

and into:

- dynamic phase guidance
- active skill instructions
- repair instructions

### 11. Use execution-local transport for native turn supplements

Do not implement native active-skill injection by writing synthetic `user` messages into durable run history.

Instead, feed the supplement through execution-local prompt assembly only.

Requirements:

- it must influence the current model turn
- it must not become part of transcript summarization
- it must not be replayed later as if it were human input
- it must not distort "continue from the latest human reply" semantics

### 12. Leave Codex/OpenCode staging alone

Codex/OpenCode already have runtime staging behavior through the existing skill staging path.

Do not regress that.

Native gains:

- dynamic skill selection
- dynamic turn-local reinjection

### 13. Preserve backend-owned domain application paths

Do not move these into prompt-only logic:

- approved PRD preview application
- approved task-plan preview application
- approved task-doc preview application
- canonical doc ensuring and linking
- spec approval
- task creation and dependency application

Those paths should remain backend-owned and idempotent. The refactor should only change how the model is instructed before those backend actions are requested or applied.

### 14. Add observability

For each native planner turn, log or persist:

- active phase
- active skills
- active skill refs
- active aggregated policy
- repair instructions injected
- latest repair error class
- whether provider continuation was active

This should be lightweight debug metadata for diagnosis.

Observability is required from the first rollout phase, not as a later polish item. This migration changes both prompt assembly and policy enforcement, so day-one diagnosis depends on being able to inspect:

- whether the selective native path was enabled
- the resolved runtime ref set
- the active skill subset
- the active aggregated policy
- whether legacy planner injection was suppressed
- whether provider continuation was active
- which repair instructions were injected

### 15. Make active policy authoritative for Temporal-side enforcement

Per-turn active policy must not stop at executor input.

The active policy selected for the current turn must become the authoritative policy used by all turn-scoped enforcement, including:

- completion gating
- approval-preview validation
- runtime interaction transport decisions
- structured review parsing

Implementation requirement:

- replace current Temporal-side consumers of the full aggregated `state.skillPolicy` with the active per-turn policy for that execution turn
- this can be done either by:
  - replacing `state.skillPolicy` with the active turn policy before execution and validation
  - or by introducing a distinct `state.activeSkillPolicy` and migrating all relevant consumers to it

Do not leave Temporal validators on the old full policy while the executor uses a narrower active policy.

### 16. Add tests

Prompt assembly:

- native epic planner before PRD approval activates PRD skills, not task decomposition
- native epic planner after PRD approval activates task decomposition, not PRD authorship
- task planner doc activates `task_planner_context`
- review run does not pull in planner skills

Policy alignment:

- active per-turn policy is aggregated from the same active skill subset used for prompt assembly
- completion gating uses active policy, not the full resolved planner policy when selective activation is enabled
- approval-preview validation uses active policy, not the inactive-skill superset
- runtime review parsing uses the active review contract only when the review skill is active
- workspace/custom skills without applicability metadata remain active by default and remain present in active policy aggregation
- known built-in phase-selectable skills can deactivate while phase-agnostic/default-active skills remain present

Failure recovery:

- failed `publish_task_plan` injects task-plan repair instructions next turn
- failed `request_approval` injects approval repair instructions next turn

Transition handling:

- PRD approval changes active skill set next turn
- PRD request-changes keeps PRD-focused active skill set
- task-plan approval changes active guidance appropriately

Continuation safety:

- native OpenAI/OpenRouter continuation still receives active turn instructions even when the system prompt is omitted
- execution-local supplements are not persisted as durable run messages
- transcript summaries do not treat native activation supplements as user input

First-turn parity:

- first-turn planner runs and resumed planner turns use the same active-skill selection path
- planner turn 1 no longer depends on a separate monolithic `initialInstructions` path once the new flow is enabled

Regression:

- native base prompt no longer contains the full planner skill blob once the selective path is enabled
- important runtime rules remain present
- Codex/OpenCode staged-skill behavior is unchanged

Invariant protection:

- approved PRD application still writes canonical doc content and updates approved spec state
- approved task-plan application still creates tasks exactly once
- approved task-doc application still persists and links the canonical planning doc
- previously applied approved previews are not replayed on resume
- approval and request-changes resume behavior remains unchanged

## Rollout Plan

### Phase 1

- define one explicit enablement predicate for the selective native planner path
- resolve that predicate once at run start
- thread that resolved flag through run state, prompt assembly, executor input, and Temporal-side validators
- add native-only active skill selection
- keep existing `ResolvedSkillInstructions` as fallback
- planner-only rollout first
- preserve all existing approved-preview application and mutation paths unchanged
- derive active prompt text and active policy from the same selected skill subset
- replace the first-turn planner instruction path for native planner runs
- add a concrete execution-local transport for turn-local instructions
- when selective path is enabled, suppress legacy full-skill injection and legacy planner `initialInstructions`
- add observability from day one

Internal execution split for Phase 1:

#### Phase 1a: dark-launch plumbing

- build the new native instruction assembly layer
- build the active selector and active policy aggregation
- add executor transport for execution-local turn instructions
- compute and thread the enablement predicate
- add observability
- keep the legacy planner behavior as the serving path while verifying parity

#### Phase 1b: planner cutover

- flip eligible native planner runs onto the selective path
- apply legacy suppression for those runs
- keep non-eligible runs fully on legacy behavior

### Selective path enablement predicate

Do not use a fuzzy description like "planner-style runs."

Use one explicit predicate, computed once at run start, based on a concrete combination such as:

- `runtime_kind == "native_sdk"`
- preset is an explicitly supported planner preset
- target type matches the supported planner target
- rollout flag/config is enabled

Requirements:

- compute once
- persist or thread through execution state
- do not re-derive independently in multiple layers
- use the same predicate for:
  - turn-local native assembly
  - legacy-injection suppression
  - active policy threading
  - observability

### Phase 2

- add normalized repair-state capture
- add repair-instruction reinjection for failed planner tool calls
- keep supplements execution-local only; do not persist them as transcript messages

### Phase 3

- reduce or remove remaining full-skill injection from the native planner base prompt
- keep Codex/OpenCode unchanged

### Phase 4

- use the native planner rollout-report tool to summarize native planner runs by provider/model/preset from persisted run and `native_turn_debug` artifacts
- compare Anthropic vs OpenAI/OpenRouter planner reliability on long interactive runs
- confirm Anthropic planner behavior does not regress under the selective native path
- ensure `AGENT_NATIVE_SELECTIVE_PLANNER_ENABLED=true` is set in both API and Temporal worker environments before collecting validation runs
- treat groups with `selective_validation_status=not_validated_no_native_debug` as baseline native planner runs, not selective-path validation

Rollout-report command:

- `cd server && go run ./cmd/native-planner-rollout-report`
- optional filters:
  - `--workspace <workspace-id>`
  - `--provider anthropic|openai|openrouter`
  - `--preset epic_planner|task_planner`
  - `--since <RFC3339>`
  - `--until <RFC3339>`
  - `--json`

Status: complete for this rollout on small-sample live data. Continue monitoring provider reliability as normal rollout work.

This phase is complete after the report has been run against live rollout data where eligible runs emit `native_turn_debug` artifacts and the results have been reviewed.

### Phase 5

- reduce planner-specific Temporal instruction builders to thin orchestration wrappers
- extract native phase-guidance composition out of the main Temporal activities file
- extract planner context assembly out of the main Temporal activities file
- extract approved-preview selection/recovery/application commands out of the main Temporal activities file
- extract completion-interaction policy enforcement and retry/repair handling out of the main Temporal activities file
- extract native repair classification/replay selection out of the main Temporal activities file
- extract native selective-path gating, active-skill selection, and active-policy derivation out of the main Temporal activities file
- extract native observability artifact persistence out of the main Temporal activities file
- scope legacy monolithic planner builders as fallback-only and keep them unreachable for selective native planner runs
- keep Temporal responsible for:
  - loading durable state
  - selecting active skills and phase guidance
  - running the generic runtime
  - applying approved artifacts through backend commands
- avoid leaving a second planner brain inside Temporal

## File-Level Areas To Change

Likely backend touchpoints:

- `server/internal/worker/prompt.go`
- `server/internal/worker/eino_executor.go`
- `server/internal/worker/eino_exec.go`
- `server/internal/temporalapp/activities.go`
- `server/internal/agentskills/resolve.go`
- `server/internal/worker/interaction_contracts.go`
- `server/internal/worker/transcript_summarizer.go`
- `server/internal/worker/tools_preview.go`
- `server/internal/worker/tools_interaction.go`
- `server/internal/worker/tools_internal_command.go`
- `server/internal/service/agent_planning.go`
- a new native instruction builder file under `server/internal/worker/` or `server/internal/temporalapp/`

Relevant built-in skills:

- `epic_state_routing`
- `prd_authorship`
- `task_decomposition`
- `approval_protocol`
- `task_planner_context`
- `review_agent`

## What Should Stay Simple For Now

Do not try to recreate Codex-style filesystem skill loading for `native_sdk`.

First make native:

- modular
- selective
- repair-aware
- transition-aware

Do not:

- move canonical-doc persistence into prompt-only behavior
- move approved-preview application into skill text
- replace backend idempotence with model discipline
- introduce a second planner-state engine beside the existing durable state model
- implement native supplements by persisting synthetic user messages into run history
- start active-skill selection from a hardcoded planner-only list instead of resolved runtime refs

## Acceptance Criteria

- Native planner runs no longer depend on one giant turn-0 skill blob. Status: implemented for selective native planner runs.
- After PRD approval, the next turn is re-anchored on task-decomposition rules. Status: implemented and covered by focused tests.
- After a `publish_task_plan` validation error, the next turn receives only the relevant task-plan repair contract. Status: implemented and covered by focused tests.
- OpenAI/OpenRouter planner reliability improves on long interactive runs. Status: validated enough for rollout continuation on small-sample Atlas/OpenRouter and Scribe/OpenAI runs; keep monitoring after rollout.
- Anthropic planner behavior does not regress. Status: validated enough for rollout continuation on sampled Scribe/Anthropic runs; keep monitoring after rollout.
- Codex/OpenCode staged skill behavior remains unchanged. Status: implementation keeps the native selective path gated away from Codex/OpenCode; continue relying on regression coverage during final cleanup.

## Recommendation

Implement this in this order:

1. native-only active skill selection
2. planner-only rollout
3. normalized repair-state capture and repair reinjection
4. reduce full-skill injection from the native planner base prompt
5. collapse Temporal planner builders into thin generic orchestration over active skills and backend application paths

That is the cleanest path from the current full-flattening model toward a Codex-like modular runtime without copying Codex filesystem mechanics.
