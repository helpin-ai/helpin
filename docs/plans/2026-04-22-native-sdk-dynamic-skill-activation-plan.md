# Native SDK Dynamic Skill Activation Plan

## Status

Draft plan for improving `native_sdk` planner reliability by moving from full skill flattening toward dynamic per-turn instruction assembly.

## Direction

Use the Codex skill model as the architectural reference, but adapt it for `native_sdk`.

The key idea to copy is:

- keep the durable base prompt small
- keep skills modular
- activate only the relevant skills for the current phase, tool, and task
- re-inject contracts when needed instead of depending on turn-0 prompt memory

For `native_sdk`, do this through dynamic prompt assembly. Do not depend on runtime filesystem skill loading.

## Verified Constraints From Current Code

### 1. Native planner instructions are still flattened into the system prompt today

`agent.ResolvedSkillInstructions` is compiled from all resolved skill definitions and injected wholesale by the native system prompt builder.

Implication:

- phase-specific planner guidance is currently always-on instead of selectively activated

### 2. Native runs do rebuild prompt state on resumed executions

The `native_sdk` executor rebuilds prompt input each execution call, including resumed interactive turns.

Implication:

- dynamic per-turn skill activation is mechanically feasible

### 3. OpenAI/OpenRouter continuation can skip resending the system prompt

When provider continuation is active, the Responses-based native path clears the system prompt and sends only incremental history with `previous_response_id`.

Implication:

- active skill reinjection must not rely only on mutating the system prompt
- the safest place for phase and repair instructions is a fresh turn-local execution supplement or equivalent injected user/context message

Required implementation consequence:

- the native path needs one explicit continuation-safe transport for turn-local instructions
- do not leave this as an abstract "supplement" concept

### 4. Planner phase routing already exists, but it is hardcoded

Epic-planner and task-planner guidance already use current state such as approved spec presence, existing tasks, and planning document state.

Implication:

- do not build a second planner-state engine
- extract and reuse the existing routing logic as the source of truth for active skill selection

### 5. Tool failures are repair-oriented, but not yet normalized as reusable repair state

Planner tools already emit good repair-oriented validation errors. However, persisted `ToolInvocation` records keep only:

- `tool_name`
- `input`
- `output_summary`
- `duration_ms`

They do not persist a first-class error type or repair contract object.

Implication:

- targeted repair reinjection should not depend on brittle scraping alone
- add explicit normalized repair metadata for the latest relevant planner tool failure

### 6. The current planner path is already split across three layers

Today the planner flow is not one monolithic thing. It is already divided into:

- Temporal-side instruction and context assembly
- generic preview and interaction tools in the worker runtime
- backend-owned mutation and approval-application logic

Implication:

- we should not collapse all planner behavior into prompt text
- we should reduce Temporal's planner-specific prompt ownership while preserving backend-owned domain transitions

### 7. Generic runtime primitives already exist

The worker runtime already has generic building blocks for:

- preview publication
- human input requests
- approval requests
- review checkpoints
- interaction persistence
- internal-command execution

Implication:

- the refactor should build on those generic primitives rather than inventing new planner-only ones

### 8. Approved preview application is a product invariant, not just model guidance

Applying an approved preview already performs real state transitions such as:

- writing the approved PRD into the canonical epic spec document
- approving the epic spec
- creating tasks from an approved task plan
- writing the approved task planning document to Docs
- marking approved previews as applied so they are not replayed

Implication:

- those operations must remain backend-owned and idempotent
- they should not move into prompt-only or skill-only enforcement

### 9. Skill activation today affects runtime policy, not just prompt text

The current runtime does not treat skills as prompt-only modules. Aggregated skill policy is already used for:

- completion gating
- approval-preview validation
- runtime-bridge behavior
- structured review parsing

Implication:

- native selective activation cannot change prompt text only
- it must also produce an active per-turn policy subset used by runtime enforcement

### 10. First-turn planner behavior currently comes from Temporal-injected initial instructions

Today, large planner-specific guidance is injected into `initialInstructions`, which becomes part of the first user prompt.

Implication:

- a new native selective-injection path must explicitly replace or subsume that first-turn path
- otherwise turn 1 and resumed turns will use different planner instruction models

### 11. Persisted synthetic user messages would pollute transcript replay and summarization

Current transcript replay and transcript summaries include all non-status messages, including synthetic user messages like policy-retry corrections.

Implication:

- active-turn supplements for native skill activation must not be persisted as normal run messages
- they should be execution-local input assembled at runtime, not durable transcript content

### 12. Activation must start from resolved runtime refs, not a hardcoded built-in planner map

The runtime ref set currently comes from effective runtime refs, which can include:

- preset-provided built-in skills for system agents
- workspace skill refs for custom and future planner configurations

Implication:

- native active selection must begin from the resolved runtime ref set
- phase filtering must select a subset of that resolved set, not replace it with a planner-only hardcoded list

### 13. Workspace-added skills currently have no phase/applicability metadata

Current skill definitions expose:

- instructions
- required tools
- supported runtimes
- interface
- policy

They do not expose phase/applicability metadata for selective activation.

Implication:

- the selector needs an explicit default rule for unknown/custom skills
- otherwise implementation will either silently drop workspace skills or force ad hoc hardcoded behavior

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

## New Native Concept

Introduce an `ActiveInstructionSet` for `native_sdk`:

- `base`
- `phase_guidance`
- `active_skills`
- `repair_instructions`
- `active_policy`

Inputs:

- target type
- preset key
- current planning stage
- durable planning facts
- latest pending or approved interaction
- latest normalized planner repair state

Output:

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

### Explicit transport decision for native turn-local instructions

For this plan, use a dedicated execution-local instruction transport in the native executor.

Implementation direction:

- add an explicit executor input for turn-local instructions
- pass that input separately from the durable system prompt and durable run-message history
- include it in the current execution request only
- do not persist it as a run message

Practical options:

- preferred: add a dedicated executor parameter such as `TurnLocalInstructions` or equivalent on `ExecutionContext`
- acceptable fallback: append one ephemeral synthetic message to the in-memory model input for that execution only, without persisting it through `runMessageRepo`

Requirements for the chosen transport:

- it must survive provider-continuation mode where the system prompt is omitted
- it must not be written into durable transcript history
- it must not be summarized as if it were human context
- it must be available on both first-turn and resumed-turn native planner executions

## Concrete Plan

### 1. Introduce native-only instruction assembly

Add builders along these lines:

- `BuildNativeBasePrompt(...)`
- `BuildNativePhaseGuidance(...)`
- `BuildNativeActiveSkillInstructions(...)`
- `BuildNativeRepairInstructions(...)`
- `BuildNativeTurnInstructions(...)`

This should live near:

- `server/internal/worker/prompt.go`
- planner instruction builders in `server/internal/temporalapp/activities.go`

This new assembly layer should absorb most of the behavior currently expressed in large planner-specific instruction builders, while preserving the same durable facts and product semantics.
It must feed both:

- the turn-local instruction text sent to the model
- the active per-turn `SkillPolicy` used by runtime enforcement

### 2. Keep `ResolvedSkillInstructions` as compatibility fallback

Do not remove the existing compiled skill blob globally in phase 1.

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

### 7. Add normalized repair-state capture

When a planner tool fails validation, capture a normalized repair object for the next turn.

Suggested fields:

- `tool_name`
- `error_class`
- `repair_hint`
- `canonical_contract_key`
- `failed_input_excerpt`

Store this as lightweight run metadata or a small run artifact.

### 8. Add failure-aware repair reinjection

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

### 9. Add transition-aware reinjection

When run state changes, switch the active guidance on the next turn.

Important transitions:

- PRD approved -> switch from PRD authorship to task decomposition
- PRD request-changes -> stay in PRD-focused guidance
- task-plan approval -> switch toward apply/create-tasks guidance
- task-plan-doc approval -> switch toward persist-doc guidance

This should work from persisted state and approved-preview application markers, not from prompt memory.

### 10. Keep the base prompt lean

Refactor the native base prompt so it carries only durable instructions.

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

- add native-only active skill selection
- keep existing `ResolvedSkillInstructions` as fallback
- planner-only rollout first
- preserve all existing approved-preview application and mutation paths unchanged
- derive active prompt text and active policy from the same selected skill subset
- replace the first-turn planner instruction path for native planner runs
- add a concrete execution-local transport for turn-local instructions
- when selective path is enabled, suppress legacy full-skill injection and legacy planner `initialInstructions`

### Phase 2

- add normalized repair-state capture
- add repair-instruction reinjection for failed planner tool calls
- keep supplements execution-local only; do not persist them as transcript messages

### Phase 3

- reduce or remove full-skill injection from the native planner base prompt
- keep Codex/OpenCode unchanged

### Phase 4

- add observability
- compare Anthropic vs OpenAI/OpenRouter planner reliability on long interactive runs

### Phase 5

- reduce planner-specific Temporal instruction builders to thin orchestration wrappers
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

- Native planner runs no longer depend on one giant turn-0 skill blob.
- After PRD approval, the next turn is re-anchored on task-decomposition rules.
- After a `publish_task_plan` validation error, the next turn receives only the relevant task-plan repair contract.
- OpenAI/OpenRouter planner reliability improves on long interactive runs.
- Anthropic planner behavior does not regress.
- Codex/OpenCode staged skill behavior remains unchanged.

## Recommendation

Implement this in this order:

1. native-only active skill selection
2. planner-only rollout
3. normalized repair-state capture and repair reinjection
4. reduce full-skill injection from the native planner base prompt
5. collapse Temporal planner builders into thin generic orchestration over active skills and backend application paths

That is the cleanest path from the current full-flattening model toward a Codex-like modular runtime without copying Codex filesystem mechanics.
