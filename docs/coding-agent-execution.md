# How coding agents execute work

This guide explains what happens when an agent works on a repository. It helps
backend and frontend contributors follow a run from authorization and workspace
preparation through execution and delivery back into Helpin.

This document describes repository-backed agent execution after the Agent
Runtime hard cutover. It applies to Forge/Code Builder, Lens/Review Agent, and
custom agents configured for repository work.

For the broader ownership and trigger model, see
[agents and automation](agents-and-automation.md).

## Boundary

Helpin owns product configuration and delivery semantics. Agent Runtime owns
the execution workspace and model/tool loop.

| Helpin | Agent Runtime |
| --- | --- |
| Resolve workspace agent, version, target, repository, and delivery target | Store the projected executable agent definition |
| Enforce workspace/team access, entitlements, billing, tools, and targets | Enforce accepted runtime/tool/workspace policy |
| Create the Helpin `agent_run` and launch metadata | Create and execute the runtime run/workflow |
| Supply target context, domain commands, skills, and repository spec | Prepare the repository workspace and expose runtime tools |
| Project runtime events into product records | Run `native_sdk`, `codex`, or `opencode` |
| Apply product finalizers such as PR/task/git-link bookkeeping | Persist transcript, interactions, artifacts, usage, and repository outcome |

System and custom agents use this same boundary. `is_system` never selects a
coding executor.

## Executable configuration

A repository-capable agent is still an ordinary agent definition composed
from:

- `runtime_kind`, provider, model, and execution config;
- system prompt and skills;
- allowed tools and targets;
- approval and invocation policy;
- repository workspace mode and access policy.

Forge and Lens receive these values from managed presets. A custom agent may
request the same generic repository capabilities through explicit
configuration, but it does not inherit or track those presets.

Runtime kinds select adapters only:

- `native_sdk`: in-process model/tool loop inside Agent Runtime;
- `codex`: Codex app-server execution inside Agent Runtime;
- `opencode`: OpenCode command execution inside Agent Runtime.

## Launch sequence

```text
manual action, flow, schedule, or orchestration
  -> Helpin resolves agent + active version + target
  -> Helpin resolves repository/delivery metadata
  -> Helpin creates agent_run
  -> Helpin upserts executable agent into Agent Runtime
  -> Helpin starts runtime run with target and instructions
  -> Agent Runtime resolves target context and repository spec from Helpin
  -> Agent Runtime prepares isolated workspace and branch
  -> selected runtime adapter executes tools/model turns
  -> Agent Runtime commits/pushes when configured and reports repository result
  -> runtime events project into Helpin
  -> Helpin applies delivery finalizers and returns the agent to idle
```

Agent Runtime is the only executor. A missing/disabled runtime causes the run
start to fail; Helpin does not start a local Temporal agent workflow.

## Repository preparation

Repository preparation is selected through execution configuration, not agent
ownership. For managed Code Builder and Review Agent presets, Helpin injects
repository workspace mode. Custom agents must explicitly carry compatible
workspace configuration and be allowed to target the repository-backed object.

Agent Runtime asks Helpin's workspace provider for an authorized repository
spec. The spec identifies the repository, base branch, working branch, and
credentials without moving workspace tenancy or repository authorization into
Agent Runtime.

The runtime then creates an isolated `WorkspaceLease`, checks out the effective
working branch, stages runtime skills, and runs all filesystem, command, patch,
and git tools inside that lease.

## Tools and policy

Effective capability is the intersection of:

1. tools registered for the Helpin app;
2. tools allowed by the agent definition;
3. any narrower run-level tool list;
4. skill runtime/tool requirements;
5. command guards, approval policy, and workspace access mode.

Run-level configuration can narrow an agent's tools but cannot expand them.
Helpin product tools use the run-scoped MCP bridge; repository-native tools may
be supplied directly by the selected adapter/runtime registry.

## Delivery ownership

Repository mutation and product delivery are related but separate:

- Agent Runtime owns files changed during execution, runtime git operations,
  and the repository result written into the runtime output summary.
- Helpin owns delivery targets, pull-request creation/association, task git
  links, product activity, notifications, and idempotent terminal finalizers.

This split keeps Agent Runtime host-neutral while allowing Helpin to enforce
its repository and project-management semantics.

A terminal run status does not prove product delivery succeeded. Projection logs
finalizer errors without blocking the terminal status update. Summary-dependent
finalizers can also be deferred when the runtime output summary is unavailable.
Inspect repository-delivery results, finalizer markers and logs before claiming
a pull request or task update completed.

## Pauses and recovery

Interactive coding runs may pause for user input, approval, review checkpoints,
MCP authentication, or Codex authentication. Agent Runtime persists the session
and interaction; Helpin mirrors it and forwards the eventual response/resume.

Terminal runtime events are idempotently projected into Helpin. Reconciliation
is a backstop for missed live events. Delivery finalizers must remain safe to
retry because event delivery is at least once.

Cancellation and failure also flow through Agent Runtime. Helpin projects the
terminal state, records the error, releases product-level active-agent state,
and applies only the finalizers valid for that outcome.

## Architectural rules

- Do not add a Helpin-local coding executor.
- Do not branch execution on `is_system`.
- Do not encode Forge or Lens as runtime subclasses.
- Add reusable behavior as prompts, skills, tools, targets, or execution policy.
- Keep workspace tenancy, user authorization, and product delivery in Helpin.
- Keep runtime adapters, workspaces, tool execution, and durable run mechanics
  in Agent Runtime.

## Implementation references

- [Agent projection, tool subsets and launch](../server/internal/service/agent.go)
- [Repository specifications and host authorization](../server/internal/service/agent_runtime_host.go)
- [Runtime event projection](../server/internal/service/agent_runtime_projection.go)
- [Product finalizers](../server/internal/service/agent_runtime_finalizers.go)

Runtime adapters and workspace preparation live in the separate Agent Runtime
repository. Adapter support in source does not prove that a particular deployed
image includes the required executables or credentials; check the runtime image
and configuration when diagnosing a launch failure.
