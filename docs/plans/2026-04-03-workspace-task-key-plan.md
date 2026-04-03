# Workspace Task Key Plan

## Status

Plan for introducing a **workspace-level canonical task key** (e.g. `HLP-123`) as the primary human-readable identifier for PM tasks, replacing raw numeric `display_id` in all user-facing surfaces.

This plan assumes:

- the canonical identifier is **workspace-scoped**, not team-scoped
- the workspace key is **changeable but discouraged** (2-5 uppercase characters, old keys kept as aliases)
- `display_id` remains the underlying sequence source and is **not removed**
- `task_key` is a **computed field** (`{workspace_key}-{display_id}`) on `PMTask` via `gorm:"-"`, not a stored column
- team code is **not part of v1** — no `{team_code}` token, no schema change on teams
- backward compatibility is preserved: numeric `display_id` continues to work in URLs and search

## Goal

Give every PM task a stable, human-friendly identifier that:

- is instantly recognizable as belonging to a workspace (e.g. `HLP-184`)
- works in branch names, search, copy/paste, URLs, notifications, and conversation
- standardizes the currently inconsistent branch naming (`tp-{display_id}-{slug}` vs `{display_id}-{slug}` vs raw numeric)
- provides a foundation for cross-workspace task references in the future

## Core Decisions

### 1. Workspace key lives on the `workspaces` table

Add `workspace_key` as a first-class column on `workspaces`, not in settings. It is identity, not configuration.

- `VARCHAR(5)`, `NOT NULL`, `UNIQUE INDEX`
- Validated server-side: `^[A-Z]{2,5}$`
- Changeable via a guarded flow (old keys become aliases for backward resolution)
- Required for new workspaces; backfilled for existing workspaces from slug

### 2. Task key is a computed field on `PMTask` via `gorm:"-"`

`task_key` = `workspace_key` + `-` + `display_id`. It lives on the `PMTask` struct with `gorm:"-"` so GORM ignores it for storage, but JSON serialization includes it in every response.

Why this placement matters:

- `TaskDetail` embeds `PMTask` as `Task PMTask` (`server/internal/model/pm_task.go:237`)
- `BoardTask` embeds `PMTask` directly (`server/internal/model/pm_task.go:249`)
- List, sprint, epic, and search payloads all serialize `PMTask` fields
- Putting `task_key` on wrapper DTOs instead of `PMTask` would miss most payloads and require ad-hoc threading

By placing it on `PMTask`, every embedding struct gets `task_key` for free. The service layer populates it centrally before returning any task response.

Implementation:

```go
// On PMTask struct — computed, not stored
TaskKey string `json:"task_key" gorm:"-"`
```

Also add `TaskKey string \`json:"task_key" gorm:"-"\`` to:

- `TaskDependencyTask` in `server/internal/model/pm_task.go`
- `SearchResult` in `server/internal/model/search.go`

Backend helper:

```go
func FormatTaskKey(workspaceKey string, displayID int) string {
    return fmt.Sprintf("%s-%d", workspaceKey, displayID)
}
```

Frontend helper:

```typescript
function formatTaskKey(workspaceKey: string, displayId: number): string {
    return `${workspaceKey}-${displayId}`;
}
```

### 3. Branch template tokens are expanded (without team_code)

Current tokens: `{display_id}`, `{slug}`

New tokens in v1:

| Token | Resolves to | Example |
| --- | --- | --- |
| `{task_key}` | `HLP-123` | Primary new token |
| `{workspace_key}` | `HLP` | For custom patterns |
| `{display_id}` | `123` | Kept for backward compat |
| `{slug}` | `fix-login-timeout` | Unchanged |
| `{task_type}` | `feature`, `bug`, `chore` | For type-prefixed branches |

`{team_code}` is **not included in v1**. The current team model only has an editable `handle` field (`server/internal/model/settings.go:38`), not an immutable code. Using a mutable handle as a branch token defeats the plan's own stability goal. If team-aware branch tokens are needed, they require a real `team_code` schema change first — that is a v2 concern.

New default template: `{task_key}-{slug}` (e.g. `HLP-123-fix-login-timeout`)

**Three codepaths must be updated to enforce one canonical default:**

1. Model default: `PMTeamRepoDefault.BranchTemplate` in `server/internal/model/git.go:49` (currently `tp-{display_id}-{slug}`)
2. Repository hardcode: `server/internal/repository/settings.go:451` (currently `{display_id}-{slug}` — differs from model default)
3. Frontend preview: `buildBranchPreview()` in `frontend/src/components/pm/TaskDeliveryPanel.tsx:621` (currently `${displayId}-${slugify(taskName)}`)

All three must converge on `{task_key}-{slug}`.

### 4. Search and URL resolution for task keys

Search must accept:

- `123` (numeric, existing behavior)
- `HLP-123` (full task key, new)

**How `?task=HLP-123` resolves in the existing route/state flow:**

Current state: task opening uses `?task={display_id}` (numeric) in the URL search params. The frontend reads this param and calls the task detail API with the numeric display_id. Three codepaths rely on this:

- `TaskDetailPanel.tsx:564` — sets `searchParams.set('task', displayId)`
- `SearchCommandPalette.tsx:109` — navigates via `?task=${item.display_id}`
- `pmTaskLinks.ts:35` — fallback URL uses `searchParams.set('task', String(displayId))`

**Resolution strategy:**

1. The `?task=` param accepts either format: numeric (`?task=123`) or key (`?task=HLP-123`)
2. Frontend URL parser: attempt numeric parse first; if NaN, treat as task key and extract display_id from the suffix after the last `-`
3. Backend task lookup: existing `WHERE workspace_id = ? AND display_id = ?` query works for both — the workspace is already known from the route context
4. For task key format in URLs, frontend can also validate the prefix matches the current workspace key as an extra safety check
5. All new navigation should write `?task=HLP-123` format; old numeric format continues to resolve indefinitely

**Search backend** (`server/internal/model/search.go`, `server/internal/handler/search.go`):

1. Parse search query: detect `^[A-Z]{2,5}-\d+$` pattern
2. If matched: extract display_id, query directly by display_id (workspace already scoped)
3. If not matched: existing text search behavior unchanged
4. `SearchResult` struct gets `TaskKey string \`json:"task_key" gorm:"-"\`` populated by service layer

### 5. Workspace key is changeable with alias preservation

The workspace key **can be changed**, but old keys are preserved as aliases so that existing external references continue to resolve. This is built into v1 from day one.

Why allow changes:

- Typos during workspace creation should be fixable
- Company rebrands happen (Acme → NewCo means `ACM-123` → `NCO-123`)
- Workspace repurposing (a team migrates from one workspace to another and wants continuity)

Why aliases are required:

- Task keys appear in branch names, commit messages, PR titles, Slack threads, docs, emails, bookmarks, and external tools
- Without aliases, changing the key would silently orphan all external references
- Branch names in git history are permanent — `HLP-123-fix-login` will exist in the repo forever even after the key changes to `NCO`

**Schema: `workspace_key_history` table**

```sql
CREATE TABLE IF NOT EXISTS workspace_key_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    old_key VARCHAR(5) NOT NULL,
    new_key VARCHAR(5) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_by UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_wkh_workspace_id ON workspace_key_history (workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wkh_old_key ON workspace_key_history (old_key);
```

**Resolution logic (backend):**

```
ResolveWorkspaceKey(key string) → workspace_id:
  1. SELECT id FROM workspaces WHERE workspace_key = key  →  if found, return
  2. SELECT workspace_id FROM workspace_key_history WHERE old_key = key  →  if found, return
  3. Not found
```

This runs on every task key parse (search, URL resolution, API filter). The alias lookup is index-backed and only fires on cache miss of the primary key, so the performance cost is negligible.

**Change flow:**

1. User requests key change in workspace settings (admin/owner only)
2. Backend validates new key: `^[A-Z]{2,5}$`, unique across both `workspaces.workspace_key` and `workspace_key_history.old_key`
3. Insert old key into `workspace_key_history`
4. Update `workspaces.workspace_key` to new key
5. All in one transaction
6. Frontend shows confirmation dialog: "Changing from HLP to NCO. Existing references like HLP-123 will continue to work, but new task keys will use NCO-123."
7. No cooldown required — the alias table makes the change safe

**Uniqueness constraint:**

Old keys must remain globally unique. A key that was once used by any workspace can never be reused as a new key by a different workspace. This prevents ambiguous resolution. The unique index on `workspace_key_history.old_key` enforces this, and the validation on create/update must check both tables.

**Frontend UX:**

- Creation form: auto-suggest, strong encouragement to pick the right key, but no "permanent" warning
- Settings page: editable field (admin/owner), with explanation: "Changing the workspace key will update all new task identifiers. Existing references using the old key (in branches, links, etc.) will continue to work."
- After change: all new task key displays immediately show the new key (since task_key is computed from current workspace_key + display_id)

### 6. Team code is deferred to v2

Team code (`team_code` on `workspace_teams`) is not part of this plan. It would require:

- A new immutable `team_code` field alongside the editable `handle`
- A migration to backfill existing teams
- Clear rules for what happens to team code when teams are reorganized

This is explicitly out of scope for v1. No `{team_code}` token, no team code schema changes.

## Schema Changes

### Migration: Add `workspace_key` to `workspaces`

File: `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_workspace_key.sql`

```sql
-- Add workspace_key column (nullable first for backfill)
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS workspace_key VARCHAR(5);

-- Backfill existing workspaces using alpha-only strategy
-- Strategy: try progressively longer substrings from slug (3, 4, 5 chars),
-- then append alpha suffixes (A-Z) at each length, all within VARCHAR(5).
-- Bounded to 130 attempts max (5 lengths * 26 suffixes + 5 bare), which is
-- sufficient for any realistic workspace count sharing a slug prefix.
DO $$
DECLARE
  ws RECORD;
  alpha_slug TEXT;
  candidate TEXT;
  base_len INT;
  suffix_char INT;
  found BOOLEAN;
BEGIN
  FOR ws IN SELECT id, slug FROM workspaces WHERE workspace_key IS NULL ORDER BY created_at LOOP
    -- Strip non-alpha, uppercase
    alpha_slug := UPPER(REGEXP_REPLACE(ws.slug, '[^a-zA-Z]', '', 'g'));
    IF LENGTH(alpha_slug) < 2 THEN
      alpha_slug := 'WS';
    END IF;

    found := FALSE;

    -- Try bare substrings first: 3, 4, 5 chars
    FOR base_len IN 3..LEAST(LENGTH(alpha_slug), 5) LOOP
      candidate := LEFT(alpha_slug, base_len);
      IF NOT EXISTS (SELECT 1 FROM workspaces WHERE workspace_key = candidate AND id != ws.id) THEN
        found := TRUE;
        EXIT;
      END IF;
    END LOOP;

    -- If bare substrings all collide, try alpha suffixes within 5-char limit
    IF NOT found THEN
      FOR base_len IN 2..4 LOOP
        FOR suffix_char IN 65..90 LOOP  -- A=65, Z=90
          candidate := LEFT(alpha_slug, base_len) || CHR(suffix_char);
          IF LENGTH(candidate) <= 5 AND NOT EXISTS (
            SELECT 1 FROM workspaces WHERE workspace_key = candidate AND id != ws.id
          ) THEN
            found := TRUE;
            EXIT;
          END IF;
        END LOOP;
        EXIT WHEN found;
      END LOOP;
    END IF;

    -- Final fallback: should never reach here with <100 workspaces
    IF NOT found THEN
      candidate := LEFT(alpha_slug, 2) || 'X';
    END IF;

    UPDATE workspaces SET workspace_key = candidate WHERE id = ws.id;
  END LOOP;
END $$;

-- Now make it NOT NULL and add unique index
ALTER TABLE workspaces ALTER COLUMN workspace_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_workspace_key ON workspaces (workspace_key);
```

The backfill strategy is:

1. Extract alpha-only characters from slug, uppercase
2. Try bare substrings: 3, 4, 5 chars (e.g. `HEL`, `HELP`, `HELPI`)
3. If all collide, try base (2-4 chars) + alpha suffix (A-Z), staying within 5-char limit (e.g. `HEA`, `HEB`, ..., `HEZ`, `HELA`, `HELB`, ...)
4. All candidates satisfy `^[A-Z]{2,5}$` — no numeric suffixes, no length overflow
5. Bounded to ~130 attempts, sufficient for any realistic workspace count

### Migration: Add `workspace_key_history` table

Included in the same migration file (or a separate one if preferred):

```sql
CREATE TABLE IF NOT EXISTS workspace_key_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    old_key VARCHAR(5) NOT NULL,
    new_key VARCHAR(5) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_by UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_wkh_workspace_id ON workspace_key_history (workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wkh_old_key ON workspace_key_history (old_key);
```

No backfill needed — the table starts empty. It only accumulates rows when a workspace key is changed.

### Model change: `Workspace` struct

```go
type Workspace struct {
    // ... existing fields ...
    WorkspaceKey string `json:"workspace_key" gorm:"type:varchar(5);uniqueIndex;not null"`
}
```

### DTO changes

```go
type CreateWorkspaceRequest struct {
    Name           string  `json:"name"`
    Slug           string  `json:"slug"`
    WorkspaceKey   string  `json:"workspace_key"`   // NEW — required
    OrganizationID string  `json:"organization_id"`
    // ... existing optional fields ...
}
```

`UpdateWorkspaceRequest` includes `workspace_key` as optional — only admin/owner can change it.

```go
type UpdateWorkspaceRequest struct {
    // ... existing fields ...
    WorkspaceKey *string `json:"workspace_key,omitempty"` // Optional — triggers alias flow if changed
}
```

### Model: `WorkspaceKeyHistory`

```go
type WorkspaceKeyHistory struct {
    ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
    OldKey      string    `json:"old_key" gorm:"type:varchar(5);not null;uniqueIndex"`
    NewKey      string    `json:"new_key" gorm:"type:varchar(5);not null"`
    ChangedAt   time.Time `json:"changed_at" gorm:"autoCreateTime"`
    ChangedBy   string    `json:"changed_by" gorm:"type:uuid"`
}

func (WorkspaceKeyHistory) TableName() string { return "workspace_key_history" }
```

### PMTask model addition

```go
type PMTask struct {
    // ... existing fields ...
    TaskKey string `json:"task_key" gorm:"-"` // Computed: "HLP-123", not stored
}
```

Also on related structs:

```go
type TaskDependencyTask struct {
    // ... existing fields ...
    TaskKey string `json:"task_key" gorm:"-"`
}
```

```go
// In server/internal/model/search.go
type SearchResult struct {
    // ... existing fields ...
    TaskKey string `json:"task_key,omitempty" gorm:"-"`
}
```

### Frontend type additions

```typescript
// In pm-types/project.ts
export interface Task {
  // ... existing fields ...
  task_key: string;  // "HLP-123" — computed by backend
}

export interface TaskDependencyTask {
  // ... existing fields ...
  task_key: string;
}
```

## Affected Surfaces

### Backend (Go)

| File | Change |
| --- | --- |
| `server/internal/model/workspace.go` | Add `WorkspaceKey` field to struct, add to `CreateWorkspaceRequest` and `UpdateWorkspaceRequest`, add `WorkspaceKeyHistory` model |
| `server/internal/repository/workspace.go` | Pass `workspaceKey` in `Create()`, add `FindWorkspaceByKeyOrAlias()`, `InsertKeyHistory()`, key uniqueness check across both tables |
| `server/internal/service/workspace.go` | Validate `workspace_key` format and uniqueness on create; on update: validate, insert alias, update key in transaction |
| `server/internal/handler/workspace.go` | No change (auto-decoded from request DTO) |
| `server/internal/model/pm_task.go` | Add `TaskKey string \`json:"task_key" gorm:"-"\`` to `PMTask`, `TaskDependencyTask` |
| `server/internal/model/search.go` | Add `TaskKey string \`json:"task_key,omitempty" gorm:"-"\`` to `SearchResult` |
| `server/internal/service/pm_task.go` | Populate `task_key` centrally on all task responses before returning |
| `server/internal/service/search.go` | Populate `task_key` on search results |
| `server/internal/repository/pm_task.go` | No schema change; existing `workspace_id + display_id` query serves task key lookup |
| `server/internal/model/git.go` | Update `PMTeamRepoDefault.BranchTemplate` default from `tp-{display_id}-{slug}` to `{task_key}-{slug}` |
| `server/internal/repository/settings.go` | Update hardcoded default at line ~451 from `{display_id}-{slug}` to `{task_key}-{slug}` |
| `server/internal/service/git.go` | Add `{task_key}`, `{workspace_key}`, `{task_type}` token resolution |
| `server/internal/email/template.go` | Update `TaskBlockHTML` to use task key instead of raw display_id |
| `server/internal/service/notification.go` | Include `task_key` in notification payloads |
| `server/internal/dbmigrate/sql/` | Add workspace_key migration |

### Frontend (React/TypeScript)

| File | Change |
| --- | --- |
| `frontend/src/lib/pm-types/project.ts` | Add `task_key: string` to `Task`, `TaskDependencyTask` interfaces |
| `frontend/src/lib/taskKeyUtils.ts` | **NEW** — `formatTaskKey()`, `parseTaskKey()` helpers |
| `frontend/src/components/pm/TaskDetailPanel.tsx` | Replace `displayId` with `task.task_key` in header |
| `frontend/src/components/pm/TaskSidebarIdRow.tsx` | Show `task_key` instead of raw `displayId`, update copy-to-clipboard, update git branch generation |
| `frontend/src/components/pm/TaskListView.tsx` | Replace `display_id` column with `task_key` column |
| `frontend/src/components/pm/TaskCard.tsx` | Show `task_key` in blocked-by message |
| `frontend/src/components/pm/TaskRelationshipsSection.tsx` | Show `task_key` in relationship badges |
| `frontend/src/components/pm/TaskDeliveryPanel.tsx` | Update `buildBranchPreview()` (line ~621) to use `task_key` instead of raw `displayId`; update branch preview display (line ~129) |
| `frontend/src/components/pm/AssociationsPanel.tsx` | Show `task_key` for linked tasks |
| `frontend/src/pages/pm/MyWork.tsx` | Replace `task.display_id` with `task.task_key` |
| `frontend/src/components/search/SearchCommandPalette.tsx` | Parse `HLP-123` format in search input, show `task_key` in results, update navigation links |
| `frontend/src/lib/services/searchService.ts` | Add `task_key` to search result type |
| `frontend/src/lib/pmTaskLinks.ts` | Use `task_key` in URL params and fallback |
| `frontend/src/pages/Workspaces.tsx` | Add workspace key input to creation form |
| `frontend/src/components/settings/GeneralTab.tsx` | Show workspace key (editable by admin/owner), confirmation dialog on change, alias explanation |
| `frontend/src/components/settings/teams/TeamRepoDefaultForm.tsx` | Update token help text, add `{task_key}` as documented token, remove `{team_code}` reference |
| `frontend/src/lib/services/workspacesService.ts` | Add `workspace_key` to create payload |
| `frontend/src/lib/types.ts` | Add `workspace_key` to `Workspace` interface |

## Implementation Order

### Phase 1: Foundation (workspace key + backend plumbing)

1. **Schema migration**: Add `workspace_key` column to `workspaces` with alpha-only backfill
2. **Workspace model**: Add `WorkspaceKey` field to Go struct and DTOs
3. **Workspace creation flow**: Require `workspace_key` on create, validate format, reject on update
4. **Task key helper**: Backend `FormatTaskKey()` function
5. **PMTask model**: Add `TaskKey string \`gorm:"-"\`` to `PMTask`, `TaskDependencyTask`, `SearchResult`
6. **Task service**: Populate `task_key` centrally on all task responses (detail, list, board, sprint, epic, search)

### Phase 1b: Key alias infrastructure

7. **Schema migration**: Add `workspace_key_history` table with unique index on `old_key`
8. **History model**: Add `WorkspaceKeyHistory` struct to `server/internal/model/workspace.go`
9. **Repository**: Add `InsertKeyHistory()`, `FindWorkspaceByKeyOrAlias()`, cross-table uniqueness check
10. **Service update flow**: On workspace key change — validate new key (format + unique across both tables), insert old key into history, update workspace key, all in one transaction
11. **Resolution helper**: `ResolveWorkspaceKey(key) → workspace_id` — check `workspaces.workspace_key` first, fall back to `workspace_key_history.old_key`
12. **Wire into search/URL resolution**: Task key parsing uses `ResolveWorkspaceKey()` so old keys like `HLP-123` continue to resolve after the workspace changes to `NCO`

### Phase 2: Frontend workspace key setup

13. **Frontend workspace type**: Add `workspace_key` to TypeScript interface
14. **Workspace creation UI**: Add workspace key input with auto-suggestion from name
15. **Workspace settings UI**: Editable workspace key field (admin/owner only) with confirmation dialog: "Changing from HLP to NCO. Existing references like HLP-123 will continue to work, but new task keys will use NCO-123."

### Phase 3: PM UI rollout

16. **Frontend task key helper**: `formatTaskKey()` + `parseTaskKey()` utils
17. **Task detail panel**: Replace raw `displayId` with `task_key` in header
18. **Task sidebar ID row**: Update display, copy-to-clipboard, git branch preview
19. **Task list/table view**: Replace `display_id` column with `task_key`
20. **Task cards**: Update blocked-by text
21. **My Work page**: Replace `display_id` with `task_key`
22. **Relationships & associations**: Show `task_key` in badges and links
23. **Task delivery panel**: Update `buildBranchPreview()` to use `task_key`

### Phase 4: Branch template standardization

24. **Backend token expansion**: Add `{task_key}`, `{workspace_key}`, `{task_type}` tokens to `server/internal/service/git.go`
25. **Model default update**: Change `server/internal/model/git.go:49` from `tp-{display_id}-{slug}` to `{task_key}-{slug}`
26. **Repository hardcode update**: Change `server/internal/repository/settings.go:451` from `{display_id}-{slug}` to `{task_key}-{slug}`
27. **Frontend template form**: Update token documentation and preview in `TeamRepoDefaultForm.tsx`
28. **Existing template migration**: Optional data migration for existing `tp-{display_id}-{slug}` templates

### Phase 5: Search and URL resolution

29. **Backend search parser**: Detect `^[A-Z]{2,5}-\d+$` pattern, use `ResolveWorkspaceKey()` for alias-aware lookup, extract display_id
30. **Frontend URL parser**: `?task=` param accepts both `123` and `HLP-123`; extract display_id from key format
31. **Search command palette**: Update result badges, navigation links, and input parsing
32. **pmTaskLinks**: Write `?task=HLP-123` format in new navigation; keep numeric fallback

### Phase 6: Cross-product integration

33. **Email templates**: Use `task_key` in notification emails via `TaskBlockHTML()`
34. **Notification payloads**: Include `task_key` in websocket and push events
35. **Activity feeds**: Show `task_key` in activity log entries
36. **Support/CRM associations**: Show `task_key` when linking to PM tasks

### Phase 7: Polish and backward compatibility

37. **Numeric URL compat**: `?task=123` continues to resolve by `display_id` lookup indefinitely
38. **Alias URL compat**: `?task=OLD-123` resolves via `workspace_key_history` fallback
39. **API filter compat**: Continue accepting `display_id` in query params
40. **Documentation**: Update CLAUDE.md to document workspace key, task key, and key change patterns
41. **CRM verification**: Confirm CRM display_id prefixes (`C-`, `CO-`, `D-`) are not affected
42. **End-to-end test**: Full lifecycle — create workspace with key, create tasks, change key, verify old keys still resolve, verify new keys appear in UI

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Workspace key collisions during backfill | Alpha-only collision strategy with 130+ candidates; manual review for production |
| Users pick poor keys (too short, confusing) | Frontend auto-suggest from workspace name, format validation, preview of resulting task IDs |
| Performance: loading workspace key for every task response | Cache workspace key in service layer (one query per workspace, not per task) |
| Breaking existing branch templates | `{display_id}` token continues to work; only default changes for new repos |
| Existing bookmarks/links with numeric IDs | Numeric `?task=123` continues to work indefinitely |
| Three branch-default codepaths diverge again | Single canonical default constant; all three codepaths reference it |
| External references become stale after key change | Old keys preserved as aliases in `workspace_key_history`; resolution falls back to alias table |
| Old key reuse by different workspace | Unique index on `workspace_key_history.old_key` prevents reuse; validation checks both tables |

## Out of Scope (v2+)

- **Team code** (v2): Immutable `team_code` on teams, `{team_code}` branch token, team badges
- **Per-team sequences** (v2): Would require `team_id` to be non-nullable and immutable
- **Cross-workspace references** (v2): `HLP-123` linking from another workspace's context
- **Bulk key migration tooling** (v2): Rewriting old keys in external systems (git branches, CI configs) — v1 only preserves resolution, not rewriting
