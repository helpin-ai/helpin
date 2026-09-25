package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type SetupService struct {
	repo         *repository.SetupRepository
	entitlements EntitlementPolicy
}

func NewSetupService(repo *repository.SetupRepository) *SetupService {
	return &SetupService{repo: repo}
}

func (s *SetupService) SetEntitlementService(entitlements EntitlementPolicy) *SetupService {
	s.entitlements = entitlements
	return s
}

func (s *SetupService) Get(ctx context.Context, workspaceID, userID string, accesses ...SetupAccess) (model.SetupView, error) {
	access := SetupAccess{Unrestricted: true}
	if len(accesses) > 0 {
		access = accesses[0]
	}
	access = s.withEntitlements(ctx, workspaceID, access)

	workspaceStartedAt, err := s.repo.WorkspaceCreatedAt(ctx, workspaceID)
	if err != nil {
		return model.SetupView{}, err
	}
	if workspaceStartedAt.IsZero() {
		workspaceStartedAt = time.Unix(0, 0).UTC()
	}
	evidenceWindows := make(map[string]SetupEvidence, 4)
	loadEvidence := func(since time.Time) (SetupEvidence, error) {
		key := since.UTC().Format(time.RFC3339Nano)
		if evidence, ok := evidenceWindows[key]; ok {
			return evidence, nil
		}
		evidence, evidenceErr := s.repo.GetEvidenceSince(ctx, workspaceID, since)
		if evidenceErr == nil {
			evidenceWindows[key] = evidence
		}
		return evidence, evidenceErr
	}
	goals, err := s.repo.ListGoals(ctx, workspaceID)
	if err != nil {
		return model.SetupView{}, err
	}
	if len(goals) > 0 {
		pending, pendingErr := s.repo.PendingGoalKeys(ctx, workspaceID)
		if pendingErr != nil {
			return model.SetupView{}, pendingErr
		}
		if len(pending) > 0 {
			if err := s.repo.ClearPendingGoalKeys(ctx, workspaceID); err != nil {
				return model.SetupView{}, err
			}
		}
	}
	if len(goals) == 0 {
		pending, pendingErr := s.repo.PendingGoalKeys(ctx, workspaceID)
		if pendingErr != nil {
			return model.SetupView{}, pendingErr
		}
		if len(pending) > 0 {
			normalized, normalizeErr := NormalizeSetupGoals(pending)
			if normalizeErr != nil {
				return model.SetupView{}, normalizeErr
			}
			if err := s.repo.SyncGoalsAt(ctx, workspaceID, userID, normalized, "onboarding_recovered", workspaceStartedAt); err != nil {
				return model.SetupView{}, err
			}
			if err := s.repo.ClearPendingGoalKeys(ctx, workspaceID); err != nil {
				return model.SetupView{}, err
			}
			goals, err = s.repo.ListGoals(ctx, workspaceID)
			if err != nil {
				return model.SetupView{}, err
			}
		}
	}
	if len(goals) == 0 {
		inferenceSince := workspaceStartedAt
		historical, evidenceErr := loadEvidence(inferenceSince)
		if evidenceErr != nil {
			return model.SetupView{}, evidenceErr
		}
		inferred := inferSetupGoals(historical)
		if len(inferred) > 0 {
			if err := s.repo.SyncGoalsAt(ctx, workspaceID, userID, inferred, "inferred", inferenceSince); err != nil {
				return model.SetupView{}, err
			}
			goals, err = s.repo.ListGoals(ctx, workspaceID)
			if err != nil {
				return model.SetupView{}, err
			}
		}
	}

	evidenceByGoal := make(map[string]SetupEvidence, len(goals)+2)
	foundationEvidence, err := loadEvidence(workspaceStartedAt)
	if err != nil {
		return model.SetupView{}, err
	}
	evidenceByGoal[model.SetupGoalFoundation] = foundationEvidence
	for _, goal := range goals {
		if _, supported := setupJourneyCatalog[goal.Key]; !supported {
			continue
		}
		since := goal.ActivatedAt
		if since.IsZero() {
			since = workspaceStartedAt
		}
		evidence, evidenceErr := loadEvidence(since)
		if evidenceErr != nil {
			return model.SetupView{}, evidenceErr
		}
		memberRuns, memberErr := s.repo.MemberValuableAgentRunEvidence(ctx, workspaceID, userID, since)
		if memberErr != nil {
			return model.SetupView{}, memberErr
		}
		applyMemberAgentEvidence(&evidence, memberRuns)
		if goal.Key == model.SetupGoalAutomationMastery {
			memberApprovals, achievedAt, memberApprovalErr := s.repo.MemberApprovalEvidence(ctx, workspaceID, userID, since)
			if memberApprovalErr != nil {
				return model.SetupView{}, memberApprovalErr
			}
			evidence.MemberApprovalResolvedCount = memberApprovals
			setSetupAchievementTime(&evidence, "automation.approval_resolved", achievedAt)
		}
		evidenceByGoal[goal.Key] = evidence
	}
	if _, selected := evidenceByGoal[model.SetupGoalAutomationMastery]; !selected {
		memberRuns, memberErr := s.repo.MemberValuableAgentRunEvidence(ctx, workspaceID, userID, workspaceStartedAt)
		if memberErr != nil {
			return model.SetupView{}, memberErr
		}
		applyMemberAgentEvidence(&foundationEvidence, memberRuns)
		memberApprovals, achievedAt, memberApprovalErr := s.repo.MemberApprovalEvidence(ctx, workspaceID, userID, workspaceStartedAt)
		if memberApprovalErr != nil {
			return model.SetupView{}, memberApprovalErr
		}
		foundationEvidence.MemberApprovalResolvedCount = memberApprovals
		setSetupAchievementTime(&foundationEvidence, "automation.approval_resolved", achievedAt)
		evidenceByGoal[model.SetupGoalAutomationMastery] = foundationEvidence
	}

	verifiedAt := time.Now().UTC()
	for goalKey, evidence := range evidenceByGoal {
		for taskKey, achievedAt := range verifiedSetupTaskAchievements(goalKey, evidence, verifiedAt) {
			if err := s.repo.EnsureAchievementsAt(ctx, workspaceID, goalKey, []string{taskKey}, achievedAt); err != nil {
				return model.SetupView{}, err
			}
		}
		for taskKey, achievedAt := range verifiedMemberSetupTaskAchievements(goalKey, evidence) {
			if err := s.repo.EnsureAchievementsAt(ctx, workspaceID, goalKey, []string{taskKey}, achievedAt, userID); err != nil {
				return model.SetupView{}, err
			}
		}
	}
	achievements, err := s.repo.ListAchievements(ctx, workspaceID, userID)
	if err != nil {
		return model.SetupView{}, err
	}
	preference, err := s.repo.GetPreference(ctx, workspaceID, userID)
	if err != nil {
		return model.SetupView{}, err
	}
	return BuildSetupViewWithState(evidenceByGoal, goals, achievements, access, preference), nil
}

func applyMemberAgentEvidence(evidence *model.SetupEvidence, member model.SetupMemberAgentEvidence) {
	evidence.MemberAgentRunCount = member.RunCount
	evidence.MemberAgentRunDayCount = member.RunDayCount
	evidence.MemberAgentTargetCount = member.RunTargetCount
	evidence.MemberProductAgentRunCount = member.ProductCount
	setSetupAchievementTime(evidence, "automation.personal_contribution", member.FirstRunAt)
	setSetupAchievementTime(evidence, "automation.personal_repeat_contribution", member.RepeatRunAt)
	setSetupAchievementTime(evidence, "product.agent_result_used", member.FirstProductAt)
}

func setSetupAchievementTime(evidence *model.SetupEvidence, taskKey string, achievedAt time.Time) {
	if achievedAt.IsZero() {
		return
	}
	if evidence.TaskAchievementTimes == nil {
		evidence.TaskAchievementTimes = make(map[string]time.Time)
	}
	evidence.TaskAchievementTimes[taskKey] = achievedAt
}

func (s *SetupService) UpdateGoals(ctx context.Context, workspaceID, actorID string, raw []string, accesses ...SetupAccess) (model.SetupView, error) {
	goals, err := NormalizeSetupGoals(raw)
	if err != nil {
		return model.SetupView{}, err
	}
	if len(goals) == 0 {
		return model.SetupView{}, fmt.Errorf("choose at least one setup goal")
	}
	if err := s.repo.SyncGoals(ctx, workspaceID, actorID, goals, "manual"); err != nil {
		return model.SetupView{}, err
	}
	return s.Get(ctx, workspaceID, actorID, accesses...)
}

func (s *SetupService) UpdatePreference(ctx context.Context, workspaceID, userID string, dismissed bool) (model.MemberSetupPreference, error) {
	return s.repo.UpdatePreference(ctx, workspaceID, userID, dismissed)
}

func (s *SetupService) StartRecommendation(ctx context.Context, workspaceID, userID, taskKey string) error {
	goalKey, actionKey, ok := setupTaskAction(taskKey)
	if !ok {
		return fmt.Errorf("unknown setup task %q", taskKey)
	}
	return s.repo.RecordActionIntent(ctx, workspaceID, userID, goalKey, taskKey, actionKey)
}

func (s *SetupService) InitializeSetupGoals(ctx context.Context, workspaceID, actorID string, raw []string) error {
	goals, err := NormalizeSetupGoals(raw)
	if err != nil {
		return err
	}
	if len(goals) == 0 {
		return nil
	}
	if err := s.repo.SyncGoals(ctx, workspaceID, actorID, goals, "onboarding"); err != nil {
		return fmt.Errorf("initialize setup goals: %w", err)
	}
	return s.repo.ClearPendingGoalKeys(ctx, workspaceID)
}

func (s *SetupService) withEntitlements(ctx context.Context, workspaceID string, access SetupAccess) SetupAccess {
	if access.Unrestricted {
		return access
	}
	if access.Entitlements == nil {
		access.Entitlements = map[string]bool{}
	}
	for _, feature := range []EntitlementFeature{EntitlementFeatureAutomationFlows, EntitlementFeatureCustomAgents, EntitlementFeatureAgentScheduling, EntitlementFeatureDealAutomation} {
		access.Entitlements[string(feature)] = s.entitlements == nil || s.entitlements.RequireFeature(ctx, workspaceID, feature) == nil
	}
	return access
}

func inferSetupGoals(evidence model.SetupEvidence) []string {
	goals := make([]string, 0, 3)
	if evidence.PlannedProjectCount > 0 || evidence.PlannedSprintCount > 0 || evidence.AssignedProjectTaskCount > 0 || evidence.SprintCloseoutCount > 0 || evidence.ConnectedRepositoryCount > 0 {
		goals = append(goals, model.SetupGoalProductDelivery)
	}
	if evidence.SupportEmailInboxCount > 0 || evidence.LiveChatInstallationCount > 0 || evidence.PublicHelpDocCount > 0 ||
		evidence.BrandKnowledgeSourceCount > 0 || evidence.SupportAIAgentActive || evidence.TeamInboxCount > 0 ||
		evidence.AutomaticRoutingCount > 0 || evidence.LinkedSupportTaskCount > 0 || evidence.CoverageImprovementCount > 0 {
		goals = append(goals, model.SetupGoalCustomerSupport)
	}
	if evidence.CompletedAgentRunCount > 0 || evidence.EnabledAutomationCount > 0 || evidence.TriggeredSuccessRunCount > 0 || evidence.CustomAgentSuccessCount > 0 {
		goals = append(goals, model.SetupGoalAutomationMastery)
	}
	if len(goals) < 3 && (evidence.HelpCenterSpaceCount > 0 || evidence.HelpCenterContentCount > 0 || evidence.HelpCenterSiteCount > 0 || evidence.HelpCenterWidgetCount > 0) {
		goals = append(goals, model.SetupGoalHelpCenterDocs)
	}
	if len(goals) < 3 && (evidence.InternalDocsSpaceCount > 0 || evidence.InternalDocsContentCount > 0 || evidence.InternalDocsPublishedCount > 0 || evidence.InternalAgentKnowledgeCount > 0) {
		goals = append(goals, model.SetupGoalInternalDocs)
	}
	if len(goals) < 3 && (evidence.CRMContactCount > 0 || evidence.CRMCompanyCount > 0 || evidence.CRMActionableDealCount > 0 || evidence.CRMConnectedEmailCount > 0) {
		goals = append(goals, model.SetupGoalSalesCRM)
	}
	return goals
}
