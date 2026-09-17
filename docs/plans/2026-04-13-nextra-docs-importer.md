# Nextra Docs Importer Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a one-time Nextra repository zip importer that previews and imports Nextra docs into Helpin external docs spaces, collections, articles, assets, redirects, and import reports with high fidelity.

**Architecture:** Generalize the existing HelpScout docs importer into a source-adapter pipeline: source zip -> Nextra adapter -> normalized import plan -> preview -> shared executor. Keep Git sync out of scope, but store source provenance so future re-import/sync is possible without redesigning the importer.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL, S3/MinIO, React 19, TypeScript 5.9, TanStack Query, shadcn/ui, existing TipTap/Goldmark docs conversion.

---

## Decision Lock-Ins

- Import source is a manually uploaded `.zip` of the Nextra repo. Do not build GitHub OAuth, GitHub App, webhooks, or Git sync in this plan.
- Helpin becomes the source of truth after import. Re-import/sync is not part of v1.
- The importer must not execute user-provided JavaScript/TypeScript. `_meta.js` / `_meta.ts` support is static parsing only; dynamic metadata is warned and partially skipped.
- The importer must be source-agnostic below the adapter layer. Nextra is the first adapter, but the normalized model must also fit Mintlify/Fumadocs later.
- Preview is mandatory before import. Users must see tree shape, article count, unsupported MDX components, broken links, image issues, slug conflicts, and redirects before committing.
- Import creates redirects from original Nextra routes to Helpin public article routes.
- Import stores source provenance: `source_system=nextra`, `source_path`, `source_route`, optional user-provided `source_commit`, and `import_job_id`.
- Unknown/custom MDX components are not silently dropped. They become sanitized HTML blocks or visible unsupported-component placeholders and are listed in warnings.
- Existing Helpin docs remain unchanged until the user confirms start. Preview may create a pending import job and upload archive, but it must not create spaces/collections/documents.
- Nextra top-level docs groups default to one target external-capable Helpin space. The preview UI may offer “split top-level folders into spaces,” but v1 execution should support one target space first.

## Current-State Notes

- Current import UI only exposes HelpScout in [frontend/src/components/settings/ImportTab.tsx](/root/teampulse/frontend/src/components/settings/ImportTab.tsx).
- Current backend routes are HelpScout-specific in [server/internal/handler/docs_import.go](/root/teampulse/server/internal/handler/docs_import.go).
- Current import service is HelpScout-specific in [server/internal/service/docs_import.go](/root/teampulse/server/internal/service/docs_import.go).
- Existing `docs_import_jobs.source` is a free string and can store `nextra` without a migration.
- Existing `docs_import_jobs.config`, `summary`, `redirect_map`, and `failures` can carry Nextra-specific metadata and reports.
- Existing `server/internal/docsimport/collection_tree_mapping.go` already has generic source-group-to-collection-tree mapping with a depth cap.
- Existing HTML converter in [server/internal/docsimport/html_to_tiptap.go](/root/teampulse/server/internal/docsimport/html_to_tiptap.go) supports common HTML, tables, code, callouts, images, and HTML block fallback.
- Existing Markdown converter in [server/internal/tiptap/markdown.go](/root/teampulse/server/internal/tiptap/markdown.go) supports GFM but does not understand MDX JSX or produce import warnings.

## File Structure

### Backend

- Create: `server/internal/docsimport/import_plan.go`
  - Normalized import DTOs shared by Nextra and future adapters.
- Create: `server/internal/docsimport/nextra_archive.go`
  - Zip validation, safe extraction, Nextra root detection.
- Create: `server/internal/docsimport/nextra_adapter.go`
  - Nextra source adapter: scans files, parses frontmatter/meta, builds `ImportPlan`.
- Create: `server/internal/docsimport/nextra_meta.go`
  - Static `_meta.json` and simple `_meta.js` / `_meta.ts` parser.
- Create: `server/internal/docsimport/nextra_mdx.go`
  - MDX preprocessing, known component mapping, warnings.
- Create: `server/internal/docsimport/nextra_assets.go`
  - Relative asset discovery and path rewrite planning.
- Create: `server/internal/docsimport/nextra_links.go`
  - Nextra route derivation and internal link rewrite planning.
- Create: `server/internal/docsimport/nextra_adapter_test.go`
- Create: `server/internal/docsimport/nextra_meta_test.go`
- Create: `server/internal/docsimport/nextra_mdx_test.go`
- Create: `server/internal/docsimport/nextra_assets_test.go`
- Create: `server/internal/docsimport/nextra_links_test.go`
- Modify: `server/internal/model/docs_import.go`
  - Add generic Nextra preview/start request/response DTOs and warning/report models.
- Modify: `server/internal/service/docs_import.go`
  - Add source-agnostic preview/start executor for Nextra while leaving HelpScout routes working.
- Create: `server/internal/service/docs_import_nextra_test.go`
- Modify: `server/internal/handler/docs_import.go`
  - Add multipart Nextra preview/start handlers.
- Modify: `server/internal/router/router.go`
  - Register Nextra import endpoints.
- Modify: `server/cmd/api/main.go`
  - No new dependencies expected unless constructor wiring changes.

### Frontend

- Modify: `frontend/src/components/settings/ImportTab.tsx`
  - Add Nextra card under Help Center.
- Create: `frontend/src/components/settings/NextraImportWizard.tsx`
  - Zip upload, preview display, target space selection, import confirmation, progress link.
- Modify: `frontend/src/lib/services/docsService.ts`
  - Add Nextra preview/start API methods.
- Modify: `frontend/src/lib/types.ts` or docs-specific types file if present
  - Add Nextra import request/response DTOs.
- Modify: `frontend/src/hooks/queries/useDocs.ts`
  - Add mutation hooks for Nextra preview/start if docs import hooks already live there.
- Create: `frontend/src/components/settings/__tests__/NextraImportWizard.test.tsx`
- Create: `frontend/src/lib/__tests__/nextraImportTypes.test.ts` if helper functions are added.

## API Shape

### Preview

`POST /api/docs/import/nextra/preview?workspace_id=...`

Content type: `multipart/form-data`

Fields:
- `archive`: `.zip`
- `source_commit`: optional user-provided commit/version label

Response:

```json
{
  "job_id": "uuid",
  "archive_name": "usermaven-docs.zip",
  "source_commit": "abc123",
  "detected_root": "apps/docs",
  "spaces": [
    {
      "source_id": "default",
      "name": "Usermaven Docs",
      "collection_count": 12,
      "article_count": 84
    }
  ],
  "collections": 12,
  "articles": 84,
  "assets": 120,
  "redirects": 84,
  "warnings": [
    {
      "type": "unsupported_mdx_component",
      "source_path": "content/getting-started.mdx",
      "message": "Component <CustomVideo> will be imported as an HTML block"
    }
  ],
  "broken_links": [],
  "unsupported_components": [
    { "name": "CustomVideo", "count": 4 }
  ]
}
```

### Start

`POST /api/docs/import/nextra/start?workspace_id=...`

Body:

```json
{
  "job_id": "uuid",
  "target_space_id": "uuid-or-null",
  "new_space_name": "Usermaven Docs",
  "import_status": "draft|published",
  "create_redirects": true
}
```

Response:

```json
{ "job_id": "uuid" }
```

## Normalized Import Model

Create a shared normalized model in `server/internal/docsimport/import_plan.go`:

```go
type ImportPlan struct {
    SourceSystem string
    SourceCommit string
    RootPath string
    Spaces []ImportSpace
    Collections []ImportCollection
    Articles []ImportArticle
    Assets []ImportAsset
    Redirects []ImportRedirect
    Warnings []Warning
}

type ImportSpace struct {
    SourceID string
    Name string
    Slug string
}

type ImportCollection struct {
    SourceID string
    ParentSourceID string
    SpaceSourceID string
    Name string
    Slug string
    Position int
    Hidden bool
    SourcePath string
    SourceRoute string
}

type ImportArticle struct {
    SourceID string
    CollectionSourceID string
    SpaceSourceID string
    Title string
    Slug string
    Description string
    Position int
    Hidden bool
    SourcePath string
    SourceRoute string
    Frontmatter map[string]any
    RawContent string
    RewrittenContent string
    UnsupportedComponents []string
}

type ImportAsset struct {
    SourcePath string
    ReferencedBy string
    ContentType string
    Size int64
}

type ImportRedirect struct {
    SourcePath string
    TargetCollectionSlug string
    TargetArticleSlug string
}
```

Keep this model free of Helpin DB IDs. The executor resolves source IDs to created Helpin IDs.

### Public ID / Canonical URL Rule

**Nextra slugs become Helpin readable slugs, but final public URLs must be generated from Helpin public IDs after creation.**

Helpin public help center URLs include an 8-char hex PublicID suffix:
- Collections: `/c/{collectionSlug}-{collectionPublicID}`
- Articles: `/articles/{articleSlug}-{articlePublicID}`

PublicIDs are generated by the database layer when collections and articles are created via `DocsCollectionService.Create()` and `DocsHelpcenterService.CreateArticle()`. They cannot be predicted at preview time.

**Practical implications for the executor:**
1. The preview `ImportPlan` uses slugs only — this is correct because PublicIDs don't exist yet.
2. The executor must create collections/articles first, collect their generated `public_id` values, then produce:
   - Redirect records (the `docs_redirects` table stores `target_collection_slug` and `target_article_slug` which is fine — the public route layer resolves slugs with PublicID fallback)
   - Redirect map JSON for the import report (must use canonical URLs with PublicIDs, e.g. `/articles/getting-started-abc123ef`)
   - Rewritten internal links (must use canonical public paths with PublicIDs)
3. The preview report can show estimated target slugs but must note that final URLs will include a PublicID suffix.
4. The `ImportRedirect.TargetCollectionSlug` and `TargetArticleSlug` fields remain slug-only for the normalized plan — the executor adds PublicIDs when writing the actual redirect map and report.

## Task 1: Add Normalized Import Models

**Files:**
- Create: `server/internal/docsimport/import_plan.go`
- Create: `server/internal/docsimport/import_plan_test.go`

- [ ] **Step 1: Write failing tests for import plan validation**

Test:
- rejects empty source system
- rejects duplicate source IDs within collections/articles
- accepts warnings and hidden items
- sorts collections/articles by position

Run:

```bash
cd /root/teampulse/server
go test ./internal/docsimport -run 'TestImportPlan'
```

Expected: FAIL because `ImportPlan` does not exist.

- [ ] **Step 2: Implement normalized DTOs and validation helpers**

Add:
- `ImportPlan`
- `ImportSpace`
- `ImportCollection`
- `ImportArticle`
- `ImportAsset`
- `ImportRedirect`
- `UnsupportedComponentSummary`
- `ValidateImportPlan(plan ImportPlan) []Warning`

- [ ] **Step 3: Run tests**

```bash
go test ./internal/docsimport -run 'TestImportPlan'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/docsimport/import_plan.go server/internal/docsimport/import_plan_test.go
git commit -m "feat: add normalized docs import plan"
```

## Task 2: Add Safe Nextra Zip Archive Reader

**Files:**
- Create: `server/internal/docsimport/nextra_archive.go`
- Create: `server/internal/docsimport/nextra_archive_test.go`

- [ ] **Step 1: Write failing tests for zip validation**

Fixtures should cover:
- accepts `.md`, `.mdx`, `_meta.json`, `_meta.js`, image assets
- rejects zip-slip paths like `../../evil`
- rejects absolute paths
- rejects archives over configured max uncompressed size
- detects Nextra root at repo root, `content/`, `src/content/`, `apps/docs`, and `docs`

Run:

```bash
go test ./internal/docsimport -run 'TestNextraArchive'
```

Expected: FAIL.

- [ ] **Step 2: Implement archive reader**

Rules:
- max compressed upload size from handler, suggested 100MB
- max uncompressed total size, suggested 500MB
- max file count, suggested 10,000
- no path traversal
- normalize all paths to slash-separated relative paths
- do not extract to permanent disk if parsing can happen in memory
- if temp files are needed, use `os.MkdirTemp` and cleanup with `defer os.RemoveAll`

Expose:

```go
type NextraArchive struct {
    Files map[string][]byte
    RootPath string
}

func ReadNextraArchive(r io.Reader, sizeLimit int64) (*NextraArchive, []Warning, error)
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/docsimport -run 'TestNextraArchive'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/docsimport/nextra_archive.go server/internal/docsimport/nextra_archive_test.go
git commit -m "feat: read nextra import archives safely"
```

## Task 3: Parse Nextra Metadata And Navigation

**Files:**
- Create: `server/internal/docsimport/nextra_meta.go`
- Create: `server/internal/docsimport/nextra_meta_test.go`

- [ ] **Step 1: Write failing tests for `_meta` parsing**

Test:
- `_meta.json` object order is preserved
- simple `_meta.js` with `export default {}` is parsed
- simple `_meta.ts` with `export default { ... } as const` is parsed
- supports string values: `{ "index": "Introduction" }`
- supports object values: `{ "guide": { "title": "Guide", "display": "hidden" } }`
- supports `type: "page"`, `href`, `display: "hidden"`, `theme.sidebar`
- dynamic/function values produce warnings, not crashes

Run:

```bash
go test ./internal/docsimport -run 'TestNextraMeta'
```

Expected: FAIL.

- [ ] **Step 2: Implement static meta parser**

Do not execute JS. Implement:

```go
type NextraMetaEntry struct {
    Key string
    Title string
    Type string
    Href string
    Display string
    Items []NextraMetaEntry
    Theme map[string]any
}

func ParseNextraMeta(path string, data []byte) ([]NextraMetaEntry, []Warning)
```

Parser strategy:
- `_meta.json`: decode with ordered JSON helper.
- `_meta.js` / `_meta.ts`: support common static subset by extracting object after `export default` or `module.exports =`.
- Strip comments, trailing commas, `as const`, and `satisfies Meta`.
- Quote unquoted object keys before JSON decode.
- If parsing fails, emit warning and continue with filesystem fallback.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/docsimport -run 'TestNextraMeta'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/docsimport/nextra_meta.go server/internal/docsimport/nextra_meta_test.go
git commit -m "feat: parse nextra navigation metadata"
```

## Task 4: Parse MDX Frontmatter And Known Components

**Files:**
- Create: `server/internal/docsimport/nextra_mdx.go`
- Create: `server/internal/docsimport/nextra_mdx_test.go`
- Modify: `server/internal/docsimport/html_to_tiptap.go`
- Modify: `server/internal/docsimport/helpscout_normalize.go`

- [ ] **Step 1: Write failing tests for frontmatter**

Test:
- YAML frontmatter `title`, `description`, `slug`, `tags`, `draft`
- title fallback from first heading
- slug fallback from file name
- import/export lines are removed

Run:

```bash
go test ./internal/docsimport -run 'TestNextraMDX'
```

Expected: FAIL.

- [ ] **Step 2: Write failing tests for known component mapping**

Test:
- `<Callout type="warning">Body</Callout>` maps to Helpin callout warning/yellow.
- `<Callout type="info">Body</Callout>` maps to Helpin callout blue.
- `<Steps>` preserves inner headings/content without dropping text.
- `<Tabs items={["npm","pnpm"]}>` emits fallback sections and warning if not natively supported.
- `<Cards>` / `<Card title href>` becomes link list or HTML block with warning.
- unknown `<CustomThing>` emits `unsupported_mdx_component` warning and preserves visible placeholder/source.

- [ ] **Step 3: Implement MDX preprocessing**

Add:

```go
type NextraDocumentSource struct {
    Frontmatter map[string]any
    Title string
    Description string
    Slug string
    Body string
    UnsupportedComponents []string
    Warnings []Warning
}

func ParseNextraMDX(path string, data []byte) (*NextraDocumentSource, []Warning, error)
func ConvertNextraMDXToTipTap(path string, source string) (*ConversionResult, []Warning, error)
```

Implementation:
- parse frontmatter using `gopkg.in/yaml.v3`
- remove import/export statements
- normalize known Nextra components into markdown/HTML
- render markdown to HTML with Goldmark GFM
- pass HTML into existing `docsimport.ConvertHTML`
- extend generic callout detection beyond HelpScout classes so Nextra callout HTML maps to Helpin callout
- preserve unknown components as sanitized `htmlBlock` or placeholder with warning

- [ ] **Step 4: Run tests**

```bash
go test ./internal/docsimport -run 'TestNextraMDX'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/docsimport/nextra_mdx.go server/internal/docsimport/nextra_mdx_test.go server/internal/docsimport/html_to_tiptap.go server/internal/docsimport/helpscout_normalize.go
git commit -m "feat: convert nextra mdx content"
```

## Task 5: Derive Routes, Assets, And Internal Links

**Files:**
- Create: `server/internal/docsimport/nextra_links.go`
- Create: `server/internal/docsimport/nextra_links_test.go`
- Create: `server/internal/docsimport/nextra_assets.go`
- Create: `server/internal/docsimport/nextra_assets_test.go`

- [ ] **Step 1: Write failing route tests**

Test:
- `content/index.mdx` -> `/`
- `content/getting-started.mdx` -> `/getting-started`
- `content/guides/index.mdx` -> `/guides`
- `app/docs/getting-started/page.mdx` -> `/docs/getting-started`
- route base respects detected docs root

- [ ] **Step 2: Write failing link rewrite tests**

Test:
- relative link `./install` resolves to source route and target slug (slug-only at preview time)
- `../reference/api` resolves correctly
- hash links are preserved
- external links remain unchanged
- missing internal links create warnings
- **Post-creation rewrite**: after the executor creates articles/collections, internal links must be rewritten to canonical public paths using PublicIDs (e.g. `/articles/install-abc123ef`), not slug-only paths

- [ ] **Step 3: Write failing asset tests**

Test:
- `![Alt](./img.png)` resolves relative to MDX file
- `/logo.png` resolves from `public/logo.png`
- `<img src="./hero.png" />` resolves
- missing asset creates warning
- remote image URLs are kept or optionally imported by existing external-image path

- [ ] **Step 4: Implement route/link/asset helpers**

Expose:

```go
func NextraRouteForPath(rootPath, filePath string) string
func RewriteNextraInternalLinks(content string, currentPath string, routeMap map[string]string) (string, []Warning)
func PlanNextraAssets(files map[string][]byte, article ImportArticle) ([]ImportAsset, string, []Warning)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/docsimport -run 'TestNextra(Route|Links|Assets)'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/docsimport/nextra_links.go server/internal/docsimport/nextra_links_test.go server/internal/docsimport/nextra_assets.go server/internal/docsimport/nextra_assets_test.go
git commit -m "feat: resolve nextra routes links and assets"
```

## Task 6: Build Nextra Adapter Preview Plan

**Files:**
- Create: `server/internal/docsimport/nextra_adapter.go`
- Create: `server/internal/docsimport/nextra_adapter_test.go`
- Modify: `server/internal/docsimport/collection_tree_mapping.go` if needed

- [ ] **Step 1: Write failing adapter tests**

Use in-memory fake repo files:
- `package.json`
- `content/_meta.json`
- `content/index.mdx`
- `content/getting-started.mdx`
- `content/guides/_meta.json`
- `content/guides/install.mdx`
- `public/logo.png`

Assert:
- one `ImportPlan`
- collections match folder tree
- articles match files
- positions follow `_meta`
- hidden pages are marked hidden
- source routes are correct
- redirects are planned
- warnings include unsupported components

Run:

```bash
go test ./internal/docsimport -run 'TestNextraAdapter'
```

Expected: FAIL.

- [ ] **Step 2: Implement adapter**

Expose:

```go
type NextraAdapter struct{}

func (a NextraAdapter) Preview(ctx context.Context, archive *NextraArchive, opts NextraPreviewOptions) (*ImportPlan, []Warning, error)
```

Implementation rules:
- detect docs root
- walk `.md` / `.mdx`
- parse nearest `_meta`
- build folders as source groups
- use `MapSourceGroupsToDocsCollections`
- treat folder `index.mdx` / `page.mdx` as a collection landing page candidate; v1 may import as normal article with clear warning if collection landing pages are not supported in executor
- generate redirect plan entries from source route to target slug (slug-only at preview time; the executor resolves canonical public paths with PublicIDs after creation)
- dedupe slugs deterministically with suffixes and warnings

- [ ] **Step 3: Run tests**

```bash
go test ./internal/docsimport -run 'TestNextraAdapter'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/docsimport/nextra_adapter.go server/internal/docsimport/nextra_adapter_test.go server/internal/docsimport/collection_tree_mapping.go
git commit -m "feat: build nextra import preview plan"
```

## Task 7: Add Nextra Preview/Start DTOs And Job Config

**Files:**
- Modify: `server/internal/model/docs_import.go`
- Modify: `server/internal/service/docs_import.go`
- Create: `server/internal/service/docs_import_nextra_config_test.go`

- [ ] **Step 1: Write failing config tests**

Test:
- Nextra preview creates job config with `source=nextra`
- config stores archive key/name, detected root, source commit, import status
- read config is backward compatible with HelpScout jobs

Run:

```bash
go test ./internal/service -run 'TestDocsImportNextraConfig'
```

Expected: FAIL.

- [ ] **Step 2: Add model DTOs**

Add:
- `DocsNextraImportPreviewResponse`
- `DocsNextraImportStartRequest`
- `DocsImportWarningResponse`
- `DocsImportBrokenLinkResponse`
- `DocsImportUnsupportedComponentResponse`

Extend internal `docsImportJobConfig`:

```go
SourceSystem string `json:"source_system,omitempty"`
ArchiveKey string `json:"archive_key,omitempty"`
ArchiveName string `json:"archive_name,omitempty"`
DetectedRoot string `json:"detected_root,omitempty"`
SourceCommit string `json:"source_commit,omitempty"`
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/service -run 'TestDocsImportNextraConfig'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/model/docs_import.go server/internal/service/docs_import.go server/internal/service/docs_import_nextra_config_test.go
git commit -m "feat: add nextra import job config"
```

## Task 8: Add Backend Preview Endpoint

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/handler/docs_import.go`
- Modify: `server/internal/router/router.go`
- Create: `server/internal/service/docs_import_nextra_test.go`
- Create: `server/internal/handler/docs_import_nextra_test.go`

- [ ] **Step 1: Write failing service preview test**

Test:
- multipart archive input produces pending `docs_import_jobs` row with `source=nextra`
- preview response contains article counts, collection counts, warnings
- no docs spaces/collections/documents are created during preview

Run:

```bash
go test ./internal/service -run 'TestDocsImportService_PreviewNextra'
```

Expected: FAIL.

- [ ] **Step 2: Implement `PreviewNextra` service method**

```go
func (s *DocsImportService) PreviewNextra(ctx context.Context, workspaceID, userID, archiveName, sourceCommit string, archive io.Reader, size int64) (*model.DocsNextraImportPreviewResponse, error)
```

Implementation:
- validate archive
- parse Nextra plan
- store archive privately in S3 under `docs-import-archives/{workspaceID}/{jobID}.zip`
- create `docs_import_jobs` row with `source=nextra`, `status=pending`, `config`, `summary`
- return preview response

If S3 is unavailable, return `file storage is not configured`.

- [ ] **Step 3: Write failing handler test**

Test:
- rejects missing workspace_id
- rejects missing archive
- accepts multipart zip and returns 200 with `job_id`

- [ ] **Step 4: Implement handler and route**

Route:

```go
r.With(requirePerm(authorization.PermDocsImport)).Post("/import/nextra/preview", h.Docs.ImportPreviewNextra)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/service -run 'TestDocsImportService_PreviewNextra'
go test ./internal/handler -run 'TestDocsHandler_ImportPreviewNextra'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/docs_import.go server/internal/service/docs_import_nextra_test.go server/internal/handler/docs_import.go server/internal/handler/docs_import_nextra_test.go server/internal/router/router.go
git commit -m "feat: preview nextra docs imports"
```

## Task 9: Add Shared Import Executor For Nextra

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Create: `server/internal/service/docs_import_executor_test.go`

- [ ] **Step 1: Write failing executor tests**

Test:
- creates external-capable space when `new_space_name` is set
- creates collections parent-before-child
- creates documents/articles with source slugs
- saves converted TipTap JSON content
- uploads local assets and rewrites URLs
- creates imported redirects
- stores source provenance on docs content
- updates job totals/progress/summary/failures

Run:

```bash
go test ./internal/service -run 'TestDocsImportExecutor_Nextra'
```

Expected: FAIL.

- [ ] **Step 2: Implement source-agnostic executor**

Add internal method:

```go
func (s *DocsImportService) executeImportPlan(ctx context.Context, jobID, workspaceID, userID, targetSpaceID string, plan *docsimport.ImportPlan, opts executeImportOptions) error
```

Rules:
- create collections in plan order
- map source collection IDs to Helpin collection IDs/slugs
- convert each article using `ConvertNextraMDXToTipTap`
- use `documentSvc.Create`
- use `contentSvc.Save`
- use `contentSvc.SetImportProvenance`
- use `helpcenterSvc.CreateArticle`
- publish based on `import_status`
- create redirects via `redirectRepo.Create`
- collect failures per article
- update progress every 5 articles and at completion

- [ ] **Step 3: Preserve article metadata**

Map:
- frontmatter `title` -> document title
- frontmatter `description` -> help center article meta description if supported by existing model; if not supported, add warning and do not create schema in this plan
- frontmatter `slug` -> helpcenter slug
- frontmatter `draft: true` -> draft unless import mode forces published

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service -run 'TestDocsImportExecutor_Nextra'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/docs_import.go server/internal/service/docs_import_executor_test.go
git commit -m "feat: execute normalized docs import plans"
```

## Task 10: Add Nextra Start Endpoint

**Files:**
- Modify: `server/internal/service/docs_import.go`
- Modify: `server/internal/handler/docs_import.go`
- Modify: `server/internal/router/router.go`
- Create: `server/internal/service/docs_import_nextra_start_test.go`
- Create: `server/internal/handler/docs_import_nextra_start_test.go`

- [ ] **Step 1: Write failing service start tests**

Test:
- start loads pending Nextra job
- rejects non-Nextra jobs
- rejects missing target space and new space name
- marks job running and eventually done
- reads archive from S3/private store

Run:

```bash
go test ./internal/service -run 'TestDocsImportService_StartNextra'
```

Expected: FAIL.

- [ ] **Step 2: Implement `StartNextra`**

```go
func (s *DocsImportService) StartNextra(ctx context.Context, req model.DocsNextraImportStartRequest, workspaceID, userID string) (string, error)
```

Implementation:
- load job
- validate `source=nextra`
- resolve target space
- launch background goroutine
- re-read archive and rebuild plan at start to avoid trusting stale preview JSON
- call `executeImportPlan`

- [ ] **Step 3: Write failing handler test**

Test:
- bad request for missing job_id
- accepted response for valid request

- [ ] **Step 4: Add route**

```go
r.With(requirePerm(authorization.PermDocsImport)).Post("/import/nextra/start", h.Docs.ImportStartNextra)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/service -run 'TestDocsImportService_StartNextra'
go test ./internal/handler -run 'TestDocsHandler_ImportStartNextra'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/docs_import.go server/internal/service/docs_import_nextra_start_test.go server/internal/handler/docs_import.go server/internal/handler/docs_import_nextra_start_test.go server/internal/router/router.go
git commit -m "feat: start nextra docs imports"
```

## Task 11: Add Frontend API Types And Service Methods

**Files:**
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/lib/types.ts` or docs import type file
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Create: `frontend/src/lib/__tests__/nextraImportTypes.test.ts`

- [ ] **Step 1: Write failing frontend type/helper tests**

Test:
- warning counts summarize correctly
- multipart preview call includes archive and source commit
- start payload includes job/space/import mode

Run:

```bash
cd /root/teampulse/frontend
npx vitest run src/lib/__tests__/nextraImportTypes.test.ts
```

Expected: FAIL.

- [ ] **Step 2: Add TypeScript DTOs**

Add:
- `DocsNextraImportPreviewResponse`
- `DocsNextraImportStartRequest`
- `DocsImportWarning`
- `DocsImportUnsupportedComponent`
- `DocsImportBrokenLink`

- [ ] **Step 3: Add service methods**

```ts
previewNextra(workspaceId: string, payload: { archive: File; source_commit?: string })
startNextra(workspaceId: string, payload: DocsNextraImportStartRequest)
```

Use multipart `FormData` for preview.

- [ ] **Step 4: Add hooks**

Add `usePreviewNextraImport` and `useStartNextraImport`, or keep direct service use if existing import UI does not use query hooks.

- [ ] **Step 5: Run tests and typecheck**

```bash
npx vitest run src/lib/__tests__/nextraImportTypes.test.ts
npx tsc -b --pretty false
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/services/docsService.ts frontend/src/lib/types.ts frontend/src/hooks/queries/useDocs.ts frontend/src/lib/__tests__/nextraImportTypes.test.ts
git commit -m "feat: add nextra import frontend api"
```

## Task 12: Add Nextra Import Wizard UI

**Files:**
- Modify: `frontend/src/components/settings/ImportTab.tsx`
- Create: `frontend/src/components/settings/NextraImportWizard.tsx`
- Create: `frontend/src/components/settings/__tests__/NextraImportWizard.test.tsx`
- Optional: add `frontend/src/assets/import/nextra.svg`

- [ ] **Step 1: Write failing wizard tests**

Test:
- shows zip upload input
- preview button disabled until zip selected
- renders collection/article/warning counts after preview
- renders unsupported components and broken links
- starts import with selected target/new space
- shows job id/progress link after start

Run:

```bash
npx vitest run src/components/settings/__tests__/NextraImportWizard.test.tsx
```

Expected: FAIL.

- [ ] **Step 2: Add Nextra source card**

In `ImportTab`, add Help Center source:
- title: `Nextra`
- description: `Import MDX docs, navigation, images, and redirects from a Nextra repo zip.`
- not coming soon

- [ ] **Step 3: Implement wizard**

Flow:
1. Upload `.zip`
2. Optional source commit/version label
3. Click `Preview import`
4. Show preview cards:
   - detected root
   - collections
   - articles
   - assets
   - redirects
   - warnings
5. Select existing target space or new external docs space name
6. Select import mode: draft or published
7. Confirm start

UX rules:
- default import mode: draft
- show warning if unsupported components exist
- show warning if broken links exist
- do not allow start if no target space/new space name
- keep copy explicit that Helpin becomes canonical after import

- [ ] **Step 4: Run tests**

```bash
npx vitest run src/components/settings/__tests__/NextraImportWizard.test.tsx
npx tsc -b --pretty false
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/settings/ImportTab.tsx frontend/src/components/settings/NextraImportWizard.tsx frontend/src/components/settings/__tests__/NextraImportWizard.test.tsx frontend/src/assets/import/nextra.svg
git commit -m "feat: add nextra import wizard"
```

## Task 13: Import Report And Redirect Map Fidelity

**Files:**
- Modify: `server/internal/model/docs_import.go`
- Modify: `server/internal/service/docs_import.go`
- Modify: `frontend/src/components/settings/HelpCenterImportSection.tsx` or shared import job UI if present
- Test: `server/internal/service/docs_import_nextra_test.go`

- [ ] **Step 1: Write failing report tests**

Test summary includes:
- `source_system`
- `collections_created`
- `articles_published`
- `articles_drafted`
- `redirects_created`
- `unsupported_component_count`
- `broken_link_count`
- `asset_rewrite_failures`
- `html_block_fallbacks`

Run:

```bash
go test ./internal/service -run 'TestDocsImportService_NextraReport'
```

Expected: FAIL.

- [ ] **Step 2: Extend summary model without breaking HelpScout**

Add optional fields to `ImportSummary`:
- `SourceSystem string`
- `UnsupportedComponents int`
- `BrokenLinks int`
- `AssetRewriteFailures int`

Keep JSON backwards-compatible.

- [ ] **Step 3: Ensure redirect map includes old/new paths with PublicIDs**

After creating collections and articles, the executor collects their generated `public_id` values and builds the redirect map using canonical public URLs:

For each imported article:

```json
{
  "old_url": "/docs/getting-started",
  "new_slug": "getting-started",
  "new_public_id": "abc123ef",
  "new_path": "/articles/getting-started-abc123ef"
}
```

For each imported collection:

```json
{
  "old_url": "/docs/guides",
  "new_slug": "guides",
  "new_public_id": "1234abcd",
  "new_path": "/c/guides-1234abcd"
}
```

The redirect map must be built **after** DB creation when PublicIDs are available. Use `buildDocsHelpcenterArticleCanonicalPath` and `buildDocsHelpcenterCollectionCanonicalPath` from `docs_public_paths.go` to generate the canonical URLs.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service -run 'TestDocsImportService_NextraReport'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/docs_import.go server/internal/service/docs_import.go frontend/src/components/settings/HelpCenterImportSection.tsx server/internal/service/docs_import_nextra_test.go
git commit -m "feat: report nextra import fidelity"
```

## Task 14: End-To-End Staging Fixture

**Files:**
- Create: `server/internal/docsimport/testdata/nextra-basic.zip` or generate zip in test code
- Create: `server/internal/service/docs_import_nextra_e2e_test.go`

- [ ] **Step 1: Write E2E test with realistic Nextra fixture**

Fixture should include:
- root `_meta.json`
- nested `_meta.js`
- `index.mdx`
- nested article
- image from `public/`
- relative internal links
- one unsupported custom component
- one hidden page

Run:

```bash
go test ./internal/service -run 'TestDocsImportNextra_EndToEnd'
```

Expected: FAIL until all pieces are integrated.

- [ ] **Step 2: Make E2E pass**

Fix integration gaps only. Do not add new product scope.

- [ ] **Step 3: Run focused backend tests**

```bash
go test ./internal/docsimport
go test ./internal/service -run 'TestDocsImport.*Nextra'
go test ./internal/handler -run 'TestDocsHandler_Import.*Nextra'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/docsimport/testdata server/internal/service/docs_import_nextra_e2e_test.go
git commit -m "test: cover nextra import end to end"
```

## Task 15: Manual QA Checklist

**Files:**
- Create: `docs/qa/nextra-importer.md`

- [ ] **Step 1: Add QA checklist**

Checklist:
- upload real Usermaven Nextra zip in staging
- confirm preview counts match source repo
- inspect collection tree ordering
- inspect article formatting
- inspect callouts, code blocks, tables, images
- inspect unsupported component report
- inspect internal links
- inspect old-route redirects
- import as draft first
- publish after QA
- verify public help center pages

- [ ] **Step 2: Commit**

```bash
git add docs/qa/nextra-importer.md
git commit -m "docs: add nextra importer qa checklist"
```

## Verification Commands

Backend:

```bash
cd /root/teampulse/server
go test ./internal/docsimport
go test ./internal/service -run 'TestDocsImport.*Nextra'
go test ./internal/handler -run 'TestDocsHandler_Import.*Nextra'
go test ./internal/service -run 'TestDocsImportService_PreviewNextra|TestDocsImportService_StartNextra|TestDocsImportExecutor_Nextra'
```

Frontend:

```bash
cd /root/teampulse/frontend
npx vitest run src/components/settings/__tests__/NextraImportWizard.test.tsx src/lib/__tests__/nextraImportTypes.test.ts
npx tsc -b --pretty false
```

Final focused smoke:

```bash
cd /root/teampulse/server
go test ./internal/docsimport ./internal/service ./internal/handler
```

## Risks And Mitigations

- **Dynamic `_meta.js` cannot be fully parsed without executing user code.**
  - Mitigation: static subset parser, warning, filesystem fallback, optional user guidance to convert `_meta` to JSON before import.

- **Custom MDX components cannot be perfectly mapped.**
  - Mitigation: map known Nextra components, preserve unknown content as sanitized blocks/placeholders, report every fallback.

- **Large zip uploads can strain memory.**
  - Mitigation: file count and size caps, stream validation where possible, reject huge archives with clear errors.

- **Duplicate slugs can cause import failure.**
  - Mitigation: deterministic suffixing during preview and warnings before import.

- **Internal links may point to dynamic routes or anchors only.**
  - Mitigation: preserve hash anchors, rewrite resolvable links, report unresolved links.

- **Preview/start drift if archive changes.**
  - Mitigation: preview stores exact archive object in private S3 and start imports from the stored archive, not a new upload.

## Out Of Scope

- GitHub OAuth/GitHub App integration.
- Git sync or webhook sync.
- Permanent two-way authoring between Nextra and Helpin.
- Full arbitrary React component execution/rendering.
- Native Helpin tabs/cards component implementation unless already available.
- Historical import from live crawled public Nextra site.

