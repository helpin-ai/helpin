package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

type documentSection struct {
	ID           string `json:"section_id"`
	Title        string `json:"title"`
	Level        int    `json:"level"`
	ParentID     string `json:"parent_section_id,omitempty"`
	StartBlockID string `json:"start_block_id"`
	EndBlockID   string `json:"end_block_id"`
	Start        int    `json:"start_index"`
	End          int    `json:"end_index"`
	Characters   int    `json:"characters"`
	BlockCount   int    `json:"block_count"`
}

type documentReadBlock struct {
	ID               string                          `json:"id"`
	Type             string                          `json:"type"`
	Revision         int                             `json:"revision"`
	Index            int                             `json:"index"`
	SectionID        string                          `json:"section_id,omitempty"`
	Markdown         string                          `json:"markdown,omitempty"`
	ContentText      string                          `json:"content_text,omitempty"`
	Content          json.RawMessage                 `json:"content,omitempty"`
	AgentReadable    *model.DocsBlockAgentProjection `json:"agent_readable,omitempty"`
	Match            bool                            `json:"match,omitempty"`
	FragmentOffset   *int                            `json:"fragment_offset,omitempty"`
	FragmentComplete *bool                           `json:"fragment_complete,omitempty"`
	ContentFragment  string                          `json:"content_fragment,omitempty"`
}

func documentOutline(blocks []model.DocsBlock) []documentSection {
	sections := []documentSection{}
	for i, block := range blocks {
		var node tiptap.Node
		if json.Unmarshal(block.Content, &node) != nil || node.Type != "heading" {
			continue
		}
		level := 1
		if n, ok := node.Attrs["level"].(float64); ok {
			level = int(n)
		}
		sections = append(sections, documentSection{ID: block.ID, Title: block.ContentText, Level: level, Start: i})
	}
	if len(blocks) > 0 && (len(sections) == 0 || sections[0].Start > 0) {
		sections = append([]documentSection{{ID: "preamble", Title: "Preamble", Level: 0, Start: 0}}, sections...)
	}
	for i := range sections {
		section := &sections[i]
		section.End = len(blocks) - 1
		for j := i + 1; j < len(sections); j++ {
			if section.Level == 0 || sections[j].Level <= section.Level {
				section.End = sections[j].Start - 1
				break
			}
		}
		for j := i - 1; j >= 0; j-- {
			if sections[j].Level > 0 && sections[j].Level < section.Level {
				section.ParentID = sections[j].ID
				break
			}
		}
		section.StartBlockID = blocks[section.Start].ID
		section.EndBlockID = blocks[section.End].ID
		section.BlockCount = section.End - section.Start + 1
		for _, b := range blocks[section.Start : section.End+1] {
			section.Characters += utf8.RuneCountInString(b.ContentText)
		}
	}
	return sections
}

func readableDocumentBlock(block model.DocsBlock, index int, sections []documentSection, format string) documentReadBlock {
	result := documentReadBlock{ID: block.ID, Type: block.Type, Revision: block.Revision, Index: index}
	for _, section := range sections {
		if index >= section.Start && index <= section.End {
			result.SectionID = section.ID
		}
	}
	switch format {
	case "json":
		result.Content = block.Content
	case "summary":
		result.ContentText = truncateCommandBarText(block.ContentText, 140)
	default:
		wrapper := json.RawMessage(`{"type":"doc","content":[` + string(block.Content) + `]}`)
		result.Markdown = tiptap.RichTextToMarkdown(string(wrapper))
		if result.Markdown == string(wrapper) {
			result.Markdown = block.ContentText
		}
		if _, ok := docsBlockDefinitionFor(block.Type); ok && block.Type != "codeBlock" && block.Type != "table" {
			projection := docsBlockAgentProjection(block)
			projection.Text = ""
			// Raw HTML belongs to the explicit JSON representation, not duplicated metadata.
			delete(projection.Attrs, "html")
			result.AgentReadable = projection
		}
	}
	return result
}

func documentBlockMatches(block model.DocsBlock, query string) bool {
	text := strings.ToLower(block.ContentText)
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
