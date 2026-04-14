package docsimport

import (
	"strings"
	"testing"
)

func TestNextraMDX_YAMLFrontmatter(t *testing.T) {
	data := []byte(`---
title: Getting Started
description: Learn how to set up
slug: custom-slug
tags:
  - setup
  - intro
draft: true
---

# Getting Started

Some content here.
`)
	doc, warnings, err := ParseNextraMDX("getting-started.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if doc.Title != "Getting Started" {
		t.Errorf("expected title 'Getting Started', got %q", doc.Title)
	}
	if doc.Description != "Learn how to set up" {
		t.Errorf("expected description 'Learn how to set up', got %q", doc.Description)
	}
	if doc.Slug != "custom-slug" {
		t.Errorf("expected slug 'custom-slug', got %q", doc.Slug)
	}
	if !doc.Draft {
		t.Error("expected draft to be true")
	}
}

func TestNextraMDX_TitleFallbackFromHeading(t *testing.T) {
	data := []byte(`# My Page Title

Some content.
`)
	doc, _, err := ParseNextraMDX("my-page.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "My Page Title" {
		t.Errorf("expected title 'My Page Title', got %q", doc.Title)
	}
}

func TestNextraMDX_SlugFallbackFromFilename(t *testing.T) {
	data := []byte(`# Hello

Content.
`)
	doc, _, err := ParseNextraMDX("my-article.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Slug != "my-article" {
		t.Errorf("expected slug 'my-article', got %q", doc.Slug)
	}
}

func TestNextraMDX_ImportExportLinesRemoved(t *testing.T) {
	data := []byte(`import { Callout } from 'nextra/components'
import CustomComponent from '../components/Custom'
export const meta = { title: 'Test' }

# Hello

Some content.
`)
	doc, _, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(doc.Body, "import") {
		t.Error("import lines should be removed from body")
	}
	if strings.Contains(doc.Body, "export const") {
		t.Error("export lines should be removed from body")
	}
	// The H1 is stripped from body since it becomes the title.
	if !strings.Contains(doc.Body, "Some content") {
		t.Error("body content should remain")
	}
	if doc.Title != "Hello" {
		t.Errorf("expected title 'Hello', got %q", doc.Title)
	}
}

func TestNextraMDX_CalloutWarning(t *testing.T) {
	data := []byte(`# Test

<Callout type="warning">
  This is a warning message.
</Callout>
`)
	doc, _, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Callout should be converted to a blockquote with type marker.
	if !strings.Contains(doc.Body, "warning") {
		t.Error("warning callout content should be preserved")
	}
	if !strings.Contains(doc.Body, "This is a warning message") {
		t.Error("callout body should be preserved")
	}
}

func TestNextraMDX_CalloutInfo(t *testing.T) {
	data := []byte(`# Test

<Callout type="info">
  Information here.
</Callout>
`)
	doc, _, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Body, "Information here") {
		t.Error("info callout body should be preserved")
	}
}

func TestNextraMDX_CalloutDefault(t *testing.T) {
	data := []byte(`# Test

<Callout>
  Default callout.
</Callout>
`)
	doc, _, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Body, "Default callout") {
		t.Error("default callout body should be preserved")
	}
}

func TestNextraMDX_StepsPreservesContent(t *testing.T) {
	data := []byte(`# Setup

<Steps>

### Step 1

Do this first.

### Step 2

Then do this.

</Steps>
`)
	doc, _, err := ParseNextraMDX("setup.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Body, "Step 1") {
		t.Error("Steps inner content should be preserved")
	}
	if !strings.Contains(doc.Body, "Do this first") {
		t.Error("Steps inner text should be preserved")
	}
}

func TestNextraMDX_TabsEmitsFallback(t *testing.T) {
	data := []byte(`# Install

<Tabs items={["npm", "pnpm", "yarn"]}>
  <Tab>
    npm install my-package
  </Tab>
  <Tab>
    pnpm add my-package
  </Tab>
  <Tab>
    yarn add my-package
  </Tab>
</Tabs>
`)
	doc, warnings, err := ParseNextraMDX("install.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Tabs content should be preserved in some form.
	if !strings.Contains(doc.Body, "npm install") {
		t.Error("Tabs content should be preserved")
	}
	_ = warnings
}

func TestNextraMDX_CardsBecomesLinkList(t *testing.T) {
	data := []byte(`# Resources

<Cards>
  <Card title="Guide" href="/guide" />
  <Card title="API" href="/api" />
</Cards>
`)
	doc, _, err := ParseNextraMDX("resources.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.Body, "Guide") {
		t.Error("Card title should be preserved")
	}
}

func TestNextraMDX_UnknownComponentWarning(t *testing.T) {
	data := []byte(`# Test

<CustomVideo src="video.mp4" />

Some text after.
`)
	doc, warnings, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.UnsupportedComponents) == 0 {
		t.Error("expected unsupported component to be listed")
	}
	foundWarning := false
	for _, w := range warnings {
		if w.Type == "unsupported_mdx_component" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected unsupported_mdx_component warning")
	}
	if !strings.Contains(doc.Body, "Some text after") {
		t.Error("content after unknown component should be preserved")
	}
}

func TestNextraMDX_MultipleUnknownComponents(t *testing.T) {
	data := []byte(`# Test

<CustomVideo src="a.mp4" />
<CustomChart data={[1,2,3]} />
<CustomVideo src="b.mp4" />
`)
	doc, _, err := ParseNextraMDX("test.mdx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should list unique component names.
	found := map[string]bool{}
	for _, c := range doc.UnsupportedComponents {
		found[c] = true
	}
	if !found["CustomVideo"] {
		t.Error("expected CustomVideo in unsupported components")
	}
	if !found["CustomChart"] {
		t.Error("expected CustomChart in unsupported components")
	}
}

func TestNextraMDX_NoFrontmatter(t *testing.T) {
	data := []byte(`# Simple Page

Just some content without frontmatter.
`)
	doc, _, err := ParseNextraMDX("simple.md", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Title != "Simple Page" {
		t.Errorf("expected title from heading, got %q", doc.Title)
	}
	if doc.Slug != "simple" {
		t.Errorf("expected slug 'simple', got %q", doc.Slug)
	}
}
