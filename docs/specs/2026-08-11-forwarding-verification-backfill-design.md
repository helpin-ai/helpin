# Forwarding Verification Backfill Design

> Historical backfill design/plan, source-compared on 2026-09-17. The
> [migration](../../server/internal/dbmigrate/sql/202608110004_backfill_support_email_route_verification.sql)
> exists and fills only unverified routes using the latest qualifying inbound log.
> Gmail/Zoho confirmation exclusions are present. Its
> [contract test](../../server/internal/dbmigrate/migrator_test.go) checks SQL
> clauses; that test is not proof that a particular database applied the migration.
> Existing completion checkboxes describe the original work, not a fresh test run.

Current route verification is broader than the historical round-trip description:
[the inbound service](../../server/internal/service/email_fallback.go) accepts a
matching test token or qualifying ordinary inbound mail mentioning the configured
source address. Provider confirmation messages do not verify forwarding.


Existing email routes became orange when `forwarding_verified_at` was introduced because historical routes started with that field empty. A route's `last_inbound_at` alone is not sufficient evidence: a provider confirmation email can set it before forwarding is enabled.

Add one idempotent database migration that marks a historical route verified only when `support_email_logs` contains a qualifying inbound message for that route. Gmail and Zoho confirmation messages are excluded using the same sender, subject, and confirmation-link signals as the inbound service. The latest qualifying log timestamp becomes `forwarding_verified_at`; routes with no qualifying evidence remain incomplete.

No UI fallback, cutoff date, provider OAuth, or user action is added. New routes continue using the round-trip test.

