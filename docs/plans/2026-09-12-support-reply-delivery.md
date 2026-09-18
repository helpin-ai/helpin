# Support reply delivery continuation

> Historical implementation record (2026-09-12), source-compared on 2026-09-17.
> Several composer details below have since changed. Original completion, browser
> and test claims, branch/push instructions and recovered-session references are
> not current validation or deployment instructions.

## Current behavior

- [Delivery state](../../frontend/src/components/support/replyDelivery.ts) is
  scoped to the current draft. Unknown presence defaults to chat for chat-source
  conversations; email-source conversations default to email. Confirmed offline
  presence can add eligible email, and returning online does not remove it from
  the same draft. Explicit manual/restored intent takes precedence.
- [ReplyComposer](../../frontend/src/components/support/ReplyComposer.tsx) sends
  explicit `delivery_mode` and channels for public replies and freezes the selected
  intent for failed retries. **A successful send clears the manual draft choice**,
  so it does not persist as a preference for every future reply. Undo can restore
  the prior reply's mode and attachments.
- The current composer has **no editable subject input and sends no `email_subject`
  field**. Subject helpers remain in `replyDelivery.ts`, but their presence does
  not make the old subject UI active. The backend
  [delivery service](../../server/internal/service/support_delivery.go) still accepts
  a subject override for public teammate email modes, validates 1–500 characters
  without control characters, and snapshots it into message metadata. Without an
  override, it uses the conversation subject or `Support conversation`.
- [Delivery selector](../../frontend/src/components/support/ReplyDeliverySelector.tsx)
  exposes chat-only, chat-and-email, and email-only with availability explanations.
  Backend validation maps an explicit mode to channels; legacy callers without a
  mode can still use the existing fallback path.

These are source comparisons; no email was sent and the historical browser/test
results below were not reproduced.

**Goal:** Complete the September 12 agreed composer plan on `waqar-fixes`.
**Architecture:** Reuse the explicit delivery backend and shared dropdown. Keep manual conversation preferences separate from automatic per-reply state. Persist a draft subject separately from the conversation title and snapshot it into each outgoing email.

- [x] Finish existing inline channel icons and disabled-option hover/keyboard tooltips in `ReplyDeliverySelector.tsx` and `quiet-dropdown.tsx`; verify the real route with Playwright.
- [x] Add delivery-state tests in `replyDelivery.test.ts`: unknown presence defaults to chat, confirmed offline adds eligible email, coming online never removes it, manual choices win, new automatic replies reset. Implement in `replyDelivery.ts` and `ReplyComposer.tsx`; use authoritative presence readiness.
- [x] Replace offline banner/confirmation with persistent exact recipient wording, include Cc, and show an editable underline Subject for email modes. Send an explicit mode, preserve manual choices through send/reload, and restore draft mode/subject/attachments for either Undo entry point.
- [x] Extend `CreateMessageRequest` with `email_subject`; validate and snapshot the value in message metadata, deliver separately for different subjects, retain reply threading. Backend-owned changes and regression tests stay in `server/`.
- [x] Extend `support-reply-delivery.spec.ts` for presence transitions, manual persistence, note mode, recipients, subject, failure, Undo, and narrow layout. Update obsolete offline-confirmation expectations.
- [x] Run focused frontend tests, deterministic browser suite, frontend production build, affected Go tests, Go vet/build; review final diff, commit and push `waqar-fixes`.

User decisions are recorded in the recovered session; no new design approval is needed. Existing automatic fallback remains for legacy/API callers; the composer always sends the displayed explicit intent. Do not promote branches as part of this continuation.

Validation: focused frontend unit and shared presence tests passed; all delivery browser scenarios passed across targeted Chromium runs (final Undo/privacy and failed-subject checks against the production build). Frontend TypeScript and production build passed. Affected Go service regressions, full Go vet and full Go build passed. Independent review findings for failed-send mode retention, duplicate keyboard Undo, and email-only Undo typing leakage were fixed and verified.
