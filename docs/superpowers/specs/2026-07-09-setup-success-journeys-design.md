# Setup and Success Journeys Design

**Date:** 2026-07-09  
**Status:** Approved design, pending spec review  
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
- **Member scope:** personal contribution and learning, such as completing assigned work, reviewing a suggestion, or using an agent successfully.

Owners and admins manage shared workspace goals. Members manage personal goals and dismissals. All workspace members can see relevant shared progress, subject to module access and permissions.

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

## Initial Outcome Journeys

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
- Required/recommended/dependency/advanced classification.
- Prerequisite keys.
- Typed action key and parameters.
- Verifier key and verification mode.
- Estimated effort and user-facing copy.

Catalog validation must reject duplicate keys, dependency cycles, unknown verifier types, and invalid stage transitions.

### Persisted models

Use focused records rather than expanding the Workspace row with JSON blobs:

1. `WorkspaceSetupGoal`
   - `id`, `workspace_id`, `goal_key`, `priority`, `status`, `selected_by_id`, `created_at`, `updated_at`, `completed_at`.
   - Unique active goal per workspace and key.

2. `MemberSetupPreference`
   - `id`, `workspace_id`, `workspace_member_id`, personal goal preferences, sidebar dismissal, last-viewed journey, timestamps.
   - Structured columns for stable state; JSONB only for small versioned preference metadata.

3. `SetupTaskProgress`
   - `id`, `workspace_id`, optional `workspace_member_id`, `task_key`, `definition_version`, `status`, `completion_source`, `evidence`, `completed_at`, `dismissed_at`, `needs_attention_at`, timestamps.
   - Unique key by workspace, member scope, and task key.
   - Evidence stores only minimal identifiers, counts, and timestamps; never customer content.

Exact model names may be adjusted during implementation to avoid ambiguity with PM checklist models, but the boundaries and ownership remain as specified.

### Onboarding integration

Add selected setup goals to the create-workspace request and persist them transactionally with first-workspace creation. The existing onboarding use-case choices become the initial goal set. Analytics tracking remains, but is no longer the source of truth.

Existing workspaces receive inferred goals only when inference is sufficiently clear; otherwise they receive a neutral goal-selection prompt. Their progress is evaluated from existing product state so mature workspaces are not shown beginner work.

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

Automatic tasks cannot be completed through the manual API.

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
  - Add, pause, remove, complete, or reprioritize goals according to scope and permission.
- `POST /api/workspaces/{id}/setup/tasks/{taskKey}/complete`
  - Manual-only task completion.
- `POST /api/workspaces/{id}/setup/tasks/{taskKey}/dismiss`
  - Dismiss optional recommendation for the current scope.
- `POST /api/workspaces/{id}/setup/recommendations/{taskKey}/start`
  - Records action intent before the frontend performs the product action.

Workspace read access permits reading the personalized view. Existing workspace update/manage permissions govern shared goals. Members may manage their personal goals and dismissals.

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
- Fix CRM import navigation before it can enter the catalog.

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

- CRM journey constrained to production-accessible actions.
- Visible outcome summaries.
- Automation health and value indicators.
- Behavior-based nudges after baseline measurement.

Journeys are independently feature flagged. Catalog definitions may ship dark before a journey is exposed.

## Testing Strategy

### Backend

- Catalog validation: uniqueness, dependencies, cycles, stages, verifier registration.
- Evidence aggregation and query-count expectations.
- Deterministic recommendation ranking.
- Achievement versus readiness behavior.
- Shared versus member progress.
- Existing-workspace inference and backfill.
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

