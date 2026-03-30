package docsi18n

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func TestPrepareArticleTranslationPlan_PreservesStructureAndMarks(t *testing.T) {
	source := ArticleSource{
		SourceLocale: "en",
		TargetLocale: "fr",
		Title:        "Getting started with Helpin",
		Excerpt:      testStringPtr("Open Settings to connect your workspace."),
		SEOTitle:     testStringPtr("Helpin getting started"),
		SEODescription: testStringPtr("Learn how to configure Helpin."),
		Content: sampleDoc(),
	}

	plan, err := PrepareArticleTranslationPlan(source, PrepareOptions{
		ProtectedTerms: []string{"Helpin", "SLA"},
	})
	if err != nil {
		t.Fatalf("PrepareArticleTranslationPlan: %v", err)
	}

	if len(plan.Request.Segments) == 0 {
		t.Fatal("expected extracted segments")
	}

	response := TranslationResponse{
		Segments: []TranslatedSegment{
			{ID: "meta:title", TranslatedText: "Premiers pas avec __TERM_001__"},
			{ID: "meta:excerpt", TranslatedText: "Ouvrez Parametres pour connecter votre espace de travail."},
			{ID: "meta:seo_title", TranslatedText: "Premiers pas __TERM_001__"},
			{ID: "meta:seo_description", TranslatedText: "Apprenez a configurer __TERM_001__."},
			{ID: "doc/0", TranslatedText: "Bienvenue"},
			{ID: "doc/1", TranslatedText: "Open "},
			{ID: "doc/2", TranslatedText: "Settings"},
			{ID: "doc/3", TranslatedText: " pour connecter __TERM_001__."},
			{ID: "doc/4", TranslatedText: "Important"},
			{ID: "doc/5", TranslatedText: "Consultez le __TERM_002__ avant le deploiement."},
			{ID: "doc/6", TranslatedText: "Step"},
			{ID: "doc/7", TranslatedText: "Open dashboard"},
			{ID: "doc/8", TranslatedText: "Name"},
			{ID: "doc/9", TranslatedText: "Value"},
			{ID: "doc/10", TranslatedText: "Plan"},
			{ID: "doc/11", TranslatedText: "Growth"},
			{ID: "doc/12", TranslatedText: "Diagram"},
			{ID: "doc/13", TranslatedText: "Product diagram"},
		},
	}

	draft, err := plan.Apply(response)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if draft.Title != "Premiers pas avec Helpin" {
		t.Fatalf("draft title = %q", draft.Title)
	}
	if draft.Content.Type != "doc" {
		t.Fatalf("draft content type = %q", draft.Content.Type)
	}

	firstParagraph := draft.Content.Content[1]
	if firstParagraph.Type != "paragraph" {
		t.Fatalf("first paragraph type = %q", firstParagraph.Type)
	}
	if len(firstParagraph.Content) != 3 {
		t.Fatalf("paragraph text-run count = %d, want 3", len(firstParagraph.Content))
	}
	if got := firstParagraph.Content[1].Marks[0].Type; got != "bold" {
		t.Fatalf("bold mark lost, got %q", got)
	}
	if got := firstParagraph.Content[2].Text; !strings.Contains(got, "Helpin") {
		t.Fatalf("protected term not preserved, got %q", got)
	}

	if draft.Content.Content[2].Type != "callout" {
		t.Fatalf("callout node lost, got %q", draft.Content.Content[2].Type)
	}
	if draft.Content.Content[3].Type != "bulletList" {
		t.Fatalf("bullet list node lost, got %q", draft.Content.Content[3].Type)
	}
	if draft.Content.Content[4].Type != "table" {
		t.Fatalf("table node lost, got %q", draft.Content.Content[4].Type)
	}
	if draft.Content.Content[5].Type != "resizableImage" {
		t.Fatalf("image node lost, got %q", draft.Content.Content[5].Type)
	}
	if draft.Content.Content[6].Type != "codeBlock" {
		t.Fatalf("code block node lost, got %q", draft.Content.Content[6].Type)
	}
	if draft.Content.Content[7].Type != "htmlBlock" {
		t.Fatalf("htmlBlock node lost, got %q", draft.Content.Content[7].Type)
	}
	if draft.Content.Content[8].Type != "videoEmbed" {
		t.Fatalf("video node lost, got %q", draft.Content.Content[8].Type)
	}
}

func TestPrepareArticleTranslationPlan_FailsClosedOnUnsafeResponses(t *testing.T) {
	source := ArticleSource{
		SourceLocale: "en",
		TargetLocale: "fr",
		Title:        "Safe docs",
		Content:      sampleDoc(),
	}

	plan, err := PrepareArticleTranslationPlan(source, PrepareOptions{})
	if err != nil {
		t.Fatalf("PrepareArticleTranslationPlan: %v", err)
	}

	_, err = plan.Apply(TranslationResponse{
		Segments: []TranslatedSegment{
			{ID: "meta:title", TranslatedText: "Docs sures"},
		},
	})
	if err == nil {
		t.Fatal("expected missing segment response to fail")
	}
}

func TestPrepareArticleTranslationPlan_LeavesPreserveOnlyNodesUnchanged(t *testing.T) {
	source := ArticleSource{
		SourceLocale: "en",
		TargetLocale: "de",
		Title:        "Preserve special nodes",
		Content:      sampleDoc(),
	}

	plan, err := PrepareArticleTranslationPlan(source, PrepareOptions{})
	if err != nil {
		t.Fatalf("PrepareArticleTranslationPlan: %v", err)
	}

	response := TranslationResponse{Segments: []TranslatedSegment{
		{ID: "meta:title", TranslatedText: "Spezielle Knoten erhalten"},
		{ID: "doc/0", TranslatedText: "Willkommen"},
		{ID: "doc/1", TranslatedText: "Open "},
		{ID: "doc/2", TranslatedText: "Settings"},
		{ID: "doc/3", TranslatedText: " um __TERM_001__ zu verbinden."},
		{ID: "doc/4", TranslatedText: "Wichtig"},
		{ID: "doc/5", TranslatedText: "Prufen Sie die __TERM_002__ vor dem Rollout."},
		{ID: "doc/6", TranslatedText: "Schritt"},
		{ID: "doc/7", TranslatedText: "Dashboard offnen"},
		{ID: "doc/8", TranslatedText: "Name"},
		{ID: "doc/9", TranslatedText: "Wert"},
		{ID: "doc/10", TranslatedText: "Plan"},
		{ID: "doc/11", TranslatedText: "Growth"},
		{ID: "doc/12", TranslatedText: "Diagramm"},
		{ID: "doc/13", TranslatedText: "Produktdiagramm"},
	}}

	draft, err := plan.Apply(response)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := draft.Content.Content[7].Attrs["html"]; got != "<div><strong>Raw HTML</strong> should stay intact.</div>" {
		t.Fatalf("htmlBlock html changed: %#v", got)
	}
	if got := draft.Content.Content[8].Attrs["embedUrl"]; got != "https://www.youtube.com/embed/demo" {
		t.Fatalf("video embed url changed: %#v", got)
	}
	if got := draft.Content.Content[6].Content[0].Text; got != "npm install helpin" {
		t.Fatalf("code block changed: %q", got)
	}
}

func TestPrepareArticleTranslationPlan_ProtectedTermsMustRoundTrip(t *testing.T) {
	source := ArticleSource{
		SourceLocale: "en",
		TargetLocale: "fr",
		Title:        "Helpin and SLA guide",
		Content: tiptap.Node{
			Type: "doc",
			Content: []tiptap.Node{
				{
					Type: "paragraph",
					Content: []tiptap.Node{
						{Type: "text", Text: "Helpin requires the SLA."},
					},
				},
			},
		},
	}

	plan, err := PrepareArticleTranslationPlan(source, PrepareOptions{
		ProtectedTerms: []string{"Helpin", "SLA"},
	})
	if err != nil {
		t.Fatalf("PrepareArticleTranslationPlan: %v", err)
	}

	_, err = plan.Apply(TranslationResponse{
		Segments: []TranslatedSegment{
			{ID: "meta:title", TranslatedText: "Guide Helpin et SLA"},
			{ID: "doc/0", TranslatedText: "Helpin requiert le SLA."},
		},
	})
	if err == nil {
		t.Fatal("expected protected-term loss to fail")
	}
}

func TestPrepareArticleTranslationPlan_PreservesLinkMarksAndCodeRuns(t *testing.T) {
	source := ArticleSource{
		SourceLocale: "en",
		TargetLocale: "fr",
		Title:        "Link and code",
		Content: tiptap.Node{
			Type: "doc",
			Content: []tiptap.Node{
				{
					Type: "paragraph",
					Content: []tiptap.Node{
						{Type: "text", Text: "Visit "},
						{Type: "text", Text: "dashboard", Marks: []tiptap.Mark{{Type: "link", Attrs: map[string]any{"href": "https://helpin.ai/dashboard", "target": "_blank", "rel": "noopener noreferrer"}}}},
						{Type: "text", Text: " and run "},
						{Type: "text", Text: "npm install helpin", Marks: []tiptap.Mark{{Type: "code"}}},
						{Type: "text", Text: "."},
					},
				},
			},
		},
	}

	plan, err := PrepareArticleTranslationPlan(source, PrepareOptions{})
	if err != nil {
		t.Fatalf("PrepareArticleTranslationPlan: %v", err)
	}

	draft, err := plan.Apply(TranslationResponse{
		Segments: []TranslatedSegment{
			{ID: "meta:title", TranslatedText: "Lien et code"},
			{ID: "doc/0", TranslatedText: "Visitez "},
			{ID: "doc/1", TranslatedText: "tableau de bord"},
			{ID: "doc/2", TranslatedText: " et executez "},
			{ID: "doc/3", TranslatedText: "."},
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	paragraph := draft.Content.Content[0]
	if href := paragraph.Content[1].Marks[0].Attrs["href"]; href != "https://helpin.ai/dashboard" {
		t.Fatalf("link href changed: %#v", href)
	}
	if codeText := paragraph.Content[3].Text; codeText != "npm install helpin" {
		t.Fatalf("code-mark text changed: %q", codeText)
	}
}

func sampleDoc() tiptap.Node {
	return tiptap.Node{
		Type: "doc",
		Content: []tiptap.Node{
			{
				Type: "heading",
				Attrs: map[string]any{"level": 2},
				Content: []tiptap.Node{
					{Type: "text", Text: "Welcome"},
				},
			},
			{
				Type: "paragraph",
				Content: []tiptap.Node{
					{Type: "text", Text: "Open "},
					{Type: "text", Text: "Settings", Marks: []tiptap.Mark{{Type: "bold"}}},
					{Type: "text", Text: " to connect Helpin."},
				},
			},
			{
				Type: "callout",
				Attrs: map[string]any{"variant": "blue"},
				Content: []tiptap.Node{
					{
						Type: "paragraph",
						Content: []tiptap.Node{
							{Type: "text", Text: "Important"},
						},
					},
					{
						Type: "paragraph",
						Content: []tiptap.Node{
							{Type: "text", Text: "Review the SLA before rollout."},
						},
					},
				},
			},
			{
				Type: "bulletList",
				Content: []tiptap.Node{
					{
						Type: "listItem",
						Content: []tiptap.Node{
							{
								Type: "paragraph",
								Content: []tiptap.Node{
									{Type: "text", Text: "Step"},
								},
							},
							{
								Type: "paragraph",
								Content: []tiptap.Node{
									{Type: "text", Text: "Open dashboard"},
								},
							},
						},
					},
				},
			},
			{
				Type: "table",
				Content: []tiptap.Node{
					{
						Type: "tableRow",
						Content: []tiptap.Node{
							{
								Type: "tableHeader",
								Content: []tiptap.Node{
									{
										Type: "paragraph",
										Content: []tiptap.Node{{Type: "text", Text: "Name"}},
									},
								},
							},
							{
								Type: "tableHeader",
								Content: []tiptap.Node{
									{
										Type: "paragraph",
										Content: []tiptap.Node{{Type: "text", Text: "Value"}},
									},
								},
							},
						},
					},
					{
						Type: "tableRow",
						Content: []tiptap.Node{
							{
								Type: "tableCell",
								Content: []tiptap.Node{
									{
										Type: "paragraph",
										Content: []tiptap.Node{{Type: "text", Text: "Plan"}},
									},
								},
							},
							{
								Type: "tableCell",
								Content: []tiptap.Node{
									{
										Type: "paragraph",
										Content: []tiptap.Node{{Type: "text", Text: "Growth"}},
									},
								},
							},
						},
					},
				},
			},
			{
				Type: "resizableImage",
				Attrs: map[string]any{
					"src":        "https://cdn.example.com/diagram.png",
					"alt":        "Diagram",
					"title":      "Product diagram",
					"width":      "60%",
					"height":     "auto",
					"alignment":  "center",
					"attachmentId": "att-1",
				},
			},
			{
				Type: "codeBlock",
				Attrs: map[string]any{"language": "bash"},
				Content: []tiptap.Node{
					{Type: "text", Text: "npm install helpin"},
				},
			},
			{
				Type: "htmlBlock",
				Attrs: map[string]any{"html": "<div><strong>Raw HTML</strong> should stay intact.</div>"},
			},
			{
				Type: "videoEmbed",
				Attrs: map[string]any{
					"provider":  "youtube",
					"sourceUrl": "https://www.youtube.com/watch?v=demo",
					"embedUrl":  "https://www.youtube.com/embed/demo",
				},
			},
		},
	}
}

func testStringPtr(value string) *string {
	return &value
}
