package linkpreview

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

var previewURLs = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>]+`)

// Normalize bounds every externally displayed value and keeps descriptions
// independent of image properties. Call before hashing, HTML, or PNG rendering.
func Normalize(card model.LinkPreview) model.LinkPreview {
	card.Title = plainValue(card.Title, 180)
	card.Type = plainValue(card.Type, 40)
	card.Reference = plainValue(card.Reference, 40)
	card.Description = description(card.Description)
	if card.Title == "" {
		card.Title = "Helpin"
	}
	if card.Description == "" {
		card.Description = FallbackDescription
	}
	fields := make([]model.LinkPreviewField, 0, 3)
	for _, field := range card.Fields {
		field.Label = plainValue(field.Label, 24)
		field.Value = plainValue(field.Value, 100)
		if field.Label != "" && field.Value != "" {
			fields = append(fields, field)
		}
		if len(fields) == 3 {
			break
		}
	}
	card.Fields = fields
	return card
}

func description(value string) string {
	// Bound rich-text parsing even for very large saved descriptions.
	if len(value) > 64*1024 {
		value = value[:64*1024]
	}
	source := []byte(tiptap.RichTextToMarkdown(value))
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
	var result strings.Builder
	// The visitor cannot return an error; it only collects allowed prose nodes.
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if n.Type() == ast.TypeBlock {
				result.WriteByte(' ')
			}
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Image, *ast.CodeBlock, *ast.FencedCodeBlock, *ast.HTMLBlock, *ast.RawHTML, *ast.AutoLink:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			result.Write(node.Segment.Value(source))
			if node.SoftLineBreak() || node.HardLineBreak() {
				result.WriteByte(' ')
			}
		case *ast.String:
			result.Write(node.Value)
		}
		return ast.WalkContinue, nil
	})
	return plainValue(previewURLs.ReplaceAllString(html.UnescapeString(result.String()), ""), 160)
}

func plainValue(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || r == '\u202a' || r == '\u202b' || r == '\u202c' || r == '\u202d' || r == '\u202e' {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	cut := limit - 1
	// Prefer a word boundary without dropping most of a short CJK/long-word title.
	for i := cut; i > limit*2/3; i-- {
		if unicode.IsSpace(runes[i]) {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}
