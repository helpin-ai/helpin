# CRM email workflows — steps 2–6

Goal: finish templates, sequences, enrollment, monitoring and automatic enrollment without a second general-purpose workflow builder. Work only in waqar-fixes; no subagents.

Design choice: Attio's inline email steps and compact settings; Close's separate recipient activity and review-before-send option. Reviewed their official documentation and screenshots, plus HubSpot's manual task screen. CRM gains one Emails destination with Templates, Sequences and Activity. Contacts/deals expose enrollment with a personalized preview. Reuse Quiet controls and the existing conversation editor.

- [x] Templates: personal/shared library, search, reusable rich text and subject, variables/fallbacks, preview, insert and save from composer.
- [x] Sequences: ordered email/task steps, delays, automatic/review email modes, delivery window/timezone, publish/pause/archive. Enrollment snapshots insulate active recipients from later edits.
- [x] Enrollment: contacts/deals and bulk contact selection; owned connected sender, personalization validation, duplicate and suppression checks, preview before start.
- [x] Monitoring/execution: durable progress on existing Temporal schedule; stop on reply, bounce, opt-out or closed deal; pause/resume/stop, review and native follow-up tasks. Persist a send intent before provider calls and reconcile ambiguous delivery without automatic resends. Show real counts and next actions, no invented open rates.
- [x] Automatic enrollment: bounded deal-stage entry rules with explicit sender authorization, using CRM activity events and the existing scheduler; expose connections from Automations as well as sequence settings.
- [x] Verify behavior, permissions, retries, timing, native email/task integrations, responsive light/dark screens, TypeScript/lint and Go tests/build/vet. Commit/push; no deployment or real outbound messages during tests.

Sources: https://attio.com/help/reference/automations/sequences/create-a-sequence ; https://help.close.com/feature-guide/workflows ; https://knowledge.hubspot.com/sequences/create-and-edit-sequences .

## Verification and delivery

- Implemented steps 2–6 in the existing `waqar-fixes` worktree. Gmail/Google Workspace remains the native sending provider from step 1.
- Backend package checks passed for service, repository, handler, Temporal activities, API and worker. Focused outreach tests cover private/shared templates, personalization and escaping, version conflicts, snapshots, duplicate enrollment, rate limiting, review approval, ambiguous delivery reconciliation, reply/opt-out/invalid-address stops, revoked access, task ownership and completion, stage enrollment cursors, pagination and workspace isolation. Router checks cover public preferences, protected library and CORS. Go build and vet passed.
- Browser verification: 11 composer checks, 7 outreach checks, including public unsubscribe, review discard protection, personalized preview, stage configuration, and light/dark/390px screenshots. Six existing EmailTimeline unit tests passed. TypeScript and targeted ESLint checked.
- Corrected an existing CRM activity test fixture to include the already-existing revenue_type column so the broader backend tests run cleanly.
- Operational behavior: existing Temporal tick, weekday/timezone windows, up to 100 sequence emails per mailbox per rolling day and at least one minute between emails. Recipient steps are snapshotted. Unconfirmed provider delivery is held for reconciliation; it is never blindly resent. Automatic enrollment applies to future stage changes and uses the deal's primary contact, once per recipient per sequence.
- No deployment or real outbound email was performed. Final commit/push is recorded in the session.
