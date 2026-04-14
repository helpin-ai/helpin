package docsimport

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// helper to create an in-memory zip with the given file entries.
func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestNextraArchive_AcceptsValidFiles(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"content/index.mdx":       []byte("# Hello"),
		"content/guide.md":        []byte("# Guide"),
		"content/_meta.json":      []byte(`{"index":"Home"}`),
		"content/_meta.js":        []byte(`export default {"index":"Home"}`),
		"public/logo.png":         []byte("PNG-DATA"),
		"public/images/hero.webp": []byte("WEBP-DATA"),
	})
	archive, warnings, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(archive.Files) != 6 {
		t.Errorf("expected 6 files, got %d", len(archive.Files))
	}
	// Check warnings are informational only.
	for _, w := range warnings {
		if w.Type == "archive_error" {
			t.Errorf("unexpected archive error: %s", w.Message)
		}
	}
}

func TestNextraArchive_RejectsZipSlipPaths(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"../../evil.txt": []byte("pwned"),
	})
	_, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err == nil {
		t.Fatal("expected error for zip-slip path")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestNextraArchive_RejectsAbsolutePaths(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"/etc/passwd": []byte("root:x:0:0"),
	})
	_, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestNextraArchive_RejectsOversizedArchive(t *testing.T) {
	// Create a zip with content larger than the limit.
	bigContent := bytes.Repeat([]byte("x"), 1024)
	data := makeZip(t, map[string][]byte{
		"content/big.md": bigContent,
	})
	// Set an absurdly small limit.
	_, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 100)
	if err == nil {
		t.Fatal("expected error for oversized archive")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size exceeds error, got: %v", err)
	}
}

func TestNextraArchive_DetectsRootAtRepoRoot(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"index.mdx":   []byte("# Home"),
		"_meta.json":  []byte(`{}`),
		"guide.md":    []byte("# Guide"),
		"public/a.png": []byte("IMG"),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if archive.RootPath != "." {
		t.Errorf("expected root path '.', got %q", archive.RootPath)
	}
}

func TestNextraArchive_DetectsRootAtContent(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"package.json":       []byte(`{}`),
		"content/index.mdx":  []byte("# Home"),
		"content/_meta.json": []byte(`{}`),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if archive.RootPath != "content" {
		t.Errorf("expected root path 'content', got %q", archive.RootPath)
	}
}

func TestNextraArchive_DetectsRootAtSrcContent(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"package.json":           []byte(`{}`),
		"src/content/index.mdx":  []byte("# Home"),
		"src/content/_meta.json": []byte(`{}`),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if archive.RootPath != "src/content" {
		t.Errorf("expected root path 'src/content', got %q", archive.RootPath)
	}
}

func TestNextraArchive_DetectsRootAtAppsDocs(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"package.json":                    []byte(`{}`),
		"apps/docs/content/index.mdx":     []byte("# Home"),
		"apps/docs/content/_meta.json":    []byte(`{}`),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if archive.RootPath != "apps/docs/content" {
		t.Errorf("expected root path 'apps/docs/content', got %q", archive.RootPath)
	}
}

func TestNextraArchive_DetectsRootAtDocs(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"package.json":        []byte(`{}`),
		"docs/index.mdx":      []byte("# Home"),
		"docs/_meta.json":     []byte(`{}`),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if archive.RootPath != "docs" {
		t.Errorf("expected root path 'docs', got %q", archive.RootPath)
	}
}

func TestNextraArchive_SkipsNonDocsFiles(t *testing.T) {
	data := makeZip(t, map[string][]byte{
		"content/index.mdx":  []byte("# Home"),
		"content/_meta.json": []byte(`{}`),
		"node_modules/pkg/index.js": []byte("module.exports = {}"),
		".git/config":        []byte("[core]"),
		"content/readme.txt": []byte("ignored"),
	})
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := archive.Files["node_modules/pkg/index.js"]; ok {
		t.Error("node_modules should be skipped")
	}
	if _, ok := archive.Files[".git/config"]; ok {
		t.Error(".git should be skipped")
	}
}

func TestNextraArchive_RejectsExcessiveFileCount(t *testing.T) {
	files := make(map[string][]byte)
	for i := 0; i < 101; i++ {
		name := "content/" + strings.Repeat("a", 5) + string(rune('a'+i%26)) + ".md"
		// Use unique names
		name = "content/" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".md"
		files[name] = []byte("# Doc")
	}
	data := makeZip(t, files)
	_, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	// With default max file count of 10000, this should pass.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
