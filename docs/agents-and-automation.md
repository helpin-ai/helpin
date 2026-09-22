# Agents and automation

This guide helps backend contributors understand how Helpin starts and tracks
agent work. It explains which records and decisions belong to Helpin and which
execution responsibilities belong to Agent Runtime. It covers:

- automation flows and rules
- trigger executions
- agent records
- agent runs
- built-in automations

For the product-facing explanation of flows and agents, see
[how automation works](automation-product-model.md).

For repository work, see [coding-agent execution](coding-agent-execution.md).
PM and CRM targets require those modules; Community enables every module by
default.

**Source review:** 2026-09-18. The launch and policy references below describe this checkout. Agent Runtime is a separate component; its deployed adapters, browser lifecycle, and external service configuration were not exercised by this review.

## Architectural summary

Helpin has one agent abstraction and one execution boundary.

- Helpin owns agent definitions, workspace and team access, presets, versions,
  triggers, targets, entitlements, billing preflight, and product launch
  surfaces.
- Agent Runtime owns execution mechanics: durable workflows, runtime adapters,
  tool invocation, workspaces, transcripts, interactions, artifacts, and usage.
- `is_system` describes who owns an agent's configuration. It does not select
  an executor.
- `runtime_kind` records the execution adapter. New Helpin runs currently require
  `native_sdk`; `codex` and `opencode` survive in historical/configuration contracts.
  It does not select product behavior or ownership.
- Every real agent execution creates the normal Helpin `agent_run`, is projected
  to Agent Runtime, and returns through the same event/projection lifecycle.

In one sentence: **system agents are product-managed configurations of the
generic agent primitive, while custom agents are workspace-managed
configurations of that same primitive.**

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

Examples of trigger families in code (not an exhaustive catalog):

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
  - `automation_rule.cron`
- built-in or product-owned launchers
  - `support.widget_message`
  - `task.assigned_agent_state_change`

Canonical trigger and binding identity lives in [the trigger catalog](../server/internal/automationcatalog/triggers.go). It also includes GitLab events, document publication, agent completion, AI-section events, and additional manual target bindings.

## Built-in automations vs flows

There are two important kinds of automation behavior:

### Built-in automations

Product-owned backend behavior implemented directly in services, repositories, or Temporal workflows.

Examples:

- CRM signal ingestion
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
- `approval_mode`
- `default_invocation_mode`

The product surfaces are:

- `Agents` for configuration and ownership
- `Runs` for execution details
- `Activity` for automation-layer history involving those agents

## Executor architecture

The current execution architecture is intentionally generic:

```text
agent record
  -> agent_run
  -> Helpin runtime launch preparation
  -> Agent Runtime StartRun
  -> native_sdk execution adapter
  -> messages, tool calls, interactions, artifacts
```

`runtime_kind` chooses the backend adapter. It does not choose a product-specific
agent type. System agents and custom agents both create normal `agent_run` records
and both flow through the same workflow, interaction, artifact, and transcript
surfaces.

Execution mechanics belong to Agent Runtime. Helpin's [launch policy](../server/internal/service/agent_policy.go) rejects new non-`native_sdk` runs, and [preset policy](../server/internal/service/agent_presets.go) allows only that runtime for current presets. Historical Codex/OpenCode adapter descriptions are not a promise that those runs can be started from this checkout.

Product behavior is expressed through agent configuration:

- prompt / preset preamble
- skills
- allowed tools
- allowed targets
- invocation mode
- approval mode
- trigger or flow configuration

There is no separate native planner controller. Planner behavior is a preset plus
skills plus Helpin MCP tools. Planner contracts remain separate from runtime selection; current launch validation permits `native_sdk` only.

## Tool contract

Helpin product and interaction tools are exposed to model backends through the
Helpin MCP provider. Model-facing names are bare canonical aliases:

```text
update_plan
request_user_input
request_approval
publish_prd_draft
publish_task_plan
publish_task_plan_doc
list_tasks
```

The same canonical names are used for policy, validation, artifact application,
storage, and model definitions. Runtime code still strips the legacy
`mcp__helpin__` prefix when reading stored or in-flight compatibility data.

Repo-local execution tools such as file reads, patching, and shell commands may
still be provided directly by a backend where appropriate. Helpin product tools
go through MCP; runtime-local tools are provided by Agent Runtime.

Authenticated browser automation is also runtime-owned. The canonical browser
bundle is `browser_open`, `browser_snapshot`, `browser_act`, and
`browser_screenshot`, plus `browser_record` for bounded recordings. The host-side [browser contracts](../server/internal/agentcontract/browser_tool_catalog.json) describe these tools. Agent Runtime executes the bounded contracts through the
pinned `agent-browser` CLI and Kernel; Helpin owns selection metadata and the
durable S3-backed screenshot asset endpoint. Each app/run receives an isolated,
short-lived browser session without a persistent Kernel profile. Paused turns
retain the session; the runtime closes it when the run becomes terminal, and
Kernel's configured idle timeout is the fallback. Login state does not survive
a closed or timed-out session.

Use web search, `fetch_url`, or `crawl_url` for public read-only research. Use
the browser bundle only for authenticated pages, UI interactions, or visual
capture. Browser output is compact and screenshots return asset metadata rather
than inline image bytes.

When writing prompts, skill markdown, planner guidance, backend tests, policies,
or artifact decoders, use the bare canonical tool name, for example
`update_plan`. Use the prefixed form only in tests for legacy compatibility.

## The Dock (Ask Agents)

The dock is the primary chat surface over the agent system. Each user has
multiple dock chats per workspace; **each chat is an agent-runtime chat-mode
run** of the `ask_agent` system preset (`turn_policy.mode =
pause_after_assistant`, `turn_policy.completion_mode = explicit_finish`). There
is no in-app classifier or inline LLM tool loop anymore — the chat run itself
answers read-only questions with product command tools and orchestrates durable
work through the `agents.*` command tools.

Current reference: [agent-dock.md](agent-dock.md).

Important boundaries:

- the chat agent's tool surface is read-only product tools + `agents.*`
  orchestration tools, narrowed per user at run start
- Agent Runtime owns the `finish_turn` control tool. A prose-only model stop is
  corrected inside the same turn up to the configured bound; after exhaustion
  the run fails explicitly instead of becoming an `awaiting_user_message`
  pause. Helpin also sends this policy on dock-run resumes so pre-policy paused
  runs upgrade in place. Other chat-mode hosts retain implicit completion unless
  they opt in.
- launch tools accept complete direct requests under the runtime approval policy
  as well as legacy approved launch interactions. Reusable-agent creation and
  other operations retain operation-specific `dock_plan_confirm` checks; inspect
  [the orchestration handlers](../server/internal/service/internal_command_agents.go)
  rather than assuming every mutation requires the same interaction
- durable work still creates `agent_run` records, grouped by
  `command_bar_plans` when orchestration is needed; plans launched from a chat
  carry `parent_chat_run_id` / `dock_chat_id` and deliver a
  `<child_run_result>` back into the chat when they settle
- one-shot ad hoc work still uses the system `Command Agent`
- saved custom agents still persist through the normal `agents` creation path

## Support conversations

Support conversations execute through Agent Runtime chat-mode runs. Helpin owns
message admission, knowledge retrieval, reply validation/publication and human
handoff. `SupportAIService` supplies these collaborators; it is not a separate
in-process answer executor. See [Support execution through Agent Runtime](support-agent-runtime.md)
for the code map and retained composer-assistance boundary.

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
- `crm_contact`
- `crm_company`
- `document`

Important current behavior:

- manual launches can start directly against an explicit target
- rule-driven `start_agent_run` can use either an event-derived target or an explicit fixed target
- GitHub-triggered `start_agent_run` should be treated as fixed-target rules in practice unless the event resolves cleanly to an existing linked task
- `support_coverage_gap` is a product-owned support/docs target used by the Documentation system agent to act on support coverage analysis through the normal `agent_run` path

The Flow composer exposes **CRM → a specific deal / contact / company** using a shared, server-searched record picker. Custom Agent creation/editing and template Agent setup expose all three CRM record types using familiar Deal, Contact, and Company labels. Run now uses the same picker, restricted to the Agent's explicitly saved allowed targets. The picker resolves saved selections independently of the first search page, keeps workspace and record-type caches separate, participates in CRM invalidation, and distinguishes unavailable records from empty search results. Company run history includes the company name and a link to its CRM record.

These options reuse existing generic target launch paths; they do not change saved Flow enablement, Agent defaults, tool access, approvals, AI usage preflight, or create PM work. The draft generator recognizes all three existing CRM record types but still requires user review before creation. Playbook lifecycle coordination and bound execution are implemented separately through the [CRM connection](crm-playbook-automation-change-proposal.md); selecting an ordinary CRM record does not enroll it in a Playbook or authorize Playbook actions. The agreed direction keeps Flows and reuses Beacon with specialized skills; it does not require an ordered-step builder. Do not present a new “Customer work” category or advertise unsupported Signals/Playbooks targets in the builders.

## Shared scheduled product events

`automation_scheduled_events` is a versioned-SQL-owned durable outbox for timed
product events. It is not a user-authored Flow, a second Agent executor, or a
parallel task system. Producers enqueue an immutable workspace-scoped event key,
target, expected revision, and due time in the same transaction as the originating
domain change. Consumers are explicitly registered by kind; unknown kinds fail
closed. No public scheduling API or new builder controls are exposed by this layer.

The stable `automation-scheduled-events` workflow ticks once per minute on
`automation-default`. API and worker startup ensure it independently of saved
Flow schedules. A tick drains a bounded batch; database leases, claim tokens,
retry backoff, and a maximum of five attempts protect delivery across restarts.
Consumers must commit their receipt and local domain effect atomically. Returning
success without acknowledging the event is a retryable failure, not delivery.
External actions require their own durable identity and execution policy; a
scheduled event receipt does not grant permission to launch an Agent or skip
AI preflight, tool authorization, or approvals.

The first registered consumer is `crm.checkpoint_due`, which only reevaluates
current Signal ownership and linked actions. Lifecycle changes atomically revoke
older pending claims, and the consumer rechecks lifecycle, revision, due time, and
lease under the CRM workspace lock. `delivered` means the check was processed,
not that the customer objective succeeded. `cancelled` means event delivery was
revoked, not that an already-started Agent or message was cancelled. Current
checkpoint failures are available to CRM's Needs attention projection. Playbook-specific entry/check consumers and maintenance now use this same worker for bound normal Agent Runtime launch, cancellation and recovery. See the [CRM reference](crm-signals.md) for the checkpoint contract.

## CRM Playbook policy and execution boundary

CRM owns draft/published business policy, enrollment, canonical Signal ownership/lifecycle, explicit human-assessed milestones and outcomes. Publication and accepting new customers are not execution authority. Policy/preview receipts keep their historical `execution_enabled: false` contract; live activation is read separately from Playbook automation settings and Signal bindings.

The three supplied journeys reuse the existing built-in CRM Agent **Beacon** (`crm_operator`). Opt-in captured core/job packages live under `server/skills/crm_playbooks/`; ordinary preset skill discovery and saved Agent settings remain unchanged. Connections freeze complete files and digests, effective saved prompts, host limits, CRM scope and runtime configuration. Runtime preparation never substitutes today's mutable instructions for reviewed bytes.

Guided Setup prepares a disabled, dedicated `crm.playbook.work_due` Flow. It is labelled Playbook managed in Automation and links back to CRM Setup; generic editing, enabling, deletion and direct launch cannot bypass its policy. Ordinary CRM/non-CRM Flows retain the existing When / If / Then / Using / On behavior. Beacon has a read-only Used by Playbooks section; no new saved Agent or ordered-step editor is created.

Explicit settings activate a reviewed connection for manual starts or fresh automatic eligibility. Automatic entry uses qualified new source evidence or a newly confirmed won-deal transition, never a historical enrollment scan. Source creation and wake-up are transactional; overlapping policies and missing role owners require review. A Signal retains its pinned policy/connection through later publications and resumes against that binding.

The shared scheduled-event worker handles entry/check claims and bounded maintenance. Each dispatch reserves a normal `agent_run` identity, rechecks permissions/billing/limits and registers a deterministic private frozen runtime profile. Existing Beacon remains the Helpin identity for access, usage and Activity. A lost start response is recovered by exact host/app/profile/target correlation, never another StartRun.

Bound target, command and skill/package callbacks require durable run binding, live tenant/target/generation/member/module authority and the captured bytes. Metadata only constrains; it cannot authorize. Tools are restricted to current Playbook context, typed action proposal and finish. Generic launch/resume/approval/continuation paths reject reserved Playbook context and do not act as CRM approval.

Typed email, PM task, handoff, milestone and deal-stage actions use the existing CRM suggestion lifecycle plus immutable intent/approval correlation. Exact payload, approver, revision, expiry, current facts and destination are checked before existing owning-module services execute. PM is optional and uses a real accessible team/assignee, with native CRM task links; no dummy task starts Beacon. The designated Success owner must explicitly accept a handoff.

Pending approval, unresolved results and linked work use cheap checks. Material changes fence stale runs/proposals; paused/closed Signals revoke future work. Caps, escalation routing, stop conditions and visible blockers preserve follow-through. Unknown sends/creates are inspected, not blindly retried. Customer milestones/outcomes never derive from run completion.

Playbooks Activity projects existing connection/gate/run receipts without raw prompts, packages or runtime output. Canonical decisions, human inspections and customer progress stay in CRM. Independently configured ordinary automations and external senders are not silently migrated into these controls.

See the [CRM reference](crm-signals.md) for endpoints and the [connection plan](crm-playbook-automation-change-proposal.md) for verification and deployment boundaries. These capabilities are implemented in this checkout; no production migration, activation or customer send is implied.

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

automation-rule schedule
  -> scheduled trigger
  -> agent_trigger_execution(source=schedule)
  -> agent_run

built-in automation
  -> product-owned service/workflow logic
  -> may create trigger execution and/or agent_run depending on behavior
```

## Agent ownership and configuration

The code records two ownership styles. They share the same execution path.

| Concern | System agent | Custom agent | Owner |
| --- | --- | --- | --- |
| Identity | `is_system = true` | `is_system = false` | Helpin |
| Scope | Managed per preset and workspace | Created inside one workspace | Helpin |
| Behavior source | Product preset or workspace version of that preset | Active custom-agent version | Helpin |
| Prompt and skills | Preset-owned defaults and guardrails | Explicit workspace configuration | Helpin |
| Tools and targets | Preset policy, optionally workspace-versioned | Explicit allowlists | Helpin |
| Runtime backend | `runtime_kind` | `runtime_kind` | Agent Runtime |
| Durable execution | `agent_run` | `agent_run` | Shared |
| Transcript, interactions, artifacts, usage | Generic runtime contracts | Generic runtime contracts | Agent Runtime, projected into Helpin |

### System agents

Product-owned, usually preset-backed agents.

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
- usually preset-bound
- backend-owned defaults, prompt/skill bundles, allowed tools, and guardrails
- may have product-owned launch buttons, default targets, or seed behavior
- are reconciled by preset family so product defaults can be upgraded
- may be pinned to a workspace-owned preset version while remaining system
  agents

Preset contract behavior:

- presets are configuration contracts, not separate execution paths
- `epic_planner` on epic targets and `task_planner` on task targets use planner tool/artifact contracts without introducing a separate planner executor
- phase guidance, active skill contracts, repair instructions, and active skill policy are prompt/tool-contract inputs to a generic run, not a separate planner controller
- canonical mutations such as approved PRD persistence, task creation, task-plan-doc persistence, and replay protection remain backend-owned and runtime-neutral
- backend-specific code should only handle execution mechanics such as model-loop and provider configuration

### Custom agents

Workspace-created agents.

Current truth:

- `is_system = false`
- not preset-backed
- generic executor model
- not on a special custom-agent execution path
- scoped to a Helpin workspace and optionally limited to teams
- carry an active, durable `agent_version` containing runtime, model, prompt,
  skills, tools, targets, and invocation configuration

Simplified creation behavior:

- draft generation and the custom-agent creation UI are helpers for producing a normal custom agent record
- created custom agents still use the existing `/automation/agents` persistence path
- execution still creates normal `agent_run` records
- no separate custom-agent runtime, trigger model, or persistence model exists
- custom-agent creation rejects `preset_key` and `preset_version_key`; a custom
  agent can reproduce planner/reviewer-like behavior by explicitly selecting
  the corresponding prompt, skills, tools, targets, and contracts, but it does
  not inherit or track the product preset

### Launch and registration boundary

Helpin is the source of truth for both ownership styles. Agent Runtime is
host-neutral and does not decide which agents should exist in a workspace.

At launch, Helpin:

1. resolves the workspace agent and its active configuration;
2. checks access, target policy, entitlements, and AI usage;
3. creates the Helpin `agent_run`;
4. projects and upserts the executable agent definition into Agent Runtime;
5. starts the runtime run and stores its external runtime ID;
6. projects runtime messages, interactions, artifacts, status, and usage back
   into the Helpin run.

Agent Runtime partitions its own durable records by host `app_id`. Helpin's
`workspace_id` and team rules remain host-owned tenancy and authorization
boundaries. Do not move workspace ownership decisions into Agent Runtime.

### What shared execution does not imply

Sharing the executor does not mean every agent automatically has identical
capabilities or entry points.

- A tool must be both registered for the Helpin app and allowed by the agent.
- A run-level tool policy may narrow, but never expand, the agent policy.
- Target context and repository workspaces depend on host-provided target and
  workspace contracts.
- System presets may have product-owned buttons, trigger defaults, artifact
  requirements, and finalizers.
- Genuine product exceptions, such as support ingestion or delivery-pipeline
  orchestration, may wrap the generic run path. They must not introduce a
  second executor for custom agents.

Direction:

- keep custom execution generic
- prefer minimal trigger payloads
- let agents gather additional context through tools instead of bespoke backend orchestration
- if custom agents need planner-like behavior, configure it through prompt, skills, and allowed MCP tools instead of runtime branching

### Agent team access

Agents can be limited to one or more teams through `team_ids`.

Current intended semantics:

- ordinary members need membership in one of a team-scoped agent’s teams; workspace owners and admins bypass that team visibility/use restriction
- workspace-scoped agents have no `team_ids` and are available across the workspace subject to normal permissions
- team access is an agent access boundary, not a new agent category
- direct run, update, and delete paths should enforce the same team boundary as list and create/update UI paths

Target interaction:

- team-scoped agents should only run against targets that resolve to an allowed team
- workspace-level targets such as `support_coverage_gap` should generally be handled by workspace-scoped agents or product-owned system agents unless explicit team semantics are added

## Runtime kinds

New runs currently use `native_sdk`. Legacy `codex` and `opencode` values and queue mappings remain in compatibility contracts, but they do not establish available launch options.

The historical `agent-native-*`, `agent-codex-*`, and `agent-opencode-*` queue names in [contract resolution](../server/internal/agentcontract/resolve.go) are not instructions to deploy local agent workers. Helpin calls the separate Agent Runtime service. Non-agent scheduled product work still uses Temporal's `automation-default` queue through [temporalapp](../server/internal/temporalapp/scheduled_events_workflow.go).

Runtime differences should remain below the generic executor boundary. A new
feature should not add one path for "system agents" and another for "custom
agents" unless it is a true product-owned exception such as support ingestion.
Prefer adding a tool, skill, prompt rule, or target contract that both backends
can consume.

## Main architectural rules

- treat `Flow` as the primary product abstraction
- treat `automation_rule` as a persistence detail
- keep `Trigger Execution` and `Agent Run` as distinct records
- built-in automations and user-authored flows should share the same ecosystem, but not be modeled as the same object
- frontend should render backend-owned normalized trigger metadata instead of rebuilding trigger meaning locally
- treat `agent_run` as the only durable execution primitive
- treat `runtime_kind` as backend selection, not product behavior selection
- express planner/review/support behavior through prompts, skills, tools, targets, and artifact contracts
- expose Helpin product tools to model backends through MCP with bare canonical names
- keep canonical backend tool aliases prefix-free for validation, policy, and persistence

## Current known limitations

- some legacy PM and settings routes still exist as redirects or compatibility aliases
- some backend package and model names still use older PM-era terminology
- some product-owned automations still have domain-specific orchestration paths, especially in support
- some historical design docs still describe retired local agent workflows or legacy prefixed tool names; treat this file and [internal tools](internal-tools-framework.md) as the current contract

Those do not change the current product direction:

- `Flows` is the main configuration surface
- `Activity` is the main operational surface
- `Agents` manages executors
- `Library` is reference only


### Repository preview runs

Task and manual run launch requests accept `delivery_mode: "preview"` or
`"publish"` (default). The task Delivery panel and Run agent now dialog expose
this as Preview changes / Publish changes. The saved run input is authoritative;
resumes preserve it, continuations and server-created child runs inherit preview,
and an active run cannot be silently reused with a different mode.

Preview disables runtime repository finalization, `commit_and_push`, `open_pr`,
and Helpin pull-request delivery. Direct Git commit/push and config/alias command
forms are rejected by `run_command`. The checkout is retained with manual cleanup
for inspection; operators must clean it up explicitly. Preview does not disable
other host-app tool effects and does not provide a network or OS sandbox.
Automatic coding handoff is hidden for previews because each run has a separate
checkout; this feature does not transfer local changes to another run.

Runtime-local `run_command` also accepts `working_directory`, an existing
repository-relative directory. Omitting it keeps the workspace-root default.
Absolute paths, traversal outside the workspace, symlinks, files, and unknown
fields are rejected. No host callback or new credential type is involved.
