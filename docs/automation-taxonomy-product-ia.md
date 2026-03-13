# Automation Taxonomy Product IA Note

This note describes how the simplified automation model should appear in the product once it becomes user-visible.

The deeper architecture lives in:

- `docs/automation-taxonomy-rfc.md`
- `docs/automation-taxonomy-seed-catalog.md`
- `docs/automation-taxonomy-migration.md`

## Future Home

The future shared home is:

- `Settings > AI & Automations`

That home should begin as:

- inventory
- governance
- diagnostics

It should not begin as the canonical write surface for all automation.

Near-term implication:

- shared Settings is the place to understand automation
- existing module surfaces remain the place to change behavior

## Primary Product Groups

The future shared home should organize automation into three primary groups:

- `Built-in Automations`
- `Contextual Agents`
- `Custom Automations`

Optional grouping inside `Built-in Automations`:

- CRM system intelligence
- PM built-in rules

That gives users a simpler story than teaching a separate top-level “rule automation” category.

## Kind-by-Kind Product Model

| Kind | Visible in | Configured in today | Runs on | Output appears in | Who invokes / governs |
|---|---|---|---|---|---|
| `built_in_automation` | future `Settings > AI & Automations`, plus indirect module output | current module/domain settings where applicable | current built-in runtime path | CRM surfaces, PM state/activity changes, future diagnostics | system invokes; admins govern where applicable |
| `contextual_agent` | `/pm/agents`, target detail surfaces, future shared inventory | `/pm/agents` and current contextual surfaces | class-specific agent queues | run panels, artifacts, target detail pages, support inbox | users run from context; admins govern setup |
| `custom_automation` | future shared inventory | future dedicated builder/config UI | future shared automation lanes | module-specific outcomes and future diagnostics | future admin/builder-managed |

## Product Principles

### Built-ins should feel built in

CRM buyer-signal ingestion, CRM summaries, and PM deterministic rules should feel like product behavior.

Users should understand them as:

- built-in system behavior
- optionally governed by admins

Not as:

- agents they assigned
- a separate platform category they must learn

### Contextual agents stay explicit

PM/support agents remain valuable because they are:

- explicit
- contextual
- artifact-producing

The shared settings home should list and govern them, but their primary execution surfaces should remain contextual.

### Shared settings is inventory first

`Settings > AI & Automations` should first answer:

- what exists
- what is enabled
- what is failing
- where to configure or run it

That is enough value before a unified write model exists.

## Near-Term Surface Ownership

Until a later consolidation phase:

- `/pm/agents` remains the write and run surface for contextual agents
- current PM settings remain the write surface for PM built-in rules
- CRM built-ins remain governed through CRM-domain settings and diagnostics where applicable

This split is intentional. The shared home should make it legible before it tries to replace it.

## Current vs Future Surface Rules

### Current state

Keep:

- PM agents in `/pm/agents`
- PM built-in automation settings in current PM settings
- CRM built-ins implicit in CRM module behavior and diagnostics

### Future state

Later add:

- a shared inventory and diagnostics home in Settings
- subgrouping for built-ins
- links out to current write surfaces

Only later consider selective write overlays.

## UX Questions This IA Must Answer

Any future UI based on this note should make these answers obvious:

- Is this built in, an explicit agent, or a future custom automation?
- Where do I inspect its health?
- Where do I configure it today?
- Can I run it manually?
- Where will its output show up?
