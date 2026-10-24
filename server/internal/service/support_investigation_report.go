package service

import "strings"

const (
	// supportInvestigationReportFormat marks notes in the support-first layout,
	// so the inbox can fold the engineering section away.
	supportInvestigationReportFormat = "support_v1"
	supportInvestigationNoteLimit    = 6000
	supportInvestigationReplyLimit   = 2000
	supportInvestigationShortened    = "\n\n_Shortened to fit the note._"
	supportInvestigationReplyHeading = "Suggested reply"
)

// supportInvestigationSections are the report headings in note order. The last
// one holds engineering detail, so it is the first to go when a note is too long.
var supportInvestigationSections = []string{"What's happening", "How sure", "What to do now", supportInvestigationReplyHeading, "For engineering"}

type supportInvestigationReport struct {
	Content        string
	SuggestedReply string
	Structured     bool
}

// parseSupportInvestigationReport keeps only the final report. Models can think
// out loud before answering, so everything before the last "What's happening"
// heading is dropped. Replies without the layout are kept as written.
func parseSupportInvestigationReport(raw string) supportInvestigationReport {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(raw), "\r\n", "\n"), "\n")
	start := -1
	for index, line := range lines {
		if supportInvestigationHeading(line) == 0 {
			start = index
		}
	}
	if start < 0 {
		return supportInvestigationReport{Content: boundSupportInvestigationNote(raw)}
	}
	bodies := make([][]string, len(supportInvestigationSections))
	current := 0
	for _, line := range lines[start+1:] {
		// Headings only move forward, so an earlier heading repeated inside a
		// section stays part of that section's text.
		if next := supportInvestigationHeading(line); next > current {
			current = next
			continue
		}
		bodies[current] = append(bodies[current], line)
	}
	var report supportInvestigationReport
	var content strings.Builder
	for index, name := range supportInvestigationSections {
		body := strings.TrimSpace(strings.Join(bodies[index], "\n"))
		if body == "" || (name == supportInvestigationReplyHeading && strings.EqualFold(strings.Trim(body, " .*_"), "none")) {
			continue
		}
		if name == supportInvestigationReplyHeading && len([]rune(body)) <= supportInvestigationReplyLimit {
			report.SuggestedReply = body
		}
		content.WriteString("## " + name + "\n\n" + body + "\n\n")
	}
	if content.Len() == 0 {
		return supportInvestigationReport{Content: boundSupportInvestigationNote(raw)}
	}
	report.Content = boundSupportInvestigationNote(content.String())
	report.Structured = true
	return report
}

func supportInvestigationHeading(line string) int {
	text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
	text = strings.TrimSpace(strings.TrimSuffix(strings.Trim(text, "*_ "), ":"))
	text = strings.ReplaceAll(text, "’", "'")
	for index, name := range supportInvestigationSections {
		if strings.EqualFold(text, name) {
			return index
		}
	}
	return -1
}

// boundSupportInvestigationNote cuts a long note at a line break and closes an
// open code block, so it never ends mid-sentence inside broken formatting.
func boundSupportInvestigationNote(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= supportInvestigationNoteLimit {
		return content
	}
	// Leave room for the marker and a closing code fence.
	cut := string(runes[:supportInvestigationNoteLimit-len([]rune(supportInvestigationShortened))-4])
	if index := strings.LastIndex(cut, "\n"); index > len(cut)/2 {
		cut = cut[:index]
	}
	cut = strings.TrimSpace(cut)
	if strings.Count(cut, "```")%2 == 1 {
		cut += "\n```"
	}
	return cut + supportInvestigationShortened
}
