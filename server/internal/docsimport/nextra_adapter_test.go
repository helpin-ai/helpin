package docsimport

import (
	"bytes"
	"context"
	"testing"
)

func TestNextraAdapter_BasicPreview(t *testing.T) {
	files := map[string][]byte{
		"package.json": []byte(`{"name":"my-docs"}`),
		"content/_meta.json": []byte(`{
			"index": "Introduction",
			"getting-started": "Getting Started",
			"guides": { "title": "Guides", "type": "page" }
		}`),
		"content/index.mdx": []byte(`---
title: Welcome
description: Welcome to the docs
---

# Welcome

This is the home page.
`),
		"content/getting-started.mdx": []byte(`---
title: Getting Started
---

# Getting Started

Follow these steps.
`),
		"content/guides/_meta.json": []byte(`{
			"install": "Installation",
			"config": "Configuration"
		}`),
		"content/guides/install.mdx": []byte(`# Installation

Run the installer.

![Logo](/logo.png)
`),
		"content/guides/config.mdx": []byte(`# Configuration

<Callout type="warning">
  Be careful with config.
</Callout>

Configure your settings.
`),
		"public/logo.png": []byte("PNG-DATA"),
	}

	data := makeZip(t, files)
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	adapter := NextraAdapter{}
	plan, warnings, err := adapter.Preview(context.Background(), archive, NextraPreviewOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	// Check plan structure.
	if plan.SourceSystem != "nextra" {
		t.Errorf("expected source system 'nextra', got %q", plan.SourceSystem)
	}

	// Should have at least one space.
	if len(plan.Spaces) == 0 {
		t.Fatal("expected at least one space")
	}

	// Should have collections matching folder tree.
	if len(plan.Collections) == 0 {
		t.Fatal("expected collections")
	}
	foundGuides := false
	for _, c := range plan.Collections {
		if c.Name == "Guides" || c.Slug == "guides" {
			foundGuides = true
			break
		}
	}
	if !foundGuides {
		t.Error("expected a 'Guides' collection")
	}

	// Should have articles.
	if len(plan.Articles) < 3 {
		t.Errorf("expected at least 3 articles, got %d", len(plan.Articles))
	}

	// Check article source routes.
	routeFound := map[string]bool{}
	for _, a := range plan.Articles {
		routeFound[a.SourceRoute] = true
	}
	if !routeFound["/"] {
		t.Error("expected article with source route '/'")
	}
	if !routeFound["/getting-started"] {
		t.Error("expected article with source route '/getting-started'")
	}

	// Should have redirects planned.
	if len(plan.Redirects) == 0 {
		t.Error("expected redirects")
	}

	// Should have assets.
	if len(plan.Assets) == 0 {
		t.Error("expected at least one asset (logo.png)")
	}

	// Check warnings are reasonable (not errors).
	for _, w := range warnings {
		if w.Type == "validation_error" {
			t.Errorf("unexpected validation error: %s", w.Message)
		}
	}
}

func TestNextraAdapter_HiddenPages(t *testing.T) {
	files := map[string][]byte{
		"content/_meta.json": []byte(`{
			"index": "Home",
			"hidden": { "title": "Secret", "display": "hidden" }
		}`),
		"content/index.mdx":  []byte("# Home\nWelcome."),
		"content/hidden.mdx": []byte("# Secret\nThis is hidden."),
	}

	data := makeZip(t, files)
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	adapter := NextraAdapter{}
	plan, _, err := adapter.Preview(context.Background(), archive, NextraPreviewOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	hiddenFound := false
	for _, a := range plan.Articles {
		if a.Slug == "hidden" {
			if !a.Hidden {
				t.Error("expected hidden article to be marked hidden")
			}
			hiddenFound = true
		}
	}
	if !hiddenFound {
		t.Error("expected to find the hidden article")
	}
}

func TestNextraAdapter_PositionsFromMeta(t *testing.T) {
	files := map[string][]byte{
		"content/_meta.json": []byte(`{
			"zebra": "Zebra Page",
			"alpha": "Alpha Page",
			"middle": "Middle Page"
		}`),
		"content/zebra.mdx":  []byte("# Zebra\nContent."),
		"content/alpha.mdx":  []byte("# Alpha\nContent."),
		"content/middle.mdx": []byte("# Middle\nContent."),
	}

	data := makeZip(t, files)
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	adapter := NextraAdapter{}
	plan, _, err := adapter.Preview(context.Background(), archive, NextraPreviewOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	// Articles should be ordered by _meta position.
	if len(plan.Articles) < 3 {
		t.Fatalf("expected at least 3 articles, got %d", len(plan.Articles))
	}
	if plan.Articles[0].Slug != "zebra" {
		t.Errorf("expected first article to be 'zebra', got %q", plan.Articles[0].Slug)
	}
	if plan.Articles[1].Slug != "alpha" {
		t.Errorf("expected second article to be 'alpha', got %q", plan.Articles[1].Slug)
	}
	if plan.Articles[2].Slug != "middle" {
		t.Errorf("expected third article to be 'middle', got %q", plan.Articles[2].Slug)
	}
}

func TestNextraAdapter_UnsupportedComponentsInWarnings(t *testing.T) {
	files := map[string][]byte{
		"content/_meta.json": []byte(`{"index": "Home"}`),
		"content/index.mdx": []byte(`# Home

<CustomVideo src="demo.mp4" />

Some text.
`),
	}

	data := makeZip(t, files)
	archive, _, err := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	adapter := NextraAdapter{}
	plan, warnings, err := adapter.Preview(context.Background(), archive, NextraPreviewOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	// Plan warnings should include unsupported components.
	allWarnings := append(plan.Warnings, warnings...)
	foundUnsupported := false
	for _, w := range allWarnings {
		if w.Type == "unsupported_mdx_component" {
			foundUnsupported = true
			break
		}
	}
	if !foundUnsupported {
		t.Error("expected unsupported_mdx_component warning")
	}

	// Article should list the unsupported component.
	if len(plan.Articles) > 0 && len(plan.Articles[0].UnsupportedComponents) == 0 {
		t.Error("expected article to list unsupported components")
	}
}

func TestNextraAdapter_SourceCommit(t *testing.T) {
	files := map[string][]byte{
		"content/_meta.json": []byte(`{"index":"Home"}`),
		"content/index.mdx":  []byte("# Home"),
	}
	data := makeZip(t, files)
	archive, _, _ := ReadNextraArchive(bytes.NewReader(data), int64(len(data)), 500*1024*1024)

	adapter := NextraAdapter{}
	plan, _, err := adapter.Preview(context.Background(), archive, NextraPreviewOptions{
		SourceCommit: "abc123",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if plan.SourceCommit != "abc123" {
		t.Errorf("expected source commit 'abc123', got %q", plan.SourceCommit)
	}
}
