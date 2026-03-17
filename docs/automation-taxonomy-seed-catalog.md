# Automation Taxonomy Seed Catalog

This appendix defines the seed entries for the automation taxonomy in `docs/automation-taxonomy-rfc.md`.

The purpose is clarity, not exhaustiveness. It shows how current CRM automations, PM built-in rules, and automation rules fit into the two-kind model.

## Taxonomy Evolution

The original Phase 1c catalog had three kinds (`built_in_automation`, `contextual_agent`, `custom_automation`). With the automation rules engine, this was simplified to two kinds:

- `contextual_agent` → **dropped as taxonomy kind**. Agents remain as executors referenced by `run_agent` actions, configured at `/pm/agents`.
- `custom_automation` → **replaced by `automation_rule`**. User-configured trigger → action rules stored in `automation_rules` table.

## Seed Entry Shape

The seed appendix focuses on the metadata that matters for the simplified model:

- `id`
- `kind`
- `group`
- `target_types`
- `current_trigger_shape`
- `config_scope`
- `execution_style`
- `user_governed`
- `current_write_surface`
- `ai_automations_group`
- `runtime_path`

## Seed Entries

### Built-in Automations

| ID | Kind | Group | Target types | Current trigger shape | Config scope | Execution style | User governed | Current write surface | AI & Automations group | Runtime path |
|---|---|---|---|---|---|---|---|---|---|---|
| `crm.buyer_signal_ingestion` | `built_in_automation` | `crm_system_intelligence` | `crm_email_message` | `internal_domain_hook` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `crm.contact_summary_refresh` | `built_in_automation` | `crm_system_intelligence` | `contact` | `internal_domain_hook + cron` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `crm.deal_summary_refresh` | `built_in_automation` | `crm_system_intelligence` | `deal` | `internal_domain_hook + cron` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `pm.epic_auto_start` | `built_in_automation` | `pm_built_in_rules` | `epic` | `internal_domain_hook` | `workspace` | `deterministic_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `inline request path` |
| `pm.epic_auto_complete` | `built_in_automation` | `pm_built_in_rules` | `epic` | `internal_domain_hook` | `workspace` | `deterministic_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `inline request path` |
| `pm.sprint_auto_create` | `built_in_automation` | `pm_built_in_rules` | `team`, `sprint` | `cron` | `team` | `scheduled_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `hourly API ticker` |
| `pm.sprint_move_unfinished` | `built_in_automation` | `pm_built_in_rules` | `team`, `sprint`, `story` | `cron` | `team` | `scheduled_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `hourly API ticker` |

### Automation Rules

| ID | Kind | Group | Target types | Current trigger shape | Config scope | Execution style | User governed | Current write surface | AI & Automations group | Runtime path |
|---|---|---|---|---|---|---|---|---|---|---|
| `automation_rule` | `automation_rule` | `workflow_automation_rules` | `story` | `story.state_entered`, `agent_run.approved` | `workspace` | `event_driven_rule` | `true` | `settings.teams.workflow` | `Automation Rules` | `inline request path + agent queues` |

### Agents (Executors, not a taxonomy kind)

Agents are no longer a top-level automation kind. They are executors referenced by `run_agent` actions in automation rules. For reference:

| Agent class | Target types | Trigger modes | Write surface | Runtime queue |
|---|---|---|---|---|
| `product_planner` | `epic` | `manual` | `/pm/agents` | `agent-planner` |
| `engineer` | `story` | `manual`, `assignment_event`, `automation_rule` | `/pm/agents` | `agent-engineer` |
| `reviewer` | `story` | `manual`, `assignment_event`, `automation_rule` | `/pm/agents` | `agent-reviewer` |
| `support` | `support_conversation` | `manual` | `/pm/agents` | `agent-support` |

Note: `automation_rule` trigger mode means the agent can be invoked automatically by a `run_agent` action in an automation rule.

## Notes

### CRM built-ins

- These are system-owned built-ins
- They are not user-creatable
- They should never appear as PM-style agents
- Their shared-settings representation is diagnostics/governance

### PM built-in rules

- These are built-ins, not automation rules
- They are deterministic and workspace-governed
- They operate on epic/sprint entities, not story workflow states
- They should not inherit agent concepts like persona, handoff, or artifact history

### Automation rules

- User-created and user-managed
- Stored in `automation_rules` table
- Configured via pipeline builder in Settings > Teams > Workflow dialog
- Support chaining with loop prevention (depth counter, same-rule dedup, state guard, active run guard)
- Primary use case: stage-based agent pipelines

## Product Interpretation Rules

This appendix should be read with these product rules in mind:

- PM built-in rules are still built-ins even when they are workspace-configured
- CRM background intelligence is still built-in even when users never invoke it directly
- agents are executors, not a taxonomy kind — they are configured at `/pm/agents` and referenced by `run_agent` actions
- `story.state_entered` and `agent_run.approved` are event triggers for automation rules
- `manual`, `assignment_event`, and `cron` remain trigger shapes for built-ins and agents
- `internal_domain_hook` describes current runtime reality for some built-ins

## Seed Validation Rules

Future implementation should preserve these rules:

- built-ins may be system-governed or workspace-governed, but they remain one top-level kind
- automation rules are user-created and event-driven
- agents are executors that retain their own lifecycle (runs, approvals, handoffs, artifacts)
- inventory and health adapters should project these entries without forcing runtime migration
