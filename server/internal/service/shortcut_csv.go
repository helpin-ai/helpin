package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const shortcutDescriptionLimit = 64 * 1024

type shortcutCSVRow struct {
	RowNumber            int
	ID                   string
	Name                 string
	Type                 string
	Requester            string
	Owners               string
	Description          string
	IsCompleted          string
	CreatedAt            string
	StartedAt            string
	UpdatedAt            string
	MovedAt              string
	CompletedAt          string
	Estimate             string
	IsBlocked            string
	DueDate              string
	Labels               string
	EpicLabels           string
	Tasks                string
	State                string
	EpicID               string
	Epic                 string
	IterationID          string
	Iteration            string
	UTCOffset            string
	IsArchived           string
	Team                 string
	EpicState            string
	EpicIsArchived       string
	EpicCreatedAt        string
	EpicStartedAt        string
	EpicDueDate          string
	ObjectiveID          string
	Objective            string
	ObjectiveState       string
	ObjectiveCreatedAt   string
	ObjectiveStartedAt   string
	ObjectiveDueDate     string
	EpicPlannedStartDate string
	Workflow             string
	WorkflowID           string
	Priority             string
	Severity             string
	CustomFields         string
	ParentStoryID        string
}

type shortcutDataset struct {
	Rows     []shortcutCSVRow
	Warnings []string
}

type shortcutChecklistItem struct {
	Text      string
	Completed bool
}

var (
	requiredShortcutColumns = []string{
		"id", "name", "type", "state", "workflow",
	}
	shortcutWeekRangeRe = regexp.MustCompile(`(?i)^(.+?:\s*)?week\s+(\d{1,2})-(\d{1,2}),\s*(\d{4})(?:-(\d{4}))?$`)
	shortcutDateRangeRe = regexp.MustCompile(`(?i)^([A-Za-z]+)\s+(\d{1,2})\s*-\s*([A-Za-z]+)?\s*(\d{1,2})(?:,\s*(\d{4}))?$`)
	shortcutHTMLTagRe   = regexp.MustCompile(`(?i)<(?:p|div|br|strong|em|ul|ol|li|blockquote|pre|code|a|img|h[1-6]|table)\b`)
	shortcutMDToHTML    = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(gmhtml.WithHardWraps()),
	)
)

func parseShortcutCSV(data []byte) (*shortcutDataset, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv has no header row")
	}

	header := make(map[string]int, len(records[0]))
	for idx, raw := range records[0] {
		header[strings.TrimSpace(raw)] = idx
	}
	missing := make([]string, 0)
	for _, col := range requiredShortcutColumns {
		if _, ok := header[col]; !ok {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("csv missing required columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]shortcutCSVRow, 0, len(records)-1)
	warnings := make([]string, 0)
	longDescriptions := 0
	for idx, record := range records[1:] {
		row := shortcutCSVRow{
			RowNumber:            idx + 2,
			ID:                   shortcutColumn(record, header, "id"),
			Name:                 shortcutColumn(record, header, "name"),
			Type:                 shortcutColumn(record, header, "type"),
			Requester:            shortcutColumn(record, header, "requester"),
			Owners:               shortcutColumn(record, header, "owners"),
			Description:          shortcutColumn(record, header, "description"),
			IsCompleted:          shortcutColumn(record, header, "is_completed"),
			CreatedAt:            shortcutColumn(record, header, "created_at"),
			StartedAt:            shortcutColumn(record, header, "started_at"),
			UpdatedAt:            shortcutColumn(record, header, "updated_at"),
			MovedAt:              shortcutColumn(record, header, "moved_at"),
			CompletedAt:          shortcutColumn(record, header, "completed_at"),
			Estimate:             shortcutColumn(record, header, "estimate"),
			IsBlocked:            shortcutColumn(record, header, "is_blocked"),
			DueDate:              shortcutColumn(record, header, "due_date"),
			Labels:               shortcutColumn(record, header, "labels"),
			EpicLabels:           shortcutColumn(record, header, "epic_labels"),
			Tasks:                shortcutColumn(record, header, "tasks"),
			State:                shortcutColumn(record, header, "state"),
			EpicID:               shortcutColumn(record, header, "epic_id"),
			Epic:                 shortcutColumn(record, header, "epic"),
			IterationID:          shortcutColumn(record, header, "iteration_id"),
			Iteration:            shortcutColumn(record, header, "iteration"),
			UTCOffset:            shortcutColumn(record, header, "utc_offset"),
			IsArchived:           shortcutColumn(record, header, "is_archived"),
			Team:                 shortcutColumn(record, header, "team"),
			EpicState:            shortcutColumn(record, header, "epic_state"),
			EpicIsArchived:       shortcutColumn(record, header, "epic_is_archived"),
			EpicCreatedAt:        shortcutColumn(record, header, "epic_created_at"),
			EpicStartedAt:        shortcutColumn(record, header, "epic_started_at"),
			EpicDueDate:          shortcutColumn(record, header, "epic_due_date"),
			ObjectiveID:          shortcutColumn(record, header, "objective_id"),
			Objective:            shortcutColumn(record, header, "objective"),
			ObjectiveState:       shortcutColumn(record, header, "objective_state"),
			ObjectiveCreatedAt:   shortcutColumn(record, header, "objective_created_at"),
			ObjectiveStartedAt:   shortcutColumn(record, header, "objective_started_at"),
			ObjectiveDueDate:     shortcutColumn(record, header, "objective_due_date"),
			EpicPlannedStartDate: shortcutColumn(record, header, "epic_planned_start_date"),
			Workflow:             shortcutColumn(record, header, "workflow"),
			WorkflowID:           shortcutColumn(record, header, "workflow_id"),
			Priority:             shortcutColumn(record, header, "priority"),
			Severity:             shortcutColumn(record, header, "severity"),
			CustomFields:         shortcutColumn(record, header, "custom_fields"),
			ParentStoryID:        shortcutColumn(record, header, "parent_story_id"),
		}
		if len(row.Description) > shortcutDescriptionLimit {
			row.Description = row.Description[:shortcutDescriptionLimit]
			longDescriptions++
		}
		rows = append(rows, row)
	}
	if longDescriptions > 0 {
		warnings = append(warnings, fmt.Sprintf("%d stories have descriptions longer than 64KB and will be truncated", longDescriptions))
	}

	return &shortcutDataset{Rows: rows, Warnings: warnings}, nil
}

func shortcutColumn(record []string, header map[string]int, name string) string {
	idx, ok := header[name]
	if !ok || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func shortcutSplitSemicolon(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func shortcutOwnerEmails(raw string) []string {
	return shortcutSplitSemicolon(raw)
}

func shortcutLabelNames(raw string) []string {
	return shortcutSplitSemicolon(raw)
}

func parseShortcutChecklist(raw string) []shortcutChecklistItem {
	parts := shortcutSplitSemicolon(raw)
	items := make([]shortcutChecklistItem, 0, len(parts))
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "[X]"):
			text := strings.TrimSpace(strings.TrimPrefix(part, "[X]"))
			if text != "" {
				items = append(items, shortcutChecklistItem{Text: text, Completed: true})
			}
		case strings.HasPrefix(part, "[x]"):
			text := strings.TrimSpace(strings.TrimPrefix(part, "[x]"))
			if text != "" {
				items = append(items, shortcutChecklistItem{Text: text, Completed: true})
			}
		case strings.HasPrefix(part, "[ ]"):
			text := strings.TrimSpace(strings.TrimPrefix(part, "[ ]"))
			if text != "" {
				items = append(items, shortcutChecklistItem{Text: text, Completed: false})
			}
		default:
			items = append(items, shortcutChecklistItem{Text: part, Completed: false})
		}
	}
	return items
}

func parseShortcutBool(raw string) bool {
	v, _ := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
	return v
}

func parseShortcutInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &n
}

func normalizeShortcutDescription(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if shortcutHTMLTagRe.MatchString(trimmed) {
		return raw
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	var buf bytes.Buffer
	if err := shortcutMDToHTML.Convert([]byte(normalized), &buf); err != nil {
		return raw
	}
	rendered := strings.TrimSpace(buf.String())
	if rendered == "" {
		return ""
	}
	return rendered
}

func parseShortcutTimestamp(raw, utcOffset string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		"2006/01/02 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02",
	}
	loc := time.UTC
	if utcOffset != "" {
		if parsed, err := parseUTCOffset(utcOffset); err == nil {
			loc = parsed
		}
	}
	for _, layout := range layouts {
		ts, err := time.ParseInLocation(layout, raw, loc)
		if err == nil {
			utc := ts.UTC()
			return &utc
		}
	}
	return nil
}

func parseUTCOffset(raw string) (*time.Location, error) {
	if len(raw) != 6 || (raw[0] != '+' && raw[0] != '-') || raw[3] != ':' {
		return nil, fmt.Errorf("invalid utc offset")
	}
	hours, err := strconv.Atoi(raw[1:3])
	if err != nil {
		return nil, err
	}
	minutes, err := strconv.Atoi(raw[4:6])
	if err != nil {
		return nil, err
	}
	total := hours*3600 + minutes*60
	if raw[0] == '-' {
		total = -total
	}
	return time.FixedZone(raw, total), nil
}

func normalizeShortcutName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func suggestedStateType(name string) string {
	switch normalizeShortcutName(name) {
	case "backlog":
		return model.PMStateTypeBacklog
	case "refinement", "up next", "ready for development":
		return model.PMStateTypeUnstarted
	case "in development", "ready for review", "ready for deploy":
		return model.PMStateTypeStarted
	case "completed", "done", "abandoned":
		return model.PMStateTypeDone
	default:
		return model.PMStateTypeUnstarted
	}
}

func sortKeysByCount(counter map[string]int) []string {
	keys := make([]string, 0, len(counter))
	for key := range counter {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counter[keys[i]] == counter[keys[j]] {
			return keys[i] < keys[j]
		}
		return counter[keys[i]] > counter[keys[j]]
	})
	return keys
}

func inferShortcutSprintDates(name string, rows []shortcutCSVRow) (*time.Time, *time.Time) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	anchorYear := inferShortcutSprintYear(rows)
	if match := shortcutWeekRangeRe.FindStringSubmatch(name); match != nil {
		startWeek, _ := strconv.Atoi(match[2])
		endWeek, _ := strconv.Atoi(match[3])
		startYear, _ := strconv.Atoi(match[4])
		endYear := startYear
		if match[5] != "" {
			endYear, _ = strconv.Atoi(match[5])
		} else if endWeek < startWeek {
			endYear = startYear + 1
		}
		start := isoWeekStart(startYear, startWeek)
		end := isoWeekStart(endYear, endWeek).AddDate(0, 0, 6)
		return &start, &end
	}

	match := shortcutDateRangeRe.FindStringSubmatch(name)
	if match == nil {
		return nil, nil
	}
	if anchorYear == 0 {
		return nil, nil
	}
	startMonth, ok := parseShortcutMonth(match[1])
	if !ok {
		return nil, nil
	}
	startDay, _ := strconv.Atoi(match[2])
	endMonth := startMonth
	if strings.TrimSpace(match[3]) != "" {
		var parsed bool
		endMonth, parsed = parseShortcutMonth(match[3])
		if !parsed {
			return nil, nil
		}
	}
	endDay, _ := strconv.Atoi(match[4])
	year := anchorYear
	if strings.TrimSpace(match[5]) != "" {
		year, _ = strconv.Atoi(match[5])
	}
	start := time.Date(year, startMonth, startDay, 0, 0, 0, 0, time.UTC)
	endYear := year
	if endMonth < startMonth {
		endYear++
	}
	end := time.Date(endYear, endMonth, endDay, 0, 0, 0, 0, time.UTC)
	if end.Before(start) {
		return nil, nil
	}
	return &start, &end
}

func inferShortcutSprintYear(rows []shortcutCSVRow) int {
	candidates := make([]time.Time, 0, len(rows)*4)
	for _, row := range rows {
		for _, raw := range []string{row.StartedAt, row.CompletedAt, row.CreatedAt, row.UpdatedAt} {
			if ts := parseShortcutTimestamp(raw, row.UTCOffset); ts != nil {
				candidates = append(candidates, *ts)
			}
		}
	}
	if len(candidates) == 0 {
		return 0
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	return candidates[0].Year()
}

func parseShortcutMonth(raw string) (time.Month, bool) {
	raw = strings.TrimSpace(raw)
	layouts := []string{"Jan", "January"}
	for _, layout := range layouts {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			return ts.Month(), true
		}
	}
	return 0, false
}

func isoWeekStart(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	week1Start := jan4.AddDate(0, 0, -(weekday - 1))
	return week1Start.AddDate(0, 0, (week-1)*7)
}
