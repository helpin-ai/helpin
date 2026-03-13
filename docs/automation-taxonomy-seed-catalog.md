# Automation Taxonomy Seed Catalog

This appendix defines the initial seed entries for the simplified automation taxonomy in `docs/automation-taxonomy-rfc.md`.

The purpose is clarity, not exhaustiveness. It shows how current CRM automations, PM built-in rules, and contextual agents fit into the three-kind model.

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
- `future_ai_automations_group`
- `runtime_path`

This appendix intentionally does **not** try to define a generic config schema yet.

## Seed Entries

| ID | Kind | Group | Target types | Current trigger shape | Config scope | Execution style | User governed | Current write surface | Future AI & Automations group | Runtime path |
|---|---|---|---|---|---|---|---|---|---|---|
| `crm.buyer_signal_ingestion` | `built_in_automation` | `crm_system_intelligence` | `crm_email_message` | `internal_domain_hook` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `crm.contact_summary_refresh` | `built_in_automation` | `crm_system_intelligence` | `contact` | `internal_domain_hook + cron` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `crm.deal_summary_refresh` | `built_in_automation` | `crm_system_intelligence` | `deal` | `internal_domain_hook + cron` | `system` | `background_workflow` | `false` | `none` | `Built-in Automations > CRM system intelligence` | `automation-default` |
| `pm.epic_auto_start` | `built_in_automation` | `pm_built_in_rules` | `epic` | `internal_domain_hook` | `workspace` | `deterministic_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `inline request path` |
| `pm.epic_auto_complete` | `built_in_automation` | `pm_built_in_rules` | `epic` | `internal_domain_hook` | `workspace` | `deterministic_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `inline request path` |
| `pm.sprint_auto_create` | `built_in_automation` | `pm_built_in_rules` | `team`, `sprint` | `cron` | `team` | `scheduled_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `hourly API ticker` |
| `pm.sprint_move_unfinished` | `built_in_automation` | `pm_built_in_rules` | `team`, `sprint`, `story` | `cron` | `team` | `scheduled_rule` | `true` | `settings.pm_automations` | `Built-in Automations > PM built-in rules` | `hourly API ticker` |
| `pm.product_planner` | `contextual_agent` | `pm_agents` | `epic` | `manual` | `workspace` | `interactive_agent` | `true` | `/pm/agents` | `Contextual Agents` | `agent-planner` |
| `pm.engineer` | `contextual_agent` | `pm_agents` | `story` | `manual + assignment_event` | `workspace` | `interactive_agent` | `true` | `/pm/agents` | `Contextual Agents` | `agent-engineer` |
| `pm.reviewer` | `contextual_agent` | `pm_agents` | `story` | `manual + assignment_event` | `workspace` | `interactive_agent` | `true` | `/pm/agents` | `Contextual Agents` | `agent-reviewer` |
| `support.support_agent` | `contextual_agent` | `support_agents` | `support_conversation` | `manual` | `workspace` | `interactive_agent` | `true` | `/pm/agents` | `Contextual Agents` | `agent-support` |

## Notes

### CRM built-ins

- These are system-owned built-ins
- They are not user-creatable
- They should never appear as PM-style agents
- Their future shared-settings representation should be diagnostics/governance first

### PM built-in rules

- These are built-ins, not a separate top-level automation kind
- They are deterministic and workspace-governed
- They should be presented as PM built-in rules inside the broader built-in automation family
- They should not inherit agent concepts like persona, handoff, or artifact history

### Contextual agents

- These remain explicit and contextual
- `assignment_event` is standardized because assignment-based auto-run actually exists today
- `auto_on_event` is not represented here as a stable control-plane trigger because it is not backed by a mature shared event catalog

## Product Interpretation Rules

This appendix should be read with these product rules in mind:

- PM built-in rules are still built-ins even when they are workspace-configured
- CRM background intelligence is still built-in even when users never invoke it directly
- only `manual`, `assignment_event`, and `cron` are normalized shared trigger shapes in this phase
- `internal_domain_hook` describes current runtime reality for some built-ins; it is not a claim that a shared event contract exists yet

## Seed Validation Rules

Future implementation should preserve these rules:

- built-ins may be system-governed or workspace-governed, but they remain one top-level kind
- contextual agents must keep a clear contextual run surface
- only `manual`, `assignment_event`, and `cron` are standardized trigger shapes in this phase
- inventory and health adapters should project these entries without forcing runtime migration
