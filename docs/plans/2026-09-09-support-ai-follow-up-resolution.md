# Contextual AI support follow-up and inactivity resolution

Status: implemented locally on `feat/support-ai-inactivity-follow-up`. Production rollout remains opt-in; no customer messages have been sent.

## Problem and current evidence

AI Handling contains conversations that remain pending after the AI answers and the customer stops replying. The desired behavior is to read the context, send one useful check-in, and resolve eligible conversations after a further period without a reply.

On develop, `server/internal/model/support_inbox.go` defines `AIAutoResolveTimeout` (24 hours by default, zero disables it). It is persisted and validated by `support_inbox_settings.go`, but a repository-wide reference search found no runtime consumer. The older `docs/PRD-ai-support-agent.md` describes `support_ai_resolution.go`, which does not exist in this checkout. This is a likely implementation gap, not a verified diagnosis of production.

`internal_command_support_reply.go` marks conversations AI-resolved when a reply has both `ResolvesConversation` and confirmation reply kind. This path writes AI state, resolution type and flow state directly. Audit timestamp, canonical status, events, sidebar projections and ownership consistency before reusing it.

## Product behavior

Proposed defaults, configurable per support installation:

1. After 24 hours without a customer reply to the last successfully published AI answer, evaluate the conversation.
2. Send one short contextual follow-up if eligible.
3. Wait another 48 hours after successful follow-up publication before closing for inactivity. If channel delivery fails, suspend closure and surface the failure.
4. On explicit customer confirmation, resolve immediately as confirmed.
5. On any customer reply, invalidate the pending closure and resume normal handling. A reply after closure reopens through the existing ownership/routing policy.

Example after troubleshooting: “Were you able to reconnect Gmail using those steps, or are you still seeing the authorization error? If we don’t hear back within two days, we’ll close this conversation. You can reply anytime to reopen it.”

Ask about the actual issue, in the customer's language. Do not infer success, invent steps, promise work or introduce new product claims. One follow-up per inactivity episode; also cap repeat follow-ups per conversation to avoid recurring nudges when the customer has already been unresponsive.

AI Handling continues to own these conversations, with a secondary indicator such as “Waiting for customer” and a visible next-action time. On closure they move to AI Resolved with reason “No reply after follow-up.” Confirmed and inactivity resolutions remain separately reportable; silence is not evidence of a successful answer.

## Eligibility decisions

| Conversation context | Action |
| --- | --- |
| AI gave a supported answer or actionable steps; no outstanding obligation | Ask whether it helped; allow inactivity closure |
| AI requested information only the customer can provide | Ask for that specific information once; allow closure as no response, never confirmed success |
| Latest customer message has no AI response | Recover the failed/stuck turn or hand off; never close for customer inactivity |
| AI promised investigation, a fix, a refund action or team follow-up | Route outstanding work to the team; no inactivity closure |
| Customer asked for a person, conversation was escalated or human takeover occurred | Cancel AI follow-up and closure |
| Customer reports the solution failed, or unresolved context is ambiguous | Continue investigation or hand off; no automatic closure |
| Active agent run, human assignment, snoozed, already closed, spam or disabled AI | Skip/cancel as appropriate to existing lifecycle policy |

The classifier returns a structured decision, reason, relevant message IDs and optional follow-up text. Backend checks enforce ownership, eligibility, timing and permissions independently of model output. History is input data, not instructions authorizing lifecycle actions. Do not scan every transcript with an LLM repeatedly: evaluate only due candidates and reuse a decision only while its source revision remains current.

## Implementation sequence

### 1. Diagnose and audit lifecycle parity

- Inspect production configuration and deployment version with read-only access when available. Count pending conversations by age, latest customer/AI message, ownership, failed/active runs and outstanding promises. Sample transcripts to confirm the actual failure modes.
- Trace all resolution/reopen entry points, required fields, timestamps, tags, unread/reply state, activity, realtime outbox and sidebar counters. Preserve existing human routing, explicit confirmation, feedback/CSAT and channel behavior.
- Check whether confirmed-resolution direct updates already leave inconsistent state. Consolidate resolution behind the canonical service transition where needed.

### 2. Persist a durable inactivity episode

- Add versioned SQL migration for episode state: workspace/conversation, source public message and revision, decision/reason, follow-up due time, sent message/time, closure due time, cancellation reason, processing lease and attempts.
- Enforce uniqueness for the episode and action to prevent duplicates across workers and retries. Index due work; batch and rate-limit by workspace.
- Start/reset from meaningful public conversation activity, not generic `updated_at`. Internal notes, reads, tags and scheduler writes must not restart the inactivity clock.
- Customer messages, ownership changes, resolution and policy disablement invalidate pending actions. Recheck the source revision and eligibility immediately before enqueueing any outbound message or committing closure.

### 3. Schedule evaluation and publish through existing support infrastructure

- Use the shared durable scheduled-event infrastructure where its contract fits; otherwise use a persisted due-work queue with leases and restart recovery. Do not depend on a browser being open or an in-memory timer alone.
- Run contextual assessment through the existing agent execution and AI usage preflight/metering path. A billing/provider failure leaves the conversation actionable, not silently resolved.
- Give scheduled follow-ups an explicit validated trigger; preserve normal support reply gates and channel routing. Reuse outbound delivery, customer language, notification preferences and channel restrictions. Do not send both a follow-up and duplicate fallback email.
- Use a transactional enqueue/outbox with stable action identity and channel idempotency where available. If delivery is ambiguous, reconcile before retrying; do not claim exactly-once external delivery without provider support.
- Start the closure window only after successful publication into the channel delivery path; block closure on known delivery failure. Read receipts are not required and must not be invented.
- Close atomically only if the episode remains current, AI still owns the conversation, no new customer message/active run exists, the follow-up succeeded and its deadline elapsed. Apply canonical lifecycle transitions, timestamps, reason and events together.

### 4. Settings and inbox visibility

- Add explicit controls for contextual follow-up, delay, inactivity closure delay and conversation cap. Show a sample of the message and timing.
- Preserve `ai_auto_resolve_timeout = 0` as disabled. Do not silently activate legacy default settings that previously had no runtime effect. Introduce an explicit policy version/enablement and migrate existing installations with automation disabled until rollout is enabled.
- Define the old timeout's compatibility behavior explicitly; suggested new policy uses separate follow-up and post-follow-up delays rather than changing the old field's meaning silently.
- Show waiting reason, next scheduled action and cancellation controls in the conversation. Manual takeover cancels all scheduled AI actions.
- AI Resolved distinguishes confirmed and no-response closures. Customer-visible closure should be a quiet system state change after the prior notice, avoiding another unsolicited outbound message.

### 5. Existing backlog and rollout

- Produce a dry-run report: candidate counts, exclusion reasons, sample draft follow-ups and expected send volume. No backdated deadlines or immediate mass closure.
- Limit initial backlog eligibility to conversations with activity in the last 30 days as a proposed default. Place older items in a separate review report rather than reviving months-old conversations automatically.
- Explicit enablement starts real follow-up timers from actual sends. Apply workspace rate limits and a daily send cap; rollout starts with a small cohort and expands after reviewing reply, complaint, delivery and reopen rates.
- Existing human-owned conversations remain under human policy. Disabling the feature cancels queued actions, including sends and closures awaiting retries.

## Verification and acceptance

- Fake-clock integration tests cover follow-up due time, post-send closure timing and no early resolution.
- Test customer reply/human takeover racing evaluation, send enqueue and closure; stale actions cannot commit after a newer activity revision.
- Test concurrent workers, duplicate scheduler delivery, crash after enqueue, provider timeouts, delivery failures, restart recovery and configuration disablement.
- Test unresponded customer messages, ongoing promises, ambiguous context, confirmation, failed AI runs, billing failure and tenant isolation.
- Verify canonical status/flow/AI state, timestamps, tags, realtime/sidebar counters and reopen routing on every closure path.
- Verify contextual output is grounded, concise, localized and respects follow-up caps. No internal notes or sensitive implementation details appear in customer messages.
- Verify assumed closures do not inflate confirmed-resolution rates, successful-answer signals or knowledge-quality feedback.
- Acceptance: eligible unanswered AI conversations receive at most the allowed follow-ups, move out of AI Handling after the configured wait, remain distinguishable from confirmed success, and reopen correctly. Ineligible conversations retain an actionable owner/reason.

## Operational measurement

Track pending age distribution, eligible/skipped counts by reason, overdue scheduled work, send failures, duplicate suppression, customer replies, confirmed resolutions, inactivity closures, reopen rate and human escalations. Alert on stalled scheduling and rising closure/reopen rates. Review a transcript sample during rollout.

## Implementation record

Implemented on `feat/support-ai-inactivity-follow-up`. No deployment or production enablement has been performed.

- A dedicated PostgreSQL queue records source-message identity, reserved Agent Runtime run ID, lease, assessment start, outcome, sent message and closure deadline. The existing supervised support sweep processes it; no new long-running Runtime chat is created.
- Runtime assessments use only `get_support_conversation`, `list_conversation_messages` and `finish_support_follow_up`. The new outcome command verifies workspace, run, source-message citations, policy, current ownership and active work before atomically publishing a public support message and its deadline. Existing support publishing events and durable message projection/outbox continue delivery.
- Defaults are opt-in, 24 hours before assessment, 48 hours after follow-up, two follow-ups per conversation, 25 reserved assessments per workspace/day, and a 30-day backlog window. The legacy zero timeout still disables automation. Existing widget/email conversations are supported; other channels are excluded.
- The AI Assistant settings include timing controls and a read-only candidate count/sample preview. The preview does not call AI or send drafts; contextual eligibility is determined by the Runtime assessment when enabled. Conversation Details shows outcomes/deadlines and permits mailbox-authorized cancellation.
- Database triggers invalidate pending work on public replies, relevant ownership/state/policy changes, and source/follow-up edits or removals. Internal notes and reads do not reset the timer. Disabling and immediately re-enabling does not revive cancelled episodes.
- Confirmed and inactivity resolution share status/flow/type/timestamp updates. Existing database projections maintain inbox state and durable realtime events. The visitor reopen path now clears resolution timestamps/type and restores open status. Assumed closures do not emit confirmed-success feedback.
- Handoff decisions preserve the configured/current mailbox and existing teammate-selection policy, without sending an unsolicited new handoff promise. Failed/expired assessments and unconfirmed/failed email delivery remain visible for teammate review and do not close automatically.
- An accepted email delivery later than publication extends the reply window. Widget publication is the delivery boundary; read receipts are not required. The existing channel delivery system remains responsible for external-provider retry semantics.

Verification includes unit/service tests, a fake Agent Runtime launch contract test, UI enablement/preview tests, and isolated PostgreSQL tests for idempotent migrations, concurrent callbacks, reply races, ownership/message/policy cancellation, daily caps, lease recovery and failed delivery. Live model quality and provider delivery still need observation in the opt-in rollout; no real customer conversations were used in verification.

Validation note: the configured database's migration validation reports pending pre-existing migrations and an unchanged historical checksum mismatch in `202608170003_meeting_recording_media`. No repair or migration application was performed against that database. The new migration is verified independently against a disposable PostgreSQL database.
