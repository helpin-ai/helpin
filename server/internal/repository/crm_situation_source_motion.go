package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SourceMotion preserves explicit deal motion or unambiguous supporting evidence
// before the adapter falls back to the kind of recommendation.
func (r *CRMSituationRepository) SourceMotion(ctx context.Context, suggestion model.CRMSuggestion) (string, error) {
	fallback := ""
	if suggestion.ObjectType != nil && *suggestion.ObjectType == "deal" && suggestion.ObjectID != nil {
		var row struct{ CommercialMotion string }
		result := r.db.WithContext(ctx).Table("crm_deals d").
			Select("COALESCE(NULLIF(d.commercial_motion,''), p.default_commercial_motion, 'new_business') AS commercial_motion").
			Joins("LEFT JOIN crm_pipelines p ON p.id = d.pipeline_id AND p.workspace_id = d.workspace_id").
			Where("d.workspace_id = ? AND d.id = ?", suggestion.WorkspaceID, *suggestion.ObjectID).Scan(&row)
		if result.Error != nil {
			return "", result.Error
		}
		if result.RowsAffected == 1 {
			motion := commercialMotionForDealMotion(row.CommercialMotion)
			if motion != model.CRMCommercialMotionNeedsContext {
				return motion, nil
			}
			fallback = model.CRMCommercialMotionNeedsContext
		}
	}
	if len(suggestion.SignalIDs) != 0 {
		var motions []string
		if err := r.db.WithContext(ctx).Model(&model.CRMSignal{}).Where("workspace_id = ? AND id IN ? AND superseded_at IS NULL AND dismissed_at IS NULL", suggestion.WorkspaceID, []string(suggestion.SignalIDs)).
			Distinct("commercial_motion").Pluck("commercial_motion", &motions).Error; err != nil {
			return "", err
		}
		if len(motions) == 1 && model.CRMSituationCategoryForMotion(motions[0]) != "" {
			return motions[0], nil
		}
	}
	return fallback, nil
}
