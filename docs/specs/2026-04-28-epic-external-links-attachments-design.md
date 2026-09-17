# Epic External Links & Attachments Parity

**Date:** 2026-04-28
**Status:** Approved

## Problem

Epics lack feature parity with Tasks for external links and file attachments:

1. **External Links** — not supported on Epics at all (backend model is task-only)
2. **Epic Creation Modal** — toggle pill bar is positioned above the editor (inconsistent with Task creation which places it below), and only has "Attach Files" (missing "External Links")
3. **Epic Detail View** — no External Links section, no toolbar Attach button, no panel-wide drag-and-drop overlay for file uploads (Attachments component exists but lacks integration hooks)

## Solution

Generalize external links to support multiple entity types (matching the existing attachments pattern), then wire both external links and improved attachments into Epic creation and detail views.

---

## Backend Changes

### 1. Model — `PMExternalLink`

Add `EntityType` and `EntityID` fields. Keep `TaskID` for backward compatibility (GORM AutoMigrate cannot drop columns).

```go
type PMExternalLink struct {
    ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    TaskID      string    `json:"task_id" gorm:"column:task_id;type:uuid;index"`
    EntityType  string    `json:"entity_type" gorm:"type:varchar(32);default:'task'"`
    EntityID    string    `json:"entity_id" gorm:"type:uuid;index:idx_ext_links_entity"`
    Title       string    `json:"title" gorm:"not null"`
    URL         string    `json:"url" gorm:"not null"`
    CreatedByID string    `json:"created_by_id" gorm:"type:uuid;not null"`
    CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
```

- GORM tags for `EntityType`/`EntityID` omit `not null` — AutoMigrate adds columns as nullable. The `NOT NULL` constraints are applied by the dbmigrate migration after backfill.
- `TaskID` becomes nullable (remove `not null` via migration), kept for legacy queries
- New rows set both `EntityType`/`EntityID` and `TaskID` (when entity_type is "task") for backward compat
- Composite index on `(entity_type, entity_id)`

### 2. Migration — `202604280001_external_links_entity_type.sql`

```sql
-- Add entity_type and entity_id columns (nullable initially for safe backfill)
ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_type VARCHAR(32);
ALTER TABLE pm_external_links ADD COLUMN IF NOT EXISTS entity_id UUID;

-- Backfill entity_id from task_id for existing rows
UPDATE pm_external_links SET entity_type = 'task', entity_id = task_id WHERE entity_id IS NULL AND task_id IS NOT NULL;

-- Delete orphaned rows with no task_id (bad data safety net)
DELETE FROM pm_external_links WHERE task_id IS NULL AND entity_id IS NULL;

-- Now make entity_type and entity_id NOT NULL with defaults
ALTER TABLE pm_external_links ALTER COLUMN entity_type SET NOT NULL;
ALTER TABLE pm_external_links ALTER COLUMN entity_type SET DEFAULT 'task';
ALTER TABLE pm_external_links ALTER COLUMN entity_id SET NOT NULL;

-- Make task_id nullable (was NOT NULL)
ALTER TABLE pm_external_links ALTER COLUMN task_id DROP NOT NULL;

-- Add composite index
CREATE INDEX IF NOT EXISTS idx_pm_external_links_entity ON pm_external_links (entity_type, entity_id);
```

### 3. Repository — `PMExternalLinkRepository`

Add new method, update existing:

- **`ListByEntity(ctx, entityType, entityID)`** — queries by `entity_type` + `entity_id`. New primary method.
- **`CountByEntity(ctx, entityType, entityID)`** — same pattern.
- **`List(ctx, storyID)`** — keep as-is for backward compat (used by task service inline creation).
- **`Count(ctx, storyID)`** — keep as-is.

### 4. Service — `PMExternalLinkService`

Add new methods that accept `entityType` + `entityID`:

- **`ListByEntity(ctx, entityType, entityID)`** — validates entityType is in allowed set (`task`, `epic`), delegates to repo.
- **`CreateForEntity(ctx, entityType, entityID, req, userID, workspaceID)`** — creates link with `EntityType`/`EntityID` set. Also sets `TaskID` when `entityType == "task"`. Publishes WebSocket event with correct `ParentType`.
- Existing `List()` and `Create()` methods remain unchanged — they continue to work for tasks. `Create()` must be updated to also set `EntityType: "task"` and `EntityID: storyID` on the new link so the new columns are populated.
- **Required**: `Update()` and `Delete()` must use `link.EntityType`/`link.EntityID` for the WebSocket event `ParentType`/`ParentID` instead of hardcoded `"task"`/`link.TaskID`. For epic links `TaskID` will be empty, so using it would break realtime listeners.

### 5. Handler — `PMExternalLinkHandler`

Add two new handler methods:

- **`ListByEntity(w, r)`** — reads `entity_type` and `entity_id` from URL params.
- **`CreateForEntity(w, r)`** — reads `entity_type` and `entity_id` from URL params.

Validation: `entityType` must be `"task"` or `"epic"`. Return 400 otherwise.

Existing `List()` and `Create()` handlers remain unchanged (they serve the `/tasks/{id}/links` routes).

### 6. Router

Add new generic routes alongside existing task-specific ones:

```go
// Generic entity external links (snake_case URL params per project convention)
r.With(requirePerm(authorization.PermPMRead)).Get("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.ListByEntity)
r.With(requirePerm(authorization.PermPMEdit)).Post("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.CreateForEntity)

// Existing task-specific routes remain unchanged
r.With(requirePerm(authorization.PermPMRead)).Get("/tasks/{id}/links", h.PMExternalLink.List)
r.With(requirePerm(authorization.PermPMEdit)).Post("/tasks/{id}/links", h.PMExternalLink.Create)
r.With(requirePerm(authorization.PermPMEdit)).Put("/links/{id}", h.PMExternalLink.Update)
r.With(requirePerm(authorization.PermPMEdit)).Delete("/links/{id}", h.PMExternalLink.Delete)
```

Update/Delete routes don't change — they operate on link ID which is entity-agnostic.

---

## Frontend Changes

### 7. Service — `pmExternalLinkService.ts`

Add generic entity methods:

```ts
listByEntity: (wsId, entityType, entityId) =>
  api.get<ExternalLink[]>(`/pm/entity-links/${entityType}/${entityId}?workspace_id=${wsId}`),

createForEntity: (wsId, entityType, entityId, payload) =>
  api.post<ExternalLink>(`/pm/entity-links/${entityType}/${entityId}?workspace_id=${wsId}`, payload),
```

Existing `list()` and `create()` methods remain for backward compat.

### 7b. TypeScript Types — `pmTypes.ts`

Add `entity_type` and `entity_id` to the `ExternalLink` interface:

```ts
interface ExternalLink {
  // ... existing fields ...
  entity_type: string;
  entity_id: string;
}
```

### 7c. Query Keys — `queryKeys.ts`

Add entity-generic external links key:

```ts
entityExternalLinks: (wsId: string, entityType: string, entityId: string) =>
  ['pm', wsId, entityType, entityId, 'externalLinks'] as const,
```

### 7d. Realtime Sync — `useRealtimeSync`

When a WebSocket event arrives for `entity === 'external_link'`, use `parent_type` from the event to build the correct query key for invalidation:

- If `parent_type === 'task'`: invalidate `queryKeys.pm.externalLinks(wsId, event.parent_id)` (existing)
- Also invalidate `queryKeys.pm.entityExternalLinks(wsId, event.parent_type, event.parent_id)` (new)

Dispatch a generic `entity-child-updated` DOM event (in addition to existing `task-child-updated`) so the `ExternalLinks` component can listen on it regardless of entity type.

### 8. Component — `ExternalLinks.tsx`

Change props interface:

```ts
interface ExternalLinksProps {
  workspaceId: string;
  entityType: 'task' | 'epic';
  entityId: string;
}
```

- Use `listByEntity` / `createForEntity` service methods
- WebSocket DOM event listener: listen on both `task-child-updated` (backward compat) and `entity-child-updated`, match on `detail.parent_id === entityId`
- All existing task usages updated: `<ExternalLinks workspaceId={wsId} entityType="task" entityId={task.id} />`

### 9. Epic Creation Modal — `GlobalCreateModals.tsx` (`GlobalCreateEpic`)

**Move toggle pills below editor:**
- Current layout: `[Attach Files] → <TiptapEditor> → {attachments section}`
- New layout: `<TiptapEditor> → [External Links] [Attach Files] → {sections}`

**Add External Links pill + inline editor:**
- New state: `showExternalLinks`, `externalLinks: { url: string }[]`
- Pill with `Link01Icon`, count badge, active/inactive styling (same pattern as Task creation)
- Inline link editor: URL input + add/remove buttons (same UI as Task creation)
- After epic creation: POST each link via `pmExternalLinkService.createForEntity(wsId, 'epic', epicId, { url })`

**Keep Attach Files pill** — just repositioned below editor.

### 10. Epic Detail View — `EpicDetail.tsx`

**Add External Links section:**
- New state: `showExternalLinks` (boolean)
- On load: check if links exist via `pmExternalLinkService.listByEntity()`, auto-show if so
- Toggle button in toolbar row (matching Task detail pattern)
- `<ExternalLinks workspaceId={wsId} entityType="epic" entityId={epic.id} />`

**Improve Attachments integration:**
- Add refs: `openFilePickerRef`, `uploadFilesRef`, `dragCounterRef`
- Pass `onFilePickerReady` and `onUploadReady` callbacks to `<Attachments>`
- Add "Attach" toolbar button that triggers `openFilePickerRef.current?.()`
- Add panel-wide drag-and-drop overlay:
  - `onDragEnter`: increment `dragCounterRef`, set `panelDragging` state
  - `onDragLeave`: decrement counter, clear state when 0
  - `onDrop`: reset counter, call `uploadFilesRef.current?.(files)`
  - Overlay: translucent backdrop with "Drop files to attach" message

**Add toggle buttons row** (below description, above progress):
- External Links toggle
- Attach button (triggers file picker)

### 11. Task Detail & Task Creation — Update ExternalLinks usage

- `TaskDetailPanel.tsx`: change `<ExternalLinks taskId={task.id} />` to `<ExternalLinks entityType="task" entityId={task.id} />`
- `CreateTaskModal.tsx`: no changes needed (task creation uses inline `external_links` array in the create request, not the `ExternalLinks` component)

---

## Files Changed

### Backend
| File | Change |
|------|--------|
| `server/internal/model/pm_external_link.go` | Add `EntityType`, `EntityID` fields |
| `server/internal/dbmigrate/sql/202604280001_external_links_entity_type.sql` | New migration |
| `server/internal/repository/pm_external_link.go` | Add `ListByEntity`, `CountByEntity` |
| `server/internal/service/pm_external_link.go` | Add `ListByEntity`, `CreateForEntity`; update `Create`/`Update`/`Delete` to set/use entity fields + WS events |
| `server/internal/service/pm_task.go` | Set `EntityType`/`EntityID` on inline external link creation |
| `server/internal/handler/pm_external_link.go` | Add `ListByEntity`, `CreateForEntity` handlers |
| `server/internal/router/router.go` | Add generic entity-link routes |

### Frontend
| File | Change |
|------|--------|
| `frontend/src/lib/services/pmExternalLinkService.ts` | Add `listByEntity`, `createForEntity` |
| `frontend/src/lib/pmTypes.ts` | Add `entity_type`, `entity_id` to `ExternalLink` interface |
| `frontend/src/lib/queryKeys.ts` | Add `entityExternalLinks` key factory |
| `frontend/src/hooks/queries/useRealtimeSync.ts` | Invalidate entity-generic external links keys, dispatch `entity-child-updated` event |
| `frontend/src/components/pm/ExternalLinks.tsx` | Change props to `entityType`/`entityId`, use generic service methods |
| `frontend/src/components/pm/GlobalCreateModals.tsx` | Move pills below editor, add External Links pill + inline editor |
| `frontend/src/pages/pm/EpicDetail.tsx` | Add External Links section, drag-drop overlay, attachment integration hooks |
| `frontend/src/components/pm/TaskDetailPanel.tsx` | Update `<ExternalLinks>` props |

---

## What's NOT Changing

- Task creation modal — no changes (uses inline `external_links` in create request)
- Task detail attachments — no changes (already has full integration)
- Existing `/pm/tasks/{id}/links` API routes — preserved for backward compat
- Checklist, Relationships — not included in this scope
- Backend task service inline external link creation during task create — continues using `TaskID` field directly, but also sets `EntityType`/`EntityID` on the new link

## Risks & Mitigations

- **Migration on existing data**: Backfill is simple (`entity_id = task_id`), idempotent, and the `task_id` column is preserved. Rollback: drop the new columns.
- **Backward compat**: Old task-specific routes/service methods are untouched. New code paths are additive.
- **WebSocket events**: Update/Delete events will use `EntityType`/`EntityID` for parent info. Frontend listeners already match on `parent_id === entityId`, so this is compatible.
