# CRM Signals and Playbooks: product blueprint

Created: 2026-09-05 · Reconciled: 2026-09-08 · Branch: `waqar-fixes`

Status: **Implemented on `waqar-fixes`; deployment is separate.** Unified Signals, Playbooks, guided Flow/Beacon connections, explicit activation, durable checks, exact action approvals and inspected-result recovery are connected on this branch. Ordinary saved Flow/Agent settings are preserved. No application migration, production activation or live customer send has been performed.

Scope: Unified Signals/Review experience, a dedicated CRM Playbooks page, customer-process tracking, all three defined sales/success journeys, and integration with existing Automation. Current implementation status is recorded below. No saved customer Flow/Agent changes, automatic enrollment, activation or deployment are implied by this plan.

## 1. Product promise and boundaries

Helpin keeps customer work moving and brings the responsible person the decisions that matter. Sales and success users should be able to find relevant work, understand why it matters, take the next action, and see whether the customer actually progressed.

**Playbooks define the customer objective and rules. Flows trigger the work. Beacon uses relevant skills to perform permitted jobs. CRM records customer progress.** A successful run is not proof of a successful customer outcome.

Use the existing built-in CRM Agent, Beacon, by default, with specialized skills for buying-intent follow-up, sales-to-success handoff, and renewal-risk recovery. Keep Flows as the event/checkpoint-to-run connection. Neither a new Agent per Playbook/customer nor a general ordered-step Flow builder is required. Durable scheduling, approvals, cancellation and recovery still belong in shared Automation and existing execution services, not Agent memory.

**Delivery contract: one complete product release for the agreed scope.** Playbooks, buying-intent follow-up, sales-to-success handoff, and renewal-risk recovery are release requirements, not a staged roadmap. Engineering tasks have dependencies, but no agreed capability is deferred to another product version or represented by a “coming soon” control. Configuration versioning below means auditable changes to live definitions, not partial product releases.

This supersedes the earlier [Signals information contract](specs/2026-09-05-signals-workflow-information-contract.md) where it differs: the working unit is a persistent customer situation, and Review actions belong in the same daily workspace, including standalone recommendations. Retain evidence-level identity, provenance, feedback, permissions, independent requests and truthful counts. The earlier contract and design mock remain historical references, not competing implementation instructions.

Documentation authority:

- **This blueprint:** agreed product scope, daily experience, customer outcomes and acceptance criteria.
- **[Playbook automation plan](crm-playbook-automation-change-proposal.md):** agreed Flow/Beacon/skill connection, required extensions, safeguards and engineering order. Its earlier mandatory ordered-step proposal is superseded; the filename is retained for links.
- **[CRM reference](crm-signals.md) and [Automation reference](AGENTS_AND_AUTOMATION.md):** implemented behavior in the inspected branch, clearly separated from the agreed extension.
- **[Automation product model](AUTOMATION_PRODUCT_MODEL.md):** shared Flows / Activity / Agents / Library mental model.
- **Generated blueprint review page:** derived from this Markdown, not independently maintained policy. Historical mocks/specs do not override these documents.

Boundaries:

- No second CRM agent runtime, task system, approval engine, or general-purpose workflow builder.
- PM, Support, and Docs participate only when useful. No task, ticket, document, or enabled Flow is required to inspect or manually handle a situation.
- Signal detection and interpretation remain product-owned capabilities; users do not have to construct a Flow to make the CRM notice evidence.
- No automatic enrollment or external action merely because this UI launches. Preserve rule-version, identity, workspace rollout, routing, permission, and activation gates.
- Existing deal stages, subscription facts, tasks, and conversations remain authoritative. CRM process tracking links to them instead of maintaining competing copies.

## 2. Objects and ownership

These are logical responsibilities, not a requirement for six new user-facing concepts or database tables.

| Concept | Responsibility | Owner |
| --- | --- | --- |
| Signal | Immutable interpreted evidence, source links, confidence, priority, and evidence feedback. | Existing CRM intelligence |
| Customer situation | One bounded customer objective or decision: owner, progress, current evidence, next action, and eventual outcome. Can be handled manually or automatically. | CRM |
| Playbook definition | Reusable business policy: eligible customers, objective, roles, milestones, permitted actions, and exit conditions. References execution configuration. | CRM business configuration, connected to Flows |
| Flow / trigger execution | When work starts or is reevaluated, the connected Beacon/skill run, and why it matched, skipped, or failed to launch. | Existing Automation, extended as needed |
| Proposed action / approval | Exact operation, target, payload revision, authorized approver, and decision. | Existing approval/suggestion contracts, exposed through a common interface |
| Agent run / linked work | Execution details and artifacts, or a canonical PM task / Support conversation / Doc used by the process. | Existing owning module |

A situation is the persistent customer-process record. Applying a playbook attaches its version and progress to that record; do not create a parallel “case,” “enrollment,” and “success plan” representing the same work.

Beacon is a reusable Agent definition, not one shared customer conversation. Each bound run has isolated, permission-filtered process context and the relevant job skill. Skills provide specialized guidance, not another customer lifecycle, approval policy or scheduler. Durable progress remains in CRM and canonical linked records.

The runtime connection uses a private, immutable configuration identity without creating another saved Helpin Agent. Guarded dispatch uses the existing Agent Runtime; callbacks and complete captured skill packages require a server-owned run binding and current authority. Ordinary launch/approval routes reject reserved Playbook context. Publication, live activation and starting an existing Signal remain separate explicit decisions.

One situation can reference multiple evidence items, proposed actions, and runs. It has one primary commercial category; related motions remain context, not duplicate rows. The same evidence can inform separate situations; different customer requests or deals must not be merged just because they share a company, category, or conversation. Completing one does not complete the others.

Use conservative, deterministic linkage: a known request, proposal, deal milestone, or explicit existing situation reference. Otherwise keep work separate and offer contextual links. No AI clustering service is required. IDs must survive changes in owner, priority, title, and grouping.

Separate three roles:

- **Account owner:** Owns the relationship; unchanged by situation assignment.
- **Situation owner:** Accountable for moving this objective forward. Initialize using current motion-aware ownership, but persist explicit reassignment or explicit Unassigned. Do not silently change renewal/expansion ownership defaults.
- **Next-action owner / approver:** Responsible for a particular commitment or decision. Can differ from the situation owner. Missing or inactive owners become visible routing exceptions, not silent workspace-wide assignments.

The queue's “Mine” scope includes situations I own and situations with an open action or approval assigned to me, deduplicated. “My teams” uses active membership and those same responsibilities. “Unassigned” includes missing situation ownership or an actionable responsibility gap, identifying which is missing. The Owner cell names the situation owner; the next-step line names a different action owner when applicable. Display, filtering, counts, and backend authorization must use the same resolver.

## 3. Lifecycle and execution contract

Keep the following dimensions separate; the table specifies semantics, not mandatory enum names.

| Dimension | Meaning |
| --- | --- |
| Customer progress | Open, paused, or closed. Closing records an outcome: achieved, not pursued, invalid, or duplicate, with supporting evidence or an explicit human assessment. |
| Attention | Derived from pending decisions and work: needs context, needs approval, follow-up due, waiting on customer/work, or automation failed. A primary next step does not hide other pending approvals. |
| Approval | Pending, approved, rejected, expired, or replaced by a newer proposal. Approval is permission, not execution success. |
| Execution | Uses existing execution/run states. A launch, delivery, partial failure, and confirmed completion must be distinguishable. |
| Evidence review | Reviewed/dismissed/superseded evidence retains its own actor, revision, and history. Opening a drawer changes none of these. |

Normal sequence:

`Evidence or customer event → identify situation → assign → prepare next action → approve if required → execute → observe result → continue or close`

Required behavior:

- Manual handling follows the same CRM lifecycle without requiring a run. An explicit human outcome is labeled as such; it does not fabricate execution evidence.
- Rejecting a proposed action does not dismiss its evidence or close the situation. Dismissing one source does not dismiss other valid sources. Reevaluate the next step and retain the reason; do not regenerate a rejected proposal unchanged.
- Approvals bind to the exact target, payload, evidence revision, and authorization scope. Material edits or changed context require reevaluation and, when necessary, new approval. Recheck permission and applicability at execution time.
- Reuse existing approval UI interactions for Agent-backed work while keeping CRM suggestions/actions as their canonical decision records. Do not generate duplicate CRM/Agent approvals or fake Agent runs for deterministic actions. Each action has one authoritative approval record, even when displayed in CRM, Automation, or the Dock.
- Unsupported suggestion types must offer an honest manual decision or explain what is unavailable; they cannot report a successful execution without an action handler.
- Retry only an unconfirmed operation. Persist a stable action identity and use provider idempotency/reconciliation where available. An ambiguous email-send result must be reconciled or escalated, not blindly resent.
- Notification delivery, situation creation, task creation, and outbound communication have separate duplicate-prevention scopes. Receiving a notification does not consume permission to perform a later action. An unrelated task on the same account must not suppress work.
- Task completion, a customer reply, or source supersession prompts reevaluation; none automatically resolves every linked situation. Repeated evidence and score decay alone do not reopen closed work. A materially new event may open a new episode linked to prior history, under the configured policy.
- Pausing one situation blocks new automated side effects and attempts to cancel pending scheduled actions; completed or irreversible actions remain in history. Disabling a definition stops new enrollments, with separate explicit controls for active situations. Critical shared safety policies still apply to already-running work.
- Pin immutable effective execution configuration: Playbook policy, Flow/event binding, Beacon preset, selected skills and their resolvable dependencies. A mutable Flow or Agent version ID is not a snapshot. Publishing new settings does not silently rewrite active commitments; adoption is explicit. Future starts use the selected published connection, not arbitrary later Agent edits.
- Shared scheduling, event delivery, retries and runtime launch stay in Automation; approvals retain their canonical action contracts. CRM persists business facts and permitted transitions, not a private executor. Implement durable waits, reevaluation, cancellation and recovery without requiring an ordered-step Flow graph. Beacon can adapt the next permitted action; the system validates progress and enforces policy.
- Every active process has a useful next action and responsible party, a genuine wait with checkpoint/expiry and escalation owner, or a visible blocker. Run completion, failure or exhausted AI budget cannot erase that obligation. Expired approvals halt/escalate; silence does not grant approval.
- Use cheap deterministic checks and coalesced material events before invoking AI. Retain bounded scheduled checks for silent customers and deadlines. A fresh check is not fresh customer progress: it must not reset overdue commitments or approval expiry. Repeated no-progress attempts escalate under policy instead of looping or inventing work.

## 4. Configuration and daily experience

### Product language

Use familiar CRM terminology throughout the product: **Signals, Deals, Contacts, Companies, and Playbooks**. Extend the existing CRM options in Flows and Agent configuration; do not introduce a separate “Customer work” or “Custom work” category or builder. Customer situations remain an internal tracking model, not a new concept users must learn.

Use **Owner**, **Next step**, **Needs approval**, **Waiting for a reply**, **Check again on**, **Pause**, and **Resume** where those labels accurately describe the behavior. Keep execution IDs, revision checks, event delivery, and continuation details in technical diagnostics, not routine configuration copy. A raw signal and the tracked objective it informs still have separate identities; simpler language must not change their meaning.

Signals surfaces decisions and follow-up. Playbooks configures the customer process. Existing Flows connect events/checkpoints to execution, and Beacon supplies specialized capabilities through relevant skills. Only expose new options when their complete execution and safety contracts are implemented; a familiar label must not disguise an unsupported action.

### Playbooks page and configuration

Provide a dedicated **CRM → Playbooks** page. Signals answers “What needs my attention?”; Playbooks answers “What customer processes are we running?”; Automation explains and controls their execution. These are connected views, not separate automation systems.

The Playbooks page must support:

- **Discover and configure:** Ready-to-configure buying-intent, handoff, and renewal-recovery definitions; understandable objectives, eligibility, ownership, milestones, and approval policies. Adapt definitions to customer segments and existing pipelines without writing prompts for standard setup.
- **Publish and control:** Draft changes, preview eligible customers and intended actions without side effects, validate prerequisites, publish, and enable/disable enrollment. Separately pause or resume active customer work; never confuse those controls.
- **Track active customers:** See each participating customer, responsible owner, progress, next action, and blockers. Open the same situation used by Signals or the customer record; no parallel queue or copied process state.
- **Manage an individual process:** Apply a playbook to an eligible customer with explicit confirmation; reassign responsibility, pause/resume, or close with an outcome. Preserve permission checks, customer history, and exactly-once enrollment rules for the relevant process episode.
- **Understand results:** Show completed outcomes and unresolved exceptions, with access to supporting evidence and execution history. Do not present completed runs as successful customer outcomes.
- **Inspect execution:** Open linked Flows and Agents for advanced configuration and troubleshooting. Business policy and execution configuration remain connected by explicit references, with one authoritative owner for each setting.

The catalogue's defined journeys are complete, usable processes, not demonstration-only templates. Existing commercial motions remain available underneath the four agreed Signals navigation categories; neither the grouping nor the three journeys replace or restrict the underlying signal taxonomy.

A standard template asks only:

1. Which customers and trigger qualify?
2. What customer outcome and observable milestones are expected?
3. Who owns the process, decisions, and escalations?
4. What can run automatically, what needs approval, and what must never happen?
5. When should it pause, stop, retry, or ask for help?

Pipeline/lifecycle mappings, ownership defaults, customer segments, and source configuration remain shared CRM settings. Tool access, executor selection, advanced Flow behavior, budgets, and operational history remain Automation concerns. Do not duplicate conditions in an editable prompt and a settings form; typed policy is authoritative and server-enforced. A combined setup may edit both owned configurations through explicit references and versioning, not synchronized copies.

Default to draft preparation and human-reviewed outbound communication. Explicit policies can authorize supported, bounded automatic actions; the final product must enforce those policies and approval boundaries end to end. Financial terms, unsupported promises, sensitive changes, and external messages require explicit policy. A confidence score alone never grants authority. Customer content and retrieved Docs are evidence/reference material, not instructions that can override permissions or policy.

Contact limits and conflicting-process checks apply across automations: for example, a serious unresolved escalation can hold an upsell message for review. Permissions, data freshness, and contact restrictions are rechecked before sending. Optional module absence cannot block unrelated work; a genuinely required missing dependency pauses only the affected action with an explanation.

### Connected Flows and Beacon skills

Keep the existing Playbook **Signals / Setup / Activity** tabs, with an Automation section in Setup. Standard journeys select Beacon and the relevant job skill; an explicit setup action prepares reviewable connected configuration. Users need not write prompts or manually build a Flow for a supplied journey. Show one contextual primary action and clear readiness/blocker information, not a second tab strip or a bank of switches.

The linked Flow keeps **When / If / Then / Using / On** and the existing CRM record labels. It explains which event or checkpoint requests a Beacon run, with the related customer and process context. Business settings are read-only references back to Playbooks. There is no required step-ordering editor. Ordinary Flow Save and standalone Run now remain unchanged; they do not implicitly enroll customers or alter published connections.

Beacon's system skills remain preset-owned; its ordinary update path rejects skill changes. The guarded Playbook runtime binding supplies the captured core and matching job skills without changing shared Beacon settings. It captures an immutable effective snapshot at connected publication and selects only relevant job guidance per run. Use a separate Agent only when genuinely different permissions, capabilities or instructions justify it.

Agents and Flows remain available for authorized advanced configuration. The existing Dock may provide optional conversational access to the same Signal, decision and run records; it is not the only way to operate a Playbook. The [automation plan](crm-playbook-automation-change-proposal.md) owns the detailed setup, publication and execution contract.

### Daily workspace

Keep **Signals** as the page label. The table lists customer situations, standalone actionable recommendations, and commercially relevant evidence grouped by customer and motion—not raw observations or Agent runs. Linked evidence/recommendations stay with their canonical situation instead of duplicating rows. Untracked evidence groups remain visible without an automation-routing policy or activation-ready rules; viewing them creates no situation or playbook enrollment. Raw signals remain inspectable as evidence and on existing entity surfaces.

**Agreed category navigation:** All, Sales, Onboarding & adoption, Expansion, Retention. Use “Sales,” not the earlier proposed “New business” label.

| Visible category | Underlying commercial motions | Customer objective |
| --- | --- | --- |
| Sales | Prospecting + Conversion | Win new customers. |
| Onboarding & adoption | Onboarding + Adoption | Help customers get started and realize value. |
| Expansion | Expansion | Grow an existing account. |
| Retention | Renewal + Retention | Keep customers and address renewal risks. |

This is a presentation grouping, not a data or automation migration. Preserve the original commercial motions and signal types for playbook triggers, detailed filters, reporting, interpretation, and motion-aware ownership. Category tabs and the table's Category column use the same four-group mapping; “All” is the unfiltered view, not a fifth category. Grouped counts use distinct matching inbox rows, including standalone recommendations, under the existing permission/filter rules; they are not summed evidence, motion, or distinct-customer counts. Unclassified work remains available in All without inventing a motion. Preserve legacy motion-filter links without silently broadening their scope. Do not merge customer situations, hide work, reassign owners, or change automation behavior because categories are grouped. Assignment/team filters continue to answer who is responsible; categories describe the customer objective.

- Preserve the agreed fixed Signal/Customer/Category/Owner/Priority columns, with the next step on the secondary line. Never hide columns as filters change.
- Category is the only tab-style selector. Assignment, work state, evidence review, priority, and additional supported filter controls remain directly in the main toolbar, not inside “More filters.” Distinguish evidence review from action approval in labels.
- Default to Everyone + Needs attention so existing evidence and unassigned work remain discoverable. Preserve explicitly selected personal/team filters; an empty personal view must not silently broaden itself. Under Mine, an owned situation waiting solely on a colleague is Waiting, not automatically “needs my attention.” All open, waiting, automatically handled, paused, and closed work remain reachable through the visible work-state filter.
- Filter, count, rank, and paginate situations, standalone recommendations, and untracked evidence groups together on the server. Category counts honor all filters except category. Distinguish inbox-row, participating-situation and approval totals. Aggregate before pagination; never calculate global totals from a fetched page.
- Retain business-priority ranking; do not replace urgency with category order or extraction confidence. Search and evidence filters must retain authorized opposing context without including unseen items in an action.
- The drawer leads with what changed, the proposed next action, and essential evidence. One clear primary action; deeper history, diagnostics, and execution detail are available on demand. Show source excerpts when they change the decision, not repetitive AI summaries.
- Multiple pending actions remain visible and individually scoped. “Approve response” is not “approve everything for this customer.” Preserve queue filters and position on return.
- The old Review route becomes an approval-filtered entry to the same workspace only after parity is verified. Preserve deep links and standalone suggestions without requiring a supporting signal or enabled playbook. No additional approval tab strip is needed.
- Keep distinct loading, empty, error, stale-data, and restricted-access states. No ambiguous title dots, redundant queue-summary copy, or decorative metrics.

The historical desktop mock illustrates all three journeys and the Helpin style, but its simulated Flow steps, Agent naming and configuration are not the authority for this revised connection. It is not proof of implemented capability. This documentation pass does not alter the mock. Extra mobile/keyboard mock work remains out of scope; production accessibility is a completion requirement.

## 5. Three required end-to-end journeys

All three journeys must be implemented, configurable, and verified before the agreed product is considered complete. They are also the reference scenarios for architecture and mock review.

### A. Buying-intent follow-up

- **Entry:** An activation-approved buying-intent event linked to a permitted CRM identity. Match a known request/deal situation or create a separate one; enrich it with later related evidence without duplicating outreach.
- **Owner:** Existing deal/account routing, with explicit situation assignment and visible exceptions.
- **Automatic preparation:** Gather relevant conversation/deal context and prepare a grounded response and next-step recommendation. Check recent replies, existing commitments, and active outreach first.
- **Human action:** Review/edit the draft in the existing permitted communication path. Approve a supported deal change separately if needed. Create a PM task only for a deferred commitment requiring an assignee, deadline, or coordination.
- **Continuation:** Observe a reply, completed commitment, or due checkpoint. Stop stale nudges when the customer responds. A new reply may require a new proposal; it is not blanket authorization to continue sending.
- **Outcome:** A qualified next step is agreed and captured, or the opportunity is explicitly not pursued. A sent email or newly created task is not the outcome. A resulting deal remains governed by its existing pipeline.
- **Failure test:** Sending fails or its result is uncertain after approval. Keep the situation visible with the actual delivery state; do not falsely complete it or create a second response.

### B. Sales-to-success handoff

- **Entry:** A deal is confirmed won, under an enabled handoff policy. Carry forward customer objectives, sold scope, promises, stakeholders, and unresolved dependencies.
- **Responsibility:** Sales remains accountable for the handoff until the receiving success owner accepts. Reassignment preserves that history.
- **Work:** The agent prepares a concise handoff. The CSM confirms missing details and a customer-specific first-value milestone. Link existing onboarding work; create PM tasks only for actual deliverables. A Doc is optional, not the handoff itself.
- **Outcome:** The receiving owner accepts a usable handoff. The subsequent onboarding objective has its own completion criteria; it does not become successful merely because handoff completed.
- **Failure test:** No CSM exists, a sold commitment is ambiguous, or the customer is not ready. Route the exception; do not create an ownerless task checklist or invent dates.

### C. Renewal-risk recovery

- **Entry:** A configured renewal checkpoint or credible risk event. Preserve the specific contract/deal and risk context; no generic “one process per company” assumption.
- **Owner:** Configured renewal responsibility, with separate CSM, commercial approver, and specialist commitments where needed.
- **Work:** Prepare the evidence and recovery proposal. Link an existing Support issue or PM deliverable if it is the blocker; use relevant Docs when available. Hold conflicting expansion outreach according to shared policy.
- **Continuation:** Reevaluate when the blocker changes, the customer replies, or the next checkpoint arrives. Routine background progress should not repeatedly demand review.
- **Outcome:** Record whether the risk was resolved and whether renewal was confirmed as distinct milestones, using authoritative evidence or a labeled human confirmation.
- **Failure test:** A task closes but the customer still reports the problem, or the renewal date changes. Keep the goal open and update the plan; do not infer success from technical completion or stale timing.

## 6. Capability parity and migration gates

| Existing capability | Required destination / treatment |
| --- | --- |
| Motion/category discovery, priority, owner/account/domain/type/date/trust filtering | Unified situation query, directly accessible categories, and inspectable original evidence; preserve all supported filter capabilities. |
| Source inspection, account/meeting briefs, entity signal panels, reviewed/acted/dismissed feedback | Existing evidence surfaces and drawer detail. Preserve IDs, immutable interpretations, reasons, actors, and timestamps. |
| Review types: deal creation, stage change, follow-up, enrichment, risk | All remain discoverable, including suggestions with no signal link. Preserve supported edits, action previews, target navigation, dismissal reasons, and permissions. Distinguish executable actions from manual assessments. |
| Pending approval actions and pagination | Unified approval-filtered entry, complete server-side search/counts, exact decision scope, and surfaced execution failures. |
| Deal health, record search, email/import/autonomy setup | Retain reachable destinations outside the primary work list; explicitly relocate redundant summary panels rather than remove underlying capabilities. |
| Rule versions, promotion, routing, precision/outcome reports, shadow controls | Preserve admin configuration and diagnostics. UI consolidation never bypasses activation or changes historical evidence meaning. |
| Existing tasks and cross-module links | Open and operate on canonical records with owning-module permissions; no cloned tasks or implicit cascade completion. |

Use additive schema changes and deterministic mappings. Never bulk-convert historical `acted_at` or accepted suggestions into achieved customer outcomes. Preserve prior decisions and actual execution status. Closed historical items remain historical; backfill cannot send messages, create tasks, start agents, or replay old automation.

Map standalone actionable signals/suggestions conservatively; record explicit relationships before consolidating them. Diagnostic/context-only evidence remains accessible without automatically enrolling it in work. Validate the complete replacement behind a reversible cutover gate; compare old/new counts and access paths. Keep legacy entry points until parity checks pass, then preserve their URLs as compatible entries into the unified workspace. A migration safety gate is not a reduced product release. Do not change existing automation authorization during migration without an explicit policy change.

## 7. Complete scope and engineering dependencies

The implementation connects the existing [Flow model](../server/internal/model/automation_rule.go), [Beacon preset](../server/internal/agentcontract/skill_catalog.go), [Agent service](../server/internal/service/agent.go), and [shared scheduled-event service](../server/internal/service/automation_scheduled_event.go) through the guarded Playbook path. Legacy signal-to-task templates do not define this product's default behavior; a task is only for real work.

### Implemented branch and deployment boundary

- Unified Signals keeps the four agreed category groups, visible assignment/status/evidence filters, a scanning table, focused drawer, standalone recommendations and the Review compatibility redirect. Counts and pagination remain server-owned.
- Playbooks have Signals / Setup / Activity, immutable policy versions, explicit participation, responsibility routing, human-assessed milestones and truthful customer outcomes.
- Setup prepares a dedicated Playbook-managed Flow with the existing Beacon, captures core plus the matching job skill, reviews actual saved settings, publishes an immutable connection, and separately enables automation. Existing Signals are started/resumed explicitly; updated connections do not silently replace existing bindings.
- Fresh qualified Signals and newly confirmed won deals use the shared eligibility/entry path. Enabling does not backfill. Missing owners and overlapping matching Playbooks require review. Sales remains responsible until the designated Success owner accepts the handoff.
- Durable scheduled events and bounded maintenance launch normal Agent Runtime runs, serve exact captured packages and narrow typed commands, recheck current permissions/billing, cancel revoked work and reconcile lost start responses without duplicate runs.
- Email, real PM task, handoff, milestone and deal-stage proposals use the canonical CRM suggestion queue. Exact human approval precedes side effects. Action completion is not customer success; milestone assessment and outcome recording remain explicit.
- Pending approval, uncertain results and linked work use cheap checks. Caps, escalation routing, stop conditions and visible blockers preserve follow-through. Gmail and local adapters inspect exact results; human inspection is separately attributed and cannot retry an operation.
- Playbooks Activity projects existing publication/gate/run records. Managed Flows link back to Setup, and Beacon shows read-only Playbook dependencies. Shared components and tooltips retain the Helpin design language.
- Optional PM work is created only for an approved real deliverable. No required PM project/task, new Agent per journey/customer, forced Support/Docs usage or independent CRM executor is introduced.

### Verification and operational handoff

The [connection plan](crm-playbook-automation-change-proposal.md) and [CRM reference](crm-signals.md) describe exact API, permission and recovery contracts. Verification covers isolated SQLite/PostgreSQL suites, all-three-journey runtime launch and unknown-start recovery, canonical human-confirmed progress, Gmail send identity/inspection, ordinary Flow regression tests and 37 mocked-browser acceptance tests.

This is branch implementation and isolated verification, not a live deployment claim. Apply the SQL migrations through the normal release process before application/worker rollout, then review and explicitly activate each workspace's Playbooks. No historical Signal enrollment or live provider send is part of migration. Independently configured legacy sequences, unrelated Flows and external senders remain outside Playbook-owned action coordination; their configuration is not silently changed.

Deliberate boundaries of this product scope, not deferred promises: autonomous negotiation, unrestricted sending, new provider integrations, AI clustering, mandatory PM projects, a second general-purpose journey builder inside CRM, or new agents merely to mirror each commercial category. A complete release means finishing the agreed experience, not adding every possible sales or success feature.

## 8. Acceptance and measurement

Release requires evidence that:

1. A CSM reaches their retention decisions directly; situation owner, action owner, filters, category counts, and pagination agree across multiple pages.
2. Two unrelated requests for one account remain independent, while replay of one event creates neither duplicate work nor duplicate actions.
3. A reviewer edits and approves one exact action; another reviewer cannot execute it again. Changed evidence or access invalidates stale authority.
4. Unsupported suggestions never claim execution success. Failed and ambiguous side effects remain visible and recover without blind repetition.
5. Manual work and reassignment preserve process history without requiring an agent, task, or playbook.
6. Customer replies cancel obsolete follow-ups; completed tasks trigger reevaluation rather than false resolution. A paused process cannot resume side effects through a stale scheduled event.
7. Restricted evidence, inactive owners, missing integrations, and exhausted AI budgets produce scoped explanations without leaking data or making unrelated manual work unusable.
8. Backfill and rollback preserve original evidence, decisions, and links and never replay external actions. Disabling new enrollment and pausing active work behave differently and visibly.
9. An authorized CRM administrator configures and publishes each of the three Playbooks, with Automation permission checked for the connection, validates scope without side effects, controls enrollment separately from active work, and follows participating customers through to outcomes. No new manager-role permission is implied.
10. Sales-to-success handoff retains promises and ownership until acceptance; renewal recovery distinguishes blocker resolution from confirmed renewal. Neither journey is a placeholder or depends on an unbuilt follow-through capability.
11. The four agreed Signals categories cover all seven original commercial motions. Grouped navigation, Category cells, filters, and distinct situation counts agree; original motion filters, reporting, playbook triggers, ownership, and independent situations retain their meaning.
12. A supplied Playbook uses Beacon with the relevant skill through an inspectable connected Flow, without requiring a new Agent, custom prompt or ordered-step editor. Customer contexts remain isolated; saved edits cannot silently change pinned execution.
13. Silent customers still receive bounded due checks. Unchanged checks do not reset overdue work or count as progress. Exhausted retries, AI budgets and cancelled runs leave a visible obligation, responsible person or blocker; they never silently abandon an active objective.

Measure time to a qualified next step, handoff completeness, renewal outcomes, time waiting on human decisions, overdue or ownerless work, failed/duplicate actions, and human effort and execution cost per completed situation. Where first-value evidence is available, track it independently from handoff acceptance. Outcome reports show unavailable or not-yet-observed results honestly. Track unnecessary outreach, repeated no-progress runs and overrides as quality costs. Do not equate task/check/run volume with productivity or attribute revenue changes to automation without suitable comparison data.

## 9. Agreed decisions and implementation boundary

Confirmed as of 2026-09-07: one complete delivery, a dedicated CRM Playbooks page and all three journeys. These decisions supersede older conflicting proposals:

- Confirmed category decision: Sales (Prospecting + Conversion), Onboarding & adoption (Onboarding + Adoption), Expansion, and Retention (Renewal + Retention), with All as the unfiltered view. The original motions remain available underneath; “New business” is not the agreed label.
- Keep the Signals name and use the agreed grouped category navigation; Review becomes an approval-filtered entry with compatible legacy URLs after parity checks.
- Deliver buying-intent follow-up, sales-to-success handoff, and renewal-risk recovery together. Default to human-reviewed sending, with explicit bounded automation policies.
- Keep waiting/automatic work accessible through the visible work-state filter, while the default focuses on decisions and due actions.
- Allow CRM-only manual operation. PM tasks are created only for an explicit commitment and only with appropriate permission and policy.
- Include a dedicated Playbooks page for configuration, active customers, progress, controls, and outcomes, backed by shared Automation rather than a separate execution system.
- Keep Flows as the event/checkpoint connection and use the built-in CRM Agent Beacon with relevant specialized skills. No required ordered-step Flow builder, Agent per Playbook/customer, or per-run shared Agent reconfiguration.
- Let Beacon adapt useful next actions while the system enforces approvals, durable follow-through, live authority, milestones and truthful outcomes. Skills guide jobs; they do not become a second workflow or progress store.

The [automation plan](crm-playbook-automation-change-proposal.md) records the implemented extension, safeguards and verification boundary. The schema/API, preset/runtime skill binding, immutable snapshots, event/checkpoint coordination and provider reconciliation are implemented and tested in isolation. Production deployment, migrations and customer activation remain separate authorized operations.

Documentation was reconciled before starting the specialization/context foundation. Obtain confirmation before changing existing Flow/Agent builders, configuration or behavior. Direction agreement does not authorize changing/enabling saved customer automations, expanding access/tools/budgets, enrolling customers, replaying history, running production migrations, deployment or customer-facing actions.

## Research context

Previously inspected primary documentation and workflow screenshots inform the direction, not a requirement to copy another product:

- [Attio workflow creation](https://attio.com/help/reference/automations/workflows/create-a-workflow): editable drafts, explicit publication, and version behavior for active runs.
- [Attio workflow blocks](https://attio.com/help/reference/automations/workflows/workflows-block-library): agent outputs and explicit downstream actions.
- [Planhat workflows](https://help.planhat.com/en/articles/9587102-workflows-overview): customer-linked projects/sequences and adaptable milestones.
- [Planhat human-in-the-loop](https://help.planhat.com/en/articles/15535091-human-in-the-loop-hitl-in-automations): step-level review and continuation; the inspected documentation marks availability as Labs-gated.

No live customer credentials or private customer examples are included in this document.
