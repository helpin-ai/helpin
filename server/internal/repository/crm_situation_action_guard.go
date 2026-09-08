package repository

import "context"

// SituationActionBlocked prevents legacy approval paths from executing an action
// attached to paused/closed customer work. Unlinked Review items remain usable.
func (r *CRMSituationRepository) SituationActionBlocked(ctx context.Context, ws, actionID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("crm_situation_references ref").
		Joins("JOIN crm_situations s ON s.workspace_id = ref.workspace_id AND s.id = ref.situation_id").
		Where("ref.workspace_id = ? AND ref.kind = 'suggestion' AND ref.source_id = ? AND s.lifecycle <> 'open'", ws, actionID).
		Count(&count).Error
	return count > 0, err
}
