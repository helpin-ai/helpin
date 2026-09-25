# Support AI stuck detection and same-issue handoff

> **Historical / superseded execution design.** This describes the retired planner/answer pipeline. Its issue-stall and pre-model heuristic functions were disconnected after the Agent Runtime migration and are removed. It is not a guarantee of current runtime behavior.
> See [Support execution through Agent Runtime](../support-agent-runtime.md) for current ownership.

**Status:** Historical draft — execution design superseded
**Version:** v1.0  
**Date:** 2026-04-07  
**Owners:** Support, AI Platform, Frontend  
**Primary areas:** 
- `server/internal/service/support_ai.go`
- `server/internal/service/support_ai_escalation.go`
- `server/internal/model/support_inbox.go`
- `server/internal/service/support_inbox_settings.go`
- `frontend/src/components/settings/ChatGeneralTab.tsx`
- `frontend/src/components/support/MessageThread.tsx`
- `server/internal/service/support_ai_query_plan_test.go`
- `server/internal/service/support_ai_escalation_test.go`

---

## 1. Context

The current support AI pipeline already does several important things well:

- answers from configured knowledge sources
- asks clarifying questions when the latest customer turn is ambiguous
- escalates on hard rules
- escalates on low confidence
- detects repeated customer restatement through a similarity-based repetition loop heuristic

However, the current configurable setting `ai_max_followups` is still conversation-wide in practice. That means a long, healthy conversation across multiple topics can consume the entire AI budget, while a same-issue loop can still feel frustrating before the system decides to hand off.

The real product need is not "count AI replies globally." The real need is:

- detect when the AI is stuck on the same unresolved issue
- escalate when the conversation is not making progress
- avoid escalating productive multi-step support flows too early

This PRD defines an issue-aware stuck-detection layer for support AI. It keeps the existing support answer pipeline and repetition heuristic, but adds a more precise notion of "same issue," "progress," and "stalled attempts."

It also introduces an optional lightweight conversation analyzer step using Claude Haiku 4.5 for borderline cases where deterministic signals alone are not enough.

---

## 2. Problem Statement

Today the product has three gaps:

1. `ai_max_followups` is not aligned with the product intent.

The UI implies "stop if the AI keeps following up too much," but the implementation effectively counts answer-like AI turns across the conversation rather than attempts on the same unresolved issue.

2. "Same question" behavior is only partially covered.

The current repetition loop detection is good, but it is customer-repeat driven. It does not fully model cases where:

- the AI asks clarifying questions repeatedly
- the AI gives multiple weak answers on the same issue
- the customer provides new information but the AI still fails to advance the conversation

3. Productive multi-turn troubleshooting and setup flows can look like loops if we only count turns.

Examples:

- "Set up SSO with Okta"
- "Import contacts from HubSpot"
- "I got an error on step 3"

These are still the same issue, but they are not necessarily stuck. They are often normal, healthy support conversations.

---

## 3. Goals

1. Detect when support AI is stuck on the same unresolved issue.
2. Escalate same-issue loops earlier than the current generic conversation-wide cap.
3. Avoid premature handoff when the same issue is making real progress.
4. Keep latency and token cost low on the happy path.
5. Reuse the existing planner and answer-generation pipeline rather than introducing a second full support runtime.
6. Preserve the current repetition heuristic as a fallback safety net.
7. Improve UI wording so the setting matches actual system behavior.

---

## 4. Non-Goals

- Replacing the current support AI answer-generation architecture
- Building a persistent "case state machine" or new issue-tracking table in v1
- Introducing a mandatory extra LLM call on every support message
- Solving all customer satisfaction detection through LLM judgment alone
- Replacing human escalation rules for billing, refunds, security, or explicit human requests

---

## 5. Product Principle

The handoff decision should be based on **lack of progress**, not raw turn count.

This feature should answer:

- Is this the same underlying issue?
- Is the AI making progress on that issue?
- Has the AI already tried enough on this issue without progress?

The system should hand off when the answer is:

- same issue
- stalled
- repeated enough

It should not hand off just because:

- the issue took multiple turns
- the customer provided new information
- the AI moved the troubleshooting process forward

---

## 6. User Stories

1. As a support admin, I want the AI to hand off when it keeps circling on the same issue, so customers do not feel trapped in a bot loop.
2. As a support admin, I want productive multi-step troubleshooting to continue without premature handoff, so AI can still resolve legitimate support flows.
3. As a customer, I want a human to step in when the bot is clearly not helping, so I do not have to repeat myself several times.
4. As a support agent, I want the system to escalate with a clear reason, so I understand why the AI stopped and can pick up quickly.
5. As a product team, we want measurable loop-detection behavior, so we can tune it with real data rather than anecdotal complaints.

---

## 7. Proposed Solution

### 7.1 High-level approach

Add a new issue-aware stuck-detection layer to the existing support AI flow:

1. The current planner produces both:
   - retrieval-oriented query fields
   - a normalized issue identifier for the latest customer turn
2. The system reconstructs recent same-issue AI attempts from conversation history.
3. The system computes whether the issue is progressing or stalled.
4. If deterministic signals strongly indicate "stuck," hand off immediately.
5. If the situation is borderline, optionally run a lightweight Haiku 4.5 conversation analyzer using recent history plus structured signals.
6. Otherwise continue with the normal retrieval + answer pipeline.

### 7.2 Key concepts

#### A. `issue_key`

A short normalized identifier representing the underlying customer issue.

Examples:

- `sso_okta_setup`
- `pricing_and_plan_limits`
- `password_reset`
- `hubspot_import_error`

This is **not** the same as `standalone_query`.

`standalone_query` exists to improve retrieval. It may vary turn to turn. `issue_key` exists to group turns into the same underlying issue.

#### B. `reply_kind`

This already exists conceptually and should continue to distinguish:

- `greeting`
- `clarify`
- `answer`

We will extend how it is used.

#### C. `progress_state`

Each same-issue turn will be evaluated as one of:

- `progressing`
- `stalled`
- `regressing`

For v1, this does not need to be persisted as a separate table column. It can be inferred from recent history and stored in AI reply metadata when useful.

#### D. `stalled_attempt_count`

This is the main policy counter.

It increments only when same-issue AI behavior appears unhelpful or repetitive. It does not increment for productive progress.

---

## 8. Detailed Design

### 8.1 Planner contract changes

Extend `SupportQueryPlanContract` with new fields:

```json
{
  "decision": "answer",
  "issue_key": "password_reset",
  "issue_summary": "Customer needs help resetting their password",
  "progress_signal": "new_issue",
  "standalone_query": "how to reset password",
  "search_queries": ["reset password", "password reset help"],
  "clarifying_question": "",
  "reason": "resolved_from_context"
}
```

New fields:

- `issue_key`: short machine-friendly identifier, stable across paraphrases when possible
- `issue_summary`: short human-readable summary for debugging and future UI use
- `progress_signal`: one of:
  - `new_issue`
  - `same_issue_new_info`
  - `same_issue_repeat`
  - `same_issue_unclear`

Rules for the planner:

- `issue_key` should remain stable across follow-up turns on the same issue
- `issue_key` should change when the customer clearly moves to a new issue
- `standalone_query` may change as new detail is added
- `issue_key` and `standalone_query` are separate responsibilities

### 8.2 Metadata changes

Extend `AIMessageMetadata` with optional fields:

```json
{
  "ai_auto_reply": true,
  "ai_confidence": 0.82,
  "ai_reply_kind": "answer",
  "ai_issue_key": "password_reset",
  "ai_issue_summary": "Customer needs help resetting their password",
  "ai_progress_state": "stalled"
}
```

This allows cheap reconstruction of same-issue history from conversation messages without introducing a new table in v1.

### 8.3 New decision layer in the runtime

Current order is roughly:

1. hard max follow-ups
2. hard escalation rules
3. heuristic pre-LLM escalation
4. planner
5. retrieval
6. answer

New order should be:

1. hard escalation rules
2. lightweight repetition / frustration / low-confidence heuristics
3. planner returns `decision`, `issue_key`, `progress_signal`
4. deterministic same-issue stuck evaluation
5. optional Haiku 4.5 conversation analyzer for borderline cases
6. retrieval
7. answer
8. post-answer confidence and satisfaction checks

The current global `ai_max_followups` check should no longer run before planning. It should be replaced by issue-aware logic that depends on `issue_key`.

### 8.4 Deterministic same-issue stuck evaluation

Add a new function:

`detectStuckOnSameIssue(currentPlan, history, confidenceThreshold, settings) -> signal?`

Inputs:

- current planner result
- recent conversation history
- previous AI metadata
- previous grounded confidence values
- current settings

Signals that increment `stalled_attempt_count`:

1. `clarify` after a previous `clarify` on the same `issue_key`
2. low-confidence `answer` on the same `issue_key`
3. customer restates the same issue after an AI `answer`
4. customer dissatisfaction on the same issue:
   - "still not working"
   - "that didn't help"
   - "same issue"
   - "you already said that"
5. current planner says `same_issue_repeat`

Signals that reset or do not increment the stalled count:

1. planner says `new_issue`
2. planner says `same_issue_new_info`
3. customer provides new concrete details:
   - error code
   - vendor name
   - step number
   - screenshot / attachment
4. AI confidence improves materially on the same issue
5. the previous AI turn was a productive clarification and the customer answered it directly

### 8.5 Selective Haiku 4.5 conversation analyzer

#### Why include it

Deterministic rules will cover most loops, but there are gray areas:

- the issue is technically the same, but the conversation is still progressing
- the customer wording changes enough that simple similarity is noisy
- recent history suggests frustration or lack of progress, but not strongly enough for deterministic handoff

Claude Haiku 4.5 is a good fit here because:

- it is cheaper and faster than a larger model
- it is strong enough at short conversation classification
- the repo already uses `claude-haiku-4-5` in support-adjacent flows

#### Important constraint

Haiku 4.5 should **not** run on every support turn.

It should run only when:

- deterministic stuck score is in a middle band
- current issue has already consumed at least one stalled attempt
- hard escalation rules did not already decide
- planner did not already choose `handoff`

#### Analyzer input

- last 6 to 10 non-system conversation turns
- current planner output
- last two same-issue AI reply kinds
- last two same-issue grounded confidence scores
- whether the customer appears to be repeating themselves
- whether the customer provided new information

#### Analyzer output

```json
{
  "decision": "continue",
  "stuck_confidence": 0.72,
  "same_issue": true,
  "progressing": false,
  "reason": "customer_repeating_without_new_information"
}
```

Possible `decision` values:

- `continue`
- `handoff`

The analyzer should never generate customer-facing text. It is a classifier only.

### 8.6 Settings behavior

Keep the existing `ai_max_followups` field for v1 compatibility, but reinterpret its meaning.

New product meaning:

- "Maximum stalled AI attempts on the same issue before handoff"

This is intentionally different from:

- "maximum AI replies in the conversation"

The setting should apply to stalled attempts, not raw attempts.

Recommended default:

- `2`

Meaning:

- after two stalled same-issue AI attempts, the next stuck signal hands off

This default is safer than `1`, and more aligned with real troubleshooting flows.

### 8.7 UI copy changes

Current label:

- `Max Follow-ups`

Proposed label:

- `Handoff After AI Gets Stuck`

Helper text:

- `Counts repeated, low-progress AI attempts on the same issue. Productive troubleshooting steps do not count toward the limit.`

Optional advanced label if desired:

- `Max Stalled Attempts On Same Issue`

---

## 9. Functional Requirements

1. The planner must return an `issue_key` for all non-empty customer turns.
2. The runtime must evaluate same-issue history before generating another answer.
3. The runtime must distinguish same-issue progress from same-issue looping.
4. The runtime must preserve existing hard escalation behavior.
5. The system must continue to use the current repetition heuristic as a fallback.
6. The Haiku 4.5 analyzer must be optional and selectively invoked.
7. Escalation reasons must remain machine-readable for analytics.
8. AI-generated message metadata must store issue-aware fields for future debugging and tuning.

---

## 10. Non-Functional Requirements

1. No extra analyzer LLM call on the majority of support turns.
2. Same-issue detection must remain deterministic when analyzer is disabled.
3. The design must not require schema migration for v1.
4. Logging must expose why handoff occurred without leaking customer-sensitive content.
5. The system must degrade safely:
   - if planner extras fail, fall back to current behavior
   - if analyzer fails, continue with deterministic logic only

---

## 11. Architecture And Flow

### 11.1 Revised flow

```text
Customer message
  -> hard escalation rules
  -> current heuristic escalation checks
  -> planner (decision + issue_key + progress_signal)
  -> same-issue stuck evaluator
      -> if clearly stuck: handoff
      -> if clearly progressing: continue
      -> if borderline and enabled: Haiku 4.5 analyzer
           -> continue or handoff
  -> retrieval
  -> answer generation
  -> post-answer confidence / declining satisfaction checks
```

### 11.2 Suggested internal scoring model

Use a small deterministic score before invoking Haiku:

- repeated clarify on same issue: +1.0
- low-confidence answer on same issue: +1.0
- customer repeats same issue without new info: +1.0
- explicit dissatisfaction on same issue: +1.5
- same issue with new information: -1.0
- improving confidence trend: -0.5
- productive clarification answer from customer: -0.5

Policy:

- score <= 0: continue
- score >= threshold: handoff
- middle band: optional Haiku 4.5 analyzer

This keeps the analyzer off the hot path for obvious cases.

---

## 12. Implementation Plan

### Phase 1: Issue-aware deterministic handoff

Scope:

- extend planner contract with `issue_key`, `issue_summary`, `progress_signal`
- store issue-aware fields in `AIMessageMetadata`
- replace conversation-wide max-followups logic with same-issue stalled-attempt logic
- update UI copy only, keep storage field name
- add logs and metrics

Expected outcome:

- immediate behavior improvement
- no schema migration
- low rollout risk

### Phase 2: Selective Haiku 4.5 analyzer

Scope:

- add optional analyzer call behind config/flag
- pass recent conversation plus structured stuck signals
- use analyzer only in borderline cases
- log analyzer decision, latency, and disagreement with deterministic logic

Expected outcome:

- better handling of ambiguous support flows
- improved same-issue judgment without full-time latency cost

### Phase 3: Tuning and optional naming cleanup

Scope:

- decide whether to rename `ai_max_followups` in API/UI
- tune thresholds based on production data
- consider exposing advanced per-workspace tuning if needed

---

## 13. Metrics

### Primary success metrics

1. Reduction in customer turns before human handoff in true loop cases
2. Reduction in "AI repeated itself" complaints
3. Increase in human handoffs triggered for genuine stuck loops
4. No material decrease in AI self-resolution rate for healthy multi-step conversations

### Guardrail metrics

1. False-positive handoff rate
2. AI resolution rate by issue type
3. Analyzer invocation rate
4. Analyzer median and p95 latency
5. Additional token cost per support conversation

### Debug metrics

1. handoff reason distribution
2. same-issue stalled-attempt distribution
3. planner `progress_signal` distribution
4. deterministic vs analyzer disagreement rate

---

## 14. Testing Plan

### Unit tests

Add / update tests for:

- issue-key stability across paraphrases
- issue-key reset on clear topic change
- repeated clarify on same issue triggers stalled count
- low-confidence same-issue answers trigger stalled count
- same issue with new information does not trigger handoff
- productive multi-step troubleshooting continues
- customer repeat without new info escalates
- analyzer fallback skipped for obvious continue/handoff cases

### Service tests

Add end-to-end support AI service tests for:

- same conversation, multiple unrelated issues
- same issue with progressive detail
- same issue with no progress
- analyzer enabled vs disabled behavior

### Manual QA scenarios

1. Customer repeats same password-reset question three times.
2. Customer provides new troubleshooting detail after a clarification.
3. Customer moves from pricing to SSO setup in one conversation.
4. Customer says "still not working" after two weak AI answers.
5. AI asks one useful clarification, then resolves successfully.

---

## 15. Risks And Mitigations

### Risk 1: `issue_key` instability

If the planner emits unstable keys, same-issue counting becomes noisy.

Mitigation:

- separate `issue_key` from `standalone_query`
- keep similarity fallback
- log planner issue-key churn

### Risk 2: Premature handoff on productive same-issue flows

Mitigation:

- count stalled attempts, not raw attempts
- treat new information as progress
- start with a conservative default of `2`

### Risk 3: Analyzer adds too much latency or cost

Mitigation:

- only run Haiku 4.5 in borderline cases
- instrument invocation rate and p95 latency
- allow feature-flag disable

### Risk 4: Two sources of truth between deterministic logic and analyzer

Mitigation:

- deterministic logic remains primary
- analyzer only arbitrates gray-area cases
- log disagreements for tuning

---

## 16. Open Questions

1. Should the Haiku 4.5 analyzer be workspace-configurable, or only a backend feature flag initially?
2. Should `issue_key` be generated entirely by the planner, or should we post-normalize it server-side?
3. Should dissatisfaction phrases remain deterministic only, or also be fed into analyzer reasoning?
4. Do we want to expose advanced controls beyond the single stalled-attempt limit in v1?

---

## 17. Recommendation

Implement Phase 1 first and ship it behind the existing `ai_max_followups` setting with new UI copy.

That delivers most of the product value with low operational risk:

- same-issue awareness
- progress-vs-stuck distinction
- no migration
- no mandatory extra model call

Then add the selective Haiku 4.5 analyzer in Phase 2 once deterministic logging shows where the gray areas actually are.

This is the right balance between product quality and operational pragmatism. The system should not ask a second model for help on every support message, but it is reasonable to use Haiku 4.5 as a cheap arbiter when the deterministic signals cannot confidently tell whether the AI is making progress or just looping.
