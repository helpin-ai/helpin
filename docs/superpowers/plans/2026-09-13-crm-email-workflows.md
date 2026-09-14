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

## Scenario starters — 14 September follow-up

Add a persistent Browse starters entry in Templates and Sequences. One local curated catalog provides eight scenarios and 24 original email drafts. Choose a scenario, preview messages and relative timing, supply real business context, then open the existing editor. Required setup values must be supplied and HTML escaped; recipient merge fields remain supported. New sequences are unsaved drafts with review-mode steps and no automatic enrollment rule. No database migration, seeded workspace clutter or new sending engine.

Research: Outreach's sales email collection and standard outbound sequence guidance; Salesloft's cadence library and Big Book of Cadences; Apollo's Create a Sequence documentation; HubSpot's sequence template documentation. Vendor materials inform structure, not copied email content. Concise messages, a single low-friction ask, useful follow-ups, truthful context, spaced touches and a clear stopping point. Timing is an editable starting point, not a claimed universal optimum.

Sources: https://www.outreach.ai/resources/blog/sales-email-templates ; https://support.outreach.io/support/solutions/articles/159000426445 ; https://www.salesloft.com/platform/sales-engagement-software ; https://pages.salesloft.com/rs/432-WAJ-793/images/Cadence%20Best%20Practices%20for%20High%20Performing%20Teams%20(Big%20Book%20of%20Cadences).pdf ; https://knowledge.apollo.io/hc/en-us/articles/4409231193101-Create-a-Sequence ; https://knowledge.hubspot.com/sequences/create-and-edit-sequences .

- [x] Verify starter content, required fields, independent drafts, browser adoption flow, responsive preview, TypeScript/lint. Delivery commit is recorded in the session.

Verification: three starter unit tests and all 11 outreach browser tests passed. TypeScript and targeted ESLint passed. Inspected light, dark and 390px layouts. Confirmed adoption makes no API write, sequences begin as review-mode drafts, and unsaved adopted content has discard protection.
