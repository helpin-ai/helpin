# Agents, Automation, and Taxonomy

This is the single source of truth for the current agent and automation model in the codebase.

It describes the runtime that exists today, not older PRD language or abandoned branches.

## Why this doc exists

The codebase has three related ideas that used to blur together:

- agents
- built-in automations
- automation rules

They are connected, but they are not the same thing.

The current platform model is intentionally small:

- an `agent` is a configured executor
- an `agent_run` is the durable execution primitive
- built-in automations are product-owned system behavior
- automation rules are user-authored trigger-to-action rules
- the top-level automation taxonomy has two product kinds:
  - `built_in_automation`
  - `automation_rule`

Agents are not a third automation kind. They are reusable executors that humans, schedules, and rules can launch.

## The top-level taxonomy

### 1. Agents

Agents are workspace-scoped configuration records stored in `agents`.

An agent defines:

- which preset it starts from
- which runtime it uses
- which targets it can run against
- which tools and commands it may use
- whether approval is required
- whether it defaults to `interactive` or `autonomous`
- whether it is system-seeded or user-managed

Agents do not define a separate orchestration engine. They are policy plus defaults.

### 2. Agent runs

`agent_runs` are the real execution unit.

Every actual execution becomes one durable run with:

- a target type and target ID
- a runtime kind
- an invocation mode
- status, approval state, and pause reason
- input/output payloads
- transcript messages in `agent_run_messages`
- artifacts in `agent_run_artifacts`

If something "uses an agent", what actually happens is that the system creates and executes an `agent_run`.

### 3. Built-in automations

Built-in automations are system-owned behaviors implemented directly in backend services, repositories, and Temporal workflows.

They are not user-authored agents and they are not configurable as generic flows.

Current cataloged built-ins are:

- `crm.buyer_signal_ingestion`
- `crm.contact_summary_refresh`
- `crm.deal_summary_refresh`
- `pm.epic_auto_start`
- `pm.epic_auto_complete`
- `pm.sprint_auto_create`
- `pm.sprint_move_unfinished`

### 4. Automation rules

Automation rules are user-authored records in `automation_rules`.

They map a trigger to an action, for example:

- when a story enters a state
- when an agent run is approved
- when a cron category fires

Rules are event-driven glue. They do not introduce a separate runtime model.

## What the system is not

The current model should not be understood as:

- agent classes like planner / engineer / reviewer / support controlling execution
- a planning-session product runtime
- a flow-template runtime for normal PM or CRM work
- a hidden epic-planner phase machine
- a generic taxonomy where agents, rules, and flows are equal first-class automation kinds

The migration history explicitly removed older `agent_class`, `capability_profile`, flow, and planning-session structures. Some historical names still appear in SQL comments, old docs, or compatibility constants, but they are not the active model.

## Core concepts

### Agent

An agent is a configured executor record in `agents`.

Important fields:

- `is_system`
- `preset_key`
- `role`
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
- `max_concurrent_runs`
- `default_invocation_mode`

Important implications:

- agents are workspace-scoped, not global
- agents are not runtime classes
- presets seed defaults, but per-agent policy can override them
- system agents are seeded by the product, especially the workspace Epic Planner

### Agent run

An `agent_run` is one durable execution against one target.

Normal entry points converge here:

- running an agent on a story
- interactive epic planning
- support conversation execution
- scheduled agent execution
- automation rules that start an agent run

Important fields:

- `target_type`
- `target_id`
- `runtime_kind`
- `invocation_mode`
- `approval_state`
- `pause_reason`
- `status`
- `execution_stage`
- `task_queue`
- `runner_pool`

### Preset

A preset is a bundle of defaults, not a special engine.

Current presets:

- `epic_planner`
- `story_planner`
- `crm_operator`
- `support_agent`
- `code_builder`
- `review_agent`

Presets define defaults for:

- label and role
- runtime kind
- trigger modes allowed
- tool and command policy
- target types
- approval mode
- default invocation mode
- default system prompt

Presets do not create hidden orchestration behavior.

### Runtime kind

Runtime kind selects the underlying execution environment.

Current runtime kinds:

- `native_sdk`
- `opencode`

Current behavior:

- `native_sdk` supports both `interactive` and `autonomous`
- `opencode` supports `autonomous` only

### Invocation mode

Invocation mode describes how the run behaves from a human interaction perspective.

Current modes:

- `interactive`
  - transcript-driven
  - can ask questions inline
  - can pause for human input or human approval
- `autonomous`
  - background-oriented
  - may still pause for approval if policy demands it
  - does not depend on a chat-first planning loop

### Trigger mode

Trigger mode belongs to the agent configuration, not the run.

Current values:

- `manual`
- `auto_on_assignment`
- `auto_on_event`

This is separate from `schedule`. A scheduled agent is still an agent that ultimately creates normal `agent_runs`.

### Approval mode and pause state

Approval mode is agent policy:

- `never`
- `always`
- `preset_default`

Run-time approval and pause state are stored on the run:

- `approval_state`: `not_required`, `pending`, `approved`, `rejected`
- `pause_reason`: `none`, `human_input`, `human_approval`

The important distinction is:

- approval mode is static policy on the agent
- approval state is dynamic state on one run

### Tools are the safety boundary

Tools are the primary capability boundary.

The platform does not treat the prompt as the safety layer. The safety layer is:

- allowed tools
- allowed commands
- allowed targets
- backend validation inside tool handlers and services

Examples of backend-enforced invariants:

- story planning cannot create stories for a teamless epic
- document tools validate document and workspace ownership
- support tools keep support-specific reply behavior in service code
- CRM mutation tools validate entity existence and permissions

## Current preset model

The current presets are:

| Preset | Purpose | Runtime | Default mode | Typical targets |
| --- | --- | --- | --- | --- |
| `epic_planner` | Interactive PRD-to-stories planning loop | `native_sdk` | `interactive` | `epic`, `story`, `crm_deal` |
| `story_planner` | Interactive decomposition and refinement | `native_sdk` | `interactive` | `story`, `epic` |
| `crm_operator` | Cross-app CRM assistance | `native_sdk` | `interactive` | `crm_deal`, `support_conversation`, `document` |
| `support_agent` | Support triage and draft replies | `native_sdk` | `autonomous` | `support_conversation` |
| `code_builder` | Repo-writing implementation agent | `opencode` | `autonomous` | `story` |
| `review_agent` | Validation and review without repo mutation | `opencode` | `autonomous` | `story` |

Important details:

- `story_planner` and `crm_operator` are not separate runtimes; they are preset variants on the same core model
- `support_agent` defaults to approval-required behavior
- `code_builder` and `review_agent` are autonomous `opencode` agents
- the Epic Planner is the default system agent seeded for a workspace

## Execution model

### Queue topology

Queues are chosen by runtime kind and invocation mode:

- `agent-native-interactive`
- `agent-native-autonomous`
- `agent-opencode-autonomous`
- `automation-default`

Current queue mapping:

- `native_sdk + interactive` -> `agent-native-interactive`
- `native_sdk + autonomous` -> `agent-native-autonomous`
- `opencode` -> `agent-opencode-autonomous`
- non-agent background automations -> `automation-default`

### Durable run workflow

Agent execution uses one Temporal workflow per run:

1. create `agent_run`
2. prepare the run
3. execute
4. pause when waiting for approval or input
5. resume from Temporal signals
6. complete, fail, or cancel

The workflow exposes run-stage queries such as:

- current step
- approval wait state
- input wait state

This is the active durable runtime. There is no separate planning-session table or flow-node engine behind normal work.

### Interactive run semantics

Interactive runs are still just `agent_runs`.

The epic planning experience is one interactive run against an epic. The active model is:

1. ask clarifying questions inline when needed
2. draft the PRD inline
3. publish preview and request approval inline
4. persist the approved spec through tools
5. draft the story plan inline
6. publish preview and request approval inline
7. create stories through tools

Important constraints:

- there is no separate planning-session product
- there is no hidden planner phase machine in backend orchestration
- approvals are soft gates inside the same transcript

### Scheduled agents

Agents may also have a cron `schedule`.

When scheduled:

- a Temporal cron workflow fires on the schedule
- each tick creates a normal `agent_run`
- the run target is internal scheduled context
- overlapping scheduled runs for the same agent are prevented

This means scheduling is a triggering mechanism for agent runs, not a new automation category.

## Built-in automation taxonomy

Built-in automations are product-owned behavior. They exist outside the generic agent UI even when they use LLMs or Temporal.

The current built-in catalog breaks down into two functional families.

### CRM system intelligence

#### Buyer signal ingestion

Catalog ID: `crm.buyer_signal_ingestion`

Purpose:

- inspect stored CRM email after it has been persisted
- determine eligibility for detection
- run LLM-based signal extraction
- store durable buyer signals with provenance and duplicate suppression

Important properties:

- source of truth is normalized CRM email in Postgres, not raw provider payloads
- one Temporal workflow is started per eligible message
- task queue is `automation-default`
- duplicate suppression exists at both source and thread-window levels
- output is stored in `crm_buyer_signals`

This is built-in automation, not a user-visible agent.

#### Contact summary refresh

Catalog ID: `crm.contact_summary_refresh`

Purpose:

- maintain durable summaries for contacts
- refresh them from recent CRM email activity and buyer signals

Important properties:

- one durable summary row per workspace + entity
- debounced Temporal workflow per entity
- daily reconciliation re-queues recently touched contacts
- summary status can be `pending_refresh`, `ready`, `stale`, or `error`

#### Deal summary refresh

Catalog ID: `crm.deal_summary_refresh`

Purpose:

- maintain durable summaries for deals
- refresh them from recent CRM email activity and buyer signals

Important properties are the same shape as contact summaries, but the target entity is `deal`.

### PM built-in rules

These are deterministic PM automations stored in `pm_automations`.

#### Epic auto-start

Catalog ID: `pm.epic_auto_start`

Purpose:

- automatically start an epic when one of its stories enters a started workflow state

Characteristics:

- workspace-scoped
- deterministic
- runs inline from story state change hooks

#### Epic auto-complete

Catalog ID: `pm.epic_auto_complete`

Purpose:

- automatically complete an epic when all of its stories are done

Characteristics:

- workspace-scoped
- deterministic
- runs inline from story state change hooks

#### Sprint auto-create

Catalog ID: `pm.sprint_auto_create`

Purpose:

- keep a configured future sprint buffer for each team

Characteristics:

- team-scoped
- cron-driven
- part of the PM automation sweep

#### Move unfinished stories

Catalog ID: `pm.sprint_move_unfinished`

Purpose:

- move unfinished work into the next sprint for configured teams

Characteristics:

- team-scoped
- cron-driven
- deterministic

## Automation rules

Automation rules are the only current user-authored automation type.

They are stored in `automation_rules` and consist of:

- scope
- trigger
- action
- ordering
- optional stop-on-match behavior

Important fields:

- `team_id`
- `workflow_id`
- `trigger_type`
- `trigger_config`
- `action_type`
- `action_config`
- `position`
- `stop_on_match`

### Current triggers

- `story.state_entered`
- `agent_run.approved`
- `cron`

Trigger config shapes currently support:

- exact state match
- state-type match such as `started` or `done`
- cron category match
- approved run state match

### Current actions

The active action surface is:

- `start_agent_run`
- `move_to_state`
- `merge_branch`
- `run_command`

Historical constants still present in code:

- `run_agent`
- `start_flow`

Current behavior for those historical actions:

- `run_agent` returns an error and is no longer supported
- `start_flow` returns an error and is no longer supported

### Current limitations

Important runtime limitations today:

- `start_agent_run` currently requires a story target
- rules are evaluated by the in-process rule engine, not by a generic flow runtime
- loop prevention is handled by rule execution context and chain-depth limits

## Agents and automation rules are connected, but different

The relationship is:

- rules decide when something should happen
- agents define who can execute a kind of work
- runs are the actual durable execution object

So a rule may launch an agent run, but that does not make the agent a top-level automation kind.

## The settings and inventory model

The Settings UI has a read-only inventory model for AI and automations.

The inventory is assembled from:

- the code-defined automation catalog
- PM automation settings
- CRM email-account state
- automation health snapshots
- automation rules

Current inventory kinds exposed to the UI are only:

- `built_in_automation`
- `automation_rule`

Current inventory groups returned by the service are:

- `Built-in Automations`
- `Automation Rules`

Even though there are some unused grouping constants in code, the active settings response only exposes those two sections.

### Health model

Built-in automations record health snapshots with:

- `healthy`
- `warning`
- `error`
- `inactive`
- `unknown`

Health is currently used for:

- CRM intelligence built-ins
- PM built-in rules
- rule inventory display

It is diagnostic metadata, not a separate automation taxonomy.

## CRM taxonomy inside the automation model

The CRM subsystem has its own domain terms, but they fit into the same platform model.

### Buyer signals

Buyer signals are durable CRM intelligence records, not agent transcripts.

Current signal types:

- `buying_intent`
- `objection`
- `competitor_mention`
- `budget_signal`
- `timeline_signal`
- `champion_signal`
- `risk_signal`

Current signal sources:

- `email`
- `meeting`
- `note`
- `manual`
- `support`

Signals are outputs of built-in CRM automation and manual CRM operations. They are not "agent state".

### Entity summaries

Entity summaries are durable CRM artifacts, not chat outputs.

Current supported entity types:

- `contact`
- `deal`

Current summary statuses:

- `pending_refresh`
- `ready`
- `stale`
- `error`

Summaries are owned by the CRM intelligence pipeline, not the agent UI layer.

## PM taxonomy inside the automation model

The PM subsystem uses two separate automation shapes.

### PM built-ins

These are the deterministic `pm_automations` records:

- epic auto-start
- epic auto-complete
- sprint auto-create
- sprint move unfinished

### PM rules

These are the user-authored `automation_rules` records.

They are broader than the deterministic PM built-ins because they can:

- react to state changes
- react to run approvals
- invoke agent runs
- move work
- trigger controlled commands

## Support taxonomy inside the automation model

Support currently appears in two different ways:

- as a target for the `support_agent` preset
- as a source for CRM signals via `support`

Important distinction:

- support-agent runs are normal agent runs
- support-derived CRM signals are built-in CRM automation outputs

They are related by data flow, not by sharing a top-level taxonomy kind.

## Design rules for new work

When adding new work in this area:

- do not add a new top-level automation kind unless the platform model truly changes
- do not introduce new hidden agent classes
- do not build a parallel flow runtime for normal PM or CRM work
- prefer extending presets over adding new execution concepts
- keep safety and business invariants in tools and services
- use `agent_run` as the durable execution primitive
- put product-owned background behavior into built-in automation
- use automation rules only for user-authored trigger-to-action behavior

## Practical classification guide

Use this decision rule:

### It is an agent when

- a configurable executor should work against a target
- humans may run it directly
- policy is mostly about tools, commands, targets, runtime, and approval
- the result should be represented as an `agent_run`

### It is built-in automation when

- the product owns the behavior
- users do not author the execution logic directly
- it should run in the background or inline as system behavior
- the output is a system artifact such as a signal, summary, or deterministic PM state mutation

### It is an automation rule when

- a user is defining trigger-to-action behavior
- the action surface is small and controlled
- the rule should react to product events rather than own a complex execution transcript

## Historical residue to ignore for new work

Some residue still exists in the repository:

- reference SQL mentioning older class-era fields
- older docs describing flows or planning sessions
- compatibility constants such as `run_agent` and `start_flow`
- helper code that still maps older names onto the current preset model

These are not the platform model to build on.

## File map

Useful code entry points for this model:

- agent models: `server/internal/model/agent.go`
- invocation modes: `server/internal/model/agent_runtime.go`
- agent presets: `server/internal/service/agent_presets.go`
- agent policy normalization: `server/internal/service/agent_policy.go`
- agent service and run lifecycle: `server/internal/service/agent.go`
- runtime profiles: `server/internal/worker/runtime_profiles.go`
- queue mapping: `server/internal/temporalapp/queues.go`
- run workflow: `server/internal/temporalapp/workflow.go`
- automation catalog: `server/internal/automationcatalog/registry.go`
- automation inventory: `server/internal/service/automation_inventory.go`
- automation model types: `server/internal/model/automation.go`
- automation rule model: `server/internal/model/automation_rule.go`
- automation rule engine: `server/internal/service/automation_rule_engine.go`
- PM automations: `server/internal/model/pm_automation.go`
- PM automation service: `server/internal/service/pm_automation.go`
- CRM signal model: `server/internal/model/crm_signal.go`
- CRM signal workflow: `server/internal/temporalapp/signal_detection_workflow.go`
- CRM summary model: `server/internal/model/crm_summary.go`
- CRM summary workflow: `server/internal/temporalapp/crm_summary_workflow.go`

## Bottom line

The platform model is:

- agents are configurable executors
- agent runs are the durable unit of execution
- built-in automations are product-owned system behavior
- automation rules are user-authored trigger-to-action logic
- the top-level automation taxonomy has two kinds, not three

That is the intended shape of the platform and the model new work should follow.
