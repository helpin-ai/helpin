# Inbound email reliability implementation plan

Approved scope: durable receipt and retries; independent attachment processing with Processing/Unavailable/Retry; attachment-only messages; actual CC sender attribution; unknown senders retained team-only with human attention; reference-based threading; expandable message recipient details. No reply/reply-all controls, recipient toggles, new settings, review queue, or forwarding feature.

Architecture: database-backed inbox jobs acknowledged only after commit. Separate receipt and attachment workers with bounded execution, leased claims, stable IDs, retry backoff and exhausted-job alerts. Attachment placeholders and jobs committed with their message. Existing hydration exposes attachment state. Workspace-authorized retry endpoint requeues failed attachments. Completed raw job payloads cleared; privacy deletion covers retained jobs. Reuse existing message styles and email details.

- [x] Regression tests for CC/unknown senders, attachment-only email and unrelated threading; implement policies.
- [x] Durable receipt/job schema, migration, claim/retry tests and ingress worker wiring.
- [x] Independent attachment jobs, placeholder states, stable uploads and authorized retry; test failure/restart/deduplication.
- [x] UI attachment states and retry, team-only label, actual sender avatars, expandable recipients; focused tests.
- [x] Privacy, authorization, race and lifecycle review; backend/frontend checks, migration validation and commit on waqar-fixes.

Production replay remains separate: reconcile provider IDs against stored messages before replay, preserving timestamps and avoiding duplicate automation. No production mutations in this change.

Review findings: the former attachment insert used a separate database connection while the email transaction held a conversation lock; a PostgreSQL integration test reproduces that blocking path. Files now upload after the message transaction commits. Constraint failures count as duplicates only when a saved provider receipt exists. Trusted replies can register later CC participants without changing selected outbound recipients.

Validation: service, handler, repository and migration suites; isolated PostgreSQL attachment commit/replay and deletion/anonymization tests; 60 focused UI tests; app TypeScript check; API build. Targeted lint reports two pre-existing MessageBubble errors (exported helper/fast-refresh and Date.now render purity); new components pass.
