# Automation Taxonomy Migration Strategy

This document explains how Teampulse moved from the original split automation systems to the current two-kind taxonomy in `docs/automation-taxonomy-rfc.md`.

## Current Systems Mapped To The Taxonomy

| Current system | Current storage | Current runtime path | Top-level kind |
|---|---|---|---|
| CRM buyer-signal ingestion | CRM domain models + sync records | Temporal on `automation-default` | `built_in_automation` |
| CRM contact/deal summaries | `crm_entity_summaries` + CRM domain models | Temporal on `automation-default` | `built_in_automation` |
| PM built-in deterministic automations | `pm_automations` | inline hooks and API ticker | `built_in_automation` |
| PM/support agents | `agents`, `agent_runs`, artifacts | class-specific Temporal queues | executors (not a taxonomy kind) |
| Automation rules (pipeline) | `automation_rules` | inline rule engine + agent queues | `automation_rule` |

## Taxonomy Evolution: 3 Kinds → 2 Kinds

The original Phase 1c taxonomy had three kinds:

| Original kind | Current status | Rationale |
|---|---|---|
| `built_in_automation` | **Kept** | CRM intelligence, epic aggregation rules, sprint scheduling |
| `contextual_agent` | **Dropped as taxonomy kind** | Agents became executors — the "action" in a rule. Still configured at `/pm/agents` |
| `custom_automation` | **Replaced by `automation_rule`** | User-configured rules with trigger → action. Pipeline is the first implementation |

## What Changed

### New: Automation Rules Engine

- `automation_rules` table stores user-configured trigger → action rules
- `AutomationRuleEngine` service evaluates events and executes actions
- Rules are evaluated synchronously in `MoveToState`, `Create`, and `ApproveRun`
- Chaining with four-layer loop prevention

### New: Stage-Based Agent Pipeline

- Agents are assigned to workflow states via `story.state_entered` + `run_agent` rules
- Auto-advance via `agent_run.approved` + `move_to_state` rules
- Merge branch via `story.state_entered` + `merge_branch` rules
- Visual pipeline builder in Settings > Teams > Workflow dialog

### Unchanged

- `/pm/agents` remains the write and run surface for agents
- agent class restrictions unchanged
- CRM background runtime behavior unchanged
- PM built-in automation config storage unchanged
- PM built-in automation functional behavior unchanged

## Code vs Data

### Code-defined catalog

Code owns:

- built-in identities
- top-level kind
- target semantics
- runtime mapping defaults
- built-in metadata like:
  - `config_scope`
  - `execution_style`
  - `user_governed`

This keeps built-in identity out of free-form database state.

### Config sources

- PM agents remain backed by `agents` table
- PM built-in automations remain backed by `pm_automations` table
- CRM built-ins remain backed by domain-specific settings
- Automation rules are backed by `automation_rules` table (user-created, not code-defined)

### Adapter-backed read models

Inventory and health read models aggregate across all systems:

- `automation_inventory_view`
- `automation_health_view`

These are projection layers over current systems, not new sources of truth.

## Implemented Milestones

### Milestone 1: Code-defined built-in catalog

- `server/internal/automationcatalog/registry.go`
- Seeded built-in definitions with consistent metadata

### Milestone 2: Inventory + health adapters

- `server/internal/service/automation_inventory.go`
- `server/internal/repository/automation_health.go`
- Read-only `Settings > AI & Automations` page

### Milestone 3: Automation rules engine

- `server/internal/model/automation_rule.go` — model + DTOs
- `server/internal/repository/automation_rule.go` — CRUD + rule matching
- `server/internal/service/automation_rule_engine.go` — event evaluation + action executors
- `server/internal/handler/automation_rule.go` — HTTP endpoints
- `server/migrations/042_automation_rules.sql` — table + indexes
- Integration hooks in `PMStoryService` and `AgentService`

### Milestone 4: Frontend pipeline UI

- `frontend/src/lib/services/automationRuleService.ts` — API client
- `frontend/src/components/settings/PipelineBuilder.tsx` — visual pipeline editor
- Pipeline builder integrated into Settings > Teams > Workflow dialog
- Kanban board bot icons, story detail pipeline indicator

## Trigger Types

Current standardized triggers:

- `manual` — human-initiated agent runs
- `assignment_event` — auto-run on agent assignment
- `cron` — scheduled reconciliation sweeps
- `story.state_entered` — automation rule trigger on workflow state entry
- `agent_run.approved` — automation rule trigger on run approval
- `internal_domain_hook` — CRM/PM built-in runtime hooks (not a shared event contract)

## Future Evolution

Potential future trigger types can be added without restructuring:

- `story.created` — trigger on story creation
- `pr.merged` — trigger on PR merge
- `label.changed` — trigger on label changes
- `story.unblocked` — trigger when all blockers are resolved

## Migration Principles

- simplify the mental model before generalizing the platform
- prefer adapters over rewrites first
- keep built-in identity in code
- automation rules are user-created, stored in database
- agents are executors, not a taxonomy kind

## Readiness Questions

This migration spec is complete if future implementers can answer:

- what top-level kind each current system maps to → `built_in_automation` or `automation_rule`
- where agents fit → executors referenced by `run_agent` actions, not a taxonomy kind
- what the pipeline builder configures → `automation_rules` rows with trigger/action configs
- why PM built-in rules are kept inside built-ins → they operate on epic/sprint entities, not story workflow states
