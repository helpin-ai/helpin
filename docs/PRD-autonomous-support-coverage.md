# PRD: Autonomous Support Coverage

**Status:** Initial draft
**Date:** 2026-04-15
**Owners:** Support, Docs, AI Platform
**Primary areas:** Support, Docs, Automation, CRM

---

## 1. Product Thesis

Most support AI products frame the problem as:

> What content is missing?

Helpin should frame the problem as:

> What is preventing support from becoming autonomous?

The product should not be a generic "gap finder" or knowledge base analytics tool. It should become an operational system for increasing autonomous resolution across support conversations, docs, customer context, AI actions, routing, and workflows.

The positioning should be:

> Increase autonomous resolution by identifying, ranking, and fixing the blockers preventing AI from resolving customer issues.

This is broader and more defensible than "find missing articles." A missing article is only one possible blocker. Other blockers include missing customer context, missing permissions, missing actions, unclear policy, weak routing, conflicting docs, and missing multi-step workflows.

Customer support is moving toward an AI-driven operating model where humans supervise, improve, and handle exceptions rather than answering most conversations directly. In that world, docs are not just customer-facing content. They become part of the AI runtime:

- grounding material
- policy source
- procedural memory
- evaluation material
- fallback evidence
- customer-facing explanation layer

The first release should focus on Docs Coverage because docs are the most immediate and fixable blocker. But the product and backend model should be designed as the first layer of a broader **Autonomous Resolution Coverage** system, not as a dead-end docs analytics product.

---

## 2. Existing Helpin Advantage

Helpin already owns several systems that competitors often treat separately:

- Support conversations, inboxes, messages, assignment, handoff, and team inboxes.
- AI-first support replies with RAG, confidence gating, hard escalation, and stuck detection.
- Issue-aware support handoff concepts such as issue keys, progress signals, and repeated issue detection.
- Docs and help center publishing, imports, public URLs, redirects, translations, search, and widget article fetching.
- Knowledge source configuration for support agents.
- CRM contact linkage from support conversations.
- Automation rules, agents, agent runs, activity, and tool catalog.
- Widget surfaces where customers search, open articles, and ask questions.

The missing product layer is a durable coverage intelligence system that connects these signals into one learning loop:

```text
customer issue
  -> AI attempt
  -> retrieval / confidence / answer / handoff
  -> human resolution
  -> gap classification
  -> ranked fix suggestion
  -> docs / context / action / workflow update
  -> measured autonomous resolution lift
```

---

## 3. Product Definition

Build **Autonomous Support Coverage** as the system that answers:

1. What can AI resolve today?
2. What should AI be able to resolve but cannot?
3. Why did AI fail?
4. What is the business impact of the gap?
5. What should be changed to increase autonomous resolution?
6. Did the fix actually improve resolution?

The first release should answer a narrower version:

1. What recurring customer questions are not well-covered by docs?
2. Which articles are missing, weak, outdated, or conflicting?
3. What draft or update should be created from real support evidence?
4. Did publishing the fix reduce future handoffs or self-service failures?

This should live primarily in the Support product area, because support conversations are where the pain, evidence, and measured outcomes live. Docs is one important fix path, but the problem is not limited to docs.

Recommended navigation over time:

- Support
  - Inbox
  - Coverage
  - Gaps
  - Suggestions
- Docs
  - Drafts generated from gaps
  - Article updates generated from gaps
- Automation
  - Flows and actions generated from gaps

---

## 4. Core Concepts

### 4.1 Support Event

A durable operational event representing something relevant to support autonomy.

Examples:

- Conversation was created.
- Customer asked a question.
- Human agent replied.
- Conversation status changed.
- Conversation was assigned.
- AI attempted an answer.
- AI retrieved articles or content sources.
- AI escalated due to low confidence.
- AI escalated due to hard rule.
- AI got stuck on the same issue.
- Customer asked for a human.
- Human agent replied after AI failed.
- Conversation was resolved.
- Customer searched the help center or widget.
- Search returned no results.
- Article was opened from the widget.
- Article received helpful or unhelpful feedback.

Support events are the measurement foundation. Without them, coverage becomes a vanity score.

This event layer should be broader than Coverage but narrower than generic product analytics. It should capture support/docs/widget/AI lifecycle events that can drive operational workflows, evidence, gaps, recurrence measurement, and future Support Analytics rollups.

### 4.2 Coverage Topic

A stable cluster of related customer issues.

Examples:

- `password_reset`
- `billing_refund`
- `sso_setup`
- `import_failed`
- `cancel_subscription`
- `invoice_download`

Coverage topics should start from existing AI issue-key and triage concepts, then improve through clustering and human feedback.

### 4.2.1 V1 Gap Deduplication Strategy

V1 should use a conservative, debuggable deduplication strategy:

1. Use `issue_key` as the primary grouping key when it exists.
2. Combine events into the same gap when they share:
   - workspace
   - gap type
   - issue key
   - related article, when applicable
3. For no-retrieval missing-article gaps, group repeated failures by workspace + issue key.
4. For weak/outdated article gaps, group by workspace + issue key + related article.
5. For no-result search gaps, group normalized search terms into an existing issue-key gap when a subsequent conversation has the same issue key.
6. Allow manual merge when users see duplicate gaps.

Do not make semantic clustering the primary v1 behavior. It can be added later for gaps without reliable issue keys. The first release should prefer stable, explainable grouping over clever clustering.

### 4.3 Coverage Gap

A durable blocker preventing autonomous resolution.

Gap types:

- **Knowledge gap:** No suitable article or content exists.
- **Context gap:** Relevant data exists but is not available to AI.
- **Action gap:** AI knows what should happen but cannot perform the action.
- **Workflow gap:** The issue requires a multi-step process that is not encoded.
- **Policy gap:** The correct business rule is unclear or requires approval.
- **Conflict gap:** Existing docs or human answers disagree.
- **Structure gap:** Content exists but is too vague, fragmented, outdated, or poorly structured for reliable retrieval and answering.

V1 UI should expose only docs-related gap types:

- **Missing article**
- **Weak article**
- **Outdated/conflicting article**
- **Needs review**

The broader gap taxonomy should exist in the product model, but the first release should not ask users to reason about context, action, workflow, or policy gaps unless they are surfaced as needs-review signals.

### 4.4 Fix Suggestion

A proposed change linked to a gap.

Examples:

- Create article.
- Update article.
- Merge or deprecate conflicting articles.
- Link an article or docs space to the support agent.
- Add CRM/contact data to AI context.
- Enable a safe AI action.
- Create routing rule.
- Create automation flow.
- Update support AI policy or prompt.
- Mark topic as intentionally human-only.

### 4.5 Coverage Policy

A workspace-owned declaration of what AI should and should not resolve autonomously.

Examples:

- Refunds over a configured amount require human approval.
- Enterprise security questions always escalate.
- Account deletion requires human confirmation.
- Billing questions for high-value customers route to a specific inbox.
- Legal, compliance, abuse, or data privacy issues are human-only.

This is strategically important because "human-only" should not become a dumping ground for unclassified failures. The product must distinguish:

- intentionally human-only by policy
- currently human-only because Helpin lacks docs, context, action, workflow, or confidence

### 4.6 Answer Coverage vs Resolution Coverage

The system must distinguish whether AI can answer from whether AI can resolve.

```text
Can answer? no
  -> likely docs, retrieval, structure, or knowledge source problem

Can answer? yes, can resolve? no
  -> likely action, context, workflow, permission, or policy problem
```

This distinction matters because many future AI support failures will not be content failures.

Example:

- Customer asks to cancel a subscription.
- AI knows the cancellation policy from docs.
- AI cannot perform the cancellation.
- This is not a missing-docs gap. It is an action or approval gap.

V1 should mostly act on answer-coverage failures, but the event model should capture both dimensions so later releases do not require a rewrite.

### 4.7 Autonomy Readiness Profile

Long term, each support topic should have an autonomy readiness profile.

Example:

```text
Topic: SSO setup

Answer coverage: good
Retrieval coverage: good
Context coverage: not needed
Action coverage: missing
Policy coverage: approved
Evaluation coverage: weak
Current autonomy: partial
Main blocker: workflow/action
```

V1 does not need to expose this profile directly. It should influence how the data model is shaped.

---

## 5. Product Principles

### 5.1 Coverage Must Be Explainable

The coverage score should never be a black-box number. Every score should be traceable to conversations, AI attempts, docs, handoffs, human replies, and fix suggestions.

### 5.2 Gaps Must Be Operational

A gap is only useful if someone can act on it. Each gap should have evidence, impact, owner, status, and recommended next step.

### 5.3 Docs Are A Fix Path, Not The Whole Product

Missing content matters, but many failures are caused by missing context, missing permissions, missing actions, missing workflow, or unclear policy.

The backend should use generic coverage primitives. The v1 UI should be docs-focused.

Do:

- `support_events`
- `support_coverage_gaps`
- `support_gap_evidence`
- `support_gap_suggestions`

Avoid:

- `docs_gaps` as the core abstraction
- making every unresolved support failure look like a missing article

### 5.4 Prioritize Measured Lift Over Flashy Visuals

Simulation, heatmaps, and executive dashboards are valuable later. The first product loop should prove that Helpin can detect a real blocker, recommend a fix, and measure whether autonomous resolution improved.

### 5.5 Start With Human-Reviewable Suggestions

Early versions should recommend and draft fixes, not silently mutate support policy, workflows, or customer accounts. Autonomy should increase as confidence, auditability, and permissions mature.

### 5.6 Classify Conservatively First

The first classifier should prefer high-signal evidence and "unknown" over overconfident labels. Misclassified gaps create wrong fixes and quickly erode trust.

Initial classification should use deterministic signals first:

- no retrieval: likely knowledge gap
- weak retrieval with related articles: likely structure gap
- conflicting retrieved articles: likely conflict gap
- low confidence after good retrieval: likely policy, context, or action gap
- human override after AI answer: likely wrong-answer, policy, context, or complexity signal

LLM classification should improve these labels, but it should not be the only source of truth.

### 5.7 Human Agents Are A First-Class Signal

When a human resolves something AI could not, they are the best source of why the AI failed. The product should collect lightweight feedback at the moment of resolution instead of relying only on offline mining.

However, the product must not depend on human feedback as the primary signal. In an AI-driven support future, fewer conversations will reach humans. Scalable outcome signals matter more:

- AI handoff reason
- retrieval quality
- customer repeats the question
- customer asks for human
- article opened before conversation
- article marked unhelpful
- repeat contact on the same issue
- conversation reopened
- no-result search
- AI answer followed by no resolution

### 5.8 Self-Service Is Part Of The Same Funnel

The support journey often starts before a conversation:

```text
search
  -> article view
  -> conversation start
  -> AI attempt
  -> human resolution
```

A customer who searched, opened an article, and still opened a conversation is a stronger gap signal than a raw AI failure. Coverage should join widget/help center behavior to the subsequent support conversation when session identity allows it.

---

## 6. Phased Roadmap

The roadmap is phased by capability maturity, not necessarily by external release. For the first customer-facing release, Phases 0, 1, and 2 should ship together as the **Docs Coverage Loop**:

```text
support events
  -> rule-based docs gap detection
  -> gap inbox
  -> article draft/update suggestion
  -> publish
  -> recurrence measurement
  -> weekly digest
```

Phase 0.5 can ship alongside or shortly after the first release as a cold-start path for workspaces without enough support AI volume.

### Phase 0: Measurement Foundation

**Goal:** Make AI support outcomes observable.

Build a shared lightweight support event ledger that records support, docs, widget, and AI outcomes.

This ledger is the first operational event foundation for Support Coverage and future Support Analytics. It should not be a Coverage-owned table and it should not duplicate the existing product analytics pipeline.

Capture:

- Customer message and normalized issue key.
- AI query plan.
- Retrieved article/content IDs and PublicIDs.
- Retrieval quality.
- AI confidence.
- AI decision: answer, clarify, escalate, block.
- Handoff reason.
- Stuck/repeated issue signals.
- Human reply after AI failure.
- Conversation close/resolution.
- Widget article opens.
- Help center/widget search no-results.
- Article helpful/unhelpful feedback.
- Widget session ID and visitor/session identity where available, so self-service attempts can be linked to later conversations.
- Whether AI appeared able to answer.
- Whether AI appeared able to resolve.
- Failure mode: no answer, weak answer, no action, missing context, policy blocked, customer requested human, unknown.
- Action unavailable and context unavailable flags where detectable.

Important implementation direction:

- Store v1 operational support events in Postgres/Neon as `support_events`.
- Use an append-only event table rather than bloating the hot conversation record.
- Store canonical internal IDs and PublicIDs, not slugs.
- Treat slugs as display-only.
- Keep raw customer content access-controlled and avoid leaking sensitive data into analytics surfaces.
- Start with a small event taxonomy instead of trying to capture every possible interaction.
- Do not power gap inbox page loads by scanning raw events. Gap pages should read from gap, topic, evidence, suggestion, and snapshot tables.
- Compute aggregate metrics periodically into `support_coverage_snapshots`.
- Plan for retention: keep recent raw events hot, then archive or compact older events once they have been reflected in gaps, evidence, and snapshots.
- Keep the existing SDK/events-pipeline/Kafka/ClickHouse path for high-volume product and visitor analytics. Do not make ClickHouse the source of truth for Coverage v1.
- Do not use Redis as the event source of truth. Redis remains appropriate for ephemeral locks, presence, WebSocket relay, and delayed outboxes.
- A later Support Analytics product may export or replicate `support_events` into ClickHouse, but Coverage v1 should not depend on that pipeline.

Initial event types should include:

- `conversation_created`
- `customer_message_created`
- `human_reply_sent`
- `conversation_status_changed`
- `conversation_assigned`
- `ai_attempt_started`
- `ai_retrieval_completed`
- `ai_answer_sent`
- `ai_clarification_sent`
- `ai_handoff_triggered`
- `ai_blocked_by_policy`
- `human_reply_after_ai`
- `conversation_resolved`
- `widget_search_performed`
- `widget_article_opened`
- `article_feedback_submitted`

Exit criteria:

- For any escalated conversation, the product can explain what the customer asked, what AI tried, what it retrieved, why it failed, and what the human did next.

---

### Phase 0.5: Proactive Coverage Audit

**Goal:** Provide value before a workspace has enough support AI volume.

The product cannot depend entirely on large conversation volume. New customers, low-volume workspaces, and teams just enabling AI need an initial view of coverage quality before enough failures accumulate.

Build a bootstrap audit that analyzes existing docs and configuration.

Audit:

- Docs structure and readability.
- Articles with thin, outdated, duplicate, or conflicting content.
- Articles not linked to the support agent's knowledge sources.
- Common support topic taxonomy coverage.
- Help center search quality for seeded/common queries.
- Missing policy declarations for sensitive support categories.
- Basic action availability: can Helpin route, assign, create tasks, request approval, or hand off for common support patterns?

Output:

- Initial coverage checklist.
- Likely structure gaps.
- Likely conflict gaps.
- Suggested docs cleanup.
- Suggested knowledge source links.
- Suggested coverage policies to declare.

This should be clearly labeled as a proactive audit, not as measured AI failure data.

Exit criteria:

- A new workspace can see useful coverage recommendations on day one, before meaningful conversation volume exists.

---

### Phase 1: Gap Inbox MVP

**Goal:** Give support teams a useful first surface for AI failure review.

Build a **Support > Coverage > Gaps** view.

Each gap should show:

- Title.
- Gap type.
- Current status.
- Affected conversation count.
- Recent trend.
- Example customer questions.
- AI failure reason.
- Relevant retrieved docs/articles.
- Human answer examples.
- Suggested owner/team/mailbox.
- Last seen.

Initial detection sources:

- Low-confidence AI handoffs.
- No retrieval or weak retrieval.
- Repeated same-issue stuck detection.
- Explicit customer request for human.
- Negative article feedback.
- Search no-results.
- Human reply after AI failed.

UI should start simple:

- Ranked table.
- Filters by type, status, mailbox, topic, time window.
- Detail drawer with evidence and suggested next steps.
- Similar conversations link that opens a filtered inbox/conversation view.
- Reclassify, merge, dismiss, and mark as intentionally human-only actions.

Add a lightweight agent feedback prompt when a human resolves a conversation that had an AI handoff:

> AI couldn't answer this. Docs issue?

Options:

- Yes.
- No.

If the agent chooses Yes, create or strengthen a docs-related gap linked to the conversation. Do not ask the agent to classify the failure in v1. The system can infer the likely gap type from retrieval, article, search, and human-answer evidence.

This feedback should attach to the gap as high-quality labeled evidence without adding form work to busy support shifts.

Add a weekly digest email:

- Top 5 AI blockers this week.
- New high-volume gaps.
- Gaps with rising trend.
- Fixes shipped and measured impact when available.

V1 delivery:

- Send workspace-wide.
- Send weekly on Monday.
- Recipients: workspace admins and users with support administration permission.
- Include direct links to the top gaps and draft/update actions.
- Add unsubscribe/frequency settings after the first release if needed.

Do not build the visual coverage map yet.

Exit criteria:

- A support lead can review the top reasons AI failed last week and assign fixes.

---

### Phase 2: Knowledge Fix Loop

**Goal:** Turn support evidence into docs improvements.

For knowledge, conflict, and structure gaps, generate docs suggestions.

Build:

- Article draft generation from clustered customer questions and human replies.
- Article update suggestions when a related article already exists.
- Diff view for article update suggestions, with the evidence that justifies each change.
- Conflict detection when multiple docs or human replies disagree.
- Suggested article placement in existing docs spaces/collections.
- Direct link from gap to docs draft/editor.
- Gap status transitions when a draft is created, reviewed, published, or rejected.
- Post-publish monitoring to see whether recurrence drops.

The generated artifact should be a real docs draft or article update, not just text in a modal.

The UX should reuse the same pattern as import flows: preview, confirm, execute. Users should understand what will be created or changed before anything is published.

Exit criteria:

- A user can go from gap to draft article/update to publish to measured impact.

---

### Phase 3: Impact Scoring And Prioritization

**Goal:** Make coverage an operational prioritization tool and begin surfacing known-gap context during active support.

Score each gap using:

- Conversation volume.
- AI failure rate.
- Human time spent.
- Recurrence.
- SLA risk.
- Customer tier or CRM value when available.
- Confidence that the gap is real.
- Estimated autonomous resolution lift.
- Fix complexity.

Scoring should start as a simple, explainable weighted formula, not a black-box model.

Example:

```text
impact_score =
  volume
  * failure_rate
  * average_human_minutes
  * recurrence_factor
  * customer_value_factor
  * confidence_factor
```

The UI should show the inputs behind the score, and later allow admins to tune weights for their operation.

Produce ranked views:

- Top blockers to autonomous resolution.
- Highest revenue-risk gaps.
- Fastest fixes.
- Gaps with rising volume.
- Gaps by mailbox/team.

Introduce the first executive summary:

- Current autonomous resolution rate.
- Addressable autonomous resolution opportunity.
- Top blockers.
- Estimated lift from fixing top gaps.

Exit criteria:

- A manager can decide what to fix this week without manually reading dozens of conversations.
- An agent or supervisor can see when an active conversation matches a known gap and review prior resolutions.

#### Real-time coverage assist

Once known gaps have enough evidence, coverage should become useful during support, not only after weekly review.

When a conversation matches a known gap, show the human agent or supervisor:

- This is a known gap.
- Why AI usually fails here.
- Similar conversations.
- Prior human resolutions.
- Current recommended workaround.
- Draft or article update in progress, if any.

This is not part of the first Docs Coverage Loop release, but it should be the first expansion after the loop is working. It turns coverage from reporting into operational assist.

---

### Phase 4: Context And Action Gaps

**Goal:** Expand beyond docs, where competitors are weakest.

#### Context gaps

Detect when AI needed data that exists somewhere in Helpin but was not available in the support AI context.

Examples:

- Customer plan.
- CRM lifecycle stage.
- Previous support conversations.
- Account owner.
- Company tier.
- Recent purchases or subscription state, when integrations exist.

Suggested fixes:

- Add CRM contact fields to AI context.
- Include recent support history.
- Link relevant internal docs.
- Add missing structured field.
- Connect external data source.

#### Action gaps

Detect when AI knows what should happen but cannot perform the action.

Start with safe native Helpin actions:

- Move conversation.
- Assign mailbox.
- Create internal task.
- Start agent run.
- Request human approval.
- Update conversation status.
- Add internal note.

Defer high-risk actions such as refunds, account deletion, or billing mutation until permissions, approval, and audit controls are mature.

#### Coverage policies

Add an explicit policy surface for declaring what should stay human-reviewed.

Examples:

- Refunds over a configured amount require human approval.
- Security questionnaires route to a human.
- Enterprise contract questions route to customer success.
- Legal, compliance, privacy, or abuse topics always escalate.

Coverage policy should feed classification and scoring. A conversation that escalates because it matches declared policy should count as intentionally human-only, not as an unresolved product gap.

Exit criteria:

- The gap system can distinguish "we need a doc" from "AI needs context" and "AI needs permission to act."
- The system can distinguish intentionally human-only work from work that is human-only because Helpin is missing coverage.

---

### Phase 5: Workflow Gap Builder

**Goal:** Convert repeated multi-step support issues into workflows.

Detect issues that are not answerable by one article or one action.

Examples:

- Troubleshoot integration failure.
- Qualify billing/refund request.
- Route enterprise security question.
- Collect reproduction steps for a bug.
- Guide user through import setup.

From a workflow gap, generate:

- Internal playbook.
- Customer-facing article.
- Checklist.
- Routing rule.
- Automation flow.
- Optional agent run configuration.

Use the existing automation system rather than creating a separate workflow engine.

Exit criteria:

- A repeated issue can become a documented, routed, partially automated support workflow.

---

### Phase 6: Replay And Simulation

**Goal:** Estimate lift before publishing or enabling a fix.

Build this only after event quality and gap labels are reliable.

Replay historical conversations against proposed fixes:

- Would retrieval find the new or updated article?
- Would the answer be grounded?
- Would confidence cross the configured threshold?
- Would a context addition have changed the outcome?
- Would an action have resolved the issue?
- Which conversations would still require a human?

Output:

- Estimated autonomous resolution lift.
- Confidence/risk rating.
- Sample conversations helped.
- Sample conversations still not helped.
- Hallucination or policy risk notes.

Do not present this as guaranteed lift. It should be framed as an estimate based on historical replay.

Exit criteria:

- Before publishing a major fix, the user sees expected lift and representative evidence.

---

### Phase 7: Coverage Map And Executive View

**Goal:** Package the system for leadership and long-term operations.

Build:

- Autonomous resolution trend.
- Addressable automation opportunity.
- Coverage by topic.
- Coverage by mailbox/team.
- Coverage by customer segment.
- Gap type distribution.
- Fixed gaps and measured impact.
- Human-only topics by policy.

Executive view should answer:

- What percentage of support is autonomous today?
- What percentage is addressable with known fixes?
- What is blocked by missing docs?
- What is blocked by missing context?
- What is blocked by missing actions?
- What is intentionally human-only?
- What should we fix next for the highest lift?

Example summary:

```text
Currently autonomous: 38%
Addressable with known fixes: +14%
Blocked by missing actions: 9%
Blocked by missing context: 6%
Human-only by policy: 11%
```

Exit criteria:

- This becomes the weekly operating dashboard for support automation.

---

### Later: Cross-Workspace Coverage Intelligence

With strict tenant isolation and anonymization, Helpin can eventually learn which topics commonly appear across similar companies or industries.

Potential product value:

- Pre-populate proactive audit suggestions for new customers.
- Recommend common support articles by industry.
- Identify common action/workflow gaps for SaaS, ecommerce, agencies, marketplaces, or developer tools.
- Improve starter coverage templates.

This should not be part of v1. It requires careful privacy, aggregation, and opt-in design.

---

### Later: Code-Aware Docs Accuracy

Support-driven Coverage answers:

> Are the docs complete enough to resolve real customer issues?

Code-aware docs accuracy answers:

> Are the docs still true according to the current product, APIs, and codebase?

This is strategically important, but it should not be part of the first Docs Coverage release. It uses a different signal source and requires repo/code access, change detection, and a review workflow. It should be introduced after the support-driven Coverage loop is working.

Recommended product shape:

- Product surface: Docs and Support Coverage.
- Execution layer: system agents, automation rules, GitHub/repo tools, and durable agent runs.
- Output: docs accuracy findings, evidence, and suggested article updates.

The user should not experience this as "create an agent." The user-facing product should feel like:

> Helpin found docs that are stale against your codebase.

Possible names:

- Docs Accuracy Audit.
- Code-Aware Docs Review.
- Docs Drift Detection.
- Source-of-Truth Check.

What it should detect:

- docs mention a setting, flag, route, or API parameter that no longer exists
- docs omit a new required field or changed workflow
- docs describe old behavior after a PR/release changed the product
- docs link to removed routes or outdated endpoints
- docs miss coverage for newly shipped features
- docs contain code examples that no longer compile or match the current API

Recommended architecture:

```text
GitHub event / release / schedule / manual audit
  -> automation rule starts a system agent run
  -> Docs Accuracy Agent inspects repo, API schemas, routes, changelogs, and docs
  -> durable finding is stored
  -> finding may create or update a Coverage gap with source_signal=code_audit
  -> user reviews suggested docs update
  -> approved update creates or modifies a docs draft
```

Use normal backend checks for deterministic cases:

- broken links
- removed routes
- OpenAPI parameter mismatch
- docs last updated before release
- missing docs for known public endpoints

Use system agents for semantic/code-aware checks:

- setup guide still matches implementation
- docs accurately describe product behavior
- PR/release implies docs updates
- migration/config/feature-flag changes invalidate an article
- suggested docs patch with repo evidence

The durable product record should not be only an `agent_run`. Agent runs are execution history. Product state should be stored as docs accuracy findings and/or Coverage gaps so users can triage, assign, ignore, fix, and measure them.

This later phase should reuse Coverage concepts where appropriate:

- `source_signal = code_audit`
- `gap_category = structure`, `knowledge`, or `conflict`
- `v1_gap_type = outdated_or_conflicting_article` or `needs_review`
- suggestions can use the same article draft/update workflow

Non-goals for v1:

- Do not connect GitHub/codebase analysis to Docs Coverage v1.
- Do not compare docs to code in the first release.
- Do not expose a generic "build an agent to audit docs" setup flow as the main UX.

---

## 7. First Six-Week Bet

The first focused bet should be the full **Docs Coverage Loop**, not isolated infrastructure or a standalone dashboard.

Ship these together:

1. Shared lightweight support event ledger.
2. Rule-based docs gap detection.
3. Gap inbox.
4. Article draft generation.
5. Existing article update suggestions.
6. Self-service weak-article signals.
7. Agent "Docs issue?" Yes/No feedback.
8. Post-publish recurrence tracking.
9. Weekly digest.

This proves the core loop:

```text
support/self-service signal
  -> rule-based docs gap
  -> gap inbox
  -> draft or article update
  -> publish
  -> recurrence measurement
  -> weekly digest
```

The first release should be able to truthfully say:

> Helpin found recurring support questions missing from your docs, drafted fixes from real support evidence, and showed whether those fixes reduced future handoffs.

---

## 8. Explicit Non-Goals For V1

Do not build these first:

- Full visual coverage map.
- Full simulation engine.
- Broad customer lifecycle coverage.
- Complex action marketplace.
- Fully generated workflows.
- Autonomous refunds or account mutations.
- Executive dashboard without evidence drilldown.
- Cross-workspace intelligence.
- LLM-based gap classification as the primary source of truth.
- A docs-only backend model that cannot later support context, action, workflow, policy, or evaluation gaps.

These are valuable later, but they depend on trustworthy support events, gap labels, and fix outcomes.

---

## 9. Success Metrics

Primary:

- Autonomous resolution rate.
- Addressable autonomous resolution opportunity.
- AI handoff rate by reason.
- Percentage of AI failures classified into a known gap type.
- Gap recurrence after fix.
- Deflection lift after docs/action/context fix.

Operational:

- Time to identify gap.
- Time from gap creation to owner assignment.
- Time from gap creation to published fix.
- Fix suggestion acceptance rate.
- Article draft acceptance rate.
- Percentage of gaps with evidence from multiple conversations.

Business:

- Human minutes saved.
- SLA risk reduced.
- High-value customer gaps resolved.
- Support volume deflected.
- Cost per resolved conversation.

Quality:

- False-positive gap rate.
- Incorrect gap type rate.
- Human reclassification rate.
- Reopened conversations after AI resolution.
- Negative feedback after AI answer.
- Hallucination or policy violation rate.
- Percentage of self-service journeys linked to support conversations.
- Percentage of gaps where `can_answer` vs `can_resolve` is known.

---

## 10. Risks And Mitigations

### Risk: Coverage Score Becomes A Vanity Metric

Mitigation:

- Every score must drill down to topics, gaps, events, and evidence.
- Avoid one global number without explanation.

### Risk: Classifier Produces Noisy Gaps

Mitigation:

- Start with high-signal sources like AI handoff, no retrieval, stuck detection, and human reply after failure.
- Prefer unknown/needs-review over overconfident classification.
- Require evidence thresholds before surfacing as recurring gaps.
- Allow users to merge, dismiss, and reclassify gaps.
- Collect explicit agent feedback at resolution time.

### Risk: Cold Start Limits Value

Mitigation:

- Add proactive coverage audits that analyze docs, search, knowledge source configuration, policy declarations, and safe native action availability.
- Label proactive recommendations separately from measured support failures.
- Use coverage templates later for common industries or company types.

### Risk: Suggestions Create More Work Than They Save

Mitigation:

- Rank by impact.
- Deduplicate similar gaps.
- Provide concrete next actions.
- Track whether suggestions are accepted or ignored.

### Risk: Docs Coverage Becomes A Dead-End Product

Mitigation:

- Keep v1 UI docs-focused, but use generic support coverage primitives in the backend.
- Capture answer-vs-resolution dimensions from day one.
- Preserve future categories for context, action, workflow, policy, and evaluation gaps.
- Position the product around autonomous resolution improvement, not missing articles.

### Risk: Action Gaps Become Too Broad

Mitigation:

- Start with safe native Helpin actions.
- Require approval for sensitive actions.
- Add audit trail for every suggested or enabled action.

### Risk: Simulation Creates False Precision

Mitigation:

- Defer simulation until event quality is strong.
- Present replay output as estimated lift with sample evidence.

### Risk: Human-Only Becomes A Dumping Ground

Mitigation:

- Add explicit coverage policies.
- Separate declared human-only topics from unresolved gaps.
- Require a reason when marking a gap as intentionally human-only.

---

## 11. Data Model Sketch

Possible backend entities:

- `support_events`
- `support_coverage_topics`
- `support_coverage_gaps`
- `support_gap_evidence`
- `support_gap_suggestions`
- `support_coverage_snapshots`
- `support_coverage_policies`
- `support_coverage_topic_merges`
- `support_gap_simulations` later

These names are preliminary. The important design point is that coverage should be durable and queryable, not reconstructed ad hoc from conversations every time.

### 11.1 Entity Relationships

V1 should align engineering on these relationships before writing migrations:

```text
support_coverage_topic 1:N support_coverage_gaps

support_coverage_gap 1:N support_gap_evidence
support_coverage_gap 1:N support_gap_suggestions
support_coverage_gap N:N docs_helpcenter_articles / docs_documents

support_gap_suggestion 0:1 docs_document
support_gap_suggestion 0:1 docs_helpcenter_article

support_gap_evidence N:1 support_conversation / support_message / search event / article feedback

support_event N:1 support_conversation, when conversation-backed
support_event N:1 docs article/document, when article-backed
support_event 0:1 support_coverage_gap, once attached as evidence

support_coverage_gap 1:N support_coverage_snapshots
```

Relationship intent:

- A **topic** groups related coverage gaps over time.
- A **gap** is the durable work item users triage.
- **Evidence** links a gap to the concrete signals that justify it: conversations, messages, searches, article feedback, AI handoffs, and human replies.
- **Suggestions** are proposed fixes for a gap: create article, update article, merge article, link source, and later add context/action/policy/workflow.
- A suggestion may create or update a real docs artifact. Once that happens, the suggestion should store the resulting document/article ID.
- A gap can be related to multiple existing articles, and one article can be related to multiple gaps.
- A published fix should remain linked to the originating gap through the suggestion and resulting docs artifact so recurrence can be measured after publish.
- Raw support events should feed evidence, gaps, and snapshots, but user-facing pages should not scan raw events directly.

### 11.2 Event Storage Strategy

Use a two-lane architecture:

```text
Operational support intelligence:
support/docs/widget/AI backend
  -> support_events in Postgres
  -> support_coverage_gaps / evidence / suggestions / snapshots
  -> Coverage UI and weekly digest

Product and visitor analytics:
SDK/widget/browser/server analytics
  -> existing events pipeline
  -> Kafka/sessionization
  -> ClickHouse
  -> future analytics dashboards
```

Rationale:

- Operational support events create Coverage product state and need reliable joins to conversations, messages, docs, articles, workspaces, and permissions.
- The existing events pipeline is already the right path for high-volume visitor/product analytics, autocapture, pageviews, and long-window analytical scans.
- Redis is not a durable source of truth for coverage or analytics events.
- NATS/Temporal are useful execution primitives, but v1 does not need a new queue before writing operational events.
- If Support Analytics later needs ClickHouse-scale reporting, mirror or export `support_events` to the existing analytics pipeline instead of changing Coverage's source of truth.

Important fields to preserve early:

- `gap_category`: knowledge, structure, conflict, context, action, workflow, policy, evaluation, unknown.
- `v1_gap_type`: missing_article, weak_article, outdated_or_conflicting_article, needs_review.
- `can_answer`: yes, no, unknown.
- `can_resolve`: yes, no, unknown.
- `failure_mode`: no_retrieval, weak_retrieval, low_confidence, stuck, customer_requested_human, action_unavailable, context_unavailable, policy_blocked, unknown.
- `source_signal`: ai_handoff, self_service_search, article_feedback, agent_feedback, human_reply, recurrence.
- `fix_type`: create_article, update_article, merge_article, link_knowledge_source, add_context, enable_action, define_policy, create_workflow, ignore.

---

## 12. Product Recommendation

Build **Docs Coverage Loop v1** as the first customer-facing release of a broader **Autonomous Resolution Coverage** system.

The first marketable version should not promise a full autonomous support engine. It should promise:

> Turn failed support and self-service signals into docs fixes that reduce future handoffs.

That is the sharp wedge. It creates immediate value by keeping docs updated while preserving the architecture needed for the future where support is AI-driven and humans supervise exceptions.

Long term, the product should expand from Docs Coverage into:

1. **Docs Coverage:** Are answers documented and retrievable?
2. **Context Coverage:** Does AI have the customer/account data needed to personalize the answer?
3. **Action Coverage:** Can AI perform the resolution step?
4. **Policy Coverage:** Does AI know what it is allowed to do and when to escalate?
5. **Workflow Coverage:** Can AI handle multi-step support processes?
6. **Evaluation Coverage:** Do we know it works safely before and after changes?
7. **Autonomy Dashboard:** What percent of support is autonomous, addressable, blocked, or intentionally human-only?
