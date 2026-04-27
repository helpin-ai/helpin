# Coverage Gaps Redesign Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-event-hash dedupe + rule-based classifier with topic-scoped clustering and LLM enrichment, behind a redesigned UI with a clean `Open → Done | Rejected` lifecycle. Surfaces fewer, more actionable gaps; produces published/updated KB articles as the explicit outcome.

**Architecture:** Support events keep flowing through `ProcessSupportEvent`; clustering is now topic-scoped via a deterministic `cluster_key`. A Temporal workflow (`CoverageGapEnrichmentWorkflow`) runs daily and on volume spikes, calling the existing `internal/llm` provider to produce action-verb titles and inline article drafts. The frontend page is rewritten around three columns and a smart-routed `Add ▾` action that creates a draft Doc.

**Tech Stack:** Go 1.24, Chi, GORM (PostgreSQL/SQLite tests), Temporal Go SDK, `internal/llm` provider, React 19, TipTap, TanStack Query, Tailwind v4.

---

## Reference Documents

- **Spec:** `docs/superpowers/specs/2026-04-27-coverage-gaps-redesign-design.md`
- **Predecessor plan:** `docs/superpowers/plans/2026-04-15-docs-coverage-loop-v1.md`
- **Predecessor spec:** `docs/superpowers/specs/2026-04-15-coverage-resolution-flows-design.md`
- **Backend conventions:** `server/CLAUDE.md`
- **Project conventions:** `CLAUDE.md`
- **Skills referenced:** @superpowers:test-driven-development, @superpowers:verification-before-completion

---

## File Structure

### New files

**Backend:**
- `server/internal/dbmigrate/sql/2026042700001_coverage_topics_cluster_columns.sql`
- `server/internal/dbmigrate/sql/2026042700002_coverage_gaps_lifecycle_columns.sql`
- `server/internal/dbmigrate/sql/2026042700003_coverage_suggestions_versioning.sql`
- `server/internal/dbmigrate/sql/2026042700004_coverage_status_migration.sql`
- `server/internal/dbmigrate/sql/2026042700005_coverage_cluster_rebuild.sql` (calls a Go-side rebuild via the migrate harness — see Task 11)
- `server/internal/service/support_coverage_clusterer.go`
- `server/internal/service/support_coverage_clusterer_test.go`
- `server/internal/service/support_coverage_enrichment.go`
- `server/internal/service/support_coverage_enrichment_test.go`
- `server/internal/temporalapp/coverage_gap_workflow.go`
- `server/internal/temporalapp/coverage_gap_workflow_test.go`
- `server/internal/handler/support_coverage_regenerate.go`
- `frontend/src/lib/supportCoverageClusterTypes.ts` (new types: ClusterTopic, ImpactTier, etc.)

**Frontend:**
- `frontend/src/components/support/coverage/GapList.tsx`
- `frontend/src/components/support/coverage/GapDetailPane.tsx`
- `frontend/src/components/support/coverage/GapAddSplitButton.tsx`
- `frontend/src/components/support/coverage/GapImpactBadge.tsx`

### Modified files

**Backend:**
- `server/internal/model/support_coverage.go` — new fields on Topic, Gap, Suggestion
- `server/internal/service/support_coverage.go` — new `ProcessSupportEvent` flow uses clusterer, gap upsert by topic
- `server/internal/service/support_coverage_drafts.go` — remove `drafted`/`fixed` status writes (lines 98, 213, 268)
- `server/internal/service/support_coverage_rules.go` — kept for V1GapType subtype values; classifier dispatch now subordinate to enrichment
- `server/internal/repository/support_coverage_gaps.go` — list query joins evidence for `evidence_30d`
- `server/internal/handler/support_coverage.go` — new `Regenerate` route
- `server/internal/router/router.go` — register `POST /support/coverage/gaps/{gapId}/regenerate`
- `server/cmd/api/main.go` — register coverage gap workflow + cron schedule alongside CRM workflows
- `server/internal/llm/` — no changes required if the existing provider has structured-output support; otherwise add a `GenerateStructured(ctx, prompt, schema)` helper

**Frontend:**
- `frontend/src/pages/support/coverage/SupportCoveragePage.tsx` — substantial rewrite
- `frontend/src/lib/supportCoverageTypes.ts` — update labels, drop `drafted`/`fixed`/`ignored`
- `frontend/src/lib/services/supportCoverageService.ts` — add `regenerate()`
- `frontend/src/hooks/queries/useSupportCoverage.ts` — invalidate on regenerate

---

## Phase 1 — Schema additions (no behavior change)

These migrations add columns and indices. The application keeps writing the old way until Phase 2 cuts over.

### Task 1: Add columns to `support_coverage_topics`

**Files:**
- Create: `server/internal/dbmigrate/sql/2026042700001_coverage_topics_cluster_columns.sql`
- Modify: `server/internal/model/support_coverage.go` (the `SupportCoverageTopic` struct around line 94)

- [ ] **Step 1: Write the migration SQL**

```sql
ALTER TABLE support_coverage_topics
  ADD COLUMN IF NOT EXISTS cluster_key text,
  ADD COLUMN IF NOT EXISTS canonical_title text,
  ADD COLUMN IF NOT EXISTS last_enriched_at timestamptz,
  ADD COLUMN IF NOT EXISTS cooldown_until timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_topics_workspace_cluster_key
  ON support_coverage_topics(workspace_id, cluster_key)
  WHERE cluster_key IS NOT NULL;
```

- [ ] **Step 2: Add fields to the GORM model**

In `server/internal/model/support_coverage.go`, add to `SupportCoverageTopic`:

```go
ClusterKey      *string    `json:"cluster_key" gorm:"type:text"`
CanonicalTitle  *string    `json:"canonical_title" gorm:"type:text"`
LastEnrichedAt  *time.Time `json:"last_enriched_at"`
CooldownUntil   *time.Time `json:"cooldown_until"`
```

- [ ] **Step 3: Apply migration**

Run: `cd server && go run ./cmd/migrate up`
Expected: `applied 2026042700001_coverage_topics_cluster_columns`

- [ ] **Step 4: Verify status**

Run: `cd server && go run ./cmd/migrate status | tail -5`
Expected: row showing the migration as `applied`.

- [ ] **Step 5: Commit**

```bash
git add server/internal/dbmigrate/sql/2026042700001_coverage_topics_cluster_columns.sql server/internal/model/support_coverage.go
git commit -m "feat(coverage): add cluster_key, canonical_title, enrich timestamps to topics"
```

### Task 2: Add lifecycle columns and partial unique index to `support_coverage_gaps`

**Files:**
- Create: `server/internal/dbmigrate/sql/2026042700002_coverage_gaps_lifecycle_columns.sql`
- Modify: `server/internal/model/support_coverage.go` (the `SupportCoverageGap` struct)

- [ ] **Step 1: Write the migration SQL**

```sql
ALTER TABLE support_coverage_gaps
  ADD COLUMN IF NOT EXISTS gap_kind text NOT NULL DEFAULT 'content',
  ADD COLUMN IF NOT EXISTS closed_at timestamptz,
  ADD COLUMN IF NOT EXISTS closed_evidence_count int,
  ADD COLUMN IF NOT EXISTS result_document_id uuid,
  ADD COLUMN IF NOT EXISTS rejection_reason text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gaps_workspace_topic_open
  ON support_coverage_gaps(workspace_id, topic_id)
  WHERE status = 'open' AND topic_id IS NOT NULL;
```

- [ ] **Step 2: Add fields to the GORM model**

```go
GapKind             string     `json:"gap_kind" gorm:"not null;default:'content'"`
ClosedAt            *time.Time `json:"closed_at"`
ClosedEvidenceCount *int       `json:"closed_evidence_count"`
ResultDocumentID    *string    `json:"result_document_id" gorm:"type:uuid"`
RejectionReason     *string    `json:"rejection_reason"`
```

- [ ] **Step 3: Apply migration; verify; commit**

```bash
cd server && go run ./cmd/migrate up && go run ./cmd/migrate status | tail -5
git add -p
git commit -m "feat(coverage): add lifecycle columns and partial-unique-open index to gaps"
```

### Task 3: Add suggestion versioning columns

**Files:**
- Create: `server/internal/dbmigrate/sql/2026042700003_coverage_suggestions_versioning.sql`
- Modify: `server/internal/model/support_coverage.go` (`SupportGapSuggestion` struct)

- [ ] **Step 1: Migration SQL**

```sql
ALTER TABLE support_gap_suggestions
  ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS superseded_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_support_gap_suggestions_gap_active
  ON support_gap_suggestions(gap_id)
  WHERE is_active;
```

- [ ] **Step 2: Add fields to the model**

```go
IsActive     bool       `json:"is_active" gorm:"not null;default:true"`
SupersededAt *time.Time `json:"superseded_at"`
```

- [ ] **Step 3: Apply, verify, commit**

```bash
cd server && go run ./cmd/migrate up
git add -p
git commit -m "feat(coverage): version SupportGapSuggestion with is_active + superseded_at"
```

### Task 4: Status data migration

**Files:**
- Create: `server/internal/dbmigrate/sql/2026042700004_coverage_status_migration.sql`

- [ ] **Step 1: SQL**

```sql
UPDATE support_coverage_gaps SET status = 'done'     WHERE status = 'fixed';
UPDATE support_coverage_gaps SET status = 'rejected' WHERE status = 'ignored';
UPDATE support_coverage_gaps SET status = 'open'     WHERE status = 'drafted';
```

- [ ] **Step 2: Apply; verify with a sanity SELECT**

```bash
cd server && go run ./cmd/migrate up
psql "$DATABASE_URL" -c "SELECT status, count(*) FROM support_coverage_gaps GROUP BY status;"
```

Expected: only `open`, `done`, `rejected` appear.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): collapse drafted/fixed/ignored into open/done/rejected"
```

### Task 5: Update status constants and labels

**Files:**
- Modify: `server/internal/model/support_coverage.go` (the `SupportCoverageGapStatus*` constants)
- Modify: `frontend/src/lib/supportCoverageTypes.ts` (`GAP_STATUS_LABELS`, `STATUS_COLORS`)
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx` (filter button list at line ~302)

- [ ] **Step 1: Backend constants — keep `open`, `done`, `rejected`; remove `drafted`, `fixed`, `ignored`. Search and remove all references.**

```bash
grep -rn "SupportCoverageGapStatusDrafted\|SupportCoverageGapStatusFixed\|SupportCoverageGapStatusIgnored" server/
```

For each hit, either delete or rewrite to use the new tri-state.

- [ ] **Step 2: Frontend labels**

In `supportCoverageTypes.ts`:

```ts
export const GAP_STATUS_LABELS = {
  open: 'Open',
  done: 'Done',
  rejected: 'Rejected',
} as const;

export const STATUS_COLORS: Record<string, string> = {
  open:     'bg-amber-100 text-amber-700',
  done:     'bg-green-100 text-green-700',
  rejected: 'bg-muted text-muted-foreground/60',
};
```

- [ ] **Step 3: Update the filter button list in `SupportCoveragePage.tsx`** to `['', 'open', 'done', 'rejected']`. (The full UI replacement happens in Phase 7; this keeps the page functional in the interim.)

- [ ] **Step 4: Build both sides**

```bash
cd server && go build ./...
cd frontend && pnpm typecheck
```

Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): rename status enum to open/done/rejected"
```

---

## Phase 2 — Topic clusterer

### Task 6: Normalizer (token-sort) — TDD

@superpowers:test-driven-development

**Files:**
- Create: `server/internal/service/support_coverage_clusterer.go`
- Create: `server/internal/service/support_coverage_clusterer_test.go`

- [ ] **Step 1: Write the failing tests first** (table-driven, follows server/CLAUDE.md testing patterns)

```go
package service

import "testing"

func TestNormalizeForCluster(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lowercases and strips punct", "How DO I reset, my password?!", "how i my password reset"},
		{"sorts tokens", "billing late charge", "billing charge late"},
		{"removes stopwords", "the quick brown fox over the lazy dog", "brown dog fox lazy over quick"},
		{"deduplicates tokens", "password password password", "password"},
		{"unicode preserved", "Café résumé", "café résumé"},
		{"strips trailing whitespace", "  hello   world  ", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeForCluster(tt.in)
			if got != tt.want {
				t.Errorf("normalizeForCluster(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run; expect FAIL**

```bash
cd server && go test ./internal/service/ -run TestNormalizeForCluster -v
```

Expected: `undefined: normalizeForCluster` or `FAIL`.

- [ ] **Step 3: Implement minimal**

In `support_coverage_clusterer.go`:

```go
package service

import (
	"sort"
	"strings"
	"unicode"
)

// stopwords is intentionally small (top English support-context words).
// Keep it conservative — over-stemming creates false cluster collisions.
var stopwords = map[string]struct{}{
	"a":   {}, "an":  {}, "and": {}, "as":  {}, "at":  {}, "be":  {}, "by":  {},
	"do":  {}, "for": {}, "from":{}, "have":{}, "how": {}, "i":   {}, "if":  {},
	"in":  {}, "is":  {}, "it":  {}, "my":  {}, "of":  {}, "on":  {}, "or":  {},
	"that":{}, "the": {}, "this":{}, "to":  {}, "was": {}, "what":{}, "when":{},
	"where":{},"why":{}, "with":{},
}

// normalizeForCluster lowercases, strips punctuation, removes stopwords,
// dedupes, and token-sorts so semantically equivalent (but lexically
// distinct) phrasings cluster together. Pure, deterministic.
func normalizeForCluster(s string) string {
	if s == "" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)
	seen := map[string]struct{}{}
	var tokens []string
	for _, tok := range strings.Fields(cleaned) {
		if _, skip := stopwords[tok]; skip {
			continue
		}
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		tokens = append(tokens, tok)
	}
	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}
```

- [ ] **Step 4: Run; expect PASS**

```bash
cd server && go test ./internal/service/ -run TestNormalizeForCluster -v
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_coverage_clusterer*.go
git commit -m "feat(coverage): normalizer for topic-cluster keys"
```

### Task 7: Cluster key composition — TDD

**Files:**
- Modify: `server/internal/service/support_coverage_clusterer.go`
- Modify: `server/internal/service/support_coverage_clusterer_test.go`

- [ ] **Step 1: Tests** that prove the composition rules **don't merge across signal types or unrelated documents**

```go
func TestComputeClusterKey_NoCrossSignalCollision(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "How do I reset my password?")
	b := computeClusterKey("ws-1", "article_feedback",   "", "", "How do I reset my password?")
	if a == b {
		t.Fatalf("different signal types must produce different cluster keys: %q == %q", a, b)
	}
}

func TestComputeClusterKey_NoCrossDocumentCollision(t *testing.T) {
	a := computeClusterKey("ws-1", "article_feedback", "doc-1", "", "Confusing")
	b := computeClusterKey("ws-1", "article_feedback", "doc-2", "", "Confusing")
	if a == b {
		t.Fatalf("article-feedback gaps on different docs must not collide: %q == %q", a, b)
	}
}

func TestComputeClusterKey_StableAcrossWordOrder(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "How do I reset my password?")
	b := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "how to reset password")
	if a != b {
		t.Fatalf("normalized variants of same question must collide: %q != %q", a, b)
	}
}

func TestComputeClusterKey_DifferentWorkspacesIsolated(t *testing.T) {
	a := computeClusterKey("ws-1", "human_reply_after_ai", "", "", "billing")
	b := computeClusterKey("ws-2", "human_reply_after_ai", "", "", "billing")
	if a == b {
		t.Fatalf("workspaces must be isolated: %q == %q", a, b)
	}
}
```

- [ ] **Step 2: Run; expect FAIL**

```bash
cd server && go test ./internal/service/ -run TestComputeClusterKey -v
```

- [ ] **Step 3: Implement**

```go
import (
	"crypto/sha256"
	"encoding/hex"
)

// computeClusterKey returns a sha256 over a pipe-joined tuple that
// disambiguates by workspace, signal type, and document scope. See spec §6.2.
func computeClusterKey(workspaceID, signalType, documentID, issueKey, summary string) string {
	parts := []string{
		workspaceID,
		signalType,
		documentID,
		issueKey,
		normalizeForCluster(summary),
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4: Run; expect PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): cluster-key composition prevents cross-signal/doc merges"
```

### Task 8: Topic + gap upsert by cluster_key — TDD

**Files:**
- Modify: `server/internal/service/support_coverage_clusterer.go`
- Modify: `server/internal/service/support_coverage_clusterer_test.go`
- Modify: `server/internal/repository/support_coverage_topics.go`
- Modify: `server/internal/repository/support_coverage_gaps.go`

- [ ] **Step 1: Tests** (uses in-memory SQLite per server/CLAUDE.md pattern)

```go
func TestUpsertTopicGap_CreatesOnFirstCall(t *testing.T) {
	db := setupCoverageTestDB(t)
	c := NewClusterer(/* repos */)

	gap, err := c.UpsertTopicGap(t.Context(), &model.SupportEvent{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
	})
	if err != nil { t.Fatalf("upsert: %v", err) }
	if gap.ID == "" { t.Fatal("expected gap ID") }
	if gap.EvidenceCount != 1 { t.Errorf("evidence_count=%d, want 1", gap.EvidenceCount) }
}

func TestUpsertTopicGap_SecondCallReusesGap(t *testing.T) {
	db := setupCoverageTestDB(t)
	c := NewClusterer(/* repos */)
	ev := &model.SupportEvent{
		WorkspaceID:  "ws-1",
		EventType:    model.SupportEventHumanReplyAfterAI,
		IssueSummary: "How do I reset my password?",
	}
	g1, _ := c.UpsertTopicGap(t.Context(), ev)
	g2, _ := c.UpsertTopicGap(t.Context(), ev)

	if g1.ID != g2.ID { t.Fatalf("expected same gap, got %s vs %s", g1.ID, g2.ID) }
	if g2.EvidenceCount != 2 { t.Errorf("evidence_count=%d, want 2", g2.EvidenceCount) }
}

func TestUpsertTopicGap_DoneGapDoesNotBlockNewOpen(t *testing.T) {
	db := setupCoverageTestDB(t)
	c := NewClusterer(/* repos */)
	ev := &model.SupportEvent{ /* same as above */ }
	g1, _ := c.UpsertTopicGap(t.Context(), ev)
	// Mark closed via repo:
	_ = repo.MarkDone(t.Context(), g1.ID, ...)
	g2, _ := c.UpsertTopicGap(t.Context(), ev)
	if g1.ID == g2.ID { t.Fatal("done gap must not be reused; expected new open gap") }
}
```

- [ ] **Step 2: Run; expect FAIL**

- [ ] **Step 3: Implement `UpsertTopicGap`**

Pseudocode (adapt to actual repo signatures):

```go
func (c *Clusterer) UpsertTopicGap(ctx context.Context, ev *model.SupportEvent) (*model.SupportCoverageGap, error) {
    key := computeClusterKey(ev.WorkspaceID, string(ev.EventType), coverageDeref(ev.DocumentID), ev.IssueKey, ev.IssueSummary)
    topic, err := c.topicRepo.UpsertByClusterKey(ctx, ev.WorkspaceID, key, ev.IssueSummary /* placeholder title */)
    if err != nil { return nil, fmt.Errorf("upsert topic: %w", err) }

    gap, err := c.gapRepo.GetOpenByTopic(ctx, topic.ID)
    if err != nil { return nil, fmt.Errorf("get open gap: %w", err) }
    if gap == nil {
        gap, err = c.gapRepo.Create(ctx, &model.SupportCoverageGap{
            WorkspaceID: ev.WorkspaceID,
            TopicID:     &topic.ID,
            DedupeKey:   key, // legacy column kept in sync
            GapKind:     "content",
            Status:      model.SupportCoverageGapStatusOpen,
            Title:       coverageTruncate(ev.IssueSummary, 120),
            FirstSeenAt: time.Now(),
        })
        if err != nil { return nil, err }
    }
    if err := c.gapRepo.IncrementEvidence(ctx, gap.ID); err != nil { return nil, err }
    return gap, nil
}
```

- [ ] **Step 4: Run; expect PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): topic+gap upsert keyed by cluster_key"
```

### Task 9: Wire clusterer into `ProcessSupportEvent`

**Files:**
- Modify: `server/internal/service/support_coverage.go`

- [ ] **Step 1: Find current call site** of `classifyEvent` + `gapRule` upsert in `ProcessSupportEvent`. Replace the path that constructs `dedupeKey = hashExcerpt(...)` with a call to `c.UpsertTopicGap(ctx, event)`. **Do not delete `support_coverage_rules.go`** — its V1GapType constants are still used by the enricher (Task 14).

- [ ] **Step 2: Run existing service tests; fix any that break**

```bash
cd server && go test ./internal/service/ -run TestProcessSupportEvent -v
```

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): ProcessSupportEvent uses topic-scoped clusterer"
```

### Task 10: Remove old hash-based gap upsert path

**Files:**
- Modify: `server/internal/service/support_coverage.go`
- Modify: `server/internal/service/support_coverage_rules.go` (if dead helpers)

- [ ] **Step 1: Search for callers of `hashExcerpt` and `buildDedupeKey`**

```bash
grep -rn "hashExcerpt\|buildDedupeKey" server/
```

- [ ] **Step 2: Remove the now-unreachable helpers, but leave `classifyEvent` and the V1GapType constants — they will be used by the enricher in Phase 4 to populate the `gap_subtype`.**

- [ ] **Step 3: Build and test**

```bash
cd server && go build ./... && go test ./internal/service/ -v
```

- [ ] **Step 4: Commit**

```bash
git commit -am "refactor(coverage): remove dead hash-based dedupe helpers"
```

---

## Phase 3 — Cluster rebuild migration (one-shot, destructive)

### Task 11: Rebuild existing open gaps under the new clusterer

The cleanest implementation is **NOT** raw SQL — it requires recomputing the cluster key per gap (Go-side) and merging duplicates. We add a one-shot CLI subcommand that the operator runs once after deploying Phase 2, and an idempotent SQL migration that records the rebuild as applied.

**Files:**
- Create: `server/cmd/migrate/cluster_rebuild.go` (new subcommand)
- Create: `server/internal/dbmigrate/sql/2026042700005_coverage_cluster_rebuild.sql` — only logs that the rebuild ran; the real work is in the Go subcommand

- [ ] **Step 1: Implement the CLI subcommand** `go run ./cmd/migrate cluster-rebuild`

For each workspace:
1. List all `open` gaps (paginated).
2. Load the first evidence row for each gap.
3. Compute the new `cluster_key` from that evidence (using `computeClusterKey`).
4. Upsert the topic; collect `topic_id → []gap_id` mapping.
5. For each topic with >1 gap: pick the gap with highest `evidence_count` as the primary. Reattach evidence rows from the others. Set the others to `status='rejected'`, `rejection_reason='merged into <primary_id> during cluster rebuild 2026-04-27'`, `closed_at=now()`.
6. Log per-workspace `slog.InfoContext` with `workspace_id`, `topics_created`, `gaps_merged`.

- [ ] **Step 2: Test the rebuild on a fixture**

Create `server/cmd/migrate/cluster_rebuild_test.go` with a fixture: 5 gaps in 2 workspaces, summaries chosen so the new clusterer produces 2 topics × 2 gaps each + 1 singleton. Assert post-state.

- [ ] **Step 3: Run on staging**

After Phase 2 ships to staging:

```bash
cd server && DATABASE_URL=$STAGING_DATABASE_URL go run ./cmd/migrate cluster-rebuild
```

Expected: log lines per workspace; new topic count > old; merged-gap count reasonable.

- [ ] **Step 4: Idempotent SQL marker**

`2026042700005_coverage_cluster_rebuild.sql`:

```sql
-- Marker only — the real rebuild is in: go run ./cmd/migrate cluster-rebuild
-- This is idempotent and safe to apply multiple times.
SELECT 1;
```

- [ ] **Step 5: Commit**

```bash
git add server/cmd/migrate/cluster_rebuild*.go server/internal/dbmigrate/sql/2026042700005_*.sql
git commit -m "feat(coverage): one-shot CLI to rebuild gaps under new clusterer"
```

---

## Phase 4 — Enrichment (Temporal workflow)

### Task 12: Define enrichment input/output types

**Files:**
- Modify: `server/internal/service/support_coverage_enrichment.go` (create)

- [ ] **Step 1: Types**

```go
package service

type EnrichmentInput struct {
    TopicID       string
    Evidence      []EvidenceSnippet // top 20 by last_seen
    KBContext     KBContext         // article titles + similarity candidates
}

type EnrichmentResult struct {
    CanonicalTitle    string  // "Write article: …" / "Update article: {existing_title}"
    GapSubtype        string  // matches existing V1GapType constants
    Route             string  // "create_article" or "update_article"
    TargetDocumentID  string  // empty for create_article
    DraftMarkdown     string
    Confidence        float64
}
```

- [ ] **Step 2: Commit**

```bash
git commit -am "feat(coverage): enrichment input/output types"
```

### Task 13: KB context loader — TDD

**Files:**
- Modify: `server/internal/service/support_coverage_enrichment.go`
- Create: `server/internal/service/support_coverage_enrichment_test.go`

- [ ] **Step 1: Test** — given a workspace with N published docs and a topic summary, returns a list of titles + top-5 candidates (by simple token overlap; no embeddings yet).

- [ ] **Step 2-4: TDD cycle** as per Task 6.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): KB context loader with token-overlap top-5 candidates"
```

### Task 14: LLM enrichment call — TDD with mocked provider

**Files:**
- Modify: `server/internal/service/support_coverage_enrichment.go`
- Modify: `server/internal/service/support_coverage_enrichment_test.go`

- [ ] **Step 1: Test with a fake LLM provider that returns a structured response**

```go
type fakeLLM struct{ resp string }
func (f *fakeLLM) GenerateStructured(ctx context.Context, prompt string, _ any) (string, error) {
    return f.resp, nil
}

func TestEnrichTopic_HappyPath(t *testing.T) {
    db := setupCoverageTestDB(t)
    e := NewEnricher(db, &fakeLLM{resp: `{
        "canonical_title": "Write article: Reset your password",
        "gap_subtype":     "missing_article",
        "route":           "create_article",
        "draft_markdown":  "# Reset your password\n\n…",
        "confidence":      0.82
    }`})
    res, err := e.EnrichTopic(t.Context(), "topic-1")
    if err != nil { t.Fatalf("enrich: %v", err) }
    if res.Route != "create_article" { t.Errorf("route=%s, want create_article", res.Route) }
}

func TestEnrichTopic_LLMFailureSurfacesError(t *testing.T) { /* ... */ }
func TestEnrichTopic_SupersedesPriorActiveSuggestion(t *testing.T) { /* ... */ }
func TestEnrichTopic_RespectsOneHourCooldown(t *testing.T) { /* ... */ }
```

- [ ] **Step 2: Run; expect FAIL**

- [ ] **Step 3: Implement `EnrichTopic`** — load topic + evidence + KB context → call LLM → parse → write new active `SupportGapSuggestion` (mark prior `is_active=false, superseded_at=now()`) → update topic `last_enriched_at`, `cooldown_until=now()+1h`, `canonical_title`.

- [ ] **Step 4: Run; expect PASS**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): LLM enrichment writes versioned suggestion + updates topic"
```

### Task 15: Temporal workflow + activity registration

**Files:**
- Create: `server/internal/temporalapp/coverage_gap_workflow.go`
- Create: `server/internal/temporalapp/coverage_gap_workflow_test.go`
- Modify: `server/cmd/api/main.go` (workflow registration alongside CRM workflows)

- [ ] **Step 1: Workflow + activity skeleton**

```go
package temporalapp

import (
    "context"
    "go.temporal.io/sdk/activity"
    "go.temporal.io/sdk/workflow"
)

const CoverageGapEnrichmentWorkflow = "coverage-gap-enrichment"

func EnrichTopicActivity(ctx context.Context, topicID string) error {
    log := activity.GetLogger(ctx)
    log.Info("enriching topic", "topic_id", topicID)
    return enricherSingleton.EnrichTopic(ctx, topicID) // wired via DI in main.go
}

func CoverageGapEnrichmentFlow(ctx workflow.Context, topicID string) error {
    opts := workflow.ActivityOptions{
        StartToCloseTimeout: 60 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, opts)
    return workflow.ExecuteActivity(ctx, EnrichTopicActivity, topicID).Get(ctx, nil)
}
```

- [ ] **Step 2: Test with Temporal test suite** (`testsuite.WorkflowTestSuite`) — mock the activity, assert it's called once.

- [ ] **Step 3: Register in `cmd/api/main.go`** alongside existing CRM workflow registrations.

- [ ] **Step 4: Build**

```bash
cd server && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): Temporal workflow + activity for gap enrichment"
```

### Task 16: Daily batch cron registration

**Files:**
- Modify: `server/cmd/api/main.go` (cron schedule)
- Modify: `server/internal/temporalapp/coverage_gap_workflow.go` (batch flow)

- [ ] **Step 1: Add `CoverageGapDailyBatchFlow`** — lists all workspaces, then per workspace lists topics with `last_enriched_at < now() - 24h AND evidence_count >= 2`, fans out via `workflow.ExecuteChildWorkflow(CoverageGapEnrichmentFlow, topic_id)` with bounded concurrency (10).

- [ ] **Step 2: Register a Temporal cron schedule** (`@every 24h` at the start of UTC day, or use Temporal's `ScheduleSpec` with `CronExpressions: []string{"0 3 * * *"}`).

- [ ] **Step 3: Test the batch flow** with a fixture of N topics + a mock activity.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): daily batch cron + child workflow fan-out"
```

### Task 17: Spike trigger in `ProcessSupportEvent`

**Files:**
- Modify: `server/internal/service/support_coverage.go`

- [ ] **Step 1: Tests** — after each evidence insert, the service checks the 1-hour count and topic cooldown; if `count >= 5` and `cooldown_until < now()`, signals/starts the workflow with a deterministic ID `coverage-gap-enrich-{topic_id}`.

- [ ] **Step 2-4: TDD cycle**

The check (per spec §6.4):

```go
var count int64
err := s.db.WithContext(ctx).Model(&model.SupportGapEvidence{}).
    Where("gap_id = ? AND created_at > ?", gap.ID, time.Now().Add(-time.Hour)).
    Count(&count).Error
if err != nil { /* log and continue — spike trigger is best-effort */ }

topic, _ := s.topicRepo.Get(ctx, *gap.TopicID)
if count >= 5 && (topic.CooldownUntil == nil || topic.CooldownUntil.Before(time.Now())) {
    s.temporal.ExecuteWorkflowAsync(ctx, "coverage-gap-enrich-"+topic.ID, CoverageGapEnrichmentFlow, topic.ID)
}
```

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): spike trigger fires enrichment when ≥5 evidence/hour"
```

### Task 18: Manual regenerate endpoint

**Files:**
- Create: `server/internal/handler/support_coverage_regenerate.go`
- Modify: `server/internal/handler/support_coverage.go` (constructor wiring)
- Modify: `server/internal/router/router.go` (route)
- Modify: `frontend/src/lib/services/supportCoverageService.ts` (client)

- [ ] **Step 1: Handler test** with `httptest` per server/CLAUDE.md handler-test pattern: auth check, 30-sec debounce by per-actor in-memory map (acceptable for v1; can move to Redis later), success returns 202.

- [ ] **Step 2: Implement handler**

```go
func (h *Regenerate) Handle(w http.ResponseWriter, r *http.Request) {
    gapID := chi.URLParam(r, "gapId")
    actor := authorization.ActorFromContext(r.Context())

    if !h.debounce.Allow(actor.UserID, gapID, 30*time.Second) {
        writeError(w, http.StatusTooManyRequests, "regenerating too frequently — wait a moment")
        return
    }
    gap, err := h.svc.GetGap(r.Context(), gapID, actor.WorkspaceID)
    if err != nil { writeError(w, http.StatusNotFound, "not found"); return }
    if err := h.temporal.ExecuteWorkflowAsync(r.Context(), "coverage-gap-enrich-"+*gap.TopicID+"-"+ulid.New(), CoverageGapEnrichmentFlow, *gap.TopicID); err != nil {
        slog.ErrorContext(r.Context(), "enqueue regenerate failed", "error", err, "gap_id", gapID)
        writeError(w, http.StatusServiceUnavailable, "enrichment service unavailable")
        return
    }
    w.WriteHeader(http.StatusAccepted)
}
```

- [ ] **Step 3: Route**

In `router.go`, alongside the other coverage routes:

```go
r.With(requirePerm(authorization.PermSupportEdit)).
    Post("/support/coverage/gaps/{gapId}/regenerate", h.CoverageRegenerate.Handle)
```

- [ ] **Step 4: Frontend service**

```ts
export const supportCoverageService = {
  // ... existing
  regenerate: (gapId: string) =>
    api.post<{ status: 'queued' }>(`/support/coverage/gaps/${gapId}/regenerate`, {}),
};
```

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): POST /gaps/{id}/regenerate endpoint"
```

---

## Phase 5 — Lifecycle service rewrite

### Task 19: `ApplySuggestion` → `Add(create_article)` flow

**Files:**
- Modify: `server/internal/service/support_coverage_drafts.go` (around lines 98, 213, 268)

- [ ] **Step 1: Tests** — Add(create_article) calls `DocsDocumentService.Create` with `DocStatusDraft` content, snapshots evidence_30d into `gap.closed_evidence_count`, sets `result_document_id`, transitions status to `done`.

- [ ] **Step 2: Implement** — replace the current path that sets `status='fixed'` with the new lifecycle write:

```go
func (s *DraftsService) Add(ctx context.Context, gapID string, override *RouteOverride) (*model.DocsDocument, error) {
    sug, err := s.suggestionRepo.GetActiveByGap(ctx, gapID)
    if err != nil || sug == nil { return nil, fmt.Errorf("no active suggestion: %w", err) }

    route := sug.SuggestionType
    targetDocID := sug.TargetDocumentID
    if override != nil {
        route = override.Route
        targetDocID = override.TargetDocumentID
    }

    var resultDoc *model.DocsDocument
    switch route {
    case model.SupportCoverageSuggestionCreateArticle:
        resultDoc, err = s.docs.Create(ctx, &model.DocsCreateRequest{
            WorkspaceID: gap.WorkspaceID,
            Title:       sug.Title,
            BodyMD:      sug.DraftMarkdown,
            // Status defaults to DocStatusDraft per docs_document.go:83
        })
    case model.SupportCoverageSuggestionUpdateArticle:
        err = s.docs.AppendSection(ctx, targetDocID, sug.DraftMarkdown)
        resultDoc, _ = s.docs.Get(ctx, targetDocID)
    default:
        return nil, fmt.Errorf("unknown route %q", route)
    }
    if err != nil { return nil, err }

    evidence30d, _ := s.gapRepo.CountEvidence30d(ctx, gapID)
    return resultDoc, s.gapRepo.MarkDone(ctx, gapID, resultDoc.ID, evidence30d)
}
```

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): Add(create_article) creates a draft Doc and closes the gap"
```

### Task 20: `Add(update_article)` flow

(Mostly covered by Task 19's switch case — verify with a targeted test.)

- [ ] **Step 1: Test for `update_article` route** — ensures the section is appended to the target Doc and the gap closes with `result_document_id = target`.

- [ ] **Step 2: Verify, commit**

```bash
git commit -am "test(coverage): cover Add(update_article) lifecycle"
```

### Task 21: `Reject` flow

**Files:**
- Modify: `server/internal/service/support_coverage_drafts.go`
- Modify: `server/internal/handler/support_coverage.go`

- [ ] **Step 1: Test** — Reject sets `status='rejected'`, `closed_at=now()`, optional `rejection_reason`. No Doc is created.

- [ ] **Step 2-4: TDD cycle**

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): Reject lifecycle with optional reason"
```

### Task 22: Concurrent-resolution guard

- [ ] **Step 1: Test** — two concurrent `Add` calls on the same gap; second receives `409 Conflict` with current state.

- [ ] **Step 2: Implement** — `MarkDone` and `MarkRejected` use `WHERE status='open'` in their UPDATE; check `RowsAffected == 0` → return a sentinel error mapped to 409 in the handler.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): 409 on concurrent gap resolution"
```

### Task 23: Remove old drafted/fixed status writes from `support_coverage_drafts.go`

- [ ] **Step 1: Search**

```bash
grep -n "drafted\|StatusFixed\|StatusDrafted" server/internal/service/support_coverage_drafts.go
```

- [ ] **Step 2: Delete those lines** (specifically the `status = 'drafted'` write at line ~98, and the `status = 'fixed'` writes at ~213, ~268). The new flow in Task 19 already handles the close.

- [ ] **Step 3: Run all coverage tests**

```bash
cd server && go test ./internal/service/ -run "TestSupportCoverage|TestProcessSupportEvent" -v
```

- [ ] **Step 4: Commit**

```bash
git commit -am "refactor(coverage): drop legacy drafted/fixed status writes"
```

---

## Phase 6 — Read API

### Task 24: List query with `evidence_30d`

**Files:**
- Modify: `server/internal/repository/support_coverage_gaps.go`

- [ ] **Step 1: Test** — list returns gaps with an `evidence_30d` count joining `support_gap_evidence` (per spec §6.5 SQL).

- [ ] **Step 2: Implement** — add `evidence_30d` as a non-table-mapped field on the list DTO:

```go
type GapListItem struct {
    model.SupportCoverageGap
    Evidence30d int `json:"evidence_30d"`
}

func (r *Repo) ListOpenWithImpact(ctx context.Context, workspaceID string) ([]GapListItem, error) {
    rows, err := r.db.WithContext(ctx).Raw(`
        SELECT g.*,
          (SELECT COUNT(*) FROM support_gap_evidence e
           WHERE e.gap_id = g.id AND e.created_at > NOW() - interval '30 days') AS evidence_30d
        FROM support_coverage_gaps g
        WHERE g.workspace_id = ? AND g.status = 'open'
        ORDER BY evidence_30d DESC, g.last_seen_at DESC
        LIMIT 50`, workspaceID).Rows()
    // ... scan
}
```

- [ ] **Step 3: Verify with a fixture** that gaps with more recent evidence rank first.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): list gaps ranked by 30-day evidence"
```

### Task 25: Impact tier helper

**Files:**
- Modify: `server/internal/service/support_coverage.go` (or new helper file)

- [ ] **Step 1: Tests** for the boundary cases: 0, 2, 3, 9, 10, 100 → expected tiers (Low / Low / Medium / Medium / High / High).

- [ ] **Step 2: Implement**

```go
func ImpactTier(evidence30d int) string {
    switch {
    case evidence30d >= 10: return "high"
    case evidence30d >= 3:  return "medium"
    default:                return "low"
    }
}
```

- [ ] **Step 3: Wire into the list response** so the frontend doesn't have to recompute.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): impact tier helper exposed in list response"
```

---

## Phase 7 — Frontend rewrite

### Task 26: Type updates

**Files:**
- Modify: `frontend/src/lib/supportCoverageTypes.ts`
- Create: `frontend/src/lib/supportCoverageClusterTypes.ts`

- [ ] **Step 1: Update `SupportCoverageGapListItem`** — add `evidence_30d`, `impact_tier` (`'low' | 'medium' | 'high'`), `canonical_title?`, `gap_kind` (`'content' | 'data' | 'action'`).

- [ ] **Step 2: Update suggestion type** — `route` (`'create_article' | 'update_article'`), `target_document_id?`, `target_document_title?`.

- [ ] **Step 3: Type-check**

```bash
cd frontend && pnpm typecheck
```

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): frontend types for v2 (impact tiers, routes)"
```

### Task 27: `GapImpactBadge` component

**Files:**
- Create: `frontend/src/components/support/coverage/GapImpactBadge.tsx`
- Create: `frontend/src/components/support/coverage/GapImpactBadge.test.tsx`

- [ ] **Step 1: Test** that the component renders the correct label and color for each tier.

- [ ] **Step 2: Implement** with three discrete styles (red/amber/grey).

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): GapImpactBadge component"
```

### Task 28: `GapList` component (left pane)

**Files:**
- Create: `frontend/src/components/support/coverage/GapList.tsx`

- [ ] **Step 1: Sketch test** — with N items, renders rows with category badge + canonical_title + preview + impact badge; clicking a row fires `onSelect(id)`.

- [ ] **Step 2: Implement** — Mirror the structure currently inline in `SupportCoveragePage.tsx:331–366` but isolated.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): GapList component"
```

### Task 29: `GapAddSplitButton` component

**Files:**
- Create: `frontend/src/components/support/coverage/GapAddSplitButton.tsx`

- [ ] **Step 1: Tests** — primary button label exposes routing decision (`Add to "X"` / `Create new article`); dropdown options fire callbacks.

- [ ] **Step 2: Implement** with shadcn `DropdownMenu` (already used elsewhere in the codebase — see `frontend/src/components/ui/dropdown-menu.tsx`).

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): GapAddSplitButton with smart routing + override"
```

### Task 30: `GapDetailPane` component (right pane)

**Files:**
- Create: `frontend/src/components/support/coverage/GapDetailPane.tsx`

- [ ] **Step 1: Tests** — renders header + Reject/Add buttons + inline Markdown preview + evidence list; Regenerate button is disabled for 30 sec after click.

- [ ] **Step 2: Implement** — calls `supportCoverageService.regenerate(gapId)` then `queryClient.invalidateQueries(['support', 'coverage', 'gap', gapId])` on success.

- [ ] **Step 3: Commit**

```bash
git commit -am "feat(coverage): GapDetailPane with Add/Reject/Regenerate"
```

### Task 31: Wire components into `SupportCoveragePage`

**Files:**
- Modify: `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`

- [ ] **Step 1: Replace the inline list and detail JSX** (current lines ~319–600) with `<GapList>` and `<GapDetailPane>`. Keep the data-loading hooks and the route URL state.

- [ ] **Step 2: Add filter chips** (`CONTENT GAPS` always visible; `DATA GAPS`, `ACTION GAPS` rendered as disabled placeholders so the layout doesn't shift in Phase 3/4).

- [ ] **Step 3: Add status segmented control** (`Open` / `Done` / `Rejected`) replacing the current 5-button row.

- [ ] **Step 4: Test in browser** per CLAUDE.md "For UI or frontend changes, start the dev server and use the feature in a browser before reporting the task as complete."

```bash
cd frontend && pnpm dev
# In another terminal, follow the printed URL → Workspace → Support → Coverage
```

Verify:
- List scrolls cleanly (single page scroll, already shipped).
- Add and Reject close the gap.
- Regenerate disables for 30 sec.
- Open in editor opens TipTap with the draft loaded; saving in the editor closes the gap.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(coverage): wire new components into SupportCoveragePage"
```

### Task 32: Open-in-editor handoff

**Files:**
- Modify: `frontend/src/components/support/coverage/GapAddSplitButton.tsx` (or detail pane)
- Modify: `frontend/src/routes/_authenticated/w/$slug/docs/$documentId.tsx` (read URL state)

- [ ] **Step 1: Implementation** — "Open in editor" navigates to the docs editor with `?from_gap={gap_id}&from_suggestion={suggestion_id}` URL params and the `draft_markdown` injected as initial TipTap content via a new `editor.initialContent` prop.

- [ ] **Step 2: On save in the editor**, if the URL params are present, the editor's save handler also calls `POST /support/coverage/gaps/{gap_id}/add` with `{route: 'update_article', target_document_id: <new doc id>}` to close the gap.

- [ ] **Step 3: Browser test** — open a gap, click "Open in editor," edit, save, return to coverage page → gap shows as Done.

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(coverage): Open in editor handoff closes gap on save"
```

---

## Phase 8 — Verification

### Task 33: End-to-end smoke

@superpowers:verification-before-completion

- [ ] **Step 1: Backend tests**

```bash
cd server && go test -race ./internal/service/... ./internal/handler/... ./cmd/migrate/...
```

Expected: PASS.

- [ ] **Step 2: Frontend type check + tests**

```bash
cd frontend && pnpm typecheck && pnpm test
```

Expected: PASS.

- [ ] **Step 3: Build everything**

```bash
pnpm build
```

Expected: PASS.

- [ ] **Step 4: Run a manual smoke** in dev environment:
  1. Generate 5 fake `support_event` rows with similar summaries.
  2. Verify they cluster into 1 gap with `evidence_count=5`.
  3. Hit the spike threshold (5 in 1 hour) → verify enrichment fires.
  4. Open the gap; verify `canonical_title`, draft markdown, route.
  5. Click Add → verify a draft Doc is created and the gap moves to Done.

- [ ] **Step 5: Commit / open PR**

```bash
git push -u origin waqar-work
gh pr create --title "Coverage Gaps redesign (v2)" --body-file - <<'EOF'
## Summary
- Topic-scoped clustering replaces per-event-hash dedupe
- LLM-driven enrichment via Temporal (daily + spike triggers)
- Lifecycle collapsed to Open → Done | Rejected
- Three-column UI with action-verb titles, inline draft, smart-routed Add ▾

## Spec
docs/superpowers/specs/2026-04-27-coverage-gaps-redesign-design.md (v1.2)

## Test plan
- [ ] go test -race ./...
- [ ] pnpm test
- [ ] Manual smoke (see plan §Task 33)
- [ ] Migration applied on staging via `go run ./cmd/migrate up`
- [ ] Cluster rebuild run via `go run ./cmd/migrate cluster-rebuild` on staging

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

---

## Open follow-ups (deliberately deferred)

- Embedding-based clustering when token-sort precision proves insufficient on real data (spec §8).
- Workspace-local batch times (spec §8).
- Outcome dashboards / 30-day deflection reporting (Phase 2).
- Inline KB-editor surfacing of "N readers found this confusing" (Phase 2).
- Phase 3 (data gaps) and Phase 4 (action gaps).
