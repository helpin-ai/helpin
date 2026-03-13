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

The canonical top-level taxonomy for Phase 1c is:

- `built_in_automation`
- `contextual_agent`
- `custom_automation`

Important simplifications:

- PM built-in rules are a subtype of `built_in_automation`, not a separate top-level kind
- `Settings > AI & Automations` is a future read/governance home first
- the only standardized trigger shapes in this phase are:
  - `manual`
  - `assignment_event`
  - `cron`
- the next implementation phase after 1c is an adapter-backed inventory/health read model only

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
  - `interactive_agent`
- `user_governed`
  - `true`
  - `false`

Interpretation:

- CRM signals and summaries are **system-governed built-ins**
- PM epic/sprint rules are **workspace-governed built-ins**

### 2. `contextual_agent`

This is the category for explicit runnable agents.

Properties:

- user-visible
- contextual
- approvals, handoffs, and artifacts
- launched from a target-specific surface or a dedicated agent surface

Examples:

- PM product planner
- PM engineer
- PM reviewer
- support agent

### 3. `custom_automation`

This is the reserved future category for user-created automation.

It is intentionally not implemented in Phase 1c.

Properties:

- configurable
- future-facing
- should eventually use the same catalog and diagnostics model

## Why PM Rules Stay Inside Built-ins

PM epic/sprint automations should not become agents because they do not have:

- personas
- handoffs
- approvals
- explicit run artifacts
- open-ended prompt-driven behavior

They also should not be a separate top-level category in the product because that teaches users an extra distinction they do not need.

The simpler product story is:

- some automation is built in
- some automation is explicit agent work
- some automation will later be user-created

Within built-ins, PM rules and CRM background intelligence are separated by metadata and presentation subgrouping, not by a separate foundational kind.

## Trigger Model

Phase 1c should standardize only the trigger shapes that are real today:

- `manual`
- `assignment_event`
- `cron`

### `manual`

Used where a human intentionally launches work.

Examples:

- PM planner on an epic
- PM engineer on a story
- support agent on a conversation

### `assignment_event`

Used for the currently implemented auto-run behavior behind agent assignment.

Current truth:

- assignment-triggered auto-run exists today
- it is the only event-shaped trigger that should be standardized in Phase 1c

### `cron`

Used for scheduled reconciliation or periodic sweeps.

Examples:

- CRM daily summary reconciliation
- PM sprint auto-create sweep
- PM sprint move-unfinished sweep

## What Phase 1c Does Not Claim

Phase 1c does **not** define a mature generic event framework.

Important constraints:

- CRM built-ins may still run from internal domain hooks today
- those hooks are not yet a shared cross-app event catalog
- agent `auto_on_event` remains legacy and should be treated as future deprecation/replacement

Later phases can broaden `assignment_event` into a real event catalog. Phase 1c should stay honest about current maturity.

## Shared Catalog Shape

The first shared catalog abstraction should be:

- `automation_catalog_entry`

It should be code-defined only.

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

## Adapter-Backed Read Models

The first implementation phase after this RFC should add two shared read models:

- `automation_inventory_view`
- `automation_health_view`

Those views should be built through adapters over the current systems:

- CRM built-ins
- PM built-in deterministic automations
- contextual agents

They are intentionally read models first:

- no runtime migration
- no generic write abstraction
- no DB registry as the built-in source of truth

The first implementation milestone after this RFC is therefore:

- code-defined catalog entries
- inventory adapters
- health adapters

and not:

- a broad event framework
- a generic `automation_config` layer
- a new write surface in shared Settings

## Future Settings Role

`Settings > AI & Automations` should be introduced later as:

- inventory
- governance
- diagnostics

It should not be the initial write source of truth.

Near-term write surfaces remain:

- `/pm/agents` for contextual agents
- existing PM automation settings for PM built-in rules
- existing CRM domain settings where applicable for CRM built-ins

Future shared settings can later:

1. expose read-only inventory and health
2. link out to current write surfaces
3. add selective write overlays later

That sequencing is part of the architecture, not a UX afterthought.

It should not own runtime state or workspace-specific configuration.

## Read Models, Not Config Unification

The first implementation phase after Phase 1c should add two adapter-backed read models:

- `automation_inventory_view`
- `automation_health_view`

### `automation_inventory_view`

This read model should aggregate:

- built-in CRM automations
- built-in PM deterministic automations
- current contextual agents

### `automation_health_view`

This read model should aggregate:

- CRM diagnostics and freshness state
- contextual agent run health
- minimal PM built-in automation health

Important boundary:

- there is no generic shared config abstraction in that first implementation phase
- there is no DB registry as the source of truth
- existing write/config surfaces remain where they are

## Current Write Surfaces vs Future Read Surface

### Current write surfaces remain

- `/pm/agents` stays the write and execution surface for contextual agents
- PM automation settings stay the write surface for PM deterministic built-ins
- CRM domain settings stay the write surface for CRM built-ins where applicable

### Future shared home

The future shared home is:

- `Settings > AI & Automations`

But it should begin as:

- inventory
- governance
- diagnostics

It should not be described as the first canonical write source of truth.

## Future Product Organization

The future `Settings > AI & Automations` home should be organized into:

- `Built-in Automations`
- `Contextual Agents`
- `Custom Automations`

Inside `Built-in Automations`, subgrouping can be used for clarity:

- CRM system intelligence
- PM built-in rules

That gives product clarity without creating another top-level platform category.

## Next Implementation Milestone

The first implementation phase after this RFC should be limited to:

- code-defined built-in catalog
- adapters over current systems
- adapter-backed inventory read model
- adapter-backed health read model

Explicit non-goals of that phase:

- no runtime migration
- no generic config abstraction
- no registry-driven write path
- no broad event bus

## Decision Locks

The following are locked by this RFC:

- the top-level taxonomy has three kinds, not four
- PM built-in rules remain built-ins, not agents
- the Phase 1c trigger model is limited to `manual`, `assignment_event`, and `cron`
- `Settings > AI & Automations` is read/governance first
- the next implementation step is inventory/health adapters, not config unification

## Review Questions

This RFC is complete only if future implementers can answer these directly:

- Where does CRM summary refresh belong?
  - `built_in_automation`
- Where does PM epic auto-complete belong?
  - `built_in_automation`, workspace-governed deterministic subtype
- Where do PM/support explicit agents belong?
  - `contextual_agent`
- Does Phase 1c define a general event bus contract?
  - no
- What is the next implementation step?
  - code registry + inventory/health adapters
