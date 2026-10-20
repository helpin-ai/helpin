package service

import (
	"fmt"
	"strings"
)

type skillReadInput struct {
	Key             string `json:"key,omitempty"`
	SkillID         string `json:"skill_id,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	ExpectedVersion string `json:"expected_version,omitempty"`
	SectionHeading  string `json:"section_heading,omitempty"`
}

// Keep offsets relative to the selected section, and ignore headings in fenced
// examples. A section includes subsections up to the next peer or parent heading.
func skillInstructionSection(text, heading string) (string, []string, error) {
	type mark struct {
		text         string
		start, level int
	}
	marks := []mark{}
	offset := 0
	fence := ""
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			token := trimmed[:3]
			if fence == "" {
				fence = token
			} else if fence == token {
				fence = ""
			}
		} else if fence == "" {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				marks = append(marks, mark{trimmed, offset, level})
			}
		}
		offset += len(line)
	}
	headings := make([]string, 0, len(marks))
	found := -1
	for i, m := range marks {
		// The first page is an outline aid, not a second unbounded copy of the skill.
		if len(headings) < 100 {
			headings = append(headings, m.text)
		}
		if m.text == heading {
			if found >= 0 {
				return "", nil, fmt.Errorf("section heading is ambiguous; use full instruction pagination")
			}
			found = i
		}
	}
	if heading == "" {
		return text, headings, nil
	}
	if found < 0 {
		return "", nil, fmt.Errorf("section heading not found; use the exact heading from sections")
	}
	end := len(text)
	for _, m := range marks[found+1:] {
		if m.level <= marks[found].level {
			end = m.start
			break
		}
	}
	return text[marks[found].start:end], headings, nil
}
