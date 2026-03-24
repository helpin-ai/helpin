# Design Review: HelpScout Help Center Import

**Spec**: `docs/superpowers/specs/2026-03-18-helpscout-docs-import-design.md`
**Reviewer**: Code Review Agent
**Date**: 2026-03-18

---

## Overall Assessment

The spec is well-structured and demonstrates strong familiarity with the existing codebase patterns. The data mapping, API endpoint design, and frontend UI flow are all sound. Below are issues categorized by severity.

---

## Critical Issues (Must Fix)

### 1. API Key Handled in Plaintext -- No Encryption at Rest

The spec passes `api_key` in JSON request bodies for both `preview` and `start` endpoints but never addresses storage or encryption. The codebase already has `internal/crypto/` (AES-256-GCM) used for Gmail OAuth tokens in CRM. If the API key is stored in the `ImportJob` (even in-memory via `sync.Map`), it sits in plaintext in process memory for the duration of the import and is logged if any structured logging accidentally captures the request body.

**Recommendation**: Either (a) encrypt the API key before storing it in the job struct using `internal/crypto/`, or (b) explicitly document that the key is ephemeral and never persisted to DB or logs. Add a note that handlers must never log request bodies containing `api_key`.

### 2. `sync.Map` for Job State -- Lost on Restart, No Persistence

The spec stores `ImportJob` state in a `sync.Map` (in-memory only). If the server restarts mid-import, all job state is lost with no way to recover or retry. The existing PM import (`PMImportService`) and CRM import (`CRMImportService`) both persist import jobs to the database.

**Recommendation**: Define a `docs_import_jobs` table (or reuse a generic import jobs model) to persist job state. This follows the CRM import pattern (`model.CRMImportJob`) and enables retry across restarts. At minimum, persist `job_id`, `workspace_id`, `status`, `total`, `completed`, `failed`, `failures` (JSONB), `started_at`, `completed_at`.

### 3. Background Goroutine Without Lifecycle Management

The spec says `Start()` "launches background goroutine" but the Go backend CLAUDE.md explicitly states: "Never fire-and-forget goroutines -- every goroutine must have a stop condition and a way to wait for it." The existing codebase uses Temporal workflows for long-running background work (email sync, signal detection, deal management).

**Recommendation**: Either (a) use a Temporal workflow (consistent with CRM's `EmailSyncWorkflow` pattern) which gives you automatic retry, persistence, and observability, or (b) at minimum use `context.WithCancel` + `errgroup` with graceful shutdown wired into the server's shutdown handler. Document the stop condition.

---

## Important Issues (Should Fix)

### 4. Missing `docs.import` Permission -- Spec Says `docs.edit`, But PM Uses Dedicated `pm.import`

The spec says all endpoints require `docs.edit` permission. However, the existing PM import uses a dedicated `PermPMImport` (`pm.import`) permission that is admin-only. Import is a destructive bulk operation that creates many entities at once -- it should not be gated by the same permission as editing a single document.

**Recommendation**: Add a `PermDocsImport` (`docs.import`) permission constant in `authorization/permissions.go`, assign it to the `admin` role (matching `pm.import`), and use `requirePerm(authorization.PermDocsImport)` on the import routes.

### 5. `DocsCollection` Already Lacks `Slug` -- But the Spec's Migration Has Issues

The spec proposes adding `slug` to `DocsCollection` with `NOT NULL DEFAULT ''` and a partial unique index `WHERE slug != ''`. This is correct in concept, but:

- The existing `DocsCollection` model in `model/docs.go` (line 144-158) has no `Slug` field. The GORM model must also be updated with the field and struct tag.
- The `CreateDocsCollectionRequest` DTO (line 408-412) needs a `Slug` field.
- The `DocsCollectionService.Create()` must accept and set the slug.
- The `PublicNavCollection` response DTO (line 543-548) already has a `Slug` field, which means the public help center API already expects it but the DB column does not exist yet.

**Recommendation**: Add these details explicitly to the spec's "Schema Changes" section. Include the GORM model change, DTO changes, and service method changes.

### 6. No `workspace_id` Scoping on Import Endpoints

The spec's `POST /api/docs/import/helpscout/start` body includes `target_space_id` but no `workspace_id`. Looking at the handler pattern, `workspace_id` is typically extracted from the middleware context (`middleware.GetWorkspaceID`). The spec should clarify that these endpoints live under the workspace-scoped route group (like PM import at `/api/workspaces/{id}/import/...`) rather than under `/api/docs/import/...`.

**Recommendation**: Either (a) nest under `/api/workspaces/{id}/docs/import/helpscout/...` following the PM import pattern, or (b) explicitly note that `workspace_id` comes from the middleware context via the existing workspace route group. The current `/api/docs/...` routes are already workspace-scoped via middleware, so option (b) works if the routes are registered inside the workspace router group.

### 7. Rate Limit Handling Needs More Detail

The spec says "sleep if approaching 2000/10min" but does not specify the mechanism. HelpScout returns `X-RateLimit-Remaining` and `X-RateLimit-Limit` headers. The client should read these headers and implement token-bucket or sliding-window throttling, not just a fixed sleep.

**Recommendation**: Specify that the client reads `X-RateLimit-Remaining` from response headers and sleeps when remaining < threshold (e.g., 100). Also handle HTTP 429 responses with exponential backoff.

---

## Suggestions (Nice to Have)

### 8. Consider Using Existing `SaveDocsMarkdownRequest` Path

The spec says content is "saved via markdown endpoint." The codebase already has `SaveMarkdownContent` handler and `SaveDocsMarkdownRequest` DTO that wraps markdown in a `{"_markdown_source": "..."}` JSON envelope for TipTap auto-conversion. The spec should explicitly reference this existing path rather than having the import service call `contentSvc.Save()` directly with raw markdown.

### 9. Frontend: ImportTab Currently PM-Only -- Needs Section Separation

The existing `ImportTab.tsx` component renders PM import sources only (Shortcut, Jira, Linear). Adding a "Help Center" section means the component needs structural refactoring to have separate sections with headers (e.g., "Project Management" and "Help Center"). The spec mentions "new section added within existing ImportTab" but should note this requires refactoring the `IMPORT_SOURCES` array into categorized groups.

### 10. New Package Placement: `internal/helpscout/` vs `internal/service/`

The spec creates `internal/helpscout/client.go` and `internal/helpscout/html.go` as a new top-level internal package. This is reasonable for the API client, but the HTML converter could arguably live closer to the docs domain. Consider whether `internal/helpscout/` should be scoped more narrowly to just the API client, with HTML conversion logic in the import service itself. Either approach works; just be consistent with how `internal/sync/` (Gmail client) and `internal/crmemail/` are structured.

### 11. Missing: What Happens to Draft Articles in HelpScout?

The spec mentions HelpScout's `?draft=true` query parameter for accessing draft versions, but the import flow does not address whether to import draft articles. Should the preview step show draft vs. published counts? Should the user be able to opt in/out of importing drafts?

### 12. Missing: Redirect Map Output Format

The spec mentions generating a redirect map ("CSV of old_url -> new_slug pairs") but does not define an API endpoint to download it or where it is stored. Add a `GET /api/docs/import/helpscout/{jobId}/redirects` endpoint that returns the CSV, or include it in the job status response.

---

## What Was Done Well

- Data mapping table is clear and accurate against the existing models.
- The HTML-to-Markdown approach with image re-upload to S3 is the right design -- avoids CDN dependency.
- Error handling strategy (per-article failure tracking, retry mechanism) is practical.
- Frontend UI flow (connect -> configure -> progress) matches the existing PM import wizard pattern.
- SEO slug handling leverages the existing `DocsHelpcenterArticle` and `DocsSlugAlias` infrastructure.
- Future extensibility consideration (Zendesk, Intercom) is well thought out.

---

## HelpScout API Technical Accuracy

- Base URL `https://docsapi.helpscout.net/v1/` is correct for the Docs API (separate from the Mailbox API).
- HTTP Basic auth with API key as username is correct.
- Rate limit of 2000/10min is correct per HelpScout docs.
- Pagination via `page`/`pages` fields is correct.
- Article `text` field containing HTML is correct.
- The `?draft=true` parameter for draft access is correct.

---

## Summary

The spec is approximately 80% complete. The three critical issues (API key security, job persistence, goroutine lifecycle) must be addressed before implementation. The important issues (dedicated permission, model changes, route scoping, rate limiting) should be resolved during implementation planning. The suggestions are enhancements that can be addressed during or after implementation.
