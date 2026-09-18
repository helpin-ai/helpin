# Epic external links and attachments implementation plan

This historical implementation plan records how task external links were extended
to epics. Use it to understand the original change; consult the source notes below
for current schema and UI behavior before implementing further work.

## Source review — 2026-09-18

- [The model](../../server/internal/model/pm_external_link.go) now has nullable
  `TaskID` and **not-null** `EntityType`/`EntityID` tags. The original nullable-tag
  staging instructions below no longer describe the model.
- The original migration is accompanied by the additive
  [legacy backfill repair](../../server/internal/dbmigrate/sql/202604280010_repair_pm_external_links_entity_backfill.sql).
  It fills valid UUID task references and deletes rows whose entity cannot be
  recovered from a null or invalid legacy task ID. The SQL sketch below is not a
  replacement migration or an instruction to edit applied checksums.
- [The service](../../server/internal/service/pm_external_link.go) supports task
  and epic entities and publishes the entity parent with link events. Generic
  handlers/routes and frontend entity query keys are present.
- [Epic creation](../../frontend/src/components/pm/GlobalCreateModals.tsx) creates
  external links after the epic. These writes are not atomic with creation; the
  loop catches thrown failures but does not inspect each returned API error.
- [Epic detail](../../frontend/src/pages/pm/EpicDetail.tsx) displays links in its
  Related section and handles file drops at the description. The planned
  panel-wide overlay and toggle layout are historical UI instructions.
- Absolute `/root/teampulse` paths, checkbox status, commit commands, and expected
  build output below are historical examples, not current validation results.

## Original implementation plan

**Goal:** Bring Epics to parity with Tasks for external links and file attachments — generalize the external link backend to support multiple entity types, then wire external links and improved attachment UX into Epic creation and detail views.

**Architecture:** Backend gets `entity_type`/`entity_id` columns on `pm_external_links` with new generic API routes. Frontend `ExternalLinks` component becomes entity-aware. Epic creation modal gets pills below editor (matching Task). Epic detail gets toggle buttons, external links section, and drag-drop attachment overlay.

**Tech Stack:** Go (Chi, GORM, PostgreSQL), React, TypeScript, TanStack Query, shadcn/ui

**Spec:** `docs/specs/2026-04-28-epic-external-links-attachments-design.md`

---

## File Map

### Backend (create/modify)
| File | Responsibility |
|------|---------------|
| `server/internal/model/pm_external_link.go` (modify) | Add `EntityType`, `EntityID` fields |
| `server/internal/dbmigrate/sql/202604280001_external_links_entity_type.sql` (create) | Migration: add columns, backfill, indexes |
| `server/internal/repository/pm_external_link.go` (modify) | Add `ListByEntity`, `CountByEntity` methods |
| `server/internal/service/pm_external_link.go` (modify) | Add `ListByEntity`, `CreateForEntity`; fix `Create`/`Update`/`Delete` entity fields |
| `server/internal/service/pm_task.go` (modify) | Set `EntityType`/`EntityID` on inline external link creation |
| `server/internal/handler/pm_external_link.go` (modify) | Add `ListByEntity`, `CreateForEntity` handlers |
| `server/internal/router/router.go` (modify) | Add generic entity-link routes |

### Frontend (create/modify)
| File | Responsibility |
|------|---------------|
| `frontend/src/lib/pm-types/project.ts` (modify) | Add `entity_type`, `entity_id` to `ExternalLink` interface |
| `frontend/src/lib/services/pmExternalLinkService.ts` (modify) | Add `listByEntity`, `createForEntity` methods |
| `frontend/src/lib/queryKeys.ts` (modify) | Add `entityExternalLinks` key factory |
| `frontend/src/hooks/useRealtimeSync.ts` (modify) | Invalidate entity-generic external links query key |
| `frontend/src/components/pm/ExternalLinks.tsx` (modify) | Change props to `entityType`/`entityId` |
| `frontend/src/components/pm/TaskDetailPanel.tsx` (modify) | Update `<ExternalLinks>` props |
| `frontend/src/components/pm/GlobalCreateModals.tsx` (modify) | Move pills below editor, add External Links |
| `frontend/src/pages/pm/EpicDetail.tsx` (modify) | Add External Links section, drag-drop overlay, attachment hooks |

---

### Task 1: Backend — Model & Migration

**Files:**
- Modify: `server/internal/model/pm_external_link.go`
- Create: `server/internal/dbmigrate/sql/202604280001_external_links_entity_type.sql`

- [ ] **Step 1: Add EntityType and EntityID to the model**

In `server/internal/model/pm_external_link.go`, add two fields after `TaskID`:

```go
type PMExternalLink struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TaskID      *string   `json:"task_id,omitempty" gorm:"column:task_id;type:uuid;index"`
	EntityType  string    `json:"entity_type" gorm:"type:varchar(32);default:'task'"`
	EntityID    string    `json:"entity_id" gorm:"type:uuid;index:idx_ext_links_entity"`
	Title       string    `json:"title" gorm:"not null"`
	URL         string    `json:"url" gorm:"not null"`
	CreatedByID string    `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
```

Notes:
- `TaskID` is now `*string` (pointer) because it will be NULL for epic links. GORM tag removes `not null`.
- GORM tags for `EntityType`/`EntityID` omit `not null` — AutoMigrate adds columns as nullable. The NOT NULL constraints come from the dbmigrate migration after backfill.
- All code referencing `link.TaskID` must use pointer semantics (`&value` to assign, `*link.TaskID` to read).

- [ ] **Step 2: Create the migration file**

Create `server/internal/dbmigrate/sql/202604280001_external_links_entity_type.sql`:

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

No registration code needed — dbmigrate uses `//go:embed sql/*.sql` to pick up all files automatically.

- [ ] **Step 3: Verify Go build**

Run: `cd /root/teampulse/server && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add server/internal/model/pm_external_link.go server/internal/dbmigrate/sql/202604280001_external_links_entity_type.sql
git commit -m "feat: add entity_type/entity_id columns to pm_external_links model and migration"
```

---

### Task 2: Backend — Repository

**Files:**
- Modify: `server/internal/repository/pm_external_link.go`

- [ ] **Step 1: Add ListByEntity method**

Add after the existing `List` method in `server/internal/repository/pm_external_link.go`:

```go
// ListByEntity returns external links for any entity type.
func (r *PMExternalLinkRepository) ListByEntity(ctx context.Context, entityType, entityID string) ([]model.PMExternalLink, error) {
	var links []model.PMExternalLink
	if err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list external links by entity: %w", err)
	}
	return links, nil
}
```

- [ ] **Step 2: Add CountByEntity method**

Add after the existing `Count` method:

```go
// CountByEntity returns number of external links for any entity type.
func (r *PMExternalLinkRepository) CountByEntity(ctx context.Context, entityType, entityID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.PMExternalLink{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count external links by entity: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 3: Verify Go build**

Run: `cd /root/teampulse/server && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Commit**

```bash
git add server/internal/repository/pm_external_link.go
git commit -m "feat: add ListByEntity and CountByEntity to external link repository"
```

---

### Task 3: Backend — Service

**Files:**
- Modify: `server/internal/service/pm_external_link.go`
- Modify: `server/internal/service/pm_task.go`

- [ ] **Step 1: Add allowed entity types and new service methods**

In `server/internal/service/pm_external_link.go`, add a validation set and two new methods:

```go
// allowedExternalLinkEntityTypes defines which entity types support external links.
var allowedExternalLinkEntityTypes = map[string]bool{
	"task": true,
	"epic": true,
}
```

Add `ListByEntity`:

```go
// ListByEntity returns external links for any supported entity type.
func (s *PMExternalLinkService) ListByEntity(ctx context.Context, entityType, entityID string) ([]model.PMExternalLink, error) {
	if !allowedExternalLinkEntityTypes[entityType] {
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	return s.repo.ListByEntity(ctx, entityType, entityID)
}
```

Add `CreateForEntity`:

```go
// CreateForEntity creates an external link for any supported entity type.
func (s *PMExternalLinkService) CreateForEntity(ctx context.Context, entityType, entityID string, req model.CreateExternalLinkRequest, userID, workspaceID string) (*model.PMExternalLink, error) {
	if !allowedExternalLinkEntityTypes[entityType] {
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = deriveTitle(rawURL)
	}

	link := &model.PMExternalLink{
		EntityType:  entityType,
		EntityID:    entityID,
		Title:       title,
		URL:         rawURL,
		CreatedByID: userID,
	}
	// Backward compat: also set TaskID for task links
	if entityType == "task" {
		link.TaskID = &entityID
	}

	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{
		Action: "created", Entity: "external_link", EntityID: link.ID,
		WorkspaceID: workspaceID, ActorID: userID,
		ParentType: entityType, ParentID: entityID,
	})
	return link, nil
}
```

- [ ] **Step 2: Update existing Create to also set EntityType/EntityID**

In the existing `Create` method in `pm_external_link.go`, update the link construction to also populate the new fields:

```go
	link := &model.PMExternalLink{
		TaskID:      &storyID,
		EntityType:  "task",
		EntityID:    storyID,
		Title:       title,
		URL:         rawURL,
		CreatedByID: userID,
	}
```

- [ ] **Step 3: Update Update/Delete to use EntityType/EntityID for WS events**

In the `Update` method, change the WebSocket publish from:
```go
s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: link.TaskID})
```
to:
```go
s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: link.EntityType, ParentID: link.EntityID})
```

In the `Delete` method, make the same change:
```go
s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: link.EntityType, ParentID: link.EntityID})
```

- [ ] **Step 4: Update inline task creation in pm_task.go**

In `server/internal/service/pm_task.go`, find the external link creation loop (around line ~469) and add `EntityType`/`EntityID` to the link:

```go
			taskID := newTask.ID
			link := &model.PMExternalLink{
				TaskID:      &taskID,
				EntityType:  "task",
				EntityID:    newTask.ID,
				URL:         linkURL,
				Title:       title,
				CreatedByID: actorID,
			}
```

- [ ] **Step 5: Verify Go build**

Run: `cd /root/teampulse/server && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/pm_external_link.go server/internal/service/pm_task.go
git commit -m "feat: add entity-generic external link service methods and fix WS events"
```

---

### Task 4: Backend — Handler & Router

**Files:**
- Modify: `server/internal/handler/pm_external_link.go`
- Modify: `server/internal/router/router.go`

- [ ] **Step 1: Add ListByEntity handler**

In `server/internal/handler/pm_external_link.go`, add:

```go
// ListByEntity handles GET /api/pm/entity-links/{entity_type}/{entity_id}
func (h *PMExternalLinkHandler) ListByEntity(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entity_type")
	entityID := chi.URLParam(r, "entity_id")
	links, err := h.service.ListByEntity(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if links == nil {
		links = []model.PMExternalLink{}
	}
	writeJSON(w, http.StatusOK, links)
}
```

- [ ] **Step 2: Add CreateForEntity handler**

```go
// CreateForEntity handles POST /api/pm/entity-links/{entity_type}/{entity_id}
func (h *PMExternalLinkHandler) CreateForEntity(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entity_type")
	entityID := chi.URLParam(r, "entity_id")
	userID := middleware.GetUserID(r.Context())
	workspaceID := middleware.GetWorkspaceID(r.Context())
	var req model.CreateExternalLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	link, err := h.service.CreateForEntity(r.Context(), entityType, entityID, req, userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, link)
}
```

- [ ] **Step 3: Add routes to router**

In `server/internal/router/router.go`, add the new generic routes right after the existing external link routes (after line ~826):

```go
				// Generic entity external links — pm.read / pm.edit
				r.With(requirePerm(authorization.PermPMRead)).Get("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.ListByEntity)
				r.With(requirePerm(authorization.PermPMEdit)).Post("/entity-links/{entity_type}/{entity_id}", h.PMExternalLink.CreateForEntity)
```

- [ ] **Step 4: Verify Go build**

Run: `cd /root/teampulse/server && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/pm_external_link.go server/internal/router/router.go
git commit -m "feat: add generic entity external link routes and handlers"
```

---

### Task 5: Frontend — Service, Types, Query Keys, Realtime Sync

**Files:**
- Modify: `frontend/src/lib/pm-types/project.ts` (~line 1006)
- Modify: `frontend/src/lib/services/pmExternalLinkService.ts`
- Modify: `frontend/src/lib/queryKeys.ts` (~line 112)
- Modify: `frontend/src/hooks/useRealtimeSync.ts` (~line 516)

- [ ] **Step 1: Add entity_type and entity_id to ExternalLink interface**

In `frontend/src/lib/pm-types/project.ts`, update the `ExternalLink` interface:

```ts
export interface ExternalLink {
  id: string;
  task_id: string;
  entity_type: string;
  entity_id: string;
  title: string;
  url: string;
  created_by_id: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Add generic service methods**

In `frontend/src/lib/services/pmExternalLinkService.ts`, add two new methods:

```ts
  listByEntity: (workspaceId: string, entityType: string, entityId: string) =>
    api.get<ExternalLink[]>(`/pm/entity-links/${entityType}/${entityId}?${qs(workspaceId)}`),

  createForEntity: (workspaceId: string, entityType: string, entityId: string, payload: CreateExternalLinkRequest) =>
    api.post<ExternalLink>(`/pm/entity-links/${entityType}/${entityId}?${qs(workspaceId)}`, payload),
```

- [ ] **Step 3: Add entityExternalLinks query key**

In `frontend/src/lib/queryKeys.ts`, add after the existing `externalLinks` line (~112):

```ts
    entityExternalLinks: (wsId: string, entityType: string, entityId: string) =>
      ['pm', wsId, entityType, entityId, 'externalLinks'] as const,
```

- [ ] **Step 4: Update realtime sync to invalidate entity-generic key**

In `frontend/src/hooks/useRealtimeSync.ts`, find the `external_link` handler (~line 516):

```ts
      } else if (event.entity === 'external_link') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(workspaceId, event.parent_id) })
```

Add a second invalidation line:

```ts
      } else if (event.entity === 'external_link') {
        queryClient.invalidateQueries({ queryKey: queryKeys.pm.externalLinks(workspaceId, event.parent_id) })
        if (event.parent_type) {
          queryClient.invalidateQueries({ queryKey: queryKeys.pm.entityExternalLinks(workspaceId, event.parent_type, event.parent_id) })
        }
```

Note: The DOM event dispatch already handles this correctly — it dispatches `${event.parent_type}-child-updated` which will be `epic-child-updated` for epic links.

- [ ] **Step 5: Commit**

```bash
cd /root/teampulse/frontend
git add src/lib/pm-types/project.ts src/lib/services/pmExternalLinkService.ts src/lib/queryKeys.ts src/hooks/useRealtimeSync.ts
git commit -m "feat: add entity-generic external link service, types, and query keys"
```

---

### Task 6: Frontend — Generalize ExternalLinks Component

**Files:**
- Modify: `frontend/src/components/pm/ExternalLinks.tsx`
- Modify: `frontend/src/components/pm/TaskDetailPanel.tsx` (~line 1294)

- [ ] **Step 1: Update ExternalLinks props and internals**

In `frontend/src/components/pm/ExternalLinks.tsx`, change the props interface:

From:
```ts
interface ExternalLinksProps {
  workspaceId: string;
  taskId: string;
}
```

To:
```ts
interface ExternalLinksProps {
  workspaceId: string;
  entityType: 'task' | 'epic';
  entityId: string;
}
```

Update the component signature:
```ts
export function ExternalLinks({ workspaceId, entityType, entityId }: ExternalLinksProps) {
```

Update `reload` to use generic service:
```ts
  const reload = useCallback(async () => {
    const { data } = await pmExternalLinkService.listByEntity(workspaceId, entityType, entityId);
    setLinks(data ?? []);
  }, [workspaceId, entityType, entityId]);
```

Update `handleAdd` to use generic service:
```ts
    const { data, error } = await pmExternalLinkService.createForEntity(workspaceId, entityType, entityId, { url });
```

Update the WebSocket event listener to match on `entityId`:
```ts
  useEffect(() => {
    const handler = (e: Event) => {
      const d = (e as CustomEvent)?.detail;
      if (d?.parent_id === entityId && d?.entity === 'external_link') reload();
    };
    window.addEventListener('task-child-updated', handler);
    window.addEventListener('epic-child-updated', handler);
    return () => {
      window.removeEventListener('task-child-updated', handler);
      window.removeEventListener('epic-child-updated', handler);
    };
  }, [entityId, reload]);
```

- [ ] **Step 2: Update TaskDetailPanel ExternalLinks usage**

In `frontend/src/components/pm/TaskDetailPanel.tsx`, find the `<ExternalLinks>` usage (~line 1294-1296):

Change:
```tsx
<ExternalLinks workspaceId={workspaceId} taskId={taskDetail.task.id} />
```

To:
```tsx
<ExternalLinks workspaceId={workspaceId} entityType="task" entityId={taskDetail.task.id} />
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd /root/teampulse/frontend && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 4: Commit**

```bash
cd /root/teampulse/frontend
git add src/components/pm/ExternalLinks.tsx src/components/pm/TaskDetailPanel.tsx
git commit -m "feat: generalize ExternalLinks component to support any entity type"
```

---

### Task 7: Frontend — Epic Creation Modal (External Links + Pill Repositioning)

**Files:**
- Modify: `frontend/src/components/pm/GlobalCreateModals.tsx`

This task restructures the `GlobalCreateEpic` function to:
1. Move the toggle pill bar from above the editor to below it
2. Add External Links pill + inline editor
3. POST external links after epic creation

- [ ] **Step 1: Add imports and state**

In `GlobalCreateModals.tsx`, add to the imports at the top of the file:

Add `Link01Icon` and `LinkSquare01Icon as ExternalLinkIcon` and `PlusSignIcon` to the icon imports (if not already present).

In the `GlobalCreateEpic` function, add new state after the existing `showAttachments` state (~line 209):

```ts
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [epicExternalLinks, setEpicExternalLinks] = useState<{ url: string }[]>([]);
```

- [ ] **Step 2: Move toggle pills from above editor to below**

Find the current pill bar + editor layout (the `<div className="mt-4">` block). The current structure is:

```
<div className="mt-4">
  <div className="mb-2 flex flex-wrap items-center gap-2">
    {/* Attach Files pill */}
  </div>
  <TiptapEditor ... />
  {showAttachments && ( ... )}
</div>
```

Restructure to:

```
<div className="mt-4">
  <TiptapEditor ... />
  <div className="mt-3 flex flex-wrap items-center gap-2">
    {/* External Links pill */}
    {/* Attach Files pill */}
  </div>
  {showExternalLinks && ( ... )}
  {showAttachments && ( ... )}
</div>
```

The **External Links pill** (add before the Attach Files pill):

```tsx
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                      showExternalLinks
                        ? 'border-primary/30 bg-primary/10 text-primary'
                        : 'border-border/60 text-muted-foreground hover:bg-accent'
                    }`}
                    onClick={() => setShowExternalLinks((v) => !v)}
                  >
                    <Link01Icon className="h-3 w-3" />
                    External Links
                    {epicExternalLinks.length > 0 && (
                      <span className="text-[10px] opacity-70">({epicExternalLinks.length})</span>
                    )}
                  </button>
```

- [ ] **Step 3: Add External Links inline editor section**

Add the external links section right before the `{showAttachments && ...}` block. Copy the same UI pattern from `CreateTaskModal.tsx`:

```tsx
                {showExternalLinks && (
                  <div className="mt-3 shrink-0 rounded-lg border border-border/60 bg-card">
                    <div className="flex items-center justify-between px-4 py-2 border-b border-border/40">
                      <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                        <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                        External Links
                        {epicExternalLinks.length > 0 && (
                          <span className="text-xs text-muted-foreground font-normal">({epicExternalLinks.length})</span>
                        )}
                      </div>
                    </div>
                    <div className="px-4 py-2 space-y-1">
                      {epicExternalLinks.map((link, idx) => (
                        <div key={idx} className="group flex items-center gap-2">
                          <ExternalLinkIcon className="h-3 w-3 text-muted-foreground/40 shrink-0" />
                          <input
                            type="url"
                            value={link.url}
                            autoFocus={idx === epicExternalLinks.length - 1 && link.url === ''}
                            onChange={(e) => {
                              const next = [...epicExternalLinks];
                              next[idx] = { url: e.target.value };
                              setEpicExternalLinks(next);
                            }}
                            placeholder="https://..."
                            className="flex-1 bg-transparent text-sm py-1 outline-none placeholder:text-muted-foreground/50"
                          />
                          <button
                            type="button"
                            className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity cursor-pointer"
                            onClick={() => setEpicExternalLinks(epicExternalLinks.filter((_, i) => i !== idx))}
                          >
                            <Delete01Icon className="h-3 w-3" />
                          </button>
                        </div>
                      ))}
                      <button
                        type="button"
                        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1 cursor-pointer"
                        onClick={() => setEpicExternalLinks([...epicExternalLinks, { url: '' }])}
                      >
                        <PlusSignIcon className="h-3 w-3" />
                        Add link
                      </button>
                    </div>
                  </div>
                )}
```

- [ ] **Step 4: POST external links after epic creation**

In the `create` function, after the pending files upload loop and before the success toast, add:

```ts
      // Create external links after epic creation
      if (data?.epic) {
        const validLinks = epicExternalLinks.filter((l) => l.url.trim());
        for (const el of validLinks) {
          try {
            await pmExternalLinkService.createForEntity(workspaceId, 'epic', data.epic.id, { url: el.url.trim() });
          } catch {
            // Non-blocking — epic already created
          }
        }
      }
```

Also add `pmExternalLinkService` to the imports from `'@/lib/services/pmExternalLinkService'`.

- [ ] **Step 5: Reset external links state on close/create-more**

In `handleClose`, add:
```ts
    setEpicExternalLinks([]);
    setShowExternalLinks(false);
```

Update `hasUnsavedChanges` to include external links:
```ts
  const hasUnsavedChanges = name.trim() !== '' || description.trim() !== '' || pendingFiles.length > 0 || epicExternalLinks.some(l => l.url.trim());
```

- [ ] **Step 6: Verify frontend builds**

Run: `cd /root/teampulse/frontend && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 7: Commit**

```bash
cd /root/teampulse/frontend
git add src/components/pm/GlobalCreateModals.tsx
git commit -m "feat: add External Links and reposition pills in Epic creation modal"
```

---

### Task 8: Frontend — Epic Detail View (External Links + Attachment Improvements)

**Files:**
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`

This task adds:
1. External Links toggle button + section
2. Attach Files button in toolbar
3. `onFilePickerReady`/`onUploadReady` hooks on `<Attachments>`
4. Panel-wide drag-and-drop overlay

- [ ] **Step 1: Add imports**

In `EpicDetail.tsx`, add to imports:

```ts
import { ExternalLinks } from '@/components/pm/ExternalLinks';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
```

Add icons (if not already imported): `Link01Icon`, `Upload01Icon`, `AttachmentIcon`.

- [ ] **Step 2: Add state and refs**

In the component function, add:

```ts
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [panelDragging, setPanelDragging] = useState(false);
  const openFilePickerRef = useRef<(() => void) | null>(null);
  const uploadFilesRef = useRef<((files: FileList | File[]) => Promise<void>) | null>(null);
  const dragCounterRef = useRef(0);
```

- [ ] **Step 3: Auto-show external links if they exist**

After the epic data loads, check for existing external links:

```ts
  useEffect(() => {
    if (!workspaceId || !epic?.epic?.id) return;
    pmExternalLinkService.listByEntity(workspaceId, 'epic', epic.epic.id).then(({ data }) => {
      if (data && data.length > 0) setShowExternalLinks(true);
    });
  }, [workspaceId, epic?.epic?.id]);
```

- [ ] **Step 4: Add toggle buttons row**

After the description section and before the existing `<Attachments>` component (around the `<div className="mt-6">` that wraps `<Attachments>`), add an action bar:

```tsx
          {/* Action bar */}
          {canEdit && (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <button
                type="button"
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                  showExternalLinks
                    ? 'border-primary/30 bg-primary/10 text-primary'
                    : 'border-border/60 text-muted-foreground hover:bg-accent'
                }`}
                onClick={() => setShowExternalLinks((v) => !v)}
              >
                <Link01Icon className="h-3 w-3" />
                External Links
              </button>
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded-full border border-border/60 px-3 py-1 text-xs font-medium text-muted-foreground hover:bg-accent transition-colors cursor-pointer"
                onClick={() => openFilePickerRef.current?.()}
              >
                <AttachmentIcon className="h-3 w-3" />
                Attach Files
              </button>
            </div>
          )}
```

- [ ] **Step 5: Add External Links section**

Right after the action bar and before the `<Attachments>` div:

```tsx
          {showExternalLinks && (
            <div className="mt-4">
              <ExternalLinks workspaceId={workspaceId!} entityType="epic" entityId={epic.epic.id} />
            </div>
          )}
```

- [ ] **Step 6: Update Attachments component with integration hooks**

Change the existing `<Attachments>` usage from:

```tsx
            <Attachments
              workspaceId={workspaceId!}
              entityType="epic"
              entityId={epic.epic.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
            />
```

To:

```tsx
            <Attachments
              workspaceId={workspaceId!}
              entityType="epic"
              entityId={epic.epic.id}
              memberNameMap={assignableMemberNames}
              onDeleteAttachment={handleDescriptionAttachmentDelete}
              onFilePickerReady={(fn) => { openFilePickerRef.current = fn; }}
              onUploadReady={(fn) => { uploadFilesRef.current = fn; }}
            />
```

- [ ] **Step 7: Add drag-and-drop overlay**

Find the main content container div (the grid or scrollable area that wraps the epic content). Add drag event handlers and overlay.

On the container div, add:

```tsx
        onDragEnter={(e) => {
          e.preventDefault();
          dragCounterRef.current++;
          if (e.dataTransfer.types.includes('Files')) setPanelDragging(true);
        }}
        onDragOver={(e) => e.preventDefault()}
        onDragLeave={() => {
          dragCounterRef.current--;
          if (dragCounterRef.current === 0) setPanelDragging(false);
        }}
        onDrop={(e) => {
          e.preventDefault();
          dragCounterRef.current = 0;
          setPanelDragging(false);
          if (e.dataTransfer.files.length > 0) {
            uploadFilesRef.current?.(e.dataTransfer.files);
          }
        }}
```

Add the overlay inside that container (matching TaskDetailPanel pattern):

```tsx
        {panelDragging && (
          <div className="absolute inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
            <div className="flex flex-col items-center gap-2 rounded-xl border-2 border-dashed border-primary px-10 py-8">
              <Upload01Icon className="h-8 w-8 text-primary" />
              <p className="text-sm font-medium text-foreground">Drop files to attach</p>
              <p className="text-xs text-muted-foreground">Max 10MB per file</p>
            </div>
          </div>
        )}
```

Make sure the container has `className="relative ..."` so the absolute overlay positions correctly.

- [ ] **Step 8: Verify frontend builds**

Run: `cd /root/teampulse/frontend && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 9: Commit**

```bash
cd /root/teampulse/frontend
git add src/pages/pm/EpicDetail.tsx
git commit -m "feat: add External Links section and attachment improvements to Epic detail"
```

---

### Task 9: Verify & Build

- [ ] **Step 1: Full backend build**

Run: `cd /root/teampulse/server && go vet ./... && go build ./...`
Expected: No errors

- [ ] **Step 2: Full frontend build**

Run: `cd /root/teampulse && pnpm build`
Expected: Build succeeds

- [ ] **Step 3: Fix any build errors**

If there are type errors or build failures, fix them now.

- [ ] **Step 4: Final commit if needed**

```bash
git add -A
git commit -m "fix: resolve any remaining build issues"
```
