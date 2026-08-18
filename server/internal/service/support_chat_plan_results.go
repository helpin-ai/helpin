package service

// Child-plan result delivery for support chat runs: when a plan launched from
// a support conversation's chat run settles, its result is resumed into the
// chat run as a <child_run_result> block (same contract as the dock). A busy
// run is retried by the sweep; an ended run gets a successor so the waiting
// visitor still receives the answer.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportChildResultResumePrefix = "support-child:"

const (
	supportChildEvidencePrefix          = "child-result:"
	supportChildSourceOfficialWeb       = "support_child_official_web"
	supportChildSourceRepository        = "support_child_repository"
	supportChildSourceWorkspaceResearch = "support_child_workspace"
)

var supportChildURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// NotifyPlanSettledForRun is the immediate delivery hook: called by the
// agent-runtime terminal finalizer for command-bar child runs, it resolves the
// run's plan and, when settled, delivers the result into the parent support
// chat run.
func (s *SupportChatService) NotifyPlanSettledForRun(ctx context.Context, run *model.AgentRun) error {
	if s == nil || s.planRepo == nil || run == nil {
		return nil
	}
	payload, ok := commandBarRunPayload(run)
	if !ok || strings.TrimSpace(payload.PlanID) == "" {
		return nil
	}
	plan, err := s.planRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
	if err != nil || plan == nil {
		return err
	}
	return s.notifySupportPlanSettled(ctx, plan)
}

// SweepUnnotifiedSupportChatResults retries settled support-launched plans
// whose parent chat run was busy at finalize time.
func (s *SupportChatService) SweepUnnotifiedSupportChatResults(ctx context.Context, limit int) error {
	if s == nil || s.planRepo == nil {
		return nil
	}
	plans, err := s.planRepo.ListSettledUnnotifiedDockPlans(ctx, limit)
	if err != nil {
		return err
	}
	for i := range plans {
		if plans[i].SupportConversationID == nil {
			continue
		}
		if err := s.notifySupportPlanSettled(ctx, &plans[i]); err != nil {
			slog.WarnContext(ctx, "support chat result delivery failed",
				"workspace_id", plans[i].WorkspaceID, "plan_id", plans[i].ID, "error", err)
		}
	}
	return nil
}

// notifySupportPlanSettled delivers one settled plan's result into its support
// conversation's chat run. Idempotent via parent_notified_at + runtime
// resume_id dedupe.
func (s *SupportChatService) notifySupportPlanSettled(ctx context.Context, plan *model.CommandBarPlanRecord) error {
	if plan == nil || plan.ParentChatRunID == nil || plan.ParentNotifiedAt != nil {
		return nil
	}
	if !isDockPlanTerminalStatus(plan.Status) || plan.SupportConversationID == nil {
		return nil
	}
	conversationID := strings.TrimSpace(*plan.SupportConversationID)
	conv, err := s.conversationRepo.GetByID(ctx, plan.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	if conv == nil || supportPlanDeliveryBlocked(conv) {
		// Orphaned plan, or the conversation left AI handling (takeover /
		// escalation / visitor asked for a human): a human owns it now, so the
		// child result is permanently undeliverable to the agent.
		return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
	}

	var chatRun *model.AgentRun
	if conv.AIActiveRunID != nil && strings.TrimSpace(*conv.AIActiveRunID) != "" {
		chatRun, err = s.runRepo.GetByID(ctx, plan.WorkspaceID, *conv.AIActiveRunID)
		if err != nil {
			return err
		}
	}
	if chatRun == nil || !model.IsAgentRunActiveStatus(chatRun.Status) {
		return s.deliverPlanResultViaSuccessor(ctx, conv, chatRun, plan)
	}
	if chatRun.Status != model.AgentRunStatusPaused || chatRun.PauseReason != model.AgentRunPauseReasonUserMessage {
		// Mid-turn or paused on an interaction: the sweep retries.
		return nil
	}
	if chatRun.ExternalRuntimeID == nil || strings.TrimSpace(*chatRun.ExternalRuntimeID) == "" {
		return nil
	}
	runtimeClient := s.agentService.agentRuntimeClient
	if runtimeClient == nil {
		return nil
	}

	block, err := buildChildRunResultBlockWith(ctx, plan, s.runRepo, s.agentService.runMessageRepo, s.agentService)
	if err != nil {
		return err
	}
	if evidence := s.prepareSupportChildEvidence(ctx, plan, block); evidence != nil && s.persistSupportChildEvidence(ctx, chatRun.ID, evidence) {
		block, err = addEvidenceIDToChildRunResultBlock(block, evidence.EvidenceID)
		if err != nil {
			return err
		}
	}
	resumeID := supportChildResultResumePrefix + plan.ID
	if _, err := runtimeClient.ResumeRun(ctx, strings.TrimSpace(*chatRun.ExternalRuntimeID), AgentRuntimeResumeRunRequest{
		Intent:   "reply",
		Content:  block,
		ResumeID: resumeID,
	}); err != nil {
		if isChatRunExpiredError(err) {
			return s.deliverPlanResultViaSuccessor(ctx, conv, chatRun, plan)
		}
		// "run is not paused" and similar races resolve on the next sweep.
		return err
	}
	recordChildResultRunMessage(ctx, s.agentService.runMessageRepo, chatRun, block, resumeID)
	chatRun.Status = model.AgentRunStatusRunning
	chatRun.PauseReason = model.AgentRunPauseReasonNone
	if err := s.runRepo.Update(ctx, chatRun); err != nil {
		slog.WarnContext(ctx, "support chat result: mark chat run running failed",
			"workspace_id", plan.WorkspaceID, "run_id", chatRun.ID, "error", err)
	}
	s.supportAIService.publishTypingIndicator(ctx, plan.WorkspaceID, conversationID, true)
	return s.planRepo.MarkParentNotified(ctx, plan.WorkspaceID, plan.ID)
}

// deliverPlanResultViaSuccessor starts a successor chat run carrying the child
// result — the visitor asked a question whose answer depended on the child, so
// delivery must not wait for their next message.
func (s *SupportChatService) deliverPlanResultViaSuccessor(ctx context.Context, conv *model.SupportConversation, previousRun *model.AgentRun, plan *model.CommandBarPlanRecord) error {
	settings, err := s.supportAIService.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	agentID := strings.TrimSpace(derefString(settings.AIAgentID))
	if agentID == "" || !shouldAutomaticallyProcessSupportAI(*settings) {
		return s.planRepo.MarkParentNotified(ctx, conv.WorkspaceID, plan.ID)
	}
	agent, err := s.agentService.GetAgent(ctx, conv.WorkspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	block, err := buildChildRunResultBlockWith(ctx, plan, s.runRepo, s.agentService.runMessageRepo, s.agentService)
	if err != nil {
		return err
	}
	evidence := s.prepareSupportChildEvidence(ctx, plan, block)
	if evidence != nil {
		block, err = addEvidenceIDToChildRunResultBlock(block, evidence.EvidenceID)
		if err != nil {
			return err
		}
	}
	if err := s.startSupportChatRun(ctx, conv, agent, block, previousRun, evidence); err != nil {
		return err
	}
	s.supportAIService.publishTypingIndicator(ctx, conv.WorkspaceID, conv.ID, true)
	return s.planRepo.MarkParentNotified(ctx, conv.WorkspaceID, plan.ID)
}

// prepareSupportChildEvidence converts a completed read-only child handoff
// into evidence that the reply gate can validate. Official-web research is
// accepted only when the handoff contains a URL on the workspace's configured
// website domain. Repository and live-workspace reads remain internal.
func (s *SupportChatService) prepareSupportChildEvidence(ctx context.Context, plan *model.CommandBarPlanRecord, block string) *model.SupportRunEvidence {
	if s == nil || s.evidenceRepo == nil || plan == nil || strings.TrimSpace(plan.Status) != model.CommandBarPlanStatusCompleted {
		return nil
	}
	evidenceContent := childRunResultEvidenceContent(block)
	if evidenceContent == "" {
		return nil
	}

	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil || len(steps) == 0 {
		return nil
	}
	hasWeb, hasRepository := supportChildResearchKinds(steps)
	evidenceID := supportChildEvidencePrefix + strings.TrimSpace(plan.ID)
	row := &model.SupportRunEvidence{
		WorkspaceID:   plan.WorkspaceID,
		EvidenceID:    evidenceID,
		ReferenceID:   evidenceID,
		SourceID:      strings.TrimSpace(plan.ID),
		Content:       evidenceContent,
		VectorScore:   0.9,
		CombinedScore: 0.9,
	}

	switch {
	case hasRepository:
		row.SourceType = supportChildSourceRepository
		row.Title = "Verified product implementation"
		row.IsInternal = true
	case hasWeb:
		if s.workspaceRepo == nil {
			return nil
		}
		workspace, err := s.workspaceRepo.GetByID(ctx, plan.WorkspaceID)
		if err != nil || workspace == nil {
			return nil
		}
		officialURL := ""
		if workspace.WebsiteURL != nil {
			officialURL = strings.TrimSpace(*workspace.WebsiteURL)
		}
		matchedURL := officialURLFromChildResult(evidenceContent, officialURL)
		if matchedURL == "" {
			slog.WarnContext(ctx, "support child web result omitted an official-domain URL",
				"workspace_id", plan.WorkspaceID, "plan_id", plan.ID)
			return nil
		}
		row.SourceType = supportChildSourceOfficialWeb
		row.Title = "Official product website"
		row.URL = matchedURL
		row.IsInternal = false
	default:
		row.SourceType = supportChildSourceWorkspaceResearch
		row.Title = "Verified workspace information"
		row.IsInternal = true
		row.VectorScore = 0.85
		row.CombinedScore = 0.85
	}
	return row
}

func (s *SupportChatService) persistSupportChildEvidence(ctx context.Context, runID string, evidence *model.SupportRunEvidence) bool {
	if s == nil || s.evidenceRepo == nil || evidence == nil || strings.TrimSpace(runID) == "" {
		return false
	}
	row := *evidence
	row.RunID = strings.TrimSpace(runID)
	if err := s.evidenceRepo.UpsertBatch(ctx, []model.SupportRunEvidence{row}); err != nil {
		slog.WarnContext(ctx, "support child evidence persistence failed",
			"error", err, "workspace_id", row.WorkspaceID, "run_id", row.RunID, "evidence_id", row.EvidenceID)
		return false
	}
	return true
}

func supportChildResearchKinds(steps []model.CommandBarPlanStep) (hasWeb, hasRepository bool) {
	for _, step := range steps {
		for _, tool := range step.AllowedTools {
			switch strings.TrimSpace(tool) {
			case "web_search", "fetch_url", "crawl_url":
				hasWeb = true
			case "checkout_repositories", "list_repositories", "list_commits", "read_files", "list_directory", "repository_search", "list_symbols", "read_symbol", "trace_symbol":
				hasRepository = true
			}
		}
	}
	return hasWeb, hasRepository
}

func childRunResultHasSummary(block string) bool {
	return childRunResultEvidenceContent(block) != ""
}

// childRunResultEvidenceContent deliberately excludes the plan prompt and
// error envelope. Visitor-supplied values in the original question must never
// become evidence for themselves; only completed child summaries count.
func childRunResultEvidenceContent(block string) string {
	payload := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(block), dockChildResultOpenTag), dockChildResultCloseTag)
	var result dockChildRunResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return ""
	}
	var summaries []string
	for _, run := range result.Runs {
		if run.ResultAvailable && strings.TrimSpace(run.Summary) != "" {
			summaries = append(summaries, strings.TrimSpace(run.Summary))
		}
	}
	return strings.Join(summaries, "\n\n")
}

func addEvidenceIDToChildRunResultBlock(block, evidenceID string) (string, error) {
	payload := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(block), dockChildResultOpenTag), dockChildResultCloseTag)
	var result dockChildRunResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return "", err
	}
	result.EvidenceID = strings.TrimSpace(evidenceID)
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return dockChildResultOpenTag + string(encoded) + dockChildResultCloseTag, nil
}

func officialURLFromChildResult(block, workspaceURL string) string {
	officialHost := normalizedSupportWebsiteHost(workspaceURL)
	if officialHost == "" {
		return ""
	}
	for _, candidate := range supportChildURLPattern.FindAllString(block, -1) {
		candidate = strings.TrimRight(candidate, ".,;:!?)]}")
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		candidateHost := normalizedSupportWebsiteHost(parsed.Hostname())
		if candidateHost == officialHost || strings.HasSuffix(candidateHost, "."+officialHost) {
			return candidate
		}
	}
	return ""
}

func normalizedSupportWebsiteHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	return strings.TrimPrefix(host, "www.")
}

// supportPlanDeliveryBlocked reports whether the conversation has left AI
// handling, making child-result delivery to the agent moot.
func supportPlanDeliveryBlocked(conv *model.SupportConversation) bool {
	if conv.CustomerRequestedHumanAt != nil || (conv.HumanTakeover != nil && *conv.HumanTakeover) {
		return true
	}
	return derefString(conv.AIState) == "escalated"
}
