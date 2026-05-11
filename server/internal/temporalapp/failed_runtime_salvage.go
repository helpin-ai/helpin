package temporalapp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func (a *AgentRunActivities) salvageFailedRuntimeStateFromSnapshotStore(ctx context.Context, state *resolvedRunState) {
	if a == nil || state == nil || state.run == nil {
		return
	}
	snapshot, err := a.loadCodingSessionStreamSnapshot(ctx, state.run)
	if err != nil {
		slog.WarnContext(ctx, "load failed run stream snapshot for salvage failed",
			"run_id", state.run.ID,
			"workspace_id", state.run.WorkspaceID,
			"error", err,
		)
		return
	}
	if err := a.salvageFailedRuntimeState(ctx, state, snapshot); err != nil {
		slog.WarnContext(ctx, "salvage failed run stream state failed",
			"run_id", state.run.ID,
			"workspace_id", state.run.WorkspaceID,
			"error", err,
		)
	}
}

func (a *AgentRunActivities) salvageFailedRuntimeState(ctx context.Context, state *resolvedRunState, snapshot *model.CodingSessionStreamSnapshot) error {
	if a == nil || a.artifactRepo == nil || state == nil || state.run == nil || snapshot == nil {
		return nil
	}

	existing, err := a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
	if err != nil {
		return err
	}

	if snapshot.CurrentPlan != nil && !hasMatchingRunPlanArtifact(existing, snapshot.CurrentPlan) {
		if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, model.AgentRunArtifactTypeRunPlan, "json", snapshot.CurrentPlan, failureSalvageMetadata()); err != nil {
			return err
		}
		existing, err = a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return err
		}
	}

	for _, preview := range publishedPreviewsFromSnapshot(snapshot) {
		if hasMatchingRunPreviewArtifact(existing, preview) {
			continue
		}
		if _, err := a.appendRunArtifactWithMetadata(ctx, state.run, workerpkg.RunPreviewArtifactType, "json", preview, failureSalvageMetadata()); err != nil {
			return err
		}
		existing, err = a.artifactRepo.ListByRun(ctx, state.run.WorkspaceID, state.run.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func failureSalvageMetadata() json.RawMessage {
	return json.RawMessage(`{"source":"failure_salvage"}`)
}

func publishedPreviewsFromSnapshot(snapshot *model.CodingSessionStreamSnapshot) []workerpkg.PublishedPreview {
	if snapshot == nil {
		return nil
	}
	invocations := make([]model.ToolInvocation, 0, len(snapshot.LiveTurnSegments))
	for _, segment := range snapshot.LiveTurnSegments {
		if strings.TrimSpace(segment.Kind) != "tool_call" || segment.ToolCall == nil {
			continue
		}
		if strings.TrimSpace(segment.ToolCall.Status) == "failed" {
			continue
		}
		if strings.TrimSpace(segment.ToolCall.ArgsText) == "" {
			continue
		}
		invocations = append(invocations, model.ToolInvocation{
			ToolName: strings.TrimSpace(segment.ToolCall.ToolName),
			Input:    json.RawMessage(segment.ToolCall.ArgsText),
		})
	}
	if snapshot.LiveAssistantMessage != nil {
		for _, toolCall := range snapshot.LiveAssistantMessage.ToolCalls {
			if strings.TrimSpace(toolCall.Status) == "failed" || strings.TrimSpace(toolCall.ArgsText) == "" {
				continue
			}
			invocations = append(invocations, model.ToolInvocation{
				ToolName: strings.TrimSpace(toolCall.ToolName),
				Input:    json.RawMessage(toolCall.ArgsText),
			})
		}
	}

	previews := workerpkg.ExtractPublishedPreviews(invocations)
	if len(previews) <= 1 {
		return previews
	}
	indexByPanel := map[string]int{}
	deduped := make([]workerpkg.PublishedPreview, 0, len(previews))
	for _, preview := range previews {
		panelKey := strings.TrimSpace(preview.PanelKey)
		if index, ok := indexByPanel[panelKey]; ok {
			deduped[index] = preview
			continue
		}
		indexByPanel[panelKey] = len(deduped)
		deduped = append(deduped, preview)
	}
	return deduped
}

func hasMatchingRunPlanArtifact(artifacts []model.AgentRunArtifact, plan *model.CodingSessionRunPlan) bool {
	if plan == nil {
		return true
	}
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeRunPlan || artifact.InlineContent == nil {
			continue
		}
		var existing model.CodingSessionRunPlan
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &existing); err != nil {
			continue
		}
		if sameCodingSessionRunPlan(&existing, plan) {
			return true
		}
	}
	return false
}

func hasMatchingRunPreviewArtifact(artifacts []model.AgentRunArtifact, preview workerpkg.PublishedPreview) bool {
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != workerpkg.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		var existing workerpkg.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &existing); err != nil {
			continue
		}
		if strings.TrimSpace(existing.PanelKey) == strings.TrimSpace(preview.PanelKey) &&
			strings.TrimSpace(existing.Title) == strings.TrimSpace(preview.Title) &&
			strings.TrimSpace(existing.Format) == strings.TrimSpace(preview.Format) &&
			existing.Replace == preview.Replace &&
			string(existing.Content) == string(preview.Content) {
			return true
		}
	}
	return false
}

func sameCodingSessionRunPlan(a, b *model.CodingSessionRunPlan) bool {
	if a == nil || b == nil {
		return a == b
	}
	if strings.TrimSpace(a.Note) != strings.TrimSpace(b.Note) || len(a.Plan) != len(b.Plan) {
		return false
	}
	for index := range a.Plan {
		if strings.TrimSpace(a.Plan[index].Step) != strings.TrimSpace(b.Plan[index].Step) ||
			strings.TrimSpace(a.Plan[index].Status) != strings.TrimSpace(b.Plan[index].Status) {
			return false
		}
	}
	return true
}
