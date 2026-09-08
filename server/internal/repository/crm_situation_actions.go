package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (r *CRMSituationRepository) situationActions(ctx context.Context, ws, situationID string) ([]model.CRMSuggestion, error) {
	var rows []struct {
		Suggestion model.CRMSuggestion `gorm:"embedded"`
		MemberID   *string
	}
	err := r.db.WithContext(ctx).Table("crm_suggestions a").Select("a.*, am.id AS member_id").
		Joins("JOIN crm_situation_references ref ON ref.workspace_id = a.workspace_id AND ref.kind = 'suggestion' AND ref.source_id = a.id").
		Joins("LEFT JOIN workspace_members am ON am.workspace_id = a.workspace_id AND am.user_id = a.user_id AND am.status = 'active'").
		Where("ref.workspace_id = ? AND ref.situation_id = ?", ws, situationID).
		Order("a.created_at ASC, a.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("read situation actions: %w", err)
	}
	actions := make([]model.CRMSuggestion, 0, len(rows))
	for _, row := range rows {
		row.Suggestion.Revision = model.CRMSuggestionRevision(row.Suggestion)
		row.Suggestion.AssigneeMemberID = row.MemberID
		row.Suggestion.AssigneeAvailable = row.MemberID != nil
		// Historical internal errors may contain dependency/SQL details. The
		// canonical status remains visible without disclosing those messages.
		if row.Suggestion.ExecutionError != nil {
			message := "Execution needs review. Check the action's targets and configuration."
			row.Suggestion.ExecutionError = &message
		}
		actions = append(actions, row.Suggestion)
	}
	return actions, nil
}
