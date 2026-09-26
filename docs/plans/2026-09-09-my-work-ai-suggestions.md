# My Work AI suggestions implementation plan

> Historical implementation record (2026-09-09), source-compared on 2026-09-17.
> The feature exists; the completed checkboxes, test totals, screenshots/review
> claims, agent assignments and `waqar-fixes` push below describe the original
> session, not fresh validation or instructions for this audit.

## Current implementation references

- [Routes](../../server/internal/router/router.go) require `pm.read` for list/detail
  and `pm.edit` for decisions. The [service](../../server/internal/service/pm_ai_suggestion.go)
  also requires an active member in the requested workspace and a valid revision.
- The [repository](../../server/internal/repository/pm_ai_suggestion.go) filters
  explicit internal meeting follow-ups by suggestion recipient, otherwise meeting
  owner, otherwise creator, plus meeting visibility. Lists are 25 items per page.
  Adopted customer work is excluded. Decisions recheck access and canonical revision
  under transaction locks; accepting records `accepted` / `manual_required`, not
  execution of the draft. Review markers keep completed work out of the CRM queue.
- [Routing](../../server/internal/service/meeting_follow_up_routing.go) processes
  at most three historical suggestions per batch, with a 90-second batch deadline
  and 30-second classification deadline. Missing AI/transcripts or failed evidence
  validation leave work available in CRM. Transcript input is capped at 120,000
  bytes for this classification; it is not a full-transcript guarantee.
- The [Temporal workflow](../../server/internal/temporalapp/crm_meeting_follow_up_routing.go)
  is scheduled every five minutes by worker startup. This describes configured
  code, not proof that a deployed worker or schedule is healthy.
- [My Work suggestions](../../frontend/src/components/pm/my-work/AISuggestions.tsx)
  implements pagination, detail/review UI, and stale-decision messages. Current
  source inspection does not reproduce the historical browser or test results.

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
