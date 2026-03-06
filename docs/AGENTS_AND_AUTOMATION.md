# Helpin: Agents And Automation

## Overview

Helpin treats agents as workflow participants inside the PM and support systems. Helpin owns:

- assignments
- run lifecycle
- approvals
- handoffs
- artifacts
- story delivery targets
- branch and PR history

Execution happens through shared Temporal worker pools. Temporal owns retries, cancellation, approval waits, and handoff signaling. Shared runners own the actual tool loop and repo workspace.

## Core Model

| Concept | Meaning |
|---|---|
| `agent` | A workspace-scoped human or LLM participant |
| `agent_run` | A single execution against one target |
| `target_type` / `target_id` | Canonical run target: story or support ticket |
| `story_delivery_target` | The current repo lane for a story |
| `story_git_link` | Historical branch, commit, and PR output for a story |
| `artifact` | A stored run output such as logs, diffs, or PR metadata |
| `handoff` | Explicit transfer from one agent to another agent or a human |

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

### Execution

Each run gets:

- a fresh temp workspace
- a repo clone when the profile requires one
- a capability-profile tool policy
- heartbeat updates written back to `agent_runs`

The current implementation uses shared runners, not one container or pod per run.

## Agent Kinds

- `llm`: executable agents
- `human`: assignment and handoff only, never executed

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

## Capability Profiles

Current profiles:

- `engineer`
- `planner`
- `reviewer_tester`
- `support`
- `orchestrator`
- `human_proxy`

Profile intent:

- `engineer`: repo mutation, commit, push, PR creation
- `planner`: read-heavy planning and PRD generation, no repo mutation
- `reviewer_tester`: read-heavy validation and test execution
- `support`: ticket triage and draft replies with approval boundary
- `orchestrator`: epic decomposition flow
- `human_proxy`: explicit handoff target only

## Trigger Modes

- `manual`
- `auto_on_assignment`
- `auto_on_event`

Current behavior:

- human agents never execute
- story assignment supports `manual` and `auto_on_assignment`
- `auto_on_assignment` only starts when the story has a valid delivery target if the profile requires a repo
- support runs are still manual today

## Story Delivery Model

Stories now separate planning state from Git delivery state.

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

Helpin uses GitHub App installations for repo mutation workflows.

Current behavior:

- integrations are stored per workspace
- repository catalogs are synced from the app installation
- worker activities mint short-lived installation tokens
- authenticated remotes are not written into `.git/config`
- webhook processing resolves the workspace from the installation ID and verifies the shared secret

Legacy PAT persistence remains only for migration compatibility and should not be used for new setups.

## Story Run Flow

Practical flow:

1. Create an agent
2. Create a story
3. Configure or inherit the story delivery target
4. Assign the agent
5. Run the agent, or let `auto_on_assignment` trigger it

Execution flow:

1. API creates `agent_run`
2. Temporal workflow starts
3. prepare activity resolves repo and branch state
4. execute activity runs the tool loop in a shared runner
5. artifacts and run status update live
6. PR and push events update delivery state and history

## Support Run Flow

Support runs are still target-based agent runs, but without story delivery.

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

The worker intersects repository policy with the capability profile.

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

- story detail `Delivery` block for:
  - agent picker
  - repository selector
  - base branch
  - branch preview
  - PR status
  - manual run action
- `Agent Runs` panel for:
  - runner pool
  - execution stage
  - repo snapshot
  - artifacts
  - approval and cancellation actions
- Settings:
  - GitHub App install flow
  - GitHub integration list
  - repository catalog selection
  - repository sync
  - team delivery defaults
  - runner queue visibility
  - active run health

## What Is Not Implemented Yet

- true OpenClaw execution backend
- true ZeroClaw execution backend
- generic document-targeted agent runs
- automatic support-agent execution on assignment
- a dedicated reviewer/tester UI beyond the shared run panel

## Recommended Usage

- Use `orchestrator` for epic decomposition.
- Use `planner` for PRDs, planning, and repo-aware analysis without mutation.
- Use `engineer` for implementation, branching, commits, and PRs.
- Use `reviewer_tester` for validation and read-heavy QA runs.
- Use `support` for ticket triage and draft replies.
- Use `human_proxy` to represent explicit human ownership or handoff targets.
- Prefer explicit handoffs over adding many specialized agents to the product model.
