package docsimport

import (
	"testing"
)

func TestPlanNextraAssets_RelativeImage(t *testing.T) {
	files := map[string][]byte{
		"getting-started.mdx": []byte(`# Hello\n![Alt](./img.png)`),
		"img.png":             []byte("PNG-DATA"),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "getting-started.mdx",
		RawContent: "![Alt](./img.png)",
	}
	assets, _, warnings := PlanNextraAssets(files, nil, article)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].SourcePath != "img.png" {
		t.Errorf("expected source path 'img.png', got %q", assets[0].SourcePath)
	}
}

func TestPlanNextraAssets_PublicImage(t *testing.T) {
	files := map[string][]byte{
		"getting-started.mdx": []byte(`# Hello`),
	}
	publicFiles := map[string][]byte{
		"logo.png": []byte("PNG-DATA"),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "getting-started.mdx",
		RawContent: "![Logo](/logo.png)",
	}
	assets, _, warnings := PlanNextraAssets(files, publicFiles, article)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
}

func TestPlanNextraAssets_HTMLImg(t *testing.T) {
	files := map[string][]byte{
		"page.mdx":  []byte(`<img src="./hero.png" />`),
		"hero.png":  []byte("PNG"),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "page.mdx",
		RawContent: `<img src="./hero.png" />`,
	}
	assets, _, warnings := PlanNextraAssets(files, nil, article)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
}

func TestPlanNextraAssets_MissingAssetWarning(t *testing.T) {
	files := map[string][]byte{
		"page.mdx": []byte(`![Alt](./missing.png)`),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "page.mdx",
		RawContent: "![Alt](./missing.png)",
	}
	_, _, warnings := PlanNextraAssets(files, nil, article)
	found := false
	for _, w := range warnings {
		if w.Type == "missing_asset" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing_asset warning")
	}
}

func TestPlanNextraAssets_RemoteURLKept(t *testing.T) {
	files := map[string][]byte{
		"page.mdx": []byte(`![Alt](https://example.com/img.png)`),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "page.mdx",
		RawContent: "![Alt](https://example.com/img.png)",
	}
	assets, _, _ := PlanNextraAssets(files, nil, article)
	// Remote images are not local assets.
	if len(assets) != 0 {
		t.Errorf("expected 0 local assets for remote URL, got %d", len(assets))
	}
}

func TestPlanNextraAssets_NestedRelative(t *testing.T) {
	files := map[string][]byte{
		"guides/setup.mdx":  []byte(`![Alt](./images/hero.png)`),
		"guides/images/hero.png": []byte("PNG"),
	}
	article := ImportArticle{
		SourceID:   "art-1",
		SourcePath: "guides/setup.mdx",
		RawContent: "![Alt](./images/hero.png)",
	}
	assets, _, warnings := PlanNextraAssets(files, nil, article)
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].SourcePath != "guides/images/hero.png" {
		t.Errorf("expected 'guides/images/hero.png', got %q", assets[0].SourcePath)
	}
}
