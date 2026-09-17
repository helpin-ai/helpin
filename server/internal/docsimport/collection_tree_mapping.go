package docsimport

import "fmt"

// MaxDocsCollectionTreeDepth is the deepest allowed value of
// DocsCollection.Depth in the importer. It must stay in sync with the
// service layer constant maxCollectionDepth.
const MaxDocsCollectionTreeDepth = 2

// ImportSourceGroup describes one grouping node as returned by the
// source system, such as a category, collection, or section.
// ImportSourceGroup rows are independent of whatever
// format the source actually uses; the importer adapter translates
// the source's raw shape into this normalised form before calling
// MapSourceGroupsToDocsCollections.
type ImportSourceGroup struct {
	// ID is the source-system identifier of this group. Must be stable
	// for the duration of the import so child groups can reference it.
	ID string
	// ParentID is the source-system identifier of the parent group.
	// Empty for top-level groups.
	ParentID string
	// Name is the human-readable group name. Rendered as the docs
	// collection name.
	Name string
	// Slug is the source-system slug, if any. The importer may replace
	// it with a workspace-unique slug before calling the docs service.
	Slug string
}

// DocsCollectionMapping is one entry in the output of
// MapSourceGroupsToDocsCollections. It records how a source group
// should be materialised as a docs collection: the desired name/slug,
// the resolved parent (empty when top-level), and the clamped depth.
type DocsCollectionMapping struct {
	SourceID     string
	Name         string
	Slug         string
	ParentSource string // source-system parent id. Empty means top-level.
	Depth        int    // 0..MaxDocsCollectionTreeDepth
}

// ImportDepthWarning is emitted by MapSourceGroupsToDocsCollections
// whenever a source group's natural depth exceeds
// MaxDocsCollectionTreeDepth and the importer flattens it into the
// nearest allowed parent. Warnings carry the original source id and
// the effective parent id (after clamping) so the caller can surface
// them in the import report.
type ImportDepthWarning struct {
	SourceID       string
	NaturalDepth   int
	EffectiveDepth int
	Message        string
}

// MapSourceGroupsToDocsCollections resolves a flat list of source
// groups into DocsCollectionMappings respecting the docs collection
// tree depth cap. Groups whose natural depth would exceed the cap are
// clamped into the nearest ancestor that fits, and a warning is
// emitted for each clamped group. Unknown parent references are
// treated as top-level (depth 0) so a dirty source dump cannot block
// the whole import.
//
// The function is pure: it reads groups and produces mappings +
// warnings without touching the database. The caller is responsible
// for creating the collections in dependency order (parents before
// children) — the result is topologically sorted for that purpose.
func MapSourceGroupsToDocsCollections(groups []ImportSourceGroup) ([]DocsCollectionMapping, []ImportDepthWarning) {
	byID := make(map[string]ImportSourceGroup, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}

	// Pre-compute each group's natural depth by walking parents up to
	// the root. Missing parents short-circuit to "root", matching the
	// lenient behaviour above.
	depthOf := make(map[string]int, len(groups))
	var resolveDepth func(id string, visiting map[string]bool) int
	resolveDepth = func(id string, visiting map[string]bool) int {
		if d, ok := depthOf[id]; ok {
			return d
		}
		if visiting[id] {
			// Cycle in the source data — treat as root.
			return 0
		}
		visiting[id] = true
		defer delete(visiting, id)
		g, ok := byID[id]
		if !ok {
			return 0
		}
		if g.ParentID == "" {
			depthOf[id] = 0
			return 0
		}
		if _, exists := byID[g.ParentID]; !exists {
			depthOf[id] = 0
			return 0
		}
		d := resolveDepth(g.ParentID, visiting) + 1
		depthOf[id] = d
		return d
	}
	for _, g := range groups {
		resolveDepth(g.ID, map[string]bool{})
	}

	// Resolve the clamped parent for each group: walk up until we hit
	// an ancestor whose depth <= MaxDocsCollectionTreeDepth-1, so this
	// node lands at depth <= MaxDocsCollectionTreeDepth.
	mappings := make([]DocsCollectionMapping, 0, len(groups))
	warnings := make([]ImportDepthWarning, 0)
	for _, g := range groups {
		natural := depthOf[g.ID]
		effective := natural
		parentID := g.ParentID

		if natural > MaxDocsCollectionTreeDepth {
			// Walk up until we find an ancestor whose depth equals
			// MaxDocsCollectionTreeDepth - 1, so our node lands at the
			// maximum allowed level.
			cursor := parentID
			for cursor != "" {
				parent, ok := byID[cursor]
				if !ok {
					cursor = ""
					break
				}
				if depthOf[cursor] <= MaxDocsCollectionTreeDepth-1 {
					break
				}
				cursor = parent.ParentID
			}
			parentID = cursor
			if cursor == "" {
				effective = 0
			} else {
				effective = depthOf[cursor] + 1
			}
			warnings = append(warnings, ImportDepthWarning{
				SourceID:       g.ID,
				NaturalDepth:   natural,
				EffectiveDepth: effective,
				Message: fmt.Sprintf(
					"source group %q would land at depth %d; flattened into nearest allowed parent at depth %d",
					g.Name, natural, effective,
				),
			})
		}
		// Also drop references to missing or cyclic parents so the
		// service layer doesn't reject the create for an unknown parent.
		if parentID != "" {
			if _, ok := byID[parentID]; !ok {
				parentID = ""
				if effective != 0 {
					effective = 0
				}
			}
		}

		mappings = append(mappings, DocsCollectionMapping{
			SourceID:     g.ID,
			Name:         g.Name,
			Slug:         g.Slug,
			ParentSource: parentID,
			Depth:        effective,
		})
	}

	// Topologically sort so callers can create parents before children.
	// Input order is preserved for nodes that share the same depth.
	sorted := make([]DocsCollectionMapping, 0, len(mappings))
	added := make(map[string]bool, len(mappings))
	var emit func(id string)
	emit = func(id string) {
		if added[id] {
			return
		}
		idx := indexOfMapping(mappings, id)
		if idx < 0 {
			return
		}
		m := mappings[idx]
		if m.ParentSource != "" && !added[m.ParentSource] {
			emit(m.ParentSource)
		}
		if !added[id] {
			sorted = append(sorted, m)
			added[id] = true
		}
	}
	for _, m := range mappings {
		emit(m.SourceID)
	}

	return sorted, warnings
}

func indexOfMapping(mappings []DocsCollectionMapping, id string) int {
	for i := range mappings {
		if mappings[i].SourceID == id {
			return i
		}
	}
	return -1
}
