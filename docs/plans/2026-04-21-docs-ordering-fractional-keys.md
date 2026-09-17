# Docs Ordering — Fractional Sort Keys Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace position-based ordering with fractional sort keys so collections and docs can be freely interleaved within any bucket, and all three surfaces (internal list, arrange mode, public help center) render identical order.

**Architecture:** Add a `sort_key TEXT` column to both `docs_documents` and `docs_collections`. Vendor the dgreensp fractional-indexing algorithm (Go + TS). Gate reads/writes behind `DOCS_ORDERING_USE_SORT_KEY` env flag. Backfill existing rows from their current position order. All ORDER BY clauses switch to `sort_key ASC, id ASC` when the flag is on.

**Tech Stack:** Go 1.24, GORM, PostgreSQL, TypeScript, TanStack Query, Vitest

**Spec:** `docs/specs/2026-04-16-docs-ordering-fractional-keys-design.md`

---

## File Structure

### New files (create)

| File | Responsibility |
|------|---------------|
| `server/internal/ordering/fractional.go` | Fractional indexing `Between(lower, upper)` function |
| `server/internal/ordering/fractional_test.go` | Unit tests + stress test for fractional indexing |
| `server/internal/ordering/testdata/vectors.json` | Shared Go↔TS test vectors |
| `server/cmd/backfill-docs-ordering/main.go` | One-shot backfill command |
| `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_docs_sort_key_indexes.sql` | Indexes for bucket-scoped reads |
| `server/internal/dbmigrate/sql/YYYYMMDDNNNN_docs_sort_key_check_constraint.sql` | Post-backfill CHECK constraint |
| `frontend/src/lib/fractionalIndex.ts` | TS port of the fractional algorithm |
| `frontend/src/lib/__tests__/fractionalIndex.test.ts` | TS tests loading shared vectors |
| `frontend/src/lib/docsTreeMerge.ts` | Merge collections + docs by sort_key for rendering |

### Modified files

| File | What changes |
|------|-------------|
| `server/internal/model/docs.go:152-208` | Add `SortKey` field to `DocsCollection` and `DocsDocument` structs |
| `server/internal/config/config.go` | Add `DocsOrderingUseSortKey bool` flag |
| `server/cmd/api/main.go:837-838` | Pass config flag to services |
| `server/internal/repository/docs_document.go:71-330` | Add bucket helpers; update ORDER BY behind flag |
| `server/internal/repository/docs_collection.go:100-692` | Same; update `ReorderChildren` to write sort_key |
| `server/internal/repository/docs_helpcenter.go:753-1545` | Update ORDER BY in nav + article queries behind flag |
| `server/internal/service/docs_document.go:47,479` | Set sort_key on create; compute sort_keys in reorder shim |
| `server/internal/service/docs_collection.go:91,503,524` | Same for collections; update reparent logic |
| `server/internal/service/docs_import_nextra.go` | Set sort_key sequentially during import |
| `server/internal/handler/docs.go:508-570` | Add new move handler; existing reorder handlers unchanged |
| `server/internal/router/router.go:932-935` | Register new move route |
| `frontend/src/lib/services/docsService.ts:246-254` | Add `moveItem()` API call |
| `frontend/src/hooks/queries/useDocs.ts` | Add `useMoveDocsItem()` mutation hook |
| `frontend/src/components/docs/DocsArrangeTree.tsx` | Update drag handler to use sort_key merge + move endpoint |

---

## Task 1: Fractional Indexing Library (Go)

**Files:**
- Create: `server/internal/ordering/fractional.go`
- Create: `server/internal/ordering/fractional_test.go`
- Create: `server/internal/ordering/testdata/vectors.json`

This task is pure algorithm with zero dependencies on the rest of the codebase.

- [ ] **Step 1: Create the test vectors file**

Create `server/internal/ordering/testdata/vectors.json` with test triples. Generate expected values from the reference implementation at `https://github.com/rocicorp/fractional-indexing` (run in Node to produce the expected output for each input pair). Include all categories from spec §6.3.1:

```json
[
  {"lower": "", "upper": "", "expected": "<from-reference>"},
  {"lower": "", "upper": "c", "expected": "<from-reference>"},
  {"lower": "a", "upper": "b", "expected": "<from-reference>"},
  {"lower": "m", "upper": "n", "expected": "<from-reference>"},
  {"lower": "y", "upper": "", "expected": "<from-reference>"},
  {"lower": "z", "upper": "", "expected": "<from-reference>"},
  {"lower": "n", "upper": "m", "error": true},
  {"lower": "A", "upper": "z", "error": true}
]
```

Minimum 50 triples covering: empty bounds, both bounds, tight-upper, reverse (error), invalid chars (error), long keys (5+ chars).

- [ ] **Step 2: Write failing tests**

Create `server/internal/ordering/fractional_test.go`:

```go
package ordering

import (
    "encoding/json"
    "os"
    "testing"
)

type testVector struct {
    Lower    string `json:"lower"`
    Upper    string `json:"upper"`
    Expected string `json:"expected"`
    Error    bool   `json:"error"`
}

func loadVectors(t *testing.T) []testVector {
    t.Helper()
    data, err := os.ReadFile("testdata/vectors.json")
    if err != nil {
        t.Fatalf("load vectors: %v", err)
    }
    var vectors []testVector
    if err := json.Unmarshal(data, &vectors); err != nil {
        t.Fatalf("parse vectors: %v", err)
    }
    return vectors
}

func TestBetween_Vectors(t *testing.T) {
    for _, v := range loadVectors(t) {
        t.Run(v.Lower+"_"+v.Upper, func(t *testing.T) {
            got, err := Between(v.Lower, v.Upper)
            if v.Error {
                if err == nil {
                    t.Fatalf("expected error, got %q", got)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if got != v.Expected {
                t.Errorf("Between(%q, %q) = %q, want %q", v.Lower, v.Upper, got, v.Expected)
            }
        })
    }
}

func TestBetween_StressOrdering(t *testing.T) {
    // Insert 1000 items at random positions; verify strict ordering.
    keys := make([]string, 0, 1000)
    keys = append(keys, mustBetween(t, "", ""))
    for i := 1; i < 1000; i++ {
        pos := i % len(keys) // insert at varied positions
        lower, upper := "", ""
        if pos > 0 { lower = keys[pos-1] }
        if pos < len(keys) { upper = keys[pos] }
        key, err := Between(lower, upper)
        if err != nil {
            t.Fatalf("insert %d between %q and %q: %v", i, lower, upper, err)
        }
        // Insert key at pos.
        keys = append(keys, "")
        copy(keys[pos+1:], keys[pos:])
        keys[pos] = key
    }
    // Verify sorted.
    for i := 1; i < len(keys); i++ {
        if keys[i-1] >= keys[i] {
            t.Fatalf("out of order at %d: %q >= %q", i, keys[i-1], keys[i])
        }
    }
    // Verify max key length.
    maxLen := 0
    for _, k := range keys {
        if len(k) > maxLen { maxLen = len(k) }
    }
    if maxLen > 30 {
        t.Errorf("max key length %d exceeds expected ceiling", maxLen)
    }
}

func mustBetween(t *testing.T, lower, upper string) string {
    t.Helper()
    k, err := Between(lower, upper)
    if err != nil {
        t.Fatalf("Between(%q, %q): %v", lower, upper, err)
    }
    return k
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd server && go test ./internal/ordering/ -v 2>&1 | head -5
```

Expected: compilation error (package/function not found).

- [ ] **Step 4: Implement `Between` function**

Create `server/internal/ordering/fractional.go`. Port the dgreensp algorithm from `https://github.com/rocicorp/fractional-indexing/blob/main/src/index.ts` to Go, restricting the digit alphabet to `a-z` (26 chars). Key points:

- Use `a` as the integer-part encoding for values in range [0, 26). The dgreensp format uses separate integer and fractional parts; we simplify to fractional-only since we don't need large-magnitude integers — all our keys are small ordinals within a single bucket.
- Return `(string, error)`, never panic.
- Validate inputs: both must match `^[a-z]*$`; `lower < upper` when both non-empty.

```go
// Package ordering provides fractional-indexing sort keys for
// docs/collections ordering. Port of the dgreensp algorithm
// (https://github.com/rocicorp/fractional-indexing).
package ordering

import (
    "errors"
    "strings"
)

// Between returns a sort key k such that lower < k < upper
// lexicographically. Empty string denotes "no bound on this side."
func Between(lower, upper string) (string, error) {
    // ... port from reference implementation ...
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd server && go test ./internal/ordering/ -v -count=1
```

Expected: all vector tests pass; stress test passes with max key length < 30.

- [ ] **Step 6: Commit**

```bash
git add server/internal/ordering/
git commit -m "feat(ordering): add fractional sort-key library with shared test vectors"
```

---

## Task 2: Schema — Add `sort_key` Column + Indexes

**Files:**
- Modify: `server/internal/model/docs.go:152-208`
- Create: `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_docs_sort_key_indexes.sql`

- [ ] **Step 1: Add SortKey field to both model structs**

In `server/internal/model/docs.go`, add after the `Position` field on both structs:

```go
// DocsCollection struct (~line 161, after Position field):
SortKey  string `json:"sort_key" gorm:"not null;default:'~'"`

// DocsDocument struct (~line 185, after Position field):
SortKey  string `json:"sort_key" gorm:"not null;default:'~'"`
```

- [ ] **Step 2: Create index migration**

Create `server/internal/dbmigrate/sql/202604210001_add_docs_sort_key_indexes.sql`:

```sql
-- Bucket-scoped read indexes for sort_key ordering.
-- No WHERE clause to avoid bloat from soft-delete/restore churn.
CREATE INDEX IF NOT EXISTS idx_docs_documents_bucket_sort
  ON docs_documents (workspace_id, space_id, collection_id, sort_key, id);

CREATE INDEX IF NOT EXISTS idx_docs_collections_bucket_sort
  ON docs_collections (workspace_id, space_id, parent_collection_id, sort_key, id);
```

- [ ] **Step 3: Verify build + AutoMigrate picks up the column**

```bash
cd server && go build ./... && go run ./cmd/api 2>&1 | grep -i "auto\|migrat" | head -5
```

Expected: AutoMigrate adds `sort_key` column to both tables. Kill the server after confirming.

- [ ] **Step 4: Verify migration CLI picks up the index migration**

```bash
cd server && go run ./cmd/migrate pending
```

Expected: shows the new migration as pending.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/docs.go server/internal/dbmigrate/sql/202604210001_add_docs_sort_key_indexes.sql
git commit -m "schema: add sort_key column + bucket indexes to docs tables"
```

---

## Task 3: Feature Flag

**Files:**
- Modify: `server/internal/config/config.go`

- [ ] **Step 1: Add the flag to Config struct**

In `server/internal/config/config.go`, add to the Config struct:

```go
DocsOrderingUseSortKey bool
```

In the `Load()` function's `return &Config{...}` struct literal (~line 148), add:

```go
DocsOrderingUseSortKey: parseBoolEnv(os.Getenv("DOCS_ORDERING_USE_SORT_KEY")),
```

Note: `parseBoolEnv` takes a single string (not a default arg). Returns `true` for `"1"`, `"true"`, `"yes"`, `"on"`; false otherwise. This means env var absent = false, which is the desired default.

- [ ] **Step 2: Verify build**

```bash
cd server && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/config/config.go
git commit -m "config: add DOCS_ORDERING_USE_SORT_KEY feature flag"
```

---

## Task 4: Repository — Bucket Helpers + ORDER BY Updates

**Files:**
- Modify: `server/internal/repository/docs_document.go`
- Modify: `server/internal/repository/docs_collection.go`
- Modify: `server/internal/repository/docs_helpcenter.go`

This is the largest task. Every ORDER BY clause that sorts docs or collections by position needs a feature-flag-gated alternative.

- [ ] **Step 1: Add `useSortKey` field to repositories**

Both `DocsDocumentRepository` and `DocsCollectionRepository` need a `useSortKey bool` field. Two options: (a) add to constructor — requires updating all `New...Repository(db)` call sites in `cmd/api/main.go` to `New...Repository(db, cfg.DocsOrderingUseSortKey)`, or (b) add a `SetUseSortKey(bool)` setter called after construction. Prefer (a) since it makes the dependency explicit. Find the `NewDocsDocumentRepository` and `NewDocsCollectionRepository` calls in `cmd/api/main.go` and add the bool param.

For `DocsHelpcenterRepository`, same pattern — add to constructor.

For `DocsHelpcenterRepository`, same pattern — add the field.

- [ ] **Step 2: Add `orderByClause()` helper to each repository**

```go
func (r *DocsDocumentRepository) orderByClause() string {
    if r.useSortKey {
        return "sort_key ASC, id ASC"
    }
    return "position ASC, created_at ASC, id ASC"
}
```

Same for collection repository. Use this helper in every query.

- [ ] **Step 3: Update `docs_document.go` — `List()` ORDER BY**

At line ~97, replace the hardcoded ORDER BY with:

```go
if spaceID != nil && *spaceID != "" {
    orderBy := "collection_id ASC NULLS FIRST, " + r.orderByClause()
    if r.useSortKey {
        orderBy = "collection_id ASC NULLS FIRST, sort_key ASC, id ASC"
    }
    query = query.Order(orderBy)
} else {
    query = query.Order("is_pinned DESC, updated_at DESC")
}
```

- [ ] **Step 4: Update `docs_collection.go` — all ORDER BY occurrences**

Replace ORDER BY at lines ~104, 116, 165, 208, 333, 652 with the `r.orderByClause()` helper (or conditional). Key patterns:

- `ListBySpace` (line 104): `r.orderByClause()`
- `ListByWorkspace` (line 116): `"space_id ASC, " + r.orderByClause()`
- Nested trees (lines 165, 208, 333, 652): `r.orderByClause()`

- [ ] **Step 5: Update `docs_helpcenter.go` — all ORDER BY occurrences**

Replace ORDER BY at lines ~761, 785, 341, 468, 647, 970 with the flag-gated version. For collection queries, use the collection `orderByClause`. For article/doc queries, use the doc `orderByClause`.

Key changes:
- Line 761 (collections in `ListSpaceNavigation`): when `useSortKey`, change to `"sort_key ASC, id ASC"` (drop `depth ASC, parent_collection_id ASC` prefix — client builds the tree anyway).
- Line 785 (articles in `ListSpaceNavigation`): `"d.sort_key ASC, d.id ASC"` when flag is on.
- Line 468 and 647: same pattern.

- [ ] **Step 6: Wire `useSortKey` in `cmd/api/main.go`**

Pass `cfg.DocsOrderingUseSortKey` to repository constructors or use a setter method. At lines ~837-838:

```go
docsDocRepo.SetUseSortKey(cfg.DocsOrderingUseSortKey)
docsCollectionRepo.SetUseSortKey(cfg.DocsOrderingUseSortKey)
docsHelpcenterRepo.SetUseSortKey(cfg.DocsOrderingUseSortKey)
```

- [ ] **Step 7: Verify build**

```bash
cd server && go build ./...
```

- [ ] **Step 8: Run existing tests**

```bash
cd server && go test ./internal/repository/... ./internal/service/... -count=1 2>&1 | tail -5
```

Expected: all existing tests pass (flag defaults to false, no behavior change).

- [ ] **Step 9: Commit**

```bash
git add server/internal/repository/docs_document.go \
        server/internal/repository/docs_collection.go \
        server/internal/repository/docs_helpcenter.go \
        server/cmd/api/main.go
git commit -m "feat(docs-ordering): flag-gated sort_key ORDER BY in all doc/collection queries"
```

---

## Task 5: Service — Creation Paths Set `sort_key`

**Files:**
- Modify: `server/internal/service/docs_document.go:47`
- Modify: `server/internal/service/docs_collection.go:91`
- Modify: `server/internal/service/docs_import_nextra.go`

- [ ] **Step 1: Add `useSortKey` field to services**

Both `DocsDocumentService` and `DocsCollectionService` need the flag. Add field + setter.

- [ ] **Step 2: Update `CreateDocument` to assign sort_key**

In `docs_document.go` `Create()` method (~line 47), after setting position, add:

```go
if s.useSortKey {
    // Find the last sort_key in the target bucket.
    lastKey, err := s.docRepo.LastSortKeyInBucket(ctx, doc.SpaceID, doc.CollectionID)
    if err != nil {
        return nil, fmt.Errorf("last sort key: %w", err)
    }
    newKey, err := ordering.Between(lastKey, "")
    if err != nil {
        return nil, fmt.Errorf("compute sort key: %w", err)
    }
    doc.SortKey = newKey
}
```

Add `LastSortKeyInBucket(ctx, spaceID string, collectionID *string) (string, error)` to the document repository.

- [ ] **Step 3: Update `CreateCollection` to assign sort_key**

Same pattern in `docs_collection.go` `Create()` (~line 91). Add `LastSortKeyInBucket` to the collection repository.

- [ ] **Step 4: Update Nextra importer**

In `docs_import_nextra.go`, the function `nextraContentToTiptapJSON` / the import execution path (~line 449) creates documents via `s.contentSvc`. Also check `nextra_adapter.go:280` where `RawContent` is assigned — the document creation happens in a loop there. After setting `position`, also compute `sort_key` using `Between(prevKey, "")` sequentially. Track `prevKey` per bucket (per collection, per space root).

**Critical constraint:** The importer creates many items in the same millisecond during batch import. Sort keys MUST be assigned sequentially within the transaction, never in parallel goroutines, even if items are created concurrently elsewhere. The loop-based creation in the Nextra adapter already runs sequentially — just add the `prevKey` tracker alongside the existing `position` counter.

- [ ] **Step 5: Write a test for creation with sort_key**

In `server/internal/service/docs_ordering_test.go` (or a new test file), add a test that creates 5 documents in a collection with `useSortKey=true` and verifies their sort_keys are strictly ascending.

- [ ] **Step 6: Run tests**

```bash
cd server && go test ./internal/service/... -run "SortKey" -v -count=1
```

- [ ] **Step 7: Commit**

```bash
git add server/internal/service/docs_document.go \
        server/internal/service/docs_collection.go \
        server/internal/service/docs_import_nextra.go \
        server/internal/repository/docs_document.go \
        server/internal/repository/docs_collection.go
git commit -m "feat(docs-ordering): assign sort_key on document/collection creation"
```

---

## Task 6: Service — Reorder Shims

**Files:**
- Modify: `server/internal/service/docs_document.go:479`
- Modify: `server/internal/service/docs_collection.go:503,524`
- Modify: `server/internal/repository/docs_document.go:250`
- Modify: `server/internal/repository/docs_collection.go:433,475`

- [ ] **Step 1: Update `ReorderDocuments` to compute sort_keys**

In `docs_document.go` `ReorderDocuments()` (~line 479), after the existing position-based reorder, add:

```go
if s.useSortKey {
    // Rebuild sort_keys for every item in the submitted list from scratch.
    prevKey := ""
    for _, id := range req.OrderedIDs {
        key, err := ordering.Between(prevKey, "")
        if err != nil {
            return fmt.Errorf("compute sort key for reorder: %w", err)
        }
        if err := s.docRepo.UpdateSortKey(ctx, id, key); err != nil {
            return fmt.Errorf("update sort key: %w", err)
        }
        prevKey = key
    }
}
```

Add `UpdateSortKey(ctx, id, key string) error` to the document repository.

- [ ] **Step 2: Update `ReorderCollections` to compute sort_keys**

Same pattern in `docs_collection.go` `ReorderCollections()` (~line 503).

- [ ] **Step 3: Update `ReorderChildren` to compute sort_keys**

This is the mixed-type reorder. In `ReorderChildren()` (~line 524), after the existing position logic, iterate the ordered children list and assign sort_keys sequentially. Each child is `{Type, ID}` — update the correct table based on type.

- [ ] **Step 4: Write reorder test with sort_key verification**

Test that after calling `ReorderDocuments` with `useSortKey=true`, the docs have ascending sort_keys matching the submitted order.

- [ ] **Step 5: Run tests**

```bash
cd server && go test ./internal/service/... -run "Reorder.*SortKey" -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/docs_document.go \
        server/internal/service/docs_collection.go \
        server/internal/repository/docs_document.go \
        server/internal/repository/docs_collection.go
git commit -m "feat(docs-ordering): reorder shims compute sort_keys from scratch"
```

---

## Task 7: Handler + Route — New Move Endpoint

**Files:**
- Modify: `server/internal/handler/docs.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/internal/service/docs_document.go` or create `server/internal/service/docs_move.go`
- Modify: `server/internal/model/docs.go` (add request/response DTOs)

- [ ] **Step 1: Add request/response model**

In `server/internal/model/docs.go`, add:

```go
type MoveDocsItemRequest struct {
    Item         MoveDocsItemRef    `json:"item"`
    TargetBucket MoveDocsBucketRef  `json:"target_bucket"`
    Position     MoveDocsPositionRef `json:"position"`
}

type MoveDocsItemRef struct {
    Type string `json:"type"` // "doc" or "collection"
    ID   string `json:"id"`
}

type MoveDocsBucketRef struct {
    SpaceID            string  `json:"space_id"`
    ParentCollectionID *string `json:"parent_collection_id"`
}

type MoveDocsPositionRef struct {
    Before *MoveDocsItemRef `json:"before"`
    After  *MoveDocsItemRef `json:"after"`
}
```

- [ ] **Step 2: Implement the move service method**

Create the service method (in `docs_document.go` or a new `docs_move.go`):

```go
func (s *DocsDocumentService) MoveItem(ctx context.Context, wsID string, req model.MoveDocsItemRequest) error {
    // 1. Authz: verify PermDocsEdit on current space AND target space.
    // 2. Cross-space check: reject if different spaces.
    // 3. Load before/after neighbor sort_keys (SELECT ... FOR UPDATE inside tx).
    // 4. Compute new sort_key via ordering.Between.
    // 5. Update the item's sort_key (and parent/collection_id if bucket differs).
    return nil
}
```

- [ ] **Step 3: Implement the handler**

In `server/internal/handler/docs.go`, add:

```go
func (h *DocsHandler) MoveItem(w http.ResponseWriter, r *http.Request) {
    wsID := middleware.GetWorkspaceID(r.Context())
    var req model.MoveDocsItemRequest
    if err := decodeJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request")
        return
    }
    if err := h.documentSvc.MoveItem(r.Context(), wsID, req); err != nil {
        // Use sentinel errors to distinguish status codes:
        // - ErrCrossSpaceMove (400) — cross-space moves are not supported
        // - ErrStaleNeighbors (409) — neighbors changed since client read them; client retries
        // - ErrBetweenFailed (409) — no valid key between the given neighbors
        // Follow the existing writeDocsError helper pattern at top of docs.go.
        if errors.Is(err, service.ErrCrossSpaceMove) {
            writeError(w, http.StatusBadRequest, "cross-space moves not supported")
            return
        }
        if errors.Is(err, service.ErrStaleNeighbors) || errors.Is(err, service.ErrBetweenFailed) {
            writeError(w, http.StatusConflict, "neighbors changed; retry")
            return
        }
        writeError(w, http.StatusInternalServerError, "move failed")
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 4: Register the route**

In `server/internal/router/router.go`, inside the workspace docs route group (near line ~935 where the other reorder routes live). The `/docs/` prefix is already part of the parent group, so register as:

```go
r.With(requirePerm(authorization.PermDocsEdit)).Post("/items/move", h.Docs.MoveItem)
```

Use `Post` (not `Put`) per the spec's `POST /api/workspaces/:wsID/docs/items/move` contract.

- [ ] **Step 5: Write handler test**

Test the endpoint with valid move request + verify 409 on stale neighbors.

- [ ] **Step 6: Run tests**

```bash
cd server && go build ./... && go test ./internal/handler/... -run "MoveItem" -v -count=1
```

- [ ] **Step 7: Commit**

```bash
git add server/internal/model/docs.go \
        server/internal/service/docs_move.go \
        server/internal/handler/docs.go \
        server/internal/router/router.go
git commit -m "feat(docs-ordering): add POST /docs/items/move endpoint"
```

---

## Task 8: Service — Soft-Delete Reparent

**Files:**
- Modify: `server/internal/repository/docs_collection.go` (Delete method, ~line 145)

- [ ] **Step 1: Update reparent logic to compute sort_keys**

In the `Delete()` transaction (~line 145), after reparenting child collections and docs to the grandparent bucket, append fresh sort_keys for each reparented item:

```go
if r.useSortKey {
    // Compute sort_keys for reparented items by appending to destination bucket.
    lastKey, err := r.lastSortKeyInBucket(tx, coll.SpaceID, destParentID)
    if err != nil {
        return fmt.Errorf("last sort key in destination bucket: %w", err)
    }
    for _, child := range children {
        key, err := ordering.Between(lastKey, "")
        if err != nil {
            return fmt.Errorf("compute sort key for reparented collection %s: %w", child.ID, err)
        }
        if err := tx.Model(&model.DocsCollection{}).Where("id = ?", child.ID).Update("sort_key", key).Error; err != nil {
            return fmt.Errorf("update sort key for collection %s: %w", child.ID, err)
        }
        lastKey = key
    }
    // Same pattern for reparented docs — load direct-child docs of deleted
    // collection, append sort_keys sequentially after the reparented collections.
}
```

**Important:** Never discard errors from `ordering.Between` or GORM updates — per `server/CLAUDE.md`'s "never discard errors" rule.

- [ ] **Step 2: Write test for delete + reparent sort_key continuity**

Test: create parent → child collection → docs; delete parent; verify child and docs have valid sort_keys in the grandparent bucket that sort after existing items.

- [ ] **Step 3: Commit**

```bash
git add server/internal/repository/docs_collection.go
git commit -m "feat(docs-ordering): compute sort_keys on soft-delete reparent"
```

---

## Task 9: Backfill Command

**Files:**
- Create: `server/cmd/backfill-docs-ordering/main.go`
- Create: `server/internal/dbmigrate/sql/202604210002_docs_ordering_backfill_marker.sql`

- [ ] **Step 1: Create the command skeleton**

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log/slog"
    "os"

    "github.com/helpin-ai/helpin/server/internal/config"
    "github.com/helpin-ai/helpin/server/internal/ordering"
    "github.com/helpin-ai/helpin/server/internal/model"
    // ... DB setup imports
)

func main() {
    dryRun := flag.Bool("dry-run", false, "compute keys without writing")
    wsID := flag.String("workspace-id", "", "single workspace (empty = all)")
    flag.Parse()
    // ... DB connect, iterate workspaces, walk tree, assign sort_keys
}
```

- [ ] **Step 2: Implement the tree-walk backfill**

For each workspace → each space → DFS walk from space root:
1. Load collections in bucket (by position, created_at, id).
2. Load docs in bucket (by position, created_at, id).
3. Assign collections first, then docs, using sequential `Between(prevKey, "")`.
4. Skip rows where `sort_key != '~'` (idempotent per-row).
5. Recurse into each collection.

- [ ] **Step 3: Create marker migration**

Create `server/internal/dbmigrate/sql/202604210002_docs_ordering_backfill_marker.sql`:

```sql
-- Marker migration. The actual backfill is a one-shot Go command
-- (cmd/backfill-docs-ordering). This migration exists so the
-- migrations table records that the backfill step belongs here
-- in the sequence.
SELECT 1;
```

- [ ] **Step 4: Test on local DB**

```bash
cd server && go run ./cmd/backfill-docs-ordering --dry-run 2>&1 | head -20
cd server && go run ./cmd/backfill-docs-ordering 2>&1 | tail -5
```

Verify: `psql -c "SELECT count(*) FROM docs_documents WHERE sort_key = '~';"` returns 0.

- [ ] **Step 5: Commit**

```bash
git add server/cmd/backfill-docs-ordering/ \
        server/internal/dbmigrate/sql/202604210002_docs_ordering_backfill_marker.sql
git commit -m "feat(docs-ordering): backfill command assigns sort_keys from current position order"
```

---

## Task 10: Three-Surface Parity Test

**Files:**
- Create: `server/internal/service/docs_ordering_parity_test.go` (or extend existing `docs_ordering_test.go`)

- [ ] **Step 1: Write the parity test**

Create the test per spec §10: fixture with 3 top-level collections, 5 uncategorized docs, 2 sub-collections, 8 docs distributed. Set `sort_key` values DIRECTLY on rows (bypass creation path). Run all three surface queries. Assert identical DFS walk + expected fixture sequence.

- [ ] **Step 2: Run and verify**

```bash
cd server && go test ./internal/service/ -run "Parity" -v -count=1
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/service/docs_ordering_parity_test.go
git commit -m "test(docs-ordering): three-surface parity test with fixture-driven sort_keys"
```

---

## Task 11: Frontend — Fractional Index + Tree Merge

**Files:**
- Create: `frontend/src/lib/fractionalIndex.ts`
- Create: `frontend/src/lib/__tests__/fractionalIndex.test.ts`
- Create: `frontend/src/lib/docsTreeMerge.ts`

- [ ] **Step 1: Port the algorithm to TypeScript**

Create `frontend/src/lib/fractionalIndex.ts` — direct port of the Go implementation (or port from the dgreensp TS reference). Must produce byte-identical outputs.

```typescript
export function between(lower: string, upper: string): string {
  // ... port
}
```

- [ ] **Step 2: Write TS test loading shared vectors**

Create `frontend/src/lib/__tests__/fractionalIndex.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { between } from '../fractionalIndex'
// NOTE: importing JSON from outside the frontend workspace root may require
// a Vite resolve alias in vite.config.ts (e.g., '@test-vectors': '../server/...').
// If that's too fragile, copy vectors.json into frontend/src/lib/__tests__/
// and add a "sync-vectors" script in package.json that copies from the Go source.
import vectors from '../../../../server/internal/ordering/testdata/vectors.json'

describe('fractionalIndex', () => {
  vectors.forEach((v: any) => {
    it(`between(${JSON.stringify(v.lower)}, ${JSON.stringify(v.upper)})`, () => {
      if (v.error) {
        expect(() => between(v.lower, v.upper)).toThrow()
      } else {
        expect(between(v.lower, v.upper)).toBe(v.expected)
      }
    })
  })
})
```

- [ ] **Step 3: Run TS tests**

```bash
cd frontend && npx vitest run src/lib/__tests__/fractionalIndex.test.ts
```

- [ ] **Step 4: Create `docsTreeMerge.ts`**

```typescript
interface TreeItem {
  id: string
  sort_key: string
  type: 'doc' | 'collection'
}

export function mergeBucketItems(
  collections: TreeItem[],
  docs: TreeItem[],
): TreeItem[] {
  // Two-pointer merge on (sort_key, id).
  const result: TreeItem[] = []
  let ci = 0, di = 0
  while (ci < collections.length && di < docs.length) {
    const cmp = collections[ci].sort_key.localeCompare(docs[di].sort_key)
      || collections[ci].id.localeCompare(docs[di].id)
    if (cmp <= 0) {
      result.push(collections[ci++])
    } else {
      result.push(docs[di++])
    }
  }
  while (ci < collections.length) result.push(collections[ci++])
  while (di < docs.length) result.push(docs[di++])
  return result
}
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/fractionalIndex.ts \
        frontend/src/lib/__tests__/fractionalIndex.test.ts \
        frontend/src/lib/docsTreeMerge.ts
git commit -m "feat(docs-ordering): frontend fractional index + tree merge helpers"
```

---

## Task 12: Frontend — Drag Handler + API Integration

**Files:**
- Modify: `frontend/src/lib/services/docsService.ts`
- Modify: `frontend/src/hooks/queries/useDocs.ts`
- Modify: `frontend/src/components/docs/DocsArrangeTree.tsx`

- [ ] **Step 1: Add `moveItem` API call to service**

In `docsService.ts`, add:

```typescript
moveItem: (wsId: string, body: MoveDocsItemRequest) =>
  api.put<void>(`/workspaces/${wsId}/docs/items/move`, body),
```

- [ ] **Step 2: Add `useMoveDocsItem` mutation hook**

In `useDocs.ts`:

```typescript
export function useMoveDocsItem(wsId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: MoveDocsItemRequest) =>
      unwrap(docsService.moveItem(wsId, body)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.all(wsId) })
    },
  })
}
```

- [ ] **Step 3: Update drag handler to use move endpoint**

In `DocsArrangeTree.tsx` (or equivalent), update the drag-end handler to:
1. Compute optimistic sort_key via `between(beforeKey, afterKey)`.
2. Apply optimistic update to cached data.
3. Call `moveItem` with `{item, target_bucket, position: {before, after}}`.
4. On error: revert optimistic update + toast.

This step requires reading the current drag handler code to understand its shape — the agent must adapt to the existing pattern rather than replacing it wholesale.

- [ ] **Step 4: Update tree rendering to use sort_key merge**

Wherever the docs tree is rendered (sidebar, arrange view), replace any `sort-by-position` logic with `mergeBucketItems` from `docsTreeMerge.ts`.

- [ ] **Step 5: Run frontend build**

```bash
cd frontend && npx tsc -b && pnpm build
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/services/docsService.ts \
        frontend/src/hooks/queries/useDocs.ts \
        frontend/src/components/docs/DocsArrangeTree.tsx
git commit -m "feat(docs-ordering): frontend drag handler uses sort_key move endpoint"
```

---

## Task 13: Post-Backfill CHECK Constraint

**Files:**
- Create: `server/internal/dbmigrate/sql/202604210003_docs_sort_key_check_constraint.sql`

This migration is applied ONLY after the backfill is verified (spec §12 step 10). It's committed now but not run until the flag has been true in production for 1 week.

- [ ] **Step 1: Create migration**

```sql
-- Post-backfill safety net: fail fast if any new row ships with
-- the sentinel '~' or invalid characters. Only apply after backfill
-- has run and been verified (all rows match ^[a-z]+$).
ALTER TABLE docs_documents
  ADD CONSTRAINT docs_documents_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');

ALTER TABLE docs_collections
  ADD CONSTRAINT docs_collections_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');
```

- [ ] **Step 2: Commit**

```bash
git add server/internal/dbmigrate/sql/202604210003_docs_sort_key_check_constraint.sql
git commit -m "schema: add CHECK constraint for sort_key (apply post-backfill only)"
```

---

## Execution Order

Tasks 1–3 are independent and can run in parallel. Tasks 4–8 depend on 1–3. Task 9 depends on 1+2. Task 10 depends on 4. Tasks 11–12 depend on 1. Task 13 is deferred to post-backfill.

```
Task 1 (algorithm) ─┐
Task 2 (schema)    ─┼─→ Task 4 (repo ORDER BY) → Task 5 (creation) → Task 6 (reorder shims) → Task 7 (move endpoint) → Task 8 (reparent)
Task 3 (flag)      ─┘              │
                                   └→ Task 10 (parity test)
Task 1 ─────────────────────────────→ Task 9 (backfill)
Task 1 ─────────────────────────────→ Task 11 (frontend helpers) → Task 12 (drag integration)
                                                                    Task 13 (CHECK constraint — deferred)
```

## Post-Implementation Checklist

Before merging:

- [ ] `cd server && go test ./... -count=1` — all green
- [ ] `cd server && go vet ./...` — clean
- [ ] `cd frontend && npx tsc -b` — clean
- [ ] `cd frontend && npx vitest run` — all green
- [ ] Parity test (Task 10) passes
- [ ] Shared test vectors pass on both Go and TS sides

Staging deploy:

- [ ] Deploy with `DOCS_ORDERING_USE_SORT_KEY=false`
- [ ] Run `go run ./cmd/backfill-docs-ordering`
- [ ] Verify: `SELECT count(*) FROM docs_documents WHERE sort_key = '~';` returns 0
- [ ] Verify: `SELECT count(*) FROM docs_collections WHERE sort_key = '~';` returns 0
- [ ] Flip flag to `true`
- [ ] Spot-check UI: internal docs list, arrange mode, public help center
- [ ] 24-hour soak
- [ ] **STOP if parity test or UI spot-check fails — investigate before production**

Production deploy:

- [ ] Repeat staging steps for production
- [ ] 1-week soak
- [ ] Apply Task 13 CHECK constraint migration
