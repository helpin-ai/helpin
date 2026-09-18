# Two-stage support inactivity follow-ups

> Source review, 2026-09-17

Historical September 9 design and rollout checkpoint. Current defaults enable
follow-ups with a 24-hour first delay, 24-hour second delay, and one-hour close
delay; see [settings](../../server/internal/model/support_inbox.go).
The [sequence migration](../../server/internal/dbmigrate/sql/202609090003_support_follow_up_sequences.sql)
preserves existing single-reminder episodes and introduces version 2. The
[sequence service](../../server/internal/service/support_ai_follow_up_sequence.go)
gates progress on delivery and handles failures. The original opt-in/single-reminder
statements below are not current defaults. Saved installation settings and episode
versions still matter; this source review makes no claim about messages sent or
production enablement.

User approved: two reminders (24h after answer, another 24h later), then assumed closure after 1h; final reminder politely says closing shortly and reply anytime to reopen, no customer-visible duration. Positive response resolves confirmed; negative/unclear response resumes support or hands off. Human takeover cancels. Email sender is workspace name + Support. Settings section below escalation, no preview, enabled for existing/new workspaces, user can disable.

Implementation:
1. Add second-delay settings and durable sequence version, second-message evidence, stored closing notice, assessment attempt tracking. New sequences snapshot delays; preserve old waiting sequences. New migration enables workspaces and adjusts defaults without modifying old migrations; only disabling/ineligible policy cancels current sequences. Remove lifetime count as scheduler gate, keep source idempotency and bounded backlog.
2. Extend service state machine: first question, delivery-gated second closing notice, delivery-gated assumed close. Atomic message IDs prevent duplicates. Fix handoff open/needs-human state. Bounded retries for failed assessments, then human review. Tests for staged deadlines/cancellation/idempotency/delivery.
3. Fix email reply dispatch into existing AI handling with human ownership safeguards; use Workspace Support name for inactivity email batches. Verify widget reopen and confirmed/negative response paths.
4. Rework frontend three timing inputs below escalation, remove preview and max control, default enabled, adjust inbox status for second reminder.
5. Test Go service/repository/handler and frontend, real PostgreSQL triggers if local DB available, TypeScript/build, desktop/mobile preview, review. Commit task files only; no push without request.
