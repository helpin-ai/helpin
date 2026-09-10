package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPMAISuggestionStale = errors.New("suggestion changed; refresh before reviewing")

type PMAISuggestionRepository struct{ db *gorm.DB }

func NewPMAISuggestionRepository(db *gorm.DB) *PMAISuggestionRepository {
	return &PMAISuggestionRepository{db: db}
}

// These additive schema dependencies are stable for the lifetime of an API
// connection pool. Cache only existing tables so migration/test setup can still
// add a table after the first query, without catalog queries on every request.
var pmAISuggestionTables sync.Map

type pmAISuggestionTableKey struct {
	pool gorm.ConnPool
	name string
}

func pmAISuggestionHasTable(db *gorm.DB, table interface{ TableName() string }) bool {
	key := pmAISuggestionTableKey{pool: db.Config.ConnPool, name: table.TableName()}
	if _, ok := pmAISuggestionTables.Load(key); ok {
		return true
	}
	if db.Migrator().HasTable(table) {
		pmAISuggestionTables.Store(key, true)
		return true
	}
	return false
}

// PMAISuggestionRoutingEligible is shared by both queues and migration. Alias is
// a repository-owned SQL identifier, never request input. Untouched automatic
// projections can move; adopted customer work must retain its existing workflow.
func PMAISuggestionRoutingEligible(db *gorm.DB, alias string) string {
	predicates := []string{fmt.Sprintf(`COALESCE(%[1]s.context ->> 'situation_id', '') = '' AND COALESCE(%[1]s.context ->> 'task_id', '') = '' AND COALESCE(%[1]s.context ->> 'action_item_id', '') = '' AND COALESCE(%[1]s.context ->> 'follow_up_id', '') = ''`, alias)}
	if pmAISuggestionHasTable(db, &model.CRMMeeting{}) {
		predicates = append(predicates, fmt.Sprintf(`EXISTS (SELECT 1 FROM crm_meetings routing_meeting
         JOIN workspace_members recipient ON recipient.workspace_id = routing_meeting.workspace_id AND recipient.status = 'active' AND recipient.user_id IS NOT NULL
         WHERE routing_meeting.workspace_id = %[1]s.workspace_id AND routing_meeting.id = %[1]s.object_id
         AND ((%[1]s.user_id IS NOT NULL AND %[1]s.user_id = recipient.user_id)
          OR (%[1]s.user_id IS NULL AND routing_meeting.owner_member_id IS NOT NULL AND routing_meeting.owner_member_id = recipient.id)
          OR (%[1]s.user_id IS NULL AND routing_meeting.owner_member_id IS NULL AND routing_meeting.created_by = recipient.user_id))
         AND (routing_meeting.visibility = 'workspace' OR routing_meeting.owner_member_id = recipient.id OR routing_meeting.created_by = recipient.user_id))`, alias))
	}
	if pmAISuggestionHasTable(db, &model.CRMPlaybookActionIntent{}) {
		predicates = append(predicates, fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM crm_playbook_action_intents intent WHERE intent.workspace_id = %[1]s.workspace_id AND intent.suggestion_id = %[1]s.id)`, alias))
	}
	if pmAISuggestionHasTable(db, &model.CRMSituationReference{}) {
		automation := ""
		if pmAISuggestionHasTable(db, &model.CRMPlaybookAutomationBinding{}) {
			automation = ` AND NOT EXISTS (SELECT 1 FROM crm_playbook_automation_bindings binding WHERE binding.workspace_id = situation.workspace_id AND binding.situation_id = situation.id)`
		}
		predicates = append(predicates, fmt.Sprintf(`NOT EXISTS (
   SELECT 1 FROM crm_situation_references ref
   LEFT JOIN crm_situations situation ON situation.workspace_id = ref.workspace_id AND situation.id = ref.situation_id
   WHERE ref.workspace_id = %[1]s.workspace_id AND ref.kind = 'suggestion' AND ref.source_id = %[1]s.id
   AND NOT (situation.id IS NOT NULL AND situation.origin_kind = 'suggestion'
    AND situation.creation_key = 'source:suggestion:' || %[1]s.id
    AND situation.lifecycle = 'open' AND situation.revision = 1
    AND situation.company_id IS NULL AND situation.contact_id IS NULL AND situation.deal_id IS NULL
    AND situation.playbook_id IS NULL AND situation.playbook_version_id IS NULL
    AND situation.next_checkpoint_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM crm_situation_references other WHERE other.workspace_id = ref.workspace_id AND other.situation_id = ref.situation_id AND (other.kind <> 'suggestion' OR other.source_id <> %[1]s.id))%[2]s))`, alias, automation))
	}
	return "(" + strings.Join(predicates, " AND ") + ")"
}

func (r *PMAISuggestionRepository) personalScope(db *gorm.DB, ws, user string) *gorm.DB {
	return db.Table("crm_suggestions a").
		Joins("JOIN crm_meetings meeting ON meeting.workspace_id = a.workspace_id AND meeting.id = a.object_id").
		Joins("JOIN workspace_members member ON member.workspace_id = a.workspace_id AND member.user_id = ? AND member.status = 'active'", user).
		Where("a.workspace_id = ? AND a.suggestion_type = 'follow_up' AND a.object_type = 'meeting'", ws).
		Where("a.context ->> 'meeting_follow_up_scope' = 'internal'").
		Where(`((a.user_id IS NOT NULL AND a.user_id = member.user_id) OR
   (a.user_id IS NULL AND meeting.owner_member_id IS NOT NULL AND meeting.owner_member_id = member.id) OR
   (a.user_id IS NULL AND meeting.owner_member_id IS NULL AND meeting.created_by = member.user_id))`).
		Where("(meeting.visibility = 'workspace' OR meeting.owner_member_id = member.id OR meeting.created_by = member.user_id)")
}

func (r *PMAISuggestionRepository) scope(db *gorm.DB, ws, user string) *gorm.DB {
	return r.personalScope(db, ws, user).Where(PMAISuggestionRoutingEligible(db, "a"))
}

func (r *PMAISuggestionRepository) List(ctx context.Context, ws, user string, page int) ([]model.PMAISuggestionItem, int64, error) {
	query := r.scope(r.db.WithContext(ctx), ws, user).Where("a.status = 'pending' AND a.executed_at IS NULL AND COALESCE(a.execution_status, 'pending') IN ('', 'pending')")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []pmAISuggestionRow
	err := query.Select("a.id, a.title, a.created_at, meeting.id AS meeting_id, meeting.title AS meeting_title, meeting.actual_start_at, meeting.scheduled_start_at, meeting.created_at AS meeting_created_at").Order("a.created_at DESC, a.id DESC").Offset((page - 1) * 25).Limit(25).Scan(&rows).Error
	items := make([]model.PMAISuggestionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.item())
	}
	return items, total, err
}

type pmAISuggestionRow struct {
	model.CRMSuggestion `gorm:"embedded"`
	MeetingID           string
	MeetingTitle        string
	ActualStartAt       *time.Time
	ScheduledStartAt    *time.Time
	MeetingCreatedAt    time.Time
}

func (row pmAISuggestionRow) item() model.PMAISuggestionItem {
	at := row.ActualStartAt
	if at == nil {
		at = row.ScheduledStartAt
	}
	if at == nil {
		at = &row.MeetingCreatedAt
	}
	return model.PMAISuggestionItem{ID: row.ID, Title: row.Title, MeetingID: row.MeetingID, MeetingTitle: row.MeetingTitle, MeetingAt: at, CreatedAt: row.CreatedAt}
}
func (r *PMAISuggestionRepository) Get(ctx context.Context, ws, user, id string) (*model.PMAISuggestionDetail, error) {
	var row pmAISuggestionRow
	err := r.scope(r.db.WithContext(ctx), ws, user).Select("a.*, meeting.id AS meeting_id, meeting.title AS meeting_title, meeting.actual_start_at, meeting.scheduled_start_at, meeting.created_at AS meeting_created_at").Where("a.id = ? AND a.status = 'pending' AND a.executed_at IS NULL AND COALESCE(a.execution_status, 'pending') IN ('', 'pending')", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	subject, _ := row.Context["draft_subject"].(string)
	body, _ := row.Context["draft_body"].(string)
	if strings.TrimSpace(subject) == "" {
		subject = row.Title
	}
	if strings.TrimSpace(body) == "" && row.Description != nil {
		body = *row.Description
	}
	return &model.PMAISuggestionDetail{PMAISuggestionItem: row.item(), DraftSubject: subject, DraftBody: body, Revision: model.CRMSuggestionRevision(row.CRMSuggestion)}, nil
}

// Decide serializes with situation enrollment and lifecycle changes, then checks
// the exact canonical revision. Reviewing never executes the generated draft.
func (r *PMAISuggestionRepository) Decide(ctx context.Context, ws, user, id, revision, decision string) (*model.PMAISuggestionDecisionResult, error) {
	var result *model.PMAISuggestionDecisionResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, ws); err != nil {
			return err
		}
		var suggestion model.CRMSuggestion
		// Lock only the canonical row, then enforce the full personal scope below.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", ws, id).Take(&suggestion).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if suggestion.ObjectID == nil || suggestion.ObjectType == nil || *suggestion.ObjectType != "meeting" {
			return nil
		}
		// Keep access facts stable until the review commits. A concurrent owner,
		// visibility or membership change must finish before admission or wait.
		var source struct{ ID string }
		if err := tx.Table("crm_meetings").Clauses(clause.Locking{Strength: "SHARE"}).Select("id").Where("workspace_id = ? AND id = ?", ws, *suggestion.ObjectID).Take(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var member struct{ ID string }
		if err := tx.Table("workspace_members").Clauses(clause.Locking{Strength: "SHARE"}).Select("id").Where("workspace_id = ? AND user_id = ? AND status = 'active'", ws, user).Take(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		// Repeated decisions remain conflicts, but only after personal access checks.
		var visible int64
		scoped := r.personalScope(tx, ws, user)
		if err := scoped.Where("a.id = ?", id).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 0 {
			return nil
		}
		if suggestion.Status != model.CRMSuggestionStatusPending || suggestion.ExecutedAt != nil || (suggestion.ExecutionStatus != "" && suggestion.ExecutionStatus != model.CRMSuggestionExecutionPending) || model.CRMSuggestionRevision(suggestion) != revision {
			return ErrPMAISuggestionStale
		}
		visible = 0
		if err := r.scope(tx, ws, user).Where("a.id = ?", id).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 0 {
			return ErrPMAISuggestionStale
		}
		status, execution := model.CRMSuggestionStatusAccepted, model.CRMSuggestionExecutionManualRequired
		reviewContext := make(model.JSONB, len(suggestion.Context)+1)
		for key, value := range suggestion.Context {
			reviewContext[key] = value
		}
		reviewContext["meeting_follow_up_reviewed_in"] = "my_work"
		updates := map[string]any{"status": status, "execution_status": execution, "context": reviewContext, "updated_at": time.Now().UTC()}
		if decision == "dismiss" {
			status = model.CRMSuggestionStatusDismissed
			execution = suggestion.ExecutionStatus
			updates["status"] = status
			delete(updates, "execution_status")
			updates["dismissal_reason"] = "not_relevant"
		}
		updated := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ? AND status = ? AND updated_at = ?", ws, id, model.CRMSuggestionStatusPending, suggestion.UpdatedAt).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrPMAISuggestionStale
		}
		if err := r.closeProjection(tx, ws, user, suggestion, decision); err != nil {
			return err
		}
		result = &model.PMAISuggestionDecisionResult{ID: id, Status: status, ExecutionStatus: execution}
		return nil
	})
	return result, err
}

func (r *PMAISuggestionRepository) closeProjection(tx *gorm.DB, ws, user string, suggestion model.CRMSuggestion, decision string) error {
	if !pmAISuggestionHasTable(tx, &model.CRMSituationReference{}) {
		return nil
	}
	var situations []model.CRMSituation
	if err := tx.Table("crm_situations").Select("crm_situations.*").Joins("JOIN crm_situation_references ref ON ref.workspace_id = crm_situations.workspace_id AND ref.situation_id = crm_situations.id").Where("ref.workspace_id = ? AND ref.kind = 'suggestion' AND ref.source_id = ?", ws, suggestion.ID).Find(&situations).Error; err != nil {
		return err
	}
	if len(situations) == 0 {
		return nil
	}
	var member model.WorkspaceMember
	if err := tx.Where("workspace_id = ? AND user_id = ? AND status = 'active'", ws, user).Take(&member).Error; err != nil {
		return err
	}
	for _, current := range situations {
		before := model.CRMSituationState(current)
		after := before
		now := time.Now().UTC().Truncate(time.Microsecond)
		kind, basis, summary := "invalid", "human_assessment", "Internal meeting follow-up reviewed in My Work. No message sent or task created."
		if decision == "dismiss" {
			summary = "Internal meeting follow-up dismissed in My Work."
		}
		after.Lifecycle = model.CRMSituationClosed
		after.OutcomeKind = &kind
		after.OutcomeBasis = &basis
		after.OutcomeSummary = &summary
		after.ClosedAt = &now
		after.ClosedByMemberID = &member.ID
		receipt := model.CRMSituationChange{ID: uuid.NewString(), WorkspaceID: ws, SituationID: current.ID, Revision: current.Revision + 1, CommandKey: "pm-ai-review:" + suggestion.ID, CommandFingerprint: decision + ":" + model.CRMSuggestionRevision(suggestion), Operation: "close", ActorKind: "member", ActorMemberID: &member.ID, Reason: summary, Before: &before, After: after, CreatedAt: now}
		if err := saveSituationChange(tx, current, &receipt); err != nil {
			return err
		}
	}
	return nil
}
