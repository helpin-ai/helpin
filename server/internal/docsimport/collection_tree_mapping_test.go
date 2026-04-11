package docsimport

import "testing"

// TestMapSourceGroupsToDocsCollections_Flat verifies the common case
// where every source group is already top-level. Every mapping should
// come back at depth 0 with no parent and no warnings.
func TestMapSourceGroupsToDocsCollections_Flat(t *testing.T) {
	t.Parallel()

	groups := []ImportSourceGroup{
		{ID: "a", Name: "A", Slug: "a"},
		{ID: "b", Name: "B", Slug: "b"},
	}
	mappings, warnings := MapSourceGroupsToDocsCollections(groups)

	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(mappings) != 2 {
		t.Fatalf("mappings = %d, want 2", len(mappings))
	}
	for i, m := range mappings {
		if m.Depth != 0 {
			t.Fatalf("mappings[%d].Depth = %d, want 0", i, m.Depth)
		}
		if m.ParentSource != "" {
			t.Fatalf("mappings[%d].ParentSource = %q, want empty", i, m.ParentSource)
		}
	}
}

// TestMapSourceGroupsToDocsCollections_NestedWithinCap keeps every
// group inside the allowed depth (0..2) and verifies that parent
// references are preserved and the result is topologically sorted.
func TestMapSourceGroupsToDocsCollections_NestedWithinCap(t *testing.T) {
	t.Parallel()

	groups := []ImportSourceGroup{
		// Intentionally out of topological order so the sort is exercised.
		{ID: "leaf", ParentID: "mid", Name: "Leaf", Slug: "leaf"},
		{ID: "root", Name: "Root", Slug: "root"},
		{ID: "mid", ParentID: "root", Name: "Mid", Slug: "mid"},
	}
	mappings, warnings := MapSourceGroupsToDocsCollections(groups)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if len(mappings) != 3 {
		t.Fatalf("mappings = %d, want 3", len(mappings))
	}

	// Expected topological order: root, mid, leaf.
	want := []struct {
		id     string
		parent string
		depth  int
	}{
		{"root", "", 0},
		{"mid", "root", 1},
		{"leaf", "mid", 2},
	}
	for i, w := range want {
		got := mappings[i]
		if got.SourceID != w.id || got.ParentSource != w.parent || got.Depth != w.depth {
			t.Fatalf("mappings[%d] = %+v, want %+v", i, got, w)
		}
	}
}

// TestMapSourceGroupsToDocsCollections_OverDepthFlattens verifies that
// source groups whose natural depth exceeds the cap are flattened into
// the nearest allowed parent and emit a warning.
func TestMapSourceGroupsToDocsCollections_OverDepthFlattens(t *testing.T) {
	t.Parallel()

	// Natural depth: a(0) -> b(1) -> c(2) -> d(3) -> e(4)
	groups := []ImportSourceGroup{
		{ID: "a", Name: "A"},
		{ID: "b", ParentID: "a", Name: "B"},
		{ID: "c", ParentID: "b", Name: "C"},
		{ID: "d", ParentID: "c", Name: "D"},
		{ID: "e", ParentID: "d", Name: "E"},
	}
	mappings, warnings := MapSourceGroupsToDocsCollections(groups)

	// a, b, c are within the cap; d and e overflow.
	if len(warnings) != 2 {
		t.Fatalf("warnings = %d, want 2", len(warnings))
	}
	warnedIDs := map[string]bool{}
	for _, w := range warnings {
		warnedIDs[w.SourceID] = true
	}
	if !warnedIDs["d"] || !warnedIDs["e"] {
		t.Fatalf("expected warnings for d and e, got %+v", warnings)
	}

	// Every mapping must land at depth <= 2 after clamping.
	for _, m := range mappings {
		if m.Depth > MaxDocsCollectionTreeDepth {
			t.Fatalf("mapping %+v exceeds max depth", m)
		}
	}

	// "d" should clamp so it becomes a child of "c" — wait, c is at
	// depth 2 which is the max, so d cannot go under c. Walk up: d
	// should end up under b (depth 1) giving effective depth 2.
	byID := map[string]DocsCollectionMapping{}
	for _, m := range mappings {
		byID[m.SourceID] = m
	}
	if byID["d"].Depth != 2 || byID["d"].ParentSource != "b" {
		t.Fatalf("d mapping = %+v, want depth 2 under b", byID["d"])
	}
	// "e"'s natural depth is 4. Its parent "d" is already at clamped
	// depth 2, which means "e" can't go under it either. e should
	// walk further up to "a" (depth 0) or become top-level — either
	// way, final depth <= 2.
	if byID["e"].Depth > MaxDocsCollectionTreeDepth {
		t.Fatalf("e mapping = %+v, over max depth", byID["e"])
	}
}

// TestMapSourceGroupsToDocsCollections_MissingParent promotes groups
// with a dangling parent reference to top-level rather than erroring.
func TestMapSourceGroupsToDocsCollections_MissingParent(t *testing.T) {
	t.Parallel()

	groups := []ImportSourceGroup{
		{ID: "orphan", ParentID: "missing", Name: "Orphan"},
	}
	mappings, _ := MapSourceGroupsToDocsCollections(groups)
	if len(mappings) != 1 {
		t.Fatalf("mappings = %d, want 1", len(mappings))
	}
	if mappings[0].Depth != 0 || mappings[0].ParentSource != "" {
		t.Fatalf("mapping = %+v, want top-level", mappings[0])
	}
}
