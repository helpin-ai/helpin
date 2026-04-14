package docsimport

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// NextraAdapter builds an ImportPlan from a NextraArchive.
type NextraAdapter struct{}

// NextraPreviewOptions configures the adapter's preview behavior.
type NextraPreviewOptions struct {
	SourceCommit string
}

// separatorGroup represents a section of the sidebar defined by a
// type:"separator" entry in the root _meta. All items between two
// separators belong to the same group and become children of a
// collection named after the separator.
type separatorGroup struct {
	key      string // separator meta key (e.g. "-new-to-usermaven")
	title    string
	position int
	// keys of items that belong to this group (in order).
	itemKeys []string
}

// Preview scans the archive, parses meta files, processes MDX content,
// resolves assets and links, and returns a normalised ImportPlan.
func (a NextraAdapter) Preview(ctx context.Context, archive *NextraArchive, opts NextraPreviewOptions) (*ImportPlan, []Warning, error) {
	var allWarnings []Warning

	contentFiles := archive.ContentFiles()
	publicFiles := archive.PublicFiles()

	// Build a map of directory -> parsed _meta entries.
	metaByDir := buildMetaMap(contentFiles, &allWarnings)

	// Discover all docs files and organise by directory.
	type docFile struct {
		relPath string // relative to content root
		dir     string
		base    string // filename without extension
	}
	var docs []docFile
	dirs := map[string]bool{}

	for relPath := range contentFiles {
		if !isDocsFile(relPath) {
			continue
		}
		dir := path.Dir(relPath)
		if dir == "." {
			dir = ""
		}
		base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
		docs = append(docs, docFile{relPath: relPath, dir: dir, base: base})
		if dir != "" {
			dirs[dir] = true
		}
	}

	spaceID := "default"

	// --- Build collections ---
	// Strategy: check root _meta for separator entries. If separators
	// exist, they define the top-level collection grouping. Items
	// between separators are children of that collection. Directories
	// within a group become sub-collections.
	// If no separators exist, fall back to directory-only collections.

	rootMeta := metaByDir[""]
	groups := buildSeparatorGroups(rootMeta)

	var collections []ImportCollection
	// Map from item key (at root level) to its separator collection ID.
	itemToSepCollection := map[string]string{}
	sepCollectionPosition := map[string]int{}

	if len(groups) > 0 {
		// Create a collection for each separator group.
		for _, g := range groups {
			slug := cleanSeparatorSlug(g.key)
			colID := "sep:" + g.key
			collections = append(collections, ImportCollection{
				SourceID:      colID,
				SpaceSourceID: spaceID,
				Name:          g.title,
				Slug:          slug,
				Position:      g.position,
				SourcePath:    "",
				SourceRoute:   "",
			})
			sepCollectionPosition[colID] = g.position

			for _, itemKey := range g.itemKeys {
				itemToSepCollection[itemKey] = colID
			}
		}

		// Directories that are listed in a separator group become
		// sub-collections of that group.
		for dir := range dirs {
			// Only handle top-level dirs here; nested dirs are handled below.
			if strings.Contains(dir, "/") {
				continue
			}
			dirName := filepath.Base(dir)
			parentColID := itemToSepCollection[dirName]
			if parentColID == "" {
				// Dir not in any separator group — create as top-level.
				parentColID = ""
			}

			title, position, hidden := dirMetaInfo(dirName, rootMeta)

			collections = append(collections, ImportCollection{
				SourceID:       "col:" + dir,
				ParentSourceID: parentColID,
				SpaceSourceID:  spaceID,
				Name:           title,
				Slug:           dirName,
				Position:       position,
				Hidden:         hidden,
				SourcePath:     dir,
				SourceRoute:    "/" + dir,
			})
		}

		// Handle nested directories (depth > 1).
		for dir := range dirs {
			if !strings.Contains(dir, "/") {
				continue
			}
			dirName := filepath.Base(dir)
			parentDir := filepath.Dir(dir)

			title := titleCase(dirName)
			position := 999
			hidden := false

			if entries, ok := metaByDir[parentDir]; ok {
				for i, e := range entries {
					if e.Key == dirName {
						if e.Title != "" {
							title = e.Title
						}
						position = i
						if e.Display == "hidden" {
							hidden = true
						}
						break
					}
				}
			}

			parentColID := "col:" + parentDir

			collections = append(collections, ImportCollection{
				SourceID:       "col:" + dir,
				ParentSourceID: parentColID,
				SpaceSourceID:  spaceID,
				Name:           title,
				Slug:           dirName,
				Position:       position,
				Hidden:         hidden,
				SourcePath:     dir,
				SourceRoute:    "/" + dir,
			})
		}
	} else {
		// No separators — fall back to directory-only collections.
		var sourceGroups []ImportSourceGroup
		for dir := range dirs {
			dirName := filepath.Base(dir)
			parentDir := filepath.Dir(dir)
			if parentDir == "." {
				parentDir = ""
			}
			title, position, _ := dirMetaInfo(dirName, metaByDir[parentDir])
			sg := ImportSourceGroup{
				ID:       "col:" + dir,
				ParentID: "",
				Name:     title,
				Slug:     dirName,
			}
			if parentDir != "" {
				sg.ParentID = "col:" + parentDir
			}
			_ = position
			sourceGroups = append(sourceGroups, sg)
		}
		colMappings, depthWarnings := MapSourceGroupsToDocsCollections(sourceGroups)
		for _, dw := range depthWarnings {
			allWarnings = append(allWarnings, Warning{
				Type:    "collection_depth_capped",
				Message: dw.Message,
			})
		}
		for _, cm := range colMappings {
			dir := strings.TrimPrefix(cm.SourceID, "col:")
			title, position, hidden := dirMetaInfo(filepath.Base(dir), metaByDir[filepath.Dir(dir)])
			_ = title // use cm.Name which already has it
			collections = append(collections, ImportCollection{
				SourceID:       cm.SourceID,
				ParentSourceID: cm.ParentSource,
				SpaceSourceID:  spaceID,
				Name:           cm.Name,
				Slug:           cm.Slug,
				Position:       position,
				Hidden:         hidden,
				SourcePath:     dir,
				SourceRoute:    "/" + dir,
			})
		}
	}

	// --- Build articles ---
	articles := make([]ImportArticle, 0, len(docs))
	for _, df := range docs {
		data := contentFiles[df.relPath]
		doc, mdxWarnings, err := ParseNextraMDX(df.relPath, data)
		if err != nil {
			allWarnings = append(allWarnings, Warning{
				Type:    "mdx_parse_error",
				Message: fmt.Sprintf("%s: %v", df.relPath, err),
			})
			continue
		}
		allWarnings = append(allWarnings, mdxWarnings...)

		// Determine position from _meta (title comes from the article itself).
		position := 999
		hidden := false
		if entries, ok := metaByDir[df.dir]; ok {
			for i, e := range entries {
				if e.Key == df.base {
					position = i
					if e.Display == "hidden" {
						hidden = true
					}
					break
				}
			}
		}

		if doc.Title == "" {
			doc.Title = titleCase(df.base)
		}

		sourceRoute := NextraRouteForPath(archive.RootPath, archive.fullPath(df.relPath))

		// Determine which collection this article belongs to.
		collectionSourceID := ""
		if df.dir != "" {
			// Article is inside a directory — belongs to that directory's collection.
			collectionSourceID = "col:" + df.dir
		} else if len(groups) > 0 {
			// Root-level article — check if it's in a separator group.
			if sepColID, ok := itemToSepCollection[df.base]; ok {
				collectionSourceID = sepColID
			}
		}

		article := ImportArticle{
			SourceID:              "art:" + df.relPath,
			CollectionSourceID:    collectionSourceID,
			SpaceSourceID:         spaceID,
			Title:                 doc.Title,
			Slug:                  doc.Slug,
			Description:           doc.Description,
			Position:              position,
			Hidden:                hidden || doc.Draft,
			SourcePath:            df.relPath,
			SourceRoute:           sourceRoute,
			Frontmatter:           doc.Frontmatter,
			RawContent:            doc.Body,
			UnsupportedComponents: doc.UnsupportedComponents,
		}
		articles = append(articles, article)
	}

	// Plan assets.
	var allAssets []ImportAsset
	for i, article := range articles {
		assets, rewritten, assetWarnings := PlanNextraAssets(contentFiles, publicFiles, article)
		allWarnings = append(allWarnings, assetWarnings...)
		allAssets = append(allAssets, assets...)
		articles[i].RawContent = rewritten
	}

	// Plan redirects.
	redirects := make([]ImportRedirect, 0, len(articles))
	for _, a := range articles {
		if a.SourceRoute != "" {
			redirect := ImportRedirect{
				SourceRoute:       a.SourceRoute,
				TargetArticleSlug: a.Slug,
			}
			if a.CollectionSourceID != "" {
				for _, c := range collections {
					if c.SourceID == a.CollectionSourceID {
						redirect.TargetCollectionSlug = c.Slug
						break
					}
				}
			}
			redirects = append(redirects, redirect)
		}
	}

	plan := &ImportPlan{
		SourceSystem: "nextra",
		SourceCommit: opts.SourceCommit,
		RootPath:     archive.RootPath,
		Spaces:       []ImportSpace{{SourceID: spaceID, Name: "Imported Docs", Slug: "imported-docs"}},
		Collections:  collections,
		Articles:     articles,
		Assets:       allAssets,
		Redirects:    redirects,
		Warnings:     allWarnings,
	}

	validationWarnings := ValidateImportPlan(plan)
	allWarnings = append(allWarnings, validationWarnings...)

	return plan, allWarnings, nil
}

// buildSeparatorGroups scans root _meta entries and groups items
// between type:"separator" entries. Returns nil if no separators found.
func buildSeparatorGroups(rootMeta []NextraMetaEntry) []separatorGroup {
	if len(rootMeta) == 0 {
		return nil
	}

	// Check if there are any separators at all.
	hasSeparators := false
	for _, e := range rootMeta {
		if e.Type == "separator" {
			hasSeparators = true
			break
		}
	}
	if !hasSeparators {
		return nil
	}

	var groups []separatorGroup
	var current *separatorGroup

	for i, e := range rootMeta {
		if e.Type == "separator" {
			// Start a new group.
			if current != nil {
				groups = append(groups, *current)
			}
			current = &separatorGroup{
				key:      e.Key,
				title:    e.Title,
				position: i,
			}
			continue
		}

		// Skip separator entries with empty titles (visual spacers).
		if current != nil && current.title == "" {
			// This is a spacer separator — promote items to top level
			// by closing the current group without adding items.
			groups = append(groups, *current)
			current = nil
		}

		if current != nil {
			current.itemKeys = append(current.itemKeys, e.Key)
		}
		// Items before the first separator have no group (root level).
	}

	if current != nil {
		groups = append(groups, *current)
	}

	return groups
}

// cleanSeparatorSlug converts a separator key like "-new-to-usermaven"
// to a clean slug "new-to-usermaven".
func cleanSeparatorSlug(key string) string {
	slug := strings.TrimLeft(key, "-")
	if slug == "" {
		slug = key
	}
	// Remove common prefixes like "separator-"
	slug = strings.TrimPrefix(slug, "separator-")
	slug = strings.TrimPrefix(slug, "seprator-") // handle typos
	return slug
}

// dirMetaInfo looks up a directory name in meta entries and returns
// its title, position, and hidden state.
func dirMetaInfo(dirName string, entries []NextraMetaEntry) (string, int, bool) {
	title := titleCase(dirName)
	position := 999
	hidden := false
	for i, e := range entries {
		if e.Key == dirName {
			if e.Title != "" {
				title = e.Title
			}
			position = i
			if e.Display == "hidden" {
				hidden = true
			}
			break
		}
	}
	return title, position, hidden
}

// buildMetaMap parses all _meta files in content and returns a map
// from directory path to ordered meta entries.
func buildMetaMap(contentFiles map[string][]byte, warnings *[]Warning) map[string][]NextraMetaEntry {
	metaByDir := make(map[string][]NextraMetaEntry)

	for relPath, data := range contentFiles {
		if !isMetaFile(relPath) {
			continue
		}
		dir := path.Dir(relPath)
		if dir == "." {
			dir = ""
		}
		entries, metaWarnings := ParseNextraMeta(relPath, data)
		*warnings = append(*warnings, metaWarnings...)
		metaByDir[dir] = entries
	}

	return metaByDir
}

// fullPath returns the full path (content root + relative path) for a
// file in the archive.
func (a *NextraArchive) fullPath(relPath string) string {
	if a.RootPath == "." {
		return relPath
	}
	return a.RootPath + "/" + relPath
}

// titleCase converts a kebab-case or snake_case string to title case.
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// dedupeSlug ensures a slug is unique within a set by appending a
// numeric suffix if needed.
func dedupeSlug(slug string, seen map[string]bool) string {
	if !seen[slug] {
		seen[slug] = true
		return slug
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		if !seen[candidate] {
			seen[candidate] = true
			return candidate
		}
	}
}
