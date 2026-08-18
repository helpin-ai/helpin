package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type structuredKnowledgeChunk struct {
	Content       string
	SearchContent string
	HeadingPath   string
	SectionKey    string
	BlockID       *string
}

var (
	markdownHeadingPattern   = regexp.MustCompile(`^\s*(#{1,6})\s+(.+?)\s*#*\s*$`)
	htmlHeadingPattern       = regexp.MustCompile(`(?i)^\s*<h([1-6])[^>]*>(.*?)</h[1-6]>\s*$`)
	structuredHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)
)

// chunkStructuredDocument preserves headings as retrieval context while
// keeping the customer-facing evidence text free of synthetic prefixes.
func chunkStructuredDocument(title, text string) []structuredKnowledgeChunk {
	text = strings.ReplaceAll(strings.ToValidUTF8(text, ""), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	headings := make([]string, 0, 6)
	sectionLines := []string{}
	chunks := []structuredKnowledgeChunk{}
	sectionOccurrences := map[string]int{}

	flush := func() {
		body := strings.TrimSpace(strings.Join(sectionLines, "\n"))
		sectionLines = sectionLines[:0]
		if body == "" {
			return
		}
		path := append([]string(nil), headings...)
		if len(path) == 0 && strings.TrimSpace(title) != "" {
			path = []string{strings.TrimSpace(title)}
		}
		headingPath := strings.Join(path, " > ")
		identity := strings.ToLower(strings.TrimSpace(title) + "\n" + headingPath)
		occurrence := sectionOccurrences[identity]
		sectionOccurrences[identity]++
		sectionKey := structuredSectionKey(title, headingPath, occurrence)
		for _, part := range chunkDocumentText(body) {
			searchParts := []string{strings.TrimSpace(title)}
			if headingPath != "" && headingPath != strings.TrimSpace(title) {
				searchParts = append(searchParts, headingPath)
			}
			searchParts = append(searchParts, part)
			chunks = append(chunks, structuredKnowledgeChunk{
				Content:       part,
				SearchContent: strings.Join(nonEmptyStrings(searchParts), "\n"),
				HeadingPath:   headingPath,
				SectionKey:    sectionKey,
			})
		}
	}

	for _, line := range lines {
		level, heading, ok := parseKnowledgeHeading(line)
		if !ok {
			sectionLines = append(sectionLines, line)
			continue
		}
		flush()
		if level < 1 {
			level = 1
		}
		if level > len(headings)+1 {
			level = len(headings) + 1
		}
		headings = append(headings[:level-1], heading)
	}
	flush()

	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

func chunkStructuredBlocks(title string, blocks []model.DocsBlock) []structuredKnowledgeChunk {
	headings := make([]string, 0, 6)
	chunks := []structuredKnowledgeChunk{}
	sectionOccurrences := map[string]int{}
	nextSectionKey := func(headingPath string) string {
		identity := strings.ToLower(strings.TrimSpace(title) + "\n" + strings.TrimSpace(headingPath))
		occurrence := sectionOccurrences[identity]
		sectionOccurrences[identity]++
		return structuredSectionKey(title, headingPath, occurrence)
	}
	sectionKey := nextSectionKey(title)

	for _, block := range blocks {
		text := strings.TrimSpace(block.ContentText)
		if text == "" {
			text = strings.TrimSpace(extractEmbeddableBlockText(block.Content))
		}
		if text == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(block.Type), "heading") {
			level := docsBlockHeadingLevel(block.Content)
			if level > len(headings)+1 {
				level = len(headings) + 1
			}
			headings = append(headings[:level-1], text)
			sectionKey = nextSectionKey(strings.Join(headings, " > "))
			continue
		}

		headingPath := strings.Join(headings, " > ")
		if headingPath == "" {
			headingPath = strings.TrimSpace(title)
		}
		for _, part := range chunkDocumentText(text) {
			blockID := block.ID
			chunks = append(chunks, structuredKnowledgeChunk{
				Content:       part,
				SearchContent: strings.Join(nonEmptyStrings([]string{strings.TrimSpace(title), headingPath, part}), "\n"),
				HeadingPath:   headingPath,
				SectionKey:    sectionKey,
				BlockID:       &blockID,
			})
		}
	}
	return chunks
}

func parseKnowledgeHeading(line string) (int, string, bool) {
	if matches := markdownHeadingPattern.FindStringSubmatch(line); len(matches) == 3 {
		return len(matches[1]), strings.TrimSpace(matches[2]), true
	}
	if matches := htmlHeadingPattern.FindStringSubmatch(line); len(matches) == 3 {
		level, _ := strconv.Atoi(matches[1])
		heading := strings.TrimSpace(structuredHTMLTagPattern.ReplaceAllString(matches[2], " "))
		return level, strings.Join(strings.Fields(heading), " "), heading != ""
	}
	return 0, "", false
}

func docsBlockHeadingLevel(raw json.RawMessage) int {
	var node struct {
		Attrs struct {
			Level int `json:"level"`
		} `json:"attrs"`
	}
	if err := json.Unmarshal(raw, &node); err != nil || node.Attrs.Level < 1 || node.Attrs.Level > 6 {
		return 2
	}
	return node.Attrs.Level
}

func structuredSectionKey(title, headingPath string, occurrence int) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(title) + "\n" + strings.TrimSpace(headingPath) + "\n" + strconv.Itoa(occurrence)))
	return hex.EncodeToString(sum[:])
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func structuredChunkSearchInputs(chunks []structuredKnowledgeChunk) []string {
	inputs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		inputs = append(inputs, chunk.SearchContent)
	}
	return inputs
}

func neighborChunkIndexes(index, total int) (*int, *int) {
	var previous *int
	var next *int
	if index > 0 {
		value := index - 1
		previous = &value
	}
	if index+1 < total {
		value := index + 1
		next = &value
	}
	return previous, next
}
