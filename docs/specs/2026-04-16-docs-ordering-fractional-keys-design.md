# Docs Ordering — Fractional Sort Keys Design

> Source review, 2026-09-17

Historical ordering design. Current collections/documents have `sort_key` fields,
and repositories retain configuration-dependent position/sort-key behavior. See
[collection ordering](../../server/internal/repository/docs_collection.go),
[configuration](../../server/internal/config/config.go), and
[ordering parity tests](../../server/internal/service/docs_ordering_parity_test.go).
Check `DOCS_ORDERING_USE_SORT_KEY` for the target deployment; neither this plan nor
the presence of the code proves that the flag is enabled there.

**Status:** Draft v3 (post-review revisions)
**Owner:** azhar-teampulse
**Last updated:** 2026-04-16

## Changelog

- **v3:** second review found a real correctness bug in the hand-rolled pseudocode (upper-exhausted conflated with upper-empty → `Between("", "a")` returned `"am"` > `"a"`). Rather than patch the pseudocode in the spec (source-of-drift risk), §6 now references the canonical dgreensp fractional-indexing algorithm by name, with test-vector-driven compliance as the implementation gate. The hand-rolled pseudocode has been removed in favor of a contract + reference-implementation pointer. Also fixed 6 polish items: shim-rebuilds-from-scratch clarification (§8.1), parity-test fixture sets sort_keys directly (§10), "stop-if-staging-parity-fails" instruction (§12), pre-prod-flip mixed-reorder checklist (§12).
- **v2:** incorporated 15 review findings from v1: rewrote algorithm pseudocode, replaced broken backfill formula, committed to Go one-shot command, enumerated all four reorder endpoints, gated writes behind flag, shared Go↔TS test vectors, authz rules, per-row idempotency, no-panic, sentinel default, NULL handling, stronger parity assertion, Nextra sequential.

## 1. Goal

Every location where docs and collections render — internal "All docs" list, arrange mode, and the public help center — must show the **identical** order, and that order must be **fully user-controlled**: the user can drag any item (collection OR doc) anywhere within its parent bucket, including interleaved with items of the other type.

## 2. Non-goals

- Real-time collaborative drag-and-drop with presence
- Undo/redo for reorder operations
- Bulk move / multi-select move operations
- Changing the soft-delete or reparent rules for collections
- Migrating away from two tables (`docs_documents` and `docs_collections` stay separate)
- Dropping the `position` column in this change (later cleanup)

## 3. Current state (why this is broken)

### 3.1 Data model

`docs_documents.position` and `docs_collections.position` each store an integer per-row, scoped to the row's parent. Two separate position spaces:

- Doc's bucket: `(space_id, collection_id)`, position unique within that tuple.
- Collection's bucket: `(space_id, parent_collection_id)`, position unique within that tuple.

This means a collection at `position=0` in bucket `B` and a doc at `position=0` in the same bucket `B` don't collide — but they also can't be interleaved by position.

Consequence: renderers must pick a rule like "collections first, then docs", and different renderers have picked slightly different rules.

### 3.2 Query divergence

Verified by audit of `server/internal/repository/docs_*.go`:

| Surface | File:line | ORDER BY |
|---------|-----------|----------|
| Internal space-scoped docs | `docs_document.go:96` | `collection_id ASC NULLS FIRST, position ASC, created_at ASC` |
| Internal `ListBySpace` collections | `docs_collection.go:104` | `position ASC, created_at ASC` |
| Internal nested collection trees | `docs_collection.go:165/208/333/652` | `position ASC, created_at ASC, id ASC` |
| Help center space nav collections | `docs_helpcenter.go:761` | `depth ASC, parent_collection_id ASC, position ASC, created_at ASC` |
| Help center space nav articles | `docs_helpcenter.go:785` | `collection_id ASC (default NULLS LAST), position ASC, created_at ASC` |
| Help center collection articles | `docs_helpcenter.go:468` | `d.position ASC, d.created_at ASC` |
| Help center localized collections | `docs_helpcenter.go:341` | `c.position ASC, c.created_at ASC` |

Concrete mismatches: `NULLS FIRST` vs default `NULLS LAST`; inconsistent `id ASC` tiebreaker; inconsistent hierarchical grouping.

### 3.3 Data integrity (verified against production data)

- Docs: no duplicate `position` within the same `(space_id, collection_id)` bucket.
- Collections: many duplicates *across* buckets (because `position` is per-parent); within a parent, positions are unique.
- Timestamps (`created_at`) are unique across existing rows but not relied upon as the canonical tiebreaker.

## 4. Proposed model

### 4.1 Ordering primitive: fractional sort keys

A **sort key** is a short `TEXT` string from a lexicographic alphabet. Two properties:

1. Lexicographic comparison of sort keys equals the intended display order.
2. For any two keys `a < b`, or for the "no neighbor on one side" case, there is always a key `k` such that `a < k < b` (or, with empty bounds, just a valid key on the unbounded side).

Same primitive used by Figma, Notion, Linear, et al. No external dependency; we vendor a small Go implementation.

### 4.2 Bucket definition

A **bucket** is a tuple `(space_id, parent_id)` where `parent_id` is a collection id or `NULL` (space root).

- Doc's bucket: `(space_id, collection_id)`.
- Collection's bucket: `(space_id, parent_collection_id)`.

Collections and docs **share the same sort-key space per bucket**. Ordering is strict:

> `ORDER BY sort_key ASC, id ASC` (with `id ASC` as defensive tiebreaker only)

### 4.3 Rendering

Every surface that renders a bucket's contents:

1. Fetches collections and docs in that bucket (NULL parent_id and non-NULL branches explicit in the query — see §7.3).
2. Merges the two lists by `sort_key ASC, id ASC`.
3. Recurses into each collection.

No type-based rendering rule. User controls all ordering.

## 5. Data model changes

### 5.1 Schema

```sql
-- Step 1: add columns via AutoMigrate (GORM struct fields below).
-- Default value: "~" (tilde) — lexicographically greater than any valid key ([a-z]+),
-- so any un-backfilled row sorts at the END of its bucket, making the "forgot the flag"
-- bug visible (all unmanaged items pile at the bottom) instead of invisible.
ALTER TABLE docs_documents   ADD COLUMN sort_key TEXT NOT NULL DEFAULT '~';
ALTER TABLE docs_collections ADD COLUMN sort_key TEXT NOT NULL DEFAULT '~';

-- Step 2: bucket-scoped read index (via dbmigrate SQL file).
-- No WHERE clause — indexing all rows avoids bloat-from-soft-delete-restore churn
-- (§9 of review). Small cost at current volumes.
CREATE INDEX IF NOT EXISTS idx_docs_documents_bucket_sort
  ON docs_documents (workspace_id, space_id, collection_id, sort_key, id);

CREATE INDEX IF NOT EXISTS idx_docs_collections_bucket_sort
  ON docs_collections (workspace_id, space_id, parent_collection_id, sort_key, id);

-- Step 3: post-backfill, add a CHECK constraint to fail fast if any new row ships
-- with a sentinel key. This migration lands AFTER Phase 1 backfill is verified.
ALTER TABLE docs_documents
  ADD CONSTRAINT docs_documents_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');

ALTER TABLE docs_collections
  ADD CONSTRAINT docs_collections_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');
```

`position` stays until a later cleanup migration. Dropping it is out of scope for this change.

### 5.2 Model struct changes

```go
// internal/model/docs_document.go
type DocsDocument struct {
    // ... existing fields ...
    Position int    `json:"position" gorm:"not null;default:0"`
    SortKey  string `json:"sort_key" gorm:"not null;default:'~'"`
}

// internal/model/docs_collection.go  (same pattern)
```

## 6. Fractional indexing algorithm

### 6.1 Alphabet

Lowercase ASCII `[a-z]`, 26 characters. Integer index for char `c`: `int(c) - int('a')` → `0..25`.

Reasoning: readable in DB browser, simple invariant (`^[a-z]+$` enforced by CHECK constraint), sufficient depth (26^N grows fast). Base62 would give shorter keys but needs escaping.

### 6.2 Package

`server/internal/ordering/fractional.go` exposes:

```go
// Between returns a sort key k such that lower < k < upper, OR a key strictly
// above `lower` when upper == "", OR a key strictly below `upper` when
// lower == "", OR the canonical midpoint "m" when both are empty.
//
// Returns an error (never panics) if:
//   - lower != "" and upper != "" and lower >= upper  (invalid neighbors)
//   - lower or upper contains chars outside [a-z]
//
// Handlers recover from errors by returning a 409; see §8.3.
func Between(lower, upper string) (string, error)
```

### 6.3 Algorithm — contract and reference

We vendor the **canonical fractional-indexing algorithm by David Greenspan** (used by Figma, Notion, Linear) rather than hand-rolling our own. The algorithm is battle-tested against exactly the edge cases that trip up naive implementations (upper-exhausted vs upper-empty; descending from trailing-zero upper; etc.).

**Reference implementation:** `https://github.com/rocicorp/fractional-indexing` (TypeScript original by dgreensp; MIT-licensed, widely ported). Our Go port in `server/internal/ordering/fractional.go` is a direct translation of that reference, restricted to the 26-letter alphabet `[a-z]`.

**Contract (what callers can rely on):**

1. `Between("", "")` returns a canonical midpoint key (specifically `"a"` by the dgreensp convention, sorting at the start of an empty bucket). Subsequent appends produce `"b"`, `"c"`, etc., then `"cn"`, etc. as needed. Exact values are locked by the test vectors.
2. `Between(lower, upper)` returns `k` such that `lower < k < upper` lexicographically, where empty strings on either side denote "no bound on this side."
3. `Between` returns `(string, error)`. Errors — never panics — in these cases:
   - `lower != "" && upper != "" && lower >= upper`
   - Either argument contains a character outside `[a-z]`
   - The neighbors are too tight to produce a strictly-between key (e.g., `Between("", "a")` — the only key that could fit is the empty string, which violates contract #1 above). The caller handles this by requesting a rebuild/compaction of the bucket, or by falling back to append semantics.
4. Generated keys never end in the lowest alphabet char (`'a'`), so subsequent descent has room to prepend.

**What the spec DOES NOT specify:** the exact bytes `Between("m", "p")` returns. That's defined by the reference implementation and locked by test vectors (§6.5). Implementers port the algorithm to Go (and TS); the shared `testdata/vectors.json` is the correctness contract.

### 6.3.1 Representative test-vector entries

The full set of triples lives in `server/internal/ordering/testdata/vectors.json` and covers:

- **Empty bounds:** `Between("", "")`, `Between("", "c")`, `Between("x", "")`, `Between("y", "")`, `Between("z", "")`
- **Both bounds:** `Between("a", "b")`, `Between("m", "n")`, `Between("mh", "n")`, `Between("a", "am")`, `Between("aa", "ab")`
- **Tight-upper (potentially impossible):** `Between("", "a")`, `Between("", "aa")`, `Between("a", "aa")`, `Between("aaa", "aab")` — verify algorithm returns an error OR a valid strictly-between key per contract #3
- **Reverse (must error):** `Between("n", "m")`, `Between("b", "a")`
- **Invalid chars (must error):** `Between("A", "z")`, `Between("!", "m")`, `Between("m1", "m2")`
- **Long keys:** at least 20 entries with keys 5+ chars long, exercising descent paths

Implementers do NOT hand-trace algorithm outputs in this spec. Expected values come from running the reference implementation (dgreensp's TS) once during vector-file generation; both the Go and TS ports assert on those expected values.

### 6.4 Properties

- **Average key length** under random inserts: `O(log N)`.
- **Adversarial case** (always insert at the same spot): linear key growth. Not relevant to human reorder patterns at Helpin's scale. If a bucket ever sees keys > 12 chars, compaction is warranted (§16 future work).
- **Collisions under concurrency:** two writes choosing identical keys with identical `before`/`after` neighbors produce the same output. Safeguard: read-modify-write inside a transaction (§8.3).
- **No panics:** `Between` returns `(string, error)`. Handlers translate errors to 409 (client retries).

### 6.5 Tests

`server/internal/ordering/fractional_test.go`:

1. Table-driven with the worked examples in §6.3.1 (final outputs).
2. **Shared test vectors** at `server/internal/ordering/testdata/vectors.json`: ≥200 triples `{lower, upper, expected}`. Go tests load and assert; TS tests in `frontend/src/lib/__tests__/fractionalIndex.test.ts` load the same file and assert. **Merge blocker** on both sides.
3. Stress: 10,000 random inserts with random positions. Assert all keys strictly increasing; no duplicates; max key length ≤ ~20.
4. Error paths: invalid chars; `lower == upper`; `lower > upper`. All must return errors, no panic.

## 7. Read contract

### 7.1 Canonical ORDER BY

Every query that lists docs or collections by bucket:

```sql
ORDER BY sort_key ASC, id ASC
```

Scoped via explicit WHERE clauses (§7.3).

No `NULLS FIRST` / `NULLS LAST`, no `created_at` tiebreaker. `sort_key` is `NOT NULL` with CHECK (post-Phase 1).

### 7.2 Affected repository methods

All switched behind the feature flag (§13):

- `docs_document.go` — `List`, `ListInWorkspace`, any other bucket reader.
- `docs_collection.go` — `ListBySpace`, `ListByWorkspace`, `:165/208/333/652` (nested trees).
- `docs_helpcenter.go` — `ListSpaceNavigation`, `ListPublicCollectionArticles`, localized nav, `ListSpaceNavigation (localized)`, and every other place that returns a tree.

Search / filter endpoints that use full-text rank ordering stay unchanged.

### 7.3 Bucket queries — explicit NULL handling

A bucket read helper lives in the collection and document repositories:

```go
// listCollectionsInBucket returns all collections with
// parent_collection_id = parentID (or parent_collection_id IS NULL when parentID == nil)
// in the given space, ordered by the canonical ORDER BY.
func (r *DocsCollectionRepository) listCollectionsInBucket(
    ctx context.Context, spaceID string, parentID *string,
) ([]model.DocsCollection, error)

// listDocsInBucket — same shape, for docs (collection_id = parentID / NULL).
func (r *DocsDocumentRepository) listDocsInBucket(
    ctx context.Context, spaceID string, parentID *string,
) ([]model.DocsDocument, error)
```

Both build the WHERE clause with an explicit IS NULL branch when `parentID` is nil. No reliance on `NULLS FIRST / LAST`.

### 7.4 Tree construction

Consumers (backend serializers and frontend renderers) walk the tree DFS using the two bucket helpers:

```
walk(spaceID, parentID):
    colls := listCollectionsInBucket(spaceID, parentID)
    docs  := listDocsInBucket(spaceID, parentID)
    merged := mergeByKey(colls, docs)       // two-pointer merge on (sort_key, id)
    for item in merged:
        render(item)
        if item is collection:
            walk(spaceID, &item.ID)
```

The merge is explicit and shared across backend and frontend. No ambiguity about "collections first."

## 8. Write contract

### 8.1 Existing endpoints and their fate

Four reorder endpoints exist in the codebase today. Each has a defined path forward:

| Endpoint | Handler | Today's behavior | Post-change behavior |
|----------|---------|------------------|----------------------|
| `POST /docs/spaces/reorder`           | `ReorderSpaces`      | Reorder spaces in a workspace        | **Unchanged.** Out of scope (spaces are top-level; their ordering is workspace-global, not bucket-scoped). |
| `POST /docs/spaces/:sid/collections/reorder` | `ReorderCollections` | Reorder collections in one bucket by ordered IDs | **Shim.** Compute sort_keys via sequential `Between` from the submitted list; persist to both `position` and `sort_key` (see §13). |
| `POST /docs/spaces/:sid/documents/reorder`   | `ReorderDocuments`   | Reorder docs in one bucket by ordered IDs        | **Shim.** Same pattern as above. |
| `POST /docs/spaces/:sid/children/reorder`    | `ReorderChildren`    | Reorder mixed list of collections + docs in one bucket | **Shim.** Same — this is the endpoint arrange-mode calls today; it must keep working across the rollout. |

**Shim semantics** (for the three bucket reorders): the server accepts the client's ordered list and **rebuilds sort_keys for every item in the submitted list from scratch** — ignoring any pre-existing `sort_key` on those items. It computes sequentially via `Between(prevKey, "")` starting from the empty prefix: first item gets `Between("", "")`, second gets `Between(firstKey, "")`, etc. This guarantees idempotency (calling with the same list twice produces identical keys) and eliminates drift risk from partial re-reorders. Items in the bucket NOT included in the submitted list retain their existing sort_keys.

### 8.2 New primary endpoint

```
POST /api/workspaces/:wsID/docs/items/move

{
  "item":          { "type": "doc" | "collection", "id": "uuid" },
  "target_bucket": { "space_id": "uuid", "parent_collection_id": "uuid" | null },
  "position": {
    "before": { "type": "doc"|"collection", "id": "uuid" } | null,
    "after":  { "type": "doc"|"collection", "id": "uuid" } | null
  }
}
```

This is the endpoint the new drag-and-drop UX will call (one move per drag). It replaces the "send the whole list" pattern with a surgical "insert between these two neighbors" pattern.

Server logic (inside a single DB transaction):

1. Authz (§8.3).
2. Cross-space move: forbid. If `item.space_id != target_bucket.space_id`, return 400. (Cross-space moves are §2 non-goal.)
3. `SELECT … FOR UPDATE` on the `before` and `after` rows (if provided). Extract their `sort_key`s.
4. Sanity: if `beforeKey >= afterKey`, reject with 409 (neighbors are stale; client retries).
5. `newKey, err := ordering.Between(beforeKey, afterKey)`. On error: 409.
6. Update the item's `sort_key`; if the bucket differs from its current, also update `collection_id` / `parent_collection_id`.
7. Commit; return the updated item.

If `before` and `after` are both null → append to the end (server queries the bucket for its current last key, then `Between(lastKey, "")`). If only one side is null → insert at that extremum.

### 8.3 Authorization

- `PermDocsEdit` on the item being moved (checked against the item's current workspace/space).
- `PermDocsEdit` on the target bucket's space (so a user can't move a doc into a space they can't write to).
- The current space and target space must be the same — cross-space moves rejected with 400 per §8.2 step 2.
- For `ReorderChildren` and the other shims: same authz rules apply, scoped to the space they operate on.

### 8.4 New item creation

Creation paths (`CreateDocument`, `CreateCollection`) assign `sort_key` at the end of the target bucket:

1. Find current max `sort_key` in bucket (across both tables).
2. `newKey, _ := ordering.Between(maxKey, "")`.
3. Insert with that key.

### 8.5 Soft-delete reparent

Existing code renumbers `position` for reparented children. New code additionally computes fresh `sort_key`s for each reparented item by appending to the destination bucket sequentially (call `Between(lastKey, "")` per item; update `lastKey` after each). Preserves relative order.

### 8.6 Batch creation (Nextra importer, bulk APIs)

Importer assigns sort keys **sequentially** within its creation transaction, never in parallel across goroutines. The Nextra importer walks the source tree in natural order and emits items one at a time, each getting `Between(lastKeyInBucket, "")`.

## 9. Backfill migration

### 9.1 Scope and correctness requirement

Every existing row gets a `sort_key` that preserves its **current visual order**. Users see no reordering after the migration. Future drags use the new model.

### 9.2 Ordering rule

For each bucket (DFS from the space roots of each space):

1. Gather all collections + all docs in the bucket.
2. Sort **collections first** (by current `position, created_at, id`), then **docs** (by current `position, created_at, id`). This is today's visual order.
3. Assign sort keys by calling `ordering.Between(prevKey, "")` sequentially — first item gets `Between("", "")` = `"m"`; second gets `Between("m", "")` = `"s"`; etc. This is the SAME algorithm used by the production write path, so there is ONE code path for ordering.
4. Recurse into each collection.

This replaces the broken "evenly spaced via formula" approach from v1.

### 9.3 Mechanism

**`dbmigrate` does NOT support Go migrations** (verified: `migrator.go` embeds `sql/*.sql` only and filters by extension). So the backfill is implemented as a one-shot Go command plus a marker SQL migration:

1. `server/cmd/backfill-docs-ordering/main.go` — iterates every workspace, every space, walks the tree, assigns sort keys per §9.2. Operates under a single DB connection; transactional per workspace.
2. `server/internal/dbmigrate/sql/YYYYMMDDNNNN_docs_ordering_backfill_marker.sql` — inserts a no-op marker row. Its presence tells future deploys "backfill already ran" so the command can refuse to re-run. Alternatively, the command checks for any row with `sort_key = '~'` and refuses to run if zero (safely idempotent).

The command is:

- **Idempotent per-row**: iterates all rows, skips rows where `sort_key != '~'`. A row inserted during the backfill (via the normal creation path, which already assigns a real key) is simply skipped.
- **Resumable**: if interrupted, re-running picks up where it left off because skipped rows are the completed ones.
- **Dry-run mode** (`--dry-run`): computes and prints the proposed keys without writing. Used in staging verification.
- **Single-workspace mode** (`--workspace-id=…`): for spot-testing.

### 9.4 Verification

After the command completes:

```sql
SELECT count(*) FROM docs_documents   WHERE sort_key = '~';
SELECT count(*) FROM docs_collections WHERE sort_key = '~';
```

Both must return 0. If either is non-zero, the command failed or a new row was inserted after the final pass; re-run the command.

Then run §10 parity test against a staging workspace to confirm the backfilled order matches the pre-migration render.

Only after these pass does the engineer apply the post-backfill CHECK constraint migration (§5.1 Step 3).

## 10. Three-surface parity test

`server/internal/service/docs_ordering_parity_test.go`:

1. Fixture: a space with
   - 3 top-level collections with deliberately out-of-alpha sort keys (e.g., `"m"`, `"g"`, `"s"` — not in creation order)
   - 5 uncategorized docs with interleaved sort keys (some before collections alphabetically, some after)
   - 2 sub-collections under the first top-level collection, also interleaved with that collection's docs
   - 8 docs distributed across the collections

   **The fixture sets `sort_key` values DIRECTLY on each row** (via the test helper, bypassing the normal creation path) so the test exercises arbitrary user-chosen orderings rather than whatever the backfill or creation path would assign. This is what makes the test meaningful: it verifies the read path respects the stored keys regardless of how they got there.
2. Runs the three surfaces' navigation queries:
   - Internal bucket walk (doc+collection repos via `listDocsInBucket` / `listCollectionsInBucket`).
   - Help center `ListSpaceNavigation`.
   - Help center `ListPublicCollectionArticles` (per collection).
3. Computes the DFS walk from each surface as `[]struct{ ID, Type, Depth }`.
4. Asserts:
   - All three DFS walks are **identical** to each other.
   - The walk matches an **expected fixture sequence** computed from the sort keys directly. This is the crucial addition: without it, the surfaces could agree on a buggy order that doesn't match user intent.

Test must pass before merge.

## 11. Frontend changes

### 11.1 New files

- `frontend/src/lib/fractionalIndex.ts` — mirror of the Go algorithm for client-side key computation during optimistic drag updates.
- `frontend/src/lib/docsTreeMerge.ts` — helper that merges collection and doc arrays within a bucket by `sort_key ASC, id ASC`.

**Go↔TS parity is enforced by the shared test-vectors file** at `server/internal/ordering/testdata/vectors.json`. The TS test (`frontend/src/lib/__tests__/fractionalIndex.test.ts`) loads the same file via a relative path import and asserts on every triple. If either implementation drifts, CI blocks merge.

### 11.2 Modified files

- Every place the docs tree renders (sidebar, arrange mode, help center routes) — use `docsTreeMerge` instead of any local `sort-by-position` logic.
- Drag handlers — emit `{ item, target_bucket, before, after }` and call `/docs/items/move`. Legacy callers (`ReorderChildren` etc.) keep their existing shape.
- TanStack Query invalidations on reorder mutations: `queryKeys.docs.spaceTree(spaceID)`, every `queryKeys.docs.collection(collectionID)` on the ancestor path, and help-center preview queries. Any new `sort_key`-dependent view gets added to the invalidation set.

### 11.3 Optimistic update + rollback

- On drag drop, the frontend computes a provisional sort_key via its local `fractionalIndex.ts` and applies it to the cached data (optimistic).
- Sends the move request.
- On success: refetch (or apply the server's canonical response).
- On failure: revert the cache and show a toast "Couldn't move item — try again."

## 12. Rollout

The deploy is **sequential** with explicit flag flips. No action combines "deploy new code" and "flip reads to sort_key" — those are separate steps the engineer must perform in order.

1. **Deploy with feature flag `DOCS_ORDERING_USE_SORT_KEY=false`.** New code is present but dormant for reads. Writes still populate only `position`. Schema columns added. Indexes created. No functional change for users.
2. **Run the backfill command in staging** (`--dry-run` first, then live).
3. **Verify §9.4.**
4. **Run §10 parity test in staging.** **STOP if parity fails:** the backfill or algorithm has a bug; do NOT proceed to production. Investigate, fix, re-run backfill, re-verify.
5. **Deploy flag flip: set `DOCS_ORDERING_USE_SORT_KEY=true` in staging.** Reads now use `sort_key`. Writes now populate both `sort_key` and `position` per §13. Spot-check the UI.
6. **24-hour staging soak.** Watch logs for 409s on move and any parity test failures in nightly CI.
7. **Pre-prod-flip checklist.** Before flipping production to `true`:
   - Parity test green in staging nightly CI for the full soak window.
   - No 409s from the move endpoint outside of known client retry cases.
   - **Acknowledge rollback semantics:** if the flag is flipped back to `false` after this point, any mixed-type reorder (doc dragged above a collection) done under `true` will be lost. Review the staging soak to see whether such reorders occurred; if yes, accept that rollback requires a data rollback, not a flag flip.
8. **Production: deploy code with flag `false`.** Run backfill, verify §9.4, run parity test. **STOP if parity fails.**
9. **Production: flip flag `true`.** Soak.
10. **Add CHECK constraint migration** (§5.1 Step 3) once the flag has been true in production for 1 week without incident.
11. **(Later follow-up PR)** remove the flag and stop writing `position`; drop the column.

Any step before §12.5 (staging flag flip) can revert cleanly by reverting the PR. Steps §12.5–9 can revert by flipping the flag off — subject to the §12.7 mixed-reorder caveat.

## 13. Feature flag semantics

`DOCS_ORDERING_USE_SORT_KEY` gates BOTH reads AND writes:

- **`false` (pre-flip):** reads use `position`-based ORDER BY; writes touch only `position`.
- **`true` (post-flip):** reads use `sort_key`-based ORDER BY; writes touch both `sort_key` AND `position`.

Writing both during `true` ensures we can flip back to `false` (read from `position`) and see a consistent order, as long as reorders under `true` are type-homogeneous (same-table only). **Mixed-type reorders — the new UX — have no `position` representation.** If a user drags a doc above a collection during `true` and we later flip back to `false`, that drag's intent is lost (the doc reverts to its old position-based location).

**Trade-off, explicitly documented:** rollback via flag flip is only safe for same-type reorders during the flag-true window. Mixed-type reorders are committed; rollback would require a data rollback (restore from backup). Given the low production write volume and the staging soak before production flip, this is acceptable.

## 14. Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Algorithm bug produces non-strictly-ordered keys | Shared Go↔TS test vectors (§6.5); stress test (10k random inserts) |
| Backfill assigns wrong order | Idempotent re-run; dry-run mode; staging DFS-walk diff against `position`-based walk |
| Concurrent inserts collide on identical keys | Move handler uses `SELECT … FOR UPDATE`; `Between` returns error (no panic) on stale neighbors → handler returns 409 → client retries |
| `id ASC` tiebreaker hides a latent algorithm bug | Parity test asserts against an *expected fixture sequence* derived from keys, not just cross-surface agreement |
| Long-lived keys degrade (adversarial insertion pattern) | Monitor max key length; compaction shipped later if needed |
| Frontend forgets to invalidate a query | Regression test: reorder an item, assert immediate visibility across sidebar, arrange mode, help center preview |
| Help center uses stale snapshot | Verified: help center reads live from `docs_documents` / `docs_collections`. No publication snapshot. (Re-verify before Phase 1.) |
| Nextra importer produces colliding keys | Spec §8.6: importer assigns sequentially inside its transaction, never parallel |
| `sort_key = '~'` sentinel hides un-backfilled rows | Sentinel sorts LAST (not first) — visible pile-up at end of every bucket; easy to spot in staging. Post-backfill CHECK constraint prevents future regressions |
| Feature-flag off after mixed-type reorders | §13 documents the trade-off; staging soak surfaces any issue before prod |
| Index bloat from soft-delete-restore churn | Indexes don't filter on `deleted_at`; slight extra size accepted |

## 15. Implementation checklist

- [ ] Add `SortKey` field to `DocsDocument` and `DocsCollection` structs (AutoMigrate picks up).
- [ ] `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_docs_sort_key_indexes.sql` — column DEFAULT `'~'` (AutoMigrate) plus the two indexes.
- [ ] `server/internal/ordering/fractional.go` + tests per §6.5. Include the shared `testdata/vectors.json`.
- [ ] `server/cmd/backfill-docs-ordering/main.go` per §9.3. Dry-run and single-workspace modes.
- [ ] Marker migration SQL noting backfill location.
- [ ] Update every ORDER BY in `repository/docs_*.go` per §7.2, gated by `config.UseDocsSortKey` bool (driven by env var).
- [ ] Add `listCollectionsInBucket` / `listDocsInBucket` helpers per §7.3.
- [ ] Add `POST /docs/items/move` handler/service/repo per §8.2. `Between` errors → 409.
- [ ] Authz checks per §8.3 on move and on the four legacy reorder endpoints.
- [ ] Update `ReorderSpaces` (unchanged), `ReorderCollections`, `ReorderDocuments`, `ReorderChildren` to compute sort_keys sequentially from submitted lists (per §8.1).
- [ ] Update creation paths (`CreateDocument`, `CreateCollection`) to set `sort_key` per §8.4.
- [ ] Update soft-delete reparent to set `sort_key` per §8.5.
- [ ] Update Nextra importer per §8.6.
- [ ] Frontend: `fractionalIndex.ts`, `docsTreeMerge.ts`, drag handlers, query invalidation.
- [ ] Shared JSON test vectors imported by both Go and TS tests.
- [ ] Three-surface parity integration test per §10 (with expected fixture sequence).
- [ ] Reorder regression tests: move-within-bucket, move-between-buckets, move-to-start, move-to-end, concurrent-move 409 path.
- [ ] Pre-merge green CI: `go test ./...`, `npx tsc -b`, parity test, vector tests on both sides.
- [ ] Staging deploy + backfill + flag flip + soak.
- [ ] Production deploy + backfill + flag flip + soak.
- [ ] Post-soak: apply CHECK constraint migration.

## 16. Out of scope (future work)

- Real-time reorder conflict resolution (CRDT)
- Compaction background job
- Bulk move / multi-select move
- Drop `position` column
- Remove the feature flag
- Cross-space moves
- Drag-reorder across workspaces

## 17. Review history

- **v2 → v3:** second review caught a correctness bug in the hand-rolled pseudocode: the upper-exhausted branch conflated "upper string has been consumed" with "upper is empty/unbounded," causing `Between("", "a")` to return `"am"` — which is strictly greater than `"a"` and violates the contract. Rather than patch the pseudocode in-place (source-of-drift risk; any future edit risks re-introducing similar bugs), v3 replaces §6.3 with a contract-based specification that points to the canonical dgreensp fractional-indexing algorithm as the reference implementation. Test vectors are the compliance gate. Also fixed: §8.1 shim semantics explicit ("rebuilds from scratch"), §10 parity fixture sets sort_keys directly, §12 rollout has an explicit STOP if staging parity fails and a pre-prod-flip checklist that calls out the mixed-reorder rollback trade-off.
- **v1 → v2:** addressed 15 findings from first spec review. Critical fixes: algorithm pseudocode rewritten, backfill formula replaced with sequential `Between`, committed to Go one-shot command, enumerated all four reorder endpoints, gated writes behind the flag, added shared test vectors, added authz rules, fixed rollout sequencing. Refinements: per-row idempotency, no-panic algorithm, sentinel default that sorts last, explicit NULL handling, parity-test assertion against expected keys, Nextra importer sequential assignment, index-bloat acknowledgment.
