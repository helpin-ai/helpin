# Automation Taxonomy RFC

This document is the Phase 1c source of truth for Teampulse's shared automation control plane.

It is intentionally a documentation phase, not an implementation phase. The point is to make the automation model simpler and more coherent before more platform work lands.

Related docs:

- `docs/AGENTS_AND_AUTOMATION.md`
- `docs/crm-buyer-signal-ingestion.md`
- `docs/crm-entity-summaries.md`
- `docs/automation-taxonomy-seed-catalog.md`
- `docs/automation-taxonomy-migration.md`
- `docs/automation-taxonomy-product-ia.md`

## Summary

Teampulse already has three real automation systems:

1. CRM background automations on `automation-default`
2. explicit PM/support agents with strict target mapping and run lifecycle
3. PM built-in deterministic automations with workspace config

The shared control plane should stay close to those realities instead of introducing extra conceptual layers.

The canonical top-level taxonomy is now:

- `built_in_automation`
- `automation_rule`

### Taxonomy evolution (Phase 1c → Phase 2)

The original Phase 1c taxonomy had three kinds: `built_in_automation`, `contextual_agent`, and `custom_automation`. With the introduction of the automation rules engine, this was simplified to two kinds:

- `contextual_agent` was **dropped as an automation kind**. Agents remain as executors — the "action" side of a rule — but are no longer a separate taxonomy kind. They are still configured at `/pm/agents`.
- `custom_automation` was **replaced by `automation_rule`**. User-configured rules with trigger → action semantics. The stage-based agent pipeline is the first real implementation.

Important simplifications:

- PM built-in rules (epic auto-start/complete, sprint scheduling) remain `built_in_automation` — they operate on different entity/state systems
- Agents are executors referenced by `run_agent` actions, not a taxonomy kind
- `Settings > AI & Automations` shows both built-in automations and automation rules
- standardized trigger types now include:
  - `story.state_entered`
  - `agent_run.approved`
  - `manual`
  - `assignment_event`
  - `cron`

## Problem Statement

Without a shared model, Teampulse risks three different meanings of automation:

- CRM background intelligence
- explicit PM/support agents
- PM workspace automation rules

If each grows independently, the product becomes harder to explain:

- "agent" starts meaning too many things
- PM rules look like a separate platform
- CRM background work stays invisible and ungoverned
- future custom automation has no obvious place to live

The goal of this RFC is to simplify that story, not to build a maximal platform abstraction.

## Current Systems

### CRM background automations

Current examples:

- buyer-signal ingestion
- contact summary refresh
- deal summary refresh

Current characteristics:

- system-owned
- Temporal-backed
- run on `automation-default`
- not user-creatable
- outputs appear in CRM module surfaces, not run consoles

### Explicit PM/support agents

Current examples:

- `product_planner`
- `engineer`
- `reviewer`
- `support`

Current characteristics:

- workspace-scoped agent records
- explicit runs, approvals, handoffs, and artifacts
- strict target mapping
- class-specific queues
- user-visible primary surface in `/pm/agents`

### PM built-in deterministic automations

Current examples:

- epic auto-start
- epic auto-complete
- sprint auto-create
- sprint move unfinished

Current characteristics:

- configured in `pm_automations`
- deterministic and non-agentic
- epic rules run inline on story workflow-state changes
- sprint rules run from an hourly API-process ticker

## Canonical Model

### 1. `built_in_automation`

This is the category for product-owned automation behavior that is shipped as part of Teampulse.

It includes both:

- CRM background intelligence
- PM deterministic built-in rules

The important distinction is metadata, not a separate top-level kind.

Required metadata for built-ins:

- `config_scope`
  - `system`
  - `workspace`
  - `team`
- `execution_style`
  - `background_workflow`
  - `deterministic_rule`
  - `scheduled_rule`
- `user_governed`
  - `true`
  - `false`

Interpretation:

- CRM signals and summaries are **system-governed built-ins**
- PM epic/sprint rules are **workspace-governed built-ins**

### 2. `automation_rule`

This is the category for user-configured event-driven automation rules.

Properties:

- user-created and user-managed
- trigger → action pattern
- stored in `automation_rules` table
- evaluated by the automation rules engine
- supports chaining with loop prevention

Current trigger types:

- `story.state_entered` — fires when a story enters a workflow state
- `agent_run.approved` — fires when an agent run is approved

Current action types:

- `run_agent` — assigns and runs an agent on the story
- `move_to_state` — moves the story to a target workflow state
- `merge_branch` — merges the story's working branch into a target branch

The primary implementation is the **stage-based agent pipeline**: agents are assigned to workflow states and automatically triggered as stories move through the pipeline.

### Agents as executors, not a taxonomy kind

Agents (`product_planner`, `engineer`, `reviewer`, `support`) remain first-class entities configured at `/pm/agents`. They are the **executors** referenced by `run_agent` actions in automation rules. They retain their full run lifecycle (runs, approvals, handoffs, artifacts) but are no longer classified as a separate automation taxonomy kind.

## Why PM Rules Stay Inside Built-ins

PM epic/sprint automations should not become automation rules because:

- they operate on different entities (epics, sprints) not story workflow states
- they are aggregation rules ("are ALL stories done?") not event-driven trigger → action rules
- they do not have the same extensibility needs as user-configured pipeline rules

The simpler product story is:

- some automation is built in (CRM intelligence, PM epic/sprint rules)
- some automation is user-configured (automation rules, stage-based pipelines)
- agents are executors that both kinds can leverage

Within built-ins, PM rules and CRM background intelligence are separated by metadata and presentation subgrouping, not by a separate foundational kind.

## Trigger Model

The current standardized trigger shapes are:

- `manual`
- `assignment_event`
- `cron`
- `story.state_entered`
- `agent_run.approved`

### `manual`

Used where a human intentionally launches work.

Examples:

- PM planner on an epic
- PM engineer on a story
- support agent on a conversation

### `assignment_event`

Used for the currently implemented auto-run behavior behind agent assignment.

### `cron`

Used for scheduled reconciliation or periodic sweeps.

Examples:

- CRM daily summary reconciliation
- PM sprint auto-create sweep
- PM sprint move-unfinished sweep

### `story.state_entered`

Used by automation rules to trigger actions when a story enters a specific workflow state.

This is the primary trigger for the stage-based agent pipeline. Config shape: `{ state_id: string }`.

### `agent_run.approved`

Used by automation rules to trigger actions when an agent run is approved while the story is in a specific state.

This enables auto-advance behavior in pipelines. Config shape: `{ state_id: string }`.

## Event Framework Maturity

The automation rules engine introduces real event-driven triggers (`story.state_entered`, `agent_run.approved`), but these are purpose-built for the rules engine, not a generic cross-app event bus.

Important constraints:

- CRM built-ins still run from internal domain hooks
- those hooks are not yet a shared cross-app event catalog
- future trigger types (e.g., `story.created`, `pr.merged`, `label.changed`) can be added without restructuring

## Shared Catalog Shape

The shared catalog abstraction is:

- `automation_catalog_entry`

It is code-defined in `server/internal/automationcatalog/registry.go`.

It owns:

- built-in identity
- top-level kind
- target semantics
- default visibility
- queue/runtime mapping defaults
- built-in metadata such as:
  - `config_scope`
  - `execution_style`
  - `user_governed`

## Read Models

Two shared read models aggregate across all systems:

- `automation_inventory_view` — aggregates built-in automations and automation rules
- `automation_health_view` — aggregates health from CRM diagnostics, rule execution, and PM built-ins

These are projection layers over current systems, not new sources of truth.

## Settings and Write Surfaces

`Settings > AI & Automations` provides inventory, governance, and diagnostics.

Write surfaces remain distributed:

- `/pm/agents` — agent management
- Settings > Teams > Workflow pipeline builder — automation rules
- PM settings automations — PM built-in rules
- CRM domain settings — CRM built-ins

## Product Organization

`Settings > AI & Automations` is organized into:

- `Built-in Automations`
  - CRM system intelligence
  - PM built-in rules
- `Automation Rules`
  - User-configured trigger → action rules
  - Pipeline rules per workflow

This two-group model is simpler than the original three-group proposal and reflects the actual system boundaries.

## Decision Locks

The following are locked by this RFC:

- the top-level taxonomy has two kinds: `built_in_automation` and `automation_rule`
- PM built-in rules remain built-ins, not automation rules (they operate on epic/sprint entities, not story workflow states)
- agents are executors (the action side of rules), not a taxonomy kind
- the trigger model includes `story.state_entered` and `agent_run.approved` for automation rules, plus `manual`, `assignment_event`, and `cron` for other purposes
- `Settings > AI & Automations` shows inventory, governance, and diagnostics for both kinds
- the pipeline builder in Settings > Teams > Workflow is the primary configuration surface for automation rules
- configuration UIs are domain-specific, not generic (see below)

## UI Architecture: Domain-Specific Over Generic

The `automation_rules` backend is deliberately generic — it supports arbitrary trigger → action pairs via JSONB config. But the frontend does NOT expose a generic rules management UI.

**Rationale**: Different automation domains have fundamentally different configuration needs:

| Domain | Trigger | Action | Config Complexity |
|---|---|---|---|
| PM pipeline | story state change | run agent, move state, merge branch | workflow + state + agent pickers |
| CRM signals | external signal + LLM confidence | create/progress deal | threshold sliders, pipeline selectors |
| CRM sequences | manual enrollment | multi-step email/delay/task | step editor with sequencing |
| Cross-entity | story completed | create new story elsewhere | target team, template, repo selectors |

A single generic form cannot meaningfully configure all of these. Each domain gets its own focused UI:

- **PM pipelines** → pipeline builder in Settings > Teams > Workflow dialog
- **CRM signals** → CRM autonomy settings (threshold sliders)
- **CRM sequences** → sequence editor
- **Future domains** → purpose-built UIs informed by actual config requirements

`Settings > AI & Automations` serves as the **unified dashboard** — see all automations, their health, and follow links to the right domain-specific config surface.

The backend `automation_rules` table and rule engine remain the shared foundation. New trigger/action types are added to the backend model + engine, and each gets a purpose-built UI at the appropriate settings surface.

## Review Questions

This RFC is complete only if future implementers can answer these directly:

- Where does CRM summary refresh belong?
  - `built_in_automation`
- Where does PM epic auto-complete belong?
  - `built_in_automation`, workspace-governed deterministic subtype
- Where do PM/support explicit agents belong?
  - agents are executors referenced by `automation_rule` actions, configured at `/pm/agents`
- Where does a stage-based agent pipeline belong?
  - `automation_rule`, configured in Settings > Teams > Workflow pipeline builder
- What trigger types exist?
  - `story.state_entered`, `agent_run.approved`, `manual`, `assignment_event`, `cron`
