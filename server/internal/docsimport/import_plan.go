package docsimport

import (
	"fmt"
	"sort"
)

// ImportPlan is the source-agnostic normalized model that every import
// adapter (Nextra, Mintlify, Fumadocs, ...) produces. It is free of
// Helpin database IDs — the executor resolves source IDs to created
// Helpin IDs during import execution.
type ImportPlan struct {
	SourceSystem string
	SourceCommit string
	RootPath     string
	Spaces       []ImportSpace
	Collections  []ImportCollection
	Articles     []ImportArticle
	Assets       []ImportAsset
	Redirects    []ImportRedirect
	Warnings     []Warning
}

// ImportSpace describes a target docs space to create or import into.
type ImportSpace struct {
	SourceID string
	Name     string
	Slug     string
}

// ImportCollection describes a docs collection to create during import.
type ImportCollection struct {
	SourceID       string
	ParentSourceID string
	SpaceSourceID  string
	Name           string
	Slug           string
	Position       int
	Hidden         bool
	SourcePath     string
	SourceRoute    string
}

// ImportArticle describes one document/article to create during import.
type ImportArticle struct {
	SourceID              string
	CollectionSourceID    string
	SpaceSourceID         string
	Title                 string
	Slug                  string
	Description           string
	Position              int
	Hidden                bool
	SourcePath            string
	SourceRoute           string
	Frontmatter           map[string]any
	RawContent            string
	RewrittenContent      string
	UnsupportedComponents []string
}

// ImportAsset describes an asset file discovered in the source archive.
type ImportAsset struct {
	SourcePath   string
	ReferencedBy string
	ContentType  string
	Size         int64
}

// ImportRedirect maps a source route to a target slug in the imported
// docs. The executor resolves slugs to canonical public paths (with
// PublicIDs) after creating the collections and articles.
type ImportRedirect struct {
	SourceRoute         string
	TargetCollectionSlug string
	TargetArticleSlug   string
}

// UnsupportedComponentSummary tallies how many times an unknown MDX
// component appeared across all articles in the import.
type UnsupportedComponentSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ValidateImportPlan checks the plan for structural issues, sorts
// collections and articles by position, and returns any validation
// warnings. It mutates the plan in-place (sorting).
func ValidateImportPlan(plan *ImportPlan) []Warning {
	var warnings []Warning

	if plan.SourceSystem == "" {
		warnings = append(warnings, Warning{
			Type:    "validation_error",
			Message: "source system is required",
		})
	}

	// Check duplicate collection source IDs.
	colSeen := make(map[string]bool, len(plan.Collections))
	for _, c := range plan.Collections {
		if colSeen[c.SourceID] {
			warnings = append(warnings, Warning{
				Type:    "duplicate_source_id",
				Message: fmt.Sprintf("duplicate collection source ID: %s", c.SourceID),
			})
		}
		colSeen[c.SourceID] = true
	}

	// Check duplicate article source IDs.
	artSeen := make(map[string]bool, len(plan.Articles))
	for _, a := range plan.Articles {
		if artSeen[a.SourceID] {
			warnings = append(warnings, Warning{
				Type:    "duplicate_source_id",
				Message: fmt.Sprintf("duplicate article source ID: %s", a.SourceID),
			})
		}
		artSeen[a.SourceID] = true
	}

	// Sort collections: topological order (parents before children),
	// with position ordering among siblings.
	sort.SliceStable(plan.Collections, func(i, j int) bool {
		return plan.Collections[i].Position < plan.Collections[j].Position
	})
	plan.Collections = topoSortCollections(plan.Collections)

	// Sort articles by position.
	sort.SliceStable(plan.Articles, func(i, j int) bool {
		return plan.Articles[i].Position < plan.Articles[j].Position
	})

	return warnings
}

// topoSortCollections reorders collections so that parents appear
// before their children, preserving the existing order among siblings.
func topoSortCollections(cols []ImportCollection) []ImportCollection {
	byID := make(map[string]int, len(cols))
	for i, c := range cols {
		byID[c.SourceID] = i
	}

	sorted := make([]ImportCollection, 0, len(cols))
	added := make(map[string]bool, len(cols))

	var emit func(id string)
	emit = func(id string) {
		if added[id] {
			return
		}
		idx, ok := byID[id]
		if !ok {
			return
		}
		c := cols[idx]
		// Ensure parent is emitted first.
		if c.ParentSourceID != "" && !added[c.ParentSourceID] {
			emit(c.ParentSourceID)
		}
		if !added[id] {
			sorted = append(sorted, c)
			added[id] = true
		}
	}

	for _, c := range cols {
		emit(c.SourceID)
	}

	return sorted
}
