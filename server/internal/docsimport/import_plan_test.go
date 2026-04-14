package docsimport

import (
	"testing"
)

func TestImportPlan_Validate_RejectsEmptySourceSystem(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs"}},
	}
	warnings := ValidateImportPlan(&plan)
	found := false
	for _, w := range warnings {
		if w.Type == "validation_error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected validation_error warning for empty source system")
	}
}

func TestImportPlan_Validate_RejectsDuplicateCollectionSourceIDs(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "nextra",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs"}},
		Collections: []ImportCollection{
			{SourceID: "col-1", Name: "Guide A", SpaceSourceID: "default"},
			{SourceID: "col-1", Name: "Guide B", SpaceSourceID: "default"},
		},
	}
	warnings := ValidateImportPlan(&plan)
	found := false
	for _, w := range warnings {
		if w.Type == "duplicate_source_id" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected duplicate_source_id warning for collections")
	}
}

func TestImportPlan_Validate_RejectsDuplicateArticleSourceIDs(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "nextra",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs"}},
		Articles: []ImportArticle{
			{SourceID: "art-1", Title: "Intro", SpaceSourceID: "default"},
			{SourceID: "art-1", Title: "Intro Again", SpaceSourceID: "default"},
		},
	}
	warnings := ValidateImportPlan(&plan)
	found := false
	for _, w := range warnings {
		if w.Type == "duplicate_source_id" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected duplicate_source_id warning for articles")
	}
}

func TestImportPlan_Validate_AcceptsWarningsAndHiddenItems(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "nextra",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs"}},
		Collections: []ImportCollection{
			{SourceID: "col-1", Name: "Hidden Col", SpaceSourceID: "default", Hidden: true},
		},
		Articles: []ImportArticle{
			{SourceID: "art-1", Title: "Hidden Art", SpaceSourceID: "default", Hidden: true},
		},
		Warnings: []Warning{
			{Type: "unsupported_mdx_component", Message: "CustomVideo not supported"},
		},
	}
	warnings := ValidateImportPlan(&plan)
	for _, w := range warnings {
		if w.Type == "validation_error" || w.Type == "duplicate_source_id" {
			t.Fatalf("unexpected validation warning: %v", w)
		}
	}
}

func TestImportPlan_Validate_SortsCollectionsByPosition(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "nextra",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs"}},
		Collections: []ImportCollection{
			{SourceID: "col-c", Name: "C", SpaceSourceID: "default", Position: 3},
			{SourceID: "col-a", Name: "A", SpaceSourceID: "default", Position: 1},
			{SourceID: "col-b", Name: "B", SpaceSourceID: "default", Position: 2},
		},
		Articles: []ImportArticle{
			{SourceID: "art-z", Title: "Z", SpaceSourceID: "default", Position: 2},
			{SourceID: "art-a", Title: "A", SpaceSourceID: "default", Position: 1},
		},
	}
	ValidateImportPlan(&plan)

	if plan.Collections[0].SourceID != "col-a" {
		t.Errorf("expected first collection to be col-a, got %s", plan.Collections[0].SourceID)
	}
	if plan.Collections[1].SourceID != "col-b" {
		t.Errorf("expected second collection to be col-b, got %s", plan.Collections[1].SourceID)
	}
	if plan.Collections[2].SourceID != "col-c" {
		t.Errorf("expected third collection to be col-c, got %s", plan.Collections[2].SourceID)
	}
	if plan.Articles[0].SourceID != "art-a" {
		t.Errorf("expected first article to be art-a, got %s", plan.Articles[0].SourceID)
	}
	if plan.Articles[1].SourceID != "art-z" {
		t.Errorf("expected second article to be art-z, got %s", plan.Articles[1].SourceID)
	}
}

func TestImportPlan_Validate_AcceptsValidPlan(t *testing.T) {
	plan := ImportPlan{
		SourceSystem: "nextra",
		SourceCommit: "abc123",
		RootPath:     "content",
		Spaces:       []ImportSpace{{SourceID: "default", Name: "Docs", Slug: "docs"}},
		Collections: []ImportCollection{
			{SourceID: "col-1", Name: "Guides", Slug: "guides", SpaceSourceID: "default", Position: 1},
		},
		Articles: []ImportArticle{
			{SourceID: "art-1", Title: "Getting Started", Slug: "getting-started", SpaceSourceID: "default", CollectionSourceID: "col-1", Position: 1},
		},
		Assets: []ImportAsset{
			{SourcePath: "public/logo.png", ReferencedBy: "art-1", ContentType: "image/png", Size: 1024},
		},
		Redirects: []ImportRedirect{
			{SourceRoute: "/getting-started", TargetArticleSlug: "getting-started"},
		},
	}
	warnings := ValidateImportPlan(&plan)
	for _, w := range warnings {
		if w.Type == "validation_error" {
			t.Fatalf("unexpected validation error: %s", w.Message)
		}
	}
}
