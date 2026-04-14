package docsimport

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// NextraDocumentSource holds the parsed result of a single MDX/MD file.
type NextraDocumentSource struct {
	Frontmatter           map[string]any
	Title                 string
	Description           string
	Slug                  string
	Draft                 bool
	Body                  string
	UnsupportedComponents []string
	Warnings              []Warning
}

// ParseNextraMDX parses a Nextra .md or .mdx file, extracting
// frontmatter, cleaning import/export lines, and converting known
// MDX components to markdown equivalents.
func ParseNextraMDX(path string, data []byte) (*NextraDocumentSource, []Warning, error) {
	content := string(data)
	doc := &NextraDocumentSource{
		Frontmatter: make(map[string]any),
	}
	var warnings []Warning

	// Extract YAML frontmatter.
	content = extractFrontmatter(content, doc)

	// Remove import/export lines.
	content = removeImportExportLines(content)

	// Strip Nextra code fence modifiers (e.g. ```html copy → ```html).
	content = stripCodeFenceModifiers(content)

	// Process known MDX components.
	content, compWarnings := processKnownComponents(content)
	warnings = append(warnings, compWarnings...)

	// Detect and warn about unknown JSX components.
	content, unknownComps, unknownWarnings := detectUnknownComponents(content)
	warnings = append(warnings, unknownWarnings...)
	doc.UnsupportedComponents = unknownComps

	// Set body.
	doc.Body = strings.TrimSpace(content)

	// Derive title: frontmatter > first heading > filename.
	if doc.Title == "" {
		doc.Title = extractFirstHeading(doc.Body)
	}
	if doc.Title == "" {
		doc.Title = titleFromFilename(path)
	}

	// Strip the first H1 from body if it matches the title to avoid
	// duplication (title is shown separately in the document header).
	doc.Body = stripLeadingH1(doc.Body, doc.Title)

	// Derive slug from frontmatter or filename.
	if doc.Slug == "" {
		doc.Slug = slugFromFilename(path)
	}

	return doc, warnings, nil
}

// extractFrontmatter parses YAML frontmatter delimited by --- and
// populates the document source fields.
func extractFrontmatter(content string, doc *NextraDocumentSource) string {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return content
	}

	trimmed := strings.TrimSpace(content)
	// Find second ---.
	rest := trimmed[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return content
	}

	fmBlock := rest[:idx]
	afterFM := rest[idx+3:]

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		return content
	}

	doc.Frontmatter = fm

	if title, ok := fm["title"].(string); ok {
		doc.Title = title
	}
	if desc, ok := fm["description"].(string); ok {
		doc.Description = desc
	}
	if slug, ok := fm["slug"].(string); ok {
		doc.Slug = slug
	}
	if draft, ok := fm["draft"].(bool); ok {
		doc.Draft = draft
	}

	return afterFM
}

// removeImportExportLines strips ES module import and export lines
// that are common in MDX files.
func removeImportExportLines(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inMultilineImport := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inMultilineImport {
			if strings.Contains(trimmed, ")") || strings.HasSuffix(trimmed, "'") || strings.HasSuffix(trimmed, "\"") {
				inMultilineImport = false
			}
			continue
		}

		// Single-line import.
		if strings.HasPrefix(trimmed, "import ") {
			if !strings.Contains(trimmed, "from") && !strings.HasSuffix(trimmed, "'") && !strings.HasSuffix(trimmed, "\"") {
				inMultilineImport = true
			}
			continue
		}

		// Single-line export (not export default which would be content).
		if strings.HasPrefix(trimmed, "export ") && !strings.HasPrefix(trimmed, "export default") {
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// processKnownComponents converts known Nextra MDX components to
// markdown/HTML equivalents.
func processKnownComponents(content string) (string, []Warning) {
	var warnings []Warning

	// Process Callout components.
	content = processCallouts(content)

	// Process Steps — just strip the wrapper tags.
	content = stripComponentWrapper(content, "Steps")

	// Process Tabs — convert to sections.
	content, tabWarnings := processTabs(content)
	warnings = append(warnings, tabWarnings...)

	// Process Tab — strip wrapper (both <Tab> and <Tabs.Tab> dot notation).
	content = stripComponentWrapper(content, "Tab")
	content = stripDotComponentWrapper(content, "Tabs", "Tab")

	// Process Cards/Card — convert to link list.
	content = processCards(content)

	return content, warnings
}

// processCallouts converts <Callout ...>...</Callout> to
// blockquote-style callouts that map well to Helpin's callout system.
func processCallouts(content string) string {
	// Match paired Callout tags with any attributes.
	re := regexp.MustCompile(`(?s)<Callout\b([^>]*)>(.*?)</Callout>`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		attrs := ""
		if len(sub) > 1 {
			attrs = sub[1]
		}
		body := ""
		if len(sub) > 2 {
			body = strings.TrimSpace(sub[2])
		}

		// Extract type from attributes.
		calloutType := "info"
		typeRe := regexp.MustCompile(`type=["'](\w+)["']`)
		if m := typeRe.FindStringSubmatch(attrs); len(m) > 1 {
			calloutType = m[1]
		}

		prefix := calloutTypeToPrefix(calloutType)
		lines := strings.Split(body, "\n")
		var quoted []string
		quoted = append(quoted, fmt.Sprintf("> [!%s]", prefix))
		for _, line := range lines {
			quoted = append(quoted, "> "+strings.TrimSpace(line))
		}
		return strings.Join(quoted, "\n")
	})
}

// calloutTypeToPrefix maps Nextra callout types to GFM alert syntax.
func calloutTypeToPrefix(typ string) string {
	switch typ {
	case "warning":
		return "WARNING"
	case "error":
		return "CAUTION"
	case "info":
		return "NOTE"
	case "default", "":
		return "NOTE"
	default:
		return "NOTE"
	}
}

// stripComponentWrapper removes opening and closing tags for a
// component, preserving inner content.
func stripComponentWrapper(content, component string) string {
	// Remove opening tag (with optional attributes).
	openRe := regexp.MustCompile(`<` + component + `(?:\s[^>]*)?>`)
	content = openRe.ReplaceAllString(content, "")
	// Remove closing tag.
	closeRe := regexp.MustCompile(`</` + component + `>`)
	content = closeRe.ReplaceAllString(content, "")
	return content
}

// stripDotComponentWrapper removes dot-notation component tags like
// <Parent.Child> and </Parent.Child>.
func stripDotComponentWrapper(content, parent, child string) string {
	tag := parent + "." + child
	openRe := regexp.MustCompile(`<` + tag + `(?:\s[^>]*)?>`)
	content = openRe.ReplaceAllString(content, "")
	closeRe := regexp.MustCompile(`</` + tag + `>`)
	content = closeRe.ReplaceAllString(content, "")
	return content
}

// processTabs converts Tabs component to labeled sections.
// Extracts item labels from items={[...]} and inserts them as bold
// headings before each <Tabs.Tab> or <Tab> block.
func processTabs(content string) (string, []Warning) {
	var warnings []Warning

	// Find each <Tabs items={[...]}> and extract labels.
	tabsRe := regexp.MustCompile(`(?s)<Tabs\b([^>]*)>(.*?)</Tabs>`)
	itemsRe := regexp.MustCompile(`items=\{?\[([^\]]*)\]\}?`)
	// Match both <Tabs.Tab> and <Tab> blocks.
	tabBlockRe := regexp.MustCompile(`(?s)(?:<Tabs\.Tab>|<Tab>)(.*?)(?:</Tabs\.Tab>|</Tab>)`)

	content = tabsRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := tabsRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		attrs := sub[1]
		body := sub[2]

		// Extract labels.
		var labels []string
		if m := itemsRe.FindStringSubmatch(attrs); len(m) > 1 {
			raw := m[1]
			// Parse quoted strings from the array.
			labelRe := regexp.MustCompile(`["']([^"']+)["']`)
			for _, lm := range labelRe.FindAllStringSubmatch(raw, -1) {
				labels = append(labels, lm[1])
			}
		}

		// Replace each tab block with label + content.
		idx := 0
		result := tabBlockRe.ReplaceAllStringFunc(body, func(tabMatch string) string {
			tabSub := tabBlockRe.FindStringSubmatch(tabMatch)
			inner := ""
			if len(tabSub) > 1 {
				inner = strings.TrimSpace(tabSub[1])
			}
			var out string
			if idx < len(labels) {
				out = fmt.Sprintf("**%s**\n\n%s", labels[idx], inner)
			} else {
				out = inner
			}
			idx++
			return out
		})

		return result
	})

	return content, warnings
}

// processCards converts <Cards>/<Card> to a link list.
func processCards(content string) string {
	// Convert <Card title="..." href="..." /> to markdown link.
	selfClosingRe := regexp.MustCompile(`<Card\s+title=["']([^"']+)["']\s+href=["']([^"']+)["']\s*/?>`)
	content = selfClosingRe.ReplaceAllString(content, "- [$1]($2)")

	// Convert <Card title="..." href="...">...</Card> to markdown link.
	pairedRe := regexp.MustCompile(`(?s)<Card\s+title=["']([^"']+)["']\s+href=["']([^"']+)["']\s*>.*?</Card>`)
	content = pairedRe.ReplaceAllString(content, "- [$1]($2)")

	// Strip <Cards> wrapper.
	content = stripComponentWrapper(content, "Cards")

	return content
}

// detectUnknownComponents finds remaining JSX-like components that
// were not handled by known component processors.
func detectUnknownComponents(content string) (string, []string, []Warning) {
	var warnings []Warning
	seen := map[string]bool{}
	var unsupported []string

	// Strip fenced code blocks before scanning so that JSX inside
	// code examples (```jsx ... ```) doesn't trigger false warnings.
	codeFenceRe := regexp.MustCompile("(?s)```[^`]*```")
	contentWithoutCode := codeFenceRe.ReplaceAllString(content, "")

	// Match opening or self-closing JSX tags with PascalCase names.
	// This excludes standard HTML tags (lowercase).
	re := regexp.MustCompile(`<([A-Z][a-zA-Z0-9]*)\b[^>]*/?>`)

	matches := re.FindAllStringSubmatch(contentWithoutCode, -1)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			unsupported = append(unsupported, name)
			warnings = append(warnings, Warning{
				Type:    "unsupported_mdx_component",
				Message: fmt.Sprintf("component <%s> will be imported as placeholder text", name),
			})
		}
	}

	// Also check closing tags (in code-stripped content).
	closeRe := regexp.MustCompile(`</([A-Z][a-zA-Z0-9]*)>`)
	closeMatches := closeRe.FindAllStringSubmatch(contentWithoutCode, -1)
	for _, m := range closeMatches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			unsupported = append(unsupported, name)
			warnings = append(warnings, Warning{
				Type:    "unsupported_mdx_component",
				Message: fmt.Sprintf("component <%s> will be imported as placeholder text", name),
			})
		}
	}

	// Replace unknown self-closing components with a placeholder.
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) > 1 {
			name := sub[1]
			return fmt.Sprintf("<!-- Unsupported component: %s -->", name)
		}
		return match
	})

	// Replace unknown paired components, preserving inner text.
	for name := range seen {
		pairedRe := regexp.MustCompile(fmt.Sprintf(`(?s)<%s\b[^>]*>(.*?)</%s>`, name, name))
		content = pairedRe.ReplaceAllStringFunc(content, func(match string) string {
			inner := pairedRe.FindStringSubmatch(match)
			if len(inner) > 1 {
				return fmt.Sprintf("<!-- Unsupported component: %s -->\n%s", name, strings.TrimSpace(inner[1]))
			}
			return match
		})
	}

	return content, unsupported, warnings
}

// stripLeadingH1 removes the first line if it's a `# heading` that
// matches the document title, preventing duplication.
func stripLeadingH1(body, title string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	loc := re.FindStringIndex(body)
	if loc == nil || loc[0] != 0 {
		return body // no leading H1
	}
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return body
	}
	if strings.TrimSpace(match[1]) == strings.TrimSpace(title) {
		return strings.TrimSpace(body[loc[1]:])
	}
	return body
}

// stripCodeFenceModifiers removes Nextra-specific modifiers from
// code fence opening lines. For example, ```html copy becomes ```html.
// Common modifiers: copy, filename="...", {1,3-5} (line highlighting).
func stripCodeFenceModifiers(content string) string {
	re := regexp.MustCompile("(?m)^(```\\w+)\\s+.*$")
	return re.ReplaceAllString(content, "$1")
}

// extractFirstHeading returns the text of the first # heading.
func extractFirstHeading(content string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// titleFromFilename derives a human-readable title from a filename.
func titleFromFilename(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "index" || name == "page" {
		// Use parent directory name.
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			name = filepath.Base(dir)
		}
	}
	// Convert kebab-case to title case.
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// slugFromFilename derives a URL slug from a file path.
func slugFromFilename(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "index" || name == "page" {
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			return filepath.Base(dir)
		}
		return name
	}
	return name
}
