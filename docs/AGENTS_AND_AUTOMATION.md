# Teampulse: Agents & Automation Guide

## Overview

Teampulse treats agents as workflow participants inside the product, not as the product itself.

Teampulse owns:

- work items
- assignments
- run lifecycle
- approvals
- handoffs
- artifacts
- links between tickets, stories, branches, and PRs

Runtime adapters own execution details such as model calls, terminal access, and backend-specific skills.

## Core Model

| Concept | Meaning |
|---|---|
| `agent` | A registered automation participant in a workspace |
| `agent_run` | A single isolated execution against one target |
| `target_type` / `target_id` | The canonical run target: story, support ticket, or another future work item |
| `artifact` | A traceable run output such as logs, diffs, test output, or draft handoff notes |
| `handoff` | An explicit transition from an agent to another agent or to a human |
| `approval_state` | Whether a run result requires human approval before final publication |

## Product Principles

- One active executor per work item by default.
- Multi-agent collaboration happens through explicit handoffs or separate child work items, not concurrent swarms on the same story or ticket.
- Human review is a first-class boundary.
- Agent configuration is policy-based through capability profiles rather than ad hoc tool lists.

## Team Members And Agents

| | Team Members | Agents |
|---|---|---|
| What they are | Real users invited to the workspace | Workflow participants registered in Teampulse |
| Authentication | Email/password + JWT | No direct login |
| Primary purpose | Human collaboration and review | Planned or automated execution |
| Can be assigned | Yes | Yes |
| Can execute runs | Humans work outside the worker | Only `llm` agents can execute runs |
| Human representation | Native user account | Optional `human` agent using `backing_user_id` |

## Agent Kinds

- `llm`: executable runtime-backed agents
- `human`: non-executable human proxies used for assignment and handoff flows

## Recommended Roles

Teampulse is designed around a small role set:

- `orchestrator`
- `engineer`
- `reviewer_tester`
- `support`
- `human_proxy`

Custom labels are still allowed, but the product does not model a 5-10 agent swarm as the primary workflow.

## Agent Configuration

Each agent stores:

- `runtime_kind`
- `capability_profile`
- `skills`
- `trigger_mode`
- `backing_user_id` for human proxies
- model and system prompt for `llm` agents

### Runtime Kinds

Schema values:

- `native_claude`
- `claude_code`
- `openclaw`
- `zeroclaw`

Current execution support:

- `native_claude`: implemented
- `claude_code`: implemented through the runtime adapter path and currently routed through the same Claude worker loop
- `openclaw`: reserved, not wired yet
- `zeroclaw`: reserved, not wired yet

### Capability Profiles

Capability profiles control what a run can do. The worker enforces the profile and only exposes the allowed tools for that profile.

Current profiles:

- `engineer`
- `reviewer_tester`
- `support`
- `orchestrator`
- `human_proxy`

### Trigger Modes

- `manual`
- `auto_on_assignment`
- `auto_on_event`

Current behavior:

- story assignment supports `auto_on_assignment`
- support assignment is manual today

## Story Workflow

### Assign An Agent

Assigning an agent to a story updates `assigned_agent_id`.

If the assigned agent is:

- `human`: ownership/handoff only, no run starts
- `llm` with `trigger_mode = manual`: no run starts
- `llm` with `trigger_mode = auto_on_assignment`: a run is created automatically

### Run An Agent

Running a story agent creates:

- an `agent_run`
- an `agent_job`

The run stores:

- `target_type = story`
- `target_id = {story_id}`
- legacy `story_id` for compatibility
- `runtime_kind`
- `approval_state`

### Story Run Execution

The worker:

1. claims the queued job
2. resolves the runtime adapter
3. loads story context and checklist items
4. optionally clones the linked repository
5. applies the capability profile and repository command policy
6. executes the tool loop
7. persists artifacts and final run state

## Support Workflow

### Assign An Agent

Support tickets can be assigned an agent through `assigned_agent_id`.

Assignment alone does not send customer replies.

### Run Support Agent

`POST /api/support/tickets/{id}/run-agent` creates a ticket-targeted run with:

- `target_type = support_ticket`
- `target_id = {ticket_id}`
- legacy `ticket_id` for compatibility

The support capability profile allows the agent to:

- inspect ticket messages
- update ticket status
- draft a customer or internal reply

### Approval Boundary

Support replies are drafted during the run and stored for review.

Customer-visible publication requires human approval via:

- `POST /api/pm/agent-runs/{id}/approve`

Approval publishes the drafted support message into the ticket thread.

## Epic Orchestration

Epic orchestration remains a human-confirmed planning flow.

Current behavior:

- assign an orchestrator agent to the epic
- call the orchestration endpoint
- receive a proposed story list
- edit or remove proposals
- confirm creation

Important note:

- orchestration is currently a direct JSON-generation flow, not a general multi-agent planner runtime

## Agent Runs

### Run Status

- `queued`
- `running`
- `completed`
- `failed`
- `cancelled`

### Approval Status

- `not_required`
- `pending`
- `approved`
- `rejected`

### Run Controls

Implemented endpoints:

- `POST /api/pm/agent-runs/{id}/cancel`
- `POST /api/pm/agent-runs/{id}/approve`
- `POST /api/pm/agent-runs/{id}/handoff`

## Artifacts

Teampulse standardizes run artifacts as:

- `conversation_log`
- `tool_log`
- `diff`
- `test_report`
- `pr_metadata`
- `agent_summary`
- `file_bundle`
- `handoff_note`

Notes:

- `test_report` is produced when the worker sees test-like command execution
- `pr_metadata` is produced when a PR is opened through the git tool
- `handoff_note` is created when a run records an explicit handoff

## Worker Architecture

The worker remains a separate deployment from the API server.

Flow:

```text
API server
  -> agent_runs / agent_jobs
  -> PostgreSQL
  -> worker
  -> artifacts + run updates
  -> websocket events
  -> frontend
```

### Runtime Adapters

The worker resolves a runtime adapter by `runtime_kind`.

Current adapter path:

- `native_claude`
- `claude_code`

Reserved but not yet implemented:

- `openclaw`
- `zeroclaw`

## Repository Policy

`WORKFLOW.md` in the repo root can still constrain execution:

```yaml
---
max_iterations: 50
timeout_minutes: 30
allowed_commands: [go, npm, make, git]
handoff_state: "human_review"
---
```

The worker intersects repository command policy with the agent capability profile.

## Runtime Profiles API

Implemented:

- `GET /api/pm/runtime-profiles?workspace_id=`

This returns the capability profiles Teampulse knows how to enforce.

## API Reference

### Agents

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/pm/agents?workspace_id=` | List agents |
| POST | `/api/pm/agents?workspace_id=` | Create agent |
| GET | `/api/pm/runtime-profiles?workspace_id=` | List runtime profiles |
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

## What Is Not Implemented Yet

- true OpenClaw execution backend
- true ZeroClaw execution backend
- concurrent multi-agent execution on the same work item
- generic document-targeted agent runs
- automatic support-agent execution on assignment
- a dedicated reviewer/tester UI flow beyond the shared run model

## Recommended Usage

- Use `orchestrator` for epic decomposition.
- Use `engineer` for story implementation and PR generation.
- Use `support` for ticket triage and draft replies.
- Use `human_proxy` to represent explicit human ownership or handoff targets.
- Prefer explicit handoffs over adding more specialized agents to the product model.
