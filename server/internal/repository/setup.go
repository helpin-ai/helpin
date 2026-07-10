package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SetupRepository struct {
	db *gorm.DB
}

func NewSetupRepository(db *gorm.DB) *SetupRepository {
	return &SetupRepository{db: db}
}

func (r *SetupRepository) WorkspaceCreatedAt(ctx context.Context, workspaceID string) (time.Time, error) {
	var createdAt time.Time
	if err := r.db.WithContext(ctx).Table("workspaces").Select("created_at").Where("id = ?", workspaceID).Scan(&createdAt).Error; err != nil {
		return time.Time{}, fmt.Errorf("read workspace setup start: %w", err)
	}
	return createdAt, nil
}

func (r *SetupRepository) PendingGoalKeys(ctx context.Context, workspaceID string) ([]string, error) {
	var raw string
	if err := r.db.WithContext(ctx).Table("setup_intents").Select("goal_keys").Where("workspace_id = ?", workspaceID).Scan(&raw).Error; err != nil {
		return nil, fmt.Errorf("read pending setup goals: %w", err)
	}
	if raw == "" {
		return nil, nil
	}
	var keys []string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil, fmt.Errorf("decode pending setup goals: %w", err)
	}
	return keys, nil
}

func (r *SetupRepository) ClearPendingGoalKeys(ctx context.Context, workspaceID string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Delete(&model.SetupIntent{}).Error; err != nil {
		return fmt.Errorf("clear pending setup goals: %w", err)
	}
	return nil
}

func (r *SetupRepository) ListGoalKeys(ctx context.Context, workspaceID string) ([]string, error) {
	goals, err := r.ListGoals(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(goals))
	for _, goal := range goals {
		keys = append(keys, goal.Key)
	}
	return keys, nil
}

func (r *SetupRepository) ListGoals(ctx context.Context, workspaceID string) ([]model.SetupGoal, error) {
	var goals []model.SetupGoal
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND status = 'active'", workspaceID).
		Order("position ASC, created_at ASC").Find(&goals).Error; err != nil {
		return nil, fmt.Errorf("list setup goals: %w", err)
	}
	canonical := make([]model.SetupGoal, 0, len(goals))
	seen := map[string]bool{}
	for _, goal := range goals {
		if goal.Key == model.SetupGoalTeamProjects {
			goal.Key = model.SetupGoalProductDelivery
		}
		if seen[goal.Key] {
			continue
		}
		seen[goal.Key] = true
		canonical = append(canonical, goal)
	}
	return canonical, nil
}

func (r *SetupRepository) ReplaceGoals(ctx context.Context, workspaceID, actorID string, keys []string) error {
	return r.SyncGoals(ctx, workspaceID, actorID, keys, "manual")
}

func (r *SetupRepository) SyncGoals(ctx context.Context, workspaceID, actorID string, keys []string, source string) error {
	return r.SyncGoalsAt(ctx, workspaceID, actorID, keys, source, time.Now().UTC())
}

func (r *SetupRepository) SyncGoalsAt(ctx context.Context, workspaceID, actorID string, keys []string, source string, activatedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []model.SetupGoal
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspaceID).Find(&existing).Error; err != nil {
			return fmt.Errorf("load setup goals: %w", err)
		}
		byKey := make(map[string]model.SetupGoal, len(existing))
		for _, goal := range existing {
			byKey[goal.Key] = goal
		}
		selected := make(map[string]bool, len(keys))
		for position, key := range keys {
			selected[key] = true
			if goal, ok := byKey[key]; ok {
				if err := tx.Model(&model.SetupGoal{}).Where("id = ?", goal.ID).
					Updates(map[string]any{"status": "active", "position": position}).Error; err != nil {
					return fmt.Errorf("update setup goal %q: %w", key, err)
				}
				continue
			}
			catalogVersion := 1
			goal := model.SetupGoal{ID: uuid.NewString(), WorkspaceID: workspaceID, Key: key, CatalogVersion: catalogVersion, Source: source, Status: "active", Position: position, ActivatedAt: activatedAt, CreatedBy: actorID}
			if err := tx.Create(&goal).Error; err != nil {
				return fmt.Errorf("create setup goal %q: %w", key, err)
			}
		}
		for _, goal := range existing {
			if !selected[goal.Key] && goal.Status == "active" {
				if err := tx.Model(&model.SetupGoal{}).Where("id = ?", goal.ID).Update("status", "paused").Error; err != nil {
					return fmt.Errorf("pause setup goal %q: %w", goal.Key, err)
				}
			}
		}
		return nil
	})
}

func (r *SetupRepository) ListAchievements(ctx context.Context, workspaceID string, userIDs ...string) (map[string]time.Time, error) {
	var rows []model.SetupAchievement
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND member_id = ''", workspaceID)
	if len(userIDs) > 0 && userIDs[0] != "" {
		query = r.db.WithContext(ctx).Where("workspace_id = ? AND (member_id = '' OR member_id = ?)", workspaceID, userIDs[0])
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list setup achievements: %w", err)
	}
	result := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		result[row.TaskKey] = row.AchievedAt
	}
	return result, nil
}

func (r *SetupRepository) EnsureAchievements(ctx context.Context, workspaceID, goalKey string, taskKeys []string) error {
	return r.EnsureAchievementsAt(ctx, workspaceID, goalKey, taskKeys, time.Now().UTC())
}

func (r *SetupRepository) EnsureAchievementsAt(ctx context.Context, workspaceID, goalKey string, taskKeys []string, achievedAt time.Time, memberIDs ...string) error {
	memberID := ""
	if len(memberIDs) > 0 {
		memberID = memberIDs[0]
	}
	for _, taskKey := range taskKeys {
		evidence, _ := json.Marshal(map[string]string{"source": "computed_product_evidence", "task_key": taskKey})
		row := model.SetupAchievement{ID: uuid.NewString(), WorkspaceID: workspaceID, GoalKey: goalKey, TaskKey: taskKey, MemberID: memberID, Evidence: string(evidence), AchievedAt: achievedAt}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return fmt.Errorf("record setup achievement %q: %w", taskKey, err)
		}
	}
	return nil
}

func (r *SetupRepository) RecordActionIntent(ctx context.Context, workspaceID, memberID, goalKey, taskKey, actionKey string) error {
	intent := model.SetupActionIntent{
		ID: uuid.NewString(), WorkspaceID: workspaceID, MemberID: memberID, GoalKey: goalKey,
		TaskKey: taskKey, ActionKey: actionKey, StartedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&intent).Error; err != nil {
		return fmt.Errorf("record setup action intent: %w", err)
	}
	return nil
}

func (r *SetupRepository) GetPreference(ctx context.Context, workspaceID, userID string) (model.MemberSetupPreference, error) {
	var preference model.MemberSetupPreference
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID)
	if err := query.First(&preference).Error; err == nil {
		return preference, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MemberSetupPreference{}, fmt.Errorf("get setup preference: %w", err)
	}
	candidate := model.MemberSetupPreference{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: userID}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return model.MemberSetupPreference{}, fmt.Errorf("create setup preference: %w", err)
	}
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&preference).Error; err != nil {
		return model.MemberSetupPreference{}, fmt.Errorf("read setup preference after create: %w", err)
	}
	return preference, nil
}

func (r *SetupRepository) UpdatePreference(ctx context.Context, workspaceID, userID string, dismissed bool) (model.MemberSetupPreference, error) {
	preference := model.MemberSetupPreference{ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: userID, SidebarDismissed: dismissed}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"sidebar_dismissed", "updated_at"}),
	}).Create(&preference).Error; err != nil {
		return model.MemberSetupPreference{}, fmt.Errorf("update setup preference: %w", err)
	}
	return r.GetPreference(ctx, workspaceID, userID)
}

func (r *SetupRepository) GetEvidence(ctx context.Context, workspaceID string) (model.SetupEvidence, error) {
	return r.GetEvidenceSince(ctx, workspaceID, time.Unix(0, 0).UTC())
}

func (r *SetupRepository) GetEvidenceSince(ctx context.Context, workspaceID string, since time.Time) (model.SetupEvidence, error) {
	evidence := model.SetupEvidence{TaskAchievementTimes: make(map[string]time.Time)}
	var workspaceTimezone string
	if err := r.db.WithContext(ctx).Table("workspaces").Select("timezone").Where("id = ?", workspaceID).Scan(&workspaceTimezone).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read workspace timezone: %w", err)
	}
	location, err := time.LoadLocation(workspaceTimezone)
	if err != nil {
		location = time.UTC
	}
	var contextCount int64
	queries := []struct {
		table string
		where string
		dest  *int64
		args  []any
	}{
		{"workspaces", "id = ? AND COALESCE(TRIM(company_product_context), '') <> ''", &contextCount, []any{workspaceID}},
		{"workspace_teams", "workspace_id = ?", &evidence.TeamCount, []any{workspaceID}},
		{"workspace_invitations", "workspace_id = ? AND status IN ('pending', 'accepted') AND created_at >= ?", &evidence.InvitationCount, []any{workspaceID, since}},
		{"workspace_members", "workspace_id = ? AND status = 'active'", &evidence.ActiveMemberCount, []any{workspaceID}},
		{"pm_tasks", "workspace_id = ? AND archived = false", &evidence.InitialWorkCount, []any{workspaceID}},
		{"pm_tasks", "workspace_id = ? AND completed = true AND archived = false AND completed_at >= ?", &evidence.CompletedTaskCount, []any{workspaceID, since}},
		{"pm_sprint_closeouts", "workspace_id = ? AND closed_at >= ?", &evidence.SprintCloseoutCount, []any{workspaceID, since}},
		{"git_repositories", "workspace_id = ? AND active = true AND selected = true AND deleted_at IS NULL", &evidence.ConnectedRepositoryCount, []any{workspaceID}},
		{"automation_rules", "workspace_id = ? AND enabled = true", &evidence.EnabledAutomationCount, []any{workspaceID}},
		{"agent_trigger_executions", "workspace_id = ? AND status = 'completed' AND completed_at >= ?", &evidence.TriggeredSuccessRunCount, []any{workspaceID, since}},
		{"support_conversations", "workspace_id = ? AND status = 'resolved' AND resolved_at >= ? AND EXISTS (SELECT 1 FROM support_messages sm WHERE sm.conversation_id = support_conversations.id AND sm.sender_type = 'customer' AND sm.deleted_at IS NULL)", &evidence.ValidatedSupportCount, []any{workspaceID, since}},
		{"support_conversations", "workspace_id = ? AND status = 'resolved' AND channel IN ('widget', 'email', 'api') AND source <> 'internal' AND resolved_at >= ? AND EXISTS (SELECT 1 FROM support_messages sm WHERE sm.conversation_id = support_conversations.id AND sm.sender_type = 'customer' AND sm.deleted_at IS NULL)", &evidence.ResolvedConversationCount, []any{workspaceID, since}},
		{"support_conversations", "workspace_id = ? AND status = 'resolved' AND resolved_at >= ? AND EXISTS (SELECT 1 FROM support_messages customer_message WHERE customer_message.conversation_id = support_conversations.id AND customer_message.sender_type = 'customer' AND customer_message.deleted_at IS NULL) AND EXISTS (SELECT 1 FROM support_messages ai_message WHERE ai_message.conversation_id = support_conversations.id AND ai_message.sender_type = 'ai' AND ai_message.message_type = 'reply' AND ai_message.deleted_at IS NULL)", &evidence.AIResolvedConversationCount, []any{workspaceID, since}},
		{"support_conversations", "workspace_id = ? AND linked_task_id IS NOT NULL AND updated_at >= ?", &evidence.LinkedSupportTaskCount, []any{workspaceID, since}},
		{"support_coverage_recommendations", "workspace_id = ? AND status = 'applied' AND updated_at >= ?", &evidence.CoverageImprovementCount, []any{workspaceID, since}},
		{"agent_runs", "workspace_id = ? AND approval_state IN ('approved', 'rejected') AND updated_at >= ?", &evidence.ApprovalResolvedCount, []any{workspaceID, since}},
	}
	for _, query := range queries {
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, query.args...).Count(query.dest).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read setup evidence from %s: %w", query.table, err)
		}
	}
	dayQueries := []struct {
		table  string
		column string
		where  string
		dest   *int64
		first  string
		repeat string
	}{
		{"pm_tasks", "completed_at", "workspace_id = ? AND completed = true AND archived = false AND completed_at >= ?", &evidence.CompletedTaskDayCount, "product.first_task_completed", "product.repeat_completion"},
		{"agent_runs", "completed_at", "workspace_id = ? AND status = 'completed' AND completed_at >= ? AND (COALESCE(CAST(output_summary AS TEXT), '{}') NOT IN ('{}', 'null', '') OR EXISTS (SELECT 1 FROM agent_run_artifacts WHERE agent_run_artifacts.run_id = agent_runs.id))", &evidence.CompletedAgentRunDayCount, "automation.first_assisted_value", "automation.repeat_assisted_value"},
		{"agent_trigger_executions", "completed_at", "workspace_id = ? AND status = 'completed' AND completed_at >= ?", &evidence.TriggeredSuccessDayCount, "automation.triggered_value", ""},
	}
	for _, query := range dayQueries {
		var timestamps []time.Time
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, workspaceID, since).Pluck(query.column, &timestamps).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read repeat setup evidence from %s: %w", query.table, err)
		}
		*query.dest = countDistinctLocalDays(timestamps, location)
		first, repeat := firstAndSecondDistinctLocalDay(timestamps, location)
		if !first.IsZero() {
			evidence.TaskAchievementTimes[query.first] = first
		}
		if query.repeat != "" && !repeat.IsZero() {
			evidence.TaskAchievementTimes[query.repeat] = repeat
		}
	}
	var verifiedWidgetCount, emailRouteCount, agentKnowledgeCount, supportKnowledgeCount int64
	additional := []struct {
		table string
		where string
		dest  *int64
	}{
		{"support_email_routes", "workspace_id = ? AND active = true", &emailRouteCount},
		{"agent_knowledge_sources", "workspace_id = ?", &agentKnowledgeCount},
		{"support_content_sources", "workspace_id = ?", &supportKnowledgeCount},
		{"support_content_sources", "workspace_id = ? AND sync_status = 'ready' AND (indexed_pages > 0 OR indexed_chunks > 0)", &evidence.BrandKnowledgeSourceCount},
		{"support_mailboxes", "workspace_id = ? AND active = true", &evidence.TeamInboxCount},
	}
	for _, query := range additional {
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, workspaceID).Count(query.dest).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read setup evidence from %s: %w", query.table, err)
		}
	}
	if err := r.db.WithContext(ctx).Table("support_widget_installations AS installs").
		Joins("JOIN support_widget_sessions AS sessions ON sessions.workspace_id = installs.workspace_id").
		Where("installs.workspace_id = ? AND installs.active = true", workspaceID).
		Distinct("installs.id").Count(&verifiedWidgetCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read verified support widget evidence: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("docs_helpcenter_articles AS articles").
		Joins("JOIN docs_documents AS documents ON documents.id = articles.document_id").
		Joins("JOIN docs_spaces AS spaces ON spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id").
		Where("documents.workspace_id = ? AND documents.deleted_at IS NULL AND spaces.deleted_at IS NULL AND spaces.type = 'external_capable' AND articles.public_published_at IS NOT NULL", workspaceID).
		Count(&evidence.PublicHelpDocCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read published help-doc setup evidence: %w", err)
	}
	var supportSettingsJSON string
	if err := r.db.WithContext(ctx).Table("support_widget_installations").
		Select("settings").Where("workspace_id = ? AND active = true", workspaceID).
		Limit(1).Scan(&supportSettingsJSON).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read support settings setup evidence: %w", err)
	}
	var supportSettings struct {
		AIEnabled          bool     `json:"ai_enabled"`
		AIAgentID          *string  `json:"ai_agent_id"`
		TriageEnabled      bool     `json:"triage_enabled"`
		WidgetHelpSpaceIDs []string `json:"widget_help_space_ids"`
	}
	if supportSettingsJSON != "" {
		if err := json.Unmarshal([]byte(supportSettingsJSON), &supportSettings); err != nil {
			return model.SetupEvidence{}, fmt.Errorf("decode support settings setup evidence: %w", err)
		}
	}
	evidence.SupportAIAgentActive = supportSettings.AIEnabled && supportSettings.AIAgentID != nil && strings.TrimSpace(*supportSettings.AIAgentID) != ""
	if supportSettings.TriageEnabled && evidence.TeamInboxCount > 0 {
		if err := r.db.WithContext(ctx).Table("support_mailboxes AS mailboxes").
			Where("mailboxes.workspace_id = ? AND mailboxes.active = true AND ((mailboxes.triage_eligible = true AND (COALESCE(TRIM(mailboxes.routing_prompt), '') <> '' OR COALESCE(TRIM(mailboxes.description), '') <> '')) OR EXISTS (SELECT 1 FROM support_triage_rules rules WHERE rules.workspace_id = mailboxes.workspace_id AND rules.target_mailbox_id = mailboxes.id AND rules.active = true))", workspaceID).
			Count(&evidence.AutomaticRoutingCount).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read automatic support routing evidence: %w", err)
		}
	}
	setupCounts := []struct {
		table string
		where string
		dest  *int64
	}{
		{"pm_epics", "workspace_id = ? AND archived = false AND COALESCE(owner_member_id, owner_id) IS NOT NULL AND planned_start_date IS NOT NULL AND deadline IS NOT NULL", &evidence.PlannedProjectCount},
		{"pm_sprints", "workspace_id = ? AND archived = false AND start_date IS NOT NULL AND end_date IS NOT NULL AND EXISTS (SELECT 1 FROM pm_tasks WHERE pm_tasks.workspace_id = pm_sprints.workspace_id AND pm_tasks.sprint_id = pm_sprints.id AND pm_tasks.archived = false)", &evidence.PlannedSprintCount},
		{"pm_tasks", "workspace_id = ? AND archived = false AND (epic_id IS NOT NULL OR sprint_id IS NOT NULL) AND EXISTS (SELECT 1 FROM pm_task_owners WHERE pm_task_owners.task_id = pm_tasks.id)", &evidence.AssignedProjectTaskCount},
		{"docs_spaces", "workspace_id = ? AND deleted_at IS NULL AND is_system = false AND type = 'external_capable'", &evidence.HelpCenterSpaceCount},
		{"docs_documents AS documents", "documents.workspace_id = ? AND documents.deleted_at IS NULL AND EXISTS (SELECT 1 FROM docs_spaces spaces WHERE spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'external_capable') AND EXISTS (SELECT 1 FROM docs_contents content WHERE content.document_id = documents.id AND (content.word_count > 0 OR COALESCE(TRIM(content.content_text), '') <> ''))", &evidence.HelpCenterContentCount},
		{"docs_helpcenter_configs", "workspace_id = ? AND is_published = true", &evidence.HelpCenterSiteCount},
		{"docs_spaces", "workspace_id = ? AND deleted_at IS NULL AND is_system = false AND type = 'internal'", &evidence.InternalDocsSpaceCount},
		{"docs_documents AS documents", "documents.workspace_id = ? AND documents.deleted_at IS NULL AND EXISTS (SELECT 1 FROM docs_spaces spaces WHERE spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'internal') AND EXISTS (SELECT 1 FROM docs_contents content WHERE content.document_id = documents.id AND (content.word_count > 0 OR COALESCE(TRIM(content.content_text), '') <> ''))", &evidence.InternalDocsContentCount},
		{"docs_documents AS documents", "documents.workspace_id = ? AND documents.deleted_at IS NULL AND documents.status = 'published' AND documents.published_at IS NOT NULL AND EXISTS (SELECT 1 FROM docs_spaces spaces WHERE spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'internal')", &evidence.InternalDocsPublishedCount},
		{"docs_documents AS documents", "documents.workspace_id = ? AND documents.deleted_at IS NULL AND documents.owner_id IS NOT NULL AND documents.next_review_at IS NOT NULL AND EXISTS (SELECT 1 FROM docs_spaces spaces WHERE spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'internal')", &evidence.InternalDocsOwnershipCount},
		{"agent_knowledge_sources AS sources", "sources.workspace_id = ? AND sources.sync_status = 'ready' AND (sources.indexed_documents > 0 OR sources.indexed_chunks > 0) AND EXISTS (SELECT 1 FROM docs_spaces spaces WHERE spaces.id = sources.space_id AND spaces.workspace_id = sources.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'internal') AND EXISTS (SELECT 1 FROM agents WHERE agents.id = sources.agent_id AND agents.workspace_id = sources.workspace_id)", &evidence.InternalAgentKnowledgeCount},
		{"crm_contacts", "workspace_id = ?", &evidence.CRMContactCount},
		{"crm_companies", "workspace_id = ?", &evidence.CRMCompanyCount},
		{"crm_pipelines AS pipelines", "pipelines.workspace_id = ? AND EXISTS (SELECT 1 FROM crm_pipeline_stages stages WHERE stages.pipeline_id = pipelines.id AND stages.stage_type = 'open') AND EXISTS (SELECT 1 FROM crm_pipeline_stages stages WHERE stages.pipeline_id = pipelines.id AND stages.stage_type = 'won') AND EXISTS (SELECT 1 FROM crm_pipeline_stages stages WHERE stages.pipeline_id = pipelines.id AND stages.stage_type = 'lost')", &evidence.CRMPipelineCount},
		{"crm_deals", "workspace_id = ? AND owner_member_id IS NOT NULL AND amount > 0 AND close_date IS NOT NULL", &evidence.CRMActionableDealCount},
		{"crm_email_accounts", "workspace_id = ? AND is_active = true AND status = 'connected'", &evidence.CRMConnectedEmailCount},
		{"crm_autonomy_settings", "workspace_id = ? AND enabled = true AND (auto_create_deals = true OR auto_progress_deals = true)", &evidence.CRMAutonomyEnabledCount},
		{"crm_suggestions AS suggestions", "suggestions.workspace_id = ? AND suggestions.suggestion_type IN ('deal_create', 'deal_advance') AND suggestions.execution_status = 'succeeded' AND suggestions.executed_at IS NOT NULL AND suggestions.object_id IS NOT NULL AND EXISTS (SELECT 1 FROM crm_deals WHERE crm_deals.id = suggestions.object_id AND crm_deals.workspace_id = suggestions.workspace_id)", &evidence.CRMSignalValueCount},
		{"agents", "workspace_id = ? AND is_system = false AND approval_mode = 'always'", &evidence.ApprovalGuardCount},
	}
	for _, query := range setupCounts {
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, workspaceID).Count(query.dest).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read high-value setup evidence from %s: %w", query.table, err)
		}
	}
	if len(supportSettings.WidgetHelpSpaceIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("docs_spaces").Where("workspace_id = ? AND deleted_at IS NULL AND type = 'external_capable' AND id IN ?", workspaceID, supportSettings.WidgetHelpSpaceIDs).Count(&evidence.HelpCenterWidgetCount).Error; err != nil {
			return model.SetupEvidence{}, fmt.Errorf("read help-center widget setup evidence: %w", err)
		}
	}
	evidence.HasCompanyContext = contextCount > 0
	evidence.SupportEmailInboxCount = emailRouteCount
	evidence.LiveChatInstallationCount = verifiedWidgetCount
	evidence.SupportChannelCount = verifiedWidgetCount + emailRouteCount
	evidence.KnowledgeSourceCount = agentKnowledgeCount + supportKnowledgeCount
	type triggerEvidenceRow struct {
		BindingID   string
		Status      string
		CompletedAt *time.Time
		FiredAt     time.Time
	}
	var triggerRows []triggerEvidenceRow
	if err := r.db.WithContext(ctx).Table("agent_trigger_executions AS executions").
		Select("executions.binding_id, executions.status, executions.completed_at, executions.fired_at").
		Joins("JOIN automation_rules ON "+setupAutomationRuleBindingJoin("automation_rules", "executions")+" AND automation_rules.enabled = true").
		Where("executions.workspace_id = ? AND executions.fired_at >= ?", workspaceID, since).
		Order("executions.fired_at ASC").Scan(&triggerRows).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read reliable automation evidence: %w", err)
	}
	type flowReliability struct {
		days       map[string]struct{}
		lastStatus string
	}
	flows := map[string]*flowReliability{}
	for _, row := range triggerRows {
		flow := flows[row.BindingID]
		if flow == nil {
			flow = &flowReliability{days: map[string]struct{}{}}
			flows[row.BindingID] = flow
		}
		flow.lastStatus = row.Status
		if row.Status == model.AgentTriggerExecutionStatusCompleted && row.CompletedAt != nil {
			flow.days[row.CompletedAt.In(location).Format("2006-01-02")] = struct{}{}
		}
	}
	for _, flow := range flows {
		if len(flow.days) >= 3 && flow.lastStatus == model.AgentTriggerExecutionStatusCompleted {
			evidence.ReliableAutomationCount++
		}
	}
	if evidence.ReliableAutomationCount > 0 && len(triggerRows) > 0 {
		for index := len(triggerRows) - 1; index >= 0; index-- {
			if triggerRows[index].Status == model.AgentTriggerExecutionStatusCompleted && triggerRows[index].CompletedAt != nil {
				evidence.TaskAchievementTimes["automation.reliable_unattended_value"] = *triggerRows[index].CompletedAt
				break
			}
		}
	}
	valuableRun := valuableAgentRunPredicate()
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Where(valuableRun, workspaceID, since).
		Count(&evidence.CompletedAgentRunCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read valuable agent run setup evidence: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Where(valuableRun+" AND runs.target_type IN ('task', 'epic', 'repository')", workspaceID, since).
		Count(&evidence.ProductAgentRunCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read product agent run setup evidence: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Select("COUNT(DISTINCT (runs.target_type || ':' || runs.target_id))").
		Where(valuableRun, workspaceID, since).Scan(&evidence.CompletedAgentTargetCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read distinct agent targets: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Select("COUNT(DISTINCT runs.triggered_by_user_id)").
		Where(valuableRun+" AND runs.triggered_by_user_id IS NOT NULL", workspaceID, since).Scan(&evidence.CompletedAgentMemberCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read distinct agent contributors: %w", err)
	}
	type workspaceRun struct {
		TargetType        string
		TargetID          string
		TriggeredByUserID *string
		CompletedAt       time.Time
	}
	var workspaceRuns []workspaceRun
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Select("runs.target_type, runs.target_id, runs.triggered_by_user_id, runs.completed_at").
		Where(valuableRun, workspaceID, since).Order("runs.completed_at ASC").Scan(&workspaceRuns).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read repeat agent-run evidence: %w", err)
	}
	days := map[string]bool{}
	targets := map[string]bool{}
	members := map[string]bool{}
	for _, run := range workspaceRuns {
		days[run.CompletedAt.In(location).Format("2006-01-02")] = true
		if run.TargetID != "" {
			targets[run.TargetType+":"+run.TargetID] = true
		}
		if run.TriggeredByUserID != nil && *run.TriggeredByUserID != "" {
			members[*run.TriggeredByUserID] = true
		}
		if len(days) > 1 || len(targets) > 1 || len(members) > 1 {
			current := evidence.TaskAchievementTimes["automation.repeat_assisted_value"]
			if current.IsZero() || run.CompletedAt.Before(current) {
				evidence.TaskAchievementTimes["automation.repeat_assisted_value"] = run.CompletedAt
			}
			break
		}
	}
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").Joins("JOIN agents ON agents.id = runs.agent_id").
		Where(valuableRun+" AND agents.is_system = false", workspaceID, since).
		Count(&evidence.CustomAgentSuccessCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read custom agent setup evidence: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("agent_runs AS runs").
		Joins("JOIN agents ON agents.id = runs.agent_id AND agents.workspace_id = runs.workspace_id").
		Joins("JOIN docs_documents documents ON documents.id = runs.target_id AND documents.workspace_id = runs.workspace_id AND documents.deleted_at IS NULL").
		Joins("JOIN docs_spaces spaces ON spaces.id = documents.space_id AND spaces.workspace_id = documents.workspace_id AND spaces.deleted_at IS NULL AND spaces.type = 'internal'").
		Where(valuableRun+" AND runs.target_type = 'document' AND agents.preset_key = 'documentation_agent'", workspaceID, since).
		Count(&evidence.InternalDocAgentSuccessCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read documentation agent setup evidence: %w", err)
	}
	if err := r.db.WithContext(ctx).Table("agent_trigger_executions AS executions").
		Joins("JOIN automation_rules ON "+setupAutomationRuleBindingJoin("automation_rules", "executions")).
		Where("executions.workspace_id = ? AND executions.status = 'completed' AND automation_rules.template_key = 'release_notes_writer' AND executions.completed_at >= ?", workspaceID, since).
		Count(&evidence.ReleaseNotesSuccessCount).Error; err != nil {
		return model.SetupEvidence{}, fmt.Errorf("read release notes setup evidence: %w", err)
	}
	timestampEvidence := []struct {
		key, table, column, where string
		args                      []any
	}{
		{"product.initial_work", "pm_tasks", "created_at", "workspace_id = ? AND archived = false", []any{workspaceID}},
		{"product.agent_result_used", "agent_runs AS runs", "runs.completed_at", valuableRun + " AND runs.target_type IN ('task', 'epic', 'repository')", []any{workspaceID, since}},
		{"product.sprint_closeout_reviewable", "pm_sprint_closeouts", "closed_at", "workspace_id = ? AND closed_at >= ?", []any{workspaceID, since}},
		{"product.release_notes_flow_succeeded", "agent_trigger_executions AS executions", "executions.completed_at", "executions.workspace_id = ? AND executions.status = 'completed' AND executions.completed_at >= ? AND EXISTS (SELECT 1 FROM automation_rules WHERE " + setupAutomationRuleBindingJoin("automation_rules", "executions") + " AND automation_rules.template_key = 'release_notes_writer')", []any{workspaceID, since}},
		{"support.pm_task_linked", "support_conversations", "updated_at", "workspace_id = ? AND linked_task_id IS NOT NULL AND updated_at >= ?", []any{workspaceID, since}},
		{"support.coverage_fix_applied", "support_coverage_recommendations", "updated_at", "workspace_id = ? AND status = 'applied' AND updated_at >= ?", []any{workspaceID, since}},
		{"automation.custom_agent_succeeded", "agent_runs AS runs", "runs.completed_at", valuableRun + " AND EXISTS (SELECT 1 FROM agents WHERE agents.id = runs.agent_id AND agents.is_system = false)", []any{workspaceID, since}},
	}
	for _, item := range timestampEvidence {
		if achievedAt, evidenceErr := r.firstEvidenceTime(ctx, item.table, item.column, item.where, item.args...); evidenceErr != nil {
			return model.SetupEvidence{}, evidenceErr
		} else if !achievedAt.IsZero() {
			evidence.TaskAchievementTimes[item.key] = achievedAt
		}
	}
	return evidence, nil
}

func valuableAgentRunPredicate() string {
	return "runs.workspace_id = ? AND runs.status = 'completed' AND runs.completed_at >= ? AND runs.target_id IS NOT NULL AND runs.target_type IN ('task', 'epic', 'repository', 'support_conversation', 'document', 'crm_deal', 'crm_contact', 'crm_company') AND (COALESCE(CAST(runs.output_summary AS TEXT), '{}') NOT IN ('{}', 'null', '') OR EXISTS (SELECT 1 FROM agent_run_artifacts WHERE agent_run_artifacts.run_id = runs.id))"
}

func (r *SetupRepository) MemberValuableAgentRunEvidence(ctx context.Context, workspaceID, userID string, since time.Time) (model.SetupMemberAgentEvidence, error) {
	var workspaceTimezone string
	if err := r.db.WithContext(ctx).Table("workspaces").Select("timezone").Where("id = ?", workspaceID).Scan(&workspaceTimezone).Error; err != nil {
		return model.SetupMemberAgentEvidence{}, fmt.Errorf("read member setup timezone: %w", err)
	}
	location, err := time.LoadLocation(workspaceTimezone)
	if err != nil {
		location = time.UTC
	}
	type memberRun struct {
		TargetType  string
		TargetID    string
		CompletedAt time.Time
	}
	var runs []memberRun
	valuableRun := "workspace_id = ? AND triggered_by_user_id = ? AND status = 'completed' AND completed_at >= ? AND (COALESCE(CAST(output_summary AS TEXT), '{}') NOT IN ('{}', 'null', '') OR EXISTS (SELECT 1 FROM agent_run_artifacts WHERE agent_run_artifacts.run_id = agent_runs.id))"
	if err := r.db.WithContext(ctx).Table("agent_runs").Select("target_type, target_id, completed_at").Where(valuableRun, workspaceID, userID, since).Order("completed_at ASC").Scan(&runs).Error; err != nil {
		return model.SetupMemberAgentEvidence{}, fmt.Errorf("read member agent contribution: %w", err)
	}
	result := model.SetupMemberAgentEvidence{RunCount: int64(len(runs))}
	if len(runs) == 0 {
		return result, nil
	}
	result.FirstRunAt = runs[0].CompletedAt
	days := map[string]bool{}
	targets := map[string]bool{}
	for _, run := range runs {
		days[run.CompletedAt.In(location).Format("2006-01-02")] = true
		targets[run.TargetType+":"+run.TargetID] = true
		if run.TargetType == "task" || run.TargetType == "epic" || run.TargetType == "repository" {
			result.ProductCount++
			if result.FirstProductAt.IsZero() {
				result.FirstProductAt = run.CompletedAt
			}
		}
		if result.RepeatRunAt.IsZero() && (len(days) > 1 || len(targets) > 1) {
			result.RepeatRunAt = run.CompletedAt
		}
	}
	result.RunDayCount = int64(len(days))
	result.RunTargetCount = int64(len(targets))
	return result, nil
}

func (r *SetupRepository) MemberSupportAIReplyEvidence(ctx context.Context, workspaceID, userID string, since time.Time) (int64, time.Time, error) {
	where := "messages.workspace_id = ? AND messages.sender_user_id = ? AND messages.created_at >= ? AND messages.deleted_at IS NULL AND messages.is_internal = false AND CAST(messages.metadata AS TEXT) LIKE '%\"ai_assisted\":true%' AND EXISTS (SELECT 1 FROM support_conversations conversations WHERE conversations.id = messages.conversation_id AND conversations.status = 'resolved' AND EXISTS (SELECT 1 FROM support_messages customer_message WHERE customer_message.conversation_id = conversations.id AND customer_message.sender_type = 'customer' AND customer_message.deleted_at IS NULL))"
	var count int64
	if err := r.db.WithContext(ctx).Table("support_messages AS messages").Where(where, workspaceID, userID, since).Count(&count).Error; err != nil {
		return 0, time.Time{}, fmt.Errorf("read member AI support contribution: %w", err)
	}
	achievedAt, err := r.firstEvidenceTime(ctx, "support_messages AS messages", "messages.created_at", where, workspaceID, userID, since)
	return count, achievedAt, err
}

func (r *SetupRepository) MemberApprovalEvidence(ctx context.Context, workspaceID, userID string, since time.Time) (int64, time.Time, error) {
	where := "runs.workspace_id = ? AND artifacts.artifact_type = ? AND artifacts.created_at >= ? AND artifacts.inline_content LIKE ?"
	args := []any{workspaceID, model.AgentRunArtifactTypeApprovedPreview, since, `%"approved_by":"` + userID + `"%`}
	query := r.db.WithContext(ctx).Table("agent_run_artifacts AS artifacts").Joins("JOIN agent_runs AS runs ON runs.id = artifacts.run_id").Where(where, args...)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, time.Time{}, fmt.Errorf("read member approval contribution: %w", err)
	}
	var timestamps []time.Time
	if err := query.Order("artifacts.created_at ASC").Pluck("artifacts.created_at", &timestamps).Error; err != nil {
		return 0, time.Time{}, fmt.Errorf("read member approval time: %w", err)
	}
	if len(timestamps) == 0 {
		return count, time.Time{}, nil
	}
	return count, timestamps[0], nil
}

func countDistinctLocalDays(timestamps []time.Time, location *time.Location) int64 {
	days := make(map[string]struct{}, len(timestamps))
	for _, timestamp := range timestamps {
		if !timestamp.IsZero() {
			days[timestamp.In(location).Format("2006-01-02")] = struct{}{}
		}
	}
	return int64(len(days))
}

func firstAndSecondDistinctLocalDay(timestamps []time.Time, location *time.Location) (time.Time, time.Time) {
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
	var first time.Time
	firstDay := ""
	for _, timestamp := range timestamps {
		if timestamp.IsZero() {
			continue
		}
		day := timestamp.In(location).Format("2006-01-02")
		if first.IsZero() {
			first = timestamp
			firstDay = day
			continue
		}
		if day != firstDay {
			return first, timestamp
		}
	}
	return first, time.Time{}
}

func (r *SetupRepository) firstEvidenceTime(ctx context.Context, table, column, where string, args ...any) (time.Time, error) {
	var timestamps []time.Time
	if err := r.db.WithContext(ctx).Table(table).Where(where, args...).Order(column+" ASC").Limit(1).Pluck(column, &timestamps).Error; err != nil {
		return time.Time{}, fmt.Errorf("read setup evidence timestamp from %s: %w", table, err)
	}
	if len(timestamps) == 0 {
		return time.Time{}, nil
	}
	return timestamps[0], nil
}

func setupAutomationRuleBindingJoin(ruleAlias, executionAlias string) string {
	return fmt.Sprintf("CAST(%s.id AS TEXT) = %s.binding_id", ruleAlias, executionAlias)
}
