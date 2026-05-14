# Agents and Automation

This is the current backend reference for:

- automation flows and rules
- trigger executions
- agent records
- agent runs
- built-in automations

Use this doc for runtime and model truth. For the product-facing mental model, see [AUTOMATION_PRODUCT_MODEL.md](/root/teampulse/docs/AUTOMATION_PRODUCT_MODEL.md).

For repository-backed coding-agent execution, see [CODING_AGENT_RUNTIME_FLOW.md](/root/teampulse/docs/CODING_AGENT_RUNTIME_FLOW.md).

## Core model

The platform now treats Automation as one system with four related concepts:

- `flow`
  Product concept. A user-facing automation statement such as:
  `When GitHub PR merged to main, run Review Agent on Repository.`
- `automation_rule`
  Backend persistence model for most user-authored flows.
- `agent_trigger_execution`
  Durable record of what happened at the automation layer:
  event received, rule matched, skipped, failed to launch, or launched a run.
- `agent_run`
  Durable execution record for the agent itself.

Users see:

- `Flows`
- `Activity`
- `Agents`
- `Library`

The backend still stores:

- `automation_rules`
- `agent_trigger_executions`
- `agent_runs`

## Product surfaces

Current primary workspace routes:

- `/w/:slug/automation/flows`
- `/w/:slug/automation/activity`
- `/w/:slug/automation/agents`
- `/w/:slug/automation/library`
- `/w/:slug/automation/tools`
- `/w/:slug/automation/runs`

Current primary API namespace:

- `/api/automation/flows`
- `/api/automation/activity`
- `/api/automation/library/triggers`
- `/api/automation/library/tools`
- `/api/automation/agents`
- `/api/automation/runs`

Legacy PM and settings routes may still exist as redirects or compatibility aliases, but they are no longer the canonical product surfaces.

## Causal chain

The system should be understood as one causal chain:

```text
event or manual start
  -> trigger evaluation
  -> flow / rule match
  -> trigger execution record
  -> start agent run
  -> agent run execution
  -> messages, interactions, artifacts, outcome
```

Not every trigger execution creates an agent run:

- some events match no flow
- some match but are skipped
- some fail before launch
- some create an `agent_run`

That distinction is why `Activity` and `Runs` are separate surfaces.

## Runtime lifecycle

```text
manual launch / webhook / schedule / built-in event
                     |
                     v
        +---------------------------+
        | trigger normalization     |
        | canonical binding id      |
        +-------------+-------------+
                      |
                      v
        +---------------------------+
        | flow / rule evaluation    |
        | match? skip? fail?        |
        +-------------+-------------+
                      |
                      v
        +---------------------------+
        | agent_trigger_execution   |
        | automation-layer record   |
        +-------------+-------------+
                      |
             if launch succeeds
                      |
                      v
        +---------------------------+
        | startTargetRun            |
        | target resolution         |
        +-------------+-------------+
                      |
                      v
        +---------------------------+
        | agent_run                 |
        | durable runtime record    |
        +-------------+-------------+
                      |
                      v
        +---------------------------+
        | worker / runtime          |
        | messages / artifacts      |
        | approval / completion     |
        +---------------------------+
```

This is the main separation to keep in mind:

- `agent_trigger_execution` records the automation-layer decision path
- `agent_run` records the actual executor runtime path

## Trigger families

Current trigger families in code:

- manual launches
  - `manual.task_run`
  - `manual.epic_run`
  - `manual.support_run`
- workflow and approval rules
  - `task.state_entered`
  - `agent_run.approved`
- GitHub webhook rules
  - `github.push`
  - `github.pull_request_opened`
  - `github.pull_request_merged`
  - `github.pull_request_review_requested`
  - `github.release_published`
  - `github.check_suite_completed`
- schedules and cron
  - `agent.schedule`
  - `automation_rule.cron`
- built-in or product-owned launchers
  - `support.widget_message`
  - `task.assigned_agent_state_change`

Canonical trigger and binding identity now lives in `server/internal/automationcatalog/triggers.go`.

## Built-in automations vs flows

There are two important kinds of automation behavior:

### Built-in automations

Product-owned backend behavior implemented directly in services, repositories, or Temporal workflows.

Examples:

- CRM buyer signal ingestion
- CRM contact/deal summary refresh
- deterministic PM epic and sprint automations

These belong to the Automation ecosystem, but they are not user-authored flows.

### User-authored flows

Most user-authored flows are persisted as `automation_rules`.

Current active action types:

- `start_agent_run`
- `move_to_state`
- `merge_branch`
- `run_command`

Historical action constants still exist but are not active:

- `run_agent`
- `start_flow`

## Agents

An `agent` is a reusable executor record.

Important fields:

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
- `team_ids`
- `schedule`
- `approval_mode`
- `default_invocation_mode`

The product surfaces are:

- `Agents` for configuration and ownership
- `Runs` for execution details
- `Activity` for automation-layer history involving those agents

## Agent runs

Every real execution becomes an `agent_run`.

Important run fields:

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

- explicit `trigger`, `target`, and `event` objects
- compatibility legacy fields such as `story_id`, `epic_id`, and `conversation_id`
- trigger sources commonly include `manual`, `automation_rule`, and `schedule`

## Trigger executions

`agent_trigger_execution` is now the durable automation-layer event log.

It exists to answer:

- what fired
- what matched
- what was skipped
- what failed before launch
- what run was created

This is the data behind the `Activity` surface.

`agent_run` remains the runtime-layer execution record and is the data behind `Runs`.

## Targets

Current target model supports both direct and rule-driven launches.

Common target types:

- `task`
- `epic`
- `support_conversation`
- `support_coverage_gap`
- `repository`
- `crm_deal`
- `document`

Important current behavior:

- manual launches can start directly against an explicit target
- rule-driven `start_agent_run` can use either an event-derived target or an explicit fixed target
- GitHub-triggered `start_agent_run` should be treated as fixed-target rules in practice unless the event resolves cleanly to an existing linked task
- `support_coverage_gap` is a product-owned support/docs target used by the Documentation system agent to act on support coverage analysis through the normal `agent_run` path

## Launch path comparison

```text
manual UI action
  -> direct startTargetRun
  -> agent_trigger_execution(source=manual)
  -> agent_run

automation flow event
  -> rule evaluation
  -> agent_trigger_execution(source=automation_rule)
  -> startTargetRun
  -> agent_run

agent schedule
  -> scheduled trigger
  -> agent_trigger_execution(source=schedule)
  -> agent_run

built-in automation
  -> product-owned service/workflow logic
  -> may create trigger execution and/or agent_run depending on behavior
```

## Agent categories

The code still has two practical categories:

### System agents

Product-owned, preset-backed agents.

Examples:

- `epic_planner`
- `task_planner`
- `documentation_agent`
- `crm_operator`
- `support_agent`
- `code_builder`
- `review_agent`

Characteristics:

- `is_system = true`
- preset-bound
- backend-owned defaults and guardrails
- may still have some product-specific launch or context-loading behavior

Native system selective behavior:

- `epic_planner` on epic targets, `task_planner` on task targets, and `documentation_agent` on supported documentation targets use selective native skill activation by default when running on `native_sdk`
- planner or documentation phase guidance, active skill contracts, repair instructions, and active skill policy are assembled per execution turn from durable run state
- canonical mutations such as approved PRD persistence, task creation, task-plan-doc persistence, and replay protection remain backend-owned
- Codex/OpenCode staged-skill behavior is unchanged by native system selective activation

### Custom agents

Workspace-created agents.

Current truth:

- `is_system = false`
- not preset-backed
- generic executor model
- not on the system selective activation path

Simplified creation behavior:

- draft generation and the custom-agent creation UI are helpers for producing a normal custom agent record
- created custom agents still use the existing `/automation/agents` persistence path
- execution still creates normal `agent_run` records
- no separate custom-agent runtime, trigger model, or persistence model exists

Direction:

- keep custom execution generic
- prefer minimal trigger payloads
- let agents gather additional context through tools instead of bespoke backend orchestration
- if custom agents later need selective skill activation, add explicit skill applicability metadata instead of reusing system-agent routing rules

### Agent team access

Agents can be limited to one or more teams through `team_ids`.

Current intended semantics:

- team-scoped agents are visible and usable only by actors whose workspace membership includes one of those teams
- workspace-scoped agents have no `team_ids` and are available across the workspace subject to normal permissions
- team access is an agent access boundary, not a new agent category
- direct run, update, and delete paths should enforce the same team boundary as list and create/update UI paths

Target interaction:

- team-scoped agents should only run against targets that resolve to an allowed team
- workspace-level targets such as `support_coverage_gap` should generally be handled by workspace-scoped agents or product-owned system agents unless explicit team semantics are added

## Runtime kinds

Current runtime kinds:

- `native_sdk`
- `opencode`
- `codex`

Current queue mapping:

- `native_sdk + interactive` -> `agent-native-interactive`
- `native_sdk + autonomous` -> `agent-native-autonomous`
- `opencode` -> `agent-opencode-autonomous`
- `codex + interactive` -> `agent-codex-interactive`
- `codex + autonomous` -> `agent-codex-autonomous`
- non-agent background automations -> `automation-default`

## Main architectural rules

- treat `Flow` as the primary product abstraction
- treat `automation_rule` as a persistence detail
- keep `Trigger Execution` and `Agent Run` as distinct records
- built-in automations and user-authored flows should share the same ecosystem, but not be modeled as the same object
- frontend should render backend-owned normalized trigger metadata instead of rebuilding trigger meaning locally

## Current known limitations

- some legacy PM and settings routes still exist as redirects or compatibility aliases
- some backend package and model names still use older PM-era terminology
- some product-owned automations still have domain-specific orchestration paths, especially in support

Those do not change the current product direction:

- `Flows` is the main configuration surface
- `Activity` is the main operational surface
- `Agents` manages executors
- `Library` is reference only
