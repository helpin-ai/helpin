# AI & Automations Inventory

This document describes the first implemented shared automation control-plane slice introduced in Phase 1c.

## What shipped

The product now has a read-only `Settings > AI & Automations` governance surface backed by a shared backend inventory API:

- `GET /api/settings/ai-automations?workspace_id=...`

This phase does **not** change automation runtime behavior or replace existing write surfaces. It adds:

- a code-defined automation catalog
- adapter-backed workspace inventory assembly
- shared built-in automation health snapshots
- a read-only settings page that links back to the existing write/run surfaces

## Current taxonomy

The implementation follows the two-kind model:

- `built_in_automation`
  - CRM buyer signal ingestion
  - CRM contact summary refresh
  - CRM deal summary refresh
  - PM epic/sprint deterministic rules
- `automation_rule`
  - User-configured trigger → action rules
  - Stage-based agent pipelines

PM rules are **not** automation rules. They are built-in automations with workspace/team governance metadata that operate on epic/sprint entities.

Agents (`product_planner`, `engineer`, `reviewer`, `support`) are executors referenced by `run_agent` actions in automation rules, not a separate taxonomy kind. They are still configured at `/pm/agents`.

## Architecture

### 1. Code-defined catalog

Canonical automation identities live in:

- `server/internal/automationcatalog/registry.go`

This registry is the source of truth for:

- automation ID
- kind
- module
- grouping
- trigger modes
- queue/runtime intent
- current write/run surfaces
- output surface

No database table is used as a free-form automation registry.

### 2. Inventory read model

Workspace inventory is assembled dynamically in:

- `server/internal/service/automation_inventory.go`

It combines:

- catalog entries
- CRM mailbox state
- PM automation configs
- workspace teams
- explicit agent records
- health snapshots
- derived agent runner health

Inventory is **not persisted** as a table.

Stable inventory IDs:

- workspace built-ins: `<catalog_id>:workspace`
- team built-ins: `<catalog_id>:team:<team_id>`
- contextual agents: `<catalog_id>:agent:<agent_id>`

### 3. Health persistence

Built-in automation health is persisted in:

- `automation_health_snapshots`

Model and repository:

- `server/internal/model/automation.go`
- `server/internal/repository/automation_health.go`

The shared table is health-only. It does not store configuration.

Unique scope key:

- `(workspace_id, catalog_id, scope_type, scope_id)`

Tracked fields:

- `status`
- `last_seen_at`
- `last_success_at`
- `last_error_at`
- `last_error_message`
- `metrics`

### 4. Health writers

Health observations are currently emitted from:

- CRM buyer signal detection Temporal activities
- CRM summary refresh Temporal activities
- PM epic automation evaluation
- PM sprint automation sweeps

Contextual agents do **not** write into `automation_health_snapshots`.

Their health is derived at read time from:

- agent records
- recent agent runs
- current runner health

## Trigger model in the implementation

This implementation uses the following trigger shapes:

- `manual`
- `assignment_event`
- `cron`
- `story.state_entered`
- `agent_run.approved`
- `internal_domain_hook`

The first three are standardized shared trigger shapes for agents and built-ins. `story.state_entered` and `agent_run.approved` are event-driven triggers for automation rules. `internal_domain_hook` describes current runtime reality for CRM and PM built-ins where execution is triggered by module-internal events (e.g., email sync completion, story state change).

The automation rules engine provides purpose-built event triggers, but there is no generic cross-app event bus.

## Current UI behavior

The settings page is a **read-only dashboard** that shows two primary groups:

- `Built-in Automations`
- `Automation Rules`

For each item it shows:

- title and kind
- module/group
- scope
- trigger modes
- enabled state
- health status
- last success / last error
- output surface
- current write/run surface

The page links out to the source-of-truth write/config surfaces:

- PM agents -> `/pm/agents`
- PM built-ins -> PM settings automations
- CRM built-ins -> CRM settings email/accounts surfaces
- Automation rules -> Settings > Teams > Workflow pipeline builder

**Design decision**: The AI & Automations page intentionally does NOT provide inline CRUD for automation rules. Configuration surfaces are domain-specific — the pipeline builder for PM workflow automation, CRM settings for signal automation, etc. A generic rules management UI was evaluated and rejected because different automation domains have fundamentally different configuration needs that a single form cannot serve well. See `docs/automation-taxonomy-rfc.md` for the full rationale.

### Pipeline builder

The primary configuration surface for automation rules is the pipeline builder, embedded in the Settings > Teams > Workflow dialog. It shows:

- workflow states laid out left-to-right
- agent assignment dropdowns per state (creates `story.state_entered` + `run_agent` rules)
- auto-advance toggles (creates `agent_run.approved` + `move_to_state` rules)
- merge branch inputs (creates `story.state_entered` + `merge_branch` rules)

### Kanban board indicators

States with `run_agent` rules show a bot icon in column headers. Stories with assigned agents show a bot badge on cards.

### Story detail pipeline indicator

When a workflow has automation rules, the story detail panel shows a horizontal pipeline step indicator with completed, current, and pending stages.

## Main files

Backend:

- `server/internal/automationcatalog/registry.go`
- `server/internal/model/automation.go`
- `server/internal/model/automation_rule.go`
- `server/internal/repository/automation_health.go`
- `server/internal/repository/automation_rule.go`
- `server/internal/service/automation_inventory.go`
- `server/internal/service/automation_rule_engine.go`
- `server/internal/handler/settings.go`
- `server/internal/handler/automation_rule.go`
- `server/migrations/042_automation_rules.sql`

Frontend:

- `frontend/src/components/settings/AIAutomationsTab.tsx`
- `frontend/src/components/settings/PipelineBuilder.tsx`
- `frontend/src/components/settings/TeamsTab.tsx`
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/components/pm/StoryDetailPanel.tsx`
- `frontend/src/components/pm/StoryCard.tsx`
- `frontend/src/components/pm/StoryListView.tsx`
- `frontend/src/lib/services/automationRuleService.ts`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/components/layout/Sidebar.tsx`
- `frontend/src/lib/services/settingsService.ts`
- `frontend/src/hooks/queries/useSettings.ts`
