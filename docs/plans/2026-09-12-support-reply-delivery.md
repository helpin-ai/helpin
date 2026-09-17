# Support reply delivery continuation

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
