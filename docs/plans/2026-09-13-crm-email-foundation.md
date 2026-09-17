# CRM individual email foundation

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
