package service

import (
	"regexp"
	"strings"
)

const (
	supportInvestigationNoteLimit = 6000
	supportInvestigationShortened = "\n\n_Shortened to fit the note._"
	supportCodeCheckRunning       = "running"
	supportCodeCheckDone          = "done"
	supportCodeCheckFailed        = "failed"
)

// Report headings in order. The engineering section is last, so it is the
// first to go when a note is too long.
const (
	codeCheckFound       = "What AI found"
	codeCheckReply       = "Reply"
	codeCheckDocs        = "Docs"
	codeCheckTask        = "Task"
	codeCheckInvestigate = "Investigate"
	codeCheckUnavailable = "Couldn't check"
	codeCheckEngineering = "For engineering"
)

var supportInvestigationSections = []string{codeCheckFound, codeCheckReply, codeCheckDocs, codeCheckTask, codeCheckInvestigate, codeCheckUnavailable, codeCheckEngineering}

var supportTaskKeyPattern = regexp.MustCompile(`\b([A-Z]{2,5})-(\d+)\b`)

// supportCodeCheck is the code-check card saved in the note metadata. Empty
// sections are omitted; the inbox shows a row only for what is present.
type supportCodeCheck struct {
	Status          string                `json:"status"`
	Focus           string                `json:"focus,omitempty"`
	Message         string                `json:"message,omitempty"`
	Found           string                `json:"found,omitempty"`
	Reply           string                `json:"reply,omitempty"`
	Docs            string                `json:"docs,omitempty"`
	Task            *supportCodeCheckTask `json:"task,omitempty"`
	Investigate     string                `json:"investigate,omitempty"`
	UnavailableData []string              `json:"unavailable_data,omitempty"`
	Engineering     string                `json:"engineering,omitempty"`
}

// supportCodeCheckTask is either an existing task the work belongs to or a
// proposed new one. ID is set only after the key resolves in the workspace.
type supportCodeCheckTask struct {
	Kind string `json:"kind"`
	Key  string `json:"key,omitempty"`
	ID   string `json:"id,omitempty"`
	Text string `json:"text"`
}

// parseSupportInvestigationReport keeps only the final report. Models can think
// out loud before answering, so everything before the last "What AI found"
// heading is dropped. Replies without the layout become the finding as written.
func parseSupportInvestigationReport(raw string) supportCodeCheck {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(raw), "\r\n", "\n"), "\n")
	start := -1
	for index, line := range lines {
		if supportInvestigationHeading(line) == 0 {
			start = index
		}
	}
	check := supportCodeCheck{Status: supportCodeCheckDone}
	if start < 0 {
		check.Found = boundSupportInvestigationNote(raw)
		return check
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
	section := func(name string) string {
		for index, heading := range supportInvestigationSections {
			if heading == name {
				body := strings.TrimSpace(strings.Join(bodies[index], "\n"))
				if strings.EqualFold(strings.Trim(body, " .*_"), "none") {
					return ""
				}
				return body
			}
		}
		return ""
	}
	check.Found = section(codeCheckFound)
	check.Reply = formatSupportDraftReply(section(codeCheckReply))
	check.Docs = section(codeCheckDocs)
	check.Task = parseSupportCodeCheckTask(section(codeCheckTask))
	check.Investigate = section(codeCheckInvestigate)
	for _, line := range strings.Split(section(codeCheckUnavailable), "\n") {
		if item := strings.TrimSpace(supportDraftListItem.ReplaceAllString(line, "")); item != "" && !strings.EqualFold(strings.Trim(item, " ."), "none") {
			check.UnavailableData = append(check.UnavailableData, item)
		}
	}
	check.Engineering = boundSupportInvestigationNote(section(codeCheckEngineering))
	return check
}

// parseSupportCodeCheckTask reads "Existing: KEY-123 — what it covers" or
// "New: title". Anything else is kept as a proposed task.
func parseSupportCodeCheckTask(text string) *supportCodeCheckTask {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	kind, rest := "new", text
	if label, body, ok := strings.Cut(text, ":"); ok {
		switch strings.ToLower(strings.Trim(label, " *_")) {
		case "existing":
			kind, rest = "existing", body
		case "new":
			rest = body
		}
	}
	task := &supportCodeCheckTask{Kind: kind, Text: strings.TrimSpace(rest)}
	if kind == "existing" {
		match := supportTaskKeyPattern.FindStringIndex(rest)
		if match == nil {
			task.Kind = "new"
			return task
		}
		task.Key = rest[match[0]:match[1]]
		task.Text = strings.TrimSpace(strings.TrimLeft(rest[match[1]:], " —–-:"))
	}
	if task.Text == "" && task.Key == "" {
		return nil
	}
	return task
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

// renderSupportCodeCheckContent is the plain-text copy of the card for search,
// notifications and clients that predate the structured card.
func renderSupportCodeCheckContent(check supportCodeCheck) string {
	var b strings.Builder
	b.WriteString("Code check")
	switch check.Status {
	case supportCodeCheckRunning:
		if check.Focus != "" {
			b.WriteString("\n\nChecking the code: " + check.Focus)
		} else {
			b.WriteString("\n\nChecking the code for the cause.")
		}
		b.WriteString("\n\nDetails will appear here when done.")
		return b.String()
	case supportCodeCheckFailed:
		b.WriteString("\n\n" + check.Message)
		return b.String()
	}
	if check.Found != "" {
		b.WriteString("\n\n## " + codeCheckFound + "\n\n" + check.Found)
	}
	if len(check.UnavailableData) > 0 {
		b.WriteString("\n\n## " + codeCheckUnavailable + "\n\n" + briefList(check.UnavailableData))
	}
	var steps []string
	if check.Reply != "" {
		steps = append(steps, "Reply:\n\n"+check.Reply)
	}
	if check.Docs != "" {
		steps = append(steps, "Docs: "+check.Docs)
	}
	if check.Task != nil {
		label := "New task: "
		if check.Task.Key != "" {
			label = "Existing task " + check.Task.Key + ": "
		}
		steps = append(steps, "Task: "+label+check.Task.Text)
	}
	if check.Investigate != "" {
		steps = append(steps, "Investigate: "+check.Investigate)
	}
	if len(steps) > 0 {
		b.WriteString("\n\n## Next steps\n\n" + strings.Join(steps, "\n\n"))
	}
	if check.Engineering != "" {
		b.WriteString("\n\n## " + codeCheckEngineering + "\n\n" + check.Engineering)
	}
	return boundSupportInvestigationNote(b.String())
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
