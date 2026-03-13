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

The implementation follows the approved simplified model:

- `built_in_automation`
  - CRM buyer signal ingestion
  - CRM contact summary refresh
  - CRM deal summary refresh
  - PM epic/sprint deterministic rules
- `contextual_agent`
  - PM planner
  - PM engineer
  - PM reviewer
  - support agent
- `custom_automation`
  - represented only as a placeholder group in this phase

PM rules are **not** a separate top-level kind. They are built-in automations with workspace/team governance metadata.

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
- `internal_domain_hook`

The first three are standardized shared trigger shapes. `internal_domain_hook` describes current runtime reality for CRM and PM built-ins where execution is triggered by module-internal events (e.g., email sync completion, story state change). It is not a claim that a shared event contract exists yet.

There is still **no** generic shared event catalog in this phase.

## Current UI behavior

The new settings page is intentionally read-only.

It shows:

- `Built-in Automations`
- `Contextual Agents`
- `Custom Automations`

For each real workspace item it shows:

- title and kind
- module/group
- scope
- trigger modes
- enabled state
- health status
- last success / last error
- output surface
- current write/run surface

The page links out to the current source-of-truth write surfaces instead of trying to replace them:

- PM agents -> `/pm/agents`
- PM built-ins -> PM settings automations
- CRM built-ins -> CRM settings email/accounts surfaces

## What this phase explicitly does not do

- no runtime migration
- no shared config write model
- no generic event bus
- no custom automation builder
- no replacement of PM agents or PM settings as write surfaces

## Main files

Backend:

- `server/internal/automationcatalog/registry.go`
- `server/internal/model/automation.go`
- `server/internal/repository/automation_health.go`
- `server/internal/service/automation_inventory.go`
- `server/internal/handler/settings.go`

Frontend:

- `frontend/src/components/settings/AIAutomationsTab.tsx`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/components/layout/Sidebar.tsx`
- `frontend/src/lib/services/settingsService.ts`
- `frontend/src/hooks/queries/useSettings.ts`

## Next likely phase

The next implementation step after this inventory slice should be:

- code-registry refinements if needed
- adapter-backed inventory/health improvements
- optional diagnostics deep links

It should **not** jump straight to a shared write surface or a generic automation builder.
