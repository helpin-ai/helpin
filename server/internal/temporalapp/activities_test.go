package temporalapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidatePlanningProposalStoriesRejectsCycle(t *testing.T) {
	stories := []model.ProposedStory{
		{Ref: "story_a", Name: "Story A", DependencyRefs: []string{"story_b"}},
		{Ref: "story_b", Name: "Story B", DependencyRefs: []string{"story_a"}},
	}

	err := validatePlanningProposalStories(stories)
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestSelectRelevantPlanningFilesPrefersRelevantPaths(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                             "module example.com/test\n",
		"server/internal/handler/billing.go": "package handler\n",
		"server/internal/service/billing_service.go": "package service\n",
		"server/internal/model/invoice.go":           "package model\n",
		"web/src/pages/BillingPage.tsx":              "export const BillingPage = () => null\n",
		"docs/notes.md":                              "billing settings\n",
	}
	for name, content := range files {
		fullPath := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	paths, err := selectRelevantPlanningFiles(root, "We need to improve billing invoices and billing settings.")
	if err != nil {
		t.Fatalf("selectRelevantPlanningFiles: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected relevant planning files")
	}
	found := false
	for _, path := range paths {
		if path == "server/internal/service/billing_service.go" || path == "server/internal/handler/billing.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected billing-related backend file in result, got %#v", paths)
	}
}

func TestCollectPlanningTreeBoundsDepth(t *testing.T) {
	root := t.TempDir()
	deepPath := filepath.Join(root, "server", "internal", "service", "payments", "v2", "handler.go")
	if err := os.MkdirAll(filepath.Dir(deepPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(deepPath, []byte("package service\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tree, err := collectPlanningTree(root, 3, 20)
	if err != nil {
		t.Fatalf("collectPlanningTree: %v", err)
	}
	for _, item := range tree {
		if item == "        server/internal/service/payments/v2/handler.go" {
			t.Fatalf("expected deep file to be truncated by depth, got %#v", tree)
		}
	}
}

func TestValidatePlanningProposalStoriesNormalizesMissingRefs(t *testing.T) {
	stories := []model.ProposedStory{
		{Name: "Story A"},
		{Name: "Story B", DependencyRefs: []string{"story_1"}},
	}

	if err := validatePlanningProposalStories(stories); err != nil {
		t.Fatalf("validatePlanningProposalStories returned error: %v", err)
	}
	if stories[0].Ref != "story_1" {
		t.Fatalf("expected first story ref to default to story_1, got %q", stories[0].Ref)
	}
	if stories[1].Ref != "story_2" {
		t.Fatalf("expected second story ref to default to story_2, got %q", stories[1].Ref)
	}
}

func TestMarkdownToDocsJSONPreservesHeadingsAndBullets(t *testing.T) {
	raw := markdownToDocsJSON("# Problem\n\n- first item\n- second item\n\nPlain paragraph")

	var doc struct {
		Type    string                   `json:"type"`
		Content []map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal docs json: %v", err)
	}
	if doc.Type != "doc" {
		t.Fatalf("expected root type doc, got %q", doc.Type)
	}
	if len(doc.Content) < 3 {
		t.Fatalf("expected heading, list, and paragraph nodes, got %d nodes", len(doc.Content))
	}
	if doc.Content[0]["type"] != "heading" {
		t.Fatalf("expected first node to be heading, got %#v", doc.Content[0]["type"])
	}
	if doc.Content[1]["type"] != "bulletList" {
		t.Fatalf("expected second node to be bulletList, got %#v", doc.Content[1]["type"])
	}
	if doc.Content[2]["type"] != "paragraph" {
		t.Fatalf("expected third node to be paragraph, got %#v", doc.Content[2]["type"])
	}
}

func TestRenderProductSpecMarkdownAppendsNormalizedResearchSources(t *testing.T) {
	rendered := renderProductSpecMarkdown("# Problem\n\nBase spec", []model.PlanningResearchSource{
		{Title: "NIST", URL: "https://example.com/nist", Note: "security baseline"},
		{Title: "NIST duplicate", URL: "https://example.com/nist"},
		{Title: "WCAG", URL: "https://example.com/wcag", PublishedAt: "2025-01-01"},
	})

	if !strings.Contains(rendered, "## Research Sources") {
		t.Fatalf("expected research sources section, got %q", rendered)
	}
	if strings.Count(rendered, "https://example.com/nist") != 1 {
		t.Fatalf("expected duplicate source URLs to be de-duplicated, got %q", rendered)
	}
	if !strings.Contains(rendered, "published 2025-01-01") {
		t.Fatalf("expected published date note in rendered markdown, got %q", rendered)
	}
}
