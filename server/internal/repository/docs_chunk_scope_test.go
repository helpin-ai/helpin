package repository

import (
	"reflect"
	"sort"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildDocsKnowledgeScopeFilterKeepsEachScopeNarrow(t *testing.T) {
	collectionID := "collection-1"
	documentID := "document-1"
	sources := []model.AgentKnowledgeSource{
		{ScopeType: model.KnowledgeSourceScopeSpace, SpaceID: "space-1"},
		{ScopeType: model.KnowledgeSourceScopeCollection, SpaceID: "space-2", CollectionID: &collectionID},
		{ScopeType: model.KnowledgeSourceScopeArticle, SpaceID: "space-3", DocumentID: &documentID},
		// Duplicate rows must not widen the resulting SQL filter.
		{ScopeType: model.KnowledgeSourceScopeArticle, SpaceID: "space-3", DocumentID: &documentID},
	}

	filter := BuildDocsKnowledgeScopeFilter(sources)
	sort.Strings(filter.FullSpaceIDs)
	sort.Strings(filter.CollectionIDs)
	sort.Strings(filter.DocumentIDs)

	if !reflect.DeepEqual(filter.FullSpaceIDs, []string{"space-1"}) {
		t.Fatalf("full spaces = %#v, want only the explicitly selected whole space", filter.FullSpaceIDs)
	}
	if !reflect.DeepEqual(filter.CollectionIDs, []string{"collection-1"}) {
		t.Fatalf("collections = %#v, want selected collection", filter.CollectionIDs)
	}
	if !reflect.DeepEqual(filter.DocumentIDs, []string{"document-1"}) {
		t.Fatalf("documents = %#v, want selected article without duplicates", filter.DocumentIDs)
	}
}
