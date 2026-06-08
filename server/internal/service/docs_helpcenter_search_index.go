package service

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type helpcenterSearchNode struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text"`
	Attrs   map[string]interface{} `json:"attrs"`
	Content []helpcenterSearchNode `json:"content"`
}

// BuildHelpcenterSearchEntries converts a live article publication snapshot into
// structured page/section records suitable for public search.
func BuildHelpcenterSearchEntries(publication model.DocsHelpcenterArticlePublication) []model.DocsHelpcenterSearchEntry {
	entries := make([]model.DocsHelpcenterSearchEntry, 0, 8)
	position := 0
	addEntry := func(entryType, key, content string, sectionTitle, anchor *string, weight float64) {
		content = normalizeSearchText(content)
		if content == "" {
			return
		}
		entries = append(entries, model.DocsHelpcenterSearchEntry{
			WorkspaceID:  publication.WorkspaceID,
			DocumentID:   publication.DocumentID,
			Locale:       publication.Locale,
			EntryKey:     key,
			EntryType:    entryType,
			Content:      content,
			SectionTitle: sectionTitle,
			Anchor:       anchor,
			Position:     position,
			RankWeight:   weight,
			SearchConfig: helpcenterSearchConfigForLocale(publication.Locale),
			SearchVector: "",
		})
		position++
	}

	addEntry(model.DocsHelpcenterSearchEntryTypeTitle, "title", publication.Title, nil, nil, 8)
	if publication.Excerpt != nil {
		addEntry(model.DocsHelpcenterSearchEntryTypeExcerpt, "excerpt", *publication.Excerpt, nil, nil, 5)
	}

	var root helpcenterSearchNode
	if len(publication.Content) == 0 || json.Unmarshal(publication.Content, &root) != nil {
		addEntry(model.DocsHelpcenterSearchEntryTypeBody, "body:0", publication.ContentText, nil, nil, 1)
		return entries
	}

	seenAnchors := map[string]int{}
	var currentSectionTitle *string
	var currentAnchor *string
	bodyIndex := 0
	headingIndex := 0
	for _, node := range root.Content {
		if node.Type == "heading" {
			level := intAttr(node.Attrs, "level")
			text := normalizeSearchText(nodeText(node))
			if text == "" {
				continue
			}
			if level == 2 || level == 3 {
				base := searchStringAttr(node.Attrs, "id")
				if base == "" {
					base = slugifySearchAnchor(text)
				}
				anchor := uniqueSearchAnchor(base, seenAnchors)
				currentSectionTitle = searchStringPtr(text)
				currentAnchor = searchStringPtr(anchor)
				addEntry(model.DocsHelpcenterSearchEntryTypeHeading, "heading:"+anchor, text, currentSectionTitle, currentAnchor, 6)
			} else {
				currentSectionTitle = searchStringPtr(text)
				currentAnchor = nil
				addEntry(model.DocsHelpcenterSearchEntryTypeHeading, "heading:"+strconvItoa(headingIndex), text, currentSectionTitle, nil, 4)
			}
			headingIndex++
			continue
		}
		text := normalizeSearchText(nodeText(node))
		if text == "" {
			continue
		}
		key := "body:" + strconvItoa(bodyIndex)
		addEntry(model.DocsHelpcenterSearchEntryTypeBody, key, text, currentSectionTitle, currentAnchor, 1)
		bodyIndex++
	}

	return entries
}

func BuildHelpcenterSearchSnippet(content, query string) string {
	content = normalizeSearchText(content)
	terms := searchQueryTerms(query)
	if content == "" || len(terms) == 0 {
		return html.EscapeString(truncateSearchSnippet(content, 180))
	}

	lowerContent := strings.ToLower(content)
	start := 0
	for _, term := range terms {
		if idx := strings.Index(lowerContent, strings.ToLower(term)); idx >= 0 {
			start = idx - 60
			if start < 0 {
				start = 0
			}
			break
		}
	}
	snippet := content[start:]
	if start > 0 {
		snippet = "..." + snippet
	}
	snippet = truncateSearchSnippet(snippet, 220)
	escaped := html.EscapeString(snippet)
	for _, term := range terms {
		if term == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(html.EscapeString(term)))
		escaped = re.ReplaceAllStringFunc(escaped, func(match string) string {
			return "<mark>" + match + "</mark>"
		})
	}
	return escaped
}

func nodeText(node helpcenterSearchNode) string {
	if node.Text != "" {
		return node.Text
	}
	parts := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		text := nodeText(child)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func normalizeSearchText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func helpcenterSearchConfigForLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "en", "en-us", "en-gb":
		return "english"
	default:
		return "simple"
	}
}

func searchQueryTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
	}
	return terms
}

func truncateSearchSnippet(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func intAttr(attrs map[string]interface{}, key string) int {
	if attrs == nil {
		return 0
	}
	switch v := attrs[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func searchStringAttr(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func searchStringPtr(value string) *string {
	return &value
}

func uniqueSearchAnchor(base string, seen map[string]int) string {
	base = slugifySearchAnchor(base)
	if base == "" {
		base = "section"
	}
	seen[base]++
	if seen[base] == 1 {
		return base
	}
	return base + "-" + strconvItoa(seen[base])
}

func slugifySearchAnchor(text string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
