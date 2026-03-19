# Current Agent, Flow, and Automation Architecture

This document is the current-state guide to the new automation platform in Teampulse. It explains what each concept means, how the pieces connect, where they run, what is configurable today, and what is groundwork versus fully active behavior.

It is written from the code as it exists today in:

- `server/internal/service/*`
- `server/internal/worker/*`
- `server/internal/temporalapp/*`
- `server/internal/model/*`
- `frontend/src/pages/pm/Agents.tsx`
- `frontend/src/pages/pm/Flows.tsx`
- `frontend/src/components/settings/PipelineBuilder.tsx`

## 1. The simplest mental model

There are 4 different layers that people often mix together:

```mermaid
graph TD
    subgraph "The 4 Layers"
        BI["<b>Built-in Automations</b><br/>Product-owned behavior<br/><i>CRM sync, epic auto-start,<br/>sprint auto-create</i>"]
        AR["<b>Automation Rules</b><br/>Trigger → Action rules<br/><i>story enters state → run agent,<br/>agent approved → move state</i>"]
        AG["<b>Agents</b><br/>LLM executors<br/><i>engineer, planner,<br/>reviewer, support</i>"]
        FL["<b>Flows</b><br/>Multi-step orchestrations<br/><i>planning sessions, agent runs,<br/>approval gates, system actions</i>"]
    end

    AR -->|"decides WHEN"| AG
    AG -->|"does THE WORK"| FL
    FL -->|"coordinates STEPS"| AG

    style BI fill:#e8f4e8,stroke:#4a9
    style AR fill:#e8e8f4,stroke:#66a
    style AG fill:#f4e8e8,stroke:#a66
    style FL fill:#f4f4e8,stroke:#aa6
```

1. **Built-in automations**
   Product-owned automation Teampulse ships itself.
   Examples: CRM buyer-signal ingestion, CRM summary refresh.

2. **Automation rules**
   User-configured trigger -> action rules, used for PM workflow pipelines and PM built-in automations.
   Examples: "when a story enters In Progress, run engineer agent", "when an agent run is approved, move the story to the next state", "when any story enters a started state, auto-start its epic".

3. **Agents**
   Explicit LLM executors that can be assigned, configured, run, approved, handed off, and audited.

4. **Flows**
   Durable multi-step orchestrations made of nodes.
   A flow can launch planning sessions, agent runs, approval gates, and system actions in a single tracked run.

The important split is:

- **rules decide when something should happen**
- **agents do the work**
- **flows coordinate multiple steps**
- **built-ins are product behavior that exists outside the user-authored rule graph**

## 2. How the pieces connect

### A. Stage-based PM pipeline

```mermaid
flowchart TD
    A["Story enters workflow state"] --> B["AutomationRuleEngine<br/>evaluates matching rules"]
    B --> C["run_agent"]
    B --> H["move_to_state"]
    B --> I["merge_branch"]
    B --> J["run_command"]
    J --> K["InternalCommandService"]
    C --> D["AgentRun created"]
    D --> E["Temporal AgentRunWorkflow"]
    E --> F["Runtime executes tools/work"]
    F --> G{"Approval<br/>required?"}
    G -->|"Yes"| G2["Awaiting approval"]
    G -->|"No"| DONE["Complete"]
    G2 -->|"agent_run.approved<br/>trigger fires"| B

    style A fill:#e8f4e8,stroke:#4a9
    style B fill:#e8e8f4,stroke:#66a
    style E fill:#f4e8e8,stroke:#a66
    style G fill:#f4f4e8,stroke:#aa6
```

### B. Durable Flow

```mermaid
flowchart LR
    FR["FlowRun"] --> N1["system_action"]
    FR --> N2["interactive_agent"]
    FR --> N3["agent_task"]
    FR --> N4["approval_gate"]
    FR --> N5["terminal"]

    N2 -->|"creates child"| PS["PlanningSession"]
    N3 -->|"creates child"| AR["AgentRun"]

    PS -->|"signals parent"| FR
    AR -->|"signals parent"| FR

    style FR fill:#e8e8f4,stroke:#66a
    style PS fill:#f4e8e8,stroke:#a66
    style AR fill:#f4e8e8,stroke:#a66
    style N5 fill:#ddd,stroke:#999
```

The parent flow waits for child state changes from planning sessions or agent runs, then moves to the next node.

## 3. Current architecture by category

## 3.1 Built-in automations

Current built-in automations fall into 3 groups:

- **CRM background workflows**
  Examples: buyer-signal ingestion, contact summary refresh, deal summary refresh.
- **PM deterministic inline rules**
  Examples: epic auto-start, epic auto-complete.
- **PM scheduled rules**
  Examples: sprint auto-create, move unfinished stories.

Where they are configured:

- CRM intelligence: CRM settings surfaces
- PM built-ins: workspace/team settings automations
- Inventory and health: `Settings > AI & Automations`

Where they run:

- CRM background jobs: mostly **Temporal-backed**
- PM epic auto-start / auto-complete: **inline in API/service code** on story state changes, and also via `run_command` automation rules (both paths fire during transition; idempotent)
- PM sprint automations: **Temporal cron workflow** (`SprintAutomationCronWorkflow`) on the `automation-default` queue, running hourly

Important: not all automation in Teampulse is a flow or an agent.

## 3.2 Automation rules

Automation rules are stored in `automation_rules` and evaluated by `AutomationRuleEngine`.

Current active trigger types:

- `story.state_entered` — fires when a story enters a workflow state (supports matching by exact `state_id` or by `state_type` like `"started"` or `"done"`)
- `agent_run.approved` — fires when an agent run is approved
- `cron` — fires on a scheduled category (e.g. `"sprint_hourly"`), evaluated by `EvaluateCronRules`

Current active action types:

- `run_agent` — assigns an agent and starts an agent run
- `move_to_state` — moves the story to a target workflow state
- `merge_branch` — merges the story's working branch into a target branch
- `run_command` — dispatches to `InternalCommandService.Execute()` with a command name and input payload
- `start_flow` — (defined, not yet implemented) will dispatch via `FlowService.StartRun()`

Current main UI:

- `Settings > Teams > Workflow`
- `Settings > Workflows`
- the visual stage pipeline builder in `PipelineBuilder.tsx`

What the pipeline builder actually does:

- assigning an agent to a state creates a `story.state_entered -> run_agent` rule
- enabling auto-advance creates an `agent_run.approved -> move_to_state` rule
- enabling merge branch creates a `story.state_entered -> merge_branch` rule

So the "pipeline" is not a separate runtime. It is a structured UI on top of automation rules.

Additionally, PM built-in automations (epic auto-start/complete, sprint auto-create, sprint move-unfinished) are now also stored as `automation_rules` with `run_command` actions that dispatch through `InternalCommandService`.

## 3.3 Agents

Agents are workspace-scoped records in `agents`.

They have their own lifecycle:

- configuration
- assignment
- run creation
- execution
- approval
- handoff
- artifacts
- audit history

### Agent classes

Current canonical classes:

- `product_planner`
- `engineer`
- `reviewer`
- `support`

Agents are LLM-only. Humans are modeled separately as users or workspace members for assignment and handoff.

### Runtime kinds

Current implemented runtime kinds:

- `opencode`
- `native_sdk`

What they mean:

- `opencode`
  The Temporal worker prepares a workspace and shells out to the OpenCode runtime.
- `native_sdk`
  The Temporal worker runs the agent in-process using the SDK-backed runtime and tool registry.

### Current runtime profiles

Runtime policy starts from a class/capability profile, then may be overridden per agent.

Current class defaults:

| Profile | Default runtime | Main purpose | Default target types |
| --- | --- | --- | --- |
| `engineer` | `opencode` | code implementation | `story` |
| `product_planner` | `native_sdk` | planning/review across PM/Docs/CRM | `epic`, `story`, `crm_deal` |
| `reviewer` | `opencode` | validation/testing | `story` |
| `support` | `native_sdk` | support triage and drafts | `support_conversation` |

### Supported invocation modes

Modes are used mostly by flows and planning:

- `interactive`
- `autonomous`

Current behavior:

- `product_planner`: autonomous, and interactive if the runtime/provider supports it
- `support`: autonomous, and interactive if the runtime/provider supports it
- `engineer`: autonomous only
- `reviewer`: autonomous only

### Per-agent options that are configurable today

These are set on the PM Agents page and stored on the agent record:

- name
- class
- runtime kind
- provider
- model
- system prompt
- planning notes
- monthly token budget
- trigger mode
- schedule
- approval mode
- max concurrent runs
- allowed tools
- allowed commands
- allowed targets
- team ownership

### Trigger modes on the agent record

The schema supports:

- `manual`
- `auto_on_assignment`
- `auto_on_event`

Current code reality:

- `manual` is active
- story assignment / story movement auto-run paths are active
- schedule-based execution is active
- rule-driven execution is active
- `auto_on_event`, `trigger_events`, and `target_selector` exist on the model but do not currently have a general event-bus runner wired through the codebase

That means some trigger configuration is already modeled, but not all of it is fully operational yet.

## 3.4 Flows

Flows are durable orchestrations stored in:

- `flow_runs`
- `flow_node_runs`

They are exposed in the PM Flows page and orchestrated by `FlowRunWorkflow`.

A flow is:

- a **template** with a fixed node graph
- a **run** against a target object
- a series of **node runs** that track progress, retries, output, approval state, and child IDs

### Current built-in flow templates

Implemented templates:

1. `pm.epic_planning_v2`
2. `pm.story_completion_v1`
3. `pm.agent_story_run`
4. `crm.deal_review_v1`

### Current flow targets

- `epic`
- `story`
- `crm_deal`

### Current flow node types

All current nodes are one of:

- `interactive_agent`
- `agent_task`
- `approval_gate`
- `system_action`
- `terminal`

### What each node type means

#### `interactive_agent`

Used when a human and agent collaborate live.

Current behavior:

- creates a child `PlanningSession`
- flow enters `awaiting_input`
- user sends messages directly to the session
- finalization signals the parent flow to continue

Typical use:

- drafting/spec planning in epic planning flows

#### `agent_task`

Used for autonomous agent work.

Current behavior:

- creates a child `AgentRun`
- flow waits while the run is queued/running
- on completion or approval state change, the child signals the parent flow

Typical use:

- story planning
- story completion assessment
- deal review

#### `approval_gate`

Used when a human must decide what happens next.

Current behavior:

- flow enters `awaiting_approval`
- actions can include `approve`, `request_changes`, `reject`
- `request_changes` can loop back to an earlier node

#### `system_action`

Used for deterministic internal commands.

Current behavior:

- no child agent run is created
- the node dispatches to `InternalCommandService.Execute()` — the same dispatch layer used by `run_command` rule actions

Current command examples:

- `docs.ensure_spec_doc`
- `pm.approve_epic_spec`
- `pm.create_story_batch`
- `pm.create_followup_stories`
- `pm.auto_start_epic`
- `pm.auto_complete_epic`
- `pm.sprint_auto_create`
- `pm.sprint_move_unfinished`
- `delivery.merge_branch`
- `crm.apply_deal_actions`

#### `terminal`

Marks the end of the flow.

## 4. Current flow templates and their node graphs

### Epic Planning v2

```mermaid
flowchart LR
    A["ensure_spec_doc<br/><i>system_action</i>"] --> B["spec_draft<br/><i>interactive_agent</i>"]
    B --> C{"spec_approval<br/><i>approval_gate</i>"}
    C -->|"approve"| D["story_plan<br/><i>interactive_agent</i>"]
    C -->|"request_changes"| B
    D --> E{"plan_approval<br/><i>approval_gate</i>"}
    E -->|"approve"| F["create_stories<br/><i>system_action</i>"]
    E -->|"request_changes"| D
    F --> G(["done"])

    style C fill:#f4f4e8,stroke:#aa6
    style E fill:#f4f4e8,stroke:#aa6
    style G fill:#ddd,stroke:#999
```

### Story Completion

```mermaid
flowchart LR
    A["completion_assessment<br/><i>agent_task</i>"] --> B{"completion_review<br/><i>approval_gate</i>"}
    B -->|"approve"| C["create_followups<br/><i>system_action</i>"]
    C --> D(["done"])

    style B fill:#f4f4e8,stroke:#aa6
    style D fill:#ddd,stroke:#999
```

### Agent Story Run

```mermaid
flowchart LR
    A["implement<br/><i>agent_task</i>"] --> B(["done"])

    style B fill:#ddd,stroke:#999
```

### CRM Deal Review

```mermaid
flowchart LR
    A["deal_review<br/><i>agent_task</i>"] --> B{"deal_review_approval<br/><i>approval_gate</i>"}
    B -->|"approve"| C["apply_deal_actions<br/><i>system_action</i>"]
    C --> D(["done"])

    style B fill:#f4f4e8,stroke:#aa6
    style D fill:#ddd,stroke:#999
```

## 5. Node options: what is configurable and where

Today, flow templates are DB-backed, but runtime behavior is still mostly driven by:

- node type
- per-node metadata on `flow_template_nodes`
- a small set of hardcoded launch/input builders for known commands and approval flows

There is not yet a first-class producer/consumer contract between nodes.

Each node can define:

- `id` / `node_slug`
- `label`
- `type`
- `next_node_slug`
- `loopback_node_slug`
- `required_mode`
- `actions`
- `allowed_tools`
- `agent_input_key`
- `default_agent_id`
- `fallback_agent_key`
- `system_prompt`
- `output_tag`
- `command_name`
- `approve_command_name`
- `feedback_from_node`
- `additional_context_key`
- `retryable`

### Current per-node tool sets

Planning-style nodes currently use read-heavy tool sets such as:

- `read_file`
- `read_file_range`
- `list_directory`
- `search_files`
- `ripgrep`
- `grep`
- `list_symbols`
- `web_search`
- `list_documents`
- `read_document`
- `search_documents`

Story completion assessment currently uses:

- `list_documents`
- `read_document`
- `search_documents`
- `list_story_checklist`

CRM deal review currently uses:

- `list_deals`
- `list_buyer_signals`
- `list_contacts`

### Where node-level agent choice is set

It depends on the flow:

- epic planning flow start picks a spec planner and optionally a story planner
- story completion flow start picks one agent
- deal review flow start picks one agent

Those are selected at **flow start time** on the PM Flows page and persisted into the run input/spec snapshot.

### Proposed future-state plan: explicit node contracts

This section is a design proposal, not current runtime behavior.

The goal is to make three things explicit:

1. what input is required to start a flow
2. what each node requires before it can run
3. what each node guarantees it emits for later nodes

The main principle is: nodes should depend on named artifacts, not on ad hoc knowledge of a previous node's raw JSON shape.

### Why this is needed

Today, node-to-node data flow is a mix of:

- flow start input such as `agent_id` or `spec_planner_agent_id`
- node metadata such as `feedback_from_node`, `output_tag`, and `command_name`
- command-specific code that reads prior node runs and parses their output
- approval overrides stored inside approval node output

That works for the current built-ins, but it does not scale well when:

- a template gains alternative branches
- the same artifact can come from different node types
- a new system action needs structured input from two earlier nodes
- the UI needs to explain why a node cannot run
- a workspace-authored template should be validated before it is ever started

### The proposed contract model

The future model should have 4 explicit contract layers:

| Layer | Purpose |
| --- | --- |
| `flow_input_contract` | Declares fields required or accepted at flow start |
| `node_runtime_contract` | Declares node-type runtime requirements such as agent mode, approval support, command execution, tool policy |
| `node_data_contract` | Declares what the node consumes and what it produces |
| `flow_output_contract` | Declares which artifact becomes the final flow result |

### Proposed data shape

At the template level:

```json
{
  "template_slug": "pm.story_completion_v1",
  "input_contract": {
    "fields": [
      { "key": "agent_id", "schema_ref": "core.agent_ref.v1", "required": true },
      { "key": "additional_context", "schema_ref": "core.text.v1", "required": false }
    ]
  },
  "output_contract": {
    "primary_artifact": "followup_creation_result",
    "schema_ref": "pm.followup_creation_result.v1"
  }
}
```

At the node level:

```json
{
  "node_slug": "create_followups",
  "node_type": "system_action",
  "contract": {
    "consumes": [
      {
        "name": "followups",
        "from": "artifact.approved_followups",
        "schema_ref": "pm.story_completion_followups.v1",
        "required": true
      }
    ],
    "produces": [
      {
        "artifact_key": "followup_creation_result",
        "schema_ref": "pm.followup_creation_result.v1",
        "required": true
      }
    ]
  }
}
```

### Recommended storage model

Keep `flow_node_runs.input` and `flow_node_runs.output` as the low-level audit record.

Add a normalized artifact layer for contracts:

- `flow_run_artifacts`
  - `id`
  - `flow_run_id`
  - `artifact_key`
  - `schema_ref`
  - `producer_node_slug`
  - `producer_node_run_id`
  - `version`
  - `status`
  - `payload`
  - `created_at`

This gives us two useful levels:

- node run I/O for debugging and raw replay
- artifact registry for validation, lookup, UI, and future branching

### Artifact lookup rules

Nodes should consume from explicit sources:

- `flow_input.<key>`
- `artifact.<artifact_key>`
- `target_context.<key>`
- `template_default.<key>`
- `constant.<value>`

For migration only, allow one temporary legacy source:

- `legacy_node_output.<node_slug>`

The engine should prefer `artifact.*` and treat direct parsing of `legacy_node_output.*` as a compatibility bridge to be removed.

### Schema naming and versioning

Every structured artifact should carry a `schema_ref` such as:

- `core.agent_ref.v1`
- `core.review_feedback.v1`
- `pm.spec_draft.v1`
- `pm.story_plan.v1`
- `pm.story_completion_assessment.v1`
- `crm.deal_action_plan.v1`

Versioning rule:

- additive fields keep the same major version
- breaking shape changes create a new major version
- templates pin the version they consume

That lets future nodes evolve without silently breaking older templates.

### Node-type runtime contracts

Each node type should have a stable runtime contract beyond its data contract.

#### `interactive_agent`

Required runtime contract:

- exactly one resolved agent reference
- agent supports `interactive`
- optional stage
- optional system prompt
- optional narrowed tool policy

Typical data contract:

- consumes target context, optional prior review feedback, optional prior artifacts
- produces one primary structured artifact plus a `planning_session_ref`

#### `agent_task`

Required runtime contract:

- exactly one resolved agent reference
- agent supports `autonomous`
- optional system prompt
- optional narrowed tool policy

Typical data contract:

- consumes target context, optional prior review feedback, optional prior artifacts
- produces one primary structured artifact plus an `agent_run_ref`

#### `approval_gate`

Required runtime contract:

- one review subject artifact to present
- supported actions such as `approve`, `request_changes`, `reject`
- optional override schema for approve/reject payloads

Typical data contract:

- consumes one primary subject artifact
- produces `approval_decision`
- produces `review_feedback` on `request_changes`
- may produce an override artifact on `approve`

#### `system_action`

Required runtime contract:

- command name
- deterministic command input builder based on declared consumed artifacts

Typical data contract:

- consumes one or more structured artifacts
- produces one command result artifact

#### `terminal`

Required runtime contract:

- knows which artifact or artifact set becomes the flow result

Typical data contract:

- consumes the primary terminal artifact
- copies it into `flow_runs.output_summary`

### Proposed current artifact catalog

These schema refs are enough to cover the current built-in templates and leave room for future nodes:

| Artifact key | Schema ref | Purpose |
| --- | --- | --- |
| `agent_ref` | `core.agent_ref.v1` | Resolved agent identity |
| `planning_session_ref` | `core.planning_session_ref.v1` | Interactive child linkage |
| `agent_run_ref` | `core.agent_run_ref.v1` | Autonomous child linkage |
| `review_feedback` | `core.review_feedback.v1` | Reviewer comment plus structured feedback |
| `approval_decision` | `core.approval_decision.v1` | Approval outcome metadata |
| `spec_document_ref` | `pm.spec_document_ref.v1` | Spec doc identity |
| `spec_draft` | `pm.spec_draft.v1` | Structured spec output |
| `story_plan` | `pm.story_plan.v1` | Structured story decomposition |
| `story_batch_request` | `pm.story_batch_request.v1` | Approved stories ready for creation |
| `story_batch_result` | `pm.story_batch_result.v1` | Story creation result |
| `story_completion_assessment` | `pm.story_completion_assessment.v1` | Completion summary plus followups |
| `approved_followups` | `pm.story_completion_followups.v1` | Final followups to create |
| `followup_creation_result` | `pm.followup_creation_result.v1` | Followup creation result |
| `deal_action_plan` | `crm.deal_action_plan.v1` | Recommended deal changes |
| `deal_action_apply_result` | `crm.deal_action_apply_result.v1` | Applied deal action result |
| `implementation_result` | `pm.story_implementation_result.v1` | Output of `pm.agent_story_run` |

### How the current templates should look under this model

#### `pm.epic_planning_v2`

Flow input contract:

- `spec_planner_agent_id`: required, `core.agent_ref.v1`
- `story_planner_agent_id`: optional, `core.agent_ref.v1`
- `additional_context`: optional, `core.text.v1`

Resolver behavior:

- if `story_planner_agent_id` is missing, fall back to `spec_planner_agent_id`
- if still missing, use the target epic's default orchestrator agent when present

Node contracts:

1. `ensure_spec_doc` (`system_action`)
   - consumes target epic context
   - produces `spec_document_ref`

2. `spec_draft` (`interactive_agent`)
   - consumes `flow_input.spec_planner_agent_id`
   - consumes `flow_input.additional_context`
   - consumes `artifact.spec_document_ref`
   - optionally consumes `artifact.spec_review_feedback`
   - produces `spec_draft`
   - produces `planning_session_ref`

3. `spec_approval` (`approval_gate`)
   - consumes `artifact.spec_draft`
   - produces `approval_decision`
   - produces `spec_review_feedback` when requesting changes
   - may produce `approved_spec` when approving with override payload

4. `story_plan` (`interactive_agent`)
   - consumes resolved `story_planner_agent_id`
   - consumes `flow_input.additional_context`
   - consumes `artifact.spec_draft` or `artifact.approved_spec`
   - optionally consumes `artifact.story_plan_feedback`
   - produces `story_plan`
   - produces `planning_session_ref`

5. `plan_approval` (`approval_gate`)
   - consumes `artifact.story_plan`
   - produces `approval_decision`
   - produces `story_plan_feedback` when requesting changes
   - may produce `story_batch_request` when approving with override payload

6. `create_stories` (`system_action`)
   - consumes `artifact.story_batch_request` when present
   - otherwise consumes `artifact.story_plan`
   - produces `story_batch_result`

7. `done` (`terminal`)
   - consumes `artifact.story_batch_result`
   - exposes it as the flow output

This removes the current command-specific coupling where `create_stories` knows how to inspect earlier node runs and decode their child outputs.

#### `pm.story_completion_v1`

Flow input contract:

- `agent_id`: required, `core.agent_ref.v1`
- `additional_context`: optional, `core.text.v1`

Node contracts:

1. `completion_assessment` (`agent_task`)
   - consumes `flow_input.agent_id`
   - consumes `flow_input.additional_context`
   - optionally consumes `artifact.completion_review_feedback`
   - produces `story_completion_assessment`
   - produces `agent_run_ref`

2. `completion_review` (`approval_gate`)
   - consumes `artifact.story_completion_assessment`
   - produces `approval_decision`
   - produces `completion_review_feedback` when requesting changes
   - may produce `approved_followups` when approving with override payload

3. `create_followups` (`system_action`)
   - consumes `artifact.approved_followups` when present
   - otherwise consumes `artifact.story_completion_assessment`
   - produces `followup_creation_result`

4. `done` (`terminal`)
   - consumes `artifact.followup_creation_result`
   - exposes it as the flow output

#### `crm.deal_review_v1`

Flow input contract:

- `agent_id`: required, `core.agent_ref.v1`
- `additional_context`: optional, `core.text.v1`

Node contracts:

1. `deal_review` (`agent_task`)
   - consumes `flow_input.agent_id`
   - consumes `flow_input.additional_context`
   - optionally consumes `artifact.deal_review_feedback`
   - produces `deal_action_plan`
   - produces `agent_run_ref`

2. `deal_review_approval` (`approval_gate`)
   - consumes `artifact.deal_action_plan`
   - produces `approval_decision`
   - produces `deal_review_feedback` when requesting changes
   - may produce `approved_deal_action_plan` when approving with override payload

3. `apply_deal_actions` (`system_action`)
   - consumes `artifact.approved_deal_action_plan` when present
   - otherwise consumes `artifact.deal_action_plan`
   - produces `deal_action_apply_result`

4. `done` (`terminal`)
   - consumes `artifact.deal_action_apply_result`
   - exposes it as the flow output

#### `pm.agent_story_run`

Flow input contract:

- `agent_id`: required, `core.agent_ref.v1`
- `additional_context`: optional, `core.text.v1`

Node contracts:

1. `implement` (`agent_task`)
   - consumes `flow_input.agent_id`
   - consumes `flow_input.additional_context`
   - produces `implementation_result`
   - produces `agent_run_ref`

2. `done` (`terminal`)
   - consumes `artifact.implementation_result`
   - exposes it as the flow output

### Validation rules the template editor should enforce

On template save:

- every required `consumes` binding must resolve to one valid source
- every referenced artifact must be produced earlier on every reachable non-reject path
- `request_changes` loopbacks must point to a reachable earlier node
- the consumed `schema_ref` must match the producer's declared `schema_ref`
- every `interactive_agent` and `agent_task` node must have a resolvable agent source
- every `system_action` must declare its required input artifacts
- every template must declare one terminal output contract

On flow start:

- flow input contract must be satisfied
- agent references must resolve after applying fallbacks
- resolved agents must support the node's required invocation mode
- target defaults may fill gaps only when explicitly allowed by the template

At runtime:

- a node should not start if its required artifacts are absent
- retries should create a new artifact version for the same artifact key
- the latest successful artifact version becomes the active one for downstream nodes

### Recommended implementation phases

#### Phase 1: add contracts without changing runtime behavior

- add `input_contract` and `output_contract` to `flow_templates`
- add `consumes_contract` and `produces_contract` to `flow_template_nodes`
- add schema validation in template CRUD
- keep current execution path intact

#### Phase 2: add artifact persistence

- add `flow_run_artifacts`
- when a node completes, write declared produced artifacts into the artifact table
- continue mirroring raw output into `flow_node_runs.output`

#### Phase 3: adapt current built-ins

- `interactive_agent` adapter maps planning session output into declared artifacts
- `agent_task` adapter maps `agent_run.output_summary` into declared artifacts using `output_tag` only as a transition aid
- `approval_gate` adapter writes standard `approval_decision`, `review_feedback`, and override artifacts
- `system_action` input builders switch from direct node-run parsing to artifact lookup

#### Phase 4: make contracts authoritative

- new templates must declare contracts
- template editor shows unsatisfied dependencies before save
- flow start UI renders input form from `flow_input_contract`
- `legacy_node_output.*` bindings are deprecated and later removed

#### Phase 5: support future node types cleanly

Once the artifact contract layer exists, new node types can plug in without inventing one-off wiring. Examples:

- `branch_gate`: consumes multiple approval artifacts and picks a branch
- `fork_join`: waits for a set of artifact keys before continuing
- `webhook_action`: consumes a typed artifact and posts externally
- `human_task`: produces a typed human-supplied artifact
- `transform`: converts one schema into another without invoking an agent

Those future nodes stay generic because they only need:

- a runtime adapter for their node type
- declared `consumes`
- declared `produces`

They do not need bespoke knowledge of any specific predecessor node.

## 6. Tool model

Tools live in the worker tool registry.

They are the capabilities an agent/runtime is allowed to use while executing.

Current tool families:

- Code/filesystem
  `read_file`, `write_file`, `list_directory`, `search_files`, `read_file_range`, `ripgrep`, `grep`, `list_symbols`
- Commands
  `run_command`
- Git/delivery
  `create_branch`, `commit_and_push`, `open_pr`
- PM
  `add_story_comment`, `update_story_state`, `list_story_checklist`
- Support
  `list_conversation_messages`, `draft_support_reply`, `update_conversation_status`
- CRM
  `list_deals`, `update_deal_stage`, `add_deal_note`, `list_contacts`, `list_buyer_signals`
- Docs
  `list_documents`, `read_document`, `search_documents`
- Research
  `web_search`

How tool access is decided:

1. start from the class runtime profile
2. apply per-agent overrides from `allowed_tools`
3. for some flows, narrow the tool set again at node-launch time
4. pass the final allowed tool set into the runtime execution context

So the final tool boundary is:

- **class defaults**
- then **agent overrides**
- then sometimes **flow-node narrowing**

## 7. Where things are configured in the product

| Thing | Main UI surface | Backing concept |
| --- | --- | --- |
| Agents | `PM > Agents` | `agents` |
| Workflow-stage pipeline | `Settings > Teams > Workflow` / workflow manager | `automation_rules` |
| PM built-in automations | `Settings > Automations` | `automation_rules` (migrated from `pm_automations`) |
| Flow start/run inspection | `PM > Flows` | `flow_runs`, `flow_node_runs` |
| Runner health | project delivery / runner health views | Temporal queues + active `agent_runs` |
| Support AI auto-reply | `Settings > Chat AI` | support settings + support agent run trigger |
| CRM intelligence | CRM email/settings surfaces | Temporal CRM workflows |
| Workspace planning methodology / web research | workspace settings | planning input resolution |

## 8. Triggers: what starts what today

```mermaid
flowchart LR
    subgraph TRIGGERS["What starts what"]
        direction TB
        S1["Story state change"] -->|"rule"| AGENT_RUN["AgentRun"]
        S1 -->|"rule"| COMMANDS["run_command<br/><i>epic auto-start/complete,<br/>merge_branch, etc.</i>"]
        S2["Agent approved"] -->|"rule"| ACTIONS["move_to_state / merge_branch"]
        S3["Manual UI action"] --> AGENT_RUN
        S4["Agent assigned to story"] --> AGENT_RUN
        S5["Cron schedule"] --> AGENT_RUN
        S5C["Cron rule"] -->|"run_command"| COMMANDS
        S6["Support widget"] --> AGENT_RUN
        S7["Flow agent_task node"] --> AGENT_RUN
        S8["Manual from Flows UI"] --> FLOW_RUN["FlowRun"]
    end

    style AGENT_RUN fill:#f4e8e8,stroke:#a66
    style FLOW_RUN fill:#e8e8f4,stroke:#66a
    style ACTIONS fill:#e8f4e8,stroke:#4a9
    style COMMANDS fill:#f4f4e8,stroke:#aa6
```

## 8.1 What starts automation rules

Current active triggers:

- story creation enters an initial state
- story moved to a new workflow state (via Create, Update, or MoveToState)
- agent run approved
- cron category tick (e.g. `sprint_hourly` — evaluated by `EvaluateCronRules`)

These are evaluated inline in the PM story/agent services. Cron rules are loaded across all workspaces by `ListEnabledCronRules`.

## 8.2 What starts agent runs

Current active start paths:

- manual run from the UI/API
- assignment of an agent to a story
- story state change when a story already has an assigned agent
- `run_agent` automation-rule action
- support widget AI auto-reply path for support conversations
- cron schedule for scheduled agents
- flow `agent_task` nodes

## 8.3 What starts flows

Current active start path:

- manual start from the PM Flows UI / flow API

Important current-state note:

- flow templates declare supported triggers such as `manual`, `internal_domain_hook`, `story.state_entered`, and `cron`
- the rule engine defines a `start_flow` action type but it is not yet wired to a dispatcher
- the `flow_triggers` table is scheduled for removal (migration 054); when flow auto-start is needed, the rule engine's `start_flow` action will be the entry point

So today, durable `FlowRun` execution is real, but generalized auto-triggered flow launching is not yet wired.

## 9. Where everything runs

This is the question that causes the most confusion.

```mermaid
flowchart TB
    subgraph API["API Server (inline)"]
        EPIC_AUTO["Epic auto-start/complete<br/><i>(legacy path, transitioning to rules)</i>"]
        RULE_EVAL["AutomationRuleEngine"]
    end

    subgraph TEMPORAL["Temporal Workers"]
        subgraph QUEUES["Task Queues"]
            Q1["agent-engineer"]
            Q2["agent-planner"]
            Q3["agent-reviewer"]
            Q4["agent-support"]
            Q5["automation-default"]
            Q6["planning-interactive"]
            Q7["flow-orchestrator"]
        end
        ARW["AgentRunWorkflow"]
        FRW["FlowRunWorkflow"]
        SPRINT_WF["SprintAutomationCronWorkflow<br/><i>hourly cron</i>"]
        CRM_WF["CRM Workflows<br/><i>EmailSync, SignalDetection,<br/>DealManagement</i>"]
    end

    RULE_EVAL -->|"launches"| ARW
    RULE_EVAL -->|"run_command"| CMD["InternalCommandService"]
    API -->|"starts"| FRW
    FRW -->|"child"| ARW

    ARW --> RT{"Runtime"}
    RT -->|"opencode"| OC["External subprocess"]
    RT -->|"native_sdk"| NS["In-process tool registry"]

    style API fill:#e8f4e8,stroke:#4a9
    style TEMPORAL fill:#e8e8f4,stroke:#66a
    style QUEUES fill:#f4f4e8,stroke:#aa6
    style CMD fill:#f4f4e8,stroke:#aa6
```

## 9.1 Are flows always run in Temporal?

**Durable `FlowRun` flows: yes.**

If a `FlowRun` is started, the orchestration loop is Temporal-backed:

- `FlowService.StartRun` creates the DB record
- `RunEngine.StartFlowRun` starts `FlowRunWorkflow`
- the workflow runs on the `flow-orchestrator` queue

But:

- not every automation in the product is a `FlowRun`
- PM built-ins are not `FlowRun`s
- automation rules are not `FlowRun`s
- CRM background automations are Temporal-backed, but they are their own workflows, not `FlowRun`s

So the accurate answer is:

- **FlowRun-based flows are always Temporal**
- **the broader automation platform is not always a FlowRun**

## 9.2 Where are agent runs executed?

Executable agent runs are Temporal-backed.

The path is:

1. create `agent_runs` row
2. start `AgentRunWorkflow`
3. Temporal worker picks it up from a task queue
4. worker prepares workspace/repo state
5. worker dispatches to the runtime adapter
6. runtime uses tools and writes artifacts
7. run completes or waits for approval

### Shared Temporal queues

Current shared queues:

- `agent-engineer`
- `agent-planner`
- `agent-reviewer`
- `agent-support`
- `automation-default`
- `planning-interactive`
- `flow-orchestrator`

Queue selection is based on resolved profile, with cross-module agents falling back to `automation-default`.

## 9.3 Where does the actual model/tool loop run?

Inside the **Temporal worker process**, not the API server.

Current execution styles:

- `opencode`
  external runtime/subprocess in the worker workspace
- `native_sdk`
  in-process runtime in the worker using the tool registry

Planning sessions also run in Temporal workers, with a Temporal session used to keep the same cloned repo across message turns.

## 9.4 What still runs inline in the API server?

These are not offloaded into `FlowRun` orchestration:

- automation-rule evaluation (including epic auto-start/complete via `run_command` rules)
- legacy `OnStoryStateChange` epic path (kept during transition, will be removed after migration validation)
- some settings/inventory/health assembly

Inline code can still launch Temporal work after making a decision.

## 10. Current data model

The most important records are:

```mermaid
erDiagram
    agents ||--o{ agent_runs : "has many"
    agent_runs ||--o{ agent_run_artifacts : "produces"
    agents ||--o{ agent_handoffs : "handed off via"

    flow_runs ||--o{ flow_node_runs : "contains"
    flow_node_runs ||--o| agent_runs : "may create"
    flow_node_runs ||--o| planning_sessions : "may create"

    planning_sessions ||--o{ planning_session_messages : "has many"

    automation_rules }o--|| agents : "may reference"

    agents {
        uuid id PK
        string class
        string kind
        string runtime_kind
    }
    agent_runs {
        uuid id PK
        string status
        uuid agent_id FK
    }
    flow_runs {
        uuid id PK
        string template
        string status
    }
    flow_node_runs {
        uuid id PK
        string node_type
        uuid flow_run_id FK
    }
    automation_rules {
        uuid id PK
        string trigger_type
        string action_type
    }
```

### Agent execution

- `agents`
- `agent_runs`
- `agent_run_artifacts`
- `agent_handoffs`

### Durable flows

- `flow_runs`
- `flow_node_runs`

### Interactive planning

- `planning_sessions`
- `planning_session_messages`

### Rules and built-ins

- `automation_rules` — unified rule table for all trigger→action rules (including migrated epic/sprint automations)
- `pm_automations` — legacy table, data migrated to `automation_rules` via migration 052; will be dropped after validation
- `automation_health`

## 11. The main "what should I use?" guide

Use this when deciding how a new automation should fit into the platform:

### Use a built-in automation when

- the behavior is product-owned
- the rules are fixed by Teampulse
- it is not meant to be configured as an open-ended user workflow

### Use an automation rule when

- the trigger is a simple event like story-state entry or agent approval
- the action is straightforward
- the user should configure it from workflow settings

### Use an agent when

- work should be done by an explicit actor
- you need runs, approvals, artifacts, or handoffs
- you need tool access and auditability

### Use a flow when

- the work has multiple steps
- it needs human checkpoints
- it needs retries/loopbacks per step
- it may mix sessions, agent runs, and deterministic commands

## 12. What is fully implemented vs partly prepared

### Fully active today

- agent CRUD
- agent runs through Temporal
- runtime profiles and queue routing
- per-agent tool/command/target overrides
- runner health reporting
- workflow-stage automation rules (with `run_command`, `cron`, and `state_type` matching)
- durable flow runs and node runs
- interactive planning session child workflows
- story completion flow
- CRM deal review flow
- epic planning flows
- schedule-based Temporal cron launcher for agents
- sprint automation via Temporal cron workflow (`SprintAutomationCronWorkflow`)
- unified command dispatch via `InternalCommandService` (used by both rules and flows)
- epic auto-start/complete via `automation_rules` `run_command` actions

### Present in schema/model but not yet broadly wired as a general platform

- `start_flow` rule action type (defined, dispatcher not wired)
- generalized `auto_on_event` agent execution
- general use of `trigger_events`
- general use of `target_selector`

## 13. Short answer summary

If you only remember one thing, remember this:

- **Automation rules** are the single trigger → action system for PM pipelines, epic lifecycle, and sprint scheduling
- **Rules dispatch through `InternalCommandService`** — one command registry shared by rules and flows
- **Agents** are executable actors with runs, approvals, tools, and artifacts
- **Flows** are durable multi-step orchestrations made of nodes
- **All scheduled work uses Temporal** — sprint automation, CRM sync, deal management, scheduled agents
- **Agent runs and flow runs are Temporal-backed**
- **Not all automation is a flow**
- **The API server evaluates rules inline, then launches Temporal work when needed**

## 14. Simplification Roadmap

This section tracks the automation unification effort. Completed items are marked; remaining work follows.

### Completed

**Sprint ticker → Temporal cron** (was 14.1)
Sprint auto-create and move-unfinished now run via `SprintAutomationCronWorkflow` on the `automation-default` queue. The hourly `time.Ticker` goroutine has been removed from `cmd/api/main.go`. Sweep logic in `pm_automation.go` is unchanged.

**Unified command dispatch** (was 14.2)
`AutomationRuleEngine` now dispatches `run_command` actions through `InternalCommandService.Execute()`. New commands (`pm.auto_start_epic`, `pm.auto_complete_epic`, `pm.sprint_auto_create`, `pm.sprint_move_unfinished`, `delivery.merge_branch`) are registered alongside existing flow commands. Both rule actions and flow `system_action` nodes share the same dispatch layer.

**Generalized rule engine** (was 14.4)
`AutomationRuleEngine` now supports:
- `cron` trigger type with category-based matching
- `run_command` action type dispatching to `InternalCommandService`
- `start_flow` action type (defined, not yet wired)
- `state_type` matching on `story.state_entered` triggers (e.g. match any `"started"` or `"done"` state)
- Non-story event context via `TargetType`, `TargetID`, `TeamID` fields on `AutomationEvent`
- Conditional story loading — cron triggers skip the story lookup entirely

**pm_automations → automation_rules migration** (was 14.4)
Migration 052 backfills all four `pm_automations` types into `automation_rules`:
- Epic auto-start → `story.state_entered` (state_type=started) + `run_command` (pm.auto_start_epic)
- Epic auto-complete → `story.state_entered` (state_type=done) + `run_command` (pm.auto_complete_epic)
- Sprint auto-create → `cron` (sprint_hourly) + `run_command` (pm.sprint_auto_create)
- Sprint move-unfinished → `cron` (sprint_hourly) + `run_command` (pm.sprint_move_unfinished)

The legacy `OnStoryStateChange` path is kept during transition for safe deploy. Both paths fire but epic mutations are idempotent.

### Remaining: cleanup after production validation

**Drop `pm_automations` table** — Migration 053 is ready. Run after verifying all workspaces have corresponding `automation_rules` rows and the legacy `OnStoryStateChange` calls can be removed from `pm_story.go`.

**Drop `flow_triggers` table** — Migration 054 is ready. Verify with `SELECT count(*) FROM flow_triggers` in prod first. When flow auto-start is needed, use the rule engine's `start_flow` action type.

**Remove legacy code** — After dropping `pm_automations`: remove `model/pm_automation.go`, `repository/pm_automation.go`, the `automationService` field from `PMStoryService`, and the `OnStoryStateChange` method.

**Update seeding and settings surfaces** — `SeedWorkspaceDefaults` still writes to `pm_automations`. The PM automation handler and inventory service still read from it. These should be migrated to read/write `automation_rules` directly.

### Not yet actionable

**Simple flows (story completion, deal review)** — These remain as flows. Demoting them to rule chains would require:
1. Rich approval behavior (`request_changes`, `reject`) in the rule/agent system
2. Non-story approval event triggers
3. Override/payload passing in rule chains
4. Equivalent manual-launch and review UX

### Architecture (current state)

```mermaid
graph TD
    subgraph "Trigger Layer"
        EV["Events<br/><i>story.state_entered, agent_run.approved,<br/>cron</i>"]
    end

    subgraph "Rule Layer (unified)"
        RE["AutomationRuleEngine<br/><i>single evaluation path for all<br/>trigger → action rules</i>"]
    end

    subgraph "Command Layer"
        CR["InternalCommandService<br/><i>shared dispatch for rules and flows:<br/>epic auto-start/complete, sprint create,<br/>merge_branch, create_followups, etc.</i>"]
    end

    subgraph "Execution Layer"
        AG["Agent Runs<br/><i>Temporal-backed</i>"]
        FL["Flow Runs<br/><i>multi-step orchestrations</i>"]
    end

    subgraph "Temporal Scheduled"
        SPRINT["Sprint Automation<br/><i>hourly cron</i>"]
        CRM["CRM Intelligence<br/><i>email sync, signals, deals</i>"]
        SCHED["Scheduled Agents<br/><i>cron launcher</i>"]
    end

    EV --> RE
    RE -->|"run_agent"| AG
    RE -->|"run_command"| CR
    RE -->|"start_flow<br/>(future)"| FL
    FL -->|"system_action nodes"| CR
    AG -->|"completion triggers"| EV

    style EV fill:#e8f4e8,stroke:#4a9
    style RE fill:#e8e8f4,stroke:#66a
    style CR fill:#f4f4e8,stroke:#aa6
    style AG fill:#f4e8e8,stroke:#a66
    style FL fill:#f4e8e8,stroke:#a66
    style SPRINT fill:#e8e8f4,stroke:#66a
    style CRM fill:#e8e8f4,stroke:#66a
    style SCHED fill:#e8e8f4,stroke:#66a
```
