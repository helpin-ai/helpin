package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) loadProviderContinuation(ctx context.Context, state *resolvedRunState) (*workerpkg.ProviderContinuation, error) {
	if state == nil || state.run == nil || state.agent == nil || a.artifactRepo == nil {
		return nil, nil
	}
	provider := strings.TrimSpace(derefString(state.agent.Provider))
	if !workerpkg.ProviderSupportsResponseContinuation(provider) {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeProviderResponseCheckpoint || artifact.InlineContent == nil {
			continue
		}
		var checkpoint model.ProviderResponseCheckpoint
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &checkpoint); err != nil {
			return nil, fmt.Errorf("parse provider response checkpoint: %w", err)
		}
		if strings.TrimSpace(checkpoint.ResponseID) == "" {
			continue
		}
		continuation := &workerpkg.ProviderContinuation{
			Provider:           strings.TrimSpace(checkpoint.Provider),
			ResponseID:         strings.TrimSpace(checkpoint.ResponseID),
			PreviousResponseID: strings.TrimSpace(checkpoint.PreviousResponseID),
			AfterSequenceNo:    checkpoint.AssistantMessageSeqNo,
		}
		slog.InfoContext(ctx, "loaded provider continuation checkpoint",
			"workspace_id", state.run.WorkspaceID,
			"run_id", state.run.ID,
			"provider", provider,
			"response_id", continuation.ResponseID,
			"after_sequence_no", continuation.AfterSequenceNo,
		)
		return continuation, nil
	}
	return nil, nil
}

func (a *AgentRunActivities) loadRunArtifactContext(ctx context.Context, state *resolvedRunState) (*workerpkg.ArtifactContext, error) {
	if state == nil || state.run == nil {
		return nil, nil
	}

	artifactContext := &workerpkg.ArtifactContext{}
	if parentRunID := strings.TrimSpace(derefString(state.run.ParentRunID)); parentRunID != "" && a.runRepo != nil {
		parentRun, err := a.runRepo.GetByID(ctx, state.run.WorkspaceID, parentRunID)
		if err != nil {
			return nil, err
		}
		parentEntries, err := a.loadParentRunArtifactContext(ctx, parentRun)
		if err != nil {
			return nil, err
		}
		artifactContext.Entries = append(artifactContext.Entries, parentEntries...)
	}
	if a.artifactRepo != nil {
		artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return nil, err
		}
		entries, err := buildArtifactContextEntries(artifacts)
		if err != nil {
			return nil, err
		}
		artifactContext.Entries = append(artifactContext.Entries, entries...)
	}
	if state.epic != nil && state.epic.SpecDocumentID != nil && strings.TrimSpace(*state.epic.SpecDocumentID) != "" && a.docsContentRepo != nil {
		content, err := a.docsContentRepo.GetByDocumentID(ctx, *state.epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if markdown := docsContentMarkdown(content); markdown != "" {
			artifactContext.Entries = append(artifactContext.Entries, workerpkg.ArtifactContextEntry{
				Label:        "Linked epic spec document",
				Source:       "spec_document",
				Status:       "approved",
				Format:       "markdown",
				Content:      markdown,
				PreserveFull: true,
			})
		}
	}
	if len(artifactContext.Entries) == 0 {
		return nil, nil
	}
	return workerpkg.TrimArtifactContext(artifactContext), nil
}

func (a *AgentRunActivities) loadParentRunArtifactContext(ctx context.Context, parentRun *model.AgentRun) ([]workerpkg.ArtifactContextEntry, error) {
	if parentRun == nil || a.artifactRepo == nil {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, parentRun.WorkspaceID, parentRun.ID)
	if err != nil {
		return nil, err
	}

	entries := make([]workerpkg.ArtifactContextEntry, 0, 4)
	if reason := strings.TrimSpace(derefString(parentRun.ErrorMessage)); reason != "" {
		entries = append(entries, workerpkg.ArtifactContextEntry{
			Label:        "Previous run failure reason",
			Source:       "previous_run_error",
			Status:       strings.TrimSpace(parentRun.Status),
			Format:       "text",
			Content:      reason,
			PreserveFull: true,
		})
	}
	if checkpoint, err := workerpkg.LatestTranscriptSummaryCheckpoint(artifacts); err != nil {
		return nil, err
	} else if checkpoint != nil && strings.TrimSpace(checkpoint.Summary) != "" {
		entries = append(entries, workerpkg.ArtifactContextEntry{
			Label:   "Previous run transcript summary",
			Source:  workerpkg.TranscriptSummaryArtifactType,
			Status:  strings.TrimSpace(parentRun.Status),
			Format:  "text",
			Content: strings.TrimSpace(checkpoint.Summary),
		})
	}

	parentEntries, err := buildArtifactContextEntries(artifacts)
	if err != nil {
		return nil, err
	}
	for _, entry := range parentEntries {
		entry.Label = "Previous run: " + entry.Label
		if strings.TrimSpace(entry.Status) == "" {
			entry.Status = strings.TrimSpace(parentRun.Status)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildArtifactContextEntries(artifacts []model.AgentRunArtifact) ([]workerpkg.ArtifactContextEntry, error) {
	if len(artifacts) == 0 {
		return nil, nil
	}
	sorted := append([]model.AgentRunArtifact(nil), artifacts...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SequenceNo == sorted[j].SequenceNo {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].SequenceNo < sorted[j].SequenceNo
	})

	applied := make(map[string]model.AppliedApprovedRunPreview)
	latestRunPreview := make(map[string]workerpkg.PublishedPreview)
	latestApproved := make(map[string]approvedPreviewContextEntry)
	var latestRunPlan *workerpkg.ArtifactContextEntry
	otherEntries := make([]workerpkg.ArtifactContextEntry, 0)

	for _, artifact := range sorted {
		if artifact.InlineContent == nil {
			continue
		}
		switch strings.TrimSpace(artifact.ArtifactType) {
		case model.AgentRunArtifactTypeApprovedPreviewApplied:
			var marker model.AppliedApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
				return nil, fmt.Errorf("parse approved preview applied artifact: %w", err)
			}
			if id := strings.TrimSpace(marker.ApprovedArtifactID); id != "" {
				applied[id] = marker
			}
		case workerpkg.RunPreviewArtifactType:
			var preview workerpkg.PublishedPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse run preview artifact: %w", err)
			}
			latestRunPreview[strings.TrimSpace(preview.PanelKey)] = preview
		case model.AgentRunArtifactTypeApprovedPreview:
			var preview model.ApprovedRunPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				return nil, fmt.Errorf("parse approved preview artifact: %w", err)
			}
			latestApproved[strings.TrimSpace(preview.PanelKey)] = approvedPreviewContextEntry{
				ArtifactID: artifact.ID,
				Preview:    preview,
			}
		case model.AgentRunArtifactTypeRunPlan:
			entry, err := buildRunPlanArtifactContextEntry(artifact)
			if err != nil {
				return nil, err
			}
			latestRunPlan = entry
		default:
			if entry := buildOtherArtifactContextEntry(artifact); entry != nil {
				otherEntries = append(otherEntries, *entry)
			}
		}
	}

	panelKeys := make([]string, 0, len(latestRunPreview)+len(latestApproved))
	seenPanels := make(map[string]bool)
	for panelKey := range latestRunPreview {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	for panelKey := range latestApproved {
		if panelKey == "" || seenPanels[panelKey] {
			continue
		}
		panelKeys = append(panelKeys, panelKey)
		seenPanels[panelKey] = true
	}
	sort.Strings(panelKeys)

	entries := make([]workerpkg.ArtifactContextEntry, 0, len(panelKeys)*2+len(otherEntries))
	for _, panelKey := range panelKeys {
		if preview, ok := latestRunPreview[panelKey]; ok && latestApproved[panelKey].Preview.Phase == "" {
			content, err := renderArtifactContextContent(preview.Format, preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Current preview for %s", panelKey),
				Source:       workerpkg.RunPreviewArtifactType,
				Status:       "draft",
				Format:       preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
		if approved, ok := latestApproved[panelKey]; ok {
			status := "approved"
			if marker, ok := applied[approved.ArtifactID]; ok && strings.TrimSpace(marker.Action) != "" {
				status = "approved_and_" + strings.TrimSpace(marker.Action)
			}
			content, err := renderArtifactContextContent(approved.Preview.Format, approved.Preview.Content)
			if err != nil {
				return nil, err
			}
			entries = append(entries, workerpkg.ArtifactContextEntry{
				Label:        fmt.Sprintf("Approved preview for %s", panelKey),
				Source:       model.AgentRunArtifactTypeApprovedPreview,
				Status:       status,
				Format:       approved.Preview.Format,
				Content:      content,
				PreserveFull: true,
			})
		}
	}
	if latestRunPlan != nil {
		entries = append(entries, *latestRunPlan)
	}
	entries = append(entries, otherEntries...)
	return entries, nil
}

type approvedPreviewContextEntry struct {
	ArtifactID string
	Preview    model.ApprovedRunPreview
}

func buildRunPlanArtifactContextEntry(artifact model.AgentRunArtifact) (*workerpkg.ArtifactContextEntry, error) {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil, nil
	}
	var plan workerpkg.RunPlanArtifact
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("parse run plan artifact: %w", err)
	}
	if err := workerpkg.ValidateRunPlanArtifactForContext(&plan); err != nil {
		return nil, nil
	}
	rendered := workerpkg.FormatRunPlanArtifactContentForContext(&plan)
	if rendered == "" {
		return nil, nil
	}
	return &workerpkg.ArtifactContextEntry{
		Label:   "Current execution plan",
		Source:  model.AgentRunArtifactTypeRunPlan,
		Status:  "active",
		Format:  "text",
		Content: rendered,
	}, nil
}

func buildOtherArtifactContextEntry(artifact model.AgentRunArtifact) *workerpkg.ArtifactContextEntry {
	content := strings.TrimSpace(derefString(artifact.InlineContent))
	if content == "" {
		return nil
	}
	switch strings.TrimSpace(artifact.ArtifactType) {
	case "product_spec_draft":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured product spec draft artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	case "task_plan_proposal":
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured task plan proposal artifact",
			Source:       artifact.ArtifactType,
			Status:       "draft",
			Format:       artifact.Format,
			Content:      content,
			PreserveFull: true,
		}
	case model.AgentRunArtifactTypeReviewFindings:
		rendered := formatReviewFindingsArtifactContent(content)
		if rendered == "" {
			return nil
		}
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured review findings artifact",
			Source:       artifact.ArtifactType,
			Status:       "active",
			Format:       "text",
			Content:      rendered,
			PreserveFull: true,
		}
	case model.AgentRunArtifactTypeReviewDecision:
		rendered := formatReviewDecisionArtifactContent(content)
		if rendered == "" {
			return nil
		}
		return &workerpkg.ArtifactContextEntry{
			Label:        "Structured review decision artifact",
			Source:       artifact.ArtifactType,
			Status:       "active",
			Format:       "text",
			Content:      rendered,
			PreserveFull: true,
		}
	default:
		return nil
	}
}

func formatReviewFindingsArtifactContent(content string) string {
	var artifact model.ReviewFindingsArtifact
	if err := json.Unmarshal([]byte(content), &artifact); err != nil {
		return ""
	}
	lines := make([]string, 0, len(artifact.Findings)+4)
	if title := strings.TrimSpace(artifact.Title); title != "" {
		lines = append(lines, title)
	}
	if summary := strings.TrimSpace(artifact.Summary); summary != "" {
		lines = append(lines, summary)
	}
	if overall := strings.TrimSpace(artifact.OverallCorrectness); overall != "" {
		lines = append(lines, "Overall correctness: "+overall)
	}
	if explanation := strings.TrimSpace(artifact.OverallExplanation); explanation != "" {
		lines = append(lines, "Overall explanation: "+explanation)
	}
	for _, finding := range artifact.Findings {
		line := "- "
		if priority := strings.TrimSpace(finding.Priority); priority != "" {
			line += priority + " "
		}
		line += strings.TrimSpace(finding.Title)
		if location := strings.TrimSpace(finding.CodeLocation); location != "" {
			line += " (" + location + ")"
		}
		body := strings.TrimSpace(finding.Body)
		if body != "" {
			line += ": " + body
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatReviewDecisionArtifactContent(content string) string {
	var artifact model.ReviewDecisionArtifact
	if err := json.Unmarshal([]byte(content), &artifact); err != nil {
		return ""
	}
	lines := make([]string, 0, len(artifact.Findings)+4)
	if title := strings.TrimSpace(artifact.Title); title != "" {
		lines = append(lines, title)
	}
	if decision := strings.TrimSpace(artifact.Decision); decision != "" {
		lines = append(lines, "Decision: "+decision)
	}
	if message := strings.TrimSpace(artifact.Message); message != "" {
		lines = append(lines, "Message: "+message)
	}
	for _, finding := range artifact.Findings {
		line := "- " + strings.TrimSpace(finding.Title)
		if status := strings.TrimSpace(finding.Status); status != "" {
			line += " [" + status + "]"
		}
		if location := strings.TrimSpace(finding.CodeLocation); location != "" {
			line += " (" + location + ")"
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (a *AgentRunActivities) ensureTranscriptSummaryCheckpoint(ctx context.Context, state *resolvedRunState, messages []model.AgentRunMessage) (*workerpkg.TranscriptSummaryCheckpoint, error) {
	if a.artifactRepo == nil || state == nil || state.run == nil {
		return nil, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return nil, err
	}
	latest, err := workerpkg.LatestTranscriptSummaryCheckpoint(artifacts)
	if err != nil {
		return nil, err
	}

	next := workerpkg.BuildTranscriptSummaryCheckpoint(messages)
	if next == nil {
		return latest, nil
	}
	if latest != nil && latest.CoveredThroughSequenceNo >= next.CoveredThroughSequenceNo {
		return latest, nil
	}

	if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, workerpkg.TranscriptSummaryArtifactType, "json", next, buildAssistantSequenceArtifactMetadata(lastAssistantSequenceNoUpTo(messages, next.CoveredThroughSequenceNo))); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "saved transcript summary checkpoint",
		"workspace_id", state.run.WorkspaceID,
		"run_id", state.run.ID,
		"covered_through_sequence_no", next.CoveredThroughSequenceNo,
		"source_message_count", next.SourceMessageCount,
	)
	return next, nil
}

func renderArtifactContextContent(format string, raw json.RawMessage) (string, error) {
	switch strings.TrimSpace(format) {
	case workerpkg.PreviewFormatMarkdown:
		var markdown string
		if err := json.Unmarshal(raw, &markdown); err != nil {
			return "", fmt.Errorf("parse markdown artifact content: %w", err)
		}
		return strings.TrimSpace(markdown), nil
	case workerpkg.PreviewFormatJSON:
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("parse json artifact content: %w", err)
		}
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", fmt.Errorf("format json artifact content: %w", err)
		}
		return string(pretty), nil
	default:
		return strings.TrimSpace(string(raw)), nil
	}
}

func shouldIncludeRunMessageInExecutionHistory(message model.AgentRunMessage) bool {
	return strings.TrimSpace(message.MessageType) != "status"
}

func hasExecutionHistoryMessages(messages []model.AgentRunMessage) bool {
	for _, message := range messages {
		if shouldIncludeRunMessageInExecutionHistory(message) {
			return true
		}
	}
	return false
}
