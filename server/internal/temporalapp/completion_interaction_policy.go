package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) enforceCompletionInteractionPolicy(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage) error {
	if a == nil || state == nil || state.run == nil {
		return nil
	}

	requiredKinds := completionRequiredInteractionKinds(state.skillPolicy)
	if len(requiredKinds) == 0 {
		return nil
	}
	if a.interactionRepo == nil {
		kinds := sortedCompletionInteractionKinds(requiredKinds)
		return fmt.Errorf("run cannot complete because active skills require one of [%s] before completion, but interaction storage is unavailable", strings.Join(kinds, ", "))
	}

	interactions, err := a.interactionRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return fmt.Errorf("verify completion interaction requirements: %w", err)
	}
	currentAssistantSequenceNo := 0
	if assistantMessage != nil {
		currentAssistantSequenceNo = assistantMessage.SequenceNo
	}
	for _, interaction := range interactions {
		if currentAssistantSequenceNo > 0 {
			if interaction.AssistantMessageSequenceNo == nil || *interaction.AssistantMessageSequenceNo != currentAssistantSequenceNo {
				continue
			}
		}
		if _, ok := requiredKinds[strings.TrimSpace(interaction.InteractionKind)]; ok {
			if err := a.validateApprovalInteractionPreviewContract(ctx, state, interaction, currentAssistantSequenceNo); err != nil {
				return err
			}
			return nil
		}
	}
	if allowsTerminalCleanImplementationReview(state, assistantMessage) {
		return nil
	}
	if allowsPostApprovalImplementationCompletion(interactions, currentAssistantSequenceNo) {
		return nil
	}
	allowed, err := a.allowsCompletionAfterAppliedApprovedPreview(ctx, state)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	kinds := sortedCompletionInteractionKinds(requiredKinds)
	return fmt.Errorf("run cannot complete because active skills require one of [%s] before completion", strings.Join(kinds, ", "))
}

func (a *AgentRunActivities) allowsCompletionAfterAppliedApprovedPreview(ctx context.Context, state *resolvedRunState) (bool, error) {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || state.agent == nil {
		return false, nil
	}
	if state.run.InvocationMode != model.InvocationModeInteractive {
		return false, nil
	}
	switch strings.TrimSpace(state.agent.EffectivePresetKey()) {
	case model.AgentPresetEpicPlanner, model.AgentPresetTaskPlanner:
	default:
		return false, nil
	}

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return false, fmt.Errorf("verify applied approved previews: %w", err)
	}
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreviewApplied || artifact.InlineContent == nil {
			continue
		}
		var marker model.AppliedApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &marker); err != nil {
			return false, fmt.Errorf("parse approved preview applied artifact: %w", err)
		}
		if appliedPreviewCompletionActionAllowed(state, marker) {
			return true, nil
		}
	}
	return false, nil
}

func appliedPreviewCompletionActionAllowed(state *resolvedRunState, marker model.AppliedApprovedRunPreview) bool {
	if state == nil || state.run == nil || state.agent == nil {
		return false
	}
	switch strings.TrimSpace(marker.Action) {
	case "persist_task_doc":
		return strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetTaskPlanner &&
			strings.TrimSpace(state.run.TargetType) == "task"
	case "create_tasks":
		return strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetEpicPlanner &&
			strings.TrimSpace(state.run.TargetType) == "epic"
	default:
		return false
	}
}

func (a *AgentRunActivities) validateApprovalInteractionPreviewContract(ctx context.Context, state *resolvedRunState, interaction model.AgentRunInteraction, currentAssistantSequenceNo int) error {
	if a == nil || state == nil || state.run == nil || a.artifactRepo == nil {
		return nil
	}
	interactionKind := strings.TrimSpace(interaction.InteractionKind)
	if interactionKind != model.AgentRunInteractionKindReviewCheckpoint && interactionKind != model.AgentRunInteractionKindApprovalRequest {
		return nil
	}

	var approval model.ApprovalRequest
	if err := json.Unmarshal(interaction.RequestPayload, &approval); err != nil {
		return fmt.Errorf("parse approval payload: %w", err)
	}
	approval.PreviewPanelKey = normalizeApprovalPreviewPanelKey(approval.PreviewPanelKey)

	artifacts, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return fmt.Errorf("verify approval preview artifacts: %w", err)
	}
	matchCount := 0
	sameTurnPreviewCount := 0
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != workerpkg.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		if currentAssistantSequenceNo > 0 && artifactAssistantMessageSequenceNo(artifact) != currentAssistantSequenceNo {
			continue
		}
		sameTurnPreviewCount++
		var preview workerpkg.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
			return fmt.Errorf("parse approval preview artifact: %w", err)
		}
		if approval.PreviewPanelKey != "" && normalizeApprovalPreviewPanelKey(preview.PanelKey) != approval.PreviewPanelKey {
			continue
		}
		matchCount++
		if approval.PreviewPanelKey != "" {
			return nil
		}
	}
	if approval.PreviewPanelKey != "" {
		if !isCanonicalApprovalPreviewPanelKey(approval.PreviewPanelKey) && sameTurnPreviewCount == 1 {
			return nil
		}
		return fmt.Errorf("%s requires a same-turn %s preview before requesting approval", interactionKind, approval.PreviewPanelKey)
	}
	if matchCount == 1 {
		return nil
	}
	if matchCount > 1 {
		return fmt.Errorf("%s requires preview_panel_key when multiple same-turn previews exist", interactionKind)
	}
	return fmt.Errorf("%s requires a same-turn preview before requesting approval", interactionKind)
}

func completionRequiredInteractionKinds(policy workerpkg.SkillPolicy) map[string]struct{} {
	if len(policy.CompletionRequiresInteractionKinds) == 0 {
		return nil
	}

	declaredContracts := make(map[string]struct{}, len(policy.InteractionContracts))
	for _, contract := range workerpkg.NormalizeInteractionContracts(policy.InteractionContracts) {
		if kind := strings.TrimSpace(contract.Kind); kind != "" {
			declaredContracts[kind] = struct{}{}
		}
	}

	out := make(map[string]struct{}, len(policy.CompletionRequiresInteractionKinds))
	for _, value := range policy.CompletionRequiresInteractionKinds {
		normalized := strings.TrimSpace(value)
		switch normalized {
		case model.AgentRunInteractionKindRequestUserInput:
			out[model.AgentRunInteractionKindRequestUserInput] = struct{}{}
		case model.AgentRunInteractionKindApprovalRequest:
			out[model.AgentRunInteractionKindApprovalRequest] = struct{}{}
		case model.AgentRunInteractionKindReviewCheckpoint:
			out[model.AgentRunInteractionKindReviewCheckpoint] = struct{}{}
		case model.AgentRunInteractionKindCommandExecutionApproval,
			model.AgentRunInteractionKindFileChangeApproval,
			model.AgentRunInteractionKindPermissionsApproval,
			model.AgentRunInteractionKindAuthRequired:
			out[normalized] = struct{}{}
		default:
			if _, ok := declaredContracts[normalized]; ok {
				out[normalized] = struct{}{}
			}
		}
	}
	return out
}

func sortedCompletionInteractionKinds(values map[string]struct{}) []string {
	kinds := make([]string, 0, len(values))
	for kind := range values {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func (a *AgentRunActivities) synthesizeCompletionInteractionFallback(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage) (*model.ReviewCheckpointRequest, *workerpkg.UserInputRequest, error) {
	if a == nil || state == nil || state.run == nil || assistantMessage == nil {
		return nil, nil, nil
	}
	switch strings.TrimSpace(state.run.RuntimeKind) {
	case "codex", "opencode":
	default:
		return nil, nil, nil
	}

	requiredKinds := completionRequiredInteractionKinds(state.skillPolicy)
	if len(requiredKinds) == 0 {
		return nil, nil, nil
	}

	content := strings.TrimSpace(assistantMessage.Content)
	if content == "" {
		return nil, nil, nil
	}

	if _, ok := requiredKinds[model.AgentRunInteractionKindReviewCheckpoint]; ok {
		if a.interactionRepo != nil {
			interactions, err := a.interactionRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
			if err != nil {
				return nil, nil, err
			}
			if allowsPostApprovalImplementationCompletion(interactions, assistantMessage.SequenceNo) {
				return nil, nil, nil
			}
		}
		if terminalReview := terminalCleanImplementationReviewRequest(state, assistantMessage); terminalReview != nil {
			metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)
			if a.artifactRepo != nil {
				if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(terminalReview, assistantMessage.SequenceNo); reviewFindings != nil {
					if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, metadata); err != nil {
						return nil, nil, err
					}
				}
			}
			return nil, nil, nil
		}
		reviewRequest := synthesizedReviewCheckpointFromAssistantMessage(state, assistantMessage)
		if reviewRequest == nil {
			return nil, nil, nil
		}
		metadata := buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo)
		if a.artifactRepo != nil {
			if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeHumanApprovalRequest, "json", reviewRequest, metadata); err != nil {
				return nil, nil, err
			}
			if reviewFindings := reviewFindingsArtifactFromReviewCheckpointRequest(reviewRequest, assistantMessage.SequenceNo); reviewFindings != nil {
				if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeReviewFindings, "json", reviewFindings, metadata); err != nil {
					return nil, nil, err
				}
			}
		}
		interaction, err := a.persistHumanApprovalInteraction(ctx, state, model.AgentRunInteractionKindReviewCheckpoint, reviewRequest.Title, reviewRequest.Summary, reviewRequest, metadata, assistantMessage.SequenceNo)
		if err != nil {
			return nil, nil, err
		}
		if interaction != nil {
			a.maybeNotifyAgentAttentionRequired(ctx, state, interaction)
			a.publishCodingSessionInteractionEvent(state.run, interaction)
		} else {
			a.publishCodingSessionEvent(state.run, "approval.requested", map[string]any{
				"content": reviewRequest,
			})
		}
		return reviewRequest, nil, nil
	}

	return nil, nil, nil
}

func allowsTerminalCleanImplementationReview(state *resolvedRunState, assistantMessage *model.AgentRunMessage) bool {
	return terminalCleanImplementationReviewRequest(state, assistantMessage) != nil
}

func allowsPostApprovalImplementationCompletion(interactions []model.AgentRunInteraction, currentAssistantSequenceNo int) bool {
	if len(interactions) == 0 {
		return false
	}
	for idx := len(interactions) - 1; idx >= 0; idx-- {
		interaction := interactions[idx]
		if strings.TrimSpace(interaction.Status) != model.AgentRunInteractionStatusResolved {
			continue
		}
		if strings.TrimSpace(interaction.InteractionKind) != model.AgentRunInteractionKindReviewCheckpoint {
			continue
		}
		var response model.ReviewCheckpointResponse
		if err := json.Unmarshal(interaction.ResponsePayload, &response); err != nil {
			return false
		}
		if strings.TrimSpace(response.Decision) != "approve" {
			return false
		}
		if currentAssistantSequenceNo > 0 && interaction.AssistantMessageSequenceNo != nil && *interaction.AssistantMessageSequenceNo >= currentAssistantSequenceNo {
			return false
		}
		return true
	}
	return false
}

func terminalCleanImplementationReviewRequest(state *resolvedRunState, assistantMessage *model.AgentRunMessage) *model.ReviewCheckpointRequest {
	if state == nil || state.agent == nil || assistantMessage == nil {
		return nil
	}
	if strings.TrimSpace(state.agent.EffectivePresetKey()) != model.AgentPresetReviewAgent {
		return nil
	}
	request := synthesizedReviewCheckpointFromAssistantMessage(state, assistantMessage)
	if request == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(request.Phase), "implementation") {
		return nil
	}
	if len(request.Findings) > 0 {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(request.OverallCorrectness), "correct") {
		return nil
	}
	return request
}

func synthesizedReviewCheckpointFromAssistantMessage(state *resolvedRunState, assistantMessage *model.AgentRunMessage) *model.ReviewCheckpointRequest {
	if assistantMessage == nil {
		return nil
	}
	content := strings.TrimSpace(assistantMessage.Content)
	if content == "" {
		return nil
	}

	if structured, ok := parseStructuredReviewApprovalRequest(state.skillPolicy, strings.TrimSpace(state.run.RuntimeKind), content); ok {
		if strings.TrimSpace(structured.Phase) == "" {
			if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) == model.AgentPresetReviewAgent {
				structured.Phase = "review_findings"
			} else {
				structured.Phase = "review"
			}
		}
		return structured
	}

	title := "Review findings"
	phase := "review_findings"
	if state != nil && state.agent != nil && strings.TrimSpace(state.agent.EffectivePresetKey()) != model.AgentPresetReviewAgent {
		title = "Checkpoint review"
		phase = "review"
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if first != "" && len(first) <= 80 {
			title = first
		}
	}

	return &model.ReviewCheckpointRequest{
		Phase:   phase,
		Title:   title,
		Summary: content,
	}
}

func parseStructuredReviewApprovalRequest(policy workerpkg.SkillPolicy, runtimeKind, content string) (*model.ReviewCheckpointRequest, bool) {
	payload, prose := extractStructuredReviewPayload(policy, runtimeKind, content)
	if strings.TrimSpace(payload) == "" {
		return nil, false
	}

	var req model.ReviewCheckpointRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return nil, false
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Phase = strings.TrimSpace(req.Phase)
	req.PreviewPanelKey = normalizeApprovalPreviewPanelKey(req.PreviewPanelKey)
	req.OverallCorrectness = strings.TrimSpace(req.OverallCorrectness)
	req.OverallExplanation = strings.TrimSpace(req.OverallExplanation)
	normalizeStructuredReviewFindings(req.Findings)
	for idx := range req.Findings {
		req.Findings[idx].Title = strings.TrimSpace(req.Findings[idx].Title)
		req.Findings[idx].Body = strings.TrimSpace(req.Findings[idx].Body)
		req.Findings[idx].Priority = strings.TrimSpace(req.Findings[idx].Priority)
		req.Findings[idx].CodeLocation = strings.TrimSpace(req.Findings[idx].CodeLocation)
	}

	if req.Title == "" {
		req.Title = "Review findings"
	}
	if req.Summary == "" {
		req.Summary = prose
	}
	if req.Summary == "" && len(req.Findings) == 0 && req.OverallCorrectness == "" && req.OverallExplanation == "" {
		return nil, false
	}
	return &req, true
}

func extractStructuredReviewPayload(policy workerpkg.SkillPolicy, runtimeKind, content string) (payload string, prose string) {
	label := workerpkg.ReviewCheckpointFencedBlockLabel(policy, runtimeKind)
	pattern := regexp.MustCompile(fmt.Sprintf("(?s)```%s\\s*\\n(.*?)\\n```", regexp.QuoteMeta(label)))
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return "", strings.TrimSpace(content)
	}
	last := matches[len(matches)-1]
	if len(last) < 4 {
		return "", strings.TrimSpace(content)
	}
	payload = strings.TrimSpace(content[last[2]:last[3]])
	prose = strings.TrimSpace(strings.TrimSpace(content[:last[0]]) + "\n" + strings.TrimSpace(content[last[1]:]))
	return payload, prose
}

func normalizeStructuredReviewFindings(findings []model.ReviewFinding) {
	for idx := range findings {
		findings[idx].ID = strings.TrimSpace(findings[idx].ID)
		if findings[idx].ID == "" {
			findings[idx].ID = fmt.Sprintf("finding_%d", idx+1)
		}
	}
}

func (a *AgentRunActivities) retryInvalidCompletionTurn(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage, cause error) (bool, error) {
	if a == nil || state == nil || state.run == nil || a.runMessageRepo == nil {
		return false, nil
	}
	if cause == nil || strings.TrimSpace(cause.Error()) == "" {
		return false, nil
	}

	messages, err := a.runMessageRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if strings.TrimSpace(message.MessageType) == "policy_retry" {
			return false, nil
		}
	}

	retryInstruction := normalizedCompletionRetryInstruction(state, cause)
	instruction := strings.TrimSpace(retryInstruction.Instructions)
	if instruction == "" {
		return false, nil
	}
	if _, err := a.createRunMessage(ctx, state.run, "user", "policy_retry", instruction, nil, nil, nil, nil); err != nil {
		return false, err
	}
	if err := a.persistCompletionRetryRepairState(ctx, state, assistantMessage, cause, retryInstruction); err != nil {
		return false, err
	}
	return true, nil
}

func (a *AgentRunActivities) persistCompletionRetryRepairState(ctx context.Context, state *resolvedRunState, assistantMessage *model.AgentRunMessage, cause error, repairInstruction repairInstruction) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || assistantMessage == nil || cause == nil {
		return nil
	}
	if !state.executionContractActive {
		return nil
	}
	if strings.TrimSpace(repairInstruction.Class) == "" || strings.TrimSpace(repairInstruction.Instructions) == "" {
		return nil
	}
	payload := model.AgentRepairState{
		Source:       "completion_retry",
		RepairClass:  strings.TrimSpace(repairInstruction.Class),
		RepairHint:   strings.TrimSpace(repairInstruction.Instructions),
		ErrorSummary: strings.TrimSpace(cause.Error()),
	}
	_, err := a.appendRunArtifactWithMetadata(
		ctx,
		state.run,
		model.AgentRunArtifactTypeAgentRepairState,
		"json",
		payload,
		buildAssistantSequenceArtifactMetadata(assistantMessage.SequenceNo),
	)
	return err
}

func approvalPreviewRetryInstruction(causeText string) string {
	causeText = strings.TrimSpace(causeText)
	switch {
	case strings.Contains(causeText, "requires preview_panel_key when multiple same-turn previews exist"):
		return "System correction: the previous turn requested approval after publishing multiple same-turn previews but did not include preview_panel_key. Continue from your last assistant message instead of restarting. Do not end with prose only. If you emit request_approval or request_review_checkpoint, publish the intended preview in the same turn and include preview_panel_key so the handoff binds to the correct preview."
	case strings.Contains(causeText, "requires a same-turn ") && strings.Contains(causeText, " preview before requesting approval"):
		requiredPreviewKey := extractRequiredPreviewKeyFromCause(causeText)
		if requiredPreviewKey == "" {
			return "System correction: the previous turn requested approval without binding it to the required same-turn preview. Continue from your last assistant message instead of restarting. Do not end with prose only. Publish the intended preview in the same turn before the approval handoff, and include preview_panel_key when needed so it binds to the correct preview."
		}
		return fmt.Sprintf("System correction: the previous turn requested approval without binding it to the required same-turn %s preview. Continue from your last assistant message instead of restarting. Do not end with prose only. Publish the %s preview in the same turn before the approval handoff, and set preview_panel_key=%q on request_approval or request_review_checkpoint so it binds to the correct preview.", requiredPreviewKey, requiredPreviewKey, requiredPreviewKey)
	default:
		return ""
	}
}

func extractRequiredPreviewKeyFromCause(causeText string) string {
	return extractPreviewKeyAfterMarker(causeText, "requires a same-turn ", " preview before requesting approval")
}

func extractPreviewKeyAfterMarker(content, marker, terminator string) string {
	content = strings.TrimSpace(content)
	if content == "" || marker == "" || terminator == "" {
		return ""
	}
	start := strings.Index(content, marker)
	if start == -1 {
		return ""
	}
	remaining := content[start+len(marker):]
	end := strings.Index(remaining, terminator)
	if end == -1 {
		return ""
	}
	return normalizeApprovalPreviewPanelKey(strings.TrimSpace(remaining[:end]))
}
