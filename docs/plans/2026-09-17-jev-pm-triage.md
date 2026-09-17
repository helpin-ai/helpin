# Jev for PM: implementation and verification

Authorized scope: task triage, duplicate/related-task suggestions, and support-to-task matching. Existing labels may be applied automatically at high confidence. People review type/team changes, links, and task creation. Priority is never inferred from classification confidence.

## Product contract

- Classify created or substantively edited tasks using existing type, team and label taxonomy.
- Retrieve candidates within the user's workspace/team access before provider calls. Distinguish duplicates from related but separate work. Explain that only retrieved candidates were checked.
- Offer matching existing tasks before support creates additional work. Preserve the source conversation link and allow review of a new task draft.
- Preserve existing task creation, agent-run, team defaults, workflow validation, billing, support linking, CRM associations, navigation and failure behavior.
- Automatic labels must not overwrite manual label removals. No automatic team changes, links, merges, closure, or new tasks.
- Revalidate source revisions and current target permissions on review. Stale suggestions cannot mutate current work.
- Use the existing provider, a separate PM mode/threshold/daily cap, and shared workspace allowlist. Support lifecycle controls remain independent. Record operator-funded provider usage and audit metadata without raw source text.
- Off mode/provider absence preserves existing flows. Shadow records evaluations without applying or exposing actionable suggestions. Provider failures never fail task creation or edits.

## Implementation status

Built, with targeted tests:
- Shared closed-choice classifier, confidence abstention, bounded complete context.
- Candidate retrieval with workspace/team filtering before ranking.
- SQL-owned assessment and label-suppression tables (PostgreSQL execution still to verify).
- Actor-scoped assessment cache, provider admission, failed-attempt daily cap, pending-attempt expiry.
- Task/public-support source adapters, usage recording and independent configuration.
- POST task and support assessment endpoints and dependency wiring.
- Automatic create/edit hooks, transactional label additions and activity provenance.
- Persistent manual removal suppression for both label actions and task form updates; source edits during provider calls prevent label application.

Still required:
- Reviewed actions and persisted dismissals, including stale source/candidate checks.
- Task creation duplicate preview, task detail triage UI, support match/draft review UI.
- Support creation compatibility audit and reviewed draft implementation.
- Integration tests for public-only support evidence, source edits during calls, reviewed mutations, permission changes and provider/usage failures.
- PostgreSQL migration/concurrency tests, community/enterprise build and vet, frontend tests/build and rendered state checks.

## Verification ledger

Targeted classifier, repository and task-service tests passed with synthetic evidence and a fake provider. No customer-data provider call has been made.

`go run ./cmd/migrate validate` requires a database connection and could not run without DATABASE_URL; this is not evidence that the migration has been applied or validated against PostgreSQL.
