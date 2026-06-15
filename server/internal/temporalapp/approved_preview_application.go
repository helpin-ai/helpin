package temporalapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) applyApprovedInteractivePreview(ctx context.Context, state *resolvedRunState, input *planningRunInput) (string, error) {
	if state == nil || state.run == nil || input == nil {
		return "", nil
	}
	if state.run.InvocationMode != model.InvocationModeInteractive {
		return "", nil
	}
	if a.artifactRepo == nil {
		return "", nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return "", err
	}
	artifacts, err = a.ensureApprovedPreviewFromResolvedInteraction(ctx, state, artifacts)
	if err != nil {
		return "", err
	}
	approvedArtifact, approvedPreview, err := nextUnappliedApprovedPreview(artifacts)
	if err != nil || approvedArtifact == nil || approvedPreview == nil {
		return "", err
	}

	phase := canonicalApprovedPreviewPhase(approvedPreview.Phase, approvedPreview.PanelKey, approvedPreview.Format)

	var appliedAction string
	switch phase {
	case "prd":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved PRD preview requires an epic target")
		}
		if err := a.applyApprovedPRDPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_prd"
	case "task_doc":
		if state.task == nil {
			return "", fmt.Errorf("approved task planning doc preview requires a task target")
		}
		if err := a.applyApprovedTaskDocPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "persist_task_doc"
	case "tasks":
		if state.epic == nil || state.run.TargetType != "epic" {
			return "", fmt.Errorf("approved task plan preview requires an epic target")
		}
		if err := a.applyApprovedTaskPlanPreview(ctx, state, input, approvedPreview); err != nil {
			return "", err
		}
		appliedAction = "create_tasks"
	default:
		return "", fmt.Errorf("unsupported approved preview phase %q", approvedPreview.Phase)
	}

	if _, err := a.appendRunArtifact(ctx, state.run, model.AgentRunArtifactTypeApprovedPreviewApplied, "json", model.AppliedApprovedRunPreview{
		ApprovedArtifactID: approvedArtifact.ID,
		Phase:              phase,
		Action:             appliedAction,
		AppliedAt:          time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return appliedAction, nil
}

func (a *AgentRunActivities) ensureApprovedPreviewFromResolvedInteraction(ctx context.Context, state *resolvedRunState, artifacts []model.AgentRunArtifact) ([]model.AgentRunArtifact, error) {
	if a == nil || a.interactionRepo == nil || a.artifactRepo == nil || state == nil || state.run == nil {
		return artifacts, nil
	}
	if existingArtifact, _, err := nextUnappliedApprovedPreview(artifacts); err != nil || existingArtifact != nil {
		return artifacts, err
	}

	interaction, err := a.interactionRepo.GetLatestResolvedByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil || interaction == nil {
		return artifacts, err
	}
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindApprovalRequest, model.AgentRunInteractionKindReviewCheckpoint:
	default:
		return artifacts, nil
	}
	if !resolvedInteractionApproved(interaction) {
		return artifacts, nil
	}

	var approval model.ApprovalRequest
	if err := json.Unmarshal(interaction.RequestPayload, &approval); err != nil {
		return artifacts, fmt.Errorf("parse resolved approval request: %w", err)
	}
	approval.PreviewPanelKey = normalizeApprovalPreviewPanelKey(approval.PreviewPanelKey)

	assistantSequenceNo := 0
	if interaction.AssistantMessageSequenceNo != nil {
		assistantSequenceNo = *interaction.AssistantMessageSequenceNo
	}
	preview, err := latestRunPreviewForResolvedApproval(artifacts, assistantSequenceNo, approval.PreviewPanelKey)
	if err != nil || preview == nil {
		return artifacts, err
	}

	phase := canonicalApprovedPreviewPhase(approval.Phase, preview.PanelKey, preview.Format)

	content := append(json.RawMessage(nil), preview.Content...)
	if phase == "tasks" && strings.EqualFold(strings.TrimSpace(preview.Format), workerpkg.PreviewFormatJSON) {
		normalizedContent, err := workerpkg.NormalizeTaskPlanPreviewContent(content)
		if err != nil {
			return artifacts, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}: %w", err)
		}
		content = normalizedContent
	}
	if existingApprovedPreviewForResolvedInteraction(artifacts, phase, preview, content, assistantSequenceNo) {
		return artifacts, nil
	}

	approvedBy := ""
	if interaction.ResolvedBy != nil {
		approvedBy = strings.TrimSpace(*interaction.ResolvedBy)
	}
	approvedAt := time.Now().UTC()
	if interaction.ResolvedAt != nil {
		approvedAt = interaction.ResolvedAt.UTC()
	}
	artifact, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeApprovedPreview, "json", model.ApprovedRunPreview{
		Phase:                      phase,
		ApprovalTitle:              strings.TrimSpace(approval.Title),
		ApprovalSummary:            strings.TrimSpace(approval.Summary),
		PanelKey:                   strings.TrimSpace(preview.PanelKey),
		PreviewTitle:               strings.TrimSpace(preview.Title),
		Format:                     strings.TrimSpace(preview.Format),
		Content:                    content,
		AssistantMessageSequenceNo: assistantSequenceNo,
		ApprovedBy:                 approvedBy,
		ApprovedAt:                 approvedAt,
	}, buildAssistantSequenceArtifactMetadata(assistantSequenceNo))
	if err != nil {
		return artifacts, err
	}
	return append(artifacts, *artifact), nil
}

func existingApprovedPreviewForResolvedInteraction(artifacts []model.AgentRunArtifact, phase string, preview *workerpkg.PublishedPreview, content json.RawMessage, assistantSequenceNo int) bool {
	if preview == nil {
		return false
	}
	phase = canonicalApprovedPreviewPhase(phase, preview.PanelKey, preview.Format)
	panelKey := normalizeApprovalPreviewPanelKey(preview.PanelKey)
	format := strings.TrimSpace(preview.Format)
	normalizedContent := strings.TrimSpace(string(content))
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		var existing model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &existing); err != nil {
			continue
		}
		if canonicalApprovedPreviewPhase(existing.Phase, existing.PanelKey, existing.Format) != phase {
			continue
		}
		if normalizeApprovalPreviewPanelKey(existing.PanelKey) != panelKey {
			continue
		}
		if strings.TrimSpace(existing.Format) != format {
			continue
		}
		if assistantSequenceNo > 0 && existing.AssistantMessageSequenceNo > 0 && existing.AssistantMessageSequenceNo != assistantSequenceNo {
			continue
		}
		if strings.TrimSpace(string(existing.Content)) != normalizedContent {
			continue
		}
		return true
	}
	return false
}

func canonicalApprovedPreviewPhase(phase, panelKey, format string) string {
	normalizedPhase := strings.ToLower(strings.TrimSpace(phase))
	switch normalizedPhase {
	case "prd", "tasks", "task_doc":
		return normalizedPhase
	}

	switch normalizeApprovalPreviewPanelKey(panelKey) {
	case "prd_draft":
		return "prd"
	case "task_plan":
		return "tasks"
	case "task_plan_doc":
		return "task_doc"
	default:
		return normalizedPhase
	}
}

func resolvedInteractionApproved(interaction *model.AgentRunInteraction) bool {
	if interaction == nil || len(interaction.ResponsePayload) == 0 || strings.TrimSpace(string(interaction.ResponsePayload)) == "null" {
		return false
	}
	var payload struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(interaction.ResponsePayload, &payload); err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(payload.Decision)) {
	case "approve", "approved", "accept", "accepted":
		return true
	default:
		return false
	}
}

func latestRunPreviewForResolvedApproval(artifacts []model.AgentRunArtifact, assistantSequenceNo int, panelKey string) (*workerpkg.PublishedPreview, error) {
	targetKey := normalizeApprovalPreviewPanelKey(panelKey)
	if targetKey != "" {
		if preview, _, err := latestRunPreviewForAssistantSequence(artifacts, assistantSequenceNo, targetKey); err != nil || preview != nil {
			return preview, err
		}
		if !isCanonicalApprovalPreviewPanelKey(targetKey) {
			preview, count, err := latestRunPreviewForAssistantSequence(artifacts, assistantSequenceNo, "")
			if err != nil || count != 1 {
				return nil, err
			}
			return preview, nil
		}
		preview, _, err := latestRunPreviewForAnySequence(artifacts, targetKey)
		return preview, err
	}
	preview, count, err := latestRunPreviewForAssistantSequence(artifacts, assistantSequenceNo, "")
	if err != nil || count != 1 {
		if err != nil {
			return nil, err
		}
		preview, count, err = latestRunPreviewForAnySequence(artifacts, "")
		if err != nil || count != 1 {
			return nil, err
		}
	}
	return preview, nil
}

func latestRunPreviewForAnySequence(artifacts []model.AgentRunArtifact, panelKey string) (*workerpkg.PublishedPreview, int, error) {
	targetKey := normalizeApprovalPreviewPanelKey(panelKey)
	matchCount := 0
	var firstMatch *workerpkg.PublishedPreview
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != workerpkg.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		var payload workerpkg.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			return nil, 0, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && normalizeApprovalPreviewPanelKey(payload.PanelKey) != targetKey {
			continue
		}
		matchCount++
		if firstMatch == nil {
			previewCopy := payload
			firstMatch = &previewCopy
		}
		if targetKey != "" {
			return firstMatch, matchCount, nil
		}
	}
	if targetKey == "" && matchCount != 1 {
		return nil, matchCount, nil
	}
	return firstMatch, matchCount, nil
}

func latestRunPreviewForAssistantSequence(artifacts []model.AgentRunArtifact, assistantSequenceNo int, panelKey string) (*workerpkg.PublishedPreview, int, error) {
	if assistantSequenceNo <= 0 {
		return nil, 0, nil
	}
	targetKey := normalizeApprovalPreviewPanelKey(panelKey)
	matchCount := 0
	var firstMatch *workerpkg.PublishedPreview
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != workerpkg.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		if artifactAssistantMessageSequenceNo(artifact) != assistantSequenceNo {
			continue
		}
		var payload workerpkg.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			return nil, 0, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && normalizeApprovalPreviewPanelKey(payload.PanelKey) != targetKey {
			continue
		}
		matchCount++
		if targetKey != "" {
			return &payload, matchCount, nil
		}
		if firstMatch == nil {
			previewCopy := payload
			firstMatch = &previewCopy
		}
	}
	if targetKey == "" && matchCount == 1 {
		return firstMatch, matchCount, nil
	}
	return nil, matchCount, nil
}

func nextUnappliedApprovedPreview(artifacts []model.AgentRunArtifact) (*model.AgentRunArtifact, *model.ApprovedRunPreview, error) {
	applied := make(map[string]bool)
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreviewApplied || artifact.InlineContent == nil {
			continue
		}
		var marker model.AppliedApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
			return nil, nil, fmt.Errorf("parse applied approved preview artifact: %w", err)
		}
		if strings.TrimSpace(marker.ApprovedArtifactID) != "" {
			applied[strings.TrimSpace(marker.ApprovedArtifactID)] = true
		}
	}

	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		if applied[artifact.ID] {
			continue
		}
		var preview model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
			return nil, nil, fmt.Errorf("parse approved preview artifact: %w", err)
		}
		if approvedPreviewDebugEnabled() {
			slog.Info("selected approved preview for application",
				"artifact_id", artifact.ID,
				"sequence_no", artifact.SequenceNo,
				"phase", strings.TrimSpace(preview.Phase),
				"panel_key", strings.TrimSpace(preview.PanelKey),
				"format", strings.TrimSpace(preview.Format),
				"source_message_id", strings.TrimSpace(preview.SourceMessageID),
				"content_preview", previewDebugSnippet(preview.Content, 1600),
			)
		}
		return &artifact, &preview, nil
	}
	return nil, nil, nil
}

func decodeApprovedTaskPlanPreviewContent(raw json.RawMessage) (model.OrchestrationProposal, error) {
	var proposal model.OrchestrationProposal
	var payload map[string]json.RawMessage
	normalized, err := workerpkg.NormalizeTaskPlanPreviewContent(raw)
	if err != nil {
		if !decodeLooseApprovedTaskPlanPayload(raw, &payload) {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview normalization failed during apply",
					"raw_preview", previewDebugSnippet(raw, 1600),
					"error", err,
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}")
		}
	} else if err := json.Unmarshal(normalized, &payload); err != nil {
		if approvedPreviewDebugEnabled() {
			slog.Error("approved task plan preview payload unmarshal failed during apply",
				"normalized_preview", previewDebugSnippet(normalized, 1600),
				"error", err,
			)
		}
		return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}")
	}

	proposal.EpicID = decodeLooseJSONString(payload["epic_id"])
	proposal.Summary = decodeLooseJSONString(payload["summary"])
	proposal.SpecVersionID = decodeLooseJSONString(payload["spec_version_id"])
	proposal.OpenQuestions = decodeLooseJSONStringArray(payload["open_questions"])
	proposal.Risks = decodeLooseJSONStringArray(payload["risks"])
	if verticalCoverage, ok := decodeLooseVerticalCoverage(payload["vertical_coverage"]); ok {
		proposal.VerticalCoverage = verticalCoverage
	}

	var taskItems []json.RawMessage
	if err := json.Unmarshal(payload["proposed_tasks"], &taskItems); err != nil {
		if approvedPreviewDebugEnabled() {
			slog.Error("approved task plan preview proposed_tasks decode failed during apply",
				"normalized_preview", previewDebugSnippet(normalized, 1600),
				"error", err,
			)
		}
		return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}")
	}
	proposal.ProposedTasks = make([]model.ProposedTask, 0, len(taskItems))
	for index, item := range taskItems {
		task, ok := decodeLooseApprovedProposedTask(item)
		if !ok {
			if approvedPreviewDebugEnabled() {
				slog.Error("approved task plan preview item decode failed during apply",
					"task_index", index,
					"task_preview", previewDebugSnippet(item, 1200),
				)
			}
			return proposal, fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}")
		}
		proposal.ProposedTasks = append(proposal.ProposedTasks, task)
	}
	if approvedPreviewDebugEnabled() {
		slog.Info("decoded approved task plan preview",
			"summary_preview", truncateString(strings.TrimSpace(proposal.Summary), 240),
			"task_count", len(proposal.ProposedTasks),
		)
	}
	return proposal, nil
}

func decodeLooseApprovedTaskPlanPayload(raw json.RawMessage, payload *map[string]json.RawMessage) bool {
	if payload == nil {
		return false
	}
	if err := json.Unmarshal(raw, payload); err == nil {
		_, hasSummary := (*payload)["summary"]
		_, hasTasks := (*payload)["proposed_tasks"]
		return hasSummary && hasTasks
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return false
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" || !json.Valid([]byte(encoded)) {
		return false
	}
	if err := json.Unmarshal([]byte(encoded), payload); err != nil {
		return false
	}
	_, hasSummary := (*payload)["summary"]
	_, hasTasks := (*payload)["proposed_tasks"]
	return hasSummary && hasTasks
}

func decodeLooseApprovedProposedTask(raw json.RawMessage) (model.ProposedTask, bool) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.ProposedTask{}, false
	}

	task := model.ProposedTask{
		Ref:                decodeLooseJSONString(payload["ref"]),
		Name:               firstNonEmptyString(decodeLooseJSONString(payload["name"]), decodeLooseJSONString(payload["title"])),
		Description:        decodeLooseJSONString(payload["description"]),
		TaskType:           firstNonEmptyString(decodeLooseJSONString(payload["task_type"]), decodeLooseJSONString(payload["type"])),
		SliceType:          decodeLooseJSONString(payload["slice_type"]),
		AcceptanceCriteria: decodeLooseJSONStringArray(payload["acceptance_criteria"]),
		DependencyRefs:     decodeLooseJSONStringArray(payload["dependency_refs"]),
	}
	if estimate, ok := decodeLooseJSONInt(payload["estimate"]); ok {
		task.Estimate = &estimate
	}
	if priority := decodeLooseJSONString(payload["priority"]); priority != "" {
		task.Priority = &priority
	}
	if assignAgentID := decodeLooseJSONString(payload["assign_agent_id"]); assignAgentID != "" {
		task.AssignAgentID = &assignAgentID
	}
	if sourceRefs, ok := decodeLoosePlanningSourceRefs(payload["source_refs"]); ok {
		task.SourceRefs = sourceRefs
	}
	if brief, ok := decodeLooseTaskImplementationBrief(payload["implementation_brief"]); ok {
		task.ImplementationBrief = brief
	}
	return task, true
}

func decodeLooseJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return ""
}

func decodeLooseJSONStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return filtered
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}
	var mixed []any
	if err := json.Unmarshal(raw, &mixed); err == nil {
		filtered := make([]string, 0, len(mixed))
		for _, item := range mixed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				filtered = append(filtered, text)
			}
		}
		return filtered
	}
	return nil
}

func decodeLooseJSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func decodeLoosePlanningSourceRefs(raw json.RawMessage) ([]model.PlanningSourceRef, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var refs []model.PlanningSourceRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return refs, true
	}
	return nil, false
}

func decodeLooseTaskImplementationBrief(raw json.RawMessage) (*model.TaskImplementationBrief, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}

	brief := &model.TaskImplementationBrief{
		Approach:       decodeLooseJSONString(payload["approach"]),
		FilesToModify:  decodeLooseFileChanges(payload["files_to_modify"]),
		TestStrategy:   decodeLooseJSONText(payload["test_strategy"]),
		VerticalLayers: decodeLooseJSONStringArray(payload["vertical_layers"]),
		DependsOnFiles: decodeLooseJSONStringArray(payload["depends_on_files"]),
	}
	return brief, true
}

func decodeLooseFileChanges(raw json.RawMessage) []model.FileChange {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	changes := make([]model.FileChange, 0, len(items))
	for _, item := range items {
		var change model.FileChange
		if err := json.Unmarshal(item, &change); err != nil {
			continue
		}
		if strings.TrimSpace(change.Path) == "" {
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func decodeLooseJSONText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				filtered = append(filtered, item)
			}
		}
		return strings.Join(filtered, "\n")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		return compact.String()
	}
	return strings.TrimSpace(string(raw))
}

func decodeLooseVerticalCoverage(raw json.RawMessage) ([]model.VerticalCoverageEntry, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var entries []model.VerticalCoverageEntry
	if err := json.Unmarshal(raw, &entries); err == nil {
		return entries, true
	}
	return nil, false
}

func approvedPreviewDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_PREVIEW_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func previewDebugSnippet(raw json.RawMessage, max int) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		trimmed = compact.String()
	}
	if max > 0 && len(trimmed) > max {
		return trimmed[:max] + "...(truncated)"
	}
	return trimmed
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		return value[:max] + "...(truncated)"
	}
	return value
}

func decodeApprovedMarkdownPreviewContent(raw json.RawMessage, previewLabel string) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}

	var markdown string
	if err := json.Unmarshal(raw, &markdown); err != nil {
		return "", fmt.Errorf("%s content must be a markdown string", previewLabel)
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", fmt.Errorf("%s content is empty; publish a non-empty markdown draft before requesting approval", previewLabel)
	}
	return markdown, nil
}

func (a *AgentRunActivities) applyApprovedPRDPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved PRD preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved PRD preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved PRD preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureEpicSpecDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			ActorID:     runActorID(state.run),
			TargetType:  "epic",
			TargetID:    state.epic.ID,
		}, "pm.approve_epic_spec", json.RawMessage(`{}`)); err != nil {
			return err
		}
	} else {
		savedContent, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown))
		if err != nil {
			return err
		}
		label := "Approved Spec"
		version, err := a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), savedContent.Content, savedContent.ContentText, &label, "manual", len(strings.Fields(savedContent.ContentText)))
		if err != nil {
			return err
		}
		state.epic.SpecDocumentID = &doc.ID
		state.epic.ApprovedSpecVersionID = &version.ID
		if err := a.epicRepo.Update(ctx, state.epic); err != nil {
			return err
		}
	}

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved PRD preview")
	}
	state.epic = &epicWithStats.Epic
	input.SpecDocumentID = strings.TrimSpace(derefString(state.epic.SpecDocumentID))
	input.SpecVersionID = strings.TrimSpace(derefString(state.epic.ApprovedSpecVersionID))
	return nil
}

func (a *AgentRunActivities) applyApprovedTaskPlanPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task plan preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatJSON {
		return fmt.Errorf("approved task plan preview must use format %q", workerpkg.PreviewFormatJSON)
	}
	if a.commandExecutor == nil {
		return fmt.Errorf("planner commands are not available")
	}

	proposal, err := decodeApprovedTaskPlanPreviewContent(preview.Content)
	if err != nil {
		return err
	}
	if proposal.EpicID == "" {
		proposal.EpicID = state.epic.ID
	}
	if proposal.SpecVersionID == "" {
		proposal.SpecVersionID = strings.TrimSpace(firstNonEmptyString(input.SpecVersionID, derefString(state.epic.ApprovedSpecVersionID)))
	}
	if err := validatePlanningProposalTasks(proposal.ProposedTasks); err != nil {
		return err
	}

	output, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: state.run.WorkspaceID,
		ActorID:     runActorID(state.run),
		TargetType:  "epic",
		TargetID:    state.epic.ID,
	}, "pm.create_task_batch", mustJSON(map[string]any{
		"tasks": proposal.ProposedTasks,
	}))
	if err != nil {
		return err
	}

	var result workerpkg.CreateTaskBatchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("parse created task batch: %w", err)
	}

	tasks, err := a.epicRepo.ListTasks(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	state.epicTasks = tasks

	epicWithStats, err := a.epicRepo.GetByID(ctx, state.epic.ID)
	if err != nil {
		return err
	}
	if epicWithStats == nil {
		return fmt.Errorf("epic not found after applying approved task plan")
	}
	state.epic = &epicWithStats.Epic
	state.epic.LastPlanningRunID = &state.run.ID
	if err := a.epicRepo.Update(ctx, state.epic); err != nil {
		return err
	}

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:               model.PlanningStagePlanTasks,
		SpecDocumentID:      strings.TrimSpace(firstNonEmptyString(input.SpecDocumentID, derefString(state.epic.SpecDocumentID))),
		SpecVersionID:       proposal.SpecVersionID,
		PlanningMethodology: input.PlanningMethodology,
		Summary:             strings.TrimSpace(proposal.Summary),
		Risks:               append([]string(nil), proposal.Risks...),
		OpenQuestions:       append([]string(nil), proposal.OpenQuestions...),
		Proposal:            &proposal,
	})

	createdCount := len(result.Tasks)
	summaryText := fmt.Sprintf("Applied the approved task plan and created %d tasks.", createdCount)
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}

func (a *AgentRunActivities) applyApprovedTaskDocPreview(ctx context.Context, state *resolvedRunState, input *planningRunInput, preview *model.ApprovedRunPreview) error {
	if preview == nil {
		return fmt.Errorf("approved task planning document preview is required")
	}
	if strings.TrimSpace(preview.Format) != workerpkg.PreviewFormatMarkdown {
		return fmt.Errorf("approved task planning document preview must use format %q", workerpkg.PreviewFormatMarkdown)
	}
	if state.task == nil {
		return fmt.Errorf("approved task planning document preview requires a task target")
	}

	markdown, err := decodeApprovedMarkdownPreviewContent(preview.Content, "approved task planning document preview")
	if err != nil {
		return err
	}

	doc, err := a.ensureTaskPlanDocument(ctx, state, runActorID(state.run))
	if err != nil {
		return err
	}
	if a.commandExecutor != nil {
		if _, err := a.commandExecutor.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: state.run.WorkspaceID,
			TargetType:  "document",
			TargetID:    doc.ID,
		}, "docs.write_document_content", mustJSON(map[string]any{
			"document_id": doc.ID,
			"content":     markdown,
		})); err != nil {
			return err
		}
	} else {
		if _, err := a.docsContentRepo.Upsert(ctx, doc.ID, tiptap.MarkdownToJSON(markdown)); err != nil {
			return err
		}
		if err := a.ensureTaskPlanLink(ctx, state.run.WorkspaceID, doc.ID, state.task.ID, runActorID(state.run)); err != nil {
			return err
		}
	}

	if content, err := a.docsContentRepo.GetByDocumentID(ctx, doc.ID); err == nil && content != nil {
		label := "Approved Task Plan"
		_, _ = a.docsVersionRepo.Create(ctx, doc.ID, runActorID(state.run), content.Content, content.ContentText, &label, "manual", len(strings.Fields(content.ContentText)))
	}

	state.task.PlanDocumentID = &doc.ID
	if err := a.taskRepo.Update(ctx, state.task); err != nil {
		return err
	}
	input.PlanDocumentID = doc.ID

	state.run.OutputSummary, _ = json.Marshal(planningRunSummary{
		Stage:          model.PlanningStageTaskPlanDoc,
		PlanDocumentID: doc.ID,
		Summary:        strings.TrimSpace(preview.ApprovalSummary),
	})

	summaryText := "Persisted the approved task plan to Docs and linked it to the task."
	if _, err := a.createRunMessage(ctx, state.run, "assistant", "assistant_turn", summaryText, nil, nil, nil, nil); err != nil {
		return err
	}

	completedAt := time.Now()
	state.run.Status = model.AgentRunStatusCompleted
	state.run.PauseReason = model.AgentRunPauseReasonNone
	state.run.CompletedAt = &completedAt
	state.run.ExecutionStage = strPtr("completed")
	state.run.LastHeartbeatAt = &completedAt
	if err := a.runRepo.Update(ctx, state.run); err != nil {
		return err
	}
	a.runRepo.Notify(ctx, state.run)
	return a.markAgentIdle(ctx, state.run.WorkspaceID, state.run.AgentID, state.run.TokensUsed)
}
