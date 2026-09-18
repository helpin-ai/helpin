# GitHub PR status reconciliation plan

**Date**: 2026-04-23
**Author**: azhar
**Status**: Historical proposal; manual reconciliation implemented with different semantics.

## Current implementation review

Source-compared on 2026-09-17. This plan explains the original missed-webhook
problem for contributors. Its April incident and production metrics are historical,
and the proposal is not a description of a fully deployed reconciliation system.

The [GitHub client](../../server/internal/githubapp/client.go) now implements
`GetPullRequest`. Unlike the proposed contract, HTTP errors including 404 return
an error rather than `nil, nil`. The [service](../../server/internal/service/git.go)
implements `ReconcileOpenPullRequestStatuses`, invoked by the
[reconciliation command](../../server/cmd/reconcile-git-pr-status/main.go).
The command defaults to dry-run, a limit of 200, and a ten-minute context timeout;
`-apply` enables writes. It uses configured database and GitHub App credentials.

The [repository query](../../server/internal/repository/git.go) scans GitHub links
with a PR number and open or null status, oldest update first, across workspaces.
It has no 15-minute staleness predicate or 30-day cutoff. Invalid limits outside
1–1000 reset to 200. The service processes links sequentially, not with the proposed
five concurrent requests. Still-open results are counted without refreshing their
metadata. Its `Updated` counter increments before dry-run and write success checks,
so it is not a count of successfully persisted changes.

Applying reconciliation calls `updateDeliveryStatusForPR`, which updates delivery
state and calls `syncTaskWorkflowForPRStatus`. When team repository settings enable
auto-sync and configure the corresponding state, this can change task workflow
state. The metadata-only mitigation in the original risks section is **not** the
implemented behavior. Writes span multiple operations; this is not an atomic
all-links repair.

The proposed periodic worker, `GIT_PR_RECONCILER_ENABLED` switch, per-link refresh
route/button, and `last_webhook_*` integration fields were not found in current
source. There is no source basis for the promised 15-minute convergence window.
Webhook event recording/logging and [admin event routes](../../server/internal/router/router.go)
now exist, so the old blanket claim of no observability is also outdated. The
GitHub integration resolver still checks signatures conditionally when a secret
is present; the proposed strict empty-secret rejection is not implemented there.

Future schema additions must follow the [migration guide](../ops/database-migrations.md),
including installations with AutoMigrate disabled. The original “no dbmigrate SQL
required” statement is not current schema guidance. Provider rate limits and
capacity estimates below are historical assumptions, not verified current limits.
No live reconciliation or runtime tests were run for this documentation review.

## Original proposal and evidence

## Problem

PR #350 on helpin-ai/helpin was merged on GitHub but the task's Development History still shows it as `open` (see screenshot 2026-04-23 11:34). Other tasks in the workspace have likely drifted the same way. The UI reads `task_git_links.pr_status` directly, so "open" in the card means the row was never updated.

Today's pipeline depends **entirely** on a single GitHub webhook delivery to keep PR status in sync:

1. GitHub sends `pull_request` (action=closed, merged=true) → `POST /api/git/webhook`
2. `handleGitHubPR` (`server/internal/handler/git.go:349`) parses `merged`→`"merged"`.
3. `GitService.ProcessWebhookPR` (`server/internal/service/git.go:746`) updates `task_git_links.pr_status`, `task_delivery_targets.delivery_state`, and optionally advances the workflow state.

If that one delivery is dropped, mis-signed, or the task link lookup fails, status stays stuck forever. There is no reconciler, no manual refresh, and no visibility into whether webhook delivery is even working.

## Historical evidence of the gap (April code audit)

- **No GitHub PR-read API**: `server/internal/githubapp/client.go` exposes `MintInstallationToken`, `ListInstallationRepositories`, `GetInstallation`, `ListRepositoryBranches`, `MergeBranch` — no `GetPullRequest`.
- **No periodic reconciliation**: `rg "ReconcilePR|refresh.*pr|sync.*pr"` across `server/internal` returns nothing git-related. No Temporal workflow, cron, or worker touches open PR links.
- **Silent drops in the webhook handler** (`handler/git.go`):
  - Signature/installation resolution failure → `401` with the error, but no structured log of the failure context (repo, action, PR number).
  - `repo == ""` or missing `pull_request` object → returns `{"status":"ignored"}` with no log.
- **Silent drops in the service** (`service/git.go:746`): when no `task_git_link` matches `(repo, pr_number)` or `(repo, branch)`, `ProcessWebhookPR` exits quietly. The only write is gated on `if link != nil`.
- **Empty-secret fallback** (`service/git.go:423`): if `integration.WebhookSecret` is empty, signature is skipped — integrations created before webhook-secret support (or where the secret was never pushed to the GitHub App) may silently accept unsigned deliveries *or* reject signed ones with no admin-visible signal.
- **Secret provisioning is manual**: each integration generates a `WebhookSecret` on create (`service/git.go:135`), but the GitHub App's webhook secret is configured once at App level in GitHub's UI. If the admin never copies our generated secret into GitHub, every delivery fails signature check and returns 401.

## Goals

1. Task PR status converges to ground truth within a bounded window (≤ 15 min) even if a webhook is missed.
2. Admins can tell whether webhook delivery is healthy without leaving Helpin.
3. Users have a one-click way to force-sync a stuck link.
4. No silent drops — every webhook outcome is observable in logs.

## Non-goals

- Replacing webhooks with polling as the primary path. Webhooks remain the real-time mechanism; polling only backfills missed events.
- Reconciling PR review state, check runs, or merge-queue status (out of scope, separate follow-up).
- Retroactively editing closed PRs older than 30 days.

## Proposal

### 1. Add a `GetPullRequest` method to the GitHub App client

File: `server/internal/githubapp/client.go`

```go
type PullRequest struct {
    Number      int
    Title       string
    HTMLURL     string
    State       string // "open" | "closed"
    Merged      bool
    HeadRef     string
    BaseRef     string
    HeadSHA     string
    UpdatedAt   time.Time
}

func (c *Client) GetPullRequest(ctx context.Context, installationID, owner, repo string, number int) (*PullRequest, error)
```

Uses `GET /repos/{owner}/{repo}/pulls/{number}` with the installation token. Returns `nil, nil` for 404. Follows the existing retry / token-minting pattern in the file.

### 2. Reconciler service + worker

New file: `server/internal/service/git_reconcile.go`

- `ReconcileTaskGitLink(ctx, link *model.TaskGitLink)`: fetches the PR via `GetPullRequest`, derives `prStatus` with the same logic as the webhook handler, and reuses `ProcessWebhookPR`'s update path (extract the "apply PR update" block into a private helper so both entry points share it).
- `ReconcileStalePRLinks(ctx, olderThan time.Duration)`: lists links where `pr_status = 'open'` AND `updated_at < now - olderThan`, reconciles each with bounded concurrency (errgroup + `SetLimit(5)`).

New worker: `server/internal/worker/git_reconciler.go`

- Ticks every 10 min, calls `ReconcileStalePRLinks(ctx, 15*time.Minute)`.
- Registered in `cmd/api/main.go` alongside existing workers.
- Structured log per run: `reconciler=git-pr reconciled=N changed=M errors=E`.

### 3. Manual "Refresh" action on `TaskGitPanel`

- New endpoint: `POST /api/workspaces/{wid}/git/links/{linkID}/refresh` → `GitHandler.RefreshLink` → `GitService.ReconcileTaskGitLink`.
- Permission: `pm.edit` (same as other link mutations).
- Frontend: add a small refresh icon next to `link.pr_status` in `frontend/src/components/pm/TaskGitPanel.tsx`. Spinner while pending; toast on success with the new status; toast on error. Invalidates `queryKeys.pm.task(taskId).git` on success.

### 4. Observability + admin surfaces

- **Log every webhook outcome** in `handler/git.go`:
  - `INFO github webhook received event=pull_request action=closed merged=true repo=... pr=... workspace=...`
  - `WARN github webhook dropped reason=no_matching_link repo=... pr=...`
  - `WARN github webhook dropped reason=signature_invalid installation=...`
- **Persist last-delivery metadata** on `git_integrations`:
  - Add columns: `last_webhook_received_at`, `last_webhook_event`, `last_webhook_status` (`ok` / `signature_invalid` / `no_installation_match` / `ignored`).
  - Write these at the top of `handleGitHubPR` / `ResolveGitHubWebhookWorkspace`.
  - Surface in the Integrations settings page: green "Receiving events" pill if `last_webhook_received_at < 24h`, red "No events in X days" otherwise.
- **Remove the silent empty-secret fallback**: if `integration.WebhookSecret == ""`, treat delivery as misconfigured and return 412 Precondition Failed with a log — never accept unsigned webhooks. Migration: any integration with empty secret gets one generated on first boot, admin notified via a settings-page warning to paste it into GitHub.

### 5. Data + migration changes

- Columns on `git_integrations` (AutoMigrate handles adds):
  - `last_webhook_received_at TIMESTAMPTZ NULL`
  - `last_webhook_event TEXT NULL`
  - `last_webhook_status TEXT NULL`
- No destructive changes. No dbmigrate SQL required for the column adds.
- Index on `task_git_links (pr_status, updated_at)` via dbmigrate to keep the reconciler query cheap: `internal/dbmigrate/sql/202604230001_task_git_links_stale_idx.sql`.

## API surface

| Method | Path | Permission | Body |
|---|---|---|---|
| POST | `/api/workspaces/{wid}/git/links/{linkID}/refresh` | `pm.edit` | — |

Response: `{ link: TaskGitLink }` with fresh `pr_status`, `pr_title`, `pr_url`, `commit_sha`.

## Rollout

1. Ship `GetPullRequest` + `ReconcileTaskGitLink` + manual refresh endpoint + UI button — behind no flag, low blast radius.
2. Ship the periodic reconciler worker with a kill-switch env var `GIT_PR_RECONCILER_ENABLED` (default true in staging, gated in prod for one week).
3. Ship webhook-delivery logging + `last_webhook_*` columns + admin pill.
4. Remove empty-secret fallback once all prod integrations have generated secrets confirmed installed in GitHub.

## Success metrics

- 100% of merged PRs linked to a task reach `pr_status='merged'` within 15 min of merge (measured by comparing our `pr_status` against GitHub API state on a sample).
- Zero "stuck open" links older than 1 hour in production dashboards.
- Every integration shows `last_webhook_received_at` within the last 7 days (otherwise admins get a warning banner).

## Risks

- **Rate limits**: GitHub App installation tokens are 5,000 req/hr per installation. Reconciler scans every 10 min over all open links; bound concurrency (`errgroup SetLimit(5)`) and only fetch links stale > 15 min. Expected headroom ≥ 100×.
- **Webhook secret cutover**: removing the empty-secret fallback could break integrations whose secret was never installed in GitHub. Mitigation: staged rollout, admin warning banner first, strict mode second.
- **Workflow state auto-advance on reconcile**: `ProcessWebhookPR` advances workflow state when `auto_sync_states=true`. A reconciler that retroactively marks a PR "merged" could move a story to Done days after the fact. Mitigation: reconciler's shared helper accepts a `source` flag; only webhook-path advances workflow state. Reconciler updates PR metadata only, leaves workflow state alone and emits a log for manual follow-up.

## Out-of-scope follow-ups

- Check-suite / CI status reconciliation.
- PR review-request and review-state reconciliation.
- Multi-repo PR linking (one task → many PRs across repos).
- Backfilling historical closed PRs on first install.
