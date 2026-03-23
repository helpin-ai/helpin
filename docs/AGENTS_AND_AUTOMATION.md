# Agents And Automation

This is the single source of truth for the current agent and automation model.

It describes the runtime that exists in code now, not older design directions.

## Summary

The system is intentionally small:

- `agent_run` is the normal execution primitive
- execution modes are `interactive` and `autonomous`
- runtimes are `native_sdk` and `opencode`
- agents are configured from presets plus explicit policy
- tools and service-layer validation are the safety boundary
- automation taxonomy has two top-level kinds: built-in automation and automation rules

## Core Concepts

### Agent

An agent is a configured executor record.

Important fields:

- `preset_key`
- `system_prompt`
- `allowed_tools`
- `allowed_commands`
- `allowed_targets`
- `approval_mode`
- `runtime_kind`
- `default_invocation_mode`

Agents are not runtime classes. The product model no longer uses `agent_class` or `capability_profile`.

### Agent Run

An `agent_run` is one durable execution against one target.

Normal entry points converge here:

- story execution
- epic planning
- support conversation runs
- scheduled agent runs
- automation rules that start runs

### Preset

A preset is a bundle of defaults, not a special execution engine.

Current presets:

- `epic_planner`
- `story_planner`
- `crm_operator`
- `support_agent`
- `code_builder`
- `review_agent`

Presets provide defaults for prompt, tools, targets, runtime, and default mode. They do not create hidden orchestration behavior.

### Tool

Tools are the capability boundary.

They are also where domain safety is enforced. Examples:

- story creation inherits the epic team and rejects teamless epics
- document writes validate target type and permissions
- support reply tools keep support-specific rules in backend services
- CRM mutation tools validate entity existence and permissions

## Execution Model

### Modes

- `interactive`
  - transcript-driven
  - can ask questions inline
  - can wait for normal user replies in chat
- `autonomous`
  - background execution
  - does not depend on a chat-first loop

### Runtimes

- `native_sdk`
  - supports `interactive`
  - supports `autonomous`
- `opencode`
  - supports `autonomous`
  - does not support `interactive`

### Queues

Queues are chosen by runtime and mode, not by planner/support/engineer labels:

- `agent-native-interactive`
- `agent-native-autonomous`
- `agent-opencode-autonomous`
- `automation-default` for non-agent automation paths

## Current Agent Presets

| Preset | Typical targets | Default mode | Runtime |
| --- | --- | --- | --- |
| `epic_planner` | `epic`, `story`, `crm_deal` | `interactive` | `native_sdk` |
| `story_planner` | `story`, `epic` | `interactive` | `native_sdk` |
| `crm_operator` | `crm_deal`, `support_conversation`, `document` | `interactive` | `native_sdk` |
| `support_agent` | `support_conversation` | `autonomous` | `native_sdk` |
| `code_builder` | `story` | `autonomous` | `opencode` |
| `review_agent` | `story` | `autonomous` | `opencode` |

## Epic Planning

Epic planning is one interactive `agent_run` against an epic.

The active model is:

1. ask clarifying questions inline in chat when needed
2. draft the PRD inline
3. request approval inline in chat
4. persist the approved spec through tools
5. draft the story plan inline
6. request approval inline in chat
7. create stories through tools

Important constraints:

- there is no separate planning-session product
- there is no flow/node runtime behind normal epic planning
- there is no hidden planner phase machine deciding the steps in code
- approvals are soft gates inside the same chat transcript

## Automation Taxonomy

There are only two top-level automation kinds:

- `built_in_automation`
- `automation_rule`

Agents are not a third top-level automation kind. They are reusable executors that humans, schedules, and automation rules can launch.

## Built-In Automation

Built-ins are product-owned behavior, not user-authored agents.

Examples:

- CRM buyer-signal ingestion
- CRM summary refresh
- deterministic PM epic and sprint rules

These may run inline, on schedules, or in background workflows. They remain separate from generic agents.

## Automation Rules

Automation rules are user-authored trigger-to-action behavior.

Current active triggers include:

- `story.state_entered`
- `agent_run.approved`
- `cron`

Current active action surface includes:

- `start_agent_run`
- `move_to_state`
- `merge_branch`
- `run_command`

Legacy action names such as `run_agent` and `start_flow` still exist as historical constants in some code paths, but they are not supported runtime behavior.

## What The System Is Not

The current model should not be understood as:

- agent classes such as planner/engineer/reviewer/support controlling execution
- a planning-session runtime
- a flow-template runtime for normal work
- a hidden epic-planner workflow with hard backend phases

Those are older design directions, not the active product model.

## Historical Residue

Some historical residue still exists:

- old SQL reference migrations
- a small internal runtime-profile helper used to seed preset defaults
- older PRDs or design docs kept for historical context

Those are history only. They are not the model to build new work on.

## Rule For New Work

When adding a new operator:

- do not add a new runtime type
- do not add a new hidden workflow engine
- add or extend a preset
- give it the right tools and targets
- keep business invariants in backend services

That is the intended shape of the platform.
