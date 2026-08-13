package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// dfsItem represents a single node in the expected DFS walk.
type dfsItem struct {
	ID   string
	Type string // "collection" or "doc"
}

// TestDocsOrdering_ThreeSurfaceParity verifies that the internal listing
// (doc repo List + collection repo ListBySpace) with useSortKey=true
// produces items in the expected DFS order determined by sort_key values.
//
// The DFS walk interleaves collections and uncategorized docs at each level
// by merging them in sort_key ASC order, then recurses into each collection.
func TestDocsOrdering_ThreeSurfaceParity(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-parity"
		spaceID     = "space-parity"
		userID      = "user-parity"
	)

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	db := setupDocsOrderingTestDB(t)

	// Create repos with useSortKey=true.
	docRepo := repository.NewDocsDocumentRepository(db, true)
	collRepo := repository.NewDocsCollectionRepository(db, true)

	ctx := context.Background()

	// Seed space.
	seedDocsSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Parity Space",
		Slug:        "parity-space",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeInternal,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	// --- Collections ---
	// 3 top-level collections with deliberately out-of-alpha sort_keys.
	topColls := []model.DocsCollection{
		{ID: "coll-1", SpaceID: spaceID, WorkspaceID: workspaceID, Name: "Coll M", Slug: "coll-m", SortKey: "m", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "coll-2", SpaceID: spaceID, WorkspaceID: workspaceID, Name: "Coll G", Slug: "coll-g", SortKey: "g", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "coll-3", SpaceID: spaceID, WorkspaceID: workspaceID, Name: "Coll S", Slug: "coll-s", SortKey: "s", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
	}
	for _, c := range topColls {
		seedDocsCollection(t, db, c)
	}

	// 2 sub-collections under coll-1 (sort_keys "d" and "p").
	parentID := "coll-1"
	subColls := []model.DocsCollection{
		{ID: "sub-1", SpaceID: spaceID, WorkspaceID: workspaceID, ParentCollectionID: &parentID, Depth: 1, Name: "Sub D", Slug: "sub-d", SortKey: "d", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "sub-2", SpaceID: spaceID, WorkspaceID: workspaceID, ParentCollectionID: &parentID, Depth: 1, Name: "Sub P", Slug: "sub-p", SortKey: "p", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
	}
	for _, c := range subColls {
		seedDocsCollection(t, db, c)
	}

	// --- Documents ---
	// 5 uncategorized docs (collection_id=nil) with interleaved sort_keys.
	uncatDocs := []model.DocsDocument{
		{ID: "doc-u1", WorkspaceID: workspaceID, SpaceID: spaceID, Title: "Doc B", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "b", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-u2", WorkspaceID: workspaceID, SpaceID: spaceID, Title: "Doc H", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "h", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-u3", WorkspaceID: workspaceID, SpaceID: spaceID, Title: "Doc N", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "n", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-u4", WorkspaceID: workspaceID, SpaceID: spaceID, Title: "Doc T", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "t", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-u5", WorkspaceID: workspaceID, SpaceID: spaceID, Title: "Doc Z", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "z", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
	}
	for _, d := range uncatDocs {
		seedDocsOrderingDocument(t, db, d)
	}

	// 4 docs inside coll-1 with sort_keys "c", "f", "k", "r".
	coll1ID := "coll-1"
	coll1Docs := []model.DocsDocument{
		{ID: "doc-c1", WorkspaceID: workspaceID, SpaceID: spaceID, CollectionID: &coll1ID, Title: "Doc C", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "c", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-c2", WorkspaceID: workspaceID, SpaceID: spaceID, CollectionID: &coll1ID, Title: "Doc F", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "f", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-c3", WorkspaceID: workspaceID, SpaceID: spaceID, CollectionID: &coll1ID, Title: "Doc K", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "k", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
		{ID: "doc-c4", WorkspaceID: workspaceID, SpaceID: spaceID, CollectionID: &coll1ID, Title: "Doc R", Status: model.DocStatusPublished, Visibility: "workspace_wide", SortKey: "r", CreatedBy: userID, CreatedAt: now, UpdatedAt: now},
	}
	for _, d := range coll1Docs {
		seedDocsOrderingDocument(t, db, d)
	}

	// --- Query the internal listing ---
	spaceIDPtr := spaceID
	docs, err := docRepo.List(ctx, workspaceID, &spaceIDPtr, nil, nil, nil, "", false)
	if err != nil {
		t.Fatalf("List docs: %v", err)
	}

	colls, err := collRepo.ListBySpace(ctx, spaceID)
	if err != nil {
		t.Fatalf("ListBySpace collections: %v", err)
	}

	// --- Build DFS walk ---
	// Index collections by parent.
	collsByParent := map[string][]model.DocsCollection{} // key="" means top-level
	for _, c := range colls {
		key := ""
		if c.ParentCollectionID != nil {
			key = *c.ParentCollectionID
		}
		collsByParent[key] = append(collsByParent[key], c)
	}

	// Index docs by collection_id (nil → "").
	docsByCollection := map[string][]model.DocsDocument{}
	for _, d := range docs {
		key := ""
		if d.CollectionID != nil {
			key = *d.CollectionID
		}
		docsByCollection[key] = append(docsByCollection[key], d)
	}

	// DFS: at each bucket, merge collections and docs by sort_key ASC, then recurse into collections.
	var walk []dfsItem
	var dfs func(parentCollID string)
	dfs = func(parentCollID string) {
		childColls := collsByParent[parentCollID]
		childDocs := docsByCollection[parentCollID]

		// Merge collections and docs by sort_key ASC (with id ASC as tiebreaker).
		ci, di := 0, 0
		for ci < len(childColls) || di < len(childDocs) {
			pickColl := false
			if ci < len(childColls) && di < len(childDocs) {
				if childColls[ci].SortKey < childDocs[di].SortKey {
					pickColl = true
				} else if childColls[ci].SortKey == childDocs[di].SortKey {
					pickColl = childColls[ci].ID < childDocs[di].ID
				}
			} else if ci < len(childColls) {
				pickColl = true
			}

			if pickColl {
				c := childColls[ci]
				walk = append(walk, dfsItem{ID: c.ID, Type: "collection"})
				dfs(c.ID) // recurse into this collection
				ci++
			} else {
				d := childDocs[di]
				walk = append(walk, dfsItem{ID: d.ID, Type: "doc"})
				di++
			}
		}
	}

	dfs("") // start from top-level (no parent)

	// --- Expected DFS walk ---
	// Top-level bucket: uncategorized docs (collection_id=NULL come first in the
	// SQL ordering: "collection_id ASC NULLS FIRST, sort_key ASC, id ASC"),
	// but in the DFS we merge top-level collections and uncategorized docs by sort_key.
	//
	// Uncategorized docs sort_keys: b, h, n, t, z
	// Top-level collections sort_keys: g (coll-2), m (coll-1), s (coll-3)
	//
	// Merged top-level order by sort_key:
	//   b (doc-u1) < g (coll-2) < h (doc-u2) < m (coll-1) < n (doc-u3) < s (coll-3) < t (doc-u4) < z (doc-u5)
	//
	// After coll-2 (no children): nothing to recurse.
	// After coll-1: recurse into coll-1's children.
	//   coll-1 sub-collections sort_keys: d (sub-1), p (sub-2)
	//   coll-1 docs sort_keys: c (doc-c1), f (doc-c2), k (doc-c3), r (doc-c4)
	//   Merged: c (doc-c1) < d (sub-1) < f (doc-c2) < k (doc-c3) < p (sub-2) < r (doc-c4)
	//   sub-1 and sub-2 have no children.
	// After coll-3 (no children): nothing to recurse.

	expected := []dfsItem{
		{ID: "doc-u1", Type: "doc"},        // sort_key "b"
		{ID: "coll-2", Type: "collection"}, // sort_key "g"
		// coll-2 children: none
		{ID: "doc-u2", Type: "doc"},        // sort_key "h"
		{ID: "coll-1", Type: "collection"}, // sort_key "m"
		// coll-1 children (recursed):
		{ID: "doc-c1", Type: "doc"},       // sort_key "c"
		{ID: "sub-1", Type: "collection"}, // sort_key "d"
		// sub-1 children: none
		{ID: "doc-c2", Type: "doc"},       // sort_key "f"
		{ID: "doc-c3", Type: "doc"},       // sort_key "k"
		{ID: "sub-2", Type: "collection"}, // sort_key "p"
		// sub-2 children: none
		{ID: "doc-c4", Type: "doc"}, // sort_key "r"
		// back to top-level
		{ID: "doc-u3", Type: "doc"},        // sort_key "n"
		{ID: "coll-3", Type: "collection"}, // sort_key "s"
		// coll-3 children: none
		{ID: "doc-u4", Type: "doc"}, // sort_key "t"
		{ID: "doc-u5", Type: "doc"}, // sort_key "z"
	}

	// --- Assert ---
	if len(walk) != len(expected) {
		t.Fatalf("DFS walk length = %d, want %d\n  got:  %v\n  want: %v", len(walk), len(expected), walk, expected)
	}

	for i := range expected {
		if walk[i].ID != expected[i].ID || walk[i].Type != expected[i].Type {
			t.Errorf("DFS walk[%d] = {%s, %s}, want {%s, %s}", i, walk[i].ID, walk[i].Type, expected[i].ID, expected[i].Type)
		}
	}
}
