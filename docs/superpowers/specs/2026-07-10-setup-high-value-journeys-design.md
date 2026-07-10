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
- Retain maturity, stage, scope, core, and evidence data internally for API compatibility and analytics, but replace the old stage-based maturity algorithm with the universal completion algorithm below.
- Journey headers show only accent, title, description, verified completion count, and collapse control.
- Task rows show only the one-sentence action-and-reason label, personal/blocked status when applicable, completion state, and action.
- On initial page entry or workspace change, open only the first journey with an `available` or `needs_attention` task. Skip completed journeys, fall back to the first journey with blocked incomplete work, and keep every journey collapsed when all visible tasks are complete. Preserve manual toggles during same-workspace refreshes.
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

All three visible Foundation tasks are core. When `foundation.team_ready` is not applicable, the remaining company-context and teammate-joined tasks are the complete core denominator.

### Plan and ship team projects

`team_project_management` and the existing `product_delivery` goal resolve to this single journey. Goal normalization deduplicates the aliases so it is never rendered twice. Existing stored `team_project_management` goals remain valid and migrate logically at read/update time without destructive data migration.

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `product.project_planned` | Create an epic with an owner and timeline so the team knows what it is delivering. | A non-archived `pm_epics` row has an owner (`owner_member_id` or legacy `owner_id`), `planned_start_date`, and `deadline`. | `pm_epics` → `/pm/epics` |
| 2 | `product.sprint_planned` | Create a sprint with planned work so the team can execute a clear delivery cycle. | A non-archived `pm_sprints` row has start/end dates and at least one non-archived `pm_tasks` row referencing its `sprint_id`. | `pm_sprints` → `/pm/sprints` |
| 3 | `product.work_assigned` | Assign project work to teammates so every task has clear ownership. | A non-archived task in an epic or sprint has at least one `pm_task_owners` row. | `pm_tasks` → `/pm/tasks` |
| 4 | `product.sprint_closeout_reviewable` | Close a sprint so the team can review what shipped and improve the next cycle. | At least one `pm_sprint_closeouts` row. | `pm_sprints` → `/pm/sprints` |
| 5 | `product.repository_ready` | Connect a code repository so Helpin can link project work to what you ship. | An active, selected, non-deleted `git_repositories` row. | `git_settings` → `/settings/repositories` |
| 6 | `product.agent_result_used` | Run an agent on project work so planning or review takes less manual effort. | A valuable completed agent run targets `task`, `epic`, or `repository` and has a non-empty `target_id`. This is workspace-shared, not member-specific. | `product_agent` → `/automation/agents` |
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

When `help_center_docs` is also selected, omit `support.help_docs_ready` from the support journey. The detailed help-center journey owns that setup capability and supplies the same underlying evidence. This rule is computed from the complete canonical selected-goal set before any journey is built, so saved goal order cannot affect it.

Without Help Center selected, Support has seven core tasks and nine visible tasks. With Help Center selected, Support has six core tasks and eight visible tasks; the hidden help-doc task contributes neither journey nor global progress. `support.ai_agent_activated` checks `PublicHelpDocCount > 0` directly as its help-doc prerequisite plus the visible `support.brand_knowledge_ready` prerequisite, rather than depending on a hidden task key. Recommendations never point to the hidden key. Support uses the universal maturity algorithm against its adjusted visible core/power set.

### Publish help center docs

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `help_center.space_ready` | Create a public help-center space so customer content has a clear home. | A non-deleted, non-system `docs_spaces` row has `type = 'external_capable'`. | `docs_home` → `/docs` |
| 2 | `help_center.content_ready` | Create or import customer-facing articles so common questions are documented. | A non-deleted `docs_documents` row belongs to an external-capable space and joins `docs_contents` with `word_count > 0 OR TRIM(content_text) <> ''`. | `docs_home` → `/docs` |
| 3 | `help_center.article_published` | Publish an article so customers can use it to solve a real question. | A `docs_helpcenter_articles` row with `public_published_at IS NOT NULL` joins through a non-deleted workspace `docs_documents` row to a non-deleted external-capable workspace `docs_spaces` row. | `help_center_article_publish` → `/docs` |
| 4 | `help_center.site_published` | Publish your help center so customers can browse and search your documentation. | The workspace `docs_helpcenter_configs.is_published` value is true. | `help_center_settings` → `/settings/helpcenter` |
| 5 | `help_center.widget_connected` | Add help-center content to live chat so customers can find answers before starting a conversation. | An active support widget installation has a non-empty `widget_help_space_ids` setting containing an existing external-capable space. | `help_center_widget` → `/settings/chat-general` |

Tasks 1–4 are core. Widget integration is a power capability. Dependencies follow the displayed order, except widget integration requires article and site publication.

### Build internal knowledge

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `internal_docs.space_ready` | Create an internal knowledge space so company information has a trusted home. | A non-deleted, non-system `docs_spaces` row has `type = 'internal'`. | `docs_home` → `/docs` |
| 2 | `internal_docs.content_ready` | Create or import internal documents so teammates can find essential information. | A non-deleted `docs_documents` row belongs to an internal space and joins `docs_contents` with `word_count > 0 OR TRIM(content_text) <> ''`. | `docs_home` → `/docs` |
| 3 | `internal_docs.published` | Publish internal knowledge so it is available across the workspace. | A non-deleted workspace `docs_documents` row has `status = 'published'` and `published_at IS NOT NULL` and joins a non-deleted internal workspace `docs_spaces` row. | `docs_home` → `/docs` |
| 4 | `internal_docs.ownership_ready` | Assign document owners and review dates so important knowledge stays current. | A non-deleted workspace document has `owner_id IS NOT NULL` and `next_review_at IS NOT NULL` and joins a non-deleted internal workspace space. | `docs_home` → `/docs` |
| 5 | `internal_docs.agent_connected` | Connect internal knowledge to an AI agent so it can answer with trusted company context. | `agent_knowledge_sources.sync_status = 'ready'` and (`indexed_documents > 0 OR indexed_chunks > 0`), joined by `space_id` and `workspace_id` to a non-deleted internal `docs_spaces` row and by `agent_id`/`workspace_id` to an agent in the same workspace. | `internal_docs_agent_knowledge` → `/automation/agents` |
| 6 | `internal_docs.agent_succeeded` | Run the documentation agent on a real document so maintaining knowledge takes less manual effort. | A valuable completed `agent_runs` row has `target_type = 'document'`, a non-empty `target_id`, joins that target to a same-workspace non-deleted document in a same-workspace non-deleted internal space, and joins its agent to the same workspace with `preset_key = 'documentation_agent'`. | `internal_docs_agent_run` → `/automation/agents` |

Tasks 1–4 are core. Agent connection and successful document run are power capabilities. Dependencies follow the displayed order; the agent run requires agent-connected knowledge.

### Build a sales pipeline

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `crm.contact_ready` | Add or import a contact so customer conversations have useful sales context. | At least one `crm_contacts` row exists for the workspace. | `crm_contacts` → `/crm/contacts` |
| 2 | `crm.company_ready` | Add or import a company so contacts and opportunities can be grouped by account. | At least one `crm_companies` row exists for the workspace. | `crm_companies` → `/crm/companies` |
| 3 | `crm.pipeline_ready` | Configure pipeline stages so every opportunity follows a consistent sales process. | A workspace `crm_pipelines` row has at least one open, one won, and one lost `crm_pipeline_stages` row. | `crm_pipelines` → `/settings/crm-pipelines` |
| 4 | `crm.deal_ready` | Create a deal with an owner, value, and close date so the opportunity is actionable. | A `crm_deals` row has `owner_member_id`, positive `amount`, and `close_date`. | `crm_deals` → `/crm/deals` |
| 5 | `crm.email_connected` | Connect your sales inbox so Helpin can capture customer conversations and buying signals. | An active `crm_email_accounts` row has `status = 'connected'`. | `crm_email` → `/settings/crm-email` |
| 6 | `crm.autonomy_enabled` | Enable CRM automation so Helpin can create or progress deals from strong customer signals. | `crm_autonomy_settings.enabled` is true and either `auto_create_deals` or `auto_progress_deals` is true. | `crm_autonomy` → `/settings/crm-autonomy` |
| 7 | `crm.signal_value_proven` | Create or progress a deal from a detected customer signal so the pipeline updates itself. | A `crm_suggestions` row has type `deal_create` or `deal_advance`, `execution_status = 'succeeded'`, `executed_at IS NOT NULL`, and a non-null `object_id` referencing an existing workspace deal. | `crm_review` → `/crm/review` |

Tasks 1–4 are core. Email, autonomy, and signal-driven value are power capabilities. Dependencies: deal requires contact, company, and pipeline; signal value requires connected email and enabled autonomy.

`CRMSuggestion` gains durable `execution_status` (`pending`, `succeeded`, `failed`), `executed_at`, and `execution_error` fields via AutoMigrate. Manual acceptance records `accepted` intent, executes the action, then records `succeeded` only after the deal create/advance transaction succeeds; create stores the created deal in `object_type/object_id`, and advance stores the successfully updated deal. Failures record `failed` plus a safe error summary. Existing auto-create/auto-advance paths write accepted suggestions with `execution_status = 'succeeded'`, `executed_at`, and the affected deal ID. Historic accepted rows without execution status do not qualify.

### Automate repeatable work

| Order | Key | Label | Completion evidence | Action |
|---|---|---|---|---|
| 1 | `automation.first_assisted_value` | Complete an agent run on real work so you can see where Helpin saves time. | A valuable completed agent run has persisted output or an artifact, a non-empty `target_id`, and target type `task`, `epic`, `repository`, `support_conversation`, `document`, `crm_deal`, `crm_contact`, or `crm_company`. | `automation_agents` → `/automation/agents` |
| 2 | `automation.custom_agent_succeeded` | Run a custom agent successfully so it can handle work specific to your team. | A valuable completed run joins an agent with `is_system = false`. | `automation_custom_agent` → `/automation/agents` |
| 3 | `automation.flow_enabled` | Turn on an automation flow so repeat work can run automatically. | An enabled `automation_rules` row. | `automation_flows` → `/automation/flows` |
| 4 | `automation.approval_guard_configured` | Require approval for sensitive agent actions so automation stays under human control. | A non-system agent has effective `approval_mode = 'always'`. | `automation_approval_guard` → `/automation/agents` |
| 5 | `automation.triggered_value` | Complete a triggered or scheduled automation so value no longer depends on a manual start. | A completed `agent_trigger_executions` row. | `automation_flows` → `/automation/flows` |
| 6 | `automation.reliable_unattended_value` | Run the same automation successfully over time so your team can trust it unattended. | The same enabled rule completes successfully on three distinct local days and has no newer failure. | `automation_flows` → `/automation/flows` |

Tasks 1, 3, and 5 are core. Custom agents, approval safeguards, and demonstrated reliability are power capabilities. Dependencies: custom agent and approval safeguard require the first useful run; triggered success requires an enabled flow; reliability requires triggered success.

## Internal progress, stages, and maturity

Task `stage` remains in the API for compatibility but no longer drives behavior. Assign `Core` to core tasks and `Power` to power tasks; neither value is rendered. Replace `setupMaturity(stages)` with one universal calculation over the journey’s currently visible tasks:

- `preparing`: zero visible core tasks complete;
- `ready`: at least one but not all visible core tasks complete;
- `activated`: all visible core tasks complete and zero visible power tasks complete;
- `established`: all visible core tasks and at least one but not all visible power tasks complete;
- `advanced`: every visible task is complete. A journey with no power tasks becomes `advanced` when all core tasks complete.

Completion includes current verified evidence or a durable achievement under the existing rules. A readiness/configuration task that was achieved and is no longer configured remains `needs_attention`; it counts as complete for the progress fraction but current evidence is required for prerequisite gating and recommendations.

## Exact prerequisite graph

Prerequisites gate actions and recommendations only. A task whose evidence is already complete remains completed even when an earlier prerequisite is currently absent.

- Foundation: `foundation.member_joined` requires `foundation.company_context_ready`.
- Product: `product.sprint_planned` requires `product.project_planned`; `product.work_assigned` and `product.sprint_closeout_reviewable` require `product.sprint_planned`; `product.release_notes_flow_succeeded` requires `product.repository_ready`.
- Support: existing support dependencies remain, except `support.ai_agent_activated` requires current `PublicHelpDocCount > 0` plus `support.brand_knowledge_ready` when Help Center is selected; otherwise it requires visible `support.help_docs_ready` plus `support.brand_knowledge_ready`. `support.routing_enabled` requires `support.team_inbox_created`; `support.coverage_fix_applied` requires `support.ai_agent_activated`.
- Help Center: `help_center.content_ready` requires `help_center.space_ready`; `help_center.article_published` requires `help_center.content_ready`; `help_center.site_published` requires `help_center.article_published`; `help_center.widget_connected` requires both `help_center.article_published` and `help_center.site_published`.
- Internal Docs: each task requires the immediately previous task in displayed order.
- CRM: `crm.deal_ready` requires `crm.contact_ready`, `crm.company_ready`, and `crm.pipeline_ready`; `crm.email_connected` has no prerequisite; `crm.autonomy_enabled` requires `crm.email_connected`; `crm.signal_value_proven` requires both `crm.email_connected` and `crm.autonomy_enabled`.
- Automation: `automation.custom_agent_succeeded` and `automation.approval_guard_configured` require `automation.first_assisted_value`; `automation.triggered_value` requires `automation.flow_enabled`; `automation.reliable_unattended_value` requires `automation.triggered_value`.

`foundation.team_ready` is applicable only when the canonical selected-goal set contains `product_delivery` or `automation_mastery`. `foundation.member_joined` is applicable whenever at least one non-foundation goal is selected.

## Action and access matrix

Routes below are workspace-relative. `—` means no setup-specific entitlement; destination surfaces continue enforcing their own billing and AI-usage checks before any run starts.

| Action key | CTA label | Route | Module | Required permission(s) | Setup entitlement |
|---|---|---|---|---|---|
| `workspace_context` | Add context | `/settings/general` | workspace | `workspace.update` | — |
| `workspace_teams` | Create team | `/settings/teams` | workspace | `team.manage` | — |
| `workspace_members` | View members | `/settings/members` | workspace | `workspace.members.read` | — |
| `pm_epics` | Plan epic | `/pm/epics` | pm | `pm.edit` | — |
| `pm_sprints` | Plan sprint | `/pm/sprints` | pm | `pm.edit` | — |
| `pm_tasks` | Assign work | `/pm/tasks` | pm | `pm.edit` | — |
| `git_settings` | Connect repository | `/settings/repositories` | pm | `integrations.connect` | — |
| `product_agent` | Run planning agent | `/automation/agents` | automation + pm | `pm.edit` | — |
| `docs_home` | Open Docs | `/docs` | docs | `docs.edit` | — |
| `help_center_article_publish` | Publish article | `/docs` | docs | `docs.publish` | — |
| `help_center_settings` | Publish help center | `/settings/helpcenter` | docs | `docs.admin` | — |
| `help_center_widget` | Add to live chat | `/settings/chat-general` | support + docs | `support.admin` and `docs.read` | — |
| `internal_docs_agent_knowledge` | Connect agent | `/automation/agents` | automation + docs | `docs.edit` | — |
| `internal_docs_agent_run` | Run documentation agent | `/automation/agents` | automation + docs | `docs.edit` | — |
| `support_email_inbox` | Connect email | `/settings/inboxes-routing?tab=email` | support | `support.admin` | — |
| `support_live_chat` | Add live chat | `/settings/chat-general` | support | `support.admin` | — |
| `support_help_docs` | Add help docs | `/docs` | docs | `docs.edit` | — |
| `support_brand_knowledge` | Add knowledge | `/settings/knowledge` | support | `support.admin` | — |
| `support_ai` | Activate AI agent | `/settings/support-ai-assistant` | support | `support.admin` | — |
| `support_team_inboxes` | Create inbox | `/settings/inboxes-routing?tab=inboxes` | support | `support.admin` | — |
| `support_routing` | Set up routing | `/settings/inboxes-routing?tab=routing` | support | `support.admin` | — |
| `support_inbox` | Open inbox | `/support` | support | `support.edit` | — |
| `support_coverage` | Review coverage | `/support/coverage` | support + docs | `support.edit` and `docs.edit` | — |
| `crm_contacts` | Add contacts | `/crm/contacts` | crm | `crm.edit` | — |
| `crm_companies` | Add companies | `/crm/companies` | crm | `crm.edit` | — |
| `crm_pipelines` | Configure pipeline | `/settings/crm-pipelines` | crm | `crm.admin` | — |
| `crm_deals` | Create deal | `/crm/deals` | crm | `crm.edit` | — |
| `crm_email` | Connect inbox | `/settings/crm-email` | crm | `crm.edit` | — |
| `crm_autonomy` | Enable automation | `/settings/crm-autonomy` | crm | `crm.admin` | `deal_automation` |
| `crm_review` | Review CRM activity | `/crm/review` | crm | `crm.edit` | `deal_automation` |
| `automation_agents` | Run an agent | `/automation/agents` | automation | at least one of `pm.edit`, `docs.edit`, `crm.edit`, `support.edit` | — |
| `automation_custom_agent` | Build custom agent | `/automation/agents` | automation | at least one of `pm.edit`, `docs.edit`, `crm.edit`, `support.edit` | `custom_agents` |
| `automation_approval_guard` | Configure approvals | `/automation/agents` | automation | at least one of `pm.edit`, `docs.edit`, `crm.edit`, `support.edit` | `custom_agents` |
| `automation_flows` | Build automation | `/automation/flows` | automation | `pm.admin.automations` | `automation_flows` |

The eventual agent run is subject to the existing backend AI preflight and frontend upgrade dialog. Product release notes, triggered success, and reliability use `automation_flows`; event-triggered rules can satisfy those milestones, so `agent_scheduling` is not a setup requirement. If the user chooses a cron trigger inside the destination, that surface independently enforces scheduling entitlement.

## Goal selection and ordering

- Foundation is always first.
- Selected real journeys follow the customer’s saved goal order.
- Automation remains a featured final journey when not selected, but the UI does not label it `Power up`.
- Canonicalize `team_project_management` to `product_delivery` in every path: onboarding `SetupIntent`, stored `SetupGoal` reads, GET response construction, and update payload normalization. Canonicalize before deduplication and before the three-goal maximum validation. Preserve the position of the first alias occurrence when both forms appear, return only `product_delivery`, and on the next goal update pause/remove the redundant legacy active row so it cannot reappear. Alias-only and canonical-only work unchanged. Alias plus three other distinct goals is four canonical goals and is rejected; alias plus canonical plus two others is three and is accepted.
- Remove the duplicate Team projects option from the selector. Rename the canonical `product_delivery` option and journey to `Plan and ship team projects`.
- Remove the placeholder catalog and placeholder-goal response section once all four current goals are real.
- Preserve the maximum of three selected non-foundation goals.

## Permissions and unavailable actions

- Apply the exact action matrix above in `setupActionAllowed`; do not infer permissions from journey names.
- Imports remain available through their destination surfaces and continue to require the existing import permissions when the customer chooses import rather than create.
- Automation tasks retain destination-level AI usage preflight and upgrade-dialog behavior. Merely opening the destination does not consume AI usage.
- A completed task remains visible even if the current member cannot launch its action. An incomplete inaccessible task is blocked with the existing compact admin/plan explanation.

## Verification

- Catalog tests assert exact journey order, task order, copy, core membership, dependencies, and removed-key absence.
- Repository tests seed qualifying and non-qualifying rows for every new predicate, including workspace scoping, soft deletion, required fields, status, joined ownership, public/internal space type, indexed knowledge, agent preset, CRM stage coverage, accepted signal suggestions, and approval mode.
- Goal normalization tests cover pending intent, stored-row read, GET response, and updates for alias-only, canonical-only, both aliases in either order, alias plus canonical plus two other goals, and alias plus three other distinct goals.
- Frontend/service tests prove order-independent Support/Help Center deduplication, its six-versus-seven support core denominator, current-evidence AI prerequisite, hidden-task recommendation exclusion, and no global double-counting.
- Repository tests prove empty docs do not qualify, internal agent sources use the exact ready/indexed/workspace joins, accepted-but-failed CRM suggestions do not qualify, successful suggestions carry a real deal ID, and agent runs require a qualifying non-empty target.
- Frontend tests prove the placeholder block and all maturity/stage labels are absent, every real selected goal renders as a collapsible journey, shared help-doc setup is not duplicated, and every action key/CTA resolves to the specified route.
- Access tests cover every action matrix row, including cross-module requirements, entitlements, blocked reasons, and destination-level AI-preflight delegation.
- Catalog/reference tests assert all removed task keys are absent from tasks, prerequisites, core/readiness sets, recommendations, member-evidence loading, and new achievement writes; historical stored achievements remain tolerated.
- Maturity tests exercise every universal threshold, journeys with and without power tasks, and the adjusted support task set.
- Run focused Go service/repository tests, frontend interaction/action tests, Go build, and TypeScript validation. Do not repeat the full frontend production build for copy- or mapping-only iterations.
