# Plan Review: HelpScout Docs Import

**Plan:** `docs/superpowers/plans/2026-03-18-helpscout-docs-import.md`
**Spec:** `docs/superpowers/specs/2026-03-18-helpscout-docs-import-design.md`
**Reviewer:** Code Review Agent
**Date:** 2026-03-18

---

## Summary

The plan is well-structured and covers most spec requirements. However, there are several concrete issues -- some critical -- that would cause build failures or behavioral mismatches if implemented as-is. The task ordering is sound and each task can be built incrementally.

---

## Critical Issues (Must Fix)

### 1. Migration file number collision

The plan uses `033_docs_import.sql` but the codebase already has `033_fix_ws_member_unique_index.sql` and `033_helpcenter_config_extended.sql`. The latest migration file is `046_conversation_read_tracking_indexes.sql`.

**Fix:** Use `047_docs_import.sql` (or split into `047_docs_collection_slug.sql` + `048_docs_import_jobs.sql` to match the spec's two-migration approach).

### 2. Migration schema deviation from spec

The plan's migration combines both the slug column and the import jobs table into a single file (`033_docs_import.sql`). The spec explicitly separates them into `033_docs_collection_slug.sql` and `034_docs_import_jobs.sql`.

More importantly, the plan's `docs_import_jobs` table includes columns not in the spec:
- `error TEXT` -- present in plan, absent from spec
- `started_by UUID NOT NULL` -- present in plan, absent from spec

The spec's table has `REFERENCES workspaces(id)` and `REFERENCES docs_spaces(id)` foreign key constraints; the plan omits them. Also the spec's unique index on collection slug uses `WHERE slug != ''` but the plan uses `WHERE slug != '' AND deleted_at IS NULL` -- which is arguably better, but a deviation.

**Fix:** Add `error` and `started_by` to the spec (or remove from plan if not needed). Add the FK constraints per spec. Use correct migration numbers.

### 3. DocsImportJob model missing `started_by` field

The spec model in the design doc does not include a `started_by` field, but the plan's migration SQL does. The GORM model in the plan (Task 1 Step 2) references `started_by` in the migration but does not explicitly list it in the model struct description.

**Fix:** Add `StartedBy string` to both the GORM model description and the spec.

### 4. `docs_helpcenter.go` model file does not exist

The plan references creating `DocsHelpcenterArticle` records via `helpcenterSvc`, but the file `server/internal/model/docs_helpcenter.go` does not exist. The helpcenter article model likely lives in `server/internal/model/docs.go` alongside other docs models.

**Fix:** Verify where `DocsHelpcenterArticle` is defined (likely in `docs.go`) and update any file references.

### 5. Concurrency pattern violates CLAUDE.md rules

The plan uses `go s.runImport(jobID, apiKey, req)` (Task 8 Step 3) -- a fire-and-forget goroutine. The spec itself says to use `errgroup.Group` wired into graceful shutdown. The `server/CLAUDE.md` explicitly says: "Never fire-and-forget goroutines -- every goroutine must have a stop condition and a way to wait for it."

The existing `PMImportService` also uses `*gorm.DB` directly rather than fire-and-forget goroutines.

**Fix:** Replace with `errgroup.Group` as the spec prescribes, or use a similar pattern to the existing PM import service.

---

## Important Issues (Should Fix)

### 6. Handler/Router architecture mismatch

The plan creates a **separate** `DocsImportHandler` and adds `DocsImport *handler.DocsImportHandler` to the `Handlers` struct. But the existing pattern uses a single `DocsHandler` that wraps all docs-related services. Other modules (CRM, PM) also follow this pattern of one handler per module area.

Adding the import routes through the existing `DocsHandler` (or adding the import service as a dependency to `DocsHandler`) would be more consistent.

**Fix:** Either add import methods to `DocsHandler` and inject the import service there, OR justify why a separate handler is appropriate. Both approaches work, but the plan should be explicit about the tradeoff.

### 7. Route path inconsistency with spec

The spec says endpoints are "nested under the workspace route group (`/api/workspaces/{id}/...`)" but the plan registers routes like `/docs/import/helpscout/preview`. Looking at the actual router, docs routes live under a `/docs` group that uses `middleware.RequireWorkspaceID` (workspace ID comes from query param `?workspace_id=`).

The frontend service code in the plan also uses `?workspace_id=` query params, which is consistent with the actual router pattern. But the spec's endpoint listing shows paths like `POST /api/docs/import/helpscout/preview` without query params -- this is misleading.

**Fix:** Align the spec to match the actual router pattern, or clarify that workspace_id is a query parameter.

### 8. `DocsSpaceService.Create` requires `userID` parameter

The plan's `Start` method (Task 8 Step 3) calls `spaceSvc.Create()` when `NewSpaceName` is provided, but the actual `DocsSpaceService.Create` signature is:
```go
func (s *DocsSpaceService) Create(ctx context.Context, workspaceID string, req model.CreateDocsSpaceRequest, userID string) (*model.DocsSpaceWithTeams, error)
```
The plan needs to pass `userID` (the user who started the import) to the space creation call.

**Fix:** Ensure the import service has access to `userID` and passes it to `spaceSvc.Create()`.

### 9. Content save path needs `json.RawMessage`, not markdown string

The plan says to save content via `contentSvc.Save()` using a markdown envelope. Looking at the actual code, the existing `SaveMarkdownContent` handler in `DocsHandler` does the JSON envelope wrapping (`{"_markdown_source": "..."}`) and then calls `contentSvc.Save(ctx, docID, raw)` where `raw` is `json.RawMessage`.

The import service should replicate this envelope wrapping pattern directly rather than going through the handler.

**Fix:** The plan's description in Task 8 Step 4 is correct conceptually but should include the explicit `json.Marshal` envelope code to avoid ambiguity.

### 10. Missing `interrupted` status in plan model

The spec defines 5 statuses: `pending`, `running`, `done`, `failed`, `interrupted`. The plan (Task 1 Step 2) only defines 4 constants: `Pending`, `Running`, `Done`, `Failed`. The `interrupted` status for graceful shutdown is missing.

**Fix:** Add `DocsImportStatusInterrupted` constant.

---

## Suggestions (Nice to Have)

### 11. No `types.go` file for helpscout package

The plan correctly separates types into `server/internal/helpscout/types.go`, but the `images.go` file includes `ProcessImages` which takes `*storage.S3Client` -- coupling the helpscout package to the storage package. Consider having the import service orchestrate image processing instead, keeping the helpscout package focused on API interaction and HTML conversion.

### 12. Missing frontend permission type update

The plan does not mention updating the frontend `Permission` union type in `src/lib/types.ts` to include `'docs.import'`. Without this, TypeScript calls to `has('docs.import')` will fail type checking.

### 13. Missing frontend query hook for polling

The plan uses `setInterval` for polling job status. A TanStack Query hook with `refetchInterval` would be more idiomatic for this codebase:
```tsx
useQuery({
  queryKey: ['docs-import', jobId],
  queryFn: () => docsImportService.getStatus(wsId, jobId),
  refetchInterval: 2000,
  enabled: !!jobId,
})
```

### 14. Test coverage gaps

- No repository tests for `DocsImportRepository` (Task 2)
- No handler tests with `httptest` (Task 9)
- No service-level tests for the import orchestration (Task 8)
- Only converter and image tests are included (Tasks 6, 7)

The spec's error handling matrix (rate limits, image failures, HTML conversion failures, network errors) should each have a corresponding test case.

### 15. Image processing on HTML before conversion

The plan's Task 8 Step 4 calls `ProcessImages()` then `ConvertHTML()`. But `ProcessImages` works on HTML, and `ConvertHTML` also works on HTML. The plan should clarify that image URL replacement happens in the HTML **before** markdown conversion, which is correct but worth being explicit about in the implementation notes.

---

## Completeness Checklist

| Spec Requirement | Plan Task | Status |
|-----------------|-----------|--------|
| HelpScout API client with auth + rate limiting | Task 5 | Covered |
| HTML to Markdown conversion | Task 6 | Covered |
| Image re-upload to S3 | Task 7 | Covered |
| Collection slug preservation | Tasks 1, 3 | Covered |
| Import job tracking table | Task 1 | Covered (wrong migration number) |
| Background job with progress | Task 8 | Covered (wrong concurrency pattern) |
| Preview endpoint | Tasks 8, 9 | Covered |
| Start endpoint | Tasks 8, 9 | Covered |
| Status polling endpoint | Tasks 8, 9 | Covered |
| Retry failed articles | Tasks 8, 9 | Covered |
| Redirect map CSV download | Tasks 8, 9 | Covered |
| `docs.import` permission | Task 4 | Covered |
| Frontend connect/configure/progress UI | Task 11 | Covered |
| Draft article handling (3 modes) | Task 8 | Covered |
| `errgroup` graceful shutdown | Task 8 | NOT covered (uses fire-and-forget) |
| `interrupted` status on server restart | Task 1 model | NOT covered |
| HTML conversion failure fallback (raw HTML in code block) | Task 8 | NOT explicitly covered |
| Article fetch retry (3 retries per request) | Task 5 | Partially covered (rate limit backoff yes, retry count no) |
| API key never logged | Tasks 8, 9 | NOT explicitly addressed |
| Frontend Permission type update | -- | MISSING |

---

## Task Ordering Assessment

The task ordering is correct. Each task builds on previous ones:
1. Model + Migration (no deps)
2. Repository (depends on 1)
3. Collection slug support (depends on 1)
4. Authorization (no deps, could run parallel with 2-3)
5. HelpScout client (no deps, could run parallel with 1-4)
6. HTML converter (depends on 5 for package)
7. Image processor (no deps beyond Go stdlib + storage)
8. Import service (depends on 2, 3, 5, 6, 7)
9. Handler + routes + DI (depends on 4, 8)
10. Frontend service (no backend deps)
11. Frontend UI (depends on 10)
12. Integration testing (depends on all)

Tasks 1-5 could largely run in parallel. Tasks 6-7 could also run in parallel.
