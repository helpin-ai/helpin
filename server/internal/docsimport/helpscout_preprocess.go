package docsimport

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// PreprocessHelpScoutHTML normalizes known Help Scout HTML quirks before the
// generic HTML-to-Tiptap conversion runs.
func PreprocessHelpScoutHTML(rawHTML string) (string, []Warning) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return rawHTML, nil
	}

	var warnings []Warning
	body := findBody(doc)

	normalizeHelpScoutAsideBlocks(body, &warnings)
	normalizeHelpScoutFakeStepHeadings(body)
	normalizeHelpScoutStepParagraphs(body)
	unwrapPunctuationOnlyFormatting(body)
	unwrapRedundantHeadingFormatting(body)
	trimBoundaryWhitespace(body)
	removeEmptyHeadingNodes(body, &warnings)
	removeBlankParagraphs(body, &warnings)

	return renderNode(body), warnings
}

var helpScoutStepParagraphPattern = regexp.MustCompile(`(?i)^step\s+([0-9]+)\s+[[:graph:]].*$`)

func normalizeHelpScoutAsideBlocks(root *html.Node, warnings *[]Warning) {
	for node := root.FirstChild; node != nil; {
		next := node.NextSibling
		if node.Type == html.ElementNode && node.DataAtom == atom.P {
			text := trimImportSpace(extractText(node))
			if strings.HasPrefix(text, "<aside>") {
				callout := &html.Node{
					Type:     html.ElementNode,
					DataAtom: atom.Section,
					Data:     atom.Section.String(),
					Attr: []html.Attribute{{
						Key: "class",
						Val: "callout callout-yellow",
					}},
				}

				cursor := node
				var closing *html.Node
				for cursor != nil {
					currentNext := cursor.NextSibling
					content := trimImportSpace(extractText(cursor))
					if cursor == node {
						content = strings.TrimSpace(strings.TrimPrefix(content, "<aside>"))
					}
					if content == "</aside>" {
						closing = cursor
						break
					}
					if content != "" {
						callout.AppendChild(newParagraphNode(content))
					}
					cursor = currentNext
				}
				if callout.FirstChild != nil && closing != nil {
					root.InsertBefore(callout, node)
					for remove := node; remove != nil; {
						removeNext := remove.NextSibling
						root.RemoveChild(remove)
						if remove == closing {
							break
						}
						remove = removeNext
					}
					*warnings = append(*warnings, warnHelpScoutNoteBlockNormalized())
					node = callout.NextSibling
					continue
				}
			}
		}
		if node.Type == html.ElementNode {
			normalizeHelpScoutAsideBlocks(node, warnings)
		}
		node = next
	}
}

func normalizeHelpScoutFakeStepHeadings(root *html.Node) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && isHeadingAtom(child.DataAtom) {
			if text, ok := extractHelpScoutFakeStepHeading(child); ok {
				root.InsertBefore(newTextElementNode(child.DataAtom, text), child)
				root.RemoveChild(child)
				child = next
				continue
			}
		}
		if child.Type == html.ElementNode {
			normalizeHelpScoutFakeStepHeadings(child)
		}
		child = next
	}
}

func normalizeHelpScoutStepParagraphs(root *html.Node) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && child.DataAtom == atom.P {
			if text, ok := extractHelpScoutStepParagraph(child); ok {
				root.InsertBefore(newTextElementNode(atom.H4, text), child)
				root.RemoveChild(child)
				child = next
				continue
			}
		}
		if child.Type == html.ElementNode {
			normalizeHelpScoutStepParagraphs(child)
		}
		child = next
	}
}

func extractHelpScoutStepParagraph(node *html.Node) (string, bool) {
	if hasMeaningfulChildElements(node) {
		return "", false
	}
	text := trimImportSpace(extractText(node))
	if text == "" || len(text) > 120 {
		return "", false
	}
	if !helpScoutStepParagraphPattern.MatchString(text) {
		return "", false
	}
	if strings.ContainsAny(text, ".:;!?") {
		return "", false
	}
	return text, true
}

func hasMeaningfulChildElements(node *html.Node) bool {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.DataAtom == atom.Br {
			continue
		}
		return true
	}
	return false
}

func extractHelpScoutFakeStepHeading(node *html.Node) (string, bool) {
	var parts []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && trimImportSpace(child.Data) == "" {
			continue
		}
		if child.Type != html.ElementNode || child.DataAtom != atom.P {
			return "", false
		}
		style := strings.ToLower(getAttr(child, "style"))
		if !strings.Contains(style, "inline-block") {
			return "", false
		}
		text := trimImportSpace(extractText(child))
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	if len(parts) < 2 {
		return "", false
	}
	step := parts[0]
	if isDigitsOnly(step) {
		step += "."
	}
	return strings.TrimSpace(step + " " + strings.Join(parts[1:], " ")), true
}

func unwrapPunctuationOnlyFormatting(root *html.Node) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && isInlineFormattingAtom(child.DataAtom) {
			text := trimImportSpace(extractText(child))
			if text == "" {
				root.RemoveChild(child)
				child = next
				continue
			}
			if isPunctuationOnly(text) {
				replaceNodeWithChildren(root, child)
				child = next
				continue
			}
		}
		if child.Type == html.ElementNode {
			unwrapPunctuationOnlyFormatting(child)
		}
		child = next
	}
}

func unwrapRedundantHeadingFormatting(root *html.Node) {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && isHeadingAtom(child.DataAtom) {
			unwrapFormattingDescendants(child)
			continue
		}
		if child.Type == html.ElementNode {
			unwrapRedundantHeadingFormatting(child)
		}
	}
}

func unwrapFormattingDescendants(root *html.Node) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && isInlineFormattingAtom(child.DataAtom) {
			replaceNodeWithChildren(root, child)
			child = next
			continue
		}
		if child.Type == html.ElementNode {
			unwrapFormattingDescendants(child)
		}
		child = next
	}
}

func replaceNodeWithChildren(parent, node *html.Node) {
	for child := node.FirstChild; child != nil; {
		nextChild := child.NextSibling
		node.RemoveChild(child)
		parent.InsertBefore(child, node)
		child = nextChild
	}
	parent.RemoveChild(node)
}

func trimBoundaryWhitespace(root *html.Node) {
	if root.Type != html.ElementNode {
		return
	}
	if root.DataAtom == atom.Pre || root.DataAtom == atom.Code || isInlineSpacingSensitiveAtom(root.DataAtom) {
		return
	}
	if root.DataAtom != atom.Pre && root.DataAtom != atom.Code {
		trimFirstTextChild(root)
		trimLastTextChild(root)
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		trimBoundaryWhitespace(child)
	}
}

func trimFirstTextChild(root *html.Node) {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.TextNode {
			if child.Type == html.ElementNode {
				trimFirstTextChild(child)
			}
			return
		}
		trimmed := strings.TrimLeftFunc(replaceNBSP(child.Data), unicode.IsSpace)
		if trimmed == "" {
			root.RemoveChild(child)
			return
		}
		child.Data = trimmed
		return
	}
}

func trimLastTextChild(root *html.Node) {
	for child := root.LastChild; child != nil; child = child.PrevSibling {
		if child.Type != html.TextNode {
			if child.Type == html.ElementNode {
				trimLastTextChild(child)
			}
			return
		}
		trimmed := strings.TrimRightFunc(replaceNBSP(child.Data), unicode.IsSpace)
		if trimmed == "" {
			root.RemoveChild(child)
			return
		}
		child.Data = trimmed
		return
	}
}

func removeEmptyHeadingNodes(root *html.Node, warnings *[]Warning) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && isHeadingAtom(child.DataAtom) && trimImportSpace(extractText(child)) == "" {
			root.RemoveChild(child)
			*warnings = append(*warnings, warnEmptyHeadingRemoved())
			child = next
			continue
		}
		if child.Type == html.ElementNode {
			removeEmptyHeadingNodes(child, warnings)
		}
		child = next
	}
}

func removeBlankParagraphs(root *html.Node, warnings *[]Warning) {
	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && child.DataAtom == atom.P && trimImportSpace(extractText(child)) == "" && findFirstChild(child, atom.Img) == nil {
			root.RemoveChild(child)
			*warnings = append(*warnings, warnBlankParagraphRemoved())
			child = next
			continue
		}
		if child.Type == html.ElementNode {
			removeBlankParagraphs(child, warnings)
		}
		child = next
	}
}

func newParagraphNode(text string) *html.Node {
	return newTextElementNode(atom.P, text)
}

func newTextElementNode(tag atom.Atom, text string) *html.Node {
	node := &html.Node{Type: html.ElementNode, DataAtom: tag, Data: tag.String()}
	node.AppendChild(&html.Node{Type: html.TextNode, Data: text})
	return node
}

func trimImportSpace(text string) string {
	return strings.TrimSpace(replaceNBSP(text))
}

func replaceNBSP(text string) string {
	return strings.ReplaceAll(text, "\u00a0", " ")
}

func isHeadingAtom(a atom.Atom) bool {
	switch a {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	default:
		return false
	}
}

func isInlineFormattingAtom(a atom.Atom) bool {
	switch a {
	case atom.Strong, atom.B, atom.Em, atom.I, atom.U, atom.Span, atom.Mark, atom.Small:
		return true
	default:
		return false
	}
}

func isInlineSpacingSensitiveAtom(a atom.Atom) bool {
	switch a {
	case atom.A, atom.Strong, atom.B, atom.Em, atom.I, atom.U, atom.S, atom.Del,
		atom.Code, atom.Sub, atom.Sup, atom.Span, atom.Small, atom.Mark:
		return true
	default:
		return false
	}
}

func isDigitsOnly(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isPunctuationOnly(text string) bool {
	if text == "" {
		return false
	}
	hasPunctuation := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
		hasPunctuation = true
	}
	return hasPunctuation
}
