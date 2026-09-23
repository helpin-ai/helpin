package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SampleDataRepository tracks and removes the records created by the sample
// data loader.
type SampleDataRepository struct {
	db *gorm.DB
}

// NewSampleDataRepository creates a SampleDataRepository.
func NewSampleDataRepository(db *gorm.DB) *SampleDataRepository {
	return &SampleDataRepository{db: db}
}

// NotSampleDataSQL returns a predicate that excludes rows recorded as sample
// data. column must be a trusted, qualified primary-key column such as
// "pm_tasks.id"; it is never user input.
func NotSampleDataSQL(column string) string {
	return "NOT EXISTS (SELECT 1 FROM sample_data_items sample_data WHERE sample_data.entity_id = " + column + ")"
}

// DB returns the underlying connection.
func (r *SampleDataRepository) DB() *gorm.DB {
	return r.db
}

// WithTx returns a repository bound to tx.
func (r *SampleDataRepository) WithTx(tx *gorm.DB) *SampleDataRepository {
	return &SampleDataRepository{db: tx}
}

// LockWorkspace serializes sample data loads and removals for one workspace
// until the surrounding transaction ends. It is a no-op outside PostgreSQL.
func (r *SampleDataRepository) LockWorkspace(ctx context.Context, workspaceID string) error {
	if r.db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := r.db.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "sample_data:"+workspaceID).Error; err != nil {
		return fmt.Errorf("lock sample data workspace: %w", err)
	}
	return nil
}

// Track records sample entities.
func (r *SampleDataRepository) Track(ctx context.Context, items []model.SampleDataItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&items).Error; err != nil {
		return fmt.Errorf("track sample data: %w", err)
	}
	return nil
}

// ListItems returns every sample record of a workspace, oldest first.
func (r *SampleDataRepository) ListItems(ctx context.Context, workspaceID string) ([]model.SampleDataItem, error) {
	var items []model.SampleDataItem
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list sample data: %w", err)
	}
	return items, nil
}

// DeleteItems removes tracking rows.
func (r *SampleDataRepository) DeleteItems(ctx context.Context, workspaceID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Delete(&model.SampleDataItem{}).Error; err != nil {
		return fmt.Errorf("untrack sample data: %w", err)
	}
	return nil
}

// ErrSampleEntityRetained reports that a sample container still holds records
// that are not sample data, so it was kept.
var ErrSampleEntityRetained = errors.New("sample entity retained because it holds non-sample records")

// DeleteEntity permanently deletes one sample entity and the rows owned by it.
// Containers (spaces, pipelines, teams) that still hold non-sample records are
// kept and ErrSampleEntityRetained is returned.
func (r *SampleDataRepository) DeleteEntity(ctx context.Context, workspaceID, entityType, entityID string) error {
	spec, ok := sampleEntitySpecs[entityType]
	if !ok {
		return fmt.Errorf("unknown sample entity type %q", entityType)
	}
	db := r.db.WithContext(ctx)
	for _, guard := range spec.retainIf {
		if !r.hasTable(guard.table) {
			continue
		}
		var count int64
		query := db.Table(guard.table).Where(guard.column+" = ?", entityID)
		if r.hasTable("sample_data_items") {
			query = query.Where(NotSampleDataSQL(guard.table + ".id"))
		}
		if err := query.Count(&count).Error; err != nil {
			return fmt.Errorf("check %s usage of sample %s: %w", guard.table, entityType, err)
		}
		if count > 0 {
			return ErrSampleEntityRetained
		}
	}
	if len(spec.polymorphicTypes) > 0 {
		if err := r.deletePolymorphic(ctx, spec.polymorphicTypes, entityID); err != nil {
			return err
		}
	}
	for _, ref := range spec.nullify {
		if !r.hasTable(ref.table) {
			continue
		}
		if err := db.Exec("UPDATE "+ref.table+" SET "+ref.column+" = NULL WHERE "+ref.column+" = ?", entityID).Error; err != nil {
			return fmt.Errorf("detach %s.%s from sample %s: %w", ref.table, ref.column, entityType, err)
		}
	}
	for _, child := range spec.children {
		if !r.hasTable(child.table) {
			continue
		}
		if err := db.Exec("DELETE FROM "+child.table+" WHERE "+child.column+" = ?", entityID).Error; err != nil {
			return fmt.Errorf("delete %s rows of sample %s: %w", child.table, entityType, err)
		}
	}
	for _, assoc := range spec.associationTypes {
		if !r.hasTable("crm_associations") {
			break
		}
		if err := db.Exec("DELETE FROM crm_associations WHERE workspace_id = ? AND ((from_object_type = ? AND from_object_id = ?) OR (to_object_type = ? AND to_object_id = ?))",
			workspaceID, assoc, entityID, assoc, entityID).Error; err != nil {
			return fmt.Errorf("delete associations of sample %s: %w", entityType, err)
		}
	}
	if err := db.Exec("DELETE FROM "+spec.table+" WHERE workspace_id = ? AND id = ?", workspaceID, entityID).Error; err != nil {
		return fmt.Errorf("delete sample %s: %w", entityType, err)
	}
	return nil
}

// PurgeLiveTranslationQueue removes queued live-translation work for sample
// customer messages so no background worker (and no AI provider) processes
// them. It runs inside the load transaction, before the rows become visible.
func (r *SampleDataRepository) PurgeLiveTranslationQueue(ctx context.Context, conversationID string) error {
	if !r.hasTable("support_live_messages") {
		return nil
	}
	if err := r.db.WithContext(ctx).Exec("DELETE FROM support_live_messages WHERE conversation_id = ?", conversationID).Error; err != nil {
		return fmt.Errorf("clear sample live translation queue: %w", err)
	}
	return nil
}

// PreferredTeamID returns the oldest team the member belongs to, falling back
// to the workspace's oldest team. It returns "" when the workspace has none.
func (r *SampleDataRepository) PreferredTeamID(ctx context.Context, workspaceID, memberID string) (string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).Table("workspace_teams AS teams").
		Joins("JOIN team_workspace_memberships memberships ON memberships.team_id = teams.id").
		Where("teams.workspace_id = ? AND memberships.workspace_member_id = ?", workspaceID, memberID).
		Order("teams.created_at ASC, teams.id ASC").Limit(1).Pluck("teams.id", &ids).Error; err != nil {
		return "", fmt.Errorf("find member team: %w", err)
	}
	if len(ids) > 0 {
		return ids[0], nil
	}
	if err := r.db.WithContext(ctx).Model(&model.WorkspaceTeam{}).Where("workspace_id = ?", workspaceID).
		Order("created_at ASC, id ASC").Limit(1).Pluck("id", &ids).Error; err != nil {
		return "", fmt.Errorf("find workspace team: %w", err)
	}
	if len(ids) > 0 {
		return ids[0], nil
	}
	return "", nil
}

// CreateTeam creates a team and adds the member to it.
func (r *SampleDataRepository) CreateTeam(ctx context.Context, team *model.WorkspaceTeam, memberID string) error {
	if err := r.db.WithContext(ctx).Create(team).Error; err != nil {
		return fmt.Errorf("create sample team: %w", err)
	}
	membership := model.TeamWorkspaceMembership{TeamID: team.ID, WorkspaceMemberID: memberID, Role: "member"}
	if err := r.db.WithContext(ctx).Create(&membership).Error; err != nil {
		return fmt.Errorf("add sample team member: %w", err)
	}
	return nil
}

// CreateDisabledAutomationRule inserts a sample Flow with enabled = false.
// Writing the column explicitly bypasses the model's default:true tag, so the
// rule is never briefly enabled; no schedule is registered and the rule engine
// only matches enabled rules, so it cannot fire until someone turns it on.
func (r *SampleDataRepository) CreateDisabledAutomationRule(ctx context.Context, rule *model.AutomationRule) error {
	if rule == nil || rule.ID == "" {
		return fmt.Errorf("sample Flow requires an identity")
	}
	rule.Enabled = false
	if err := r.db.WithContext(ctx).Model(&model.AutomationRule{}).Create(map[string]any{
		"id": rule.ID, "workspace_id": rule.WorkspaceID, "name": rule.Name, "description": rule.Description,
		"enabled": false, "workflow_id": rule.WorkflowID, "trigger_type": rule.TriggerType,
		"trigger_config": rule.TriggerConfig, "action_type": rule.ActionType, "action_config": rule.ActionConfig,
		"position": rule.Position, "stop_on_match": false, "created_by": rule.CreatedBy,
		"created_at": rule.CreatedAt, "updated_at": rule.UpdatedAt,
	}).Error; err != nil {
		return fmt.Errorf("create sample Flow: %w", err)
	}
	return nil
}

// SetConversationState stamps lifecycle fields that the sample loader writes
// directly, without running status-change side effects.
func (r *SampleDataRepository) SetConversationState(ctx context.Context, workspaceID, conversationID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_at"]; !ok {
		fields["updated_at"] = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
		UpdateColumns(fields).Error; err != nil {
		return fmt.Errorf("set sample conversation state: %w", err)
	}
	return nil
}

func (r *SampleDataRepository) deletePolymorphic(ctx context.Context, entityTypes []string, entityID string) error {
	db := r.db.WithContext(ctx)
	if r.hasTable("notifications") {
		if r.hasTable("notification_deliveries") && r.hasTable("notification_events") {
			if err := db.Exec("DELETE FROM notification_deliveries WHERE notification_event_id IN (SELECT e.id FROM notification_events e JOIN notifications n ON n.id = e.notification_id WHERE n.entity_type IN ? AND n.entity_id = ?)", entityTypes, entityID).Error; err != nil {
				return fmt.Errorf("delete sample notification deliveries: %w", err)
			}
		}
		if r.hasTable("notification_events") {
			if err := db.Exec("DELETE FROM notification_events WHERE notification_id IN (SELECT id FROM notifications WHERE entity_type IN ? AND entity_id = ?)", entityTypes, entityID).Error; err != nil {
				return fmt.Errorf("delete sample notification events: %w", err)
			}
		}
	}
	for _, table := range []string{"notifications", "pm_activity_log", "pm_comments", "entity_followers", "crm_entity_summaries", "crm_signal_motion_states"} {
		if !r.hasTable(table) {
			continue
		}
		if err := db.Exec("DELETE FROM "+table+" WHERE entity_type IN ? AND entity_id = ?", entityTypes, entityID).Error; err != nil {
			return fmt.Errorf("delete sample %s rows: %w", table, err)
		}
	}
	return nil
}

func (r *SampleDataRepository) hasTable(table string) bool {
	return r.db.Migrator().HasTable(table)
}

type sampleColumnRef struct {
	table  string
	column string
}

type sampleEntitySpec struct {
	table            string
	children         []sampleColumnRef
	nullify          []sampleColumnRef
	retainIf         []sampleColumnRef
	polymorphicTypes []string
	associationTypes []string
}

// sampleEntitySpecs lists, per sample entity type, the rows owned by the entity
// that are deleted with it and the references that are detached. Table and
// column names are constants, never user input.
var sampleEntitySpecs = map[string]sampleEntitySpec{
	// A sample Flow that someone turned on and that has run keeps its run
	// history, so the Flow is kept rather than orphaning that history.
	model.SampleEntityAutomationRule: {
		table:    "automation_rules",
		retainIf: []sampleColumnRef{{"agent_trigger_executions", "binding_id"}},
	},
	model.SampleEntitySupportConversation: {
		table: "support_conversations",
		children: []sampleColumnRef{
			{"support_live_messages", "conversation_id"},
			{"support_translations", "conversation_id"},
			{"support_translation_conversations", "conversation_id"},
			{"support_conversation_tags", "conversation_id"},
			{"support_conversation_triage_events", "conversation_id"},
			{"support_conversation_triage", "conversation_id"},
			// support_conversation_user_states and the inbox projection tables
			// cascade from support_conversations; their triggers expect that path.
			{"support_events", "conversation_id"},
			{"support_messages", "conversation_id"},
		},
		polymorphicTypes: []string{"support_conversation", "conversation"},
	},
	model.SampleEntityPMTask: {
		table: "pm_tasks",
		children: []sampleColumnRef{
			{"pm_task_owners", "task_id"},
			{"pm_task_labels", "task_id"},
			{"pm_task_followers", "task_id"},
			{"pm_checklist_items", "task_id"},
			{"pm_external_links", "task_id"},
			{"pm_task_update_reads", "task_id"},
			{"pm_task_standing_briefs", "task_id"},
			{"pm_task_brief_suggestion_dismissals", "task_id"},
			{"task_delivery_targets", "task_id"},
			{"task_git_links", "task_id"},
			{"pm_task_links", "source_task_id"},
			{"pm_task_links", "target_task_id"},
		},
		nullify:          []sampleColumnRef{{"support_conversations", "linked_task_id"}},
		polymorphicTypes: []string{"task", "story"},
	},
	model.SampleEntityPMEpic: {
		table: "pm_epics",
		children: []sampleColumnRef{
			{"pm_epic_labels", "epic_id"},
			{"pm_epic_objectives", "epic_id"},
			{"epic_delivery_targets", "epic_id"},
		},
		nullify:          []sampleColumnRef{{"pm_tasks", "epic_id"}, {"pm_task_templates", "epic_id"}},
		polymorphicTypes: []string{"epic"},
	},
	model.SampleEntityDocsDocument: {
		table: "docs_documents",
		children: []sampleColumnRef{
			{"docs_contents", "document_id"},
			{"docs_blocks", "document_id"},
			{"docs_versions", "document_id"},
			{"docs_links", "document_id"},
			{"docs_comments", "document_id"},
			{"docs_document_keys", "document_id"},
			{"docs_chunks", "document_id"},
			{"docs_slug_aliases", "document_id"},
			{"docs_review_queue", "document_id"},
			{"docs_helpcenter_search_entries", "document_id"},
			{"docs_helpcenter_article_translations", "document_id"},
			{"docs_helpcenter_article_publications", "document_id"},
			{"docs_helpcenter_articles", "document_id"},
			{"docs_article_feedback", "document_id"},
			{"docs_ai_section_candidates", "document_id"},
		},
		polymorphicTypes: []string{"doc", "document", "docs_document"},
	},
	model.SampleEntityDocsSpace: {
		table:    "docs_spaces",
		retainIf: []sampleColumnRef{{"docs_documents", "space_id"}},
		children: []sampleColumnRef{
			{"docs_space_teams", "space_id"},
			{"docs_helpcenter_collection_translations", "space_id"},
			{"docs_helpcenter_space_translations", "space_id"},
			{"docs_chunks", "space_id"},
			{"docs_collections", "space_id"},
		},
	},
	model.SampleEntityCRMDeal: {
		table: "crm_deals",
		children: []sampleColumnRef{
			{"crm_activities", "deal_id"},
			{"crm_deal_health_scores", "deal_id"},
		},
		polymorphicTypes: []string{"deal", "crm_deal"},
		associationTypes: []string{model.CRMObjectDeal},
	},
	model.SampleEntityCRMContact: {
		table: "crm_contacts",
		children: []sampleColumnRef{
			{"crm_activities", "contact_id"},
			{"crm_identity_links", "contact_id"},
		},
		nullify:          []sampleColumnRef{{"support_conversations", "crm_contact_id"}},
		polymorphicTypes: []string{"contact", "crm_contact"},
		associationTypes: []string{model.CRMObjectContact},
	},
	model.SampleEntityCRMCompany: {
		table: "crm_companies",
		children: []sampleColumnRef{
			{"crm_activities", "company_id"},
			{"crm_identity_links", "company_id"},
		},
		nullify: []sampleColumnRef{
			{"support_conversations", "crm_company_id"},
			{"support_widget_sessions", "crm_company_id"},
		},
		polymorphicTypes: []string{"company", "crm_company"},
		associationTypes: []string{model.CRMObjectCompany},
	},
	model.SampleEntityCRMPipeline: {
		table:    "crm_pipelines",
		retainIf: []sampleColumnRef{{"crm_deals", "pipeline_id"}},
		children: []sampleColumnRef{{"crm_pipeline_stages", "pipeline_id"}},
	},
	model.SampleEntityWorkspaceTeam: {
		table: "workspace_teams",
		retainIf: []sampleColumnRef{
			{"pm_tasks", "team_id"},
			{"pm_epics", "team_id"},
			{"pm_sprints", "team_id"},
			{"docs_spaces", "team_id"},
			{"pm_workflows", "team_id"},
		},
		children: []sampleColumnRef{
			{"team_workspace_memberships", "team_id"},
			{"docs_space_teams", "team_id"},
		},
	},
}
