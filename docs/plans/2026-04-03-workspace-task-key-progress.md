# Workspace Task Key Progress Tracker

> **Tracking rule:** update this file as work lands. Keep statuses limited to `Pending`, `In Progress`, `Blocked`, or `Done`.

## Overall Status

| Workstream | Status | Notes |
| --- | --- | --- |
| Schema and migrations | Done | Migration `202604030001_add_workspace_key.sql`: workspace_key column + backfill + workspace_key_history table |
| Backend workspace key | Done | WorkspaceKey on Workspace struct, CreateWorkspaceRequest, UpdateWorkspaceRequest, WorkspaceKeyHistory model |
| Key alias infrastructure | Done | Merged into Phase 1 — FindWorkspaceByKeyOrAlias, InsertKeyHistory, IsWorkspaceKeyAvailable, UpdateWorkspaceKey |
| Backend task key computation | Done | FormatTaskKey() helper, TaskKey gorm:"-" on PMTask/TaskDependencyTask/SearchResult, central population in PMTaskService |
| Workspace key caching | Done | getWorkspaceKey() on PMTaskService fetches once per workspace per request |
| Frontend workspace key | Done | Workspace interface, creation form with auto-suggest, GeneralTab editable field |
| Frontend PM UI rollout | Done | TaskDetailPanel, TaskSidebarIdRow, TaskListView, TaskCard, MyWork, TaskRelationshipsSection, TaskDeliveryPanel |
| Branch template standardization | Done | {task_key}/{workspace_key}/{task_type} tokens in temporal activities, default changed to {task_key}-{slug} across model/repo/frontend |
| Search and URL resolution | Done | Backend regex parser for HLP-123, frontend parseTaskKey in GlobalTaskPanel, SearchCommandPalette badges, pmTaskLinks with taskKey |
| Cross-product integration | Done | Notification snapshots emit task_key format, emails render it automatically, WebSocket re-fetch includes task_key |
| Backward compatibility | Done | Numeric ?task=123 still works, display_id still accepted in API filters, CRM prefixes (C-/CO-/D-) unaffected |
| Optional full-page task route | Pending | `/pm/tasks/HLP-123` for external sharing (Phase 7+ additive, not blocking) |

## Checklist

### Phase 1: Foundation — workspace key + backend plumbing + alias infrastructure

- [ ] Add migration `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_workspace_key.sql`
  - Add `workspace_key VARCHAR(5)` column to `workspaces` (nullable for backfill)
  - Backfill existing workspaces using alpha-only strategy (batched with `LIMIT 1000` per iteration to avoid blocking startup):
    - Extract alpha chars from slug, uppercase
    - Try bare substrings: 3, 4, 5 chars (e.g. `HEL`, `HELP`, `HELPI`)
    - If all collide: try base (2-4 chars) + alpha suffix A-Z within 5-char limit
    - All candidates satisfy `^[A-Z]{2,5}$` — no numeric suffixes, no length overflow
  - Set `NOT NULL` constraint after backfill
  - Add unique index `idx_workspaces_workspace_key`
  - Add `workspace_key_history` table (same migration file):
    - Columns: `id` (UUID PK), `workspace_id` (FK → workspaces), `old_key` (VARCHAR(5)), `new_key` (VARCHAR(5)), `changed_at` (TIMESTAMPTZ), `changed_by` (UUID FK → users)
    - Index: `idx_wkh_workspace_id` on `workspace_id`
    - Unique index: `idx_wkh_old_key` on `old_key` (prevents any workspace from reusing a retired key)
- [ ] Add `WorkspaceKey string` field to `Workspace` struct in `server/internal/model/workspace.go`
  - GORM tag: `gorm:"type:varchar(5);uniqueIndex;not null"`
  - JSON tag: `json:"workspace_key"`
- [ ] Add `WorkspaceKey string` to `CreateWorkspaceRequest` in `server/internal/model/workspace.go`
  - Required field, validated server-side
- [ ] Add `WorkspaceKey *string` to `UpdateWorkspaceRequest` in `server/internal/model/workspace.go`
  - Optional field — triggers alias flow when changed (admin/owner only)
- [ ] Add `WorkspaceKeyHistory` model to `server/internal/model/workspace.go`
  - GORM struct with `TableName() = "workspace_key_history"`
- [ ] Add `workspace_key` format validation in `server/internal/service/workspace.go`
  - Regex: `^[A-Z]{2,5}$`
  - Uniqueness check across both `workspaces.workspace_key` and `workspace_key_history.old_key`
  - Reject empty or malformed keys on create
- [ ] Pass `workspaceKey` through `Create()` in `server/internal/repository/workspace.go`
- [ ] Add `InsertKeyHistory(ctx, workspaceID, oldKey, newKey, changedBy) error` to `server/internal/repository/workspace.go`
- [ ] Add `FindWorkspaceByKeyOrAlias(ctx, key) (*Workspace, error)` to `server/internal/repository/workspace.go`
  - Check `workspaces.workspace_key` first
  - Fall back to `workspace_key_history.old_key` → join to `workspaces` by `workspace_id`
  - Return the workspace with its current key
- [ ] Add cross-table uniqueness check to repository
  - On create/update: new key must not exist in `workspaces.workspace_key` (other than self) OR `workspace_key_history.old_key`
- [ ] Update workspace key change flow in `server/internal/service/workspace.go`
  - Validate new key format and cross-table uniqueness
  - In a single transaction: insert old key into `workspace_key_history`, update `workspaces.workspace_key`
  - Require admin/owner permission
- [ ] Add `FormatTaskKey(workspaceKey string, displayID int) string` helper
  - Location: `server/internal/service/pm_task.go` or a shared `internal/format/` package
  - Returns `fmt.Sprintf("%s-%d", workspaceKey, displayID)`
- [ ] Add `TaskKey string` field with `json:"task_key" gorm:"-"` to `PMTask` struct in `server/internal/model/pm_task.go`
  - `gorm:"-"` ensures GORM ignores it for storage
  - JSON serialization includes it in every response that embeds `PMTask` (`TaskDetail`, `BoardTask`, list payloads)
- [ ] Add `TaskKey string` field with `json:"task_key" gorm:"-"` to `TaskDependencyTask` in `server/internal/model/pm_task.go`
- [ ] Add `TaskKey string` field with `json:"task_key,omitempty" gorm:"-"` to `SearchResult` in `server/internal/model/search.go`
- [ ] Update `PMTaskService` to populate `task_key` centrally on all task responses
  - Single population point: after loading tasks, before returning response
  - Cache workspace key per request/workspace (avoid N+1 queries)
  - Applies to: single task detail, task lists, board tasks, sprint task lists, epic task lists, dependency tasks, search results
- [ ] Workspace key caching strategy
  - Inject `workspace_key` into request context from auth/workspace middleware, or batch-fetch once per request
  - Verify no N+1 queries on large task lists (200+ tasks)

### Phase 2: Frontend workspace key setup

- [ ] Add `workspace_key: string` to `Workspace` interface in `frontend/src/lib/types.ts`
- [ ] Add `workspace_key` to `workspacesService.create()` payload in `frontend/src/lib/services/workspacesService.ts`
- [ ] Add workspace key input to creation form in `frontend/src/pages/Workspaces.tsx`
  - Auto-suggest from workspace name (first 3 chars, uppercased, alpha-only)
  - Allow manual override
  - Validate format: 2-5 uppercase letters
  - Show preview: "Your task IDs will look like: HLP-1, HLP-2, ..."
- [ ] Add editable workspace key field to `frontend/src/components/settings/GeneralTab.tsx`
  - Editable by admin/owner only, read-only for other roles
  - On change: confirmation dialog explaining impact
    - "Changing from HLP to NCO. Existing references like HLP-123 in branches, bookmarks, and docs will continue to work. New task keys will use NCO-123."
  - Show current key and history of past keys (if any) as informational badges
  - Validate format client-side before submit

### Phase 3: PM UI rollout

- [ ] Create `frontend/src/lib/taskKeyUtils.ts` with helpers:
  - `formatTaskKey(workspaceKey: string, displayId: number): string` — returns `"HLP-123"`
  - `parseTaskKey(input: string): { workspaceKey: string; displayId: number } | null` — parses `"HLP-123"` format, returns null if not valid
- [ ] Add `task_key: string` to `Task` interface in `frontend/src/lib/pm-types/project.ts`
- [ ] Add `task_key: string` to `TaskDependencyTask` interface in `frontend/src/lib/pm-types/project.ts`
- [ ] Update `frontend/src/components/pm/TaskDetailPanel.tsx`
  - Replace `displayId` text in header (line ~965) with `task.task_key`
- [ ] Update `frontend/src/components/pm/TaskSidebarIdRow.tsx`
  - Replace display text (line ~52) from `{displayId}` to `{taskKey}`
  - Update copy-to-clipboard (line ~60) to copy `task_key` string
  - Update `buildGitBranch()` (line ~33-38) to use `task_key` instead of `displayId`
- [ ] Update `frontend/src/components/pm/TaskListView.tsx`
  - Replace `display_id` column accessor (line ~458) with `task_key`
  - Update column header label
- [ ] Update `frontend/src/components/pm/TaskCard.tsx`
  - Replace blocked-by text (line ~193) from `display_id` to `task_key`
- [ ] Update `frontend/src/pages/pm/MyWork.tsx`
  - Replace `task.display_id` (line ~387) with `task.task_key`
- [ ] Update `frontend/src/components/pm/TaskRelationshipsSection.tsx`
  - Replace relationship badges (lines ~481, ~583) to show `task_key`
- [ ] Update `frontend/src/components/pm/AssociationsPanel.tsx`
  - Replace linked task display (line ~303) with `task_key`
- [ ] Update `frontend/src/components/pm/TaskDeliveryPanel.tsx`
  - Update `buildBranchPreview()` (line ~621) to use `task_key` instead of raw `displayId`
  - Update branch preview display (line ~129) to reference `task_key`-based preview

### Phase 4: Branch template standardization

- [ ] Add new token resolution in `server/internal/service/git.go`
  - `{task_key}` → `HLP-123`
  - `{workspace_key}` → `HLP`
  - `{task_type}` → `feature`, `bug`, `chore`
  - Keep `{display_id}` and `{slug}` working (backward compat)
  - No `{team_code}` in v1 (requires immutable team code schema change — deferred to v2)
- [ ] Update model default in `server/internal/model/git.go` (line ~49)
  - From: `BranchTemplate: "tp-{display_id}-{slug}"`
  - To: `BranchTemplate: "{task_key}-{slug}"`
- [ ] Update hardcoded default in `server/internal/repository/settings.go` (line ~451)
  - From: `BranchTemplate: "{display_id}-{slug}"`
  - To: `BranchTemplate: "{task_key}-{slug}"`
  - Note: this currently differs from the model default — both must converge on the same value
- [ ] Update `frontend/src/components/settings/teams/TeamRepoDefaultForm.tsx`
  - Update token help text to document available v1 tokens: `{task_key}`, `{workspace_key}`, `{display_id}`, `{slug}`, `{task_type}`
  - Update placeholder/preview to show new default: `{task_key}-{slug}`
  - Do NOT document `{team_code}` — not available in v1
- [ ] Consider data migration for existing templates using `tp-{display_id}-{slug}`
  - Optional: rewrite to `{task_key}-{slug}` in a SQL migration
  - Or: leave existing templates as-is (they still resolve correctly via backward-compat `{display_id}` token)

### Phase 5: Search and URL resolution

- [ ] Backend search parser in search handler
  - Detect `^[A-Z]{2,5}-\d+$` pattern in search query
  - If matched: extract workspace key prefix and display_id suffix
  - Use `FindWorkspaceByKeyOrAlias()` to resolve workspace (handles old keys after key change)
  - Query by `workspace_id + display_id`
  - False positive mitigation: when prefix matches current workspace key or known alias, treat as exact task key lookup; otherwise boost in text search
  - If not matched: existing text search behavior unchanged
- [ ] Populate `task_key` on `SearchResult` in search service layer
- [ ] Frontend URL parser for `?task=` param
  - Accept both numeric (`?task=123`) and key format (`?task=HLP-123`)
  - Numeric: parse as display_id directly (existing behavior)
  - Key format: extract display_id from suffix after last `-`, optionally validate workspace key prefix
  - Update `TaskDetailPanel.tsx:564` to write `?task=HLP-123` format
  - Update `pmTaskLinks.ts:35` to write `?task=HLP-123` format
- [ ] Update `frontend/src/components/search/SearchCommandPalette.tsx`
  - Parse `HLP-123` input format in search
  - Update result badge (line ~176) from `#{item.display_id}` to `{item.task_key}`
  - Update navigation link (line ~109) to use `task_key` format in URL
- [ ] Update `frontend/src/lib/services/searchService.ts`
  - Add `task_key: string` to search result type interface

### Phase 6: Cross-product integration

- [ ] Update `server/internal/email/template.go`
  - `TaskBlockHTML()` to accept and display `task_key` instead of raw `displayID`
- [ ] Update `server/internal/service/notification.go`
  - Include `task_key` in notification event payloads
  - Update `immediateEmailFooterText()` to use `task_key`
- [ ] Update websocket events to include `task_key` in task entity payloads
  - Task create/update/delete events must carry `task_key` so `useRealtimeSync` can update TanStack Query cache without refetching
- [ ] Audit related entity payloads (comments, attachments, time entries, subtasks) for parent task references
  - Add `task_key` to any payload that serializes parent task info
- [ ] Update CRM/support association displays to show `task_key` when linking to PM tasks
- [ ] Update activity feed entries (if any render task references) to use `task_key`

### Phase 7: Backward compatibility, polish, and optional full-page route

- [ ] Numeric URL compat: `?task=123` continues to resolve by `display_id` lookup indefinitely
- [ ] Alias URL compat: `?task=OLD-123` resolves via `workspace_key_history` fallback after key change
- [ ] API filter compat: continue accepting `display_id` in query params alongside `task_key`
- [ ] Update `CLAUDE.md` to document the workspace key, task key, and key change alias patterns
- [ ] Verify CRM display_id prefixes (`C-`, `CO-`, `D-`) are not affected by this change
- [ ] Optional: dedicated full-page task detail route `/pm/tasks/HLP-123`
  - For external sharing (Slack links, email, bookmarks)
  - Renders same task detail content in a page layout with breadcrumbs back to board/list
  - Additive — not blocking for v1
- [ ] End-to-end test: full lifecycle including key change
  - Create workspace with key `HLP`, create tasks, verify `HLP-123` everywhere
  - Task detail header shows `HLP-123`
  - Copy-to-clipboard copies `HLP-123`
  - Task list column shows `HLP-123`
  - Branch preview shows `HLP-123-fix-login-timeout`
  - Search finds task by `HLP-123`
  - URL `?task=HLP-123` opens correct task
  - URL `?task=123` still opens correct task
  - Email notification shows `HLP-123`
  - New team repo default uses `{task_key}-{slug}` template
  - Change workspace key from `HLP` to `NCO`
  - New task keys show `NCO-124`, `NCO-125`, etc.
  - Old references `HLP-123` still resolve (via alias table)
  - Search for `HLP-123` still finds the task
  - URL `?task=HLP-123` still opens correct task
  - Settings shows key history: "Previously: HLP"
  - WebSocket task update events include `task_key` and cache updates correctly
