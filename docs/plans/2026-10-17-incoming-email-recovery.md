# Incoming email recovery implementation plan

This plan covers the approved Incoming tab and concise recovery drawer in Helpin Admin. Status: implemented and verified locally. It is source-level work, not a deployment or a change to receiving-domain DNS.

**Goal:** Let platform admins inspect retained incoming emails, confirm a valid destination, and retry processing without losing or duplicating messages.

**Architecture:** Extend the existing durable `support_inbound_jobs` queue and platform-admin endpoints. Keep email processing, attachments, notifications, and deduplication on the existing pipeline. Add a small durable recovery record to each job; completed payloads continue to be cleared.

**Classification:** Shared support processing and the existing operator admin app. All fixtures are fictional; no customer data, credentials, or private infrastructure changes are included. Platform-admin/MFA authorization remains required.

**Execution:** Work in the existing `waqar-fixes` worktree without subagents. Use the executing-plans workflow and focused behavioral tests.

## Capability audit and decisions

- Preserve outgoing queue data, countdowns, message expansion, refresh, and empty/loading states under Outgoing.
- Keep Email Diagnostics and Webhook Events available; do not duplicate their configuration/log screens in Incoming.
- Incoming defaults to Needs attention and supports search, pagination, in-progress recovery, and recovered history.
- Drawer leads with Choose a conversation, Confirm the destination, or Retry this email. Original email and technical details are collapsed. Keep for later performs no write.
- Suggest conversations sharing the actual sender; require explicit selection. A known workspace cannot be changed. Reject unrelated senders, erased/deleted/spam destinations, stale selections, and active processing leases.
- Recovery is asynchronous and reports completion only after the worker succeeds. Preserve email content/attachments, receipt time, existing deduplication, and normal inbox/AI processing policies.
- Do not introduce automatic cross-workspace guessing, domain changes, bulk recovery, new notifications infrastructure, or a separate admin app.

## Implementation

- [x] Add failing service tests for recovery, sender/workspace restrictions, stale or concurrent requests, worker deduplication, receipt time, and payload privacy.
  Files: `server/internal/service/email_inbound_admin_test.go` and existing inbound test fixtures.
- [x] Add recovery fields and versioned migration; repository queries for paginated summaries, detail, conversation candidates, and conditional recovery requests.
  Files: `server/internal/model/support_inbound_job.go`, `server/internal/model/support_inbound_admin.go`, `server/internal/repository/support_inbound_admin.go`, `server/internal/dbmigrate/sql/202610170001_support_inbound_recovery.sql`.
- [x] Add service and admin handlers, wire routes, and reuse the worker/reply pipeline with explicit recovery targets and receipt time. Read metadata from retained payloads or existing email logs after completion; never return raw attachment data in lists.
  Files: `server/internal/service/email_inbound_admin.go`, `email_inbound_jobs.go`, `email_fallback.go`, `server/internal/handler/admin_inbound_email.go`, `server/internal/router/router.go`.
- [x] Add Incoming list/drawer and query/service types; preserve outgoing queue and add a navigation attention count.
  Files under `apps/admin/src/features/email-queue/`, `hooks/queries/`, `lib/services/`, `lib/`, and `components/sidebar.tsx`.
- [x] Verify focused Go tests in Community and EE, migration validation and PostgreSQL behavior, admin typecheck/build/lint, and browser interaction against controlled fixtures. Review authorization, privacy, duplication, and error paths.
- [x] Update this plan with results, remove only this task's mockup files, and commit the finished change. Do not deploy or push without the user's instruction.

## Verification commands

- `cd server && go test ./internal/service -run 'TestAdminInbound|TestInbound' -count=1`
- `cd server && go test -tags ee ./internal/service -run 'TestAdminInbound|TestInbound' -count=1`
- `cd server && go test ./internal/handler ./internal/authorization ./internal/repository`
- `cd server && go test -tags integration ./internal/service -run TestAdminInboundRecoveryPostgres -count=1` (disposable PostgreSQL; includes migration replay and ledger validation)
- `pnpm --dir apps/admin build` and targeted ESLint on changed admin files.
- `python3 scripts/docs/check_names.py`, `python3 scripts/docs/check_links.py`, and `git diff --check`.

## Results

- Recovery and existing inbound/email-fallback regressions passed with the pinned Go 1.26.7 toolchain, in Community and EE.
- Handler, authorization, and repository suites passed. Registered-route tests reject anonymous users, workspace admins, and platform admins without MFA.
- PostgreSQL integration tests passed against an isolated disposable container: migration replay/checksums, JSON queue/search queries, concurrent recovery requests, original receipt time, reopening, attachment retention, and conversation deletion/privacy triggers.
- Admin production build, TypeScript, and targeted ESLint passed. Vite reports its existing large-bundle advisory.
- Browser checks with fictional API fixtures passed for explicit destination selection, ambiguous matches, Keep for later, attachment download, asynchronous completion, failed retries, cached-content removal after a 404, outgoing queue, loading/errors, dark mode, and the mobile drawer. No browser runtime errors occurred. Dates rendered in the browser's selected timezone without the current year.
- Documentation naming/link checks, diff whitespace checks, and the repository secret scanner passed. The scanner covers committed files; the new files were separately reviewed and contain only fictional test data.
- Failed emails appear immediately, including those awaiting an automatic retry. Manual retries report failures promptly and retain their payloads. Successful processing links the existing receipt log and clears the queue payload.
- Email Diagnostics stays read-only; recovery lives in Email Queue → Incoming. No receiving-domain, live email, workspace, or deployment settings were changed.
- Scope: manual recovery confirms an existing active conversation with the actual sender or a known CC/thread participant. The feature does not infer identity from a forwarded body or recover emails rejected before they reached Helpin's inbound webhook. Existing routed retries use the normal email pipeline.
- Removed only the incoming-email mockup for this task; unrelated mockups remain. Changes are committed locally on `waqar-fixes`; no push or deployment is included.
