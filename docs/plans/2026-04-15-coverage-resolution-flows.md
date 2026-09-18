# Coverage resolution flows implementation plan

This historical plan explains the original draft, apply, and discard workflow for support coverage gaps. Contributors should use the source review below when comparing its example code with the current implementation; its commands and unchecked steps are not a current setup guide.

## Source review — 2026-09-18

- The [draft service](../../server/internal/service/support_coverage_drafts.go) implements Markdown-to-TipTap generation and append-based updates. The existing-article prompt truncates content at 2,000 bytes, not characters. The [append helper](../../server/internal/tiptap/append.go) preserves child nodes but reconstructs the document root with only `type` and `content`; it does not preserve arbitrary root attributes or validate the complete TipTap schema.
- The [repository](../../server/internal/repository/support_coverage.go) inserts a generated suggestion without setting the gap to `drafted`. Applying updates article content, marks the suggestion applied, and marks an open gap `done` through separate operations. These writes are not one transaction, and a later failure can leave earlier writes persisted. The original `fixed` terminology is historical.
- Discard updates the suggestion and reopens its gap in a transaction, but does not enforce the original draft-only/current-gap-state guards. Apply and generation require support and documentation editing permissions; discard requires support editing permission in the [router](../../server/internal/router/router.go).
- The current [gap detail pane](../../frontend/src/components/support/coverage/GapDetailPane.tsx) links to `/docs/documents/:id`. The [coverage page](../../frontend/src/pages/support/coverage/SupportCoveragePage.tsx) also includes the newer topic/finding workflow, so the inline page implementation below is incomplete as a map of the current UI.
- Treat the hard-coded developer directories, Go version, branch pushes, and test expectations below as a historical implementation record. This review inspected source only; it did not generate drafts, mutate articles, or verify a deployed workflow.

## Original implementation plan


**Goal:** Transform the coverage gap detail panel into a resolution workspace with LLM-powered draft generation, inline preview, append-based content updates, and editor links.

**Architecture:** Backend changes are surgical — new `tiptap.AppendContent` utility, Markdown-based LLM output, article title resolution in gap detail DTO, and a discard suggestion endpoint. Frontend is a significant rework of the detail panel into sectioned layout with generate/preview/apply flows, evidence links, and confidence display.

**Tech Stack:** Go 1.24 + Chi + GORM (backend), React 19 + TypeScript + TanStack Query + shadcn/ui (frontend)

**Spec:** `docs/specs/2026-04-15-coverage-resolution-flows-design.md`

---

### Task 1: TipTap AppendContent Utility

**Files:**
- Create: `server/internal/tiptap/append.go`
- Create: `server/internal/tiptap/append_test.go`

- [ ] **Step 1: Write failing tests for AppendContent**

```go
// server/internal/tiptap/append_test.go
package tiptap

import (
	"encoding/json"
	"testing"
)

func TestAppendContent_BothValid(t *testing.T) {
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`)
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"New Section"}]}]}`)

	merged, err := AppendContent(existing, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}

	var doc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}
	if doc.Type != "doc" {
		t.Errorf("expected type=doc, got %q", doc.Type)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 content nodes, got %d", len(doc.Content))
	}
}

func TestAppendContent_ExistingNil(t *testing.T) {
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"New"}]}]}`)
	merged, err := AppendContent(nil, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}
	if string(merged) != string(additions) {
		t.Errorf("expected additions returned as-is")
	}
}

func TestAppendContent_AdditionsNil(t *testing.T) {
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Existing"}]}]}`)
	merged, err := AppendContent(existing, nil)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}
	if string(merged) != string(existing) {
		t.Errorf("expected existing returned as-is")
	}
}

func TestAppendContent_PreservesExistingNodes(t *testing.T) {
	// Existing doc has a resizableImage node — verify it survives the merge.
	existing := json.RawMessage(`{"type":"doc","content":[{"type":"resizableImage","attrs":{"src":"img.png"}},{"type":"paragraph","content":[{"type":"text","text":"Caption"}]}]}`)
	additions := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Appended"}]}]}`)

	merged, err := AppendContent(existing, additions)
	if err != nil {
		t.Fatalf("AppendContent: %v", err)
	}

	var doc struct {
		Content []json.RawMessage `json:"content"`
	}
	json.Unmarshal(merged, &doc)
	if len(doc.Content) != 3 {
		t.Fatalf("expected 3 content nodes (image + caption + appended), got %d", len(doc.Content))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /root/teampulse/server && go test ./internal/tiptap/... -run 'AppendContent' -v -count=1`
Expected: FAIL — `AppendContent` not defined

- [ ] **Step 3: Implement AppendContent**

```go
// server/internal/tiptap/append.go
package tiptap

import (
	"encoding/json"
	"fmt"
)

// AppendContent merges additional TipTap nodes into an existing document.
// Both inputs should be TipTap JSON with { "type": "doc", "content": [...] }.
// Existing content is fully preserved; new nodes are appended after existing ones.
func AppendContent(existing, additions json.RawMessage) (json.RawMessage, error) {
	if len(existing) == 0 || string(existing) == "null" {
		return additions, nil
	}
	if len(additions) == 0 || string(additions) == "null" {
		return existing, nil
	}

	var existingDoc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(existing, &existingDoc); err != nil {
		return nil, fmt.Errorf("parse existing content: %w", err)
	}

	var additionsDoc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(additions, &additionsDoc); err != nil {
		return nil, fmt.Errorf("parse additions: %w", err)
	}

	merged := struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}{
		Type:    "doc",
		Content: append(existingDoc.Content, additionsDoc.Content...),
	}

	return json.Marshal(merged)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /root/teampulse/server && go test ./internal/tiptap/... -run 'AppendContent' -v -count=1`
Expected: all 4 PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/tiptap/append.go server/internal/tiptap/append_test.go
git commit -m "feat: add TipTap AppendContent utility for merging document nodes"
```

---

### Task 2: Refactor LLM Draft Service to Markdown Output

**Files:**
- Modify: `server/internal/service/support_coverage_drafts.go`

- [ ] **Step 1: Replace CoverageArticleDraft and parseDraftResponse**

Replace the `CoverageArticleDraft` struct (line 21-27) with a simpler version that stores Markdown:

```go
// CoverageArticleDraft is the parsed LLM output.
type CoverageArticleDraft struct {
	Title           string
	MarkdownContent string
	EvidenceSummary string
}
```

Replace `parseDraftResponse` (line 376+) with:

```go
func parseDraftResponse(content, fallbackTitle string, evidenceCount int) (*CoverageArticleDraft, error) {
	var raw struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	title := raw.Title
	if title == "" {
		title = fallbackTitle
	}
	return &CoverageArticleDraft{
		Title:           title,
		MarkdownContent: raw.Content,
		EvidenceSummary: fmt.Sprintf("Generated from %d evidence items", evidenceCount),
	}, nil
}
```

- [ ] **Step 2: Update buildDraftPrompt and buildUpdatePrompt**

Replace `buildDraftPrompt` (line 332):

```go
func buildDraftPrompt(title, issueKey string, questions, answers []string) string {
	var b strings.Builder
	b.WriteString("Create a help center article for the following support gap.\n\n")
	b.WriteString(fmt.Sprintf("Topic: %s\n", title))
	if issueKey != "" {
		b.WriteString(fmt.Sprintf("Issue key: %s\n", issueKey))
	}
	if len(questions) > 0 {
		b.WriteString("\nCustomer questions:\n")
		for _, q := range questions[:min(len(questions), 5)] {
			b.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}
	if len(answers) > 0 {
		b.WriteString("\nHuman agent answers:\n")
		for _, a := range answers[:min(len(answers), 3)] {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}
	b.WriteString("\nWrite the full article in Markdown. Use headings, paragraphs, lists, and code blocks as appropriate.")
	b.WriteString("\nRespond with JSON: {\"title\": \"...\", \"content\": \"markdown content here\"}")
	return b.String()
}
```

Replace `buildUpdatePrompt` (line 355):

```go
func buildUpdatePrompt(title string, questions []string, existingContent string) string {
	var b strings.Builder
	b.WriteString("Generate ONLY new sections to add to an existing help center article.\n")
	b.WriteString("Do NOT rewrite or modify existing content. Write sections that will be appended.\n\n")
	b.WriteString(fmt.Sprintf("Topic: %s\n", title))
	if len(questions) > 0 {
		b.WriteString("\nUnanswered customer questions:\n")
		for _, q := range questions[:min(len(questions), 5)] {
			b.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}
	if existingContent != "" {
		truncated := existingContent
		if len(truncated) > 2000 {
			truncated = truncated[:2000] + "..."
		}
		b.WriteString(fmt.Sprintf("\nExisting article content (for context, do not repeat):\n%s\n", truncated))
	}
	b.WriteString("\nWrite new sections in Markdown. Use headings, paragraphs, lists, and code blocks as appropriate.")
	b.WriteString("\nRespond with JSON: {\"title\": \"...\", \"content\": \"markdown content here\"}")
	return b.String()
}
```

- [ ] **Step 3: Update generateDraftFromEvidence and generateUpdateFromEvidence**

In `generateDraftFromEvidence` (line 262), change the call to `parseDraftResponse`:
```go
return parseDraftResponse(resp.Content, detail.Title, len(detail.Evidence))
```

In `generateUpdateFromEvidence` (line 298), same change:
```go
return parseDraftResponse(resp.Content, detail.Title, len(detail.Evidence))
```

- [ ] **Step 4: Update GenerateArticleDraft to store Markdown and TipTap**

In `GenerateArticleDraft` (around line 80), after getting the draft, convert Markdown to TipTap for the suggestion content:

```go
tiptapContent := tiptap.MarkdownToJSON(draft.MarkdownContent)

suggestion := &model.SupportGapSuggestion{
	GapID:              gapID,
	WorkspaceID:        workspaceID,
	SuggestionType:     model.SupportCoverageSuggestionCreateArticle,
	Status:             model.SupportCoverageSuggestionStatusDraft,
	Title:              draft.Title,
	Content:            tiptapContent,
	EvidenceSummary:    draft.EvidenceSummary,
	TargetSpaceID:      &targetSpaceID,
	TargetCollectionID: targetCollectionID,
}
```

Same pattern in `GenerateArticleUpdate` — convert Markdown to TipTap before storing.

- [ ] **Step 5: Update applyUpdateArticle to use AppendContent**

In `applyUpdateArticle` (line 218), change from full replacement to append:

```go
if suggestion.Content != nil {
	existing, err := s.contentSvc.Get(ctx, docID)
	if err != nil {
		return fmt.Errorf("get existing content: %w", err)
	}
	var existingContent json.RawMessage
	if existing != nil {
		existingContent = existing.Content
	}
	merged, err := tiptap.AppendContent(existingContent, suggestion.Content)
	if err != nil {
		return fmt.Errorf("append content: %w", err)
	}
	if _, err := s.contentSvc.Save(ctx, docID, merged, userID); err != nil {
		return fmt.Errorf("save updated content: %w", err)
	}
}
```

Add import for `tiptap` package at top of file.

- [ ] **Step 6: Verify build passes**

Run: `cd /root/teampulse/server && go build ./...`
Expected: clean build

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/support_coverage_drafts.go
git commit -m "refactor: use Markdown LLM output and append-based content updates"
```

---

### Task 3: Enrich Gap Detail with Article Titles

**Files:**
- Modify: `server/internal/model/support_coverage.go` (SupportCoverageGapArticle)
- Modify: `server/internal/repository/support_coverage.go` (GetGapDetail)
- Modify: `frontend/src/lib/supportCoverageTypes.ts`

- [ ] **Step 1: Add ArticleTitle to the Go model**

In `server/internal/model/support_coverage.go`, add a `gorm:"-"` field to `SupportCoverageGapArticle`:

```go
type SupportCoverageGapArticle struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GapID        string    `json:"gap_id" gorm:"type:uuid;not null"`
	DocumentID   string    `json:"document_id" gorm:"type:uuid;not null"`
	WorkspaceID  string    `json:"workspace_id" gorm:"type:uuid;not null"`
	ArticleTitle string    `json:"article_title" gorm:"-"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}
```

- [ ] **Step 2: Resolve article titles in GetGapDetail**

In `server/internal/repository/support_coverage.go`, in `GetGapDetail` (after loading `relatedArticles` around line 224), add title resolution:

```go
var relatedArticles []model.SupportCoverageGapArticle
r.db.WithContext(ctx).
	Where("gap_id = ?", gapID).
	Find(&relatedArticles)

// Resolve article titles from documents table.
for i := range relatedArticles {
	var doc struct{ Title string }
	if err := r.db.WithContext(ctx).Table("docs_documents").Select("title").Where("id = ?", relatedArticles[i].DocumentID).First(&doc).Error; err == nil {
		relatedArticles[i].ArticleTitle = doc.Title
	}
}
```

- [ ] **Step 3: Update frontend type**

In `frontend/src/lib/supportCoverageTypes.ts`, update the `related_articles` type in `SupportCoverageGapDetail`:

```typescript
related_articles: { id: string; gap_id: string; document_id: string; article_title: string }[]
```

- [ ] **Step 4: Verify build**

Run: `cd /root/teampulse/server && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/support_coverage.go server/internal/repository/support_coverage.go frontend/src/lib/supportCoverageTypes.ts
git commit -m "feat: resolve article titles in gap detail for linked article display"
```

---

### Task 4: Discard Suggestion Endpoint

**Files:**
- Modify: `server/internal/handler/support_coverage.go`
- Modify: `server/internal/repository/support_coverage.go`
- Modify: `server/internal/router/router.go`
- Modify: `frontend/src/lib/services/supportCoverageService.ts`

- [ ] **Step 1: Add DiscardSuggestion to repository**

In `server/internal/repository/support_coverage.go`, add after `UpdateSuggestionResult`:

```go
// DiscardSuggestion sets a suggestion to rejected and reverts the gap to open.
func (r *SupportCoverageRepository) DiscardSuggestion(ctx context.Context, suggestionID, workspaceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var suggestion model.SupportGapSuggestion
		if err := tx.Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).First(&suggestion).Error; err != nil {
			return fmt.Errorf("suggestion not found: %w", err)
		}
		if err := tx.Model(&suggestion).Updates(map[string]interface{}{
			"status":     model.SupportCoverageSuggestionStatusRejected,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("reject suggestion: %w", err)
		}
		if err := tx.Model(&model.SupportCoverageGap{}).
			Where("id = ? AND workspace_id = ?", suggestion.GapID, workspaceID).
			Updates(map[string]interface{}{
				"status":     model.SupportCoverageGapStatusOpen,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("revert gap status: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 2: Add handler**

In `server/internal/handler/support_coverage.go`, add:

```go
// DiscardSuggestion handles POST /api/support/coverage/suggestions/{suggestionId}/discard.
func (h *SupportCoverageHandler) DiscardSuggestion(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	suggestionID := chi.URLParam(r, "suggestionId")
	if err := h.coverageSvc.DiscardSuggestion(r.Context(), wsID, suggestionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

Note: `DiscardSuggestion` needs to be added to the service layer as a pass-through, or the handler calls the repo via the coverage service. Add to `SupportCoverageService`:

```go
// DiscardSuggestion rejects a suggestion and reverts the gap to open.
func (s *SupportCoverageService) DiscardSuggestion(ctx context.Context, workspaceID, suggestionID string) error {
	return s.coverageRepo.DiscardSuggestion(ctx, suggestionID, workspaceID)
}
```

- [ ] **Step 3: Wire route**

In `server/internal/router/router.go`, inside the coverage route block, add after the existing `apply` route:

```go
r.With(requirePerm(authorization.PermSupportEdit)).Post("/suggestions/{suggestionId}/discard", h.SupportCoverage.DiscardSuggestion)
```

- [ ] **Step 4: Add frontend service method**

In `frontend/src/lib/services/supportCoverageService.ts`, add:

```typescript
discardSuggestion: (wsId: string, suggestionId: string) =>
  api.post(`/support/coverage/suggestions/${suggestionId}/discard${qs(wsId)}`, {}),
```

- [ ] **Step 5: Verify build**

Run: `cd /root/teampulse/server && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/support_coverage.go server/internal/service/support_coverage.go server/internal/repository/support_coverage.go server/internal/router/router.go frontend/src/lib/services/supportCoverageService.ts
git commit -m "feat: add discard suggestion endpoint to reject and revert gap to open"
```

---

### Task 5: Frontend — Evidence Links, Confidence, Topic Display

**Files:**
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
- Modify: `frontend/src/lib/supportCoverageTypes.ts`

- [ ] **Step 1: Add evidence type label map and confidence helper**

In `SupportCoveragePage.tsx`, add near the top with the other maps:

```tsx
const EVIDENCE_TYPE_LABELS: Record<string, string> = {
  ai_handoff_triggered: 'AI Handoff',
  article_feedback_submitted: 'Article Feedback',
  widget_search_performed: 'Widget Search',
  docs_issue_feedback: 'Agent Feedback',
  human_reply_after_ai: 'Human Reply',
}

function confidenceLabel(confidence: number): { text: string; className: string } {
  if (confidence >= 0.7) return { text: 'High confidence', className: 'text-green-600' }
  if (confidence >= 0.4) return { text: 'Medium confidence', className: 'text-amber-600' }
  return { text: 'Low confidence', className: 'text-muted-foreground' }
}

function formatTopic(issueKey: string): string {
  if (!issueKey) return 'Unknown'
  return issueKey.replace(/[_-]/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}
```

- [ ] **Step 2: Update the detail panel header**

Replace the header section to add confidence label:

After the gap type badge, add:
```tsx
<span className={`text-xs ${confidenceLabel(selectedGap.confidence).className}`}>
  {confidenceLabel(selectedGap.confidence).text}
</span>
```

- [ ] **Step 3: Update metadata to show Topic instead of Issue**

Replace `Issue: {selectedGap.issue_key || 'Unknown'}` with:
```tsx
Topic: {formatTopic(selectedGap.issue_key)}
```

- [ ] **Step 4: Add evidence links**

In the evidence rendering loop, update each evidence item to show human-readable type and links:

```tsx
{selectedGap.evidence.map((ev) => (
  <div key={ev.id} className="rounded-md bg-muted/50 p-2 text-xs">
    <div className="flex items-center justify-between">
      <p className="text-muted-foreground">
        {EVIDENCE_TYPE_LABELS[ev.evidence_type] ?? ev.evidence_type} &middot; {timeAgo(ev.created_at)}
      </p>
      <div className="flex gap-2">
        {ev.conversation_id && (
          <a
            href={`/w/${workspace?.slug}/support/inbox?conversation=${ev.conversation_id}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            View conversation ↗
          </a>
        )}
        {ev.document_id && (
          <a
            href={`/w/${workspace?.slug}/docs/${ev.document_id}`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            View article ↗
          </a>
        )}
      </div>
    </div>
    {ev.excerpt && <p className="mt-1">{ev.excerpt}</p>}
  </div>
))}
```

- [ ] **Step 5: Verify it renders (manual check)**

Refresh the coverage page and click a gap. Verify:
- Confidence label shows next to gap type badge
- Topic shows formatted (not raw issue_key with underscores)
- Evidence items show human-readable type labels
- Evidence with conversation/document IDs show clickable links

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/support/coverage/SupportCoveragePage.tsx
git commit -m "feat: add evidence links, confidence labels, topic display to coverage panel"
```

---

### Task 6: Frontend — Linked Article Section

**Files:**
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`

- [ ] **Step 1: Add linked article section**

After the metadata section and before evidence, add a linked article section for weak/outdated gaps:

```tsx
{/* Linked Article (weak/outdated gaps only) */}
{(selectedGap.v1_gap_type === 'weak_article' || selectedGap.v1_gap_type === 'outdated_or_conflicting_article') &&
  selectedGap.related_articles.length > 0 && (
  <div className="flex items-center justify-between rounded-md border border-border/40 px-3 py-2">
    <div className="flex items-center gap-2 text-sm">
      <FileSearchIcon className="h-4 w-4 text-muted-foreground" />
      <span className="font-medium">{selectedGap.related_articles[0].article_title || 'Untitled article'}</span>
    </div>
    <a
      href={`/w/${workspace?.slug}/docs/${selectedGap.related_articles[0].document_id}`}
      target="_blank"
      rel="noopener noreferrer"
      className="text-xs text-primary hover:underline"
    >
      Open in Editor ↗
    </a>
  </div>
)}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/pages/support/coverage/SupportCoveragePage.tsx
git commit -m "feat: show linked article with editor link for weak/outdated gaps"
```

---

### Task 7: Frontend — Generate Actions (Suggest Improvements + Draft New Article)

**Files:**
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
- Modify: `frontend/src/lib/supportCoverageTypes.ts`

- [ ] **Step 1: Add state variables**

Add to the component state:

```tsx
const [generating, setGenerating] = useState(false)
const [generateError, setGenerateError] = useState<string | null>(null)
const [targetSpaceId, setTargetSpaceId] = useState('')
const [targetCollectionId, setTargetCollectionId] = useState('')
```

- [ ] **Step 2: Add imports for spaces/collections hooks and Select**

```tsx
import { useDocsSpaces, useDocsCollections } from '@/hooks/queries/useDocs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useWorkspaces'
```

Fetch spaces data inside the component:
```tsx
const { data: spaces } = useDocsSpaces(wsId)
const { data: collections } = useDocsCollections(wsId, targetSpaceId)
const { data: access } = useWorkspaceAccess(wsId)
const { has } = usePermissions(access)
const canGenerate = has('support.edit') && has('docs.edit')
```

- [ ] **Step 3: Add generate handlers**

```tsx
const handleSuggestImprovements = async () => {
  if (!selectedGap || selectedGap.related_articles.length === 0) return
  setGenerating(true)
  setGenerateError(null)
  const { data, error } = await supportCoverageService.createArticleUpdate(
    wsId, selectedGap.id,
    { target_document_id: selectedGap.related_articles[0].document_id }
  )
  setGenerating(false)
  if (error) {
    setGenerateError(error)
    return
  }
  // Refresh gap detail to show new suggestion.
  const { data: refreshed } = await supportCoverageService.getGap(wsId, selectedGap.id)
  if (refreshed) setSelectedGap(refreshed)
}

const handleDraftNewArticle = async () => {
  if (!selectedGap || !targetSpaceId) return
  setGenerating(true)
  setGenerateError(null)
  const { data, error } = await supportCoverageService.createArticleDraft(
    wsId, selectedGap.id,
    { target_space_id: targetSpaceId, target_collection_id: targetCollectionId || undefined }
  )
  setGenerating(false)
  if (error) {
    setGenerateError(error)
    return
  }
  const { data: refreshed } = await supportCoverageService.getGap(wsId, selectedGap.id)
  if (refreshed) setSelectedGap(refreshed)
}
```

- [ ] **Step 4: Render generate actions section**

Add after the evidence section, before the status actions bar. Only show when no suggestion exists yet and gap is open/drafted:

```tsx
{/* Generate Actions */}
{selectedGap.status === 'open' && selectedGap.suggestions.length === 0 && canGenerate && (
  <div className="space-y-2">
    {generating ? (
      <div className="flex items-center gap-2 rounded-md border border-border/40 px-3 py-3 text-xs text-muted-foreground">
        <Loading01Icon className="h-4 w-4 animate-spin" />
        Generating suggestions...
      </div>
    ) : generateError ? (
      <div className="rounded-md border border-red-200 bg-red-50 px-3 py-3 text-xs">
        <p className="text-red-700">Could not generate suggestions. This may be due to a temporary service issue.</p>
        <button
          type="button"
          onClick={() => setGenerateError(null)}
          className="mt-1 text-xs font-medium text-primary hover:underline"
        >
          Try Again
        </button>
      </div>
    ) : (selectedGap.v1_gap_type === 'weak_article' || selectedGap.v1_gap_type === 'outdated_or_conflicting_article') && selectedGap.related_articles.length > 0 ? (
      <button
        type="button"
        onClick={handleSuggestImprovements}
        className="w-full rounded-md border border-border/60 px-3 py-2 text-xs font-medium text-primary hover:bg-primary/5"
      >
        Suggest Improvements
      </button>
    ) : selectedGap.v1_gap_type === 'missing_article' ? (
      <div className="space-y-2 rounded-md border border-border/40 p-3">
        <div className="flex gap-2">
          <Select value={targetSpaceId} onValueChange={(v) => { setTargetSpaceId(v); setTargetCollectionId('') }}>
            <SelectTrigger className="h-8 text-xs flex-1">
              <SelectValue placeholder="Select space" />
            </SelectTrigger>
            <SelectContent>
              {spaces?.filter((s) => s.type === 'external_capable').map((s) => (
                <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={targetCollectionId} onValueChange={setTargetCollectionId} disabled={!targetSpaceId}>
            <SelectTrigger className="h-8 text-xs flex-1">
              <SelectValue placeholder="Collection (optional)" />
            </SelectTrigger>
            <SelectContent>
              {collections?.map((c) => (
                <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <button
          type="button"
          onClick={handleDraftNewArticle}
          disabled={!targetSpaceId}
          className="w-full rounded-md bg-primary px-3 py-2 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          Draft New Article
        </button>
      </div>
    ) : null}
  </div>
)}
```

- [ ] **Step 5: Verify it renders (manual check)**

Refresh coverage page. For a weak_article gap: see "Suggest Improvements" button. For a missing_article gap: see space/collection dropdowns + "Draft New Article" button.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/support/coverage/SupportCoveragePage.tsx
git commit -m "feat: add generate actions for draft and improvement flows"
```

---

### Task 8: Frontend — Suggestion Preview, Apply, and Discard

**Files:**
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`

- [ ] **Step 1: Add apply/discard state and handlers**

```tsx
const [applying, setApplying] = useState(false)
const [confirmAction, setConfirmAction] = useState<'apply' | null>(null)

const handleApplySuggestion = async (suggestionId: string) => {
  setApplying(true)
  await supportCoverageService.applySuggestion(wsId, suggestionId)
  setApplying(false)
  setConfirmAction(null)
  // Refresh gap detail and list.
  const { data: refreshed } = await supportCoverageService.getGap(wsId, selectedGap!.id)
  if (refreshed) setSelectedGap(refreshed)
  const { data: listData } = await supportCoverageService.listGaps(wsId, statusFilter ? { status: statusFilter } : undefined)
  if (listData) { setGaps(listData.items || []); setTotal(listData.total || 0) }
}

const handleDiscardSuggestion = async (suggestionId: string) => {
  await supportCoverageService.discardSuggestion(wsId, suggestionId)
  const { data: refreshed } = await supportCoverageService.getGap(wsId, selectedGap!.id)
  if (refreshed) setSelectedGap(refreshed)
}
```

- [ ] **Step 2: Render suggestion preview section**

Add after the generate actions section. Show when a draft suggestion exists:

```tsx
{/* Suggestion Preview */}
{selectedGap.suggestions.filter(s => s.status === 'draft').map((suggestion) => (
  <div key={suggestion.id} className="rounded-md border border-primary/20 bg-primary/5 p-3 space-y-2">
    <h4 className="text-xs font-medium">
      {suggestion.suggestion_type === 'create_article' ? `Draft: ${suggestion.title}` : 'Suggested additions'}
    </h4>
    <div className="max-h-48 overflow-y-auto rounded bg-card p-2 text-xs leading-relaxed whitespace-pre-wrap">
      {suggestion.evidence_summary}
    </div>

    {confirmAction === 'apply' ? (
      <div className="rounded-md border border-border/40 bg-card p-2 text-xs space-y-2">
        <p>
          {suggestion.suggestion_type === 'update_article'
            ? `This will add new sections to "${selectedGap.related_articles[0]?.article_title || 'the article'}". The article will remain unpublished.`
            : `This will create a new draft article "${suggestion.title}".`}
        </p>
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={() => setConfirmAction(null)}
            className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => handleApplySuggestion(suggestion.id)}
            disabled={applying}
            className="rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {applying ? 'Applying...' : suggestion.suggestion_type === 'update_article' ? 'Apply Changes' : 'Create Draft'}
          </button>
        </div>
      </div>
    ) : (
      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={() => handleDiscardSuggestion(suggestion.id)}
          className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
        >
          Discard
        </button>
        <button
          type="button"
          onClick={() => setConfirmAction('apply')}
          className="rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
        >
          {suggestion.suggestion_type === 'update_article' ? 'Apply to Article' : 'Create as Draft in Docs'}
        </button>
      </div>
    )}
  </div>
))}

{/* Post-Apply State */}
{selectedGap.suggestions.filter(s => s.status === 'applied').map((suggestion) => (
  <div key={suggestion.id} className="rounded-md border border-green-200 bg-green-50 px-3 py-2 text-xs">
    <div className="flex items-center justify-between">
      <p className="text-green-700 font-medium">
        {suggestion.suggestion_type === 'update_article'
          ? `Sections added to "${selectedGap.related_articles[0]?.article_title || 'article'}"`
          : `Draft created: "${suggestion.title}"`}
      </p>
      {suggestion.result_document_id && (
        <a
          href={`/w/${workspace?.slug}/docs/${suggestion.result_document_id}`}
          target="_blank"
          rel="noopener noreferrer"
          className="text-primary hover:underline font-medium"
        >
          Open in Editor ↗
        </a>
      )}
    </div>
  </div>
))}
```

- [ ] **Step 3: Verify full flow (manual check)**

1. Open a weak_article gap → click "Suggest Improvements" → see loading → see preview → click "Apply to Article" → see confirmation → confirm → see success with "Open in Editor" link
2. Open a missing_article gap → select space → click "Draft New Article" → see preview → click "Create as Draft in Docs" → confirm → see success
3. Test discard: generate → click "Discard" → preview disappears, generate button returns

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/support/coverage/SupportCoveragePage.tsx
git commit -m "feat: add suggestion preview, apply confirmation, and discard flow"
```

---

### Task 9: Final Polish and Integration Test

**Files:**
- All modified files from previous tasks

- [ ] **Step 1: Run full backend test suite**

Run: `cd /root/teampulse/server && go test ./internal/service/... -run 'SupportCoverage' -count=1 -timeout 30s`
Run: `cd /root/teampulse/server && go test ./internal/repository/... -run 'SupportCoverage' -count=1 -timeout 30s`
Run: `cd /root/teampulse/server && go test ./internal/tiptap/... -count=1 -timeout 30s`
Expected: all PASS

- [ ] **Step 2: Run go vet and build**

Run: `cd /root/teampulse/server && go vet ./... && go build ./...`
Expected: clean (ignore pre-existing warnings in worker/)

- [ ] **Step 3: Frontend type check**

Run: `cd /root/teampulse/frontend && npx tsc --noEmit`
Expected: no new errors in coverage files

- [ ] **Step 4: Restart server and seed test data**

Restart the server and use the existing seed script to create gaps of each type (missing_article, weak_article, needs_review). Test the full flow end-to-end if LLM is configured.

- [ ] **Step 5: Final commit and push**

```bash
git add -A
git commit -m "feat: complete coverage resolution flows — generate, preview, apply, discard"
git push origin waqar-work
```
