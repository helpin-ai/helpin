# Connecting Playbooks, Flows, and Beacon

Date: 2026-09-08 · Branch: `waqar-fixes`

Status: **Implemented on `waqar-fixes`; deployment is separate.** Guided setup, explicit activation, durable execution through the existing Agent Runtime, exact CRM approvals and result inspection are connected on this branch. Ordinary saved Flows and Agents are not rewritten. Application migrations, production activation and live customer sends have not been performed.

This is the agreed implementation direction for the [CRM blueprint](crm-customer-work-blueprint.md). Its filename is retained for existing links; the earlier proposal requiring ordered Flow steps is superseded by this document. The [CRM reference](crm-signals.md) and [Automation reference](AGENTS_AND_AUTOMATION.md) describe implemented behavior. The [Automation product model](AUTOMATION_PRODUCT_MODEL.md) remains the shared foundation. All three customer journeys are required for one complete delivery.

## 1. Agreed architecture

**Playbooks define the customer objective and rules. Flows trigger the work. The existing built-in CRM Agent, Beacon, performs specialized jobs through skills. CRM records what actually progressed.**

Reuse Beacon by default. Buying-intent follow-up, sales-to-success handoff, and renewal-risk recovery use relevant specialized skills, not a mandatory new Agent for each Playbook, category, or customer. A skill supplies job-specific methods and guidance; it is neither the customer's progress record nor an independent scheduler. A separate Agent is justified only by genuinely different capabilities, permissions, or instructions.

Keep the existing Flow mental model: **When / If / Then / Using / On**. Connect relevant customer events and scheduled checkpoints to normal Agent runs. These journeys do not require an ordered-step editor, a new Flow canvas, or a second CRM workflow engine. Shared scheduled events provide durable coordination, including waits, approvals, scheduling, cancellation and recovery.

Beacon chooses the next useful permitted action using current evidence and the Playbook objective. The system enforces eligibility, permissions, approvals, milestones, deadlines, and duplicate prevention. Neither an Agent run nor a claim in its response can advance customer progress without validated evidence or a labeled human assessment.

Most sales and success users configure a supplied Playbook and work from Signals. Standard setup prepares connected configuration after an explicit user action; users need not assemble Flows or write prompts. Linked Flows and Beacon remain inspectable in Automation. The existing Dock can offer conversational access to the same records, not a competing process.

## 2. Implemented capability and boundaries

| Area | Branch implementation |
| --- | --- |
| Playbooks | Immutable published business policy; manual participation and human-assessed milestones; explicit automation activation and per-Signal start/resume/pause. |
| Flows | Guided setup creates a disabled, dedicated Playbook-managed Flow using the existing CRM grouping and Beacon. Its live gate belongs to Playbook Setup; generic editing, toggling, deletion and direct execution cannot bypass it. Ordinary Flows retain their behavior. |
| Beacon | The existing built-in CRM operator serves all three journeys with captured core plus job skills. No saved Agent per Playbook/customer. “Used by Playbooks” links reviewed dependencies without exposing private instructions. |
| Configuration | Append-only `crm_playbook_connections` capture the effective saved Beacon configuration, host limits, Flow settings and complete skill bytes. Publication starts nothing. Explicitly saving an updated connection affects newly started Signals; existing bindings stay pinned. |
| Scheduling | The existing scheduled-event worker owns leased, deduplicated entry/check events. Fresh qualified evidence and newly confirmed won-deal transitions enter through shared eligibility. Maintenance observes CRM updates, action decisions and linked task completion without requiring a model run every poll. |
| Approvals | Typed email, real PM task, handoff, milestone and deal-stage actions use the existing CRM suggestion queue. Exact content, actor, revision, policy, expiry, destination and current facts are checked before execution. Historical manual recommendations keep their original semantics. |
| Recovery | Stable host-run and action identities prevent duplicate launch/send/create. Gmail reconciliation is read-only; local changes use canonical receipts; task inspection repairs only links to the existing task. Unknown outcomes never trigger blind retries. |
| Activity | Paginated projections of connection publications, activation/adoption receipts and normal Agent runs; canonical Signal evidence, decisions, milestones and outcomes remain separate. No private prompts, packages or raw runtime output are exposed. |

Implementation details:

- Migration `202609070002` keeps connection receipts permanently `execution_enabled = false`. Live gates are separately owned by `crm_playbook_automation_settings` and per-Signal bindings in migration `202609080001`. Enabling never scans or adopts historical work.
- Guided setup reuses the saved Beacon without preset reconciliation. A deterministic private runtime profile holds each frozen configuration; permissions, billing and activity retain the real Helpin Beacon ID.
- Runtime launch, early callbacks, exact skill/package delivery and typed commands require a durable server-owned run binding and current tenant, target, policy, generation, member, module and billing authority. Request metadata never grants authority.
- Generic Agent launch/resume/approval/continuation paths reject reserved Playbook context. The existing Agent Runtime is the only executor; a generic Agent approval cannot approve a CRM action.
- Migration `202609080002` stores exact action intents alongside canonical suggestions, not a second approval lifecycle. Only an explicitly reviewed action reaches existing email, PM and CRM services.
- Migration `202609080003` records a confirmed won-deal transition and its shared-entry wake-up in the deal transaction. Repeated updates do not duplicate open handoff work; reopened deals fail entry eligibility. Sales remains accountable until the account's designated Success owner accepts.
- Automatic routing resolves the published responsibility role at entry. Missing roles and overlapping eligible Playbooks require human review; neither an arbitrary Playbook nor an admin fallback is chosen.
- Waiting for approval, an uncertain result or linked work uses cheap scheduled checks. Daily/run-without-progress caps and escalation keep stalled work visible. Explicit stop rules suspend execution; a model response never closes the customer objective.
- New email actions are limited to one linked contact from the approving user's connected Gmail mailbox. Typed Playbook sends share an unresolved-intent/recent-outreach guard, including recent CRM-recorded manual email. This does not take control of independently configured legacy sequences, unrelated Flows or external senders.
- The original approver can record a clearly attributed inspected result after the five-minute in-flight safety window. This records evidence; it never retries or performs the original operation.
- Custom journeys remain manually usable. Handoff automation requires the `handoff_accepted` milestone and a real receiving Success owner. PM, Support and Docs are not prerequisites for unrelated work.

Code anchors: [execution](../server/internal/service/crm_playbook_execution.go), [existing-runtime launcher](../server/internal/service/crm_playbook_agent_launcher.go), [guarded callbacks](../server/internal/service/crm_playbook_runtime_host.go), [canonical actions](../server/internal/service/crm_playbook_actions.go), [owning-module adapters](../server/internal/service/crm_playbook_module_executor.go), [maintenance](../server/internal/service/crm_playbook_maintenance.go), [native won-deal entry](../server/internal/repository/crm_playbook_deal_entry.go).

## 3. Exactly what users will see

### Playbooks: guided setup, not another builder

Keep the existing **Signals / Setup / Activity** tabs. Add an **Automation** section inside Setup; no additional tab or “Customer work” category.

| Control | Behavior |
| --- | --- |
| Start | “Manually” or “When customers qualify.” Manual by default. Automatic entry uses published eligibility and supported CRM events, not an unrestricted historical sweep. |
| Connected automation | Named links to the connected Flow and Beacon, with the selected job skill summarized in familiar language. “Set up automation” prepares a reviewable draft through an explicit action. Advanced users can open the existing configuration surfaces. |
| Status and primary action | Plain status such as “Not connected,” “Ready to enable,” “On,” or “Paused,” with the specific blocker. Show one contextual primary action, not a bank of activation/enrollment/publication toggles. |
| Preview | Read-only matching examples, expected responsibilities, permitted actions, approval requirements and blockers. Distinguish existing eligible Signals from event-driven examples. No actual runs, sends or tasks. |
| Enable automation | Explicit confirmation of published policy, connected configuration, entry scope, permissions and required connections. Existing Signals are not automatically enrolled or switched from manual handling. |

Manual entry and automated follow-through are separate choices. A team can add a customer manually and explicitly start the connected automation, or keep the Playbook entirely manual. “Stop new enrollment” and “Pause active work” remain different controls.

Keep business settings in their existing Setup sections:

- **Milestones:** explicit human confirmation or supported CRM/linked-work conditions. Typed conditions and authoritative records govern automatic completion, not prose. Handoff acceptance is an explicit receiving-owner decision.
- **Action permissions:** “Needs approval” and “Not allowed.” These three journeys require exact human approval before messages, tasks, handoff acceptance, milestone assessment or deal-stage changes. There is no blanket automatic-send/write setting.
- **Follow-up and stops:** business checkpoints, approval expiry, escalation, contact limits and stopping rules. Flow configuration and skills reference these settings rather than copying them.
- **Optional dependencies:** request an authorized mailbox when messaging is selected, a PM destination for an actual deliverable, and other module access only when needed. Missing optional modules do not block unrelated work.

Supplied journeys select Beacon and the appropriate job skill by default. Show missing prerequisites when relevant, not a permanent warning dashboard. CRM configuration permission does not grant Automation permission; setup must respect both and explain who can complete a restricted connection.

Current UI anchors: [Playbook editor](../frontend/src/components/crm/playbooks/PlaybookEditor.tsx) and [Playbook detail](../frontend/src/pages/crm/PlaybookDetail.tsx).

### Flows: familiar triggers connected to Beacon

Keep the existing Flows page, CRM grouping, and Deal / Contact / Company targets. A Playbook-bound run resolves its customer from the canonical Signal and carries the process reference; no dummy PM task or repeated customer picker is needed.

The connected Flow still reads as a sentence: when the relevant CRM event or checkpoint occurs, if this active Playbook requires work, run Beacon with the selected skill on the related CRM record. The setup may connect multiple supported event sources; it must not create unrelated rules that lose process identity.

Add supported, typed CRM events for Playbook start, customer reply, action decision, material record/linked-work change, and checkpoint due. They are workspace-scoped and correlated to the correct process episode. Deal-won and risk events go through the shared CRM eligibility/activation path before participation; they do not become a second enrollment path.

Show linked Playbook settings as read-only references, such as “Approval: from Renewal recovery.” Technical event binding, runtime limits, retries and connected Agent selection belong in Automation. Business follow-up timing and action policy stay in Playbooks.

**Do not introduce a required ordered list of Run agent / Review action / Perform action / Wait steps.** Shared scheduling and run lifecycle services manage durable wake-ups; Beacon reassesses within the Playbook rules. Approval, execution and progress validation remain explicit backend contracts even when they are not Flow blocks.

Existing ordinary Flow saving, enablement, targets and standalone execution stay unchanged. Publishing a Playbook connection captures an immutable effective execution snapshot. Later Flow/Beacon/skill edits appear as updated settings available for explicit review and publication; saving alone does not change the pinned connection. A guided publication validates all referenced settings together and fails without a partial connection update.

Versioned connected configuration now has separate storage/API, without a general published-version overhaul for all existing Flows. See the [approved setup API](crm-signals.md#approved-automation-settings). Bound execution and activation must consume this contract, not bypass it.

Current UI anchor: [Flow editor and list](../frontend/src/pages/automation/AutomationFlows.tsx).

### Beacon and skills: one reusable specialist, isolated customer work

Preserve the existing Agents page, tools, runtime/model options, permissions, budgets and Run now. Do not introduce separate Sales/Success builders or duplicate eligibility, scheduling and approval settings in Agent configuration.

Required additions:

- Specialized skills for the three journeys, selected through an explicit supported Beacon preset/runtime binding. Each bound run loads only the relevant job guidance and necessary shared capabilities, not every Playbook's instructions.
- Typed, permission-filtered context: customer, objective, current milestones, open decisions/commitments, material updates, relevant evidence, permitted actions and next checkpoint.
- Narrow context/proposal tools through the [shared tool catalog](../server/internal/agentcontract/tool_catalog.json). Existing broad write tools must not bypass the stricter policy of a Playbook-bound run; enforce restrictions at execution, not only in the prompt.
- Compact **Used by** links and run context. Show effective settings and available updates on demand; keep hashes, correlation IDs and detailed diagnostics in Activity.

The same Beacon definition serves many customers through isolated runs. Do not edit its shared configuration per run or mix customer state in a shared conversation. CRM stores durable progress and commitments; the next run reloads authorized current facts.

A skill guides judgment; it cannot grant tool access, authorize sending, mark its own approval as granted, invent completion, or replace scheduling. Separate Agents remain possible when genuinely justified, but are not a setup requirement. Run now stays standalone and does not implicitly enroll a customer.

Current UI anchors: [Agents page](../frontend/src/pages/automation/Agents.tsx), [Agent editor](../frontend/src/pages/automation/CustomAgentCreatePanel.tsx), and [CRM target support](../frontend/src/lib/agentCRMTargets.ts).

### Signals, customer records, and Activity

Keep the agreed fixed Signals table, one category tab strip, and directly visible filters. The drawer leads with the change, next action, essential evidence and responsible person. Reuse the existing composer for reviewing/editing a response.

Show truthful next steps: “Review response,” “Waiting for a reply,” “Follow-up due,” or “Send result needs checking.” Each approval has its own scope. Routine checks belong in Activity, not repeated attention notifications. Expired decisions, missing ownership, uncertain delivery, automation failures and exhausted execution budgets surface actionable blockers.

The customer record, Playbook Signals tab, Automation and optional Dock interaction reference the same canonical Signal/action/run IDs. A deterministic check is not a fake Agent run. Evidence review, approval, execution and customer outcome stay distinct.

## 4. One owner for each setting and fact

| Responsibility | Authoritative location |
| --- | --- |
| Objective, eligibility, responsibilities, milestones, business follow-up, stops, action policy | Published Playbook policy in CRM |
| Event binding, correlated wake-ups, runtime launch, technical retries/limits, connected configuration | Existing shared Automation, extended for Playbook-bound work |
| Job guidance, permitted capabilities, runtime/model behavior | Beacon preset and relevant skills; immutable effective snapshot for bound execution |
| Customer progress, next action, responsibility, outcome | Canonical Signal and linked CRM records |
| Proposed action, decision, execution result | Canonical CRM suggestion/action contract and owning executor |
| Actual AI execution, transcript, usage | Existing Agent run/runtime |
| Message, task, Support issue, document | Existing owning module |

Setup guides users across references; it does not maintain independently editable copies. Live permissions, contact restrictions, tool revocations and workspace spending limits are always stricter ceilings than saved snapshots.

## 5. The three complete journeys

| Journey and Beacon skill | Preparation | Decision and continuation | Observable outcome; optional modules |
| --- | --- | --- | --- |
| Buying-intent follow-up | Activation-approved intent matched to the request/deal episode. Resolve owner, check replies and recent outreach, prepare a grounded response. | Review/edit and authorize the exact action; observe the correlated reply or checkpoint; reassess before further contact. A deal change has its own authorization. | Qualified next step agreed and recorded, or explicitly not pursued. PM only for a genuine deferred commitment. A sent message is not completion. |
| Sales-to-success handoff | Confirmed won deal under enabled policy. Gather sold scope, objectives, promises, stakeholders and gaps. Sales remains accountable until acceptance. | Receiving owner accepts a usable handoff; missing ownership or ambiguous promises escalate. Link relevant onboarding work and a separately tracked onboarding objective when configured. | Explicit receiving-owner acceptance with evidence. Docs optional; PM for actual deliverables, not an automatic checklist. Handoff does not prove first value. |
| Renewal-risk recovery | Renewal checkpoint or credible risk event tied to the contract/renewal episode. Gather evidence and propose recovery. | Resolve CS/commercial responsibilities, authorize supported actions, observe replies/blocker changes/deadlines, and check conflicting expansion outreach. | Evidenced recovery, renewal result or explicit human decision. Support/PM only for real blockers; Docs can inform preparation, never confer authority. |

The Agent may adapt the next action, not invent commitments, customer acceptance, dates, task completion or business outcomes. Rejected proposals are not recreated unchanged. A reply is not blanket permission for further sending.

## 6. Publishing, enabling, pausing, and stopping

| User action | Exact effect |
| --- | --- |
| Save draft | Saves editable configuration only. No enrollment or execution. |
| Publish Playbook / connected settings | Validates and freezes the selected policy and effective Flow/Beacon/skill binding. Publication is not activation and does not rewrite active Signals. Broader action authority requires fresh activation review. |
| Allow new enrollment | Existing admission gate; alone it permits explicit participation, not automatic entry or execution. |
| Enable automation | Separately authorizes the reviewed connection after readiness checks. Automatic entry also requires “When customers qualify” and admission. Existing manual Signals require explicit selection and confirmation. |
| Stop new enrollment | Stops new participation. Active customer work continues under pinned settings unless separately paused. |
| Pause a Signal | Blocks new automated side effects, revokes pending wake-ups, and requests cancellation of bound active Agent runs through the existing runtime. Irreversible actions remain in history. |
| Disable connected automation | Blocks new starts and further bound actions, with impact confirmation and visible pause reasons; requests cancellation of bound active work. This connected-mode safety contract does not silently redefine ordinary Flow enablement. |
| Resume | Rechecks current facts, permissions, decisions, contact limits and remaining work; does not blindly replay attempts. |
| Adopt updated settings for active Signals | Explicit preview and confirmation; preserve history, invalidate stale proposals when necessary, and never redo completed actions automatically. |

Show “Stopping” until cancellation is acknowledged, then “Paused.” An in-flight provider operation may already be irreversible; pausing cannot retract a sent message. Flow and Playbook controls must agree on the same connected activation gate, not leave a hidden path running.

## 7. Execution contract and safeguards

### Durable follow-through without an ordered Flow graph

Every active process must have either a useful next action and responsible party, a genuine wait with a checkpoint/expiry and escalation owner, or a visible blocker. A run finishing, failing, hitting a budget limit, or finding nothing new cannot erase that obligation.

Extend shared Automation coordination to persist the bound configuration, canonical process reference, wake-up reason/correlation, expected material revision, deadline, claim/lease and links to existing action/run records. These are implemented coordination responsibilities in the shared scheduled-event service, live Playbook gates, run bindings and canonical action-intent records. **An ordered-step index or general Flow-program cursor is not required.**

CRM remains authoritative for progress. Normal Agent work creates normal `agent_runs`; deterministic checks and module operations use existing services. Shared scheduled events provide deadline checks, and domain events prompt reevaluation. No private CRM executor, duplicate approval/task system, or copied execution-success ledger.

Use stable workspace-local identities for events, process episodes, reevaluations, proposals and action intents. Coalesce related updates and serialize or safely supersede concurrent work for the same process. Event redelivery, stale runs and concurrent workers must not duplicate enrollment, proposals, sends or tasks. Different requests/deals/renewals remain independent.

### Adaptive and efficient, without losing accountability

Start with cheap eligibility, freshness, due-time and policy checks. Run AI when interpretation, preparation or a changed plan is needed, not for every event or a polling loop over every customer. Keep bounded scheduled checks for silent customers and deadlines; event-only handling would miss these obligations.

“Last checked” is not “last progressed.” An unchanged Agent assessment does not reset overdue commitments, extend approval expiry, or count as customer movement. Repeated no-progress attempts, stale evidence, repeated rejection or missing dependencies escalate under policy rather than looping or inventing work.

Bound run frequency, execution cost, automatic retries and contact attempts. On exhausted retries or AI budget, persist the blocker and required follow-through so the responsible person can act manually or explicitly resume. A finite delivery retry limit is not permission to forget the customer.

### Version-bound execution and live authority

The published connection and each participating Signal pin immutable effective configuration: Playbook policy, Flow/event binding, Beacon preset, relevant skills and their versioned dependencies. Later saved edits do not silently change active commitments or future starts using the existing published connection.

Pass typed process/target references, relevant source IDs and expected material revisions through the existing run input. Reload authorized facts at each run and before side effects. Customer messages, retrieved Docs and external content are untrusted evidence, never instructions overriding policy.

Use existing runtime launch, billing, entitlements, spending limits, workspace/team access and cancellation. Effective authority is limited by both the bound policy and current permission. Missing/deactivated owners or approvers become visible exceptions, not fallback assignments to everyone.

### Exact approval and honest execution

Extend canonical proposals with supported typed operations, executor version, stable action intent, exact target/payload, material evidence/record revisions, approval scope and expiry. Human approval and explicit automatic-policy authorization have distinct recorded bases.

Changes to sender, recipient, message, attachments or material context trigger revalidation and renewed approval when needed. Approval covers one action. Expiry halts/escalates; silence never grants consent. Reject stale or unsupported payloads before execution, including from a previously running Agent.

Old follow-up/enrichment/risk recommendations retain manual semantics; never infer an executable operation from AI text or an old suggestion type. CRM, Agent and Dock surfaces display the same canonical decision, not competing approvals.

For email, reuse authorized mailbox, composer, provider and thread services. Record durable send intent before provider contact; reconcile an uncertain send against provider state or request inspection before retrying. Do not promise exactly-once provider delivery. Sent, delivered, replied and objective achieved are separate facts.

A PM task requires a real deliverable, destination, responsible person and authorized operation. Link suitable existing work or use a stable creation intent. Approval queues, Agent runs and follow-up timers are not PM tasks.

### Coordination across customer contact

Check recent outreach/replies, opt-outs, mailbox access, frequency limits and conflicting processes before automated sends. Shared policy must cover participating CRM automated outbound paths, including overlapping legacy automations; disclose paths outside coverage. Manual messages count as recent contact, without silently enrolling manual sending in a Flow.

Support-derived CRM evidence can inform Beacon's recommendation; this release does not install a global cross-module outreach-suppression engine or modify independently configured senders. Missing optional modules block only actions that require them. Changing overlapping ordinary Flow/Agent authority requires explicit configuration review, not automatic migration.

## 8. Engineering order and acceptance gate

The implementation covers all three journeys in one delivery. Verification includes isolated SQLite and PostgreSQL CRM service/repository/handler suites, all-three-journey runtime launch and lost-response recovery, exact action approval and native CRM adapters, the Gmail adapter/service boundary, and browser acceptance of setup, activation, pinned resume, editing and result inspection. Production deployment and live provider acceptance are not represented as completed by mocked tests.

Final branch verification on 2026-09-08: 37 browser acceptance tests and 58 selected frontend unit tests passed; the frontend production build, API/worker builds, Go vet, isolated PostgreSQL CRM suites and race-enabled CRM/Agent Runtime/scheduled-event/PM regression suites passed. The unrelated existing `TestSupportConversationRepositoryListAssignmentAndSortFilters` still fails its ordering assertion in `server/internal/service/support_inbox_view_test.go:217`; this is not a claim that the full repository suite is green. The disposable PostgreSQL fixture container was removed after verification. No application migrations, real sends, saved-customer activation or deployment were performed.

Acceptance must cover:

- Existing CRM/non-CRM Flows, manual Agent runs, target pickers, saved inputs/defaults, tools, billing, access, navigation and manual email/approval behavior retain parity.
- One built-in Beacon serves isolated customer processes with the correct skill and least-privilege context. No default Agent proliferation, per-run shared-config edits or unrelated skill loading.
- Duplicate/out-of-order events, restarts, stale runs, concurrent approvals and pause-versus-send races cannot fabricate success or repeat actions.
- Saved Flow/Beacon/skill edits do not silently change pinned execution. Explicit adoption and live revocation are distinct and tested.
- No updates still leads to a due check; no progress stays overdue. AI/runtime failure leaves a visible obligation, not silently abandoned work.
- No automatic historical replay, enrollment, customer-facing actions or permission expansion during migration, publication or deployment.
- All three journeys work without optional PM/Support/Docs when unneeded, and surface scoped blockers for genuine dependencies.
- Signals counts, responsibilities, standalone recommendations, approval visibility, deep links, accessibility and low-clutter layout retain their contracts.
- Measure useful customer progress, human effort, unnecessary outreach and execution cost per outcome; raw run volume is not productivity.

Use the existing Quiet Hairline layout and shared forms, query builder, pickers, drawer, composer and Activity. Tooltips explain secondary details; permissions, sending implications and blockers stay visible.

## 9. Agreed direction and implementation boundary

Agreed on 2026-09-07:

- Keep Playbooks, Flows and the built-in CRM Agent Beacon, specialized through relevant skills.
- Preserve familiar Flow/Agent surfaces; no required ordered-step builder, separate CRM executor, or Agent per Playbook/customer.
- Implement durable, guarded follow-through and canonical actions for all three journeys; skills do not replace these guarantees.
- Keep PM/Support/Docs optional and the daily experience focused on Signals decisions and customer progress.

Implementation includes specialization/context preparation, durable approved-setup publication, server-owned dispatch claims tied to the published connection and current customer revision, guarded runtime package delivery, typed customer events/actions and the connected setup UI. Dispatch uses the normal runtime with real Beacon access/billing, checks early callback correlation and reconciles interrupted launches without duplicate execution. Separate readiness-based activation gates enforce those guarantees; a saved publication never grants execution permission. Confirm further changes to existing Flow/Agent builders, configuration or behavior with the user before making them; material departures from this direction require a new decision.

Agreement on architecture is not permission to edit or enable saved customer Flows/Agents, broaden access/tools/budgets, enroll customers, replay history, run production migrations, deploy, or perform customer-facing actions. Those remain separate explicit decisions.
