# Teampulse: Agents And Automation

For the current shipped architecture across built-ins, automation rules, agents, flows, node types, runtimes, queues, and Temporal execution, start with:

- `docs/AUTOMATION_AND_AGENT_ARCHITECTURE.md`

This document remains the source of truth for the current explicit agent model.

For the broader cross-app automation control-plane taxonomy introduced in Phase 1c, see:

- `docs/automation-taxonomy-rfc.md`
- `docs/automation-taxonomy-seed-catalog.md`
- `docs/automation-taxonomy-migration.md`
- `docs/automation-taxonomy-product-ia.md`

## Overview

Teampulse treats agents as opinionated workflow participants inside PM and support.

At the product level, the system now has a clearer split between three things:

- built-in automations
- automation rules
- agents

The intended model is:

```text
                   +----------------------+
                   | Built-in Automation  |
                   |----------------------|
                   | CRM background jobs  |
                   | PM epic rules        |
                   | PM sprint rules      |
                   +----------+-----------+
                              |
                              v
                   +----------------------+
                   | Automation Inventory |
                   |----------------------|
                   | shared catalog       |
                   | shared health model  |
                   | shared settings view |
                   +----------+-----------+
                              |
         +--------------------+--------------------+
         |                                         |
         v                                         v
+-------------------------+             +-------------------------+
|   Automation Rules      |             |         Agents          |
|-------------------------|             |-------------------------|
| trigger -> action       |             | explicit typed actors   |
| workflow-state driven   |             | planner / engineer /    |
| user-configured         |             | reviewer / support      |
| powers PM pipelines     |             | runs / approvals /      |
+-----------+-------------+             | handoffs / artifacts    |
            |                           +-----------+-------------+
            |                                       |
            +-------------- run_agent --------------+
```

Practical interpretation:

- built-in automations are product-owned behaviors that already exist in CRM and PM
- automation rules are user-configured trigger -> action rules
- agents are the executors that perform work and own the run lifecycle

Agents are no longer the taxonomy bucket for "all automation". They remain first-class product entities, but they sit beside the automation-rule system rather than inside it.

Teampulse owns:

- assignments
- run lifecycle
- approvals
- handoffs
- artifacts
- epic planning stages
- story delivery targets
- branch and PR history

Execution happens through shared Temporal worker pools. Temporal owns retries, cancellation, approval waits, and handoff signaling. Shared runners own the tool loop and repo workspace.

The product model is intentionally narrow:

- a small fixed set of user-visible agent classes
- strict target mapping by class
- one workspace-level planning methodology
- hidden planning prompt packs for epic planning

The control-plane model is also intentionally narrow:

- one shared automation inventory surface
- one shared health model for built-ins and rules
- one explicit PM pipeline model based on workflow-state triggers
- one agent execution model backed by Temporal

## Core Model

| Concept | Meaning |
|---|---|
| `agent` | A workspace-scoped LLM executor |
| `agent_class` | The user-visible class that defines intended target surface |
| `agent_run` | A single execution against one target |
| `target_type` / `target_id` | Canonical run target: story, epic, or support ticket |
| `story_delivery_target` | The current repo lane for a story |
| `story_git_link` | Historical branch, commit, and PR output for a story |
| `artifact` | A stored run output such as logs, diffs, or PR metadata |
| `handoff` | Explicit transfer from one agent to another agent or a user/member |

## Current System Boundaries

Teampulse currently has three real automation surfaces:

### 1. Built-in automations

These are product-owned behaviors shipped as part of Teampulse.

Current examples:

- CRM buyer-signal ingestion
- CRM contact/deal summary refresh
- PM epic auto-start
- PM epic auto-complete
- PM sprint auto-create
- PM sprint move unfinished

Characteristics:

- not user-created
- may be system-governed or workspace-governed
- surfaced through shared inventory and health reporting
- may run as background workflows, scheduled rules, or deterministic inline rules

### 2. Automation rules

These are user-configured trigger -> action rules.

Current trigger shapes in active use:

- `story.state_entered`
- `agent_run.approved`

Current action shapes in active use:

- `run_agent`
- `move_to_state`
- `merge_branch`

Characteristics:

- workspace-configured
- event-driven
- powers PM workflow pipelines
- can chain actions with loop prevention

### 3. Agents

Agents remain explicit workspace actors with lifecycle and auditability.

Characteristics:

- typed by class
- routed to strict target types
- executed as `agent_run`
- own approvals, handoffs, artifacts, and execution history

The important boundary is:

- rules decide when work should happen
- agents perform the work
- built-ins are shipped product behavior outside the user-authored rule graph

## Agent Classes

Current user-visible agent classes:

- `product_planner`
- `engineer`
- `reviewer`
- `support`

Intent by class:

- `product_planner`: epic-only PRD/spec/story planning
- `engineer`: story-only implementation and delivery
- `reviewer`: story-only review, testing, and readiness checks
- `support`: support-ticket triage and draft replies

Important:

- `planner` and `orchestrator` are legacy aliases that normalize to `product_planner`
- `reviewer_tester` is a legacy alias that normalizes to `reviewer`

The UI should only expose the canonical classes above.

## Strict Target Mapping

Teampulse now enforces agent-to-target compatibility server-side and client-side.

Allowed mappings:

- `product_planner` -> `epic`
- `engineer` -> `story`
- `reviewer` -> `story`
- `support` -> `support_conversation`

This means:

- a support agent cannot be assigned to a story
- an engineer cannot run on an epic
- a product planner cannot run directly on a support ticket

## Agent Identity

Agents are always LLM executors. Humans are represented as users or workspace members, not as agents.

## Runtime Kinds

Supported schema values:

- `native_claude`
- `claude_code`
- `openclaw`
- `zeroclaw`

Current backend support:

- `native_claude`: implemented
- `claude_code`: implemented through the same shared executor path
- `openclaw`: reserved, not implemented
- `zeroclaw`: reserved, not implemented

## Internal Runtime Profiles

Internally, each agent class maps 1:1 to a runtime profile with queueing and tool policy.

Current canonical runtime profiles:

- `product_planner`
- `engineer`
- `reviewer`
- `support`

Profile intent:

- `product_planner`: read-heavy planning, Docs-first epic planning, no repo mutation
- `engineer`: repo mutation, commit, push, PR creation
- `reviewer`: read-heavy validation and test execution
- `support`: support triage and draft replies with approval boundary

## Trigger Modes

- `manual`
- `auto_on_assignment`
- `auto_on_event`

Current behavior:

- `product_planner` runs are manual
- `support` runs are manual
- `engineer` and `reviewer` can use `manual`, `auto_on_assignment`, or `auto_on_event`
- auto execution only starts when repo requirements are satisfied for the selected profile

In the current PM pipeline design, `auto_on_event` is most useful when combined with automation rules rather than treated as a separate product concept. The pipeline builder now effectively maps workflow states to rule-driven stage behavior.

## Planning Methodology

Epic planning uses a workspace-level planning methodology, configured in `Project Settings > AI`.

Current options:

- `structured_v1` recommended and default
- `basic_v1` fallback/testing option

Rules:

- the setting applies to epic planning only in v1
- the setting affects `draft_spec` and `plan_stories`
- the methodology is stamped into epic planning run input and summary for auditability
- workspace AI settings can also enable external web research for `draft_spec`
- Brave Search is the first supported research provider

### Structured methodology

`structured_v1` is inspired by BMAD, OpenSpec, GitHub Spec Kit, and Taskmaster-style decomposition, but remains Teampulse-native.

Internal behavior:

- `draft_spec` uses an analyst + PM stance
- `plan_stories` uses an architect + scrum-master stance
- a built-in self-check runs inside the prompt contract before final output

Important:

- these are hidden prompt-pack behaviors, not user-visible agent personas
- Teampulse does not expose a BMAD-style catalog of planner personalities

### Planner notes vs system prompt

For `product_planner`, free-form prompt drift is intentionally reduced.

Practical behavior:

- the workspace methodology is authoritative
- `planning_notes` append planner-specific context or preferences
- legacy `system_prompt` values, if present, are treated as secondary advanced notes
- planning agents should not rely on arbitrary prompt overrides to change the workflow contract

## Runtime Model

### Orchestration

- One Temporal workflow per `agent_run`
- Shared task queues by capability:
  - `agent-engineer`
  - `agent-planner`
  - `agent-reviewer`
  - `agent-support`
  - `automation-default`
- Approval and handoff happen through workflow signals

Queue routing:

- `product_planner` -> `agent-planner`
- `engineer` -> `agent-engineer`
- `reviewer` -> `agent-reviewer`
- `support` -> `agent-support`
- everything else -> `automation-default`

### Execution

Each run gets:

- a fresh temp workspace
- a repo clone when the profile requires one
- a capability-profile tool policy
- heartbeat updates written back to `agent_runs`

The current implementation uses shared runners, not one container or pod per run. The legacy `agent_jobs` poller has been removed. Temporal workflows are the only supported execution path.

### Shared execution pattern

Across PM, CRM, and support, the common pattern is:

```text
catalog/config
    ->
inventory/read model
    ->
runtime execution
    ->
health + diagnostics
    ->
settings / operator surface
```

Operationally this means:

- Temporal is the shared async backbone
- the API owns product-facing state transitions and persistence
- worker pools own long-running execution
- WebSocket and event publication keep UI state fresh

## Story Delivery Model

Stories separate planning state from Git delivery state.

Dependency semantics follow a Shortcut-style model:

- `blocks` links are the source of truth for story-to-story sequencing
- story cards treat `blocked` as active-state only
- `blocked_by` remains visible in story detail even after a blocking story is completed
- external non-story blockers remain a separate free-text note
- story names stay flat; Teampulse does not add phase prefixes to titles
- story-to-story authoring happens through a dedicated relationships composer, not title prefixes
- cross-object links stay visible under a shared `Associations` surface, but only story relationships affect workflow state

### Team defaults

Each team can define:

- default repository
- base branch
- branch template
- optional workflow-state mapping for PR-open and PR-merged events

### Story delivery target

Each story can override or inherit:

- repository
- base branch
- working branch
- delivery state
- active PR metadata

### Branch naming

Default branch template:

```text
tp-{display_id}-{slug}
```

## GitHub Integration Model

Teampulse uses GitHub App installations for repo mutation workflows.

Current behavior:

- integrations are stored per workspace
- repository catalogs are synced from the app installation
- worker activities mint short-lived installation tokens
- authenticated remotes are not written into `.git/config`
- webhook processing resolves the workspace from the installation ID and verifies the shared secret

Legacy PAT persistence remains only for migration compatibility and should not be used for new setups.

### GitHub App setup checklist

For a working install flow, configure the GitHub App with:

- `Homepage URL`: the frontend base URL
- `Setup URL`: the backend callback endpoint
- `Callback URL`: the same backend callback endpoint is acceptable
- `Webhook URL`: the backend webhook endpoint

Recommended repository permissions:

- `Contents`: `Read and write`
- `Pull requests`: `Read and write`
- `Metadata`: `Read-only`

Current webhook subscriptions used by Teampulse:

- `Push`
- `Pull request`

## Deployment Model

Shared runners are deployed as queue-specific Temporal worker pools.

Current deployment shape:

- `agent-engineer`: isolated low-concurrency pool
- `agent-planner`: shared planning pool
- `agent-reviewer`: shared validation pool
- `agent-support`: support pool
- `automation-default`: default automation pool

Operational defaults:

- worker pods run as non-root
- root filesystem is read-only
- `/tmp` is mounted as writable scratch space
- capabilities are dropped
- `TEMPORAL_WORKER_QUEUES` can pin a deployment to one or more queues

### Runtime processes

The API server and Temporal worker are separate processes.

Local development usually runs:

- API server: `go run ./cmd/api`
- Temporal worker: `go run ./cmd/temporal-worker`

Optional worker queue pinning:

- `TEMPORAL_WORKER_QUEUES=agent-engineer go run ./cmd/temporal-worker`
- `TEMPORAL_WORKER_QUEUES=agent-planner go run ./cmd/temporal-worker`
- `TEMPORAL_WORKER_QUEUES=agent-reviewer go run ./cmd/temporal-worker`
- `TEMPORAL_WORKER_QUEUES=agent-support go run ./cmd/temporal-worker`
- `TEMPORAL_WORKER_QUEUES=automation-default go run ./cmd/temporal-worker`

## Required Runtime Environment Variables

API:

- `DATABASE_URL`
- `JWT_SECRET`
- `TEMPORAL_ADDRESS`
- `TEMPORAL_NAMESPACE`
- `TEMPORAL_API_KEY` for Temporal Cloud
- `TEMPORAL_TLS_ENABLED`
- `TEMPORAL_TLS_SERVER_NAME`
- `APP_BASE_URL`
- `GITHUB_APP_ID`
- `GITHUB_APP_SLUG`
- `GITHUB_APP_PRIVATE_KEY` as base64-encoded PEM
- `BRAVE_SEARCH_API_KEY` to enable planner web research in workspace AI settings

Worker:

- `DATABASE_URL`
- `JWT_SECRET`
- `TEMPORAL_ADDRESS`
- `TEMPORAL_NAMESPACE`
- `TEMPORAL_API_KEY`
- `TEMPORAL_TLS_ENABLED`
- `TEMPORAL_TLS_SERVER_NAME`
- `GITHUB_APP_ID`
- `GITHUB_APP_PRIVATE_KEY` as base64-encoded PEM
- `ANTHROPIC_API_KEY` for LLM-backed runs
- `BRAVE_SEARCH_API_KEY` to expose the `web_search` tool for `draft_spec`

## Story Run Flow

Practical flow:

1. Create an `engineer` or `reviewer` agent.
2. Create a story.
3. Configure or inherit the story delivery target.
4. Assign the agent.
5. Run the agent, or let `auto_on_assignment` trigger it.

Execution flow:

1. API creates `agent_run`.
2. Temporal workflow starts.
3. prepare activity resolves repo and branch state.
4. execute activity runs the tool loop in a shared runner.
5. artifacts and run status update live.
6. PR and push events update delivery state and history.

## PM Pipeline Model

PM workflows now support a more explicit stage-based automation model.

Conceptually:

```text
story enters workflow state
          |
          v
   automation rule evaluates
          |
          +--> run_agent
          |       |
          |       v
          |   agent run lifecycle
          |
          +--> move_to_state
          |
          +--> merge_branch
```

Practical behavior:

- a workflow stage can have an assigned agent
- entering that stage can automatically run the assigned agent
- approving the resulting run can automatically advance the story to the next state
- entering a stage can also merge the story branch to a configured target branch

This is the first concrete implementation of the stage-based agent pipeline. It is implemented through automation rules, not a separate hidden pipeline engine.

## Epic Planning Flow

Epic planning is docs-first and now has two parallel modes:

- autonomous staged planning through `agent_run`
- interactive planning sessions for collaborative planner/human dialogue

Core rules:

- Docs is the canonical source of truth for product specs
- epics are the planning entrypoint
- the epic planning repo is the live code source for both `draft_spec` and `plan_stories`
- `agent_run` remains the audit trail for autonomous draft and planning stages
- planning sessions persist their own conversation and draft state
- stories are only created after human confirmation
- code execution remains story-scoped

### Planning stages

- `draft_spec`
- `in_session`
- `awaiting_spec_clarification`
- `awaiting_spec_approval`
- `plan_stories`
- `awaiting_plan_approval`
- `stories_created`
- `execution_started` or `ready_for_execution`

### Interactive planning session path

The planner is no longer limited to a single batch draft. Teampulse now also supports a persistent interactive planning session for an epic.

Practical flow:

1. A human starts a planning session on the epic.
2. The planner and human collaborate in a persistent conversation.
3. The planner can ask structured questions progressively instead of dumping all open questions at once.
4. The planner can inspect bounded repo context with read-only tools during the conversation.
5. A live `spec_draft` evolves during the session.
6. Finalization writes the draft into Docs and moves the epic directly to `awaiting_spec_approval`.

Important semantics:

- this is collaborative planning, not autonomous execution
- the conversation is the process; the spec is the artifact
- Temporal manages lifecycle and workspace preparation, not the chat transport itself
- interactive planning is an alternative to batch `draft_spec`, not a replacement for it

### Draft spec stage

Practical flow:

1. Epic planning ensures a `product_spec` doc exists.
2. The product planner drafts spec content from epic metadata, linked docs, linked support tickets, operator notes, bounded live repo context, and optional external web research.
3. The worker writes the result into Docs and snapshots a `DocsVersion` labeled `AI Draft`.
4. The worker extracts `assumptions` and `open_questions` from the draft into persisted epic clarifications.
5. The epic moves to `awaiting_spec_clarification` when clarification items exist, otherwise directly to `awaiting_spec_approval`.

Artifacts:

- `product_spec_draft`
- `external_research_sources` when external citations were used
- normal run logs and conversation artifacts

### Clarify spec stage

Before approval, a human resolves the planner's uncertainty explicitly.

Practical flow:

1. The product planner returns both `assumptions` and `open_questions`.
2. Teampulse stores them as epic clarification items.
3. A human answers open questions and accepts or rejects assumptions in the epic planning UI.
4. Once all clarification items are resolved, the epic moves to `awaiting_spec_approval`.

Important semantics:

- this is the product-owner decision boundary, not a prompt side effect
- rejected assumptions require an explanatory note
- approved planning should not proceed while clarification items remain unresolved

### Spec approval boundary

Approval happens through Docs versioning, not by trusting the latest mutable document state.

Important semantics:

- `approved_spec_version_id` pins the exact version used for later story planning
- new drafts can be generated later without invalidating the previously approved version until a human approves again
- when external research is used, the saved spec includes a normalized `Research Sources` section appended by the runtime
- when clarifications exist, the runtime appends a normalized `Clarifications` section into the spec before creating the approved version

### Story planning stage

Practical flow:

1. A `plan_stories` run loads the approved spec version.
2. The worker clones the epic planning repository and builds an ephemeral bounded code-context summary.
3. The product planner proposes stories with stable refs, dependencies, acceptance criteria, risks, and open questions.
4. Resolved spec clarifications are injected into the planning prompt alongside the approved spec snapshot.
5. Stories are expected to prefer vertical, user-visible slices; enabler stories are exceptions, not the default.
6. The epic moves to `awaiting_plan_approval`.
7. A human confirms the proposal before stories are created.

Artifacts:

- `story_plan_proposal`
- `orchestration_proposal` for backward compatibility

Important runtime rule:

- the code-context summary used for planning is not saved as a Docs artifact, so it cannot become stale canonical documentation
- external web search is not exposed during `plan_stories` in v1

### Planning mode split

The current product split is:

- `draft_spec` and `plan_stories` are autonomous planning runs
- `in_session` is a collaborative planning state
- both paths converge back to Docs approval and later story creation

This keeps planning collaborative where needed without changing the downstream story execution model.

### Confirmation and handoff

On confirmation:

- PM stories are created
- `pm_story_links` are written for dependency edges
- the run stores `created_story_ids`
- reconfirming the same approved run is idempotent and does not create duplicates

### Execution handoff

Execution kickoff starts story-level runs only.

The story-level `additional_context` includes:

- approved spec snapshot
- acceptance criteria
- dependency refs
- source refs

Story dependencies are handed off as explicit links and refs, not as phase labels.

This preserves the planning model even if future execution moves from the current shared API executor to CLI-native runtimes such as `codex_cli` or `claude_cli`.

## Support Run Flow

Support runs are target-based agent runs without story delivery.

Current behavior:

- assignment does not execute anything
- running a support agent creates a support-ticket-targeted `agent_run`
- draft replies are stored in `output_summary`
- approval publishes the draft to the support thread

## Run Status

- `queued`
- `running`
- `awaiting_approval`
- `completed`
- `failed`
- `cancelled`

## Approval Status

- `not_required`
- `pending`
- `approved`
- `rejected`

## Artifacts

Current artifact types:

- `conversation_log`
- `tool_log`
- `diff`
- `test_report`
- `pr_metadata`
- `agent_summary`
- `file_bundle`
- `handoff_note`
- `product_spec_draft`
- `story_plan_proposal`

## Repository Policy

`WORKFLOW.md` in the repo root can still narrow command policy:

```yaml
---
max_iterations: 50
timeout_minutes: 30
allowed_commands: [go, npm, make, git]
handoff_state: human_review
---
```

The worker intersects repository policy with the runtime profile.

## API Reference

### Agents

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/pm/agents?workspace_id=` | List agents |
| POST | `/api/pm/agents?workspace_id=` | Create agent |
| GET | `/api/pm/runtime-profiles?workspace_id=` | List runtime profiles |
| GET | `/api/pm/runner-health?workspace_id=` | Get shared runner queue and active-run health |
| GET | `/api/pm/agents/{id}?workspace_id=` | Get agent |
| PUT | `/api/pm/agents/{id}?workspace_id=` | Update agent |
| DELETE | `/api/pm/agents/{id}?workspace_id=` | Delete agent |

### Story Runs

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/pm/stories/{id}/assign-agent?workspace_id=` | Assign story agent |
| POST | `/api/pm/stories/{id}/run-agent?workspace_id=` | Run story agent |
| GET | `/api/pm/agents/{id}/runs?workspace_id=` | List runs for agent |
| GET | `/api/pm/agent-runs/{id}?workspace_id=` | Get run |
| GET | `/api/pm/agent-runs/{id}/artifacts?workspace_id=` | Get artifacts |
| POST | `/api/pm/agent-runs/{id}/cancel?workspace_id=` | Cancel run |
| POST | `/api/pm/agent-runs/{id}/approve?workspace_id=` | Approve run result |
| POST | `/api/pm/agent-runs/{id}/handoff?workspace_id=` | Record handoff |

### Epic Planning

Compatibility note:

- route names still use historical `orchestrator` wording in a few places
- the product-facing term is now `product planner`

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/pm/epics/{id}/agent-runs?workspace_id=` | List runs for epic planning work |
| POST | `/api/pm/epics/{id}/run-agent?workspace_id=` | Backward-compatible epic planning entrypoint; currently defaults to `draft_spec` |
| POST | `/api/pm/epics/{id}/draft-spec?workspace_id=` | Draft or refresh the product spec |
| POST | `/api/pm/epics/{id}/approve-spec?workspace_id=` | Approve the current or selected spec version |
| POST | `/api/pm/epics/{id}/plan-stories?workspace_id=` | Generate a reviewable story plan |
| POST | `/api/pm/agent-runs/{id}/confirm-orchestration?workspace_id=` | Create stories from a planning proposal and approve the run |
| POST | `/api/pm/epics/{id}/kickoff-execution?workspace_id=` | Start story-level execution for selected stories |
| POST | `/api/pm/epics/{id}/assign-orchestrator?workspace_id=` | Backward-compatible endpoint to assign the epic product planner |

### Story Delivery

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/pm/stories/{id}/delivery-target?workspace_id=` | Get current delivery target |
| PUT | `/api/pm/stories/{id}/delivery-target?workspace_id=` | Update story delivery target |
| GET | `/api/pm/stories/{id}/git-links?workspace_id=` | List historical git links |
| POST | `/api/pm/stories/{id}/create-branch?workspace_id=` | Legacy branch setup endpoint |

### Support

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/support/tickets?workspace_id=` | List tickets |
| POST | `/api/support/tickets?workspace_id=` | Create ticket |
| GET | `/api/support/tickets/{id}?workspace_id=` | Get ticket |
| PUT | `/api/support/tickets/{id}/status?workspace_id=` | Update ticket status |
| GET | `/api/support/tickets/{id}/messages?workspace_id=` | List messages |
| POST | `/api/support/tickets/{id}/messages?workspace_id=` | Add message |
| POST | `/api/support/tickets/{id}/assign-agent?workspace_id=` | Assign support agent |
| POST | `/api/support/tickets/{id}/run-agent?workspace_id=` | Run support agent |
| POST | `/api/support/tickets/{id}/link-story?workspace_id=` | Link ticket to story |

### Git

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/git/integrations?workspace_id=` | List git integrations |
| GET | `/api/git/github/install-url?workspace_id=` | Get GitHub App install URL |
| GET | `/api/git/github/callback` | Public GitHub App install callback |
| POST | `/api/git/integrations?workspace_id=` | Create git integration |
| POST | `/api/git/integrations/{id}/sync?workspace_id=` | Sync repositories from installation |
| GET | `/api/git/repositories?workspace_id=` | List synced repositories |
| PUT | `/api/git/repositories/{id}?workspace_id=` | Update repository catalog selection |
| POST | `/api/git/webhook` | Public webhook endpoint |

### Team Delivery Defaults

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/settings/teams/{id}/repo-default` | Get team repo default |
| PUT | `/api/settings/teams/{id}/repo-default` | Update team repo default |

## Frontend Surfaces

Current UI support:

- Agents page:
  - fixed agent-class templates
  - class-specific trigger mode rules
  - `planning_notes` for `product_planner`
  - `system_prompt` for non-planner LLM classes
- story detail `Delivery` block:
  - agent picker filtered to `engineer` and `reviewer`
  - repository selector
  - base branch
  - branch preview
  - PR status
  - manual run action
- story, epic, and support detail `Associations` panel:
  - Shortcut-style story relationships
  - linked support tickets
  - linked CRM records
  - linked docs
- epic detail planning surface:
  - product planner assignment
  - spec draft
  - plan generation
  - run history
  - proposal review
  - confirmation into story creation
  - execution kickoff
- support page:
  - support-agent picker filtered to `support`
  - manual agent run
  - approval of draft replies
- Project Settings > AI:
  - workspace planning methodology selector

## Automation Rules Engine

Teampulse includes a generic automation rules engine that enables event-driven workflow automation through a trigger → action pattern.

### Data Model

Rules are stored in the `automation_rules` table with:

- `trigger_type` + `trigger_config`: what event fires the rule
- `action_type` + `action_config`: what happens when the rule fires
- `workflow_id`: optional scope to a specific workflow
- `team_id`: optional scope to a specific team
- `position`: execution order when multiple rules match
- `stop_on_match`: prevents later rules from firing
- `enabled`: toggle without deletion

### Trigger Types

| Trigger | Config | Fires when |
|---|---|---|
| `story.state_entered` | `{ state_id }` | A story moves into the specified workflow state |
| `agent_run.approved` | `{ state_id }` | An agent run is approved while the story is in the specified state |

### Action Types

| Action | Config | Effect |
|---|---|---|
| `run_agent` | `{ agent_id }` | Assigns and runs the specified agent on the story |
| `move_to_state` | `{ target_state_id }` | Moves the story to the target workflow state |
| `merge_branch` | `{ target_branch }` | Merges the story's working branch into the target branch |

### Pipeline Pattern

The primary use case is stage-based agent pipelines. Example for a workflow with states Planning → Development → Review → Deploy → Done:

| Rule | Trigger | Action | Effect |
|---|---|---|---|
| 1 | `story.state_entered` (Planning) | `run_agent` (planner) | Planner starts when story enters Planning |
| 2 | `agent_run.approved` (Planning) | `move_to_state` (Development) | Auto-advance after planner approval |
| 3 | `story.state_entered` (Development) | `run_agent` (engineer) | Engineer starts when story enters Development |
| 4 | `agent_run.approved` (Development) | `move_to_state` (Review) | Auto-advance after engineer approval |
| 5 | `story.state_entered` (Review) | `run_agent` (reviewer) | Reviewer starts when story enters Review |
| 6 | `agent_run.approved` (Review) | `move_to_state` (Deploy) | Auto-advance after reviewer approval |
| 7 | `story.state_entered` (Deploy) | `merge_branch` (develop) | Merge to staging on deploy |

**Chaining flow**: User drags story to Planning → Rule 1 fires (run planner) → planner works → user approves → Rule 2 fires (move to Dev) → Rule 3 fires (run engineer) → ... cascading through the pipeline.

### Loop Prevention

Four layers prevent infinite loops:

1. **Chain depth counter**: `RuleExecutionContext.Depth` incremented per chained event. Max 10.
2. **Same-rule dedup**: `FiredRuleIDs` tracks rules fired in the current chain. Same rule can't fire twice.
3. **State transition guard**: `move_to_state` is a no-op if story is already in the target state.
4. **Active run guard**: `AgentService.RunAgent` prevents duplicate concurrent runs on the same story.

### Integration Points

Rules are evaluated synchronously in:

- `PMStoryService.MoveToState` — emits `story.state_entered` after state change
- `PMStoryService.Create` — emits `story.state_entered` for initial state
- `AgentService.ApproveRun` — emits `agent_run.approved` after run approval

### API

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/pm/automation-rules?workspace_id=` | List rules |
| GET | `/api/pm/automation-rules?workspace_id=&workflow_id=` | List rules for workflow |
| POST | `/api/pm/automation-rules?workspace_id=` | Create rule |
| GET | `/api/pm/automation-rules/{id}?workspace_id=` | Get rule |
| PUT | `/api/pm/automation-rules/{id}?workspace_id=` | Update rule |
| DELETE | `/api/pm/automation-rules/{id}?workspace_id=` | Delete rule |

### Frontend Surfaces

- **Pipeline builder**: Visual horizontal pipeline editor in Settings > Teams > Workflow dialog. Shows states left-to-right with agent assignment dropdowns, auto-advance toggles, and merge branch inputs.
- **Kanban board**: Bot icon on column headers for states with `run_agent` rules.
- **Story cards**: Bot badge when an agent is assigned.
- **Story list view**: Bot icon next to story name when agent is assigned.
- **Story detail panel**: Horizontal pipeline step indicator showing completed, current, and pending stages with automation badges.
- **Settings > AI & Automations**: Automation rules group showing configured rules with health status.

### Key Files

Backend:

- `server/internal/model/automation_rule.go` — model, DTOs, event types
- `server/internal/repository/automation_rule.go` — CRUD + rule matching
- `server/internal/service/automation_rule_engine.go` — evaluation + action executors
- `server/internal/handler/automation_rule.go` — HTTP endpoints
- `server/migrations/042_automation_rules.sql` — table + indexes

Frontend:

- `frontend/src/lib/services/automationRuleService.ts` — API client
- `frontend/src/components/settings/PipelineBuilder.tsx` — visual pipeline editor
- `frontend/src/components/settings/TeamsTab.tsx` — pipeline builder integration
- `frontend/src/components/pm/KanbanBoard.tsx` — bot icons on columns
- `frontend/src/components/pm/StoryDetailPanel.tsx` — pipeline step indicator

## What Is Not Implemented Yet

- true `openclaw` execution backend
- true `zeroclaw` execution backend
- generic document-targeted agent runs
- automatic support-agent execution on assignment
- methodology packs for `engineer`, `reviewer`, or `support`
- public BMAD-style multi-persona agent catalog

## Recommended Usage

- Use `product_planner` for epic decomposition, product specs, and story planning.
- Use `engineer` for story implementation, branching, commits, and PRs.
- Use `reviewer` for validation and read-heavy QA runs.
- Use `support` for ticket triage and draft replies.
- Use users and workspace members to represent explicit human ownership or handoff targets.
- Prefer explicit handoffs and member assignment over adding more product-visible agent classes.
