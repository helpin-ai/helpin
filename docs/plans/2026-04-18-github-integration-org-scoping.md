# Organization-scoped GitHub integrations plan

**Status**: Historical proposal; superseded in part by shared repository catalogs
**Author**: Azhar
**Date**: 2026-04-18
**Target branch**: `feature/github-integration-org-scoping`
**Related**: Builds on the GitHub App install flow introduced in `internal/githubapp/` and `internal/service/git.go`.

---

This historical plan explains why GitHub installations moved to the organization boundary. Its original one-repository/one-workspace restriction and rollout checklist no longer describe the full implementation. Read the source review before using the original proposal for engineering work.

## Source review — 2026-09-18

- Organization ownership, repository lifecycle fields, installation reuse, repository wiring, and agent-run scope validation exist in `server/internal/model/git.go` and `server/internal/service/git.go`. `ResolveForAgentRun` rejects repositories outside the run workspace and inactive repositories or integrations.
- **A repository can now be enabled in multiple workspace catalogs.** Migration `202605210002_org_level_git_connections.sql` replaces the original organization-wide live-claim uniqueness with `(workspace_id, integration_id, external_id)`. The original cross-workspace conflict behavior, disabled sibling claims, and one-live-workspace invariant below are superseded.
- Repository webhooks resolve a list of active workspace mappings. The handler still has a legacy fallback to `integration.workspace_id` when there are no claims. The model retains this nullable legacy column; the proposed Phase 3 column removal is not complete in source.
- The additive migrations are dated `20260423`, not the proposed `20260418_000*` filenames. Use the checked-in migration sequence, including later changes, rather than executing the illustrative SQL below as an upgrade procedure.
- Installation deletion/removal and suspend/unsuspend handling exist. The 30-day tombstone cleanup is implemented in `server/internal/service/git_grace_cleanup.go` and started by the Temporal worker; it runs immediately and then every 24 hours. Failed repository deletions are logged and skipped. This is not evidence that a deployed worker has run cleanup or that every reinstall restores every historical mapping.
- Workspace teardown deletes that workspace's repository rows and deactivates organization integrations when no sibling workspaces remain. The available-repository access helper accepts an organization owner or an owner/admin of a visible workspace in the organization; the historical table's broad reference to any workspace member should not be read as the implemented permission rule.
- `ProjectDeliveryTab.tsx` now points to workspace repository settings instead of hosting the proposed install/picker flow. Organization-level integration routes and GitLab wiring also exist beyond this plan's original scope. Rollout dates, production metrics, migration execution, and manual QA outcomes were not verified in this source review.

## Original proposal

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

**`git_repositories`** (extended in place — **must not** be dropped or re-keyed)

This table already exists in production. Its `id` column is a durable foreign key referenced by:

- `pm_team_repo_defaults.repository_id` (team default delivery repo)
- `task_delivery_targets.repository_id` (per-task delivery target)
- `task_git_links.repository_id` (task → PR/branch links)

The migration **preserves every existing row in place** — same `id`, same `workspace_id`, same `integration_id`, same `external_id`. No rows are deleted, re-inserted, or rebuilt from `pm_tasks` / `task_delivery_targets`. Only `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` and index swaps run.

**Live schema today** (authoritative, from `server/internal/model/git.go`):

| Column | Type | Notes |
|---|---|---|
| `id` | uuid | PK, preserved |
| `workspace_id` | uuid | NOT NULL, kept |
| `integration_id` | uuid | NOT NULL, kept |
| `provider` | text | NOT NULL, kept |
| `external_id` | text | NOT NULL — GitHub's repo id stored as a string. **This is the column; earlier PRD drafts called it `github_id`, which does not exist.** |
| `full_name` | text | NOT NULL, `"owner/repo"` |
| `default_branch` | text | NOT NULL default `'main'` |
| `permissions` | jsonb | NOT NULL default `'{}'` |
| `private` | bool | NOT NULL default `true` |
| `archived` | bool | NOT NULL default `false` — mirrors GitHub's `archived` flag |
| `selected` | bool | NOT NULL default `true` — user-facing "wired for delivery" flag |
| `created_at`, `updated_at` | timestamptz | |

**Existing index to be REPLACED** (not additive): the current unique index `idx_git_repo_external` on `(workspace_id, integration_id, external_id)` scopes uniqueness per-workspace, which is exactly the behavior this PRD is removing. It must be dropped and replaced in migration 3. Before dropping, run the duplicate-check in §8.

**Columns added by this change:**

```
active          bool NOT NULL DEFAULT true   -- lifecycle flag, distinct from `selected`
                                             --   selected  = user intent (wired for delivery)
                                             --   archived  = GitHub reports repo archived
                                             --   active    = Helpin-side lifecycle (not uninstalled/removed)
deleted_at      timestamptz                  -- soft-delete tombstone (uninstall grace)
```

**Indexes after migration 3:**

- `UNIQUE (integration_id, external_id) WHERE deleted_at IS NULL` — **new**; one live workspace wiring per (install, repo), org-wide. Replaces the old `idx_git_repo_external`.
- `INDEX (integration_id, external_id)` — **new**; non-partial, for webhook lookup that must hit tombstoned rows for reactivation.
- `INDEX (workspace_id, active) WHERE deleted_at IS NULL` — **new**; list-by-workspace hot path.
- Existing `INDEX (full_name)` — kept.

One repo maps to **exactly one live workspace** within an organization. Tombstoned rows are retained so that dependent tables (`pm_team_repo_defaults`, `task_delivery_targets`, `task_git_links`) do not dangle during the uninstall grace window; they are only hard-deleted by the cron in §6.8.

**Test fixture alignment**: `server/internal/service/testdb_test.go` currently uses a legacy SQLite schema for `git_repositories` with the old columns `git_integration_id`, `repo_full_name`, `is_active` — this fixture is divergent from the production GORM schema. Migration 3 does not touch SQLite, but the fixture **must be updated in the same PR** to match the real columns (`integration_id`, `full_name`, plus the new `active` and `deleted_at`), otherwise the new service tests cannot compile. Listed as a line item in Appendix A.

### 6.2 Uniqueness Rule

On install callback for `installation_id=X`, scoped to requesting `organization_id=O`:

1. Look up existing integration by `(provider, installation_id=X)`, **including inactive/soft-deleted rows**.
2. If found, `active = true`, `organization_id != O` → reject with `"installation already connected to another organization"`.
3. If found, `active = true`, `organization_id == O` → **reuse** it, skip GitHub redirect, go straight to repo picker.
4. If found, `active = false`, `organization_id == O` → **reactivate in place** (set `active = true`, clear `deleted_at`, refresh `webhook_secret`/`app_id` from callback). Keep the same `id` so dependent `git_repositories` (also reactivated, see §6.8) remain valid.
5. If found, `active = false`, `organization_id != O` → treat as cross-org collision — reject with the same error as case 2. Do not silently transfer ownership between orgs.
6. If not found → create new integration row for `O`.

The partial unique index (`UNIQUE (provider, installation_id) WHERE active = true`) is the authoritative guard for cases 2–3 under concurrent callbacks. Cases 4–5 are serialized by a row-level lock on the matched inactive row inside the callback transaction.

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

**Current**: webhook → `installation_id` → `git_integrations` → single workspace. Non-repo lifecycle events (`installation`, `installation_repositories`) are ignored by `handler.git.go`.

**New — repo-scoped events** (push, pull_request, issues, check_suite, release, etc.):

```
webhook arrives
  → lookup git_integrations by installation_id (include inactive rows; reactivating flows need signature verify)
  → verify HMAC signature with integration.webhook_secret
  → reject if integration.active = false (204 + metric)
  → for each repo_id in payload:
       lookup git_repositories by (integration_id, external_id) WHERE deleted_at IS NULL
       → delivers to git_repositories.workspace_id
  → if no matching row, increment webhook_unwired counter and drop silently
    (install covers repo but no workspace wired it, OR tombstoned — both are expected)
```

**New — lifecycle events** (must be handled, not ignored):

| Event | Action | Action on repo rows |
|---|---|---|
| `installation.created` | No-op. Callback path handled the create. | — |
| `installation.deleted` | Soft-delete integration: `active = false`, `deleted_at = now()`. | Soft-delete **all** matching `git_repositories`: `active = false`, `deleted_at = now()`. Do not cascade to `pm_team_repo_defaults` / `task_delivery_targets` — those keep dangling pointers that surface as "delivery target unavailable" in UI. |
| `installation.suspend` | `active = false`, leave `deleted_at` null. Repos stay as-is but webhook routing rejects. | — |
| `installation.unsuspend` | `active = true` if `deleted_at` is null. | — |
| `installation_repositories.removed` | — | Soft-delete matched `git_repositories` (`active = false, deleted_at = now()`). |
| `installation_repositories.added` | — | Reactivate a matching tombstoned row for the same `(integration_id, external_id)` if one exists (clear `deleted_at`, `active = true`). Otherwise no-op — wiring requires an explicit workspace pick via the API (§6.6). |

Hard-deletion only happens via the §6.8 grace-window cron; the webhook handler itself never drops rows.

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

All endpoints are workspace-scoped (`/api/workspaces/{ws}/...`) except where noted; authorization resolves against `workspace.organization_id`.

| Endpoint | Auth | Change |
|---|---|---|
| `GET /api/git/github/install-url` | `integrations.connect` | Returns `action: "install" \| "pick_repos"` plus `integration_id` when the org already has an active install |
| `GET /api/git/github/callback` | GitHub-signed state | Uniqueness check moves to org level (§6.2); reactivates soft-deleted integration of the same org in place |
| `GET /api/git/integrations` | workspace member | Scoped by `organization_id` (derived from workspace) — returns integrations available to this workspace's org. Includes only `active = true` rows. |
| `GET /api/git/integrations/{id}/available-repos` | `integrations.enumerate_repos` (org admin **or** any admin/member of a workspace in that org — see below) | **New** — lists GitHub-visible repos for the installation, each annotated with `claimed_by` (`null`, or `{ workspace_id, workspace_name, repo_id }`) so UI can disable rows already wired elsewhere |
| `POST /api/git/integrations/{id}/repositories` | `integrations.link_repo` on the target workspace | **New** — wires selected repos to the target workspace. Transactional: inserts rows with `ON CONFLICT (integration_id, external_id) WHERE deleted_at IS NULL DO NOTHING`. If any requested repo is already claimed by another workspace in the org, returns `409 Conflict` with `{ conflicts: [{ external_id, claimed_by_workspace_id }] }` and no partial inserts. |
| `DELETE /api/git/integrations/{id}/repositories/{repo_id}` | `integrations.link_repo` on the owning workspace | **New** — soft-deletes the `git_repositories` row (`active = false, deleted_at = now()`). Does not uninstall. Repo becomes claimable **immediately** because the live uniqueness index only covers rows where `deleted_at IS NULL`; the 30-day grace only preserves the tombstone for reactivation/history (§6.8). |
| `DELETE /api/git/integrations/{id}` | `integrations.uninstall` (owner of the org) | Soft-deletes the integration and all its repos org-wide (§6.8). Response body lists affected workspaces so UI can confirm. |

**Authorization for `available-repos`**: the GitHub installation repo inventory is *shared* across the org, so the permission is org-scoped but permissive — any org member who holds `integrations.link_repo` on **any** workspace in that org may enumerate, since they can already claim repos there. Org owners always can. Non-org members cannot, even if they have a workspace role elsewhere.

**Concurrent-claim behavior**: two workspaces in the same org attempting to claim the same repo simultaneously are serialized by the partial unique index. The losing request receives `409 Conflict` with the winning workspace in the payload. The UI refetches `available-repos` and re-renders the disabled state.

### 6.7 Frontend Changes

**`ProjectDeliveryTab.tsx`**

- On mount, call `install-url` endpoint.
- If `action === "pick_repos"`: render repo picker using `GET /api/git/integrations/{id}/available-repos`.
- If `action === "install"`: render the existing "Connect GitHub" CTA.
- Repos where `claimed_by !== null` and `claimed_by.workspace_id !== currentWorkspaceId` render **disabled** with a subdued label `Claimed by {workspace_name}` — **hidden** is wrong because admins otherwise cannot see why the list feels short, and because seeing the claim enables "ask that workspace admin to release it".
- Repos where `claimed_by.workspace_id === currentWorkspaceId` render as already-selected.
- "Disconnect repo from this workspace" action per repo (soft-delete the `git_repositories` row; does not uninstall).
- "Uninstall GitHub App" is gated on `integrations.uninstall`. Confirmation modal fetches `GET /api/git/integrations/{id}` and lists every workspace in the org that has wired repos, so the operator sees the blast radius before confirming.

**Error toasts**

- `403` on `available-repos` → "You don't have access to this organization's integrations."
- `409` on `POST repositories` → "That repo was just claimed by *{workspace_name}*." Picker auto-refetches.
- "Already connected to a different organization" — distinct message, points to support if the operator thinks this is wrong.

### 6.8 Lifecycle & Teardown

Integration lifecycle must be decoupled from workspace lifecycle. Today `repository/workspace.go` hard-deletes both `git_repositories` and `git_integrations` by `workspace_id` in a workspace teardown — after this change, that would wipe the shared org integration when one workspace is deleted. This section specifies the rewritten teardown semantics.

**Workspace delete** (`WorkspaceRepository.Delete`, inside the existing transaction):

**Rule (single, explicit):**

- Always HARD-delete the deleting workspace's `git_repositories` rows. No soft-delete step, no grace window for repo rows.
- Do **not** touch `git_integrations` when at least one other workspace remains in the same org.
- If this delete removes the **last remaining workspace in the org**, immediately soft-delete the org's `git_integrations` rows (`active = false, deleted_at = now()`). This prevents an `active = true` integration from pointing at a deleted workspace during Phase 1/Phase 2 fallback behavior. A future install from a newly created workspace in the same org reactivates that integration in place via §6.2 case 4.

Workspace deletion is terminal (not undoable today), and all dependents (`pm_team_repo_defaults`, `task_delivery_targets`, `task_git_links`) for that workspace are already removed earlier in the same transaction, so hard-deleting the workspace's repo rows cannot dangle.

Replace the two current statements:

```sql
DELETE FROM git_repositories WHERE workspace_id = ?
DELETE FROM git_integrations WHERE workspace_id = ?
```

with:

```sql
-- workspace_id is a direct column on git_repositories (kept forever).
-- Dependent tables above were already deleted earlier in this tx.
-- Step 1: always release this workspace's repo claims immediately.
DELETE FROM git_repositories WHERE workspace_id = ?;

-- Step 2: only if this was the org's last workspace, deactivate the
-- org-scoped integrations so no active row points at a deleted workspace.
UPDATE git_integrations
   SET active = false,
       deleted_at = COALESCE(deleted_at, now()),
       updated_at = now()
 WHERE organization_id = ?
   AND NOT EXISTS (
         SELECT 1
           FROM workspaces
          WHERE organization_id = ?
            AND id <> ?
       );
```

This is deliberately different from uninstall (below), which soft-deletes the integration and repos org-wide with a 30-day grace window. The asymmetry is intentional:

| Action | `git_repositories` | `git_integrations` | Why |
|---|---|---|---|
| Workspace delete, other workspaces remain | Hard-delete (this workspace only) | Untouched | Terminal operation; sibling workspaces must keep working |
| Workspace delete, last workspace in org | Hard-delete (this workspace only) | Soft-delete org integration(s) | Prevent active integrations from pointing at a deleted workspace during staged rollout/fallback |
| Uninstall (API or `installation.deleted`) | Soft-delete (org-wide), 30-day grace | Soft-delete, 30-day grace | Reinstall is a common recovery path; grace enables transparent reattach |

**Uninstall (operator-driven via `DELETE /api/git/integrations/{id}` or webhook `installation.deleted`)**:

1. Mark integration `active = false`, `deleted_at = now()`.
2. Mark every `git_repositories` row for that integration `active = false`, `deleted_at = now()`.
3. Leave `pm_team_repo_defaults`, `task_delivery_targets`, and `task_git_links` untouched — they now point at tombstoned repo rows, which surface as "delivery target unavailable" in the task UI instead of disappearing silently.
4. Emit `git.integration.uninstalled` event so frontends can invalidate caches.

**Grace window + hard delete cron**:

- Soft-deleted rows (integration and repos) live for **30 days** after `deleted_at`.
- Reinstalling the App in GitHub within that window reactivates the integration row (§6.2 case 4) and the matching `installation_repositories.added` events (§6.4) reactivate the corresponding repo tombstones by `(integration_id, external_id)`, restoring per-workspace wiring transparently.
- After 30 days, a daily cron hard-deletes:
  - `git_repositories` rows with `deleted_at < now() - interval '30 days'`
  - dependent `task_delivery_targets`, `task_git_links`, `pm_team_repo_defaults` rows whose `repository_id` no longer exists (ON DELETE SET NULL semantics where the dependent table permits; otherwise DELETE)
  - `git_integrations` rows that are soft-deleted and have zero live or tombstoned repos

**FK integrity rule**: the partial unique index on `(integration_id, external_id) WHERE deleted_at IS NULL` allows multiple historical tombstones per repo. Dependent tables always hold the most-recent live row's `id`; reactivation preserves `id`, so dependent pointers remain valid. Re-wiring a repo after the grace window creates a new row with a new `id` — dependent tables will need to be re-pointed by the user (a new delivery target), which is consistent with re-claiming a repo after it was intentionally abandoned.

## 7. Permissions

New permissions (added to `server/internal/authorization/permissions.go` and the RBAC matrix in `rbac.go`):

- `integrations.connect` — initiate an install or repo-pick flow. **Granted to**: workspace owner, admin.
- `integrations.enumerate_repos` — list the org installation's full repo inventory via `available-repos`. **Granted to**: any user who holds `integrations.link_repo` on at least one workspace in the same org (checked dynamically in the handler, not in the static matrix). Org owners implicitly have it.
- `integrations.link_repo` — wire a repo to or unwire it from a workspace. **Granted to**: workspace owner, admin.
- `integrations.uninstall` — remove the org-wide integration (destructive). **Granted to**: org owner only. **Not** workspace owner — a workspace owner must not be able to unilaterally break other workspaces in the same org.

Webhook events require no user permission — they route to the workspace that owns the repo via `git_repositories`.

Removed: the old per-workspace integration-delete permission collapses into `integrations.uninstall` at the org level. Phase-1 code keeps both permission checks; Phase-3 cleanup removes the legacy one.

## 8. Migration Plan

All via `dbmigrate` (new SQL files under `server/internal/dbmigrate/sql/`). Every statement is idempotent.

1. **`20260418_0001_git_integrations_add_organization_id.sql`**
   - `ALTER TABLE git_integrations ADD COLUMN IF NOT EXISTS organization_id uuid`.
   - Backfill: `UPDATE git_integrations SET organization_id = (SELECT organization_id FROM workspaces WHERE id = git_integrations.workspace_id) WHERE organization_id IS NULL`.
   - Abort loudly (raise exception) if any row has a null `organization_id` after backfill — means a workspace is missing or has no org.
   - `ALTER TABLE git_integrations ALTER COLUMN organization_id SET NOT NULL`.
   - `ALTER TABLE git_integrations ADD CONSTRAINT git_integrations_organization_id_fk FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT`.
   - `CREATE INDEX IF NOT EXISTS git_integrations_org_provider_idx ON git_integrations (organization_id, provider)`.

2. **`20260418_0002_git_integrations_unique_installation.sql`**
   - `CREATE UNIQUE INDEX IF NOT EXISTS git_integrations_installation_unique ON git_integrations (provider, installation_id) WHERE active = true`.
   - Pre-check: `SELECT provider, installation_id, count(*) FROM git_integrations WHERE active = true GROUP BY 1,2 HAVING count(*) > 1` — if any, raise and require manual reconciliation before the index creation runs.

3. **`20260418_0003_git_repositories_extend_in_place.sql`** (CRITICAL — see §6.1)
   - **Row-preserving.** This migration must not drop, truncate, or re-insert any row. `git_repositories.id` is a durable FK referenced by `pm_team_repo_defaults.repository_id`, `task_delivery_targets.repository_id`, and `task_git_links.repository_id`.
   - **No row-level backfill.** Existing rows already carry `workspace_id`, `integration_id`, and `external_id`. Do not synthesize rows from `pm_tasks` or `task_delivery_targets`.

   Statements (all idempotent, in order):

   ```sql
   -- a) Additive columns.
   ALTER TABLE git_repositories
     ADD COLUMN IF NOT EXISTS active     boolean NOT NULL DEFAULT true,
     ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

   -- b) Pre-check. The existing `idx_git_repo_external` is UNIQUE on
   --    (workspace_id, integration_id, external_id) — per-workspace scoping.
   --    The new model requires org-wide uniqueness on (integration_id, external_id).
   --    If any (integration_id, external_id) pair currently has rows in two or
   --    more workspaces of the same org, we cannot safely flip the index:
   --    fail the migration loudly so the data can be reconciled by a human.
   DO $$
   DECLARE dupes int;
   BEGIN
     SELECT COUNT(*) INTO dupes FROM (
       SELECT integration_id, external_id
         FROM git_repositories
        GROUP BY 1,2
       HAVING COUNT(*) > 1
     ) q;
     IF dupes > 0 THEN
       RAISE EXCEPTION
         'git_repositories has % (integration_id, external_id) duplicate groups; reconcile before org-scoping migration', dupes;
     END IF;
   END $$;

   -- c) Swap index: drop the workspace-scoped uniqueness, add org-scoped partial uniqueness.
   DROP INDEX IF EXISTS idx_git_repo_external;
   CREATE UNIQUE INDEX IF NOT EXISTS git_repositories_integration_external_live_uidx
     ON git_repositories (integration_id, external_id)
     WHERE deleted_at IS NULL;

   -- d) Non-partial lookup index for webhook routing (must see tombstoned rows
   --    so installation_repositories.added can reactivate them by matching id).
   CREATE INDEX IF NOT EXISTS git_repositories_integration_external_idx
     ON git_repositories (integration_id, external_id);

   -- e) Workspace-list hot path.
   CREATE INDEX IF NOT EXISTS git_repositories_workspace_active_idx
     ON git_repositories (workspace_id, active)
     WHERE deleted_at IS NULL;
   ```

   Post-migration assertion (run in the migration's `up` tail, or in `cmd/migrate validate`): count and MD5-of-sorted-ids of `git_repositories` must equal the pre-migration snapshot — any divergence is a bug in this migration.

4. **`20260418_0004_git_integrations_soft_delete_column.sql`**
   - `ALTER TABLE git_integrations ADD COLUMN IF NOT EXISTS deleted_at timestamptz`.
   - Needed for the uninstall grace window (§6.8).

5. **`20260418_0005_git_dependent_tables_fk_set_null.sql`**
   - Change FK behavior on dependent tables so the grace-window cron can hard-delete `git_repositories` without violating integrity:
     - `task_delivery_targets.repository_id`: ON DELETE SET NULL
     - `task_git_links.repository_id`: ON DELETE SET NULL
     - `pm_team_repo_defaults.repository_id`: ON DELETE RESTRICT (forces admin to pick a new default before the repo finally hard-deletes; exposed in UI as a "repository removed" warning).

6. **`20260418_0006_git_integrations_drop_workspace_id.sql`** (deferred until Phase 3 of rollout — see §9)
   - `ALTER TABLE git_integrations DROP COLUMN IF EXISTS workspace_id`.

## 9. Rollout

Three-phase to de-risk the data cutover:

**Phase 1 — additive (no behavior change):**
- Deploy migrations 1–5.
- Deploy service code that can read from both `workspace_id` (legacy) and `organization_id` (new).
- Install flow still workspace-scoped; new repo picker exists but is dark-launched.
- **Rewrite `WorkspaceRepository.Delete`** in this phase (§6.8) — replace the old per-workspace git cleanup with: (1) `DELETE FROM git_repositories WHERE workspace_id = ?`; (2) if the org has no remaining workspaces after this delete, soft-delete that org's `git_integrations` rows (`active = false, deleted_at = now()`). This keeps sibling-workspace scenarios working while preventing Phase-1/Phase-2 fallback from resolving to a deleted workspace in the single-workspace-org case. Add unit tests for both branches: multi-workspace org leaves the integration live; last-workspace delete deactivates the integration in place.

**Phase 2 — cutover:**
- Flip the install uniqueness check to org level.
- Enable the "pick_repos" action when an org already has an integration.
- Webhook handler switches to repo-based routing. Lifecycle event handling (§6.4) goes live.
- Fallback: if no `git_repositories` rows exist for an integration (legacy/untouched workspace), route to the integration's original workspace (`integration.workspace_id`) — this is why migration 6 is deferred.
- Frontend enables the picker + disconnect actions.
- Grace-window cron deployed but idle (no soft-deleted rows are older than 30 days yet).

**Phase 3 — cleanup (≥ 30 days after Phase 2, not 14 — first grace cohort must have aged out):**
- Drop `workspace_id` from `git_integrations` (migration 6).
- Remove the legacy webhook fallback branch.
- Remove the legacy per-workspace integration-delete permission check.

## 10. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Workspace delete wipes the shared org integration for sibling workspaces | §6.8 rewrite of `WorkspaceRepository.Delete`; Phase-1 test asserts integration survives workspace delete in a multi-workspace org |
| Last-workspace delete leaves an active integration pointing at a deleted workspace during staged fallback | §6.8 special-case: if the org has zero remaining workspaces, soft-delete the org integration(s) in place |
| Migration 3 accidentally re-keys `git_repositories.id`, breaking team defaults / delivery targets / task links | Migration is purely additive (`ADD COLUMN IF NOT EXISTS` only), no backfill from `pm_tasks`. Pre-migration assertion counts live rows; post-migration re-asserts same count and same id set |
| Reinstall within grace window fails to reattach | `installation.created` callback + `installation_repositories.added` webhook both call reactivation-by-`(integration_id, external_id)`; integration test covers uninstall → reinstall cycle |
| Reinstall after grace window leaves dangling delivery targets | `task_delivery_targets.repository_id ON DELETE SET NULL` (migration 5) — target surfaces as "unconfigured" and prompts re-pick. `pm_team_repo_defaults` is RESTRICT: hard delete blocks until team admin picks a new default |
| Webhook drops events for repos that were live in Phase 1 but not yet wired in Phase 2 | Fallback to integration's original workspace during Phase 2; `webhook_unwired` counter + log catches gaps for manual follow-up |
| Customer uninstalls GitHub App to "fix" a perceived duplicate | UI warns that uninstall is org-wide; confirmation dialog lists affected workspaces; soft-delete + 30-day grace means operator can undo by reinstalling |
| Agent run references a repo the workspace no longer owns | `ResolveForAgentRun` verifies `repo.workspace_id == run.workspace_id`; tombstoned repos reject with a clear error |
| Cross-org collision on `installation_id` (rare — same GitHub account used by two Helpin orgs) | Explicit error, including the inactive-row case (§6.2 case 5); do not auto-transfer |
| Race between two simultaneous install callbacks | Partial unique index in migration 2 is the authoritative guard |
| Two workspaces in same org race to claim the same repo | Partial unique index on `git_repositories (integration_id, external_id) WHERE deleted_at IS NULL`; loser gets 409 with `claimed_by_workspace_id` in payload |
| `available-repos` leaks repo names to users who shouldn't see them | Handler enforces "member holds `integrations.link_repo` on at least one workspace in this org" before calling the GitHub API — org outsiders get 403 |
| PRD-schema drift — PRD text names a column (`github_id`) that does not exist; migrations written against PRD would fail | PRD now pinned to live schema (`external_id`, `full_name`, `integration_id`); §6.1 lists every current column; migration 3 references `idx_git_repo_external` by its real name |
| Legacy SQLite test fixture (`testdb_test.go`) names columns differently (`git_integration_id`, `repo_full_name`, `is_active`) and diverges from production | Appendix A includes an explicit fixture-realignment line item; PR must update SQLite schema to match the production GORM column names before new service tests can compile |

## 11. Testing Strategy

**Backend:**
- Unit tests on `upsertGitHubIntegration` covering: new install, same-org reuse, cross-org rejection, inactive-same-org reactivation, inactive-cross-org rejection.
- Unit tests on `ResolveForAgentRun`: happy path, cross-workspace repo rejection, missing repo, inactive integration, tombstoned repo.
- Unit tests on `WireRepositoryToWorkspace`: success, already-claimed-by-self (idempotent), already-claimed-by-sibling-workspace (409 with `claimed_by_workspace_id`), integration inactive (reject).
- Webhook routing tests: single-workspace, multi-workspace same-install, orphaned repo (counter increments + drop), `installation.suspend` (rejects subsequent repo events), `installation.deleted` (soft-deletes repos), `installation_repositories.added` reactivates tombstoned row, `installation_repositories.removed` soft-deletes.
- **Workspace delete tests** (new — covers finding #2; uses the single rule from §6.8):
  - Two-workspace org, both wire repos from the same integration; delete workspace A; assert integration row still `active = true`, workspace-B repos still live (unchanged), workspace-A `git_repositories` rows are **hard-deleted** (`SELECT COUNT(*) ... WHERE workspace_id = A = 0`), and workspace-A's `task_delivery_targets` / `task_git_links` / `pm_team_repo_defaults` are all gone via the existing per-workspace cascade.
  - Single-workspace org; delete workspace; assert `git_repositories` rows are hard-deleted and the org's `git_integrations` row is retained **but soft-deactivated** (`active = false`, `deleted_at IS NOT NULL`). Creating a new workspace in the same org and reinstalling/reactivating should reuse the same integration row (§6.2 case 4).
  - After workspace-A delete, assert workspace B can immediately claim one of the repos A previously held — partial unique index must not block because the losing row is hard-deleted, not tombstoned.
- **Migration tests** (new — covers finding #1):
  - Snapshot `git_repositories.id` set and `(integration_id, external_id)` set before migration 3, assert identical sets after.
  - Duplicate pre-check: insert two rows with identical `(integration_id, external_id)` across two workspaces of the same org, run migration 3, assert it raises and the DB is unchanged (no index swap happens).
  - Happy path: seed rows with referenced repos in `pm_team_repo_defaults`, `task_delivery_targets`, `task_git_links`; run migration 3; assert every FK still resolves and `idx_git_repo_external` is gone, `git_repositories_integration_external_live_uidx` exists.
  - Grace-window cron test: tombstone a repo referenced by a `pm_team_repo_defaults` row, advance clock, run cron, assert RESTRICT prevents delete and returns a structured error.
- Lifecycle cycle test: install → wire repo → uninstall (webhook) → reinstall (callback) → `installation_repositories.added` → assert original `git_repositories.id` is revived, delivery targets still valid.

**Frontend:**
- `ProjectDeliveryTab` tests: install action vs pick_repos action rendering, claimed-by-other-workspace disabled state, 409 on concurrent claim triggers refetch, uninstall confirmation modal lists all affected workspaces.

**Manual QA:**
- Reproduce the Contentpen / ContentStudio scenario in staging: connect Contentpen, attempt ContentStudio, confirm repo picker appears, confirm webhooks route correctly.
- Uninstall ContentStudio's repo from its workspace settings; confirm Contentpen's repo still receives webhooks.
- Delete a staging workspace that has wired repos; confirm its sibling workspace in the same org still works and still receives webhooks for its repos.

## 12. Success Metrics

- Zero "already connected to another workspace" errors in production logs 30 days post-Phase 2.
- ≥ 90% of multi-workspace orgs that connected GitHub pre-cutover retain an active integration post-cutover (retention via backfill correctness).
- No increase in webhook delivery failures (`webhook_dropped` counter) during or after rollout.

## 13. Open Questions

1. ~~Should uninstall from GitHub (webhook `installation.deleted`) soft-delete or hard-delete?~~ **Resolved in §6.8**: soft-delete integration + repos, 30-day grace, cron hard-delete. Reinstall within grace reactivates in place (§6.2 case 4 + §6.4 `installation_repositories.added`).
2. If Org A invites a user who was already an admin of Org B's integration, should we surface that? Out of scope for this PRD; track separately if needed.
3. Do we expose "which workspace is consuming this repo" in a read-only org admin view? The `available-repos` endpoint already returns `claimed_by.workspace_name` for admins who can wire repos; a dedicated read-only org-admin view is a nice-to-have and is not in scope for this PRD.
4. ~~Should deleting a workspace with wired repos prompt the user to re-claim those repos in another workspace before hard-deleting?~~ **Resolved in §6.8**: workspace delete HARD-deletes only this workspace's `git_repositories` rows (no grace window, no soft-delete for repo rows). If sibling workspaces remain, `git_integrations` is untouched; if this was the last workspace in the org, `git_integrations` is soft-deactivated to avoid stale fallback resolution. Released repos become immediately claimable from any other workspace in the same org. An optional pre-delete UX prompt ("you're about to release N repos — continue?") is nice-to-have and can be added later without a data-model change.

---

## Appendix A: File-Level Change Inventory

**Backend:**
- `server/internal/model/git.go` — add `OrganizationID`, `DeletedAt` to `GitIntegration`; add `Active`, `DeletedAt` to existing `GitRepository` struct. Existing `ExternalID`, `FullName`, `IntegrationID`, `Selected`, `Archived`, `Permissions` fields are kept as-is — **do not rename** (these are live in production and referenced across the repository, service, and handler layers). The PRD's "repo id" is `external_id` throughout.
- `server/internal/repository/git.go` — org-scoped `GetByInstallationID` (includes inactive rows for the reactivation path in §6.2), `UpsertRepository` (idempotent on `(integration_id, external_id) WHERE deleted_at IS NULL`), `HardDeleteRepositoriesByWorkspace` (used by workspace teardown per §6.8), `SoftDeleteRepositoriesByIntegration` (used by uninstall / `installation.deleted`), `SoftDeleteIntegrationsByOrganizationIfNoWorkspacesRemain` (used by last-workspace teardown per §6.8), `ReactivateRepositoryByExternalID` (used by callback and `installation_repositories.added`), `ListRepositoriesByWorkspace`, `GetRepositoryByExternalID`, `GetClaimedByByExternalID` (for `available-repos` `claimed_by` annotation)
- `server/internal/repository/workspace.go` — **rewrite `Delete`** per §6.8: replace the old `DELETE FROM git_repositories WHERE workspace_id = ?` / `DELETE FROM git_integrations WHERE workspace_id = ?` pair with (1) `DELETE FROM git_repositories WHERE workspace_id = ?`; (2) conditional soft-deactivation of `git_integrations` when this delete removes the org's last workspace. Multi-workspace teardowns leave the integration untouched; last-workspace teardowns set `active = false`, `deleted_at = now()`.
- `server/internal/service/git.go` — rewrite `upsertGitHubIntegration` (§6.2, including inactive reactivation path); add `ResolveForAgentRun`, `WireRepositoryToWorkspace` (transactional, 409-aware), `UnwireRepository`, `UninstallIntegration`, `ListAvailableRepos` (GitHub API + claim annotation), `HandleInstallationLifecycleEvent`
- `server/internal/handler/git.go` — `install-url` action switching, new repo-wiring endpoints, new `available-repos` endpoint, lifecycle webhook branches (currently ignored at `handler/git.go:306`)
- `server/internal/router/router.go` — register routes with `integrations.connect`, `integrations.enumerate_repos`, `integrations.link_repo`, `integrations.uninstall`
- `server/internal/authorization/permissions.go` + `rbac.go` — four new permission constants + role matrix entries; runtime helper `HasLinkRepoOnAnyWorkspaceInOrg(actor, orgID)` for `enumerate_repos`
- `server/internal/temporalapp/activities.go` — switch `loadRunState` and `resolvePlanningRunInput` call sites to `ResolveForAgentRun`
- `server/internal/dbmigrate/sql/20260418_*` — 6 migration files (see §8)
- `server/internal/worker/` — new `git_grace_cleanup.go` cron (§6.8)

**Frontend:**
- `frontend/src/components/settings/ProjectDeliveryTab.tsx` — install vs pick_repos branching, repo picker with disabled/claimed-by rows, uninstall confirmation modal with affected-workspaces list
- `frontend/src/lib/services/gitService.ts` — new endpoints: `installUrl`, `listAvailableRepos`, `wireRepositories`, `unwireRepository`, `uninstallIntegration`
- `frontend/src/hooks/queries/` — add `useAvailableRepos`, `useWireRepository`, `useUninstallIntegration`
- `frontend/src/lib/types.ts` — `GitRepository` with `active`, `deleted_at`; `AvailableRepo` with `claimed_by`; `WireRepoConflictError`
- `frontend/src/lib/queryKeys.ts` — `availableRepos(integrationId)`, `orgIntegrations(orgId)`

**Tests:**
- `server/internal/service/git_test.go` — new install/reactivation/wire/unwire/uninstall cases
- `server/internal/service/testdb_test.go` — **realign the SQLite fixture for `git_repositories`** to the production GORM column names: rename `git_integration_id` → `integration_id`, `repo_full_name` → `full_name`, remove `is_active`, add `external_id`, `selected`, `archived`, `permissions` (TEXT in SQLite), `active`, `deleted_at`. Drop the unused `repo_name`/`repo_owner` columns (the production schema does not have them). Without this, new service tests that reference the real column names won't compile.
- `server/internal/repository/workspace_delete_test.go` — multi-workspace org teardown (new file or new test in existing)
- `server/internal/dbmigrate/migrations_test.go` — migration 3 id-stability + duplicate pre-check + index swap assertions
- `server/internal/temporalapp/activities_test.go` — ResolveForAgentRun contract
- `frontend/src/components/settings/ProjectDeliveryTab.test.tsx`
