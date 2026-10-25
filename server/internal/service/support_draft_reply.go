package service

import (
	"regexp"
	"strings"
)

var supportDraftListItem = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s+`)

// formatSupportDraftReply prepares an AI-written reply for the reply box,
// whose Markdown editor joins lines that are not separated by a blank line.
// Each plain line becomes its own paragraph and list items stay together.
func formatSupportDraftReply(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	for _, pair := range [][2]string{{`"`, `"`}, {"“", "”"}} {
		if strings.HasPrefix(text, pair[0]) && strings.HasSuffix(text, pair[1]) && len(text) > len(pair[0])+len(pair[1]) {
			inner := strings.TrimSpace(text[len(pair[0]) : len(text)-len(pair[1])])
			if !strings.ContainsAny(inner, `"“”`) {
				text = inner
			}
		}
	}
	var blocks []string
	inList := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			inList = false
			continue
		}
		item := supportDraftListItem.MatchString(line)
		if item && inList {
			blocks[len(blocks)-1] += "\n" + line
			continue
		}
		blocks = append(blocks, line)
		inList = item
	}
	return strings.Join(blocks, "\n\n")
}
