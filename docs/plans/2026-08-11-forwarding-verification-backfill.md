# Forwarding Verification Backfill Implementation Plan

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


> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore green verification for historical routes with real inbound-email evidence without accepting provider confirmation emails as proof.

**Architecture:** Add an idempotent PostgreSQL data migration that derives verification from existing route-linked inbound logs. Protect its evidence rules with a migration contract test.

**Tech Stack:** Go tests, embedded PostgreSQL migrations.

---

### Task 1: Backfill historical route verification

**Files:**
- Create: `server/internal/dbmigrate/sql/202608110004_backfill_support_email_route_verification.sql`
- Modify: `server/internal/dbmigrate/migrator_test.go`

- [x] Add a failing migration contract test covering qualifying inbound evidence and Gmail/Zoho exclusions.
- [x] Run `go test ./internal/dbmigrate -run TestSupportEmailRouteVerificationBackfillMigrationContract -count=1` and confirm it fails because the migration is absent.
- [x] Add the idempotent backfill migration.
- [x] Rerun the focused test and migration validation.
- [x] Run `git diff --check`, inspect the diff, and commit.
