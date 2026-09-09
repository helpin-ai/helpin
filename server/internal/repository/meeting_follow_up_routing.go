package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MeetingFollowUpsToRoute returns a bounded, fair sample of undecided legacy drafts.
// Linked customer work is retained unless it is an untouched automatic projection.
func (r *CRMSuggestionRepository) MeetingFollowUpsToRoute(ctx context.Context, limit int) ([]model.CRMSuggestion, error) {
	if limit < 1 || limit > 3 {
		limit = 3
	}
	var items []model.CRMSuggestion
	err := r.db.WithContext(ctx).Table("crm_suggestions a").Select("a.*").
		Where("a.status = ? AND a.suggestion_type = ? AND a.object_type = ? AND a.object_id IS NOT NULL", model.CRMSuggestionStatusPending, model.CRMSuggestionFollowUp, model.CRMObjectMeeting).
		Where("COALESCE(a.context->>?, '') = ''", model.MeetingFollowUpRoutingVersionKey).
		Where(PMAISuggestionRoutingEligible(r.db, "a")).
		Order("RANDOM()").Limit(limit).Find(&items).Error
	return items, err
}

// RouteMeetingFollowUp updates only routing metadata on the exact still-pending draft.
// A concurrent review or edit wins; timestamps, execution and decision history are untouched.
func (r *CRMSuggestionRepository) RouteMeetingFollowUp(ctx context.Context, original model.CRMSuggestion, scope string) error {
	if scope != "internal" && scope != "customer" && scope != "uncertain" {
		return fmt.Errorf("invalid meeting follow-up scope")
	}
	_, err := r.routeMeetingFollowUp(ctx, original, scope, "")
	return err
}

func (r *CRMSuggestionRepository) RouteMeetingFollowUpForActor(ctx context.Context, original model.CRMSuggestion, scope, user string) (bool, error) {
	return r.routeMeetingFollowUp(ctx, original, scope, user)
}

func (r *CRMSuggestionRepository) routeMeetingFollowUp(ctx context.Context, original model.CRMSuggestion, scope, user string) (bool, error) {
	if scope != "internal" && scope != "customer" && scope != "uncertain" {
		return false, fmt.Errorf("invalid meeting follow-up scope")
	}
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, original.WorkspaceID); err != nil {
			return err
		}
		var current model.CRMSuggestion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", original.WorkspaceID, original.ID).Take(&current).Error; err != nil {
			if user != "" && errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if current.Status != model.CRMSuggestionStatusPending || model.CRMSuggestionRevision(current) != model.CRMSuggestionRevision(original) {
			return nil
		}
		if current.Context == nil {
			current.Context = model.JSONB{}
		}
		current.Context[model.MeetingFollowUpScopeKey] = scope
		current.Context[model.MeetingFollowUpRoutingVersionKey] = model.MeetingFollowUpRoutingVersion
		query := tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ? AND status = ?", original.WorkspaceID, original.ID, model.CRMSuggestionStatusPending).Where(PMAISuggestionRoutingEligible(tx, "crm_suggestions"))
		if user != "" {
			query = query.Where(meetingFollowUpActorAccess("crm_suggestions"), user)
		}
		update := query.UpdateColumn("context", current.Context)
		changed = update.RowsAffected == 1
		return update.Error
	})
	return changed, err
}

// CRMVisibleMeetingFollowUpsSQL preserves unknown/customer work and complex linked lifecycles.
func CRMVisibleMeetingFollowUpsSQL(db *gorm.DB, alias string) string {
	return "NOT (" + alias + ".suggestion_type = 'follow_up' AND COALESCE(" + alias + ".object_type, '') = 'meeting' AND COALESCE(" + alias + ".context->>'meeting_follow_up_scope', '') = 'internal' AND (COALESCE(" + alias + ".context->>'meeting_follow_up_reviewed_in', '') = 'my_work' OR " + PMAISuggestionRoutingEligible(db, alias) + "))"
}

// The caller must have current membership and visibility of the source meeting.
func meetingFollowUpActorAccess(alias string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM crm_meetings visible_meeting
 JOIN workspace_members caller ON caller.workspace_id = visible_meeting.workspace_id AND caller.user_id = ? AND caller.status = 'active'
 WHERE visible_meeting.workspace_id = %[1]s.workspace_id AND visible_meeting.id = %[1]s.object_id
 AND (visible_meeting.visibility = 'workspace' OR visible_meeting.owner_member_id = caller.id OR visible_meeting.created_by = caller.user_id))`, alias)
}

// Read one extra candidate to provide a stable continuation cursor. Include
// ineligible drafts so the action can explain why they cannot move.
func (r *CRMSuggestionRepository) MeetingFollowUpsToRecheck(ctx context.Context, ws, user, cursor string) ([]model.MeetingFollowUpRoutingCandidate, error) {
	var items []model.MeetingFollowUpRoutingCandidate
	query := r.db.WithContext(ctx).Table("crm_suggestions a").Select("a.*, "+PMAISuggestionRoutingEligible(r.db, "a")+" AS routing_eligible").
		Where("a.workspace_id = ? AND a.status = 'pending' AND a.suggestion_type = 'follow_up' AND a.object_type = 'meeting' AND a.object_id IS NOT NULL", ws).
		Where("a.executed_at IS NULL AND COALESCE(a.execution_status, 'pending') IN ('', 'pending')").
		Where("COALESCE(a.context->>?, '') = ''", model.MeetingFollowUpRoutingVersionKey).
		Where(meetingFollowUpActorAccess("a"), user)
	if cursor != "" {
		query = query.Where("a.id > ?", cursor)
	}
	err := query.Order("a.id ASC").Limit(4).Find(&items).Error
	return items, err
}
