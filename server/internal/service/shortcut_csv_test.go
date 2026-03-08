package service

import (
	"strings"
	"testing"
	"time"
)

func TestParseShortcutChecklist(t *testing.T) {
	items := parseShortcutChecklist("[X] Done task;[ ] Todo task;plain task")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if !items[0].Completed || items[0].Text != "Done task" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].Completed || items[1].Text != "Todo task" {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
	if items[2].Completed || items[2].Text != "plain task" {
		t.Fatalf("unexpected third item: %+v", items[2])
	}
}

func TestInferShortcutSprintDatesWeekRange(t *testing.T) {
	start, end := inferShortcutSprintDates("Dev Team: Week 4-5, 2026", nil)
	if start == nil || end == nil {
		t.Fatal("expected parsed dates")
	}
	if got := start.Format("2006-01-02"); got != "2026-01-19" {
		t.Fatalf("unexpected start date: %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-02-01" {
		t.Fatalf("unexpected end date: %s", got)
	}
}

func TestInferShortcutSprintDatesMonthRangeUsesAnchorYear(t *testing.T) {
	rows := []shortcutCSVRow{
		{
			CreatedAt: "2025/11/08 10:00:00",
			UTCOffset: "+00:00",
		},
	}
	start, end := inferShortcutSprintDates("Nov 7 - Nov 21", rows)
	if start == nil || end == nil {
		t.Fatal("expected parsed dates")
	}
	if got := start.Format("2006-01-02"); got != "2025-11-07" {
		t.Fatalf("unexpected start date: %s", got)
	}
	if got := end.Format("2006-01-02"); got != "2025-11-21" {
		t.Fatalf("unexpected end date: %s", got)
	}
}

func TestParseShortcutTimestampWithUTCOffset(t *testing.T) {
	ts := parseShortcutTimestamp("2026/03/04 22:18:07", "+05:00")
	if ts == nil {
		t.Fatal("expected timestamp")
	}
	expected := time.Date(2026, time.March, 4, 17, 18, 7, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Fatalf("unexpected timestamp: got %s want %s", ts.UTC().Format(time.RFC3339), expected.Format(time.RFC3339))
	}
}

func TestNormalizeShortcutDescriptionMarkdownToHTML(t *testing.T) {
	raw := strings.Join([]string{
		"**Email:** bod@hanzonation.com",
		"**Plan:** premium-monthly",
		"**Chat:** https://app.crisp.chat/example",
		"",
		"![image.png](https://media.app.shortcut.com/example/image.png)",
	}, "\n")

	got := normalizeShortcutDescription(raw)
	wantFragments := []string{
		"<strong>Email:</strong>",
		"<strong>Plan:</strong>",
		`<a href="https://app.crisp.chat/example">https://app.crisp.chat/example</a>`,
		`<img src="https://media.app.shortcut.com/example/image.png" alt="image.png"/>`,
		"<br/>",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to contain %q, got %s", fragment, got)
		}
	}
}

func TestNormalizeShortcutDescriptionMarkdownTableToParagraphs(t *testing.T) {
	raw := strings.Join([]string{
		"### Reported By (Customer)",
		"",
		"| Field | Detail |",
		"|---|---|",
		"| **Name** | Paul Wright |",
		"| **Primary Email** | calmingthemindofcancer@gmail.com |",
		"",
		"> \"Credits did not reset after renewal.\"",
		"",
		"| Field | Value |",
		"|---|---|",
		"| **Articles per Month Counter** | **10 / 10** (showing fully exhausted — should be reset) |",
	}, "\n")

	got := normalizeShortcutDescription(raw)
	unwanted := []string{"<table", "<tr", "<td", "<th"}
	for _, fragment := range unwanted {
		if strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to strip table markup, got %s", got)
		}
	}
	wantFragments := []string{
		"<h3>Reported By (Customer)</h3>",
		"<p><strong>Name:</strong> Paul Wright</p>",
		`<p><strong>Primary Email:</strong> <a href="mailto:calmingthemindofcancer@gmail.com">calmingthemindofcancer@gmail.com</a></p>`,
		"<blockquote>",
		"<p><strong>Articles per Month Counter:</strong> <strong>10 / 10</strong> (showing fully exhausted",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to contain %q, got %s", fragment, got)
		}
	}
}

func TestNormalizeShortcutDescriptionMarkdownMultiColumnTableToParagraphs(t *testing.T) {
	raw := strings.Join([]string{
		"### Acceptance Criteria",
		"| # | Description | Notes / Edge cases |",
		"| --- | --- | --- |",
		"| **AC-1** | A \"Remove from folder\" action is available. | Shown next to Edit and Delete. |",
		"| **AC-2** | The item moves back to the root list. | Folder metadata is cleared only. |",
	}, "\n")

	got := normalizeShortcutDescription(raw)
	unwanted := []string{"<table", "<tr", "<td", "<th"}
	for _, fragment := range unwanted {
		if strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to strip table markup, got %s", got)
		}
	}
	wantFragments := []string{
		"<h3>Acceptance Criteria</h3>",
		"<p><strong>AC-1</strong> | A",
		"Shown next to Edit and Delete.",
		"<p><strong>AC-2</strong> | The item moves back to the root list. | Folder metadata is cleared only.</p>",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to contain %q, got %s", fragment, got)
		}
	}
}

func TestNormalizeShortcutDescriptionHTMLTableToParagraphs(t *testing.T) {
	raw := `<p>Subscription details</p><table><thead><tr><th>Field</th><th>Value</th></tr></thead><tbody><tr><td>Name</td><td>Paul Wright</td></tr><tr><td>Plan</td><td><strong>Starter Monthly</strong></td></tr></tbody></table>`

	got := normalizeShortcutDescription(raw)
	if strings.Contains(got, "<table") {
		t.Fatalf("expected rendered HTML to strip table markup, got %s", got)
	}
	wantFragments := []string{
		"<p>Subscription details</p>",
		"<p><strong>Name:</strong> Paul Wright</p>",
		"<p><strong>Plan:</strong> <strong>Starter Monthly</strong></p>",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected rendered HTML to contain %q, got %s", fragment, got)
		}
	}
}
