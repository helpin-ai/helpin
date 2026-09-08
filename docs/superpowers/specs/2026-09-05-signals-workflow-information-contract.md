# Signals workflow and information contract

Date: 2026-09-05

Branch: `waqar-fixes`

Status: **Historical contract; superseded where it differs from the agreed CRM blueprint (2026-09-07).** The body below preserves the earlier proposal, not current implementation instructions or status.

Use the [CRM blueprint](../../crm-customer-work-blueprint.md) for agreed product scope and the [Flow/Beacon/skill connection plan](../../crm-playbook-automation-change-proposal.md) for automation direction. In particular, the earlier individual-signal queue model, separate Review assumptions and proposed workflow behavior must not override the unified customer-situation/standalone-recommendation workspace or the Playbooks → Flows → Beacon + skills direction. The [CRM reference](../../crm-signals.md) describes implemented branch behavior.

## Purpose and scope

Signals helps a team member find relevant customer situations, evaluate the evidence, and decide what needs action. Success means the right person can find an important situation, understand it, and record a decision without searching the entire workspace feed.

This contract follows the code audit and comparison of Pocus, Common Room, Gong, Gainsight, and Vitally. It defines behavior, information, and delivery boundaries. It does not select a screen layout or document existing behavior as if these changes were already implemented. `docs/crm-signals.md` remains the reference for the current system.

Helpin's CRM, PM, Support, and Docs capabilities are available when the situation needs them. Signals must remain useful independently. Reviewing, assigning, handling, or dismissing a signal must not require a task, support ticket, document, or automation.

## 1. Find the right work

- Responsibility, commercial category, and work state are independent filters.
- Provide Mine, My teams, Unassigned, and All scopes, within existing permissions. Mine is the initial scope; an empty Mine view must explicitly offer the other permitted scopes rather than silently switching to All.
- My teams means signals whose effective owner is an active member of one of the viewer's teams. Use canonical workspace-member team membership. Deduplicate members belonging to multiple teams. An unowned signal belongs to Unassigned, not implicitly to every team.
- Every supported commercial category has a directly accessible destination and count before its entries need to be read. Zero and unavailable counts must be distinguishable. Retention must not require scrolling through expansion.
- The queue's ranking and pagination unit is an individual signal. The All-categories view orders these across categories by individual business priority, then latest evidence time, then signal ID. Account/category summaries are contextual detail, not competing ranked rows or pagination units. Category order must not override urgency. Category-specific views use the same signal-level ordering and keep their own pagination.
- Preserve responsibility, category, work state, search, sorting, and paging when returning from a record. Represent shareable filter state in the route; Mine resolves to the person opening the link, not the original sender. Do not share a private scope by bypassing authorization.
- Search must find situations by customer identity and evidence summary. A separate CRM record search is not an adequate substitute.

### Ownership contract

One server-side effective-owner resolver must govern display, filtering, counts, assignment context, and any configured routing. Expose whether ownership is explicit, inherited, a fallback, or absent.

Preserve the existing inherited policy initially: with a linked company, onboarding, adoption, and retention prefer an active customer-success owner, then company owner; other motions prefer an active linked deal owner, then company owner. Without a linked company, an eligible deal owner remains applicable to all motions. Existing configured signal-owner and workspace-owner fallbacks remain visible as fallbacks. Do not silently reroute renewal or expansion to customer success in this release.

An explicit signal assignment overrides inherited ownership without changing the company, deal, conversation, or task owner. Explicitly clearing an assignment must support a genuinely Unassigned state; it must not immediately reapply a fallback. Returning to inherited ownership is a separate choice. Assignment changes record actor and time and require CRM edit permission. Inactive assignees return to the inherited resolution path with an auditable ownership change.

## 2. Define what is actionable

An account can legitimately have expansion and retention evidence at the same time. Preserve these meanings and make related categories discoverable.

For the initial release, retain account/category summaries as context but use existing individual signals as the durable decision units. A summary is not one indivisible task. Unrelated requests, different deals, and different owners must remain individually identifiable and actionable inside that context. Show mixed ownership explicitly rather than choosing one signal's owner as the owner of the whole group.

Do not introduce an AI clustering service or assume all evidence for an account/category describes the same situation. Correlated evidence may be presented together and counted once per source, while preserving individual IDs, original excerpts, ownership, and decisions. A source conversation may contain more than one commercial event; sharing a source does not authorize silently completing all its signals.

If a user acts on multiple signals together, show the selected signals and scope of the action. Never apply a decision to unseen signals or to evidence arriving after the action's evidence snapshot.

## 3. Separate review, work, and evidence validity

| Dimension | Meaning |
| --- | --- |
| Needs review | Current evidence has not been explicitly reviewed by the team. Opening a row or source does not change this. |
| Reviewed | Someone explicitly evaluated this evidence. Store who and when. This alone does not mean follow-up is finished. |
| Open | The signal still needs a decision or follow-up. This is the default work state. |
| Handled | No further signal-level action is needed for the current evidence. Store actor and time; permit an optional outcome note or work link. |
| Dismissed | The signal should not drive work, with a reason such as incorrect evidence, wrong entity, duplicate, or irrelevant. Preserve it in history. |
| Superseded | The system says this interpretation is no longer current. It is historical evidence, not unfinished work. |

Review is shared team feedback in this release, not a personal unread system. The default queue contains current, open signals whether reviewed or not. Handled and dismissed signals remain discoverable separately. Supersession takes precedence over open work in the active queue without erasing prior decisions.

Marking handled also acknowledges review of the evidence being handled. Explicit reopening restores open work without deleting history. New evidence requires review again: newly created signal IDs start Open and Needs review. A materially changed evidence fingerprint on an existing, non-superseded signal starts a new revision in Open and Needs review, including when the previous revision was Handled or Dismissed; preserve the previous decision in history and explain the new evidence. A superseded interpretation never reopens: a valid replacement must be a new signal. Score decay, replay of identical evidence, owner changes, and visual regrouping must not reopen work. Decisions apply to a specific evidence revision; stale actions must prompt a refresh instead of handling newer unseen evidence.

Existing `reviewed_at` and `acted_at` are useful compatibility inputs, but an old `acted_at` is not proof that all follow-up is complete. Preserve legacy activity as history; do not bulk-convert it to Handled. Existing dismissed and superseded signals keep their historical state. Add explicit handled state and revision-aware decision history as needed rather than relabeling telemetry.

## 4. Minimum information

| Question | Information required |
| --- | --- |
| What happened? | Short factual event summary; distinguish the customer statement from an inferred commercial consequence. |
| Who is affected? | Named customer and relevant contact/deal when available; clear unresolved identity when not. |
| Why does it matter? | Specific opportunity, blocker, or risk. Conflicting evidence and missing customer context remain visible. |
| Who owns it? | Effective owner, assignment basis, and relevant handling history. |
| When? | Source occurrence time when known, separately labeled detection time, and an explicit deadline only when supported. |
| What supports it? | Source type, available author, date, original excerpt, and an accurately labeled source destination. |
| What next? | A concrete recommendation when supported, plus an available action; otherwise an honest request to review the evidence. |
| Has it been addressed? | Review and work state, actor/time, and relevant existing follow-up links. |

Use priority language for score buckets, not invented deadlines such as Act now. Confidence describes evidence support, not likelihood of a sale. Identity verification does not certify the whole claim. Commercial value must have a source, currency, and context; do not invent revenue from product interest.

Deduplicate source counts. Keep score factors, detector versions, identity methods, and raw diagnostic counts available in detail without making them the main explanation. If a source link is unavailable, explain that limitation. A link to a record tab must not be called an exact-source link.

## 5. Filtering must preserve meaning

Separate matching from assessment. Evidence-domain, direction, type, and search filters select matching signals and account/category contexts. Assessments must still use the complete authorized, current evidence for that context within the explicitly selected time window. Show which evidence matched; supporting or opposing evidence retained only for context must not silently join the selected action set.

For assessment, current evidence excludes superseded and dismissed signals, including those dismissed as incorrect or attributed to the wrong entity. Handled evidence can remain valid context but must carry its handled state and must not generate an instruction to repeat completed work. Historical views show prior evidence and decisions without presenting them as current recommendations.

Responsibility filtering narrows actionable work. Additional evidence may provide context only if the viewer is authorized to see it; it must not change the entry's assigned owner. Mark assessments incomplete when relevant source content is unavailable. A narrower explicit time window must be labeled as the assessment window.

Counts and pagination must come from the same server contract as the queue. Category counts apply all active filters except the category selector, so users can see other matching categories. The result count applies all filters. Count durable actionable signals, label account counts separately, and never mix signal totals with account/category totals. Aggregation must precede pagination. Unloaded pages are not zero results.

Changing any filter resets incompatible pagination. Clear returns to a visibly stated default. Owner and account pickers search the full permitted population rather than the first fetched page.

## 6. Use other Helpin modules only when needed

| Capability | Appropriate use | Boundary |
| --- | --- | --- |
| Support | Inspect or respond to the conversation that supplied the evidence. | Do not create a ticket to process a CRM signal. Support conversation ownership and resolution remain independent. |
| PM / tasks | A follow-up needs an assignee, due date, coordination, or progress tracking. Link relevant existing work before offering new work. | No task is required to review or handle a signal. Task creation is user-initiated unless separately authorized by an existing automation. Creating a task does not handle the signal automatically. |
| Docs | Open an existing pricing policy, runbook, or other reference relevant to the decision. | No generated document, required documentation step, or automatic knowledge search for every signal. |
| CRM | Inspect the relevant customer or deal; use existing permitted record actions when needed. | Do not force deal creation for every opportunity or replace existing CRM record workflows. |

Cross-module links do not imply shared lifecycle. Closing a task or support conversation must not automatically handle every related signal. Likewise, handling a signal must not close external work. Such automation requires an explicit, separately defined rule.

Unavailable modules or inaccessible records must not prevent signal-level decisions. Honor source permissions and avoid exposing restricted excerpts through the feed. No new cross-module automation engine or document workflow is part of this release.

## 7. Page boundaries and reliability

- Signal review and automation approval remain distinct. A signal recommendation is not an approved executable action. Existing Review approvals retain their confirmation, permission, and execution semantics.
- Keep access to Review, deal health, record search, and setup where useful. They need not occupy the main signal queue, and their absence must not block it. Do not remove their underlying capabilities as part of this change.
- Empty states distinguish no assigned work, no matching results, no qualifying evidence, and configuration restrictions. Query failures are errors with retry, not zero counts or onboarding states.
- Show refresh/freshness state and reconcile relevant changes, reconnects, assignment changes, and decisions. Do not claim live delivery without a supported refresh path.
- Permission changes must be enforced on reads and mutations. Read-only members should not see enabled mutation controls that inevitably fail.
- Direct category access, source inspection, and decisions must work at narrow widths and with keyboard navigation. Preserve focus and the user's queue context after opening and closing details.

## 8. Required implementation work and order

1. **Owner and query correctness:** unify the current query-builder owner expression with effective-owner resolution; support explicit/unassigned ownership; separate evidence matching from full-context composition; define consistent count and pagination responses.
2. **Decision state:** introduce handled state, actor/history, and evidence-revision checks while retaining existing feedback and historical data. Surface decisions consistently on workspace and entity signal views. Keep grouping independent of mutable owner/score and avoid group-wide destructive updates.
3. **Information and destinations:** expose named context, timestamps, supported deadlines, provenance, and relevant existing work. Reuse current source routes and action surfaces, with truthful fallback labels.
4. **Queue experience:** add responsibility/category/state navigation, scoped search, return-state preservation, and appropriate empty/error/permission states. Resolve visual composition in the subsequent design step using Helpin's design system.

Likely touchpoints: `SignalWorkspaceFeed.tsx`, `Insights.tsx`, `EntitySignals.tsx`, `crmSignalQueryBuilder.ts`, CRM query hooks/types/services, and backend signal models, query definitions, context hydration, scoring/composition, feedback, and routing. Database changes must be additive, preserve prior decisions, and use new versioned migrations. Do not mutate existing interpretation meaning or bypass commercial qualification and activation gates.

## 9. Acceptance scenarios

1. A CSM reaches their retention work directly even when preceding categories contain hundreds of entries. Display, Owner filtering, and category counts agree for a customer-success owner.
2. A manager finds explicitly unassigned signals. Multi-team membership produces no duplicate entries or inflated counts. A fallback-assigned item is labeled as such rather than counted as unassigned.
3. A salesperson understands an add-on purchase blocker, sees its original evidence, and opens the right conversation. They can review or handle it without creating a task or document.
4. Two unrelated requests for the same account/category remain separately actionable. Handling one does not handle the other or transfer its ownership.
5. A teammate sees who reviewed or handled current evidence. An existing task can be opened, but neither linking nor completing it silently changes the signal's work state.
6. A Momentum filter finds a matching signal while retaining authorized conflicting evidence in its assessment. The recommendation does not become falsely certain because negative evidence was filtered out.
7. New evidence returns a previously handled or dismissed, non-superseded signal to Open and Needs review; an unchanged replay or score recalculation does not. Superseded interpretations remain historical. A stale browser cannot handle unseen evidence, and prior decisions remain auditable.
8. Changing category, status, or owner keeps counts accurate across pagination. Back navigation restores the queue. Empty and failed requests have distinct outcomes.
9. Missing Support/PM/Docs access leaves a usable signal workflow and leaks no restricted content. Read-only members can inspect permitted evidence but cannot mutate state.
10. The same critical discovery and decision scenarios work on a narrow screen and by keyboard. A real deadline is distinguishable from priority and detection time.
11. In All categories, a retention signal with priority 20 precedes expansion signals with priorities 12 and 9, even if the two expansion signals share an account/category summary with a higher compound score. Those three signals count as three entries and consume three pagination positions; context-only evidence consumes none. Dismissed evidence cannot restore a rejected claim to an assessment.

Verification should target these outcomes with backend authorization/filter/state tests, representative UI integration tests, and populated/empty/error/narrow/keyboard visual checks. A happy-path screenshot alone does not verify this contract.

## Research references

- [Pocus Inbox](https://docs.pocus.com/docs/inbox): direct playbook access and explicit completion/disqualification.
- [Pocus inbox configuration](https://docs.pocus.com/docs/surface-to-reps): context-specific fields, actions, and destinations.
- [Common Room team segments](https://www.commonroom.io/docs/using-common-room/segments/team-segments/): ownership-scoped work.
- [Gainsight Cockpit](https://support.gainsight.com/gainsight_nxt/04Cockpit_and_Playbooks/00Cockpit_Horizon_Experience/User_Guides/Cockpit_List_View): assigned work, due dates, and progress.
- [Vitally indicators](https://docs.vitally.io/en/articles/9919032-creating-opportunity-risk-indicators): addressed work can coexist with a persistent customer condition.
- [Gong deal boards](https://help.gong.io/docs/understanding-deal-boards): commercial context alongside evidence and warnings.

These references inform the contract; they do not require Helpin to reproduce another product's interface or introduce its full workflow machinery.
