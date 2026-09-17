# Help Center Article Preview v2 — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preview any document (including drafts) inside the actual help-center app using a short-lived JWT preview token, so the preview uses real help center styling with zero CSS duplication.

**Architecture:** The editor's preview button calls an authenticated endpoint to get a signed JWT preview token. It then opens the help-center app at `/preview/{docId}?token=xxx`. The help-center app has a new route that calls a public backend endpoint with the token. The backend validates the JWT, fetches the doc content, renders TipTap→HTML, and returns it. The help-center renders it using its existing ArticleShell + ArticleContent components.

**Tech Stack:** Go (JWT, Chi handler), React (TanStack Router, TanStack Query), existing help-center components.

---

### Task 1: Clean up old preview approach from main frontend

**Files:**
- Modify: `frontend/src/index.css` — remove `hc-prose` CSS block
- Delete: `frontend/src/pages/docs/ArticlePreviewPage.tsx`
- Delete: `frontend/src/routes/_authenticated/w/$slug/docs/documents/$docId/index.tsx`
- Delete: `frontend/src/routes/_authenticated/w/$slug/docs/documents/$docId/preview.tsx`
- Modify: `frontend/src/routes/_authenticated/w/$slug/docs/documents/$docId.tsx` — revert to leaf route
- Modify: `frontend/src/hooks/queries/useDocs.ts` — remove `useDocsPreviewHTML`
- Modify: `frontend/src/lib/queryKeys.ts` — remove `previewHTML` key
- Modify: `frontend/src/lib/services/docsService.ts` — remove `getPreviewHTML`
- Modify: `frontend/src/lib/docsTypes.ts` — remove `PreviewArticleResponse`

Note: Keep the backend `PreviewArticleHTML` service method and `PreviewArticleResponse` model — they'll be reused by the new public endpoint.

- [ ] **Step 1: Remove hc-prose CSS from index.css**

Remove the entire `/* ─── Help Center Article Preview Prose ─── */` block that was added.

- [ ] **Step 2: Delete old preview page + route files**

```bash
rm frontend/src/pages/docs/ArticlePreviewPage.tsx
rm frontend/src/routes/_authenticated/w/\$slug/docs/documents/\$docId/index.tsx
rm frontend/src/routes/_authenticated/w/\$slug/docs/documents/\$docId/preview.tsx
rmdir frontend/src/routes/_authenticated/w/\$slug/docs/documents/\$docId/
```

- [ ] **Step 3: Revert `$docId.tsx` to leaf route**

Replace the Outlet layout with the original leaf route:

```tsx
import { createFileRoute, useParams } from '@tanstack/react-router'
import { DocsDocumentDetail } from '@/pages/docs/DocsDocumentDetail'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/documents/$docId')({
  component: DocDetailRoute,
})

function DocDetailRoute() {
  const { docId } = useParams({ strict: false }) as { docId: string }
  return (
    <div className="h-full overflow-hidden" key={docId}>
      <DocsDocumentDetail />
    </div>
  )
}
```

- [ ] **Step 4: Remove frontend query/service/type artifacts**

In `useDocs.ts`: remove the `useDocsPreviewHTML` function.
In `queryKeys.ts`: remove the `previewHTML` key.
In `docsService.ts`: remove the `getPreviewHTML` method and its comment.
In `docsTypes.ts`: remove the `PreviewArticleResponse` interface.

- [ ] **Step 5: Regenerate route tree and verify build**

```bash
cd frontend && ./node_modules/.bin/tsc --noEmit
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(docs): remove old preview approach (CSS duplication)"
```

---

### Task 2: Backend — Add preview token generation endpoint

**Files:**
- Modify: `server/internal/auth/jwt.go` — add `GeneratePreviewToken` and `ValidatePreviewToken`
- Modify: `server/internal/handler/docs.go` — add `GeneratePreviewToken` handler
- Modify: `server/internal/router/router.go` — register route

The preview token is a JWT containing `doc_id`, `workspace_id`, and a 15-minute expiry. It uses the same `JWT_SECRET` but with a `preview:` subject prefix to distinguish from auth tokens.

- [ ] **Step 1: Add preview token methods to JWTManager**

In `server/internal/auth/jwt.go`, add:

```go
// PreviewClaims contains claims for a document preview token.
type PreviewClaims struct {
	DocID       string `json:"doc_id"`
	WorkspaceID string `json:"workspace_id"`
	jwt.RegisteredClaims
}

// GeneratePreviewToken creates a short-lived JWT for document preview.
func (m *JWTManager) GeneratePreviewToken(docID, workspaceID string) (string, error) {
	claims := PreviewClaims{
		DocID:       docID,
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "preview:" + docID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidatePreviewToken validates a preview JWT and returns its claims.
func (m *JWTManager) ValidatePreviewToken(tokenString string) (*PreviewClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PreviewClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*PreviewClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid preview token")
	}
	return claims, nil
}
```

- [ ] **Step 2: Add handler for generating preview token**

In `server/internal/handler/docs.go`, the handler needs access to `JWTManager`. Check if `DocsHandler` already has it; if not, it must be accessible.

**Decision rule:** If `DocsHandler` doesn't have `jwtManager`, pass it through the helpcenter service or add it directly to the handler struct.

Handler method:

```go
// GeneratePreviewToken creates a short-lived JWT for previewing a document in the help center app.
func (h *DocsHandler) GeneratePreviewToken(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspace_id")
	docID := chi.URLParam(r, "docId")

	// Verify doc exists and belongs to workspace.
	doc, err := h.documentSvc.Get(r.Context(), docID)
	if err != nil || doc == nil || doc.WorkspaceID != wsID {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Get help center config to include subdomain.
	cfg, err := h.helpcenterSvc.GetConfigByWorkspaceID(r.Context(), wsID)
	if err != nil || cfg == nil {
		writeError(w, http.StatusNotFound, "help center not configured")
		return
	}

	token, err := h.jwtManager.GeneratePreviewToken(docID, wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":     token,
		"subdomain": cfg.Subdomain,
	})
}
```

Note: Need to verify `helpcenterSvc.GetConfigByWorkspaceID` exists or find the equivalent method. Also need to add `jwtManager` to `DocsHandler`.

- [ ] **Step 3: Register the route**

In `router.go`, in the docs document routes group:

```go
r.With(requirePerm(authorization.PermDocsRead)).Post("/documents/{docId}/preview-token", h.Docs.GeneratePreviewToken)
```

- [ ] **Step 4: Build and verify**

```bash
cd server && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/auth/jwt.go server/internal/handler/docs.go server/internal/router/router.go server/cmd/api/main.go
git commit -m "feat(docs): add preview token generation endpoint"
```

---

### Task 3: Backend — Add public preview endpoint

**Files:**
- Modify: `server/internal/handler/docs.go` — add `PublicPreviewArticle` handler
- Modify: `server/internal/router/router.go` — register under `/hc/{subdomain}/preview/{docId}`

This is a public endpoint (no auth required) that validates the preview JWT and returns rendered HTML.

- [ ] **Step 1: Add public preview handler**

```go
// PublicPreviewArticle returns a preview of a document for the help center app.
// Requires a valid preview JWT token as query parameter.
func (h *DocsHandler) PublicPreviewArticle(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}

	docID := chi.URLParam(r, "docId")
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "preview token required")
		return
	}

	claims, err := h.jwtManager.ValidatePreviewToken(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired preview token")
		return
	}

	// Verify token matches requested doc and workspace.
	if claims.DocID != docID || claims.WorkspaceID != cfg.WorkspaceID {
		writeError(w, http.StatusForbidden, "token does not match document")
		return
	}

	resp, err := h.helpcenterSvc.PreviewArticleHTML(r.Context(), cfg.WorkspaceID, docID)
	if err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Register route under help center public routes**

In `router.go`, within the `r.Route("/hc/{subdomain}", ...)` block:

```go
r.Get("/preview/{docId}", h.Docs.PublicPreviewArticle)
```

- [ ] **Step 3: Build and verify**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/docs.go server/internal/router/router.go
git commit -m "feat(docs): add public preview endpoint with JWT validation"
```

---

### Task 4: Frontend — Update preview button to use token flow

**Files:**
- Modify: `frontend/src/lib/services/docsService.ts` — add `getPreviewToken`
- Modify: `frontend/src/pages/docs/DocsDocumentDetail.tsx` — update preview button

- [ ] **Step 1: Add preview token service method**

In `docsService.ts`:

```typescript
// ── Preview ────────────────────────────────────────────────────────────
getPreviewToken: (wsId: string, docId: string) =>
  api.post<{ token: string; subdomain: string }>(`/docs/documents/${docId}/preview-token${qs(wsId)}`),
```

- [ ] **Step 2: Update preview button in DocsDocumentDetail**

Replace the current preview button `onClick` with:

```tsx
onClick={async () => {
  try {
    const res = await docsService.getPreviewToken(wsId, docId)
    if (res.error || !res.data) {
      toast.error(res.error || 'Failed to generate preview')
      return
    }
    const hcUrl = import.meta.env.VITE_HELPCENTER_URL || 'http://localhost:5174'
    const { token, subdomain } = res.data
    window.open(
      `${hcUrl}/preview/${docId}?subdomain=${subdomain}&token=${token}`,
      '_blank',
      'noopener',
    )
  } catch {
    toast.error('Failed to generate preview')
  }
}}
```

Also add `docsService` import if not already present:
```tsx
import { docsService } from '@/lib/services/docsService'
```

- [ ] **Step 3: Build and verify**

```bash
cd frontend && ./node_modules/.bin/tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/services/docsService.ts frontend/src/pages/docs/DocsDocumentDetail.tsx
git commit -m "feat(docs): update preview button to use token flow"
```

---

### Task 5: Help Center — Add preview service method and query hook

**Files:**
- Modify: `help-center/src/lib/services.ts` — add `getPreview`
- Modify: `help-center/src/lib/types.ts` — add `PreviewArticleDetail` type
- Modify: `help-center/src/hooks/queries.ts` (or wherever hooks live) — add `usePreviewArticle`

- [ ] **Step 1: Add type**

In `help-center/src/lib/types.ts`:

```typescript
export interface PreviewArticleDetail {
  id: string
  title: string
  excerpt?: string
  icon?: string
  status: string
  collection_id?: string
  collection_name?: string
  space_name: string
  content_html: string
}
```

- [ ] **Step 2: Add service method**

In `help-center/src/lib/services.ts`:

```typescript
getPreview: (subdomain: string, docId: string, token: string) =>
  api.get<PreviewArticleDetail>(
    `/hc/${subdomain}/preview/${docId}?token=${encodeURIComponent(token)}`,
  ),
```

- [ ] **Step 3: Add query hook**

In the help center hooks file:

```typescript
export function usePreviewArticle(subdomain: string, docId: string, token: string) {
  return useQuery({
    queryKey: ['preview', subdomain, docId],
    queryFn: async () => {
      const res = await helpCenterService.getPreview(subdomain, docId, token)
      if (res.error || !res.data) throw new Error(res.error || 'Preview not found')
      return res.data
    },
    enabled: !!subdomain && !!docId && !!token,
    staleTime: 0,
    retry: false,
  })
}
```

- [ ] **Step 4: Commit**

```bash
git add help-center/src/lib/types.ts help-center/src/lib/services.ts help-center/src/hooks/
git commit -m "feat(help-center): add preview article service and hook"
```

---

### Task 6: Help Center — Add preview route and page

**Files:**
- Create: `help-center/src/routes/preview.$docId.tsx`

The preview page renders outside the normal space layout. It uses `ArticleContent` for the body and replicates the ArticleShell layout with a preview banner.

- [ ] **Step 1: Create the preview route**

```tsx
import { createFileRoute, useSearch } from '@tanstack/react-router'
import { useMemo } from 'react'
import { Eye, ArrowLeft } from 'lucide-react'
import { useDocsContext } from '@/contexts/DocsContext'
import { usePreviewArticle } from '@/hooks/queries'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { useScrollSpy } from '@/hooks/useScrollSpy'
import { extractTocFromHtml } from '@/lib/toc'
import { ArticleContent } from '@/components/ArticleContent'
import { TableOfContents } from '@/components/layout/TableOfContents'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'

export const Route = createFileRoute('/preview/$docId')({
  validateSearch: (search: Record<string, unknown>) => ({
    token: (search.token as string) || '',
  }),
  component: PreviewPage,
})

function PreviewPage() {
  const { docId } = Route.useParams()
  const { token } = Route.useSearch()
  const { subdomain } = useDocsContext()

  const { data: article, isLoading, error } = usePreviewArticle(subdomain, docId, token)

  useDocumentTitle(article ? `Preview: ${article.title}` : 'Article Preview')

  const tocItems = useMemo(
    () => (article?.content_html ? extractTocFromHtml(article.content_html) : []),
    [article?.content_html],
  )
  const tocIds = useMemo(() => tocItems.map((item) => item.id), [tocItems])
  const activeHeadingId = useScrollSpy(tocIds)

  if (!token) {
    return (
      <ErrorState
        title="Preview token required"
        message="A valid preview token is needed to view this article preview."
        statusCode={401}
      />
    )
  }

  if (isLoading) return <LoadingState />

  if (error || !article) {
    return (
      <ErrorState
        title="Preview not available"
        message="The preview token may have expired or the document was not found. Go back to the editor and click Preview again."
        statusCode={404}
      />
    )
  }

  return (
    <>
      {/* Preview banner */}
      <div className="sticky top-0 z-50 flex items-center gap-2 border-b bg-amber-50 dark:bg-amber-950/30 px-4 py-2">
        <Eye size={16} className="text-amber-600 dark:text-amber-400" />
        <span className="text-sm font-medium text-amber-700 dark:text-amber-300">
          Preview Mode
        </span>
        <span className="text-xs text-amber-600/70 dark:text-amber-400/60">
          This is how your article will appear in the help center.
          {article.status === 'draft' && ' This document is still a draft.'}
        </span>
        <button
          type="button"
          onClick={() => window.close()}
          className="ml-auto flex items-center gap-1 rounded px-2 py-1 text-xs text-amber-700 hover:bg-amber-100 dark:text-amber-300 dark:hover:bg-amber-900/40 transition-colors"
        >
          <ArrowLeft size={12} />
          Close preview
        </button>
      </div>

      <div className="flex">
        <div className="flex-1 min-w-0">
          <article
            className="mx-auto pt-16 pb-8 px-5 lg:px-6"
            style={{ maxWidth: 'var(--hc-content-max-width)' }}
          >
            {article.collection_name && (
              <div className="mb-2.5">
                <nav className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
                  {article.space_name && (
                    <>
                      <span>{article.space_name}</span>
                      <span className="text-muted-foreground/50">/</span>
                    </>
                  )}
                  <span>{article.collection_name}</span>
                </nav>
              </div>
            )}

            <header className="mb-8">
              <h1 className="text-[1.875rem] font-bold leading-tight tracking-tight mb-2">
                {article.icon && <span className="mr-2">{article.icon}</span>}
                {article.title}
              </h1>
              {article.excerpt && (
                <p className="text-[15px] leading-relaxed text-muted-foreground">
                  {article.excerpt}
                </p>
              )}
            </header>

            <ArticleContent html={article.content_html} />

            {/* Feedback section (disabled in preview) */}
            <div className="mt-12 pt-6 border-t border-border">
              <p className="text-[13px] text-muted-foreground mb-3">
                Was this article helpful?
              </p>
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled
                  className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground opacity-50 cursor-not-allowed"
                >
                  Yes
                </button>
                <button
                  type="button"
                  disabled
                  className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[13px] text-muted-foreground opacity-50 cursor-not-allowed"
                >
                  No
                </button>
              </div>
              <p className="text-[11px] text-muted-foreground/50 mt-2">(Disabled in preview)</p>
            </div>
          </article>
        </div>

        <TableOfContents items={tocItems} activeId={activeHeadingId} />
      </div>
    </>
  )
}
```

- [ ] **Step 2: Verify help center builds**

```bash
cd help-center && pnpm build
```

- [ ] **Step 3: Commit**

```bash
git add help-center/src/routes/preview.\$docId.tsx
git commit -m "feat(help-center): add preview route with token auth and banner"
```

---

### Task 7: Backend — Remove old preview-html endpoint

**Files:**
- Modify: `server/internal/router/router.go` — remove the `/documents/{docId}/preview-html` route
- Modify: `server/internal/handler/docs.go` — remove `PreviewArticleHTML` handler (the one that used workspace auth)

Keep the `helpcenterSvc.PreviewArticleHTML` service method — it's now called by the public preview handler.

- [ ] **Step 1: Remove old route and handler**

In `router.go`: remove the `preview-html` route line.
In `handler/docs.go`: remove the `PreviewArticleHTML` handler function (the workspace-auth one, NOT the public one).

- [ ] **Step 2: Build and verify**

```bash
cd server && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/router/router.go server/internal/handler/docs.go
git commit -m "refactor(docs): remove old workspace-auth preview endpoint"
```

---

### Task 8: Add VITE_HELPCENTER_URL environment variable

**Files:**
- Modify: `frontend/.env.example` (if it exists) or document in code

- [ ] **Step 1: Add env var documentation**

If `.env.example` or `.env` exists in frontend, add:
```
VITE_HELPCENTER_URL=http://91.98.85.12:5174
```

If no env file exists, the code already has a fallback default of `http://localhost:5174`.

- [ ] **Step 2: Commit if changes made**

---

### Task 9: End-to-end verification

- [ ] **Step 1: Restart backend server**
- [ ] **Step 2: Start help center dev server** (`cd help-center && pnpm dev`)
- [ ] **Step 3: Open a draft document in the editor**
- [ ] **Step 4: Click the Eye preview button**
- [ ] **Step 5: Verify**: Help center opens in new tab with preview banner, real help center styling, TOC, breadcrumbs, disabled feedback section
- [ ] **Step 6: Verify token expiry**: Wait 15+ min or tamper with token — should show error
- [ ] **Step 7: Test with published document** — verify no "draft" notice
- [ ] **Step 8: Test with empty document** — verify graceful "No content" message
