# Forwarding Verification Backfill Implementation Plan

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
