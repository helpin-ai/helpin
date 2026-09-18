# CRM individual email foundation

> Historical implementation record (2026-09-13), source-compared on 2026-09-17.
> Compose/reply, mailbox signatures, deal entry and draft protection are present.
> Original test totals, visual inspection and external inspiration notes below
> are historical, not fresh verification or refreshed vendor documentation.

## Current implementation and limits

- [Email service](../../server/internal/service/crm_email.go) validates recipients,
  deal workspace, mailbox ownership and active connected Gmail state before send.
  Signature updates are mailbox-owner scoped; the server limit is **10,000 bytes**
  (`len(signature)`), despite the error wording saying characters.
- [Composer](../../frontend/src/components/crm/CRMEmailComposerDialog.tsx) and
  [reply composer](../../frontend/src/components/crm/CRMEmailReplyComposer.tsx)
  include signatures through the [HTML escaping helper](../../frontend/src/components/crm/emailComposition.ts).
  Signature inclusion is a composer action, not an automatic guarantee for every
  API caller. [EmailTimeline](../../frontend/src/components/crm/EmailTimeline.tsx)
  retains mounted reply drafts by workspace/thread and adds an unload warning.
- Successful provider send is distinct from local persistence. Failed deal linking
  produces `association_warning`; failure to store the sent message returns the
  sent result without failing the send, while attachment/contact/count update
  failures are logged. A successful response therefore does not prove every local
  record or association was persisted. Do not retry delivery solely to repair a
  local record.
- Playbook-intent sending has a separate message-ID reconciliation path; its
  existence does not provide a blanket exactly-once guarantee for ordinary sends.
  See [email sync](../crm-email-sync.md) for participant and ingestion boundaries.

No live mail, browser session, deployment or fresh runtime test suite was used in
this source review.

Goal: compose and reply confidently from contact and deal records using the existing Gmail integration.

Design: compact address rows and optional Cc, shared Quiet rich-text composer, saved per-mailbox signatures, explicit deal context, and protected drafts. Preserve attachments, AI rewrite, reply-all, conversation history, permissions and task linking. Templates and sequences follow separately.

Inspiration reviewed: Attio Send emails documentation and its compose/reply screenshots (https://attio.com/help/reference/email-calendar/send-emails-in-attio), Close Emailing documentation (https://help.close.com/feature-guide/emailing). Apply compact headers and progressive disclosure within Helpin's design system.

- [x] Add owner-scoped mailbox signature persistence and settings control.
- [x] Improve new composer: recipients, signature preview, draft protection, busy guards and accessible compact layout.
- [x] Add deal compose entry and validated association on send.
- [x] Protect reply drafts and keyboard submission; retain existing thread semantics.
- [x] Verify behavioral tests, TypeScript, lint, Go tests/build/vet and browser visual states with mocked delivery.

Use this one document for scope and progress. No subagents or new worktrees. Do not send real mail during verification.

Implementation notes: signatures are saved per mailbox as plain text and safely included in HTML mail. Reply text and attachment drafts survive conversation switching within the mounted timeline; refresh prompts before losing a draft. Cross-session/cross-device draft autosave is not included. Gmail remains the supported sending provider. New sends validate mailbox ownership, recipients and deal workspace before delivery. Confirmed delivery with failed deal linking returns a warning rather than a retryable send error.

Verification: 10 Playwright scenarios passed across the final suite and the deterministic reply rerun, including light/dark/390px layouts; 6 EmailTimeline tests passed. CRM email service tests and affected repository/Temporal email tests passed. TypeScript build, targeted ESLint, Go build ./..., Go vet ./..., and git diff --check passed. No live delivery or deployment performed.
