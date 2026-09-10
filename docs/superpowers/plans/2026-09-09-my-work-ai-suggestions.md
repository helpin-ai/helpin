# My Work AI suggestions implementation plan

> Use independent subagents for the bounded backend changes and review, with the primary agent implementing the UI and integration.

**Goal:** Route explicitly internal meeting follow-up suggestions to My Work, retain customer and uncertain suggestions in CRM, and present a minimal personal review queue.

**Architecture:** Reuse canonical crm_suggestions records and revision guards. Store action-level routing in suggestion context (meeting_follow_up_scope: internal/customer/uncertain, routing version). Classify new meeting follow-ups from their draft plus transcript; backfill pending legacy meeting follow-ups in bounded background batches without regenerating meeting artifacts. Only explicit internal classification leaves CRM. PM endpoints authorize PM access and personal ownership using suggestion user, meeting owner, or creator; never expose workspace-wide suggestions to PM users. UI list shows title, meeting, and received time; drawer reveals draft, source, and review/dismiss controls. Approval records review only; no message is sent or duplicate task created.

**Tech stack:** Go/GORM/Temporal, React/TypeScript/TanStack Query, Quiet design primitives.

- [x] Backend classification, safe legacy backfill, and CRM queue filtering with tests.
- [x] PM personal suggestion list/detail/decision API with ownership, visibility, revision and duplicate guards; tests.
- [x] My Work AI suggestions list, pagination, drawer and accessible states; frontend tests.
- [x] Integrated build, targeted tests, browser review and independent code review.
- [x] Commit only task files and push waqar-fixes.

Existing meeting action items and linked tasks remain canonical. The new follow-up review does not convert summaries into tasks. Unknown ownership stays outside the personal queue. Unknown routing stays visible in CRM until classified; lack of a customer link is not classification evidence.

## Validation

- My Work: 9 unit tests and 6 Playwright browser scenarios passed, including narrow dark layout, keyboard focus, read-only access, stale decisions, and successful review/dismissal.
- Frontend TypeScript build passed.
- Targeted Go service, repository, handler and Temporal tests passed; API and worker builds passed.
- Independent UI/backend review completed; durable completion prevents reviewed suggestions from resurfacing after owner changes.
- Historical routing runs independently every five minutes in bounded batches. Ambiguous or adopted CRM work stays in CRM.
