# Help Center Article Preview Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to preview any document (including drafts) as it would appear when published as a help center article, opening in a new browser tab.

**Architecture:** Add a backend endpoint that renders TipTap JSON content to HTML (reusing existing `tiptap.RenderHTML`) for any document regardless of status. Create a new authenticated frontend route that fetches this HTML + help center config and renders a standalone article page with help center branding. Add a "Preview" button in the document detail toolbar.

**Tech Stack:** Go (handler/service), React + TanStack Router + TanStack Query, TipTap CSS, DOMPurify for safe HTML rendering.

---

### Task 1: Backend — Add preview HTML endpoint to docs service

**Files:**
- Modify: `server/internal/service/docs_document.go`

This adds a service method that fetches any document's content and renders TipTap JSON → HTML, regardless of document status.

- [ ] **Step 1: Add `PreviewArticleHTML` method to `DocsDocumentService`**

```go
// PreviewArticleResponse contains rendered HTML for article preview.
type PreviewArticleResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Excerpt        *string `json:"excerpt"`
	Icon           *string `json:"icon"`
	Status         string  `json:"status"`
	CollectionID   *string `json:"collection_id"`
	CollectionName *string `json:"collection_name"`
	SpaceName      string  `json:"space_name"`
	ContentHTML    string  `json:"content_html"`
}
```

Add to `model/docs.go`, then add service method in `docs_document.go`:

```go
func (s *DocsDocumentService) PreviewArticleHTML(ctx context.Context, workspaceID, docID string) (*model.PreviewArticleResponse, error) {
	doc, err := s.repo.GetByID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}

	content, err := s.contentRepo.GetByDocumentID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get content: %w", err)
	}

	var contentHTML string
	if content != nil && len(content.Content) > 0 {
		rendered, err := tiptap.RenderHTML(content.Content)
		if err != nil {
			slog.ErrorContext(ctx, "preview render failed", "error", err, "doc_id", docID)
		} else {
			contentHTML = rendered
		}
	}

	// Resolve space name.
	var spaceName string
	space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID)
	if err == nil && space != nil {
		spaceName = space.Name
	}

	// Resolve collection name.
	var collectionName *string
	if doc.CollectionID != nil {
		coll, err := s.collectionRepo.GetByID(ctx, *doc.CollectionID)
		if err == nil && coll != nil {
			collectionName = &coll.Name
		}
	}

	return &model.PreviewArticleResponse{
		ID:             doc.ID,
		Title:          doc.Title,
		Excerpt:        doc.Excerpt,
		Icon:           doc.Icon,
		Status:         doc.Status,
		CollectionID:   doc.CollectionID,
		CollectionName: collectionName,
		SpaceName:      spaceName,
		ContentHTML:    contentHTML,
	}, nil
}
```

- [ ] **Step 2: Verify the service has access to required repositories**

Check that `DocsDocumentService` already has `spaceRepo` and `collectionRepo` fields. If not, check if we need to use a different service or add the dependency. The helpcenter service already has these — we may need to put this method on the helpcenter service instead, or pass the repos to `DocsDocumentService`.

**Decision rule:** If `DocsDocumentService` already has `spaceRepo` and `collectionRepo`, add the method there. If not, add a standalone method to `DocsHelpcenterService` that accepts a docID directly (bypassing the slug-based lookup and published-only checks).

- [ ] **Step 3: Build and verify no compilation errors**

Run: `cd /root/teampulse/server && go build ./...`
Expected: Clean build with no errors.

- [ ] **Step 4: Commit**

```bash
git add server/internal/model/docs.go server/internal/service/docs_document.go
git commit -m "feat(docs): add PreviewArticleHTML service method for article preview"
```

---

### Task 2: Backend — Add preview handler and route

**Files:**
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`

- [ ] **Step 1: Add handler method**

```go
// PreviewArticleHTML returns rendered HTML for a document preview (any status).
func (h *DocsHandler) PreviewArticleHTML(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspace_id")
	docID := chi.URLParam(r, "docId")

	resp, err := h.docSvc.PreviewArticleHTML(r.Context(), wsID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
```

Adapt to match the handler's existing field names for the document service (may be `h.documentSvc`, `h.docService`, etc. — check the existing handler struct).

- [ ] **Step 2: Register route in router**

Add within the docs document routes group (where other `/{docId}/...` routes live):

```go
r.Get("/{docId}/preview-html", h.Docs.PreviewArticleHTML)
```

This route should be within the authenticated workspace-scoped group, requiring at least workspace read access. No additional permission needed since the user can already view the document.

- [ ] **Step 3: Build and verify**

Run: `cd /root/teampulse/server && go build ./...`
Expected: Clean build.

- [ ] **Step 4: Quick manual test**

Run: `curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/docs/documents/{docId}/preview-html?workspace_id={wsId}"` with a known draft document ID. Expected: JSON response with `content_html` field containing rendered HTML.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/docs.go server/internal/router/router.go
git commit -m "feat(docs): add GET /docs/documents/{docId}/preview-html endpoint"
```

---

### Task 3: Frontend — Add preview service method and query hook

**Files:**
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/lib/docsTypes.ts`
- Modify: `frontend/src/hooks/queries/index.ts` (or the docs query file)
- Modify: `frontend/src/lib/queryKeys.ts`

- [ ] **Step 1: Add TypeScript type for preview response**

In `frontend/src/lib/docsTypes.ts`:

```typescript
export interface PreviewArticleResponse {
  id: string;
  title: string;
  excerpt?: string;
  icon?: string;
  status: string;
  collection_id?: string;
  collection_name?: string;
  space_name: string;
  content_html: string;
}
```

- [ ] **Step 2: Add service method**

In `frontend/src/lib/services/docsService.ts`:

```typescript
getPreviewHTML: (wsId: string, docId: string) =>
  api.get<PreviewArticleResponse>(`/docs/documents/${docId}/preview-html${qs(wsId)}`),
```

- [ ] **Step 3: Add query key**

In `frontend/src/lib/queryKeys.ts`, add within the docs section:

```typescript
previewHTML: (wsId: string, docId: string) => ['docs', 'preview-html', wsId, docId] as const,
```

- [ ] **Step 4: Add query hook**

In the docs query hooks file (find where `useDocsDocument` is defined):

```typescript
export function useDocsPreviewHTML(wsId: string, docId: string) {
  return useQuery({
    queryKey: queryKeys.docs.previewHTML(wsId, docId),
    queryFn: async () => unwrap(await docsService.getPreviewHTML(wsId, docId)),
    enabled: !!wsId && !!docId,
    staleTime: 0, // Always fresh for preview
  });
}
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /root/teampulse/frontend && pnpm build`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/docsTypes.ts frontend/src/lib/services/docsService.ts frontend/src/lib/queryKeys.ts frontend/src/hooks/queries/
git commit -m "feat(docs): add preview HTML service method, type, and query hook"
```

---

### Task 4: Frontend — Create ArticlePreviewPage component

**Files:**
- Create: `frontend/src/pages/docs/ArticlePreviewPage.tsx`

This is the standalone page that renders a document as a help center article. It fetches the preview HTML and help center config, then renders a clean article layout.

- [ ] **Step 1: Create the page component**

```tsx
import { useParams } from '@tanstack/react-router'
import { format, parseISO } from 'date-fns'
import { ArrowLeft, Eye, FileText } from 'lucide-react'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useDocsPreviewHTML, useDocsHelpcenterConfig } from '@/hooks/queries'
import DOMPurify from 'dompurify'

export function ArticlePreviewPage() {
  const { docId } = useParams({ strict: false }) as { docId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''

  const { data: preview, isLoading, error } = useDocsPreviewHTML(wsId, docId)
  const { data: hcConfig } = useDocsHelpcenterConfig(wsId)

  useTitle(preview ? `Preview: ${preview.title}` : 'Article Preview')

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground" />
          <p className="text-sm text-muted-foreground">Loading preview...</p>
        </div>
      </div>
    )
  }

  if (error || !preview) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-background p-4">
        <FileText className="h-16 w-16 text-muted-foreground/20 mb-4" />
        <h1 className="text-lg font-medium text-foreground mb-1">Preview not available</h1>
        <p className="text-sm text-muted-foreground">
          Could not load the article preview. The document may have been deleted.
        </p>
      </div>
    )
  }

  const brandName = hcConfig?.brand_name || workspace?.name || 'Help Center'
  const brandColor = hcConfig?.brand_color || '#2563eb'

  return (
    <div className="min-h-screen bg-background">
      {/* Preview banner */}
      <div className="sticky top-0 z-50 flex items-center gap-2 border-b bg-amber-50 dark:bg-amber-950/30 px-4 py-2">
        <Eye className="h-4 w-4 text-amber-600 dark:text-amber-400" />
        <span className="text-sm font-medium text-amber-700 dark:text-amber-300">
          Preview Mode
        </span>
        <span className="text-xs text-amber-600/70 dark:text-amber-400/60">
          This is how your article will appear in the help center.
          {preview.status === 'draft' && ' This document is still a draft.'}
        </span>
        <button
          type="button"
          onClick={() => window.close()}
          className="ml-auto flex items-center gap-1 rounded px-2 py-1 text-xs text-amber-700 hover:bg-amber-100 dark:text-amber-300 dark:hover:bg-amber-900/40 transition-colors"
        >
          <ArrowLeft className="h-3 w-3" />
          Close preview
        </button>
      </div>

      {/* Help center header mock */}
      <header className="border-b border-border/40 bg-background">
        <div className="mx-auto max-w-4xl px-6 py-4">
          <div className="flex items-center gap-3">
            {hcConfig?.brand_logo_url && (
              <img src={hcConfig.brand_logo_url} alt="" className="h-8 object-contain" />
            )}
            <span className="text-lg font-semibold" style={{ color: brandColor }}>
              {brandName}
            </span>
          </div>
        </div>
      </header>

      {/* Breadcrumb */}
      <div className="mx-auto max-w-4xl px-6 pt-6 pb-2">
        <nav className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span>{brandName}</span>
          {preview.space_name && (
            <>
              <span>/</span>
              <span>{preview.space_name}</span>
            </>
          )}
          {preview.collection_name && (
            <>
              <span>/</span>
              <span>{preview.collection_name}</span>
            </>
          )}
        </nav>
      </div>

      {/* Article content */}
      <main className="mx-auto max-w-4xl px-6 py-6">
        <article>
          <h1 className="text-3xl font-bold tracking-tight text-foreground mb-2">
            {preview.icon && <span className="mr-2">{preview.icon}</span>}
            {preview.title}
          </h1>

          {preview.excerpt && (
            <p className="text-base text-muted-foreground mb-6">{preview.excerpt}</p>
          )}

          {/* Rendered article HTML */}
          <div
            className="tiptap ProseMirror"
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(preview.content_html),
            }}
          />
        </article>

        {/* Feedback section mock */}
        <div className="mt-12 border-t border-border/40 pt-8">
          <div className="text-center">
            <p className="text-sm text-muted-foreground mb-3">Was this article helpful?</p>
            <div className="flex items-center justify-center gap-3">
              <button
                type="button"
                disabled
                className="rounded-lg border border-border px-4 py-2 text-sm text-muted-foreground opacity-60 cursor-not-allowed"
              >
                👍 Yes
              </button>
              <button
                type="button"
                disabled
                className="rounded-lg border border-border px-4 py-2 text-sm text-muted-foreground opacity-60 cursor-not-allowed"
              >
                👎 No
              </button>
            </div>
            <p className="text-[11px] text-muted-foreground/50 mt-2">(Disabled in preview)</p>
          </div>
        </div>
      </main>

      {/* Footer mock */}
      <footer className="border-t border-border/40 mt-8">
        <div className="mx-auto max-w-4xl px-6 py-6 text-center text-xs text-muted-foreground">
          {hcConfig?.footer_config?.copyright_text || `© ${new Date().getFullYear()} ${brandName}`}
        </div>
      </footer>
    </div>
  )
}
```

- [ ] **Step 2: Verify DOMPurify is available**

Check if `dompurify` is already a dependency. If not, install it:

```bash
cd /root/teampulse/frontend && pnpm add dompurify && pnpm add -D @types/dompurify
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/docs/ArticlePreviewPage.tsx
git commit -m "feat(docs): add ArticlePreviewPage component for help center preview"
```

---

### Task 5: Frontend — Create route for article preview

**Files:**
- Create: `frontend/src/routes/_authenticated/w/$slug/docs/documents/$docId.preview.tsx`

TanStack Router file-based routing uses dot notation for nested paths. This creates route: `/w/:slug/docs/documents/:docId/preview`.

- [ ] **Step 1: Create the route file**

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ArticlePreviewPage } from '@/pages/docs/ArticlePreviewPage'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId/preview')({
  component: ArticlePreviewPage,
})
```

- [ ] **Step 2: Regenerate route tree**

The route tree auto-generates when the dev server is running. If needed:

```bash
cd /root/teampulse/frontend && npx tsr generate
```

Verify that `routeTree.gen.ts` now includes the preview route.

- [ ] **Step 3: Verify the route loads in browser**

Navigate to `http://localhost:5173/w/{slug}/docs/documents/{docId}/preview` and confirm the page renders.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/_authenticated/w/\$slug/docs/documents/\$docId.preview.tsx frontend/src/routeTree.gen.ts
git commit -m "feat(docs): add /preview route for article preview page"
```

---

### Task 6: Frontend — Add "Preview" button to document detail toolbar

**Files:**
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx`

- [ ] **Step 1: Add the preview button in the toolbar**

In the top bar section of `DocsDocumentDetail`, add a preview button next to the existing action buttons (after the status badge, before the metadata toggle button):

```tsx
<QuickTooltip label="Preview as help center article">
  <Button
    variant="ghost"
    size="icon"
    className="h-8 w-8 shrink-0"
    onClick={() => {
      const previewUrl = `/w/${wsSlug}/docs/documents/${docId}/preview`
      window.open(previewUrl, '_blank', 'noopener')
    }}
  >
    <Eye className="h-4 w-4" />
  </Button>
</QuickTooltip>
```

Place it between the status badge `<span>` and the publish button or metadata toggle — so it's visible but not dominating the toolbar.

Note: `Eye` is already imported in the file (used for version preview banner).

- [ ] **Step 2: Verify in browser**

1. Open a document in the editor
2. See the Eye icon button in the toolbar
3. Click it — a new tab opens showing the article with help center styling
4. Preview banner shows "Preview Mode"
5. Breadcrumb shows space name and collection name
6. Content renders with proper `.tiptap` CSS styling
7. Feedback section shows disabled buttons
8. Footer shows copyright text
9. "Close preview" button works

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/docs/DocsDocumentDetail.tsx
git commit -m "feat(docs): add preview button to document detail toolbar"
```

---

### Task 7: Handle help center config query hook (if missing)

**Files:**
- Modify: `frontend/src/hooks/queries/` (docs query file)
- Modify: `frontend/src/lib/queryKeys.ts`

- [ ] **Step 1: Check if `useDocsHelpcenterConfig` hook already exists**

Search for `useDocsHelpcenterConfig` in the hooks/queries directory. If it exists, skip this task.

- [ ] **Step 2: If missing, add query key and hook**

Query key in `queryKeys.ts`:
```typescript
helpcenterConfig: (wsId: string) => ['docs', 'helpcenter-config', wsId] as const,
```

Query hook:
```typescript
export function useDocsHelpcenterConfig(wsId: string) {
  return useQuery({
    queryKey: queryKeys.docs.helpcenterConfig(wsId),
    queryFn: async () => unwrap(await docsService.getHelpcenterConfig(wsId)),
    enabled: !!wsId,
    staleTime: 300_000, // 5 min — config rarely changes
  });
}
```

- [ ] **Step 3: Commit if changes were needed**

```bash
git add frontend/src/hooks/queries/ frontend/src/lib/queryKeys.ts
git commit -m "feat(docs): add useDocsHelpcenterConfig query hook"
```

---

### Task 8: End-to-end verification

- [ ] **Step 1: Test with a draft document**

1. Create a new document (or use existing draft)
2. Add various content: headings, paragraphs, images, code blocks, callouts, tables, lists
3. Click the Eye preview button in the toolbar
4. New tab opens with the article preview
5. Verify all content types render correctly with help center styling
6. Verify preview banner shows "draft" notice
7. Verify breadcrumb shows correct space/collection hierarchy

- [ ] **Step 2: Test with a published document**

1. Open a published document
2. Click preview button
3. Verify content matches what the public would see
4. Verify no "draft" notice in the banner

- [ ] **Step 3: Test with empty document**

1. Create a blank document with only a title
2. Click preview
3. Verify graceful handling — title shows, empty content area, no errors

- [ ] **Step 4: Test help center branding**

1. If help center config exists with brand name/logo/color, verify they appear in preview
2. If no config exists, verify fallback to workspace name and default blue color

- [ ] **Step 5: Test dark mode**

1. Toggle dark mode
2. Open preview
3. Verify dark mode styling applies correctly throughout

- [ ] **Step 6: Final commit with any fixes**

```bash
git add -A
git commit -m "fix(docs): article preview polish and edge case handling"
```
