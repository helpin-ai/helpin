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
| `target_type` / `target_id` | Canonical run target: story, epic, or support ticket |
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
The legacy `agent_jobs` poller has been removed. Temporal workflows are the only supported execution path.

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
- epic orchestrator runs are manual today
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

### GitHub App setup checklist

For a working install flow, configure the GitHub App with:

- `Homepage URL`: the frontend base URL, for example `http://65.109.173.49:5173`
- `Setup URL`: the backend callback endpoint, for example `http://65.109.173.49:8080/api/git/github/callback`
- `Callback URL`: the same backend callback endpoint is acceptable, but the install return path depends on `Setup URL`
- `Webhook URL`: the backend webhook endpoint, for example `http://65.109.173.49:8080/api/git/webhook`

Recommended repository permissions:

- `Contents`: `Read and write`
- `Pull requests`: `Read and write`
- `Metadata`: `Read-only`

Current webhook subscriptions used by Helpin:

- `Push`
- `Pull request`

Install flow notes:

- Saving the GitHub App settings page does not redirect back to Helpin
- The redirect back to Helpin only happens when the install is started from `Project Settings > Delivery`
- The install completion path depends on the GitHub App `Setup URL`

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

Required runtime environment variables:

- API:
  - `DATABASE_URL`
  - `JWT_SECRET`
  - `TEMPORAL_ADDRESS`
  - `TEMPORAL_NAMESPACE`
  - `TEMPORAL_API_KEY` for Temporal Cloud
  - `TEMPORAL_TLS_ENABLED` for TLS connections; automatically enabled when `TEMPORAL_API_KEY` is set
  - `TEMPORAL_TLS_SERVER_NAME` optional override for TLS server name
  - `APP_BASE_URL`
  - `GITHUB_APP_ID`
  - `GITHUB_APP_SLUG`
  - `GITHUB_APP_PRIVATE_KEY` as base64-encoded PEM
- Worker:
  - `DATABASE_URL`
  - `JWT_SECRET`
  - `TEMPORAL_ADDRESS`
  - `TEMPORAL_NAMESPACE`
  - `TEMPORAL_API_KEY` for Temporal Cloud
  - `TEMPORAL_TLS_ENABLED` for TLS connections; automatically enabled when `TEMPORAL_API_KEY` is set
  - `TEMPORAL_TLS_SERVER_NAME` optional override for TLS server name
  - `GITHUB_APP_ID`
  - `GITHUB_APP_PRIVATE_KEY` as base64-encoded PEM
  - `ANTHROPIC_API_KEY` for LLM-backed runs

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

## Epic Orchestration Flow

Epic orchestration now uses the same `agent_run` and Temporal workflow model as story and support runs.

Practical flow:

1. Create an orchestrator agent
2. Assign the orchestrator to an epic
3. Start an epic agent run
4. Review the proposal artifact
5. Confirm the proposal to create stories

Execution flow:

1. API creates an epic-targeted `agent_run`
2. Temporal workflow starts
3. worker loads epic context and existing stories
4. orchestrator runtime produces an `orchestration_proposal` artifact
5. run waits in approval state for human review
6. confirming the run creates stories and approves the run

Current behavior:

- the orchestrator produces planning output only
- implementation work still happens on story-targeted runs
- epic proposals are stored as artifacts and can be edited before story creation
- confirmation can optionally assign agents to created stories

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
| GET | `/api/pm/epics/{id}/agent-runs?workspace_id=` | List runs for epic orchestrator work |
| POST | `/api/pm/epics/{id}/run-agent?workspace_id=` | Run epic orchestrator |
| GET | `/api/pm/agents/{id}/runs?workspace_id=` | List runs for agent |
| GET | `/api/pm/agent-runs/{id}?workspace_id=` | Get run |
| GET | `/api/pm/agent-runs/{id}/artifacts?workspace_id=` | Get artifacts |
| POST | `/api/pm/agent-runs/{id}/cancel?workspace_id=` | Cancel run |
| POST | `/api/pm/agent-runs/{id}/approve?workspace_id=` | Approve run result |
| POST | `/api/pm/agent-runs/{id}/confirm-orchestration?workspace_id=` | Create stories from an epic proposal and approve the run |
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
- epic detail `Epic Orchestration` block for:
  - orchestrator assignment
  - epic run start
  - run history
  - proposal artifact review
  - confirmation into story creation
- Settings:
  - Project Settings > Delivery:
    GitHub App install flow, integration list, repository catalog selection, repository sync, runner queue visibility, and active run health
  - team delivery defaults

## What Is Not Implemented Yet

- true OpenClaw execution backend
- true ZeroClaw execution backend
- generic document-targeted agent runs
- automatic support-agent execution on assignment
- a dedicated reviewer/tester UI beyond the shared run panel

## Recommended Usage

- Use `orchestrator` for epic decomposition.
- Let the orchestrator decompose epics into stories first; do not use it for implementation work.
- Use `planner` for PRDs, planning, and repo-aware analysis without mutation.
- Use `engineer` for implementation, branching, commits, and PRs.
- Use `reviewer_tester` for validation and read-heavy QA runs.
- Use `support` for ticket triage and draft replies.
- Use `human_proxy` to represent explicit human ownership or handoff targets.
- Prefer explicit handoffs over adding many specialized agents to the product model.
