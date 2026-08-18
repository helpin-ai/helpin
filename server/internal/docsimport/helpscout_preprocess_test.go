package docsimport

import (
	"strings"
	"testing"
)

func TestPreprocessHelpScoutHTML_NormalizesEscapedAsideBlocks(t *testing.T) {
	normalized, warnings := PreprocessHelpScoutHTML(readFixture(t, "replug_alt_text.html"))
	if strings.Contains(normalized, "&lt;aside&gt;") || strings.Contains(normalized, "&lt;/aside&gt;") {
		t.Fatalf("expected escaped aside wrappers removed, got: %s", normalized)
	}
	if !strings.Contains(normalized, `class="callout`) && !strings.Contains(normalized, `class='callout`) {
		t.Fatalf("expected normalized output to include a callout wrapper, got: %s", normalized)
	}
	if len(warnings) == 0 {
		t.Fatal("expected preprocessing warnings for normalized help scout note blocks")
	}
}

func TestPreprocessHelpScoutHTML_RemovesEmptyHeadingsAndWhitespaceNoise(t *testing.T) {
	normalized, _ := PreprocessHelpScoutHTML(readFixture(t, "replug_first_comment.html"))
	if strings.Contains(normalized, "<h2></h2>") {
		t.Fatalf("expected empty headings removed, got: %s", normalized)
	}
	if strings.Contains(normalized, "<p> The ") || strings.Contains(normalized, "<p> Go to the") {
		t.Fatalf("expected leading import whitespace trimmed, got: %s", normalized)
	}
}

func TestPreprocessHelpScoutHTML_NormalizesFakeStepHeadings(t *testing.T) {
	raw := `<h4><p style="display: inline-block; background:#3988fe;">1</p><p style="display:inline-block"> Log into your account </p></h4>`
	normalized, _ := PreprocessHelpScoutHTML(raw)
	if !strings.Contains(normalized, "<h4>1. Log into your account</h4>") {
		t.Fatalf("expected fake step heading text preserved as a clean heading, got: %s", normalized)
	}
}

func TestPreprocessHelpScoutHTML_NormalizesFakeStepHeadingsWithWhitespaceChildren(t *testing.T) {
	raw := "<h4>\n  <p style=\"font-weight: normal; display: inline-block; background:#3988fe;\">  3</p>\n  <p style=\"display:inline-block;\">  New Brand</p>\n</h4>"
	normalized, _ := PreprocessHelpScoutHTML(raw)
	if !strings.Contains(normalized, "<h4>3. New Brand</h4>") {
		t.Fatalf("expected whitespace-padded fake step heading preserved as a clean heading, got: %s", normalized)
	}
}

func TestPreprocessHelpScoutHTML_NormalizesStepParagraphsToHeadings(t *testing.T) {
	raw := `<p>Step 3 Save Your Campaign</p><p>After adding the pixels, click on save.</p>`
	normalized, _ := PreprocessHelpScoutHTML(raw)
	if !strings.Contains(normalized, "<h4>Step 3 Save Your Campaign</h4>") {
		t.Fatalf("expected step paragraph to be promoted to a heading, got: %s", normalized)
	}
	if strings.Contains(normalized, "<p>Step 3 Save Your Campaign</p>") {
		t.Fatalf("expected original step paragraph to be replaced, got: %s", normalized)
	}
}

func TestPreprocessHelpScoutHTML_UnwrapsRedundantHeadingBoldAndPunctuationFormatting(t *testing.T) {
	raw := `<p>In <a href="https://replug.io">Replug</a><strong>, </strong>the archive function.</p><h4><span class="step">Step 3</span> <strong>Create a Campaign</strong></h4>`
	normalized, _ := PreprocessHelpScoutHTML(raw)
	if strings.Contains(normalized, "<strong>,") {
		t.Fatalf("expected punctuation-only strong formatting to be unwrapped, got: %s", normalized)
	}
	if strings.Contains(normalized, "<h4><span class=\"step\">Step 3</span> <strong>Create a Campaign</strong></h4>") {
		t.Fatalf("expected redundant bold inside heading to be unwrapped, got: %s", normalized)
	}
	if !strings.Contains(normalized, "<h4>Step 3 Create a Campaign</h4>") {
		t.Fatalf("expected heading text to be preserved without nested strong, got: %s", normalized)
	}
}

func TestPreprocessHelpScoutHTML_PreservesInlineBoundarySpaceInsideHeadingSpans(t *testing.T) {
	raw := `<h4 class="step-main-head"><span class="step-head"><span class="step">Step 1 </span>Create a Campaign</span></h4>`
	normalized, _ := PreprocessHelpScoutHTML(raw)
	if !strings.Contains(normalized, "<span class=\"step\">Step 1 </span>Create a Campaign") {
		t.Fatalf("expected meaningful inline boundary space inside heading span to be preserved, got: %s", normalized)
	}
}

func TestContainsGIFImage(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{name: "gif source", html: `<img src="https://assets.example.com/demo.gif">`, want: true},
		{name: "uppercase extension and query", html: `<img src="https://assets.example.com/demo.GIF?version=2">`, want: true},
		{name: "lazy gif", html: `<img src="placeholder.png" data-src="https://assets.example.com/demo.gif">`, want: true},
		{name: "data gif", html: `<img src="data:image/gif;base64,R0lGODlh">`, want: true},
		{name: "gif text is not an image", html: `<p>Upload a .gif file</p>`, want: false},
		{name: "png image", html: `<img src="https://assets.example.com/demo.png">`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsGIFImage(tt.html); got != tt.want {
				t.Fatalf("ContainsGIFImage() = %v, want %v", got, tt.want)
			}
		})
	}
}
