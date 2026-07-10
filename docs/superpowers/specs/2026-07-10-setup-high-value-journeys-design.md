# High-value Setup Journeys

**Date:** 2026-07-10
**Status:** Approved direction

## Goal

Replace placeholder and low-value activity milestones with setup capabilities and verified business outcomes that help customers adopt more of Helpin. Generic repetition is never a milestone. Repetition qualifies only when it proves unattended system reliability.

## Customer-facing presentation

- Remove the separate `Goals saved for what’s next` block.
- Every selected goal renders as a normal collapsible journey in goal order.
- Remove journey labels `Preparing`, `Ready`, `Activated`, `Established`, `Advanced`, and `Power up` from the UI.
- Remove task-stage pills `Prepare`, `Activate`, `Repeat`, `Connect`, and `Prove value` from the UI.
- Retain maturity, stage, scope, core, and evidence data internally for ordering, progress, recommendations, and analytics.
- Journey headers show only accent, title, description, verified completion count, and collapse control.
- Task rows show only the one-sentence action-and-reason label, personal/blocked status when applicable, completion state, and action.
- Keep the existing first-expanded/all-later-collapsed behavior.
- A genuine future placeholder may use the normal journey shell with `Not available yet`, no progress fraction, and no tasks. None of the currently selectable goals require this state.

## Low-value milestone removal

Remove these catalog tasks and all prerequisites, core/readiness membership, recommendations, and achievement writes that refer to them:

| Removed task | Reason |
|---|---|
| `foundation.invite_sent` | Sending an invitation is activity; an active teammate proves collaboration. |
| `product.initial_work` | Creating an isolated task does not prove a team project is configured. |
| `product.first_task_completed` | A single task completion is subsumed by executing and closing a sprint. |
| `product.repeat_completion` | Generic repetition encourages checklist gaming. |
| `automation.target_ready` | “Choose real work” is vague and not a product capability. |
| `automation.repeat_assisted_value` | Another manual run does not add a new capability. |
| `automation.personal_contribution` | Duplicates the workspace agent-run outcome. |
| `automation.personal_repeat_contribution` | Duplicates manual repetition and personalizes a shared journey. |
| `automation.approval_resolved` | Resolving one approval is activity; configuring safeguards is the durable capability. |

Old durable achievement rows may remain in storage for analytics, but they are ignored by the catalog and never rendered.

## Final journeys

All labels follow the established one-sentence “what to do + why it matters” rule.

### Workspace essentials

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `foundation.company_context_ready` | Add company details so Helpin understands your business. | Workspace `company_product_context` is non-empty. | `workspace_context` → `/settings/general` |
| 2 | `foundation.team_ready` | Create a team so work has a clear owner. | At least one `workspace_teams` row. | `workspace_teams` → `/settings/teams` |
| 3 | `foundation.member_joined` | Have a teammate join so progress can be shared. | More than one active `workspace_members` row. | `workspace_members` → `/settings/members` |

Company context is the prerequisite for teammate participation. Team creation is applicable when a selected journey uses team-owned work. Do not require an invitation record.

### Plan and ship team projects

`team_project_management` and the existing `product_delivery` goal resolve to this single journey. Goal normalization deduplicates the aliases so it is never rendered twice. Existing stored `team_project_management` goals remain valid and migrate logically at read/update time without destructive data migration.

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `product.project_planned` | Create an epic with an owner and timeline so the team knows what it is delivering. | A non-archived `pm_epics` row has an owner (`owner_member_id` or legacy `owner_id`), `planned_start_date`, and `deadline`. | `pm_epics` → `/pm/epics` |
| 2 | `product.sprint_planned` | Create a sprint with planned work so the team can execute a clear delivery cycle. | A non-archived `pm_sprints` row has start/end dates and at least one non-archived `pm_tasks` row referencing its `sprint_id`. | `pm_sprints` → `/pm/sprints` |
| 3 | `product.work_assigned` | Assign project work to teammates so every task has clear ownership. | A non-archived task in an epic or sprint has at least one `pm_task_owners` row. | `pm_tasks` → `/pm/tasks` |
| 4 | `product.sprint_closeout_reviewable` | Close a sprint so the team can review what shipped and improve the next cycle. | At least one `pm_sprint_closeouts` row. | `pm_sprints` → `/pm/sprints` |
| 5 | `product.repository_ready` | Connect a code repository so Helpin can link project work to what you ship. | An active, selected, non-deleted `git_repositories` row. | `git_settings` → `/settings/repositories` |
| 6 | `product.agent_result_used` | Run an agent on project work so planning or review takes less manual effort. | A valuable completed agent run targets `task`, `epic`, or `repository`. This is workspace-shared, not member-specific. | `automation_agents` → `/automation/agents` |
| 7 | `product.release_notes_flow_succeeded` | Run release-notes automation so customer updates are created from shipped work. | A completed trigger execution for the `release_notes_writer` template. | `automation_flows` → `/automation/flows` |

Core progress uses tasks 1–4. Repository, agent, and release-notes steps are power capabilities. Dependencies: sprint planned requires project planned; work assigned and sprint closeout require sprint planned; release notes requires repository connected.

### Scale customer support

Keep the approved nine support tasks and evidence unchanged:

1. `support.email_inbox_connected`
2. `support.live_chat_installed`
3. `support.help_docs_ready`
4. `support.brand_knowledge_ready`
5. `support.ai_agent_activated`
6. `support.team_inbox_created`
7. `support.routing_enabled`
8. `support.pm_task_linked`
9. `support.coverage_fix_applied`

When `help_center_docs` is also selected, omit `support.help_docs_ready` from the support journey. The detailed help-center journey owns that setup capability and supplies the same underlying evidence.

### Publish help center docs

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `help_center.space_ready` | Create a public help-center space so customer content has a clear home. | A non-deleted, non-system `docs_spaces` row has `type = 'external_capable'`. | `docs_home` → `/docs` |
| 2 | `help_center.content_ready` | Create or import customer-facing articles so common questions are documented. | A non-deleted `docs_documents` row belongs to an external-capable space. | `docs_home` → `/docs` |
| 3 | `help_center.article_published` | Publish an article so customers can use it to solve a real question. | A joined `docs_helpcenter_articles` row has `public_published_at IS NOT NULL`. | `docs_home` → `/docs` |
| 4 | `help_center.site_published` | Publish your help center so customers can browse and search your documentation. | The workspace `docs_helpcenter_configs.is_published` value is true. | `help_center_settings` → `/settings/helpcenter` |
| 5 | `help_center.widget_connected` | Add help-center content to live chat so customers can find answers before starting a conversation. | An active support widget installation has a non-empty `widget_help_space_ids` setting containing an existing external-capable space. | `support_live_chat` → `/settings/chat-general` |

Tasks 1–4 are core. Widget integration is a power capability. Dependencies follow the displayed order, except widget integration requires article and site publication.

### Build internal knowledge

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `internal_docs.space_ready` | Create an internal knowledge space so company information has a trusted home. | A non-deleted, non-system `docs_spaces` row has `type = 'internal'`. | `docs_home` → `/docs` |
| 2 | `internal_docs.content_ready` | Create or import internal documents so teammates can find essential information. | A non-deleted `docs_documents` row belongs to an internal space. | `docs_home` → `/docs` |
| 3 | `internal_docs.published` | Publish internal knowledge so it is available across the workspace. | An internal-space document has `status = 'published'` and `published_at IS NOT NULL`. | `docs_home` → `/docs` |
| 4 | `internal_docs.ownership_ready` | Assign document owners and review dates so important knowledge stays current. | An internal-space document has `owner_id IS NOT NULL` and `next_review_at IS NOT NULL`. | `docs_home` → `/docs` |
| 5 | `internal_docs.agent_connected` | Connect internal knowledge to an AI agent so it can answer with trusted company context. | A ready `agent_knowledge_sources` row with indexed content references an internal space. | `automation_agents` → `/automation/agents` |
| 6 | `internal_docs.agent_succeeded` | Run the documentation agent on a real document so maintaining knowledge takes less manual effort. | A valuable completed `agent_runs` row targets `document` and joins an agent with `preset_key = 'documentation_agent'`. | `automation_agents` → `/automation/agents` |

Tasks 1–4 are core. Agent connection and successful document run are power capabilities. Dependencies follow the displayed order; the agent run requires agent-connected knowledge.

### Build a sales pipeline

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `crm.customer_data_ready` | Add or import contacts and companies so your team has customer context in one place. | At least one `crm_contacts` row and one `crm_companies` row exist for the workspace. | `crm_contacts` → `/crm/contacts` |
| 2 | `crm.pipeline_ready` | Configure pipeline stages so every opportunity follows a consistent sales process. | A `crm_pipelines` row has at least one open, one won, and one lost `crm_pipeline_stages` row. | `crm_pipelines` → `/settings/crm-pipelines` |
| 3 | `crm.deal_ready` | Create a deal with an owner, value, and close date so the opportunity is actionable. | A `crm_deals` row has `owner_member_id`, positive `amount`, and `close_date`. | `crm_deals` → `/crm/deals` |
| 4 | `crm.email_connected` | Connect your sales inbox so Helpin can capture customer conversations and buying signals. | An active `crm_email_accounts` row has `status = 'connected'`. | `crm_email` → `/settings/crm-email` |
| 5 | `crm.autonomy_enabled` | Enable CRM automation so Helpin can create or progress deals from strong customer signals. | `crm_autonomy_settings.enabled` is true and either `auto_create_deals` or `auto_progress_deals` is true. | `crm_autonomy` → `/settings/crm-autonomy` |
| 6 | `crm.signal_value_proven` | Create or progress a deal from a detected customer signal so the pipeline updates itself. | An accepted `crm_suggestions` row has type `deal_create` or `deal_advance`; automatic executions already store accepted suggestions, so manual and automatic success share one predicate. | `crm_review` → `/crm/review` |

Tasks 1–3 are core. Email, autonomy, and signal-driven value are power capabilities. Dependencies: deal requires customer data and pipeline; signal value requires connected email and enabled autonomy.

### Automate repeatable work

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `automation.first_assisted_value` | Complete an agent run on real work so you can see where Helpin saves time. | A valuable completed agent run with persisted output or artifact. | `automation_agents` → `/automation/agents` |
| 2 | `automation.custom_agent_succeeded` | Run a custom agent successfully so it can handle work specific to your team. | A valuable completed run joins an agent with `is_system = false`. | `automation_agents` → `/automation/agents` |
| 3 | `automation.flow_enabled` | Turn on an automation flow so repeat work can run automatically. | An enabled `automation_rules` row. | `automation_flows` → `/automation/flows` |
| 4 | `automation.approval_guard_configured` | Require approval for sensitive agent actions so automation stays under human control. | A non-system agent has effective `approval_mode = 'always'`. | `automation_agents` → `/automation/agents` |
| 5 | `automation.triggered_value` | Complete a triggered or scheduled automation so value no longer depends on a manual start. | A completed `agent_trigger_executions` row. | `automation_flows` → `/automation/flows` |
| 6 | `automation.reliable_unattended_value` | Run the same automation successfully over time so your team can trust it unattended. | The same enabled rule completes successfully on three distinct local days and has no newer failure. | `automation_flows` → `/automation/flows` |

Tasks 1, 3, and 5 are core. Custom agents, approval safeguards, and demonstrated reliability are power capabilities. Dependencies: custom agent and approval safeguard require the first useful run; triggered success requires an enabled flow; reliability requires triggered success.

## Goal selection and ordering

- Foundation is always first.
- Selected real journeys follow the customer’s saved goal order.
- Automation remains a featured final journey when not selected, but the UI does not label it `Power up`.
- Normalize `team_project_management` to `product_delivery` and remove the duplicate goal option from the selector. Rename the option and journey to `Plan and ship team projects`.
- Remove the placeholder catalog and placeholder-goal response section once all four current goals are real.
- Preserve the maximum of three selected non-foundation goals.

## Permissions and unavailable actions

- PM actions require the existing PM edit/integration permissions.
- Docs create/edit tasks require Docs edit; publication tasks require Docs publish; imports remain available through the Docs surface and follow its existing import permission.
- CRM setup tasks require CRM edit or CRM admin according to the destination surface.
- Automation tasks retain module, entitlement, AI usage, and admin checks already applied by their launch surfaces.
- A completed task remains visible even if the current member cannot launch its action. An incomplete inaccessible task is blocked with the existing compact admin/plan explanation.

## Verification

- Catalog tests assert exact journey order, task order, copy, core membership, dependencies, and removed-key absence.
- Repository tests seed qualifying and non-qualifying rows for every new predicate, including workspace scoping, soft deletion, required fields, status, joined ownership, public/internal space type, indexed knowledge, agent preset, CRM stage coverage, accepted signal suggestions, and approval mode.
- Goal normalization tests prove the team-project alias cannot render a duplicate journey and stored legacy goals remain readable.
- Frontend tests prove the placeholder block and all maturity/stage labels are absent, every real selected goal renders as a collapsible journey, shared help-doc setup is not duplicated, and new action keys resolve to existing routes.
- Run focused Go service/repository tests, frontend interaction/action tests, Go build, and TypeScript validation. Do not repeat the full frontend production build for copy- or mapping-only iterations.
