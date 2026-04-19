# PRD: Org-Scoped GitHub Integrations with Per-Workspace Repo Mapping

**Status**: Draft
**Author**: Azhar
**Date**: 2026-04-18
**Target branch**: `feature/github-integration-org-scoping`
**Related**: Builds on the GitHub App install flow introduced in `internal/githubapp/` and `internal/service/git.go`.

---

## 1. Context

Helpin connects to customer repositories via a GitHub App installation. Today, each `git_integrations` row is bound to a single `workspace_id`, and the install callback rejects any attempt to reuse an `installation_id` across workspaces.

This worked for single-workspace customers, but breaks for the common case where one Helpin organization owns multiple product workspaces that all live inside the **same GitHub organization**.

## 2. Problem

A GitHub App installation is a per-GitHub-account resource. GitHub issues exactly one `installation_id` per account, and "installing again" from a second Helpin workspace just returns the same id.

Concrete customer scenario:

- Helpin Organization "D4" has two workspaces: **Contentpen** and **ContentStudio**
- GitHub org `d4` has the Helpin App installed — `installation_id=X`
- Contentpen connects a repo → `git_integrations(workspace=Contentpen, installation_id=X)` is created
- ContentStudio tries to connect a different repo under the same `d4` org → callback returns the same `installation_id=X` → service rejects with "installation already connected to another workspace"

The user has no workaround. Uninstalling from Contentpen to make room for ContentStudio breaks Contentpen. This is a P0 blocker for multi-workspace customers and will recur for every customer that grows beyond one workspace.

Root cause: the uniqueness check is at the **workspace** boundary, but GitHub installations are a **GitHub-account** boundary. The right Helpin-side boundary is the **organization**.

## 3. Goals

1. Allow multiple workspaces within the same Helpin organization to share a single GitHub App installation.
2. Let each workspace independently choose which repos from the installation it consumes.
3. Reject cross-organization reuse of the same `installation_id` (the real security boundary).
4. Route webhooks to only the workspace(s) that own the affected repo.
5. Ship with zero breakage for existing single-workspace customers.

## 4. Non-Goals

- Cross-workspace repo sharing (one repo → one workspace is enforced).
- Supporting multiple GitHub installations per organization targeting the same GitHub account (GitHub disallows this).
- Changing the agent runner's execution contract beyond the integration lookup call.
- Introducing GitLab / Bitbucket support.

## 5. User Stories

- **As an admin of a multi-workspace org**, I can connect the GitHub App once and then, from any workspace in my org, pick which repos to wire up — without being asked to reinstall or disconnect anything.
- **As an admin**, when a second workspace clicks "Connect GitHub" and the org already has an active install, I go straight to a repo picker instead of the GitHub install flow.
- **As an admin of Org B**, I cannot claim an `installation_id` already bound to Org A, even if my GitHub App install would technically return the same id (e.g. shared GitHub account scenarios — we reject explicitly).
- **As a developer**, when a PR is opened in a repo wired to ContentStudio, the webhook only fires agent runs / story updates inside ContentStudio, not Contentpen.

## 6. Proposed Architecture

Mirror GitHub's real shape:

- **GitHub installation = organization-level resource** in Helpin → `git_integrations.organization_id`
- **Repos = workspace-consumable units** → new or extended `git_repositories` table binding repos to workspaces

### 6.1 Data Model

**`git_integrations`** (modified)

| Column | Change |
|---|---|
| `organization_id` | **New**, non-null, FK → `organizations.id` |
| `workspace_id` | **Dropped** after cutover |
| `installation_id` | Unchanged |
| `provider` | Unchanged |
| `account_login`, `app_id`, `webhook_secret`, `credential_mode`, `active` | Unchanged |

Indexes:
- `UNIQUE (provider, installation_id) WHERE active = true` — authoritative uniqueness, closes the current race.
- `INDEX (organization_id, provider)`

**`git_repositories`** (new or extended)

```
id                uuid PK
integration_id    uuid NOT NULL REFERENCES git_integrations(id) ON DELETE CASCADE
workspace_id      uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE
github_repo_id    bigint NOT NULL
name              text NOT NULL        -- "owner/repo"
default_branch    text
active            bool DEFAULT true
created_at        timestamptz
updated_at        timestamptz

UNIQUE (integration_id, github_repo_id)   -- one workspace per repo, org-wide
INDEX  (workspace_id, active)
INDEX  (integration_id, github_repo_id)
```

One repo maps to **exactly one workspace** within an organization. This is intentional — a PR/issue event should route to one inbox.

### 6.2 Uniqueness Rule

On install callback for `installation_id=X`, scoped to requesting `organization_id=O`:

1. Look up existing active integration by `installation_id=X`.
2. If found and `organization_id != O` → reject with `"installation already connected to another organization"`.
3. If found and `organization_id == O` → **reuse** it, skip GitHub redirect, go straight to repo picker.
4. If not found → create new integration row for `O`.

### 6.3 Install Flow Changes

**Current flow**: Workspace → Install URL → GitHub → Callback → Create integration for workspace.

**New flow**:

```
Workspace "Connect GitHub" click
        │
        ▼
GET /api/git/github/install-url?workspace_id=W
        │
        ├─ Resolve workspace.organization_id = O
        ├─ Does O already have an active integration?
        │     YES → return { action: "pick_repos", integration_id }
        │     NO  → return { action: "install", install_url }
        ▼
(if install) GitHub → GET /api/git/github/callback?state=…&installation_id=X
        │
        ├─ Decode state → { workspace_id: W, organization_id: O }
        ├─ Apply uniqueness rule (§6.2)
        ├─ Upsert integration for O
        ▼
Redirect → /w/{slug}/settings/delivery?github_app=connected&integration_id=…
        │
        ▼
Frontend opens repo picker automatically; user selects repos
        │
        ▼
POST /api/git/integrations/{id}/repositories
        Body: { workspace_id: W, repo_ids: [...] }
        │
        ▼
Service creates git_repositories rows (workspace_id = W)
```

### 6.4 Webhook Routing

**Current**: webhook → `installation_id` → `git_integrations` → single workspace.

**New**:

```
webhook arrives
  → lookup git_integrations by installation_id (→ organization_id, webhook_secret)
  → verify HMAC signature
  → for each repo_id in payload:
       lookup git_repositories by (integration_id, github_repo_id)
       → delivers to git_repositories.workspace_id
  → if no matching row, drop event silently (install covers repo but no workspace wired it)
```

### 6.5 Agent Runner Changes

Blast radius is small (confirmed by code audit):

- Agents don't hold an integration id. They target a **repo** via `task.integration_id` / `repository.integration_id`.
- Only 3 call sites resolve the integration:
  - `temporalapp/activities.go` — `loadRunState()`
  - `temporalapp/activities.go` — `resolvePlanningRunInput()`
  - `service/git.go` — `GetIntegrationWithRepo()`

Change each site from:

```go
gitIntRepo.GetByID(ctx, run.WorkspaceID, target.IntegrationID)
```

to a new method:

```go
gitService.ResolveForAgentRun(ctx, run.WorkspaceID, target.RepositoryID) (Integration, Repo, error)
```

Contract:

1. Load the repo by id.
2. Verify `repo.workspace_id == run.workspace_id` (agents cannot cross-reach).
3. Load the integration by `repo.integration_id`.
4. Return both.

### 6.6 API Changes

| Endpoint | Change |
|---|---|
| `GET /api/git/github/install-url` | Returns `action: "install" \| "pick_repos"` plus `integration_id` when the org already has an active install |
| `GET /api/git/github/callback` | Uniqueness check moves to org level (§6.2) |
| `GET /api/git/integrations` | Scoped by `organization_id` (derived from workspace) — returns integrations available to this workspace's org |
| `POST /api/git/integrations/{id}/repositories` | **New** — wire selected repos to a workspace |
| `DELETE /api/git/integrations/{id}/repositories/{repo_id}` | **New** — unwire a repo from the workspace |
| `DELETE /api/git/integrations/{id}` | Now org-admin-only; removes integration org-wide and cascades `git_repositories` |

### 6.7 Frontend Changes

**`ProjectDeliveryTab.tsx`**

- On mount, call `install-url` endpoint.
- If `action === "pick_repos"`: render repo picker directly using integration's available repos from GitHub API (`GET /api/git/integrations/{id}/available-repos`).
- If `action === "install"`: render the existing "Connect GitHub" CTA.
- Add a "Disconnect repo from this workspace" action per repo (soft-delete the `git_repositories` row; does not uninstall).
- Surface org-wide uninstall only for org admins, with a warning that it affects all workspaces.

**Error toasts**

- "Already connected to a different organization" gets a distinct message and links to the integration owner's contact (if resolvable).

## 7. Permissions

- `integrations.connect` (new): required to add / remove the org-level integration. Owner/admin only.
- `integrations.link_repo` (new): required to wire a repo to a workspace. Admin/member of that workspace.
- Webhook events require no user permission — they route to the workspace that owns the repo.

## 8. Migration Plan

All via `dbmigrate` (new SQL files under `server/internal/dbmigrate/sql/`).

1. **`20260418_0001_git_integrations_add_organization_id.sql`**
   - Add nullable `organization_id` to `git_integrations`.
   - Backfill: `UPDATE git_integrations SET organization_id = (SELECT organization_id FROM workspaces WHERE id = git_integrations.workspace_id)`.
   - Set `NOT NULL` once backfilled.

2. **`20260418_0002_git_integrations_unique_installation.sql`**
   - Add partial unique index: `CREATE UNIQUE INDEX git_integrations_installation_unique ON git_integrations (provider, installation_id) WHERE active = true`.
   - If duplicates exist (they shouldn't under the old check), migration aborts loudly; requires manual reconciliation.

3. **`20260418_0003_git_repositories_create_or_extend.sql`**
   - If `git_repositories` exists already: ensure it has the columns above, add missing ones idempotently.
   - If not: create it.
   - Backfill: for each existing `git_integrations` row, if any tasks/stories in its workspace reference a repo, create a matching `git_repositories` row keyed to that workspace.

4. **`20260418_0004_git_integrations_drop_workspace_id.sql`** (deferred until Phase 3 of rollout — see §9)
   - `ALTER TABLE git_integrations DROP COLUMN workspace_id`.

## 9. Rollout

Three-phase to de-risk the data cutover:

**Phase 1 — additive (no behavior change):**
- Deploy migrations 1-3.
- Deploy service code that can read from both `workspace_id` (legacy) and `organization_id` (new).
- Install flow still workspace-scoped; new repo picker exists but is dark-launched.

**Phase 2 — cutover:**
- Flip the install uniqueness check to org level.
- Enable the "pick_repos" action when an org already has an integration.
- Webhook handler switches to repo-based routing. Fallback: if no `git_repositories` rows exist for an integration (legacy), route to the integration's original workspace.

**Phase 3 — cleanup (≥ 14 days after Phase 2):**
- Drop `workspace_id` from `git_integrations` (migration 4).
- Remove legacy code branches.

## 10. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Backfill misses a repo, webhook drops events | Phase 2 fallback to integration's original workspace until `git_repositories` is authoritative; log every fallback to catch gaps |
| Customer uninstalls GitHub App to "fix" a perceived duplicate | UI warns that uninstall is org-wide; confirmation dialog lists affected workspaces |
| Agent run references a repo the workspace no longer owns | `ResolveForAgentRun` rejects with a clear error; agent run fails gracefully, notifies owner |
| Cross-org collision on `installation_id` (rare — same GitHub account used by two Helpin orgs) | Explicit error + contact the existing owner; do not auto-transfer |
| Race between two simultaneous install callbacks | Partial unique index in migration 2 is the authoritative guard |

## 11. Testing Strategy

**Backend:**
- Unit tests on `upsertGitHubIntegration` covering: new install, same-org reuse, cross-org rejection, soft-deleted integration reactivation.
- Unit tests on `ResolveForAgentRun`: happy path, cross-workspace repo rejection, missing repo, inactive integration.
- Webhook routing tests: single-workspace, multi-workspace same-install, orphaned repo (dropped event).
- Migration tests: backfill correctness with fixtures representing single-workspace and multi-workspace orgs.

**Frontend:**
- `ProjectDeliveryTab` tests: install action vs pick_repos action rendering, error toast paths.

**Manual QA:**
- Reproduce the Contentpen / ContentStudio scenario in staging: connect Contentpen, attempt ContentStudio, confirm repo picker appears, confirm webhooks route correctly.

## 12. Success Metrics

- Zero "already connected to another workspace" errors in production logs 30 days post-Phase 2.
- ≥ 90% of multi-workspace orgs that connected GitHub pre-cutover retain an active integration post-cutover (retention via backfill correctness).
- No increase in webhook delivery failures (`webhook_dropped` counter) during or after rollout.

## 13. Open Questions

1. Should uninstall from GitHub (webhook `installation.deleted`) soft-delete the integration and all `git_repositories`, or hard-delete? Recommendation: soft-delete with 30-day grace period, then hard-delete on cron.
2. If Org A invites a user who was already an admin of Org B's integration, should we surface that? Out of scope for this PRD; track separately if needed.
3. Do we expose "which workspace is consuming this repo" in a read-only org admin view? Nice-to-have, not in scope.

---

## Appendix A: File-Level Change Inventory

**Backend:**
- `server/internal/model/git.go` — add `OrganizationID` to `GitIntegration`, new `GitRepository` model (if not present)
- `server/internal/repository/git.go` — add `GetByInstallationID` org-scoped variant, `CreateRepository`, `ListRepositoriesByWorkspace`, `GetRepositoryByGitHubID`
- `server/internal/service/git.go` — rewrite `upsertGitHubIntegration` (§6.2), add `ResolveForAgentRun`, add `WireRepositoryToWorkspace`
- `server/internal/handler/git.go` — `install-url` action switching, new repo-wiring endpoints
- `server/internal/router/router.go` — register new routes with `integrations.connect` / `integrations.link_repo` permissions
- `server/internal/authorization/permissions.go` — new permission constants
- `server/internal/webhook/github.go` (or equivalent) — repo-based routing
- `server/internal/temporalapp/activities.go` — switch 2 call sites to `ResolveForAgentRun`
- `server/internal/dbmigrate/sql/20260418_*` — 4 migration files

**Frontend:**
- `frontend/src/components/settings/ProjectDeliveryTab.tsx` — install vs pick_repos branching, repo picker UI
- `frontend/src/lib/services/gitService.ts` — new endpoints
- `frontend/src/hooks/queries/` — add `useAvailableRepos`, `useWireRepository`
- `frontend/src/lib/types.ts` — `GitRepository` interface

**Tests:**
- `server/internal/service/git_test.go`
- `server/internal/temporalapp/activities_test.go`
- `frontend/src/components/settings/ProjectDeliveryTab.test.tsx` (if tests exist for this component)
