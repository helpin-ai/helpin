# Help Scout docs import implementation plan

This historical plan records the first Help Scout help-center importer. Use it to
understand source mapping and UI intent; current execution and content storage
have changed substantially from the original snippets.

## Source review — 2026-09-18

- [Start and retry](../../server/internal/service/docs_import.go) now launch a
  Temporal workflow and require the worker client plus a 32-byte encryption key.
  The request payload is encrypted in the job record. The proposed untracked
  goroutine execution is superseded by the
  [durable import migration](../../server/internal/dbmigrate/sql/202608100003_docs_import_temporal.sql).
- Article import saves canonical Tiptap JSON from the shared converter, retains
  source HTML/provenance, and records conversion warnings. `_markdown_source`
  envelopes and the old Markdown-only pipeline below are not current storage.
- Image failures retain original URLs and produce warnings. The current
  [image downloader](../../server/internal/helpscout/images.go) uses an HTTP
  client; these snippets do not prove URL isolation, download bounds, or complete
  image preservation. Imported documents can still depend on external image URLs.
- [Status/retry/redirect handlers](../../server/internal/handler/docs_import.go)
  pass a job ID without a workspace argument to their service lookups. The
  [repository](../../server/internal/repository/docs_import.go) fetches by raw ID;
  unlike Cancel, these inspected paths do not compare job workspace. Route-level
  `docs.import` permission alone must not be described as object-level isolation.
- The redirect map is a JSON download (`redirect-map.json`), not CSV. The
  [frontend import section](../../frontend/src/components/settings/HelpCenterImportSection.tsx)
  polls progress and downloads the map through the API service; list/cancel and
  reconversion endpoints also exist.
- `server/migrations/047_docs_import.sql`, Go 1.24, and the filtered TypeScript
  command below are historical implementation references. New schema work uses
  versioned SQL under `internal/dbmigrate/sql`. A filtered compiler-output pipe
  cannot establish a passing frontend build. No import, credentials, migration,
  or external Help Scout API was exercised in this review.

## Original implementation plan

**Goal:** Import help center articles from HelpScout into Helpin's Docs module with progress tracking, image re-upload, slug preservation, and retry.

**Architecture:** Go backend fetches from HelpScout Docs API, converts HTML→Markdown, re-uploads images to S3, creates docs via existing services. Background goroutine (matching PM import pattern) tracks progress in a DB table. Frontend adds a Help Center section to the existing Import/Export settings tab.

**Tech Stack:** Go 1.24, Chi router, GORM/PostgreSQL, `html-to-markdown` Go library, React/TypeScript frontend

**Spec:** `docs/specs/2026-03-18-helpscout-docs-import-design.md`

---

## File Structure

### Backend — New Files

| File | Responsibility |
|------|----------------|
| `server/internal/helpscout/client.go` | HelpScout Docs API HTTP client (auth, pagination, rate limits) |
| `server/internal/helpscout/types.go` | HelpScout API response structs |
| `server/internal/helpscout/convert.go` | HTML → Markdown conversion with edge case handling |
| `server/internal/helpscout/images.go` | Image download from CDN + S3 re-upload + URL replacement |
| `server/internal/model/docs_import.go` | `DocsImportJob` GORM model + request/response DTOs |
| `server/internal/repository/docs_import.go` | Import job CRUD (create, update progress, get, list) |
| `server/internal/service/docs_import.go` | Import orchestration, preview, background job, retry |
| `server/internal/handler/docs_import.go` | Import HTTP handlers on `DocsHandler` (preview, start, status, retry, redirect map) |
| `server/migrations/047_docs_import.sql` | Schema: `docs_import_jobs` table + `DocsCollection.slug` column |

### Backend — Modified Files

| File | Change |
|------|--------|
| `server/internal/model/docs.go` | Add `Slug` field to `DocsCollection`, update `CreateDocsCollectionRequest` |
| `server/internal/service/docs_collection.go` | Accept optional `slug` in `Create()` |
| `server/internal/repository/docs_collection.go` | Persist slug on create |
| `server/internal/authorization/permissions.go` | Add `PermDocsImport` |
| `server/internal/authorization/rbac.go` | Grant `docs.import` to admin + owner |
| `server/internal/router/router.go` | Register import routes |
| `server/cmd/api/main.go` | Wire `DocsImportService` + `DocsImportHandler`, register `DocsImportJob` in AutoMigrate |

### Frontend — New Files

| File | Responsibility |
|------|----------------|
| `frontend/src/components/settings/HelpCenterImportSection.tsx` | Connect + Configure + Progress UI |
| `frontend/src/lib/services/docsImportService.ts` | API service (preview, start, status, retry, redirect map) |

### Frontend — Modified Files

| File | Change |
|------|--------|
| `frontend/src/components/settings/ImportTab.tsx` | Add "Help Center" section header + render `HelpCenterImportSection` |
| `frontend/src/lib/types.ts` | Add `'docs.import'` to `Permission` union type |

---

## Task 1: Database Migration + Model

**Files:**
- Create: `server/migrations/047_docs_import.sql`
- Create: `server/internal/model/docs_import.go`
- Modify: `server/internal/model/docs.go`

- [ ] **Step 1: Write the migration file**

```sql
-- server/migrations/047_docs_import.sql

-- Import job tracking
CREATE TABLE IF NOT EXISTS docs_import_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  space_id UUID,
  source TEXT NOT NULL DEFAULT 'helpscout',
  status TEXT NOT NULL DEFAULT 'pending',
  total INT NOT NULL DEFAULT 0,
  completed INT NOT NULL DEFAULT 0,
  failed INT NOT NULL DEFAULT 0,
  failures JSONB NOT NULL DEFAULT '[]',
  config JSONB NOT NULL DEFAULT '{}',
  redirect_map JSONB,
  error TEXT,
  started_by UUID NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_docs_import_jobs_ws ON docs_import_jobs (workspace_id);

-- Add slug to docs_collections
ALTER TABLE docs_collections ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_space_slug
  ON docs_collections (space_id, slug) WHERE slug != '' AND deleted_at IS NULL;
```

- [ ] **Step 2: Create DocsImportJob model**

Create `server/internal/model/docs_import.go` with:
- `DocsImportJob` struct (mirrors the table above, with GORM tags)
- `TableName()` → `"docs_import_jobs"`
- `ImportFailure` struct: `{ ArticleID string, Title string, Error string }`
- `DocsImportPreviewRequest` DTO: `{ APIKey string }`
- `DocsImportStartRequest` DTO: `{ APIKey, HelpscoutCollectionID, TargetSpaceID, NewSpaceName, ImportStatus string }`
- `DocsImportPreviewResponse` DTO: `{ Collections []HelpscoutCollectionPreview }`
- `HelpscoutCollectionPreview`: `{ ID, Name, Slug string; CategoryCount, ArticleCount int }`
- Status constants: `DocsImportStatusPending`, `DocsImportStatusRunning`, `DocsImportStatusDone`, `DocsImportStatusFailed`, `DocsImportStatusInterrupted`

- [ ] **Step 3: Add Slug field to DocsCollection**

In `server/internal/model/docs.go`, add to `DocsCollection` struct:
```go
Slug string `json:"slug" gorm:"not null;default:''"`
```

Add `Slug *string` to `CreateDocsCollectionRequest`.

- [ ] **Step 4: Register model in AutoMigrate**

In `server/cmd/api/main.go`, add `&model.DocsImportJob{}` to the AutoMigrate call.

- [ ] **Step 5: Build and verify**

Run: `cd server && go build ./...`
Expected: compiles clean

- [ ] **Step 6: Commit**

```
feat: add docs import job model and collection slug migration
```

---

## Task 2: Import Job Repository

**Files:**
- Create: `server/internal/repository/docs_import.go`

- [ ] **Step 1: Write the repository**

Create `server/internal/repository/docs_import.go` with:

```go
type DocsImportRepository struct {
    db *gorm.DB
}

func NewDocsImportRepository(db *gorm.DB) *DocsImportRepository

func (r *DocsImportRepository) Create(ctx, job *model.DocsImportJob) error
func (r *DocsImportRepository) GetByID(ctx, id string) (*model.DocsImportJob, error)
func (r *DocsImportRepository) UpdateProgress(ctx, id string, completed, failed int, failures []model.ImportFailure) error
func (r *DocsImportRepository) UpdateStatus(ctx, id, status string, completedAt *time.Time) error
func (r *DocsImportRepository) SetRedirectMap(ctx, id string, redirectMap json.RawMessage) error
func (r *DocsImportRepository) SetError(ctx, id, errMsg string) error
```

Follow existing repository patterns: `db.WithContext(ctx)`, return `nil` for not-found, `fmt.Errorf` with context for errors.

- [ ] **Step 2: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 3: Commit**

```
feat: add docs import job repository
```

---

## Task 3: Collection Slug Support

**Files:**
- Modify: `server/internal/service/docs_collection.go`
- Modify: `server/internal/repository/docs_collection.go`

- [ ] **Step 1: Update collection repository to persist slug**

In `docs_collection.go` repository, update the `Create` method to set `Slug` from the request if provided.

- [ ] **Step 2: Update collection service to pass slug**

In `docs_collection.go` service, pass `req.Slug` through to the repository in `Create()`.

- [ ] **Step 3: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 4: Commit**

```
feat: support optional slug on docs collection creation
```

---

## Task 4: Authorization — Add `docs.import` Permission

**Files:**
- Modify: `server/internal/authorization/permissions.go`
- Modify: `server/internal/authorization/rbac.go`

- [ ] **Step 1: Add permission constant**

In `permissions.go`, add:
```go
PermDocsImport Permission = "docs.import"
```

- [ ] **Step 2: Grant to admin and owner roles**

In `rbac.go`, add `PermDocsImport` to the `admin` and `owner` permission sets.

- [ ] **Step 3: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 4: Commit**

```
feat: add docs.import permission for admin/owner
```

---

## Task 5: HelpScout API Client

**Files:**
- Create: `server/internal/helpscout/types.go`
- Create: `server/internal/helpscout/client.go`

- [ ] **Step 1: Define HelpScout API response types**

Create `server/internal/helpscout/types.go`:
- `Collection` struct: `ID, Number, Slug, Name, Visibility, Order, CreatedAt, UpdatedAt`
- `Category` struct: `ID, Number, Slug, Name, CollectionID, Order, CreatedAt, UpdatedAt`
- `ArticleRef` struct: `ID, Number, Slug, Name, Status, HasDraft, CollectionID, Categories[]`
- `Article` struct: extends `ArticleRef` with `Text` (HTML body)
- `PaginatedResponse[T]` generic: `Items []T, Page, Pages int`

- [ ] **Step 2: Implement the HTTP client**

Create `server/internal/helpscout/client.go`:
- `Client` struct with `apiKey`, `httpClient *http.Client`, `logger *slog.Logger`
- `NewClient(apiKey string) *Client` — creates client with shared `http.Client`
- `ListCollections(ctx)` — `GET /v1/collections`, auto-paginate
- `ListCategories(ctx, collectionID)` — `GET /v1/collections/{id}/categories`, auto-paginate
- `ListArticles(ctx, collectionID)` — `GET /v1/collections/{id}/articles`, auto-paginate
- `GetArticle(ctx, articleID)` — `GET /v1/articles/{id}`, single fetch
- Private `doRequest(ctx, method, path, result)` — handles auth (Basic), rate limit headers (`X-RateLimit-Remaining`), HTTP 429 backoff, JSON decode
- Private `doRequestPaginated[T](ctx, path)` — wraps `doRequest` with page iteration

Auth: HTTP Basic with `apiKey` as username, empty password.
Rate limits: Read `X-RateLimit-Remaining`; if < 50, sleep 2s. On 429, sleep `Retry-After` seconds.

- [ ] **Step 3: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 4: Commit**

```
feat: add HelpScout Docs API client
```

---

## Task 6: HTML → Markdown Converter

**Files:**
- Create: `server/internal/helpscout/convert.go`
- Create: `server/internal/helpscout/convert_test.go`

- [ ] **Step 1: Add `html-to-markdown` dependency**

Run: `cd server && go get github.com/JohannesKaufmann/html-to-markdown/v2`

- [ ] **Step 2: Write tests for conversion edge cases**

Create `server/internal/helpscout/convert_test.go` with table-driven tests:
- Basic HTML (headings, paragraphs, bold, italic, links) → clean Markdown
- `<table>` → Markdown pipe table
- `<div class="callout callout-info">` → `> **Note:** ...`
- `<div class="callout callout-warn">` → `> **Warning:** ...`
- `<iframe src="youtube.com/...">` → plain link
- `<figure><img><figcaption>` → image + italic caption
- Inline `style` attributes → stripped
- Empty/nil HTML → empty string

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd server && go test ./internal/helpscout/ -v -run TestConvert`

- [ ] **Step 4: Implement the converter**

Create `server/internal/helpscout/convert.go`:
- `ConvertHTML(html string) (string, error)` — public entry point
- Uses `html-to-markdown` library with custom rules:
  - Before conversion: regex/goquery to transform callout divs → blockquotes, iframes → links
  - Configure converter with table support enabled
  - Strip all `style` attributes
  - After conversion: clean up excess whitespace

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd server && go test ./internal/helpscout/ -v -run TestConvert`

- [ ] **Step 6: Commit**

```
feat: add HTML to Markdown converter for HelpScout import
```

---

## Task 7: Image Processor

**Files:**
- Create: `server/internal/helpscout/images.go`
- Create: `server/internal/helpscout/images_test.go`

- [ ] **Step 1: Write tests**

Test `ExtractImageURLs(html)` returns list of image URLs found in `<img>` tags.
Test `ReplaceImageURLs(html, urlMap)` replaces src attributes.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd server && go test ./internal/helpscout/ -v -run TestImage`

- [ ] **Step 3: Implement image processing**

Create `server/internal/helpscout/images.go`:
- `ExtractImageURLs(html string) []string` — parse HTML, find `<img src="...">`, return unique URLs
- `ReplaceImageURLs(html string, urlMap map[string]string) string` — replace old URLs with new
- `ProcessImages(ctx, html string, s3Client *storage.S3Client, workspaceID string) (string, error)` — orchestrator:
  1. Extract image URLs
  2. For each URL: download via HTTP GET, upload to S3 (`PutObject`), get public URL
  3. Replace all URLs in HTML
  4. Return modified HTML

On image download failure: log warning, skip that image (keep original URL).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd server && go test ./internal/helpscout/ -v -run TestImage`

- [ ] **Step 5: Commit**

```
feat: add image processor for HelpScout import
```

---

## Task 8: Import Service

**Files:**
- Create: `server/internal/service/docs_import.go`

- [ ] **Step 1: Create the import service**

Create `server/internal/service/docs_import.go`:

```go
type DocsImportService struct {
    importRepo    *repository.DocsImportRepository
    spaceSvc      *DocsSpaceService
    collectionSvc *DocsCollectionService
    documentSvc   *DocsDocumentService
    contentSvc    *DocsContentService
    helpcenterSvc *DocsHelpcenterService
    s3Client      *storage.S3Client
    logger        *slog.Logger
}

func NewDocsImportService(...) *DocsImportService
```

- [ ] **Step 2: Implement Preview**

```go
func (s *DocsImportService) Preview(ctx context.Context, apiKey string) (*model.DocsImportPreviewResponse, error)
```
- Create HelpScout client with provided API key
- Fetch collections, for each fetch categories and article count
- Return structured preview response
- On auth failure (401): return clear error "Invalid API key"

- [ ] **Step 3: Implement Start**

```go
func (s *DocsImportService) Start(ctx context.Context, req model.DocsImportStartRequest, userID string) (string, error)
```
- Validate request (collection ID required, space or new name required)
- If `NewSpaceName` provided: create new space via `spaceSvc.Create(workspaceID, req, userID)` — `userID` is passed from the handler
- Create `DocsImportJob` record in DB (status: pending)
- Launch background goroutine with `defer/recover` to catch panics (matches PM import pattern):
  ```go
  go func() {
      defer func() {
          if r := recover(); r != nil {
              s.importRepo.SetError(context.Background(), jobID, fmt.Sprintf("panic: %v", r))
              s.importRepo.UpdateStatus(context.Background(), jobID, model.DocsImportStatusFailed, timePtr(time.Now()))
          }
      }()
      s.runImport(jobID, apiKey, req)
  }()
  ```
- Return job ID

- [ ] **Step 4: Implement runImport (background job)**

```go
func (s *DocsImportService) runImport(jobID, apiKey string, req model.DocsImportStartRequest)
```
- Create HelpScout client
- Update job status to `running`
- Fetch categories → create DocsCollection for each (with slug)
- Fetch article list → count total, update job
- For each article:
  - `GetArticle(id)` (with draft param if match_source mode)
  - `ProcessImages()` — download/re-upload images
  - `ConvertHTML()` — HTML to Markdown
  - Create DocsDocument via `documentSvc.Create()`
  - Save content via `contentSvc.Save()` — wrap markdown in JSON envelope: `json.Marshal(map[string]string{"_markdown_source": markdown})`, then pass raw bytes to `Save()`
  - Create DocsHelpcenterArticle via `helpcenterSvc` with original slug
  - If import_status is "published" or "match_source" and article was published: publish document
  - Update job progress (batch: every 5 articles or on failure)
  - On failure: append to failures list, continue
- Build redirect map: `[{old_url, new_slug}]`
- Store redirect map in job, mark as done
- On panic/unhandled error: mark job as failed with error message

- [ ] **Step 5: Implement GetStatus and Retry**

```go
func (s *DocsImportService) GetStatus(ctx context.Context, jobID string) (*model.DocsImportJob, error)
func (s *DocsImportService) Retry(ctx context.Context, jobID, apiKey string) error
func (s *DocsImportService) GetRedirectMap(ctx context.Context, jobID string) (json.RawMessage, error)
```
- `GetStatus`: simple DB read
- `Retry`: load job, re-process only failed articles, launch new goroutine
- `GetRedirectMap`: load job, return redirect_map field

- [ ] **Step 6: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 7: Commit**

```
feat: add docs import service with HelpScout support
```

---

## Task 9: Import Handler + Routes

**Files:**
- Create: `server/internal/handler/docs_import.go`
- Modify: `server/internal/handler/docs.go` (add `importService` field to `DocsHandler`)
- Modify: `server/internal/router/router.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Add import service to DocsHandler**

The codebase uses a single `DocsHandler` for all docs operations. Add `importService *service.DocsImportService` field to the existing `DocsHandler` struct and update its constructor `NewDocsHandler(...)`.

- [ ] **Step 2: Create import handler methods**

Create `server/internal/handler/docs_import.go` with methods on `*DocsHandler`:
- `ImportPreviewHelpscout(w, r)` — decode `DocsImportPreviewRequest`, call `importService.Preview()`, return JSON
- `ImportStartHelpscout(w, r)` — decode `DocsImportStartRequest`, extract `userID` from context, call `importService.Start()`, return `{ job_id }` with 202 Accepted
- `ImportGetStatus(w, r)` — read `jobId` URL param, call `importService.GetStatus()`, return JSON
- `ImportRetry(w, r)` — read `jobId` + API key from body, call `importService.Retry()`, return status
- `ImportGetRedirectMap(w, r)` — read `jobId`, call `importService.GetRedirectMap()`, return CSV with `Content-Type: text/csv` and `Content-Disposition: attachment` headers

- [ ] **Step 2: Register routes**

In `server/internal/router/router.go`, add under the workspace-scoped docs group:
```go
// Docs import
r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/preview", h.Docs.ImportPreviewHelpscout)
r.With(requirePerm(authorization.PermDocsImport)).Post("/import/helpscout/start", h.Docs.ImportStartHelpscout)
r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/status", h.Docs.ImportGetStatus)
r.With(requirePerm(authorization.PermDocsImport)).Post("/import/{jobId}/retry", h.Docs.ImportRetry)
r.With(requirePerm(authorization.PermDocsImport)).Get("/import/{jobId}/redirect-map", h.Docs.ImportGetRedirectMap)
```

- [ ] **Step 4: Wire DI in main.go**

In `server/cmd/api/main.go`:
- Create `docsImportRepo := repository.NewDocsImportRepository(db)`
- Create `docsImportService := service.NewDocsImportService(docsImportRepo, docsSpaceService, docsCollectionService, docsDocumentService, docsContentService, docsHelpcenterService, s3Client)`
- Pass `docsImportService` to `handler.NewDocsHandler(...)` (updated constructor)

- [ ] **Step 5: Build and verify**

Run: `cd server && go build ./...`

- [ ] **Step 6: Commit**

```
feat: add docs import handler and routes
```

---

## Task 10: Frontend API Service

**Files:**
- Create: `frontend/src/lib/services/docsImportService.ts`

- [ ] **Step 1: Create the API service**

```typescript
import { api } from '../api';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export interface HelpscoutCollectionPreview {
  id: string;
  name: string;
  slug: string;
  category_count: number;
  article_count: number;
}

export interface ImportPreviewResponse {
  collections: HelpscoutCollectionPreview[];
}

export interface ImportStatusResponse {
  id: string;
  status: 'pending' | 'running' | 'done' | 'failed';
  total: number;
  completed: number;
  failed: number;
  failures: { article_id: string; title: string; error: string }[];
  redirect_map_available: boolean;
}

export const docsImportService = {
  previewHelpscout: (workspaceId: string, apiKey: string) =>
    api.post<ImportPreviewResponse>(`/docs/import/helpscout/preview${qs(workspaceId)}`, { api_key: apiKey }),

  startHelpscout: (workspaceId: string, data: {
    api_key: string;
    helpscout_collection_id: string;
    target_space_id?: string;
    new_space_name?: string;
    import_status: 'draft' | 'published' | 'match_source';
  }) => api.post<{ job_id: string }>(`/docs/import/helpscout/start${qs(workspaceId)}`, data),

  getStatus: (workspaceId: string, jobId: string) =>
    api.get<ImportStatusResponse>(`/docs/import/${jobId}/status${qs(workspaceId)}`),

  retry: (workspaceId: string, jobId: string, apiKey: string) =>
    api.post(`/docs/import/${jobId}/retry${qs(workspaceId)}`, { api_key: apiKey }),

  getRedirectMapUrl: (workspaceId: string, jobId: string) =>
    `/api/docs/import/${jobId}/redirect-map?workspace_id=${encodeURIComponent(workspaceId)}`,
};
```

- [ ] **Step 2: Commit**

```
feat: add docs import frontend API service
```

---

## Task 11: Frontend Import UI

**Files:**
- Create: `frontend/src/components/settings/HelpCenterImportSection.tsx`
- Modify: `frontend/src/components/settings/ImportTab.tsx`

- [ ] **Step 1: Create HelpCenterImportSection component**

Create `frontend/src/components/settings/HelpCenterImportSection.tsx`:

Three internal states: `'connect' | 'configure' | 'progress'`

**Connect state:**
- Source dropdown (just "HelpScout" for now, extensible)
- API key password input
- "Connect" button → calls `previewHelpscout()`
- On success → stores preview data, transitions to configure
- On error → toast with "Invalid API key" or error message

**Configure state:**
- Shows preview: "Found X categories, Y articles in [Collection]"
- If multiple collections: dropdown to pick one
- Target space: Select existing from `useDocsSpaces()` or "Create new" with name input
- Import status: radio group with cards (same pattern as invite role picker):
  - "Import as draft" — all articles imported as draft
  - "Import as published" — all articles imported as published
  - "Match source status" — preserves original status
- "Start Import" button → calls `startHelpscout()`, transitions to progress

**Progress state:**
- Progress bar using shadcn `Progress` component
- Text: "Importing... X of Y articles"
- Polls `getStatus()` every 2 seconds via `setInterval`
- On done: shows summary "X succeeded, Y failed"
- If failures: list with article titles + errors, "Retry" button
- "Download Redirect Map" link (uses `getRedirectMapUrl()` with auth token)
- "Go to Space" link

- [ ] **Step 2: Add `docs.import` to frontend Permission type**

In `frontend/src/lib/types.ts`, add `| 'docs.import'` to the `Permission` union type.

- [ ] **Step 3: Update ImportTab to include Help Center section**

In `frontend/src/components/settings/ImportTab.tsx`:
- Add section header "Project Management" above existing Shortcut import
- Add section header "Help Center" below
- Render `<HelpCenterImportSection workspaceId={workspaceId} editable={editable} />`
- Import the new component

- [ ] **Step 4: Verify no TS errors**

Run: `npx tsc --noEmit 2>&1 | grep -E "ImportTab|HelpCenter"`

- [ ] **Step 5: Commit**

```
feat: add Help Center import UI to settings
```

---

## Task 12: Integration Testing

- [ ] **Step 1: Test the full backend flow manually**

1. Start the server: `cd server && go run ./cmd/api`
2. Call preview endpoint with a real HelpScout API key
3. Call start endpoint with a valid collection ID
4. Poll status until done
5. Verify: documents created in docs_documents, content in docs_contents, slugs in docs_helpcenter_articles, images in S3

- [ ] **Step 2: Verify frontend flow**

1. Go to Settings → Import / Export
2. See "Help Center" section
3. Enter HelpScout API key, click Connect
4. Select collection, configure options, click Import
5. Watch progress bar
6. Verify imported docs appear in the Docs module

- [ ] **Step 3: Test error scenarios**

- Invalid API key → clear error toast
- Rate limit hit → import continues (slower)
- Image download failure → article still imports with original URLs
- Network interruption → failed articles listed, retry works

- [ ] **Step 4: Final commit**

```
feat: complete HelpScout docs import feature
```
