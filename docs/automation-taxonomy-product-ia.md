# Automation Taxonomy Product IA Note

This note describes how the automation model appears in the product.

The deeper architecture lives in:

- `docs/automation-taxonomy-rfc.md`
- `docs/automation-taxonomy-seed-catalog.md`
- `docs/automation-taxonomy-migration.md`

## Shared Home

The shared automation home is:

- `Settings > AI & Automations`

It provides:

- inventory
- governance
- diagnostics

Configuration surfaces remain distributed:

- automation rules → Settings > Teams > Workflow pipeline builder
- agents → `/pm/agents`
- PM built-in rules → PM settings automations
- CRM built-ins → CRM domain settings

## Primary Product Groups

The shared home organizes automation into two primary groups:

- `Built-in Automations`
  - CRM system intelligence
  - PM built-in rules
- `Automation Rules`
  - User-configured trigger → action rules
  - Stage-based agent pipelines

## Kind-by-Kind Product Model

| Kind | Visible in | Configured in | Runs on | Output appears in | Who invokes / governs |
|---|---|---|---|---|---|
| `built_in_automation` | `Settings > AI & Automations`, module output | module/domain settings | built-in runtime path | CRM surfaces, PM state/activity changes, diagnostics | system invokes; admins govern |
| `automation_rule` | `Settings > AI & Automations`, pipeline builder | Settings > Teams > Workflow pipeline builder | inline rule engine + agent queues | story state changes, agent runs, branch merges | admins configure; events invoke |

### Agents as executors

Agents are executors, not a taxonomy kind:

| Agent surface | Purpose |
|---|---|
| `/pm/agents` | Create, configure, and manage agents |
| Story detail Delivery block | Assign agent manually, run agent manually |
| Pipeline builder | Assign agent to workflow state via automation rule |
| Kanban board | Bot icon on automated columns |
| Story detail | Pipeline step indicator showing automated stages |

## Product Principles

### Built-ins should feel built in

CRM buyer-signal ingestion, CRM summaries, and PM deterministic rules should feel like product behavior.

Users should understand them as:

- built-in system behavior
- optionally governed by admins

### Automation rules should feel configurable

Stage-based agent pipelines should feel like user-configured workflow automation.

Users should understand them as:

- rules they created to automate their workflow
- agents assigned to pipeline stages
- events that trigger actions

### Pipeline builder is the primary configuration surface

The pipeline builder in Settings > Teams > Workflow dialog is where users configure automation rules. It provides:

- visual horizontal layout of workflow states
- agent assignment dropdowns per state
- auto-advance toggles for approval-based progression
- merge branch inputs for deployment automation

## Current Surface Ownership

- `/pm/agents` — create and configure agents (executors)
- Settings > Teams > Workflow — configure automation rules via pipeline builder
- PM settings automations — PM built-in rules (epic auto-start/complete, sprint scheduling)
- CRM settings — CRM built-in diagnostics

## UI Architecture Principle: Domain-Specific Configuration

`Settings > AI & Automations` is a **read-only dashboard** — it shows inventory, governance, and diagnostics. It does not provide inline CRUD for automation rules.

Configuration surfaces are **domain-specific** because different automation types have fundamentally different config needs (workflow state pickers vs CRM threshold sliders vs email sequence editors). A generic rules form cannot serve all domains well.

Each automation domain owns its configuration surface:

- PM pipeline rules → pipeline builder in Settings > Teams > Workflow dialog
- PM built-in rules → PM settings automations
- CRM built-ins → CRM domain settings
- Future automation types → purpose-built UIs at the relevant settings surface

The backend `automation_rules` engine is generic and extensible. The frontend intentionally is not.

## UX Questions This IA Must Answer

Any UI based on this note should make these answers obvious:

- Is this a built-in automation or a user-configured rule?
- Where do I configure it?
- What events trigger it?
- What agent runs when it fires?
- Where will its output show up?
- Is it enabled or disabled?
- Is it healthy or failing?
