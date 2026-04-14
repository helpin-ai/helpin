package docsimport

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	// MaxArchiveFiles is the maximum number of files allowed in a zip.
	MaxArchiveFiles = 10_000
)

// NextraArchive holds the in-memory contents of a validated Nextra
// docs zip archive. Paths are slash-separated and relative.
type NextraArchive struct {
	// Files maps relative paths to file contents.
	Files map[string][]byte
	// RootPath is the detected Nextra content root (e.g. "content",
	// "src/content", "docs", or "." for repo root).
	RootPath string
}

// ReadNextraArchive reads a zip archive from r, validates it for
// safety (no path traversal, size limits, file count), and returns
// the in-memory file map plus the detected Nextra content root.
func ReadNextraArchive(r io.ReaderAt, size int64, maxUncompressedSize int64) (*NextraArchive, []Warning, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip: %w", err)
	}

	if len(zr.File) > MaxArchiveFiles {
		return nil, nil, fmt.Errorf("archive contains %d files, exceeds maximum %d", len(zr.File), MaxArchiveFiles)
	}

	var warnings []Warning
	files := make(map[string][]byte, len(zr.File))
	var totalSize int64

	for _, f := range zr.File {
		// Skip directories.
		if f.FileInfo().IsDir() {
			continue
		}

		name := filepath.ToSlash(f.Name)

		// Skip macOS resource fork directories.
		if strings.HasPrefix(name, "__MACOSX/") || strings.Contains(name, "/__MACOSX/") {
			continue
		}

		// Skip macOS .DS_Store and ._ resource files.
		base := filepath.Base(name)
		if base == ".DS_Store" || strings.HasPrefix(base, "._") {
			continue
		}

		// Reject path traversal (../ or /..) but allow Next.js
		// catch-all routes like [[...slug]].
		if strings.Contains(name, "../") || strings.HasPrefix(name, "..") {
			return nil, nil, fmt.Errorf("path traversal detected: %q", name)
		}

		// Reject absolute paths.
		if strings.HasPrefix(name, "/") {
			return nil, nil, fmt.Errorf("absolute path detected: %q", name)
		}

		// Skip non-docs files.
		if shouldSkipFile(name) {
			continue
		}

		// Check cumulative uncompressed size.
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxUncompressedSize {
			return nil, nil, fmt.Errorf("uncompressed size exceeds limit of %d bytes", maxUncompressedSize)
		}

		rc, err := f.Open()
		if err != nil {
			warnings = append(warnings, Warning{
				Type:    "archive_read_error",
				Message: fmt.Sprintf("cannot read %q: %v", name, err),
			})
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxUncompressedSize-totalSize+int64(f.UncompressedSize64)))
		rc.Close()
		if err != nil {
			warnings = append(warnings, Warning{
				Type:    "archive_read_error",
				Message: fmt.Sprintf("error reading %q: %v", name, err),
			})
			continue
		}
		files[name] = data
	}

	// Strip common top-level directory wrapper (e.g. "myrepo-main/"
	// from GitHub zip downloads).
	files = stripCommonPrefix(files)

	rootPath := detectNextraRoot(files)

	return &NextraArchive{Files: files, RootPath: rootPath}, warnings, nil
}

// stripCommonPrefix detects and removes a single common top-level
// directory that wraps all files (e.g. "myrepo-main/" from GitHub
// zip downloads). If files don't share a common prefix, returns as-is.
func stripCommonPrefix(files map[string][]byte) map[string][]byte {
	if len(files) == 0 {
		return files
	}

	// Find the common first path component.
	var commonPrefix string
	first := true
	for name := range files {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			// File at root level — no common wrapper.
			return files
		}
		if first {
			commonPrefix = parts[0]
			first = false
			continue
		}
		if parts[0] != commonPrefix {
			// Files don't share a common top-level dir.
			return files
		}
	}

	if commonPrefix == "" {
		return files
	}

	// Strip the prefix.
	prefix := commonPrefix + "/"
	result := make(map[string][]byte, len(files))
	for name, data := range files {
		result[strings.TrimPrefix(name, prefix)] = data
	}
	return result
}

// shouldSkipFile returns true for paths that should not be imported.
func shouldSkipFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, p := range parts {
		switch p {
		case "node_modules", ".git", ".next", ".turbo", ".cache", "__pycache__":
			return true
		}
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".mdx", ".json", ".js", ".ts", ".jsx", ".tsx",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico",
		".mp4", ".webm", ".pdf":
		return false
	}

	// Allow specific config files.
	base := filepath.Base(name)
	switch base {
	case "package.json", "next.config.js", "next.config.mjs", "next.config.ts":
		return false
	}

	// Skip everything else.
	return true
}

// detectNextraRoot examines file paths to find the Nextra content
// root. It checks common locations in priority order.
func detectNextraRoot(files map[string][]byte) string {
	// Candidate roots in priority order. Each is checked for the
	// presence of .md/.mdx files or _meta files.
	candidates := []string{
		"content",
		"src/content",
		"pages",
		"src/pages",
		"app",
		"src/app",
		"docs",
		"apps/docs/content",
		"apps/docs/pages",
		"apps/docs/src/content",
		"apps/docs",
	}

	for _, candidate := range candidates {
		if hasDocsFilesUnder(files, candidate) {
			return candidate
		}
	}

	// Check if docs files exist at the root.
	if hasDocsFilesUnder(files, ".") {
		return "."
	}

	// Fallback: find the first directory containing an .mdx or .md file.
	for path := range files {
		if isDocsFile(path) {
			dir := filepath.Dir(filepath.ToSlash(path))
			if dir == "." {
				return "."
			}
			// Return the top-most directory.
			parts := strings.SplitN(dir, "/", 2)
			return parts[0]
		}
	}

	return "."
}

// hasDocsFilesUnder returns true if there are .md/.mdx or _meta files
// under the given prefix.
func hasDocsFilesUnder(files map[string][]byte, prefix string) bool {
	for path := range files {
		var rel string
		if prefix == "." {
			rel = path
		} else if strings.HasPrefix(path, prefix+"/") {
			rel = strings.TrimPrefix(path, prefix+"/")
		} else {
			continue
		}
		if rel == "" {
			continue
		}
		if isDocsFile(rel) || isMetaFile(rel) {
			return true
		}
	}
	return false
}

// isDocsFile returns true for .md and .mdx files.
func isDocsFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

// isMetaFile returns true for Nextra _meta files.
func isMetaFile(path string) bool {
	base := filepath.Base(path)
	return base == "_meta.json" || base == "_meta.js" || base == "_meta.ts" ||
		base == "_meta.jsx" || base == "_meta.tsx"
}

// ContentFiles returns only the files under the detected root path,
// with paths relative to that root.
func (a *NextraArchive) ContentFiles() map[string][]byte {
	if a.RootPath == "." {
		return a.Files
	}
	prefix := a.RootPath + "/"
	result := make(map[string][]byte)
	for path, data := range a.Files {
		if strings.HasPrefix(path, prefix) {
			result[strings.TrimPrefix(path, prefix)] = data
		}
	}
	return result
}

// PublicFiles returns files from the public/ directory (relative to
// the repo root, not the content root), with paths relative to public/.
func (a *NextraArchive) PublicFiles() map[string][]byte {
	result := make(map[string][]byte)
	// Try common public dirs relative to content root's parent.
	publicPrefixes := []string{"public/"}
	if a.RootPath != "." {
		// Also check public/ relative to the parent of the content root.
		parts := strings.Split(a.RootPath, "/")
		if len(parts) > 1 {
			parent := strings.Join(parts[:len(parts)-1], "/")
			publicPrefixes = append(publicPrefixes, parent+"/public/")
		}
	}

	for path, data := range a.Files {
		for _, prefix := range publicPrefixes {
			if strings.HasPrefix(path, prefix) {
				rel := strings.TrimPrefix(path, prefix)
				if rel != "" {
					result[rel] = data
					break
				}
			}
		}
	}
	return result
}

// ReadNextraArchiveFromBytes is a convenience wrapper that reads from
// an in-memory byte slice.
func ReadNextraArchiveFromBytes(data []byte, maxUncompressedSize int64) (*NextraArchive, []Warning, error) {
	return ReadNextraArchive(bytes.NewReader(data), int64(len(data)), maxUncompressedSize)
}
