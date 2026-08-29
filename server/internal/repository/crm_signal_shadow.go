package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GetSignalShadowGate calculates the objective Phase 1 rollout gate.
func (r *CRMSignalRepository) GetSignalShadowGate(
	ctx context.Context,
	workspaceID string,
) (*model.CRMSignalShadowGate, error) {
	gate := &model.CRMSignalShadowGate{}
	if !r.db.Migrator().HasTable(&model.CRMSignalObservation{}) {
		return gate, nil
	}
	if err := r.db.WithContext(ctx).Model(&model.CRMSignalObservation{}).
		Where("workspace_id = ?", workspaceID).Count(&gate.ObservationCount).Error; err != nil {
		return nil, fmt.Errorf("count signal observations: %w", err)
	}
	if gate.ObservationCount == 0 {
		return gate, nil
	}
	var mappingCoverage struct{ Total, Unmapped int64 }
	if err := r.db.WithContext(ctx).Raw(`
		WITH observation_motions AS (
			SELECT o.id, jsonb_array_elements_text(
				COALESCE(o.motions_at_detection->'values', '[]'::jsonb)
			) AS motion
			FROM crm_signal_observations o WHERE o.workspace_id = ?
		)
		SELECT COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE NOT EXISTS (
			   SELECT 1 FROM crm_signals s
			   WHERE s.observation_id = observation_motions.id
			     AND s.commercial_motion = observation_motions.motion
		   )) AS unmapped
		FROM observation_motions`, workspaceID).Scan(&mappingCoverage).Error; err != nil {
		return nil, fmt.Errorf("count unmapped observation motions: %w", err)
	}
	var duplicateRows struct {
		Total         int64
		DistinctCount int64
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS total,
		       COUNT(DISTINCT evidence_fingerprint || ':' || commercial_motion || ':' ||
		         COALESCE(CAST(contact_id AS TEXT), '') || ':' ||
		         COALESCE(CAST(deal_id AS TEXT), '') || ':' ||
		         COALESCE(CAST(company_id AS TEXT), '')) AS distinct_count
		FROM crm_signals
		WHERE workspace_id = ? AND meaning_fingerprint <> ''`, workspaceID).
		Scan(&duplicateRows).Error; err != nil {
		return nil, fmt.Errorf("calculate signal duplicate rate: %w", err)
	}
	if mappingCoverage.Total > 0 {
		gate.UnmappedObservationRate = float64(mappingCoverage.Unmapped) / float64(mappingCoverage.Total)
	}
	if duplicateRows.Total > 0 {
		gate.DuplicateFingerprintRate = float64(duplicateRows.Total-duplicateRows.DistinctCount) /
			float64(duplicateRows.Total)
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM crm_signals
		WHERE workspace_id = ? AND meaning_fingerprint <> '' AND (
			commercial_motion <> interpretation_snapshot->>'motion'
			OR signal_type <> interpretation_snapshot->>'signal_type'
			OR polarity <> interpretation_snapshot->>'polarity'
			OR interpretation_version <> (interpretation_snapshot->>'interpretation_version')::integer
		)`, workspaceID).Scan(&gate.ImmutableMeaningViolations).Error; err != nil {
		return nil, fmt.Errorf("assert immutable signal meaning: %w", err)
	}
	gate.Eligible = gate.ObservationCount >= 100 && gate.UnmappedObservationRate < 0.05 &&
		gate.DuplicateFingerprintRate < 0.01 && gate.ImmutableMeaningViolations == 0
	return gate, nil
}
