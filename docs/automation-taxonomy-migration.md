# Automation Taxonomy Migration Strategy

This document explains how Teampulse should move from today's split automation systems to the simplified shared taxonomy in `docs/automation-taxonomy-rfc.md`.

Phase 1c is documentation-only. This file defines the first implementation sequence after the docs.

## Current Systems Mapped To The Simplified Model

| Current system | Current storage | Current runtime path | Future top-level kind |
|---|---|---|---|
| CRM buyer-signal ingestion | CRM domain models + sync records | Temporal on `automation-default` | `built_in_automation` |
| CRM contact/deal summaries | `crm_entity_summaries` + CRM domain models | Temporal on `automation-default` | `built_in_automation` |
| PM built-in deterministic automations | `pm_automations` | inline hooks and API ticker | `built_in_automation` |
| PM/support agents | `agents`, `agent_runs`, artifacts | class-specific Temporal queues | `contextual_agent` |

## What Stays Unchanged First

The first implementation phase after this RFC should not change:

- `/pm/agents`
- current agent class restrictions
- current CRM background runtime behavior
- current PM built-in automation config storage
- current PM built-in automation functional behavior

The point is to project the current systems into a shared read model first, not to rewrite them.

## Code vs Data

### Code-defined catalog

Code should own:

- built-in identities
- top-level kind
- target semantics
- runtime mapping defaults
- built-in metadata like:
  - `config_scope`
  - `execution_style`
  - `user_governed`

This keeps built-in identity out of free-form database state.

### Existing config sources remain

The first implementation phase should keep current config sources intact:

- PM agents remain backed by `agents`
- PM built-in automations remain backed by `pm_automations`
- CRM built-ins remain backed by domain-specific settings where applicable

Do **not** introduce a generic shared config abstraction in that first phase.

### Adapter-backed read models

The first implementation phase should add:

- `automation_inventory_view`
- `automation_health_view`

These are projection layers over current systems, not new sources of truth.

## First Implementation Phase After Docs

### 1. Code-defined built-in catalog

Add a canonical code registry for built-ins and known contextual-agent classes.

Expected outcome:

- one catalog entry type
- seeded built-in definitions
- consistent metadata for:
  - built-in scope
  - execution style
  - governance

### 2. Inventory adapters

Add adapters that project current systems into a shared inventory read model.

Adapters should cover:

- CRM built-ins
- PM built-in automations
- contextual agents

Expected outcome:

- a single read API or service can list automation inventory coherently
- no runtime migration is required

### 3. Health adapters

Add adapters that project current health and diagnostics into a shared health read model.

Expected outcome:

- CRM diagnostics become visible through a shared shape
- contextual agent run health becomes visible through a shared shape
- PM built-in automations gain a minimal health representation instead of being effectively log-only

### 4. No shared config abstraction yet

The first implementation phase should deliberately stop before introducing:

- shared config writes
- generic automation settings overlays
- registry-driven mutation APIs

That work should wait until the read models prove useful and the product semantics remain clear.

## What The First Implementation Phase Must Not Do

- no runtime migration
- no generic config abstraction
- no registry-driven write path
- no UI write consolidation
- no broad event bus

## Trigger Normalization Rules

The first implementation phase should normalize only:

- `manual`
- `assignment_event`
- `cron`

Important rules:

- `assignment_event` exists because assignment-triggered auto-run is implemented today
- CRM internal hooks remain module-owned trigger behavior for now
- `auto_on_event` is legacy and should not become a first-class shared trigger in the new model

## Future Phases After The First Implementation

### Phase A: Read-only shared settings home

Add a read-only `Settings > AI & Automations` experience backed by inventory and health views.

Expected outcome:

- shared discovery
- shared governance visibility
- shared diagnostics
- links out to current write surfaces

### Phase B: Selective write overlays

Only after the read models are stable should Teampulse consider selective write overlays for:

- contextual agent governance
- PM built-in automation settings links or wrappers
- CRM diagnostics and narrow built-in controls

### Phase C: Broader trigger/event model

Only later should Teampulse decide whether to generalize:

- `assignment_event` into a broader canonical event catalog
- custom automation creation
- broader cross-module trigger authoring

## Migration Principles

- simplify the mental model before generalizing the platform
- prefer adapters over rewrites first
- keep built-in identity in code
- keep current write surfaces until read models prove useful
- do not introduce new abstractions earlier than necessary

## Readiness Questions

This migration spec is complete only if future implementers can answer:

- what top-level kind each current system maps to
- what stays unchanged first
- what belongs in code vs existing config stores
- what the first implementation milestone actually ships
- why PM built-in rules are kept inside built-ins instead of becoming a separate foundational kind
