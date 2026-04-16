package docsimport

import (
	"fmt"
	"mime"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// PlanNextraAssets discovers image and asset references in an
// article's raw content and resolves them against the archive files.
// contentFiles are relative to the content root; publicFiles are
// relative to the public/ directory.
func PlanNextraAssets(contentFiles map[string][]byte, publicFiles map[string][]byte, article ImportArticle) ([]ImportAsset, string, []Warning) {
	var assets []ImportAsset
	var warnings []Warning
	seen := map[string]bool{}
	rewritten := article.RawContent

	// Find markdown image references: ![alt](src)
	mdImgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	// Find HTML img src: <img src="..." />
	htmlImgRe := regexp.MustCompile(`<img\s[^>]*src=["']([^"']+)["']`)

	refs := extractRefs(article.RawContent, mdImgRe, htmlImgRe)

	articleDir := path.Dir(article.SourcePath)

	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true

		// Skip remote URLs.
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			continue
		}

		// Skip data URIs.
		if strings.HasPrefix(ref, "data:") {
			continue
		}

		var resolvedPath string
		var found bool

		if strings.HasPrefix(ref, "/") {
			// Absolute path — resolve from public/ directory.
			pubPath := strings.TrimPrefix(ref, "/")
			if publicFiles != nil {
				if _, ok := publicFiles[pubPath]; ok {
					resolvedPath = "public/" + pubPath
					found = true
				}
			}
			// Also try in content files.
			if !found {
				if _, ok := contentFiles[pubPath]; ok {
					resolvedPath = pubPath
					found = true
				}
			}
		} else {
			// Relative path — resolve from article's directory.
			rel := ref
			if strings.HasPrefix(rel, "./") {
				rel = strings.TrimPrefix(rel, "./")
			}
			candidate := path.Clean(articleDir + "/" + rel)
			if candidate == "." {
				candidate = rel
			}
			if _, ok := contentFiles[candidate]; ok {
				resolvedPath = candidate
				found = true
			}
		}

		if !found {
			warnings = append(warnings, Warning{
				Type:    "missing_asset",
				Message: fmt.Sprintf("asset %q referenced in %s not found in archive", ref, article.SourcePath),
			})
			continue
		}

		contentType := mime.TypeByExtension(filepath.Ext(resolvedPath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		var size int64
		if data, ok := contentFiles[resolvedPath]; ok {
			size = int64(len(data))
		} else if publicFiles != nil {
			pubPath := strings.TrimPrefix(resolvedPath, "public/")
			if data, ok := publicFiles[pubPath]; ok {
				size = int64(len(data))
			}
		}

		assets = append(assets, ImportAsset{
			SourcePath:   resolvedPath,
			ReferencedBy: article.SourceID,
			ContentType:  contentType,
			Size:         size,
		})
		rewritten = rewriteNextraAssetRef(rewritten, ref, resolvedPath)
	}

	return assets, rewritten, warnings
}

func rewriteNextraAssetRef(content, oldRef, newRef string) string {
	if oldRef == "" || newRef == "" || oldRef == newRef {
		return content
	}
	content = strings.ReplaceAll(content, "("+oldRef+")", "("+newRef+")")
	content = strings.ReplaceAll(content, `"`+oldRef+`"`, `"`+newRef+`"`)
	content = strings.ReplaceAll(content, `'`+oldRef+`'`, `'`+newRef+`'`)
	return content
}

// extractRefs collects all capture group 1 matches from the given
// regexps applied to content.
func extractRefs(content string, patterns ...*regexp.Regexp) []string {
	var refs []string
	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) > 1 && m[1] != "" {
				refs = append(refs, m[1])
			}
		}
	}
	return refs
}
