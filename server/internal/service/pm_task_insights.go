package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	taskUpdatesDefaultLimit    = 50
	taskUpdatesMaxLimit        = 100
	taskInsightsSourceLimit    = 100
	taskStandingBriefVersion   = "v1"
	taskStandingBriefMaxTokens = 1200
)

var allowedTaskBriefActions = map[string]struct{}{
	"retry_run":           {},
	"reply_to_comment":    {},
	"add_checklist_item":  {},
	"open_related_object": {},
}

type PMTaskInsightsService struct {
	insightsRepo  *repository.PMTaskInsightsRepository
	taskRepo      *repository.PMTaskRepository
	commentRepo   *repository.PMCommentRepository
	activityRepo  *repository.PMActivityRepository
	agentRunRepo  *repository.AgentRunRepository
	agentRepo     *repository.AgentRepository
	gitLinkRepo   *repository.TaskGitLinkRepository
	checklistRepo *repository.PMChecklistItemRepository
	llmProvider   llm.Provider
}

func NewPMTaskInsightsService(
	insightsRepo *repository.PMTaskInsightsRepository,
	taskRepo *repository.PMTaskRepository,
	commentRepo *repository.PMCommentRepository,
	activityRepo *repository.PMActivityRepository,
	agentRunRepo *repository.AgentRunRepository,
	agentRepo *repository.AgentRepository,
	gitLinkRepo *repository.TaskGitLinkRepository,
	checklistRepo *repository.PMChecklistItemRepository,
	llmProvider llm.Provider,
) *PMTaskInsightsService {
	return &PMTaskInsightsService{
		insightsRepo:  insightsRepo,
		taskRepo:      taskRepo,
		commentRepo:   commentRepo,
		activityRepo:  activityRepo,
		agentRunRepo:  agentRunRepo,
		agentRepo:     agentRepo,
		gitLinkRepo:   gitLinkRepo,
		checklistRepo: checklistRepo,
		llmProvider:   llmProvider,
	}
}

func (s *PMTaskInsightsService) task(ctx context.Context, workspaceID, taskID string) (*model.TaskDetail, error) {
	detail, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail == nil || detail.Task.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("task not found")
	}
	return detail, nil
}

func (s *PMTaskInsightsService) ListUpdates(ctx context.Context, workspaceID, taskID, userID, filter, cursor string, limit int) (*model.TaskUpdatesResponse, error) {
	if _, err := s.task(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	if filter == "" {
		filter = model.TaskUpdateFilterAll
	}
	if filter != model.TaskUpdateFilterAll && filter != model.TaskUpdateFilterDiscussion && filter != model.TaskUpdateFilterChanges {
		return nil, fmt.Errorf("invalid updates filter")
	}
	if limit <= 0 {
		limit = taskUpdatesDefaultLimit
	}
	if limit > taskUpdatesMaxLimit {
		limit = taskUpdatesMaxLimit
	}

	entries, err := s.loadUpdateEntries(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	highWater := time.Time{}
	for _, entry := range entries {
		if entry.OccurredAt.After(highWater) {
			highWater = entry.OccurredAt
		}
	}
	readState, err := s.insightsRepo.GetReadState(ctx, workspaceID, taskID, userID)
	if err != nil {
		return nil, err
	}
	unread := 0
	if readState != nil {
		for _, entry := range entries {
			if entry.OccurredAt.After(readState.SeenThrough) && !taskUpdateAuthoredBy(entry, userID) {
				unread++
			}
		}
	}

	if filter == model.TaskUpdateFilterDiscussion {
		entries = filterTaskUpdateEntries(entries, func(entry model.TaskUpdateEntry) bool { return entry.Kind == model.TaskUpdateKindComment })
	} else if filter == model.TaskUpdateFilterChanges {
		entries = filterTaskUpdateEntries(entries, func(entry model.TaskUpdateEntry) bool { return entry.Kind != model.TaskUpdateKindComment })
	}
	if cursor != "" {
		before, err := decodeTaskUpdateCursor(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid updates cursor")
		}
		entries = filterTaskUpdateEntries(entries, func(entry model.TaskUpdateEntry) bool { return entry.OccurredAt.Before(before) })
	}

	response := &model.TaskUpdatesResponse{
		Data:            []model.TaskUpdateEntry{},
		HighWater:       highWater,
		UnreadCount:     unread,
		ReadInitialized: readState != nil,
	}
	if len(entries) > limit {
		response.Data = entries[:limit]
		response.NextCursor = encodeTaskUpdateCursor(entries[limit-1].OccurredAt)
	} else {
		response.Data = entries
	}
	return response, nil
}

func (s *PMTaskInsightsService) UpdateReadState(ctx context.Context, workspaceID, taskID, userID string, req model.UpdateTaskReadStateRequest) (*model.PMTaskUpdateRead, error) {
	if _, err := s.task(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	seenThrough := time.Now().UTC()
	if req.SeenThrough != nil {
		seenThrough = req.SeenThrough.UTC()
	}
	return s.insightsRepo.AdvanceReadState(ctx, workspaceID, taskID, userID, seenThrough, req.InitializeOnly)
}

func (s *PMTaskInsightsService) loadUpdateEntries(ctx context.Context, workspaceID, taskID string) ([]model.TaskUpdateEntry, error) {
	comments, err := s.commentRepo.List(ctx, "task", taskID)
	if err != nil {
		return nil, err
	}
	activity, _, err := s.activityRepo.List(ctx, "task", taskID, model.PMPagination{Page: 1, PerPage: taskInsightsSourceLimit})
	if err != nil {
		return nil, err
	}
	runs, err := s.agentRunRepo.ListByTarget(ctx, workspaceID, "task", taskID)
	if err != nil {
		return nil, err
	}
	gitLinks, err := s.gitLinkRepo.ListByTask(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}

	agentIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		agentIDs = append(agentIDs, run.AgentID)
	}
	agents, err := s.agentRepo.ListByIDs(ctx, workspaceID, agentIDs)
	if err != nil {
		return nil, err
	}
	agentNames := make(map[string]string, len(agents))
	for _, agent := range agents {
		agentNames[agent.ID] = agent.Name
	}

	entries := make([]model.TaskUpdateEntry, 0, len(comments)+len(activity)+len(runs)+len(gitLinks))
	for i := range comments {
		comment := comments[i]
		entries = append(entries, model.TaskUpdateEntry{
			ID: "comment:" + comment.Comment.ID, Kind: model.TaskUpdateKindComment,
			OccurredAt: comment.Comment.CreatedAt, Actor: &comment.Author, Comment: &comment,
		})
	}
	for i := range activity {
		item := activity[i]
		if isCommentLifecycleActivity(item.Activity.Action) {
			continue
		}
		entries = append(entries, model.TaskUpdateEntry{
			ID: "activity:" + item.Activity.ID, Kind: model.TaskUpdateKindChange,
			OccurredAt: item.Activity.CreatedAt, Actor: item.Actor, Activity: &item.Activity,
		})
	}
	for i := range runs {
		run := runs[i]
		occurredAt := run.UpdatedAt
		if run.CompletedAt != nil {
			occurredAt = *run.CompletedAt
		}
		entries = append(entries, model.TaskUpdateEntry{
			ID: "run:" + run.ID, Kind: model.TaskUpdateKindAgentRun,
			OccurredAt: occurredAt, AgentRun: &run, AgentName: agentNames[run.AgentID],
		})
	}
	for i := range gitLinks {
		link := gitLinks[i]
		entries = append(entries, model.TaskUpdateEntry{
			ID: "git:" + link.ID, Kind: model.TaskUpdateKindGit,
			OccurredAt: link.UpdatedAt, GitLink: &link,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].OccurredAt.Equal(entries[j].OccurredAt) {
			return entries[i].ID > entries[j].ID
		}
		return entries[i].OccurredAt.After(entries[j].OccurredAt)
	})
	return entries, nil
}

func isCommentLifecycleActivity(action string) bool {
	return strings.HasPrefix(action, "comment_") || action == "block_commented"
}

func taskUpdateAuthoredBy(entry model.TaskUpdateEntry, userID string) bool {
	if entry.Actor != nil && entry.Actor.ID == userID {
		return true
	}
	return entry.AgentRun != nil && entry.AgentRun.TriggeredByUserID != nil && *entry.AgentRun.TriggeredByUserID == userID
}

func filterTaskUpdateEntries(entries []model.TaskUpdateEntry, keep func(model.TaskUpdateEntry) bool) []model.TaskUpdateEntry {
	filtered := make([]model.TaskUpdateEntry, 0, len(entries))
	for _, entry := range entries {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func encodeTaskUpdateCursor(value time.Time) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value.UTC().Format(time.RFC3339Nano)))
}

func decodeTaskUpdateCursor(value string) (time.Time, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, string(decoded))
}

func (s *PMTaskInsightsService) GetStandingBrief(ctx context.Context, workspaceID, taskID string) (*model.TaskStandingBriefResponse, error) {
	detail, err := s.task(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	sourceUpdatedAt, err := s.insightsRepo.LatestSourceUpdatedAt(ctx, workspaceID, taskID, detail.Task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	brief, err := s.insightsRepo.GetBrief(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if brief == nil {
		return &model.TaskStandingBriefResponse{Status: model.TaskStandingBriefStale, IsStale: true, SourceUpdatedAt: &sourceUpdatedAt, Suggestions: []model.TaskStandingBriefSuggestion{}, Evidence: []model.TaskStandingBriefEvidence{}}, nil
	}
	return s.standingBriefResponse(ctx, brief, sourceUpdatedAt)
}

func (s *PMTaskInsightsService) RefreshStandingBrief(ctx context.Context, workspaceID, taskID string) (*model.TaskStandingBriefResponse, error) {
	if s.llmProvider == nil {
		return nil, fmt.Errorf("task standing brief AI provider is not configured")
	}
	detail, err := s.task(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	sourceUpdatedAt, err := s.insightsRepo.LatestSourceUpdatedAt(ctx, workspaceID, taskID, detail.Task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	existing, err := s.insightsRepo.GetBrief(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	pending := &model.PMTaskStandingBrief{WorkspaceID: workspaceID, TaskID: taskID, Status: model.TaskStandingBriefPending, SourceUpdatedAt: &sourceUpdatedAt, LastTriggeredAt: &now, Version: taskStandingBriefVersion, Suggestions: json.RawMessage("[]"), Evidence: json.RawMessage("[]")}
	if existing != nil {
		pending.ID = existing.ID
		pending.Narrative = existing.Narrative
		pending.Suggestions = existing.Suggestions
		pending.Evidence = existing.Evidence
		pending.ComputedAt = existing.ComputedAt
	}
	if err := s.insightsRepo.SaveBrief(ctx, pending); err != nil {
		return nil, err
	}

	prompt, signalCount, err := s.taskStandingBriefPrompt(ctx, detail)
	if err != nil {
		return nil, err
	}
	if signalCount == 0 {
		pending.Narrative = ""
		pending.Status = model.TaskStandingBriefReady
		pending.ComputedAt = &now
		if err := s.insightsRepo.SaveBrief(ctx, pending); err != nil {
			return nil, err
		}
		return s.standingBriefResponse(ctx, pending, sourceUpdatedAt)
	}

	callCtx := WithAIUsageMetering(ctx, AIUsageMeteringContext{
		WorkspaceID:    workspaceID,
		FeatureKey:     BillingFeatureTaskStandingBrief,
		IdempotencyKey: fmt.Sprintf("task-standing-brief:%s:%d", taskID, sourceUpdatedAt.UnixNano()),
		Metadata:       map[string]interface{}{"task_id": taskID},
	})
	resp, err := s.llmProvider.ChatCompletion(callCtx, llm.ChatRequest{
		SystemPrompt: taskStandingBriefSystemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(prompt)}},
		Temperature:  0.1,
		MaxTokens:    taskStandingBriefMaxTokens,
		JSONMode:     true,
	})
	if err != nil {
		message := err.Error()
		pending.Status = model.TaskStandingBriefError
		pending.LastError = &message
		if saveErr := s.insightsRepo.SaveBrief(ctx, pending); saveErr != nil {
			return nil, fmt.Errorf("generate task standing brief: %w (failed to persist error state: %v)", err, saveErr)
		}
		return nil, fmt.Errorf("generate task standing brief: %w", err)
	}
	var output taskStandingBriefGenerationOutput
	if err := llm.UnmarshalResponse(resp.Content, &output); err != nil {
		message := err.Error()
		pending.Status = model.TaskStandingBriefError
		pending.LastError = &message
		if saveErr := s.insightsRepo.SaveBrief(ctx, pending); saveErr != nil {
			return nil, fmt.Errorf("parse task standing brief: %w (failed to persist error state: %v)", err, saveErr)
		}
		return nil, fmt.Errorf("parse task standing brief: %w", err)
	}
	output = normalizeTaskStandingBriefOutput(output)
	suggestions, _ := json.Marshal(output.Suggestions)
	evidence, _ := json.Marshal(output.Evidence)
	pending.Narrative = output.Narrative
	pending.Suggestions = suggestions
	pending.Evidence = evidence
	pending.Status = model.TaskStandingBriefReady
	pending.ComputedAt = &now
	pending.LastError = nil
	if err := s.insightsRepo.SaveBrief(ctx, pending); err != nil {
		return nil, err
	}
	return s.standingBriefResponse(ctx, pending, sourceUpdatedAt)
}

func (s *PMTaskInsightsService) DismissStandingBriefSuggestion(ctx context.Context, workspaceID, taskID, userID, key string) error {
	if _, err := s.task(ctx, workspaceID, taskID); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 200 {
		return fmt.Errorf("suggestion key is required")
	}
	return s.insightsRepo.DismissSuggestion(ctx, &model.PMTaskBriefSuggestionDismissal{WorkspaceID: workspaceID, TaskID: taskID, SuggestionKey: key, DismissedBy: userID})
}

func (s *PMTaskInsightsService) standingBriefResponse(ctx context.Context, brief *model.PMTaskStandingBrief, sourceUpdatedAt time.Time) (*model.TaskStandingBriefResponse, error) {
	suggestions := []model.TaskStandingBriefSuggestion{}
	evidence := []model.TaskStandingBriefEvidence{}
	_ = json.Unmarshal(brief.Suggestions, &suggestions)
	_ = json.Unmarshal(brief.Evidence, &evidence)
	dismissed, err := s.insightsRepo.ListDismissedSuggestionKeys(ctx, brief.WorkspaceID, brief.TaskID)
	if err != nil {
		return nil, err
	}
	visible := suggestions[:0]
	for _, suggestion := range suggestions {
		if _, hidden := dismissed[suggestion.Key]; !hidden {
			visible = append(visible, suggestion)
		}
	}
	isStale := brief.SourceUpdatedAt == nil || sourceUpdatedAt.After(*brief.SourceUpdatedAt)
	status := brief.Status
	if isStale && status == model.TaskStandingBriefReady {
		status = model.TaskStandingBriefStale
	}
	return &model.TaskStandingBriefResponse{Narrative: brief.Narrative, Suggestions: visible, Evidence: evidence, Status: status, IsStale: isStale, ComputedAt: brief.ComputedAt, SourceUpdatedAt: &sourceUpdatedAt}, nil
}

type taskStandingBriefGenerationOutput struct {
	Narrative   string                              `json:"narrative"`
	Suggestions []model.TaskStandingBriefSuggestion `json:"suggestions"`
	Evidence    []model.TaskStandingBriefEvidence   `json:"evidence"`
}

func (s *PMTaskInsightsService) taskStandingBriefPrompt(ctx context.Context, detail *model.TaskDetail) ([]byte, int, error) {
	comments, err := s.commentRepo.List(ctx, "task", detail.Task.ID)
	if err != nil {
		return nil, 0, err
	}
	activity, _, err := s.activityRepo.List(ctx, "task", detail.Task.ID, model.PMPagination{Page: 1, PerPage: 30})
	if err != nil {
		return nil, 0, err
	}
	runs, err := s.agentRunRepo.ListByTarget(ctx, detail.Task.WorkspaceID, "task", detail.Task.ID)
	if err != nil {
		return nil, 0, err
	}
	gitLinks, err := s.gitLinkRepo.ListByTask(ctx, detail.Task.WorkspaceID, detail.Task.ID)
	if err != nil {
		return nil, 0, err
	}
	checklist, err := s.checklistRepo.List(ctx, detail.Task.ID)
	if err != nil {
		return nil, 0, err
	}
	if len(comments) > 20 {
		comments = comments[len(comments)-20:]
	}
	if len(runs) > 10 {
		runs = runs[:10]
	}
	if len(gitLinks) > 10 {
		gitLinks = gitLinks[:10]
	}
	payload := map[string]interface{}{"task": detail, "comments": comments, "activity": activity, "agent_runs": runs, "git_links": gitLinks, "checklist": checklist}
	encoded, err := json.Marshal(payload)
	return encoded, len(comments) + len(activity) + len(runs) + len(gitLinks) + len(checklist), err
}

func normalizeTaskStandingBriefOutput(output taskStandingBriefGenerationOutput) taskStandingBriefGenerationOutput {
	output.Narrative = strings.TrimSpace(output.Narrative)
	if len(output.Narrative) > 1600 {
		output.Narrative = output.Narrative[:1600]
	}
	valid := make([]model.TaskStandingBriefSuggestion, 0, len(output.Suggestions))
	for _, suggestion := range output.Suggestions {
		suggestion.Label = strings.TrimSpace(suggestion.Label)
		if suggestion.Label == "" {
			continue
		}
		if _, ok := allowedTaskBriefActions[suggestion.Action.Type]; !ok {
			continue
		}
		if suggestion.Key == "" {
			suggestion.Key = stableTaskBriefSuggestionKey(suggestion)
		}
		valid = append(valid, suggestion)
		if len(valid) == 4 {
			break
		}
	}
	output.Suggestions = valid
	if len(output.Evidence) > 8 {
		output.Evidence = output.Evidence[:8]
	}
	return output
}

func stableTaskBriefSuggestionKey(suggestion model.TaskStandingBriefSuggestion) string {
	raw := strings.Join([]string{suggestion.Action.Type, suggestion.Action.TargetID, suggestion.Action.RunID, suggestion.Action.CommentID, suggestion.Action.Text, suggestion.Label}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}

const taskStandingBriefSystemPrompt = `You generate a concise, evidence-grounded "Where this stands" brief for a project-management task.

Return JSON only with:
- narrative: 2-3 short sentences describing current progress, anything stalled, and unanswered decisions.
- suggestions: up to 4 objects with key, label, action, and evidence.
- evidence: up to 8 objects with type, id, label, and optional timestamp.

Allowed action.type values are retry_run, reply_to_comment, add_checklist_item, and open_related_object. Use IDs present in the input. Never invent facts, IDs, links, or executable actions. Do not include restricted cross-app assumptions. If evidence is weak, return an empty narrative and no suggestions.`
