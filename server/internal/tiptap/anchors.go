package tiptap

import "strings"

type headingAnchorCandidate struct {
	id      string
	key     string
	aliases []string
}

// NormalizeInternalAnchorLinks rewrites same-document links to the canonical
// IDs generated for their target headings. Imported Help Scout fragments often
// include unstable suffixes and target=_blank; neither should survive in a
// public article snapshot.
func NormalizeInternalAnchorLinks(root *Node) {
	if root == nil {
		return
	}
	headings := collectHeadingAnchorCandidates(root)
	normalizeInternalAnchorMarks(root, headings)
}

func collectHeadingAnchorCandidates(root *Node) []headingAnchorCandidate {
	candidates := make([]headingAnchorCandidate, 0)
	var walk func(*Node)
	walk = func(node *Node) {
		if node.Type == "heading" {
			id := strAttr(node.Attrs, "id")
			if id == "" {
				id = headingID(node)
			}
			candidates = append(candidates, headingAnchorCandidate{
				id:      id,
				key:     slugifyHeading(nodeText(node)),
				aliases: stringSliceAttr(node.Attrs, "anchorAliases"),
			})
		}
		for i := range node.Content {
			walk(&node.Content[i])
		}
	}
	walk(root)
	return candidates
}

func normalizeInternalAnchorMarks(node *Node, headings []headingAnchorCandidate) {
	for i := range node.Marks {
		mark := &node.Marks[i]
		if mark.Type != "link" || mark.Attrs == nil {
			continue
		}
		href, _ := mark.Attrs["href"].(string)
		if !strings.HasPrefix(href, "#") {
			continue
		}
		delete(mark.Attrs, "target")
		delete(mark.Attrs, "rel")
		if targetID := matchInternalAnchor(strings.TrimPrefix(href, "#"), node.Text, headings); targetID != "" {
			mark.Attrs["href"] = "#" + targetID
		}
	}
	for i := range node.Content {
		normalizeInternalAnchorMarks(&node.Content[i], headings)
	}
}

func matchInternalAnchor(fragment, linkText string, headings []headingAnchorCandidate) string {
	fragment = strings.TrimSpace(fragment)
	fragmentKey := strings.ToLower(fragment)
	linkKey := slugifyHeading(linkText)
	matches := make([]string, 0, 1)
	for _, heading := range headings {
		exact := fragment == heading.id
		if !exact {
			for _, alias := range heading.aliases {
				if fragment == alias {
					exact = true
					break
				}
			}
		}
		fragmentMatchesGeneratedID := fragmentKey == strings.ToLower(heading.id)
		fragmentMatchesImportedSuffix := fragmentKey == heading.key || strings.HasPrefix(fragmentKey, heading.key+"-")
		linkTextMatches := linkKey != "heading" && linkKey == heading.key
		if exact || fragmentMatchesGeneratedID || fragmentMatchesImportedSuffix || linkTextMatches {
			matches = append(matches, heading.id)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func nodeText(node *Node) string {
	var text strings.Builder
	extractText(&text, node)
	return text.String()
}
