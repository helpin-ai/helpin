# Agents and Automation

This is the single source of truth for the current backend model for:

- agents
- agent runs
- built-in automations
- automation rules
- manual, event, cron, and schedule-based triggering

It also records the proposed direction for custom agents so future work does not drift back into older flow-runtime or agent-class ideas.

## Current truth

The active platform model is intentionally small:

- an `agent` is a configurable executor record
- an `agent_run` is the durable execution primitive
- built-in automations are product-owned backend behavior
- automation rules are user-authored trigger-to-action rules

Agents are not a top-level automation kind. If something actually executes, it becomes an `agent_run`.

## Current flow

```text
                       +----------------------+
                       |   built-in trigger   |
                       |  service/workflow    |
                       +----------+-----------+
                                  |
                                  | product-owned behavior
                                  v
                            +-----------+
                            | service / |
                            | workflow  |
                            +-----------+

  +----------------+        +----------------------+        +------------------+
  | manual UI/API  +------->+  StartTargetRun      +------->+    agent_run     |
  | POST /agent-runs|       |  generic direct path |        | durable record   |
  +----------------+        +----------+-----------+        +--------+---------+
                                       ^                             |
                                       |                             |
  +----------------------+             |                             v
  | story / epic         |-------------+                      +-------------+
  | convenience endpoints|   wrapper behavior                 | Temporal +  |
  | run-agent            |   assignment / planner defaults    | runtime     |
  +----------------------+                                    +------+------+ 
                                                                      |
                                                                      v
                                                              +---------------+
                                                              | messages /    |
                                                              | interactions /|
                                                              | artifacts     |
                                                              +---------------+

  +----------------------+        +----------------------+ 
  | automation rule      +------->+ start_agent_run      |
  | trigger              |        | target-aware action  |
  | state_entered /      |        | use config target or |
  | approved / cron      |        | fall back to event   |
  +----------------------+        +----------+-----------+
                                              |
                                              v
                                        +-----------+
                                        | agent_run |
                                        +-----------+

  +----------------------+        +----------------------+
  | agent schedule       +------->+ scheduled run path   |
  | cron on agent record |        | trigger.source=schedule
  +----------------------+        +----------------------+

Targets:
  story, epic, support_conversation, scheduled, and any other allowed target

Special case:
  support_conversation still has its own launch path and post-run behavior
```

## Top-level taxonomy

### Built-in automations

Built-in automations are product-owned behaviors implemented directly in services, repositories, and Temporal workflows.

Examples:

- CRM signal detection
- CRM summary refresh
- deterministic PM automations

These are not generic agents and are not user-authored automation rules.

### Automation rules

Automation rules are user-authored records in `automation_rules`.

They map a trigger to a controlled action.

Current trigger types:

- `story.state_entered`
- `agent_run.approved`
- `cron`

Current active action types:

- `start_agent_run`
- `move_to_state`
- `merge_branch`
- `run_command`

Historical constants still exist in code, but are not active:

- `run_agent`
- `start_flow`

### Agents

Agents are reusable executors stored in `agents`.

Important agent fields today:

- `is_system`
- `preset_key`
- `runtime_kind`
- `trigger_mode`
- `provider`
- `model`
- `system_prompt`
- `allowed_tools`
- `allowed_commands`
- `allowed_targets`
- `schedule`
- `approval_mode`
- `default_invocation_mode`

Important run fields today:

- `target_type`
- `target_id`
- `runtime_kind`
- `invocation_mode`
- `status`
- `approval_state`
- `pause_reason`
- `execution_stage`
- `task_queue`
- `runner_pool`
- `input`
- `output_summary`

Current run input contract:

- run input now supports explicit `trigger`, `target`, and `event` objects
- legacy top-level fields like `story_id`, `epic_id`, `conversation_id`, and planning fields are still preserved for compatibility
- current populated trigger sources include `manual`, `automation_rule`, and `schedule`
- some product-owned internal launches may still use system-specific trigger values

## Agent categories

The code already has an operational split between product-owned system agents and user-managed agents. That split should be treated as the real model going forward.

### System agents

System agents are product-owned agents seeded or managed by backend code.

Current examples:

- `epic_planner`
- `story_planner`
- `crm_operator`
- `support_agent`
- `code_builder`
- `review_agent`

System-agent characteristics:

- represented by `is_system = true`
- preset-bound
- for `native_sdk`, share the same run, transcript, interaction, and artifact machinery as custom agents
- currently still differ mainly in preset/default ownership plus some target-aware launch and context-loading paths
- genuine special-case backend behavior should be limited to product-specific exceptions such as support conversation handling
- backend owns their lifecycle, defaults, and guardrails

In practice, system agents are product-owned defaults. They should not imply a separate native execution engine.

### Custom agents

Custom agents are workspace-defined agents created by users.

Current code truth:

- represented by `is_system = false`
- cannot set `preset_key` or `preset_version_key`
- may currently use `native_sdk` or `opencode`
- cannot use `codex`, because codex requires a supported preset policy

Proposed product direction:

- custom agents should be `native_sdk` only
- custom agents should use one generic execution model
- custom agents should not depend on bespoke backend orchestration
- custom agents should gather most context through tools after receiving a minimal trigger payload

In practice, custom agents are where generic execution belongs.

## Presets

Presets are bundles of defaults, not separate runtimes or classes.

Current preset catalog:

| Preset | Purpose | Runtime | Default mode | Typical targets |
| --- | --- | --- | --- | --- |
| `epic_planner` | Interactive PRD-to-stories planning loop | `native_sdk` | `interactive` | `epic`, `story`, `crm_deal` |
| `story_planner` | Interactive decomposition and refinement | `native_sdk` | `interactive` | `story`, `epic` |
| `crm_operator` | Cross-app CRM assistance | `native_sdk` | `interactive` | `crm_deal`, `support_conversation`, `document` |
| `support_agent` | Support triage and draft replies | `native_sdk` | `autonomous` | `support_conversation` |
| `code_builder` | Repo-writing implementation agent | `opencode` | `autonomous` | `story` |
| `review_agent` | Validation and review without repo mutation | `opencode` | `autonomous` | `story` |

Important implications:

- presets define defaults for tools, commands, targets, approval mode, runtime, and prompt
- presets do not define a hidden orchestration engine
- custom agents are not preset-backed today
- native planning/scaffolding behavior is now selected by effective tools plus target, not by preset name alone

## Runtime kinds and invocation modes

Current runtime kinds in code:

- `native_sdk`
- `opencode`
- `codex`

Current mode support:

- `native_sdk` supports `interactive` and `autonomous`
- `opencode` supports `autonomous`
- `codex` supports `interactive` and `autonomous`, but only for preset-constrained coding agents

Current queue mapping:

- `native_sdk + interactive` -> `agent-native-interactive`
- `native_sdk + autonomous` -> `agent-native-autonomous`
- `opencode` -> `agent-opencode-autonomous`
- `codex + interactive` -> `agent-codex-interactive`
- `codex + autonomous` -> `agent-codex-autonomous`
- non-agent background automations -> `automation-default`

## Execution model

Every real execution becomes one `agent_run`.

The durable run workflow is:

1. create `agent_run`
2. prepare run state and target context
3. execute on the selected runtime
4. pause when waiting for approval, user input, or authentication
5. resume through Temporal signals
6. complete, fail, or cancel

Run transcript and state live in:

- `agent_runs`
- `agent_run_messages`
- `agent_run_artifacts`
- `agent_run_interactions`

## Current run entrypoints

The backend is still target-aware.

Current first-class launch paths are:

- story-targeted runs
- epic-targeted runs
- support-conversation runs
- scheduled agent runs
- automation-rule-launched runs

Important current limitation:

- generic agent execution exists
- generic target launching exists for:
  - direct runs via `POST /api/pm/agent-runs`
  - automation-rule `start_agent_run`

`StartTargetRun` now provides the generic run contract with explicit `target_type`, `target_id`, and `agent_id`. Story and epic endpoints remain convenience wrappers with product-specific fallback behavior. Automation-rule `start_agent_run` now uses the same target contract. Support conversations still use a separate launch path.

For `native_sdk` runs, the remaining non-generic behavior is mostly:

- target-aware launch paths
- target/context hydration for story, epic, and support conversation runs
- support-specific post-run handling

Interactive planning instructions for native runs are now gated by effective toolset plus target rather than by preset name.

## Triggering model

There are four related trigger concepts in the code today.

### Manual trigger

Manual trigger means a human explicitly starts a run through an API or UI action.

Current manual paths include:

- run agent on story
- run agent on epic
- direct `POST /api/pm/agent-runs`
- resume paused run
- resolve coding-session interaction

This is the cleanest trigger and should remain part of both system-agent and custom-agent UX.

### Agent `trigger_mode`

`trigger_mode` is agent configuration.

Current values:

- `manual`
- `auto_on_assignment`
- `auto_on_event`

Current truth:

- this field exists and is validated
- it is not yet a complete generic trigger engine by itself
- the actual event-driven behavior still comes from explicit service hooks and automation rules

### Agent `schedule`

Agents may also have a cron `schedule`.

Current truth:

- a schedule creates normal `agent_runs`
- scheduling is a trigger mechanism, not a distinct execution kind
- the schedule lives on the agent record
- scheduled runs currently use `target_type = scheduled`

This exists today, but it is not the best long-term model for custom agents if automation rules become the canonical event and cron trigger layer.

### Automation-rule triggers

Automation rules are the main user-authored event layer.

Current rule triggers:

- `story.state_entered`
- `agent_run.approved`
- `cron`

Current rule behavior:

- `start_agent_run` now accepts the same generic target contract as direct runs
- event-driven rules may omit `target_type` and `target_id` to use the triggering target
- cron rules must provide explicit `target_type` and `target_id`
- rules pass `agent_id` through to generic run start and do not mutate story assignment
- support conversations still use a separate launch path

## Proposal for custom agents

The proposed product model should be:

- system agents remain product-owned defaults
- custom agents become generic `native_sdk` executors

### Proposed custom-agent contract

Custom agents should be:

- `is_system = false`
- `runtime_kind = native_sdk`
- generic over trigger source
- generic over target type
- generic over context gathering

Custom agents should support three user-visible trigger families:

- `manual`
- `event`
- `cron`

### Proposed trigger source mapping

For custom agents:

- `manual` -> explicit run-now action
- `event` -> automation rules launching the agent
- `cron` -> automation rules with `trigger_type = cron`

This keeps one triggering model for user-authored automation instead of splitting behavior across bespoke entrypoints and agent-owned schedules.

### Proposed trigger payload

Custom agents should not start blind. They should receive a minimal structured trigger payload in `agent_run.input`, then gather the rest of the context with tools.

Recommended minimum payload:

```json
{
  "trigger": {
    "source": "manual|automation_rule|schedule",
    "trigger_type": "manual|story.state_entered|agent_run.approved|cron",
    "rule_id": "optional",
    "fired_at": "timestamp"
  },
  "target": {
    "target_type": "story|epic|support_conversation|crm_deal|document|workspace",
    "target_id": "uuid-or-logical-id"
  },
  "event": {
    "state_id": "optional",
    "team_id": "optional",
    "run_id": "optional",
    "reason": "optional"
  }
}
```

The point is:

- backend passes the reason the run exists
- agent uses tools to expand context safely
- target-aware context loading can exist, but target-specific orchestration should not be baked into the custom-agent contract

### Proposed execution rules

Custom agents should not have:

- hidden phase machines
- custom finalizers
- target-specific orchestration logic per agent
- runtime-specific exception paths beyond generic native interactive execution

Custom agents should rely on:

- system prompt
- allowed tools
- allowed targets
- generic runtime
- durable run transcript and artifacts
- explicit trigger payload

## Proposal for automation rules

Automation rules should remain the user-authored trigger layer.

Recommended changes:

### 1. Keep automation rules as the event and cron surface

Do not introduce a separate top-level custom-agent trigger engine.

### 2. Keep `start_agent_run` aligned with the generic run contract

`start_agent_run` should use the same target contract as direct runs.

Current contract:

- `agent_id`
- optional `target_type`
- optional `target_id`
- optional `additional_context`

Resolution rules:

- if `target_type` and `target_id` are present, use them
- otherwise, derive them from the triggering event
- cron rules must set them explicitly because cron events do not carry a target

### 3. Use automation rules for cron-driven custom agents

Prefer cron rules over agent-owned schedules for user-authored custom-agent automation.

Current agent `schedule` can remain for compatibility and system use, but it should not be the preferred long-term trigger model for custom agents.

### 4. Keep rule actions controlled

Automation rules should remain a small action surface. They should launch runs and system actions, not become a full flow runtime.

## Built-in automation catalog

Built-in automations remain product-owned backend behavior.

Current examples include:

- `crm.buyer_signal_ingestion`
- `crm.contact_summary_refresh`
- `crm.deal_summary_refresh`
- `pm.epic_auto_start`
- `pm.epic_auto_complete`
- `pm.sprint_auto_create`
- `pm.sprint_move_unfinished`

These do not become custom agents and should not be forced into the generic custom-agent contract.

## Design rules

When adding new work:

- keep `agent_run` as the durable execution primitive
- keep automation rules as the user-authored trigger layer
- keep built-in automations product-owned
- keep special orchestration limited to genuine product exceptions such as support flow
- keep custom agents generic
- keep safety in tools and services, not in prompt wording
- avoid reintroducing flow-runtime or agent-class taxonomy

## What to ignore from older language

Do not build on:

- historical `agent_class`
- historical `capability_profile`
- planning-session runtime language
- generic flow-template runtime language for normal PM or CRM work
- old docs that treat agents, rules, and flows as equal top-level automation kinds

## File map

Useful code entry points:

- agent models: `server/internal/model/agent.go`
- invocation modes: `server/internal/model/agent_runtime.go`
- run interactions: `server/internal/model/agent_run_interaction.go`
- agent presets: `server/internal/service/agent_presets.go`
- agent policy: `server/internal/service/agent_policy.go`
- agent service and lifecycle: `server/internal/service/agent.go`
- coding-session interaction resolution: `server/internal/service/coding_session.go`
- runtime profiles: `server/internal/worker/runtime_profiles.go`
- resolved runtime policy: `server/internal/worker/resolve.go`
- run workflow: `server/internal/temporalapp/workflow.go`
- run activities: `server/internal/temporalapp/activities.go`
- automation model types: `server/internal/model/automation.go`
- automation rule model: `server/internal/model/automation_rule.go`
- automation rule engine: `server/internal/service/automation_rule_engine.go`
- automation inventory: `server/internal/service/automation_inventory.go`

## Bottom line

The current truth is:

- built-in automations are product-owned backend behaviors
- automation rules are user-authored trigger-to-action records
- agents are reusable executors
- agent runs are the durable execution primitive
- the backend behaves as if there are two agent categories:
  - system agents as product-owned preset/default wrappers
  - custom agents as user-defined wrappers
- for `native_sdk`, both categories share the same core run machinery
- remaining differences are mostly preset ownership, target-aware startup/context loading, and the support-agent exception path

The proposed direction is:

- formalize that split
- keep special orchestration only for real product exceptions, not as a general system-agent property
- make custom agents `native_sdk` only
- make custom agents triggerable by manual actions, automation-rule events, and automation-rule cron
- pass a minimal trigger payload into the run
- let custom agents gather most additional context with tools
