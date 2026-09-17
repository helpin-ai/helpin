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

Implemented:
- Shared bounded classifier and permission-scoped candidate retrieval.
- Task creation/edit hooks with audited automatic labels and persistent manual-removal suppression.
- Actor-scoped assessment cache, failed-attempt daily cap and abandoned-attempt expiry.
- Task and unsaved-draft assessment endpoints, reviewed task type/team/relationship actions and persisted dismissals.
- Task creation duplicate preview and task-detail review section.
- Support match-first review, editable task draft, and creation of exactly the reviewed fields without a second generation call.
- Public support evidence fingerprints and stale-draft rejection.
- Source revision checks for task updates and transactional revision/access checks for task relationships.

Support compatibility decisions:
- With primary PM triage, match/draft review takes precedence over the old skip-dialog preference. The saved team default remains useful. Off/shadow modes retain the original create flow.
- Preview uses public messages, matching the evidence evaluated by Jev. The legacy create path retains its existing generator behavior.
- The reviewed create path preserves existing workflow validation, task creation, support linking, CRM association copying and navigation. Priority remains an editable draft field and never comes from Jev probability.
- Reviewed creation closes after task success even if preference persistence fails, preventing a preference retry from creating another task.

Still required before completion:
- Final concurrency review of support linking and source changes during reviewed mutations.
- Dedicated unsaved-draft async/stale-result tests and task-detail review state checks.
- Final permission/error-path audit and verification after any resulting fixes.

## Verification ledger

- Targeted classifier, repository, task-service and handler tests pass using synthetic evidence and a fake provider.
- Existing task/support creation regression tests pass.
- Tests cover reviewed type/team changes without changing priority, relationship acceptance, persisted dismissals, forged/stale choices, public-only support evidence, exact reviewed support creation without regeneration, provider and usage failure, and automatic-label suppression.
- Actual PostgreSQL tests pass for executing the SQL migration twice, concurrent daily-cap admission, concurrent deduplication, stale revision rejection and manual label removal.
- Frontend type checking and Community/Enterprise production builds pass.
- Backend Community/Enterprise API builds and backend vet pass; go mod tidy leaves module files unchanged.
- Shared review-row tests and existing CreateTaskModal tests pass (23 tests).
- Chromium full-app support tests pass for desktop/light, mobile/dark, keyboard linking and editable draft retention after a creation error. Screenshots were inspected.

No customer-data provider call, deployment, push or merge has been made. Tests use an isolated PostgreSQL container and mocked browser transport.
