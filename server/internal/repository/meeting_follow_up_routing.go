package repository

import (
	"context"
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSituationWorkspace(tx, original.WorkspaceID); err != nil {
			return err
		}
		var current model.CRMSuggestion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", original.WorkspaceID, original.ID).Take(&current).Error; err != nil {
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
		return tx.Model(&model.CRMSuggestion{}).Where("workspace_id = ? AND id = ? AND status = ?", original.WorkspaceID, original.ID, model.CRMSuggestionStatusPending).
			Where(PMAISuggestionRoutingEligible(tx, "crm_suggestions")).UpdateColumn("context", current.Context).Error
	})
}

// CRMVisibleMeetingFollowUpsSQL preserves unknown/customer work and complex linked lifecycles.
func CRMVisibleMeetingFollowUpsSQL(db *gorm.DB, alias string) string {
	return "NOT (" + alias + ".suggestion_type = 'follow_up' AND COALESCE(" + alias + ".object_type, '') = 'meeting' AND COALESCE(" + alias + ".context->>'meeting_follow_up_scope', '') = 'internal' AND (COALESCE(" + alias + ".context->>'meeting_follow_up_reviewed_in', '') = 'my_work' OR " + PMAISuggestionRoutingEligible(db, alias) + "))"
}
