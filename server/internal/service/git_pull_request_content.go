package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/prcontent"
)

const (
	maxPRSummaryLength  = 2000
	maxPRListItems      = 12
	maxPRListItemLength = 500
)

type delegatedRunPRValidation struct {
	Name    string
	Kind    string
	Command string
	Result  string
	Details string
}

type delegatedRunPRDetails struct {
	Summary         string
	Changes         []string
	Validations     []delegatedRunPRValidation
	ReviewNotes     []string
	Risks           []string
	UnresolvedItems []string
}

type delegatedRunPRContext struct {
	Title       string
	TargetKind  string
	TargetLabel string
	TargetURL   string
	RunURL      string
}

func (s *GitService) buildDelegatedRunPullRequestContent(ctx context.Context, run *model.AgentRun, delivery AgentRunRepositoryDelivery, head, base string) (string, string) {
	prContext := s.delegatedRunPullRequestContext(ctx, run)
	if prContext.Title == "" {
		prContext.Title = fmt.Sprintf("Automated changes from %s", head)
	}
	var outputSummary json.RawMessage
	if run != nil {
		outputSummary = run.OutputSummary
	}
	details := delegatedRunPullRequestDetailsFromSummary(outputSummary)
	agentName := cleanPRText(delivery.AgentName, maxPRListItemLength)
	if agentName == "" {
		agentName = "Helpin"
	}

	var sections []string
	summaryLines := []string{"## Summary", ""}
	if details.Summary != "" {
		summaryLines = append(summaryLines, details.Summary)
	} else if prContext.TargetLabel != "" {
		summaryLines = append(summaryLines, "Implements "+markdownLink(prContext.TargetLabel, prContext.TargetURL)+".")
	} else {
		summaryLines = append(summaryLines, "Automated repository changes delivered by **"+escapeMarkdownText(agentName)+"**.")
	}
	for _, change := range details.Changes {
		summaryLines = append(summaryLines, "- "+change)
	}
	sections = append(sections, strings.Join(summaryLines, "\n"))

	if len(details.Validations) > 0 {
		lines := []string{"## Validation", ""}
		for _, validation := range details.Validations {
			lines = append(lines, renderPRValidation(validation))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(details.ReviewNotes)+len(details.Risks)+len(details.UnresolvedItems) > 0 {
		lines := []string{"## Review notes", ""}
		for _, note := range details.ReviewNotes {
			lines = append(lines, "- "+note)
		}
		for _, risk := range details.Risks {
			lines = append(lines, "- **Risk:** "+risk)
		}
		for _, item := range details.UnresolvedItems {
			lines = append(lines, "- **Unresolved:** "+item)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	contextLines := []string{"## Helpin context", ""}
	if prContext.TargetLabel != "" {
		label := "Target"
		switch prContext.TargetKind {
		case "task":
			label = "Task"
		case "epic":
			label = "Epic"
		}
		contextLines = append(contextLines, "- "+label+": "+markdownLink(prContext.TargetLabel, prContext.TargetURL))
	}
	if prContext.RunURL != "" {
		contextLines = append(contextLines, "- Agent run: "+markdownLink(agentName+" delivery run", prContext.RunURL))
	} else {
		contextLines = append(contextLines, "- Agent: **"+escapeMarkdownText(agentName)+"**")
	}
	if strings.TrimSpace(head) != "" || strings.TrimSpace(base) != "" {
		contextLines = append(contextLines, "- Delivery: "+markdownCode(head)+" → "+markdownCode(base))
	}
	if commit := strings.TrimSpace(delivery.CommitSHA); commit != "" {
		contextLines = append(contextLines, "- Commit: "+markdownCode(commit))
	}
	sections = append(sections, strings.Join(contextLines, "\n"))

	footer := "Created by Helpin"
	if agentName != "Helpin" {
		footer += " agent **" + escapeMarkdownText(agentName) + "**"
	}
	footer += "."
	sections = append(sections, "---\n"+footer)

	return prContext.Title, prcontent.Wrap(strings.Join(sections, "\n\n"))
}

func (s *GitService) delegatedRunPullRequestContext(ctx context.Context, run *model.AgentRun) delegatedRunPRContext {
	if run == nil {
		return delegatedRunPRContext{}
	}
	workspaceSlug := ""
	workspaceKey := ""
	if s.workspaceRepo != nil {
		if workspace, err := s.workspaceRepo.GetByID(ctx, run.WorkspaceID); err == nil && workspace != nil {
			workspaceSlug = strings.TrimSpace(workspace.Slug)
			workspaceKey = strings.TrimSpace(workspace.WorkspaceKey)
		}
	}

	result := delegatedRunPRContext{}
	switch run.TargetType {
	case "task":
		taskID := delegatedRunTaskID(run)
		if taskID != "" && s.taskRepo != nil {
			if task, err := s.taskRepo.GetRawByID(ctx, taskID); err == nil && task != nil {
				taskName := cleanPRText(task.Name, maxPRListItemLength)
				taskKey := ""
				if workspaceKey != "" && task.DisplayID > 0 {
					taskKey = model.FormatTaskKey(workspaceKey, task.DisplayID)
				}
				result.Title = taskName
				switch {
				case taskKey != "" && taskName != "":
					result.Title = taskKey + ": " + taskName
					result.TargetLabel = taskKey + " — " + taskName
				case taskKey != "":
					result.Title = taskKey
					result.TargetLabel = taskKey
				default:
					result.TargetLabel = taskName
				}
				result.TargetKind = "task"
				result.TargetURL = s.helpinURL(workspaceSlug, "pm", "tasks", task.ID)
			}
		}
	case "epic":
		if s.epicRepo != nil {
			if epicWithStats, err := s.epicRepo.GetByID(ctx, strings.TrimSpace(run.TargetID)); err == nil && epicWithStats != nil {
				name := cleanPRText(epicWithStats.Epic.Name, maxPRListItemLength)
				if name != "" {
					result.Title = "Merge epic: " + name
					result.TargetLabel = name
				}
				result.TargetKind = "epic"
				result.TargetURL = s.helpinURL(workspaceSlug, "pm", "epics", epicWithStats.Epic.ID)
			}
		}
	}
	if workspaceSlug != "" && strings.TrimSpace(run.ID) != "" {
		result.RunURL = s.helpinRunURL(workspaceSlug, run.ID)
	}
	return result
}

func (s *GitService) helpinURL(workspaceSlug string, parts ...string) string {
	if strings.TrimSpace(s.appBaseURL) == "" || strings.TrimSpace(workspaceSlug) == "" {
		return ""
	}
	path := s.appBaseURL + "/w/" + url.PathEscape(workspaceSlug)
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			path += "/" + url.PathEscape(part)
		}
	}
	return path
}

func (s *GitService) helpinRunURL(workspaceSlug, runID string) string {
	base := s.helpinURL(workspaceSlug, "automation", "activity")
	if base == "" || strings.TrimSpace(runID) == "" {
		return ""
	}
	query := url.Values{"run_id": []string{strings.TrimSpace(runID)}}
	return base + "?" + query.Encode()
}

func delegatedRunPullRequestDetailsFromSummary(raw json.RawMessage) delegatedRunPRDetails {
	var root map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return delegatedRunPRDetails{}
	}
	var candidates []map[string]json.RawMessage
	if repository := rawJSONObject(root["repository"]); repository != nil {
		if pullRequest := rawJSONObject(repository["pull_request"]); pullRequest != nil {
			candidates = append(candidates, pullRequest)
		}
	}
	if pullRequest := rawJSONObject(root["pull_request"]); pullRequest != nil {
		candidates = append(candidates, pullRequest)
	}
	if manifest := rawJSONObject(root["delivery_manifest"]); manifest != nil {
		candidates = append(candidates, manifest)
	}
	candidates = append(candidates, root)

	var result delegatedRunPRDetails
	for _, candidate := range candidates {
		mergePRDetails(&result, decodePRDetails(candidate))
	}
	return result
}

func rawJSONObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}

func decodePRDetails(value map[string]json.RawMessage) delegatedRunPRDetails {
	result := delegatedRunPRDetails{
		Summary:         rawJSONString(value["summary"]),
		Changes:         rawJSONStringList(value["changes"]),
		ReviewNotes:     rawJSONStringList(value["review_notes"]),
		Risks:           rawJSONStringList(value["risks"]),
		UnresolvedItems: rawJSONStringList(value["unresolved_items"]),
	}
	var validations []json.RawMessage
	if json.Unmarshal(value["validations"], &validations) == nil {
		for _, raw := range validations {
			if text := rawJSONString(raw); text != "" {
				result.Validations = append(result.Validations, delegatedRunPRValidation{Name: text})
				continue
			}
			item := rawJSONObject(raw)
			if item == nil {
				continue
			}
			validation := delegatedRunPRValidation{
				Name:    firstNonEmptyString(rawJSONString(item["name"]), rawJSONString(item["title"])),
				Kind:    rawJSONString(item["kind"]),
				Command: rawJSONString(item["command"]),
				Result:  firstNonEmptyString(rawJSONString(item["result"]), rawJSONString(item["status"])),
				Details: firstNonEmptyString(rawJSONString(item["details"]), rawJSONString(item["summary"])),
			}
			if validation.Name != "" || validation.Kind != "" || validation.Command != "" || validation.Details != "" {
				result.Validations = append(result.Validations, validation)
			}
		}
	}
	return normalizePRDetails(result)
}

func mergePRDetails(target *delegatedRunPRDetails, candidate delegatedRunPRDetails) {
	if target.Summary == "" {
		target.Summary = candidate.Summary
	}
	if len(target.Changes) == 0 {
		target.Changes = candidate.Changes
	}
	if len(target.Validations) == 0 {
		target.Validations = candidate.Validations
	}
	if len(target.ReviewNotes) == 0 {
		target.ReviewNotes = candidate.ReviewNotes
	}
	if len(target.Risks) == 0 {
		target.Risks = candidate.Risks
	}
	if len(target.UnresolvedItems) == 0 {
		target.UnresolvedItems = candidate.UnresolvedItems
	}
}

func normalizePRDetails(value delegatedRunPRDetails) delegatedRunPRDetails {
	value.Summary = cleanPRText(value.Summary, maxPRSummaryLength)
	value.Changes = cleanPRList(value.Changes)
	value.ReviewNotes = cleanPRList(value.ReviewNotes)
	value.Risks = cleanPRList(value.Risks)
	value.UnresolvedItems = cleanPRList(value.UnresolvedItems)
	if len(value.Validations) > maxPRListItems {
		value.Validations = value.Validations[:maxPRListItems]
	}
	for index := range value.Validations {
		value.Validations[index].Name = cleanPRText(value.Validations[index].Name, maxPRListItemLength)
		value.Validations[index].Kind = cleanPRText(value.Validations[index].Kind, maxPRListItemLength)
		value.Validations[index].Command = cleanPRText(value.Validations[index].Command, maxPRListItemLength)
		value.Validations[index].Result = cleanPRText(value.Validations[index].Result, maxPRListItemLength)
		value.Validations[index].Details = cleanPRText(value.Validations[index].Details, maxPRListItemLength)
	}
	return value
}

func rawJSONString(raw json.RawMessage) string {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func rawJSONStringList(raw json.RawMessage) []string {
	var values []string
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil {
		return nil
	}
	return values
}

func cleanPRList(values []string) []string {
	result := make([]string, 0, min(len(values), maxPRListItems))
	for _, value := range values {
		if cleaned := cleanPRText(value, maxPRListItemLength); cleaned != "" {
			result = append(result, cleaned)
			if len(result) == maxPRListItems {
				break
			}
		}
	}
	return result
}

func cleanPRText(value string, maxLength int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	value = strings.ReplaceAll(value, prcontent.StartMarker, "")
	value = strings.ReplaceAll(value, prcontent.EndMarker, "")
	value = strings.TrimSpace(value)
	if maxLength <= 0 || utf8.RuneCountInString(value) <= maxLength {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxLength-1])) + "…"
}

func renderPRValidation(validation delegatedRunPRValidation) string {
	name := firstNonEmptyString(validation.Name, validation.Kind, "Validation")
	if validation.Command != "" {
		name += " — " + markdownCode(validation.Command)
	}
	icon := "➖"
	switch strings.ToLower(strings.TrimSpace(validation.Result)) {
	case "passed", "pass", "success", "succeeded", "ok":
		icon = "✅"
	case "failed", "fail", "error":
		icon = "❌"
	case "warning", "warn", "partial", "blocked":
		icon = "⚠️"
	case "skipped", "not_run":
		icon = "⏭️"
	}
	line := "- " + icon + " " + name
	if validation.Result != "" {
		line += " (" + strings.ToLower(validation.Result) + ")"
	}
	if validation.Details != "" {
		line += " — " + validation.Details
	}
	return line
}

func markdownLink(label, target string) string {
	label = escapeMarkdownText(label)
	if strings.TrimSpace(target) == "" {
		return label
	}
	return "[" + label + "](" + target + ")"
}

func escapeMarkdownText(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_")
	return replacer.Replace(value)
}

func markdownCode(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "`", "'"))
	if value == "" {
		return "`—`"
	}
	return "`" + value + "`"
}
