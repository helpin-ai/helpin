# Setup and Success Journeys Design

**Date:** 2026-07-09
**Status:** Approved design and spec review
**Product:** Helpin workspace application

## Summary

Helpin will add a cross-module Setup guide that helps new workspaces reach first value quickly and then evolves into a success hub for deeper adoption. The experience will be organized around customer outcomes rather than product modules. It will use workspace goals, permissions, enabled modules, and verified product state to recommend one useful next action at a time.

Automation is a first-class adoption layer. Every outcome journey progresses from manual value to agent-assisted execution and then to trusted event- or schedule-driven automation. Power users also receive a dedicated Automation mastery journey.

The guide will not award progress for page visits, feature tours, or creating configuration that has never produced a result. Completion will be based on durable product evidence such as completing work, resolving a conversation, publishing content, accepting an agent result, or observing a successful automated run.

## Goals

- Reduce time to a meaningful first outcome for new workspaces.
- Help invited members find a useful first contribution instead of landing in an empty product.
- Move activated users from one-time setup into repeated workflows.
- Make Helpin's agents, flows, tools, and skills understandable and useful.
- Expose Helpin's differentiated cross-module workflows.
- Give power users a clear path to deeper automation and measurable value.
- Produce reliable adoption data that can be correlated with workspace and member retention.

## Non-goals

- A generic feature tour or a checklist of every settings page.
- A single platform-wide percentage that implies Helpin can be completely configured.
- A customer-authored automation/checklist builder.
- Replacing module-specific empty states and contextual education.
- Recommending capabilities that exist only as backend APIs or unfinished UI.
- Building missing reporting, imports, or CRM automation as part of the initial Setup guide unless explicitly included as prerequisite work.

## Product Principles

1. **Outcomes before modules.** Users select what they want to achieve; Helpin determines which modules enable that outcome.
2. **Value before configuration.** A configured feature is not adopted until it produces a useful result.
3. **One next action.** The page may contain several journeys, but it prominently recommends only one currently actionable step.
4. **Manual before automated.** Users should understand a workflow and have usable data before Helpin recommends automating it.
5. **Automatic evidence.** Verifiable steps complete from product state. Manual completion is reserved for guidance that cannot be verified.
6. **Permission-aware.** Users are never sent to actions they cannot perform.
7. **Finite journeys, ongoing success.** Each journey has a finite activation path, followed by optional repeat, connection, automation, and proof-of-value milestones.
8. **No false failure.** Unknown verification state is not treated as incomplete.

## Information Architecture

### Route and entry points

Create a cross-module route at `/w/:slug/setup`.

- New first-workspace owners land on the Setup guide after workspace onboarding instead of `/pm/my-work`.
- A compact `Get started` entry with journey progress appears near the top of the workspace sidebar.
- The entry may be dismissed after first value, but the Setup guide remains available from the workspace menu.
- Existing workspaces are not forcibly redirected.
- Invited members receive their own relevant first-contribution experience.

The Setup guide must not be placed under Projects, CRM, Support, Docs, Automation, or Settings because its journeys cross module boundaries.

### Page hierarchy

1. **Recommended next step**
   - One actionable item selected from active goals, lifecycle stage, permissions, module access, prerequisites, and current product state.
   - Shows the expected outcome and approximate effort.
   - Starts a typed product action rather than navigating to a generic module landing page.

2. **Active journeys**
   - One to three prioritized outcome journeys.
   - Each journey shows its own finite stage progress.
   - Workspace and personal progress are clearly distinguished.

3. **Workspace essentials**
   - Shared setup such as company context, teams, members, core access, and required integrations.
   - Restricted work names the responsible role instead of exposing a broken action.

4. **Unlock more**
   - Advanced integrations, agents, flows, tools, skills, templates, reporting, and governance.
   - Revealed progressively after first value.

5. **Explore goals**
   - Add, pause, remove, or reprioritize journeys.
   - Onboarding selections are initial signals, not permanent categorization.

The page should avoid dense card grids. The recommended action and active journey should dominate the hierarchy; secondary content should use compact lists and progressive disclosure.

## Hybrid Progress Ownership

Progress has two scopes:

- **Workspace scope:** shared setup and outcomes derived from workspace state, such as connecting a support channel or publishing the first help article.
- **Member scope:** personal contribution and learning inside a shared workspace goal, such as completing assigned work, reviewing a suggestion, or using an agent successfully.

Outcome goals are workspace-scoped in V1. Owners and admins manage them. Members manage their personal focus, task dismissals, and member-scoped progress within those shared goals. All workspace members can see relevant shared progress, subject to module access and permissions. V1 does not allow a duplicate personal copy of a workspace goal.

## Journey Lifecycle

Every outcome journey follows five stages:

1. **Prepare** — establish the minimum configuration and usable data.
2. **Activate** — achieve one real end-to-end outcome.
3. **Repeat** — repeat the workflow or involve another person.
4. **Connect** — link another module, integration, agent, or automated flow.
5. **Prove value** — surface a measurable result or operational improvement.

Items within a journey can be:

- **Required:** necessary to reach the stage outcome.
- **Recommended:** valuable but not stage-blocking.
- **Dependency:** requires another role, module, plan, or prerequisite.
- **Optional advanced:** deepens adoption without preventing journey completion.

### Completion and progress semantics

Each active goal is pinned to a catalog version. A catalog version defines required tasks and alternative groups for every core stage. New required tasks do not silently regress existing goals; they apply to newly activated goals or through an explicit catalog migration. Retired tasks are excluded from current progress but their history remains available for analytics and audit.

Every goal has an evidence window:

- Onboarding-created and manually activated goals use `activated_at` as the lower bound for first-value and repeat-value evidence.
- Inferred existing-workspace goals use the workspace creation time as the lower bound so historical activity can establish earned maturity.
- Backfilled task progress stores the original qualifying event time plus `backfilled: true`. It does not emit a new real-time activation event on first Setup read; retention analysis uses the historical occurrence time and separates inferred cohorts from new-workspace cohorts.
- When an inferred workspace has historical outcome evidence but a current readiness dependency is disconnected, the achievement is backfilled and the readiness task is returned as `needs_attention`.

- A **required task** is one stage requirement.
- A **required alternative group** is one stage requirement and completes when any one child task completes. For example, `support.channel_ready` can be satisfied by either an installed widget or an active inbound email route.
- Recommended and optional advanced tasks never enter the required-task denominator.
- A stage completes when every applicable required task or alternative group in that stage has been achieved.
- `blocked`, `in_progress`, and never-achieved `needs_attention` requirements remain in the denominator and are not complete.
- `unable_to_verify` is excluded from both numerator and denominator, sets the stage to `verification_pending`, and prevents a new stage-completion transition until verification succeeds. The UI shows the last verified progress alongside the verification warning.
- A readiness requirement that was previously achieved can later become `needs_attention`. This creates a readiness alert but does not erase the recorded stage achievement.

Journey maturity is computed, not manually set:

- `preparing`: Prepare is incomplete.
- `ready`: Prepare is complete but Activate is incomplete.
- `activated`: the required Activate milestone is complete. This is the first-value event.
- `established`: the required Repeat milestone is complete. This is the finite core-journey completion.
- `advanced`: after establishment, at least one Connect milestone and one Prove value milestone have completed.

Connect and Prove value tasks are expansion work and do not keep a core journey permanently incomplete. Users may pause or archive a goal, but cannot manually mark its maturity complete.

Workspace essentials is a shared **foundation group**, not an outcome journey. Its applicable requirements may block a journey's Prepare stage, but completing essentials never emits a first-value event.

## Outcome Journey Roadmap

### Workspace essentials

Verified actions:

- Save company/product context.
- Create or confirm at least one team.
- Invite another member.
- Configure appropriate workspace and module access.
- Confirm timezone and basic workspace identity.

First value is not defined by completing essentials alone. Essentials remove blockers for an outcome journey.

### Product and engineering delivery

- Create real work or import it from Shortcut.
- Move a real task through the workflow to completion.
- Connect a GitHub or GitLab repository.
- Run a planning or review agent on real work and use its output.
- Create and close a sprint, then review its closeout.
- Install a relevant repository automation such as Review merged PRs, Triage failing checks, or Release Notes Writer.

Do not recommend Jira or Linear import until those sources are operational. Do not claim velocity or cycle-time reporting; only sprint closeouts are currently available.

### Team and project management

- Create and assign real work.
- Complete work collaboratively.
- Reuse a task template.
- Configure recurring work.
- Use a team workflow, objective, or roadmap.
- Review a sprint closeout.
- Install Stale task escalation or Advance on approval when prerequisites exist.

The journey should not refer to a generic `Project` entity because current work organization is team, task, epic, sprint, objective, and workflow based.

### Customer support

- Install the widget or configure an inbound email route.
- Create a test conversation.
- Receive and resolve a real conversation.
- Assign work or configure an inbox/routing path.
- Add a usable knowledge source.
- Use AI assistance on a conversation and successfully apply or send the result.
- Create a linked PM task or Docs improvement from a customer issue.
- Review a support coverage gap when sufficient evidence exists.

Do not recommend CSAT until the UI is enabled. Do not claim an AI-containment trend or verified fix-improvement loop until those metrics exist.

### Help center docs

- Create content or import from a supported source: Help Scout or Nextra.
- Publish a useful customer article.
- Configure hosted or custom public access.
- Receive article feedback.
- Add and publish a locale when relevant.
- Install Public help freshness sweep.

Do not recommend unsupported imports such as Zendesk, Intercom, Notion, or Confluence.

### Internal docs

- Create an internal space and useful document.
- Configure workspace-wide or team-only access.
- Invite or involve another contributor.
- Link documentation to relevant product work or customer context.
- Use an AI change proposal and apply an accepted result.
- Configure reviews or install Internal docs freshness sweep.

### Sales and CRM

- Create a real contact and deal.
- Configure a pipeline and progress a deal.
- Connect Gmail and observe synced activity.
- Receive a buyer signal or generated insight.
- Accept or dismiss an AI suggestion.
- Install High-intent buyer signal to task when the required CRM data and destination team exist.

The initial journey must not recommend CRM CSV import until its wizard is mounted in a reachable UI. It must not recommend sending CRM email, calendar creation, general CRM workflows, forecasting, or CRM reporting until those actions are exposed and production ready.

### Cross-module success loops

After first value, Helpin should deliberately recommend its strongest differentiated loops:

- Support conversation → CRM contact → PM task → Docs improvement.
- Repository event → agent run → release notes or internal report.
- Buyer signal → suggestion or automated review → PM follow-up task.
- Support knowledge gap → Docs change proposal → improved support knowledge.

These are advanced milestones inside relevant journeys, not a separate beginner checklist.

The lists above define the program roadmap. The authoritative first-release catalog is narrower and is specified below. Team/project management, Help center, Internal docs, CRM, and additional cross-module milestones require their own feature-flagged vertical specifications before they ship; their roadmap bullets are not implementation-ready catalog rows by themselves.

## Automation Adoption Model

Automation appears in every outcome journey and as a dedicated optional journey.

### Embedded automation ladder

1. **Assist** — run a built-in agent on real work and accept or use the result.
2. **Repeat** — use the assisted workflow again.
3. **Automate** — install a recommended event- or schedule-driven template and complete a successful triggered run.
4. **Trust** — review output, resolve an approval, or understand and recover from a failed run.
5. **Scale** — customize an agent, add a flow, or extend behavior with tools and skills.

Automation completion evidence includes:

- Successful agent run.
- Accepted or applied output.
- Enabled flow.
- Triggered flow execution.
- Completed automated run.
- Resolved approval or interaction.
- Repeated successful execution.

Creating an agent, creating a flow, or visiting the Automation module is not value evidence by itself.

### Automation mastery journey

Teach the current product taxonomy accurately:

- Agents are reusable executors.
- Automation rules and flows decide when an executor or action runs.
- `agent_run` is the durable execution record.
- Tools and skills expand an agent's capabilities.
- Activity is where users review runs, failures, approvals, and attention items.

Recommend prebuilt templates before custom agents. Custom agents, custom skills, and manually authored flows are power-user milestones.

All AI launch actions must use existing billing preflight and metering. Billing and usage failures open `UpgradeRequiredDialog`; raw `AI usage exhausted` messages must never be shown.

## Authoritative First-release Catalog

Only the foundation, Product delivery, Customer support, and Automation mastery rows below ship in the first adoption release. Later roadmap journeys receive separate vertical specs and catalog tables before exposure.

Legend:

- Scope: `W` = workspace, `P` = current member.
- Class: `Req` = required for its core stage, `Rec` = recommended, `Adv` = optional advanced.
- Mode: `A` = durable achievement, `R` = continuously checked readiness.
- An entitlement value is an existing `EntitlementFeature`; a metered value is an existing AI usage feature key selected by the actual launch path.
- `first_value` and `repeat_value` are analytics milestones. Other rows do not emit those milestones.

### Shared foundation group

| Key | Scope | Class | Applies to | Permission / entitlement | Action | Verifier / mode |
|---|---:|---:|---|---|---|---|
| `foundation.company_context_ready` | W | Req | Any journey that launches an agent | `workspace.update` | `settings.company_context` | Non-empty `company_product_context` / R |
| `foundation.team_ready` | W | Req | Product delivery; PM-targeted automation | `team.manage` | `settings.teams` | At least one non-deleted team / R |
| `foundation.invite_sent` | W | Rec | All collaborative journeys | `workspace.invites.manage` | `settings.members.invite` | Invitation created or second active member exists / A |
| `foundation.member_joined` | W | Rec | Repeat-stage collaboration | `workspace.members.read` | `settings.members` | At least two active workspace members / R |

Foundation applicability is evaluated per active journey. A support-only owner is not blocked by `foundation.team_ready`; a Product delivery journey is.

### Product delivery journey

| Key | Stage | Scope / class | Prerequisites | Module, permission, entitlement | Action | Verifier / mode | Milestone |
|---|---|---|---|---|---|---|---|
| `product.initial_work` | Prepare | W / Req | `foundation.team_ready` | PM; `pm.edit` | `pm.add_initial_work` | At least one real task; creation and Shortcut import both satisfy the same verifier / A | — |
| `product.first_task_completed` | Activate | W / Req | `product.initial_work` | PM; `pm.edit` | `pm.open_next_task` | First task enters a done-type workflow state within the goal evidence window / A | `first_value` |
| `product.repeat_completion` | Repeat | W / Req | `product.first_task_completed` | PM; `pm.edit` | `pm.open_next_task` | A second distinct task completes on a later workspace-local date within the evidence window / A | `repeat_value` |
| `product.repository_ready` | Connect | W / Rec | `product.initial_work` | PM; `integrations.connect`; none | `settings.repositories` | At least one active workspace-linked Git repository / R | — |
| `product.agent_result_used` | Connect | P / Rec | `product.initial_work`, `foundation.company_context_ready` | PM + Automation; `pm.edit`; metered agent-run key | `automation.run_for_product_work` | Member-owned completed task/epic agent run with a persisted plan, artifact, applied proposal, or approved result / A | — |
| `product.sprint_closeout_reviewable` | Prove value | W / Adv | `product.first_task_completed` | PM; `pm.read` | `pm.reports.sprint_closeouts` | At least one persisted sprint closeout / A | — |
| `product.release_notes_flow_succeeded` | Connect | W / Adv | `product.repository_ready`, Docs destination | PM + Docs + Automation; `pm.admin.automations`; `automation_flows` | `automation.template.release_notes_writer` | Enabled Release Notes Writer installation has a completed triggered run / A | — |

`pm.add_initial_work` presents Create task as the primary action and Shortcut import as a secondary choice. It is one requirement, not two checklist items.

For V1, every persisted PM task counts toward setup evidence because bulk seed controls are development-only and unavailable in production. A future production starter-scenario feature must mark its entities with a removable sample batch ID; sample entities may prove technical readiness but cannot emit `first_value` until the user converts or replaces them with real work.

### Customer support journey

| Key | Stage | Scope / class | Prerequisites | Module, permission, entitlement | Action | Verifier / mode | Milestone |
|---|---|---|---|---|---|---|---|
| `support.channel_ready` | Prepare | W / Req alternative group | — | Support; `support.admin` | `support.choose_channel` | `support.widget_ready` **OR** `support.email_route_ready` / R | — |
| `support.widget_ready` | Prepare | W / alternative child | — | Support; `support.admin` | `support.install_widget` | Active widget installation / R | — |
| `support.email_route_ready` | Prepare | W / alternative child | — | Support; `support.admin` | `support.configure_email_route` | Active inbound email route / R | — |
| `support.delivery_validated` | Prepare | W / Req alternative group | `support.channel_ready` | Support; `support.edit` | `support.validate_delivery` | `support.test_conversation_resolved` **OR** `support.first_real_conversation_resolved` / A | — |
| `support.test_conversation_resolved` | Prepare | W / alternative child | `support.channel_ready` | Support; `support.edit` | `support.create_test_conversation` | Marked test conversation contains a customer message and reaches resolved / A | — |
| `support.first_real_conversation_resolved` | Activate; also Prepare alternative child | W / Req | `support.channel_ready` | Support; `support.edit` | `support.open_inbox` | First non-test conversation with a customer message reaches resolved within the goal evidence window / A | `first_value` |
| `support.repeat_resolution` | Repeat | W / Req | `support.first_real_conversation_resolved` | Support; `support.edit` | `support.open_inbox` | A second distinct non-test conversation resolves on a later workspace-local date / A | `repeat_value` |
| `support.knowledge_ready` | Connect | W / Rec | `support.channel_ready` | Support; `settings.manage` | `settings.knowledge` | At least one enabled and indexed content source or selected Docs knowledge source / R | — |
| `support.ai_reply_used` | Connect | P / Rec | `support.knowledge_ready` | Support + Automation; `support.edit`; metered support-AI key | `support.use_ai_reply` | Current member sends or applies an AI-produced reply recorded in message/event metadata / A | — |
| `support.pm_task_linked` | Connect | W / Adv | `support.first_real_conversation_resolved`, PM access | Support + PM; `support.edit` and `pm.edit` | `support.create_linked_task` | Support conversation gains a persisted PM task association / A | — |
| `support.coverage_improvement_applied` | Prove value | W / Adv | Coverage gap exists | Support + Docs; `support.edit` and `docs.edit` | `support.coverage` | Gap reaches done through a recorded resolution or its Docs proposal is applied / A | — |

`support.channel_ready` and `support.delivery_validated` are each one denominator requirement. Their alternatives are displayed as choices and never counted as multiple required tasks. A real resolved conversation satisfies both delivery validation and Activate. Test conversations validate setup but do not emit first value; only a real customer conversation does.

The setup action that creates a test conversation writes an explicit test marker in conversation metadata. Conversations without that marker are treated as real for setup evidence. Deleting the test conversation does not erase its durable setup-validation achievement.

### Automation mastery journey

| Key | Stage | Scope / class | Prerequisites | Module, permission, entitlement | Action | Verifier / mode | Milestone |
|---|---|---|---|---|---|---|---|
| `automation.target_ready` | Prepare | W / Req | At least one active outcome journey | Automation plus target module; target read permission | `automation.choose_real_target` | At least one supported real target exists: task, epic, conversation, document, deal, or linked repository / R | — |
| `automation.workspace_first_assisted_value` | Activate | W / Req aggregate | `automation.target_ready` | Derived from eligible member contribution | `automation.run_recommended_agent` | At least one member-scoped `automation.first_assisted_result` completes / A | `first_value` |
| `automation.first_assisted_result` | Activate | P / contribution child | `automation.target_ready`, `foundation.company_context_ready` when required by agent | Automation plus target module; target edit permission; metered agent-run key | `automation.run_recommended_agent` | Current member starts a completed built-in agent run with a persisted plan, artifact, applied proposal, or approved result / A | member activation |
| `automation.workspace_repeat_assisted_value` | Repeat | W / Req aggregate | `automation.workspace_first_assisted_value` | Derived from eligible member contribution | `automation.run_recommended_agent` | One member completes `automation.repeat_assisted_result`, or two members complete first assisted results on distinct targets / A | `repeat_value` |
| `automation.repeat_assisted_result` | Repeat | P / contribution child | `automation.first_assisted_result` | Same as prior member row | `automation.run_recommended_agent` | Current member completes a second qualifying run on a distinct target or later workspace-local date / A | member repeat |
| `automation.first_triggered_flow_success` | Connect | W / Rec | Eligible template inputs | Automation; `pm.admin.automations`; `automation_flows` | `automation.library.recommended` | Enabled event-driven template installation records a completed triggered run / A | — |
| `automation.first_scheduled_flow_success` | Connect | W / Adv | Eligible agent and target | Automation; `pm.admin.automations`; `automation_flows`, `agent_scheduling` | `automation.template.run_on_schedule` | Enabled scheduled flow records a completed cron-triggered run / A | — |
| `automation.approval_resolved` | Connect | P / Rec | A run requests approval/input | Automation; target read/edit permission | `automation.activity.attention` | Current member resolves a persisted approval or request-user-input interaction / A | — |
| `automation.custom_agent_succeeded` | Connect | W / Adv | `automation.workspace_repeat_assisted_value` | Automation; `pm.edit`; `custom_agents`; metered custom-agent-run key | `automation.agents.create_custom` | Non-system agent has a completed qualifying run; creation alone does not complete / A | — |
| `automation.reliable_unattended_value` | Prove value | W / Adv | A triggered or scheduled flow | Automation; `pm.read`; applicable flow entitlements | `automation.activity` | Same enabled flow completes at least three unattended runs across two workspace-local dates with no unresolved latest failure / A | — |

Automation mastery is one workspace goal. Workspace maturity uses the workspace aggregate rows; member rows provide personal progress and actionable recommendations without creating duplicate personal goals. Existing-workspace inference may create the shared goal from any qualifying run, then backfills workspace aggregates and the contributing members' task progress. An approval task is conditionally applicable only while a resolvable interaction exists. Its absence never blocks stage completion. The Automation journey's core completion ends at Repeat; Connect and Prove value establish advanced maturity.

## Code-backed Capability Audit

The catalog must be constrained by the current production UI, not just backend APIs.

### Available

- Workspace context, teams, invitations, roles, permissions, and module access.
- PM tasks, epics, sprints, objectives, workflows, templates, recurring work, Shortcut import, Git integrations, sprint closeouts, and agent runs.
- Support widget, inbound email, mailboxes, routing, conversations, assignment, canned responses, AI assistance, knowledge sources, coverage gaps, task linking, and CRM linking.
- Docs spaces, collections, documents, publishing, sharing, help-center configuration, localization, feedback submission, versions, change proposals, and team access.
- CRM contacts, companies, deals, pipelines, Gmail sync, signals, suggestions, health insights, and support/PM associations.
- Agents, flows, activity, tools, skills, manual runs, event triggers, cron triggers, and the current automation-template catalog.

### Partial or unavailable and excluded from initial tasks

- Jira and Linear imports are marked Coming Soon.
- CRM CSV import has service and component code but no mounted product entry point; current CRM import CTAs lead to a settings page without a CRM importer.
- CRM outbound email has an API but no reachable CRM composer.
- CRM calendar create/update APIs and hooks exist, but current CRM UI is read-only.
- General CRM workflow automation is not wired into the platform automation-rule engine.
- CRM reporting and forecasting are absent.
- PM velocity and cycle-time reports are marked Coming Soon.
- Support CSAT is disabled in the UI.
- Support coverage does not expose AI-containment trends or post-fix verification.
- The support coverage weekly-digest service is constructed but not scheduled.
- Production-safe, removable starter scenarios do not exist; current PM and CRM seed actions are development-only and create 500 records.

## Backend Design

Create a focused setup package following Handler → Service → Repository.

### Product-owned catalog

Journey and task definitions live in versioned backend code or embedded manifests. They are not customer-editable database records.

Each definition includes:

- Stable journey and task keys.
- Definition version.
- Stage and scope.
- Required module and permission.
- Required entitlement features and capacity limits, when any.
- Metered AI usage feature key or launch-time resolver, when any.
- Required/recommended/dependency/advanced classification.
- Prerequisite keys.
- Typed action key and parameters.
- Verifier key and verification mode.
- Estimated effort and user-facing copy.

Catalog validation must reject duplicate keys, dependency cycles, unknown verifier types, and invalid stage transitions.

### Persisted models

Use focused records rather than expanding the Workspace row with JSON blobs:

1. `SetupGoal`
   - `id`, `workspace_id`, `goal_key`, `catalog_version`, `priority`, `status`, `source`, `activated_at`, `selected_by_id`, `created_at`, `updated_at`, `archived_at`.
   - Goals are workspace-scoped in V1. `source` is `onboarding`, `manual`, or `inferred` and determines the evidence window.
   - `status` is `active`, `paused`, or `archived`. Journey maturity is computed from evidence and cannot be manually overridden.
   - Unique active goal per workspace and key.

2. `MemberSetupPreference`
   - `id`, `workspace_id`, `workspace_member_id`, focused goal keys, sidebar dismissal, last-viewed journey, timestamps.
   - Structured columns for stable state; JSONB only for small versioned preference metadata.

3. `SetupTaskProgress`
   - `id`, `workspace_id`, optional `workspace_member_id`, `task_key`, `definition_version`, `status`, `completion_source`, `evidence`, `completed_at`, `dismissed_at`, `needs_attention_at`, timestamps.
   - Unique key by workspace, member scope, and task key.
   - Evidence stores only minimal identifiers, counts, and timestamps; never customer content.

Exact model names may be adjusted during implementation to avoid ambiguity with PM checklist models, but the boundaries and ownership remain as specified.

### Onboarding integration

Add selected setup goals to the create-workspace request and persist them transactionally with first-workspace creation. The existing onboarding use-case choices become the initial goal set. Analytics tracking remains, but is no longer the source of truth.

The onboarding screen changes from unlimited multi-select to **choose up to three**, with selection order setting priority. The first selected goal is primary. The UI explains that goals can be changed later. If a request contains more than three goals, the backend rejects it with validation error rather than silently choosing.

The six onboarding choices map as follows:

| Onboarding choice | Goal key | Initial exposure |
|---|---|---|
| Product & engineering work | `product_delivery` | First release |
| Team/project management | `team_project_management` | Later vertical release |
| Customer support | `customer_support` | First release |
| Help center docs | `help_center_docs` | Later vertical release |
| Internal docs | `internal_docs` | Later vertical release |
| Sales/CRM | `sales_crm` | Later vertical release |

Before a later journey is enabled, selecting it persists the goal but the Setup guide labels it `Coming later` and recommends applicable foundation or Automation work only; it must not fabricate a checklist from roadmap bullets. Product rollout should normally enable the matching journey before exposing that onboarding choice to new cohorts.

The catalog registry ships metadata-only placeholder version `0` for every recognized later goal key. Version `0` has label/description metadata, exposure state `unavailable`, and no task or maturity definitions. A persisted placeholder goal pins to version `0` and returns `Coming later` with no progress denominator. When a vertical journey is released, an explicit feature-flagged migration activates and pins that goal to its first real catalog version; the system never silently evaluates roadmap prose as tasks.

### Existing-workspace inference

Existing workspaces are never redirected. On their first Setup-guide read, the backend evaluates journey evidence and creates inferred goals only from actual product use:

| Goal candidate | Minimum inference evidence | Strength |
|---|---|---:|
| `product_delivery` | Any non-deleted PM task, epic, sprint, or objective | 1 |
| `product_delivery` | At least one completed task or sprint closeout | 3 |
| `customer_support` | Active widget/email route, mailbox beyond the system default, or any conversation | 1 |
| `customer_support` | At least one resolved real conversation | 3 |
| `team_project_management` | Task template, recurring template, objective, or activity from two active members | 2 |
| `help_center_docs` | External-capable space, enabled help center, or externally published article | 2 |
| `internal_docs` | Internal space containing a document | 1 |
| `sales_crm` | Any contact, company, deal, or connected CRM email account | 1 |
| `sales_crm` | Progressed deal, buyer signal, or accepted suggestion | 3 |
| `automation_mastery` | Any completed agent run or enabled flow | 2 |

Candidates with no evidence are not inferred merely because the user can access their modules. Rank candidates by strength, then by most recent qualifying activity, then by stable goal key. Activate the top three and persist any additional evidence-backed candidates as paused. If no candidate has evidence, create no inferred goals and show a neutral goal-selection prompt. If only one or two candidates have evidence, activate only those; do not pad to three.

Progress is immediately evaluated from existing state. Mature workspaces therefore begin at their earned maturity, the sidebar label uses `Success guide` instead of `Get started`, and no beginner redirect occurs. Personal recommendations and member-scoped task progress derive from permissions, assignments, active workspace goals, and the member's own historical qualifying actions; inference never creates duplicate personal goals.

### Evidence aggregation

A `SetupEvidenceService` gathers relevant product facts in batches:

- Workspace context, teams, members, permissions, and modules.
- Entity counts and milestone timestamps.
- Connected integrations and readiness state.
- Agent and flow execution outcomes.
- Accepted or applied AI output.
- Cross-module associations.

Verifiers evaluate all relevant definitions against one evidence snapshot. Avoid one query per task.

Two verification modes are supported:

- **Achievement:** once verified, completion remains recorded.
- **Readiness:** continuously evaluated; loss of readiness produces `needs_attention` without deleting historical achievement.

All first-release tasks use automatic evidence. A future manual task requires an explicit catalog capability and a dedicated endpoint; verifier-backed tasks can never be completed through that endpoint.

Entitlement and capacity checks are part of applicability projection, not completion evidence. A task with a missing required entitlement is returned as `blocked` with an upgrade reason and may still be visible as optional expansion work. Metered AI tasks use the existing launch preflight at action time because available credits can change between projection and launch. A limit-gated create action uses the existing entitlement limit service before launch. The catalog never hardcodes plan names; `EntitlementService` remains the source of truth.

### Recommendation ranking

V1 uses deterministic server-side ranking:

1. Exclude completed, dismissed, unavailable, and irrelevant tasks.
2. Exclude actions the user cannot perform from direct recommendations; expose them as dependencies instead.
3. Resolve blockers for the nearest first-value milestone.
4. Prefer the lowest-effort action that advances an active goal.
5. Before first value, prefer workflow activity over advanced configuration.
6. After first value, prefer repetition and collaboration before deep customization.
7. Recommend automation only when prerequisites and usable data exist.
8. Prefer a cross-module connection after a workflow has repeated successfully.
9. Surface readiness failures ahead of optional expansion work.

The backend returns the same recommendation projection for the Setup page, sidebar, notifications, and future command surfaces.

### APIs

- `GET /api/workspaces/{id}/setup`
  - Personalized journeys, shared and personal progress, dependencies, readiness alerts, and one recommendation.
- `PUT /api/workspaces/{id}/setup/goals`
  - Add, activate, pause, archive, or reprioritize workspace goals. Owners/admins only. It never overrides computed journey maturity.
- `PATCH /api/workspaces/{id}/setup/preferences`
  - Updates the current member's focused goal keys, sidebar dismissal, last-viewed journey, and other explicitly allowed member preference fields.
- `POST /api/workspaces/{id}/setup/tasks/{taskKey}/dismiss`
  - Dismiss optional recommendation for the current scope.
- `POST /api/workspaces/{id}/setup/recommendations/{taskKey}/start`
  - Records action intent before the frontend performs the product action.

Workspace read access permits reading the personalized view. Existing workspace update/manage permissions govern goals. Members may manage only their preferences and personal task dismissals.

No first-release catalog row is manual, so V1 does not expose a manual-completion endpoint. Add that endpoint only alongside the first reviewed manual task definition; automatic verifier tasks can never be asserted complete by a client.

Sidebar dismissal is a member preference, not a goal mutation. Dismissing the sidebar entry does not pause goals, dismiss recommended tasks, or suppress readiness alerts. The compact entry may reappear only for a new critical `needs_attention` state, not for ordinary incomplete work.

### Typed action registry

The backend returns stable action descriptors, not arbitrary URLs:

```json
{
  "action_key": "support.install_widget",
  "params": {}
}
```

The frontend owns a typed registry that maps an action key to a TanStack route, existing modal, or contextual product action. Registry contract tests fail when a catalog action has no frontend implementation.

### Live progress

Relevant existing WebSocket events invalidate the setup query. Re-evaluation uses current evidence, so progress updates across members without duplicating business events. If later performance requires it, event-driven reconciliation may materialize progress asynchronously, but V1 should begin with batched evaluation plus persisted transitions.

## Frontend Design

Create a focused setup page and components rather than adding responsibility to existing module pages or the large Sidebar orchestrator.

Suggested boundaries:

- Setup route page: data loading, error boundary, high-level composition.
- Recommended action component.
- Journey progress component.
- Workspace essentials component.
- Goal selector/editor.
- Dependency and readiness-state presentation.
- Typed action registry and action launcher.
- Sidebar setup-progress entry extracted into the sidebar component folder.

TanStack Query owns server state. Local component state is limited to expanded sections, dialogs, and optimistic dismissal. Query keys belong in the shared query-key factory; API calls belong in a thin service adapter.

The page must support keyboard navigation, screen readers, reduced motion, responsive layouts, loading skeletons, and meaningful empty/error states.

## Failure Handling

The UI distinguishes:

- `incomplete`: reliably verified as not done.
- `in_progress`: import, sync, agent run, or flow execution is active.
- `needs_attention`: readiness was lost or a relevant process failed.
- `unable_to_verify`: evidence could not be loaded.
- `blocked`: another permission, module, plan, or prerequisite is required.

Rules:

- `unable_to_verify` never counts as incomplete.
- Verifier failures are logged with workspace ID, task key, and verifier; do not log sensitive content.
- External integration progress should expose actionable retry or settings destinations.
- A missing action-registry entry disables the action, reports a client error, and renders a safe fallback rather than navigating incorrectly.
- Entity creation plus agent launch handles `agent_run_error` separately. The entity may exist even when the run failed.
- Billing and usage errors use `getUpgradeRequiredReason` and `UpgradeRequiredDialog`.
- Removed catalog tasks do not damage historical progress or denominators.
- Dependency and readiness failures do not erase prior achievements.

## Analytics and Success Metrics

### Primary metric

Percentage of new workspaces that reach a journey-specific first-value event within seven days and repeat that value within the following seven days.

### Supporting metrics

- Time to first value by journey.
- First-value rate at day 1, 3, and 7.
- Repeat-value rate.
- Invited-member activation rate.
- First successful agent run.
- Accepted agent-output rate.
- First successful unattended automation.
- Automation run success, failure, and attention rates.
- Cross-module workflow adoption.
- Day 7, 30, and 90 retention by initial journey and role.
- Step impression, start, completion, dismissal, inability, and blockage rates.
- Time and AI usage required to activate.

Events include stable journey/task keys, definition version, role, module, lifecycle stage, completion source, and non-sensitive failure category. Do not send entity names, prompts, message contents, document contents, or customer data.

Use holdout cohorts for later nudges and recommendation experiments. Checklist completion is a diagnostic metric, not the north-star outcome.

## Nudges

Do not include broad onboarding email campaigns in the first release. Collect baseline behavior first.

Later behavior-based nudges must:

- Recommend only a valuable and currently possible action.
- Stop immediately when the evidence changes.
- Never ask members to perform admin-only work.
- Respect account and workspace notification preferences and do-not-disturb state.
- Prefer in-product recommendations before email.
- Enforce a frequency cap.
- Be measured against a holdout cohort.

## Rollout

### Phase 1: Foundation

- Persist onboarding goals.
- Add catalog, models, evidence aggregation, progress APIs, analytics, action registry, and sidebar entry.
- Add existing-workspace backfill behavior.

### Phase 2: First adoption release

- Workspace essentials.
- Product and engineering delivery.
- Customer support.
- Automation mastery.
- Embedded automation milestones.

### Phase 3: Content and collaboration

- Team/project management.
- Help center docs.
- Internal docs.
- Invited-member personal journeys.
- Cross-module Support → CRM → PM → Docs milestones.

### Phase 4: CRM and proof of value

- Fix and mount CRM import only if the CRM vertical spec selects it as a catalog action.
- CRM journey constrained to production-accessible actions.
- Visible outcome summaries.
- Automation health and value indicators.
- Behavior-based nudges after baseline measurement.

Journeys are independently feature flagged. Catalog definitions may ship dark before a journey is exposed.

## Implementation-plan Decomposition

This design is a product program, not one implementation branch. Writing-plans must produce separate executable plans with independent feature flags and verification:

1. **Setup platform foundation** — catalog types and validation, persistence, onboarding goal capture, evidence framework, APIs, frontend shell/action registry, analytics, sidebar entry, and existing-workspace inference.
2. **Product delivery journey** — the authoritative Product rows and their domain verifiers/actions.
3. **Customer support journey** — the authoritative Support rows, test marker, and domain verifiers/actions.
4. **Automation mastery journey** — the authoritative Automation rows, entitlement projection, and run/flow evidence.
5. **Later vertical journeys** — one spec and plan each for Team/project management, Help center, Internal docs, and CRM after a fresh capability audit.

The foundation plan may ship dark without any public journey. Each vertical plan produces working, testable software and can roll out independently. Existing-workspace inference belongs to the foundation plan because it determines the first visible state for every later journey, but its rollout remains separately feature flagged.

## Testing Strategy

### Backend

- Catalog validation: uniqueness, dependencies, cycles, stages, verifier registration.
- Evidence aggregation and query-count expectations.
- Deterministic recommendation ranking.
- Achievement versus readiness behavior.
- Shared versus member progress.
- Existing-workspace inference and backfill.
- Historical evidence windows and backfilled occurrence timestamps.
- Workspace Automation aggregates derived from member-scoped contributions.
- Metadata-only catalog version `0` and explicit vertical-version activation.
- Onboarding maximum-three validation and priority order.
- Permission, role, module, and plan filtering.
- Manual completion restrictions.
- Billing and external-service failure handling.
- Handler request validation and workspace isolation.
- Repository uniqueness and concurrent transition behavior.

### Frontend

- Action-registry completeness.
- Recommended action rendering and launching.
- Shared and personal journey presentation.
- Admin dependency states.
- In-progress, blocked, needs-attention, and unable-to-verify states.
- Goal editing, pausing, reprioritizing, and dismissal.
- Upgrade-dialog handling.
- Query invalidation after product events.
- Accessibility, keyboard navigation, reduced motion, and responsive layout.

### End to end

- A new owner selects Support and reaches first value.
- A product user connects a repository and completes an agent-assisted task.
- An admin installs a template and observes its first successful triggered run.
- An invited member receives a personal first-contribution action.
- An existing mature workspace receives backfilled progress.
- A disconnected integration becomes `needs_attention` without erasing achievement.
- A billing-limited AI action opens the upgrade experience.
- A restricted member sees an admin dependency instead of a broken action.

## Risks and Mitigations

### Checklist theater

Risk: users complete setup without experiencing value.
Mitigation: define completion from outcome evidence; measure repeat value and retention.

### Overwhelming new users

Risk: the platform's breadth produces a long list.
Mitigation: one recommended action, progressive disclosure, finite active journeys, and staged automation.

### Stale action definitions

Risk: product routes and capabilities change.
Mitigation: versioned catalog, typed action registry, catalog validation, and capability audit as part of feature changes.

### Expensive evidence evaluation

Risk: evaluating every task causes N+1 queries.
Mitigation: batch evidence by domain, test query counts, cache where justified, and persist transitions.

### Incorrect personalization

Risk: onboarding choices do not match later intent.
Mitigation: editable goals, product-state adaptation, dismissals, and neutral recommendations when confidence is low.

### Automation before trust

Risk: users enable autonomous behavior without understanding it.
Mitigation: manual → assisted → repeated → automated → trusted progression, conservative defaults, approvals, and activity visibility.

### Existing-user annoyance

Risk: experienced workspaces receive beginner prompts.
Mitigation: no forced redirect, evidence backfill, inferred maturity, and dismissible entry point.

## Open Implementation Decisions

These are implementation details to settle in the implementation plan without changing the approved product design:

- Exact package and model names that avoid collision with PM checklists.
- Whether catalog definitions use Go structs or embedded YAML validated into Go structs.
- The initial evidence snapshot cache duration.
- The feature-flag mechanism and rollout cohort percentages.
- Exact event names within the existing analytics naming convention.
- The smallest production-safe starter scenarios to build after baseline activation data is available.
