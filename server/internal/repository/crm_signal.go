package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const minimumAutomatedSignalConfidence = 0.6

// CRMSignalRepository handles DB operations for CRM signals and health scores.
type CRMSignalRepository struct {
	db *gorm.DB
}

// NewCRMSignalRepository creates a new CRMSignalRepository.
func NewCRMSignalRepository(db *gorm.DB) *CRMSignalRepository {
	return &CRMSignalRepository{db: db}
}

// ── Buyer Signals ──

// CreateSignal inserts a buyer signal.
func (r *CRMSignalRepository) CreateSignal(ctx context.Context, signal *model.CRMBuyerSignal) error {
	ensureSignalEvidenceFingerprint(signal)
	if err := r.db.WithContext(ctx).Create(signal).Error; err != nil {
		return fmt.Errorf("create buyer signal: %w", err)
	}
	return nil
}

// CreateSignalIfAbsent inserts a buyer signal once for a given source and signal type.
func (r *CRMSignalRepository) CreateSignalIfAbsent(ctx context.Context, signal *model.CRMBuyerSignal) (bool, error) {
	if signal == nil {
		return false, nil
	}
	ensureSignalEvidenceFingerprint(signal)
	var existing model.CRMBuyerSignal
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", signal.WorkspaceID).
		Where("source_type = ?", signal.SourceType).
		Where("source_id = ?", signal.SourceID).
		Where("signal_type = ?", signal.SignalType).
		First(&existing).Error
	if err == nil {
		ensureSignalEvidenceFingerprint(&existing)
		if existing.EvidenceFingerprint == signal.EvidenceFingerprint {
			return false, nil
		}
		updates := map[string]interface{}{
			"contact_id": signal.ContactID, "deal_id": signal.DealID, "company_id": signal.CompanyID,
			"source_thread_id": signal.SourceThreadID, "summary": signal.Summary,
			"evidence_excerpt": signal.EvidenceExcerpt, "metadata": signal.Metadata,
			"confidence": signal.Confidence, "detected_at": signal.DetectedAt,
			"evidence_fingerprint": signal.EvidenceFingerprint,
			"dismissed_at":         nil, "dismissed_by_member_id": nil,
		}
		if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
			return false, fmt.Errorf("refresh buyer signal evidence: %w", err)
		}
		return true, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("lookup buyer signal by source: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(signal).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("create buyer signal if absent: %w", err)
	}
	return true, nil
}

// ListSignals returns buyer signals with optional filters.
func (r *CRMSignalRepository) ListSignals(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination) ([]model.CRMBuyerSignal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).Where("workspace_id = ?", workspaceID)
	if !filters.IncludeDismissed {
		query = query.Where("dismissed_at IS NULL")
	}
	if !filters.IncludeLowConfidence {
		query = query.Where("source_type = ? OR confidence >= ?", model.CRMSignalSourceManual, minimumAutomatedSignalConfidence)
	}

	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.CompanyID != nil && *filters.CompanyID != "" {
		query = query.Where("company_id = ?", *filters.CompanyID)
	}
	if filters.SignalType != nil && *filters.SignalType != "" {
		query = query.Where("signal_type = ?", *filters.SignalType)
	}
	if filters.SourceType != nil && *filters.SourceType != "" {
		query = query.Where("source_type = ?", *filters.SourceType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count buyer signals: %w", err)
	}

	var signals []model.CRMBuyerSignal
	offset := (pagination.Page - 1) * pagination.PerPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
	if err := query.Order("detected_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&signals).Error; err != nil {
		return nil, 0, fmt.Errorf("list buyer signals: %w", err)
	}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, 0, err
	}
	return signals, total, nil
}

// ListSignalsByCompany returns canonical signals related to a company directly,
// through its contacts, or through deals associated with those contacts.
func (r *CRMSignalRepository) ListSignalsByCompany(
	ctx context.Context,
	workspaceID, companyID string,
	pagination model.PMPagination,
) ([]model.CRMBuyerSignal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
		Where("crm_buyer_signals.workspace_id = ?", workspaceID).
		Where("crm_buyer_signals.dismissed_at IS NULL").
		Where("crm_buyer_signals.source_type = ? OR crm_buyer_signals.confidence >= ?", model.CRMSignalSourceManual, minimumAutomatedSignalConfidence).
		Where(`(
			crm_buyer_signals.company_id = ?
			OR crm_buyer_signals.contact_id IN (
				SELECT CASE WHEN ca.from_object_type = 'contact' THEN ca.from_object_id ELSE ca.to_object_id END
				FROM crm_associations ca
				WHERE ca.workspace_id = ? AND (
					(ca.from_object_type = 'contact' AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
					OR (ca.to_object_type = 'contact' AND ca.from_object_type = 'company' AND ca.from_object_id = ?)
				)
			)
			OR crm_buyer_signals.deal_id IN (
				SELECT d.id FROM crm_deals d WHERE d.workspace_id = ? AND EXISTS (
					SELECT 1 FROM crm_associations da WHERE da.workspace_id = d.workspace_id AND (
						(da.from_object_type = 'deal' AND da.from_object_id = d.id AND da.to_object_type = 'company' AND da.to_object_id = ?)
						OR (da.to_object_type = 'deal' AND da.to_object_id = d.id AND da.from_object_type = 'company' AND da.from_object_id = ?)
						OR (da.from_object_type = 'deal' AND da.from_object_id = d.id AND da.to_object_type = 'contact' AND da.to_object_id IN (
							SELECT CASE WHEN ca.from_object_type = 'contact' THEN ca.from_object_id ELSE ca.to_object_id END FROM crm_associations ca
							WHERE ca.workspace_id = d.workspace_id AND ((ca.from_object_type = 'contact' AND ca.to_object_type = 'company' AND ca.to_object_id = ?) OR (ca.to_object_type = 'contact' AND ca.from_object_type = 'company' AND ca.from_object_id = ?))
						))
						OR (da.to_object_type = 'deal' AND da.to_object_id = d.id AND da.from_object_type = 'contact' AND da.from_object_id IN (
							SELECT CASE WHEN ca.from_object_type = 'contact' THEN ca.from_object_id ELSE ca.to_object_id END FROM crm_associations ca
							WHERE ca.workspace_id = d.workspace_id AND ((ca.from_object_type = 'contact' AND ca.to_object_type = 'company' AND ca.to_object_id = ?) OR (ca.to_object_type = 'contact' AND ca.from_object_type = 'company' AND ca.from_object_id = ?))
						))
					)
				)
			)
		)`, companyID, workspaceID, companyID, companyID, workspaceID,
			companyID, companyID, companyID, companyID, companyID, companyID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count company buyer signals: %w", err)
	}
	page, perPage := pagination.Page, pagination.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	var signals []model.CRMBuyerSignal
	if err := query.Order("detected_at DESC, id DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&signals).Error; err != nil {
		return nil, 0, fmt.Errorf("list company buyer signals: %w", err)
	}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, 0, err
	}
	return signals, total, nil
}

func (r *CRMSignalRepository) hydrateSignalContext(ctx context.Context, workspaceID string, signals []model.CRMBuyerSignal) error {
	contactIDs := make([]string, 0, len(signals))
	dealIDs := make([]string, 0, len(signals))
	for i := range signals {
		if signals[i].ContactID != nil {
			contactIDs = append(contactIDs, *signals[i].ContactID)
		}
		if signals[i].DealID != nil {
			dealIDs = append(dealIDs, *signals[i].DealID)
		}
	}
	type contactRow struct{ ID, FirstName, LastName string }
	var contacts []contactRow
	if len(contactIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("crm_contacts").Select("id, first_name, last_name").Where("workspace_id = ? AND id IN ?", workspaceID, contactIDs).Scan(&contacts).Error; err != nil {
			return fmt.Errorf("hydrate signal contacts: %w", err)
		}
	}
	type dealRow struct{ ID, Name, DisplayID string }
	var deals []dealRow
	if len(dealIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("crm_deals").Select("id, name, display_id").Where("workspace_id = ? AND id IN ?", workspaceID, dealIDs).Scan(&deals).Error; err != nil {
			return fmt.Errorf("hydrate signal deals: %w", err)
		}
	}
	contactNames := make(map[string]string, len(contacts))
	for _, contact := range contacts {
		contactNames[contact.ID] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
	}
	dealRows := make(map[string]dealRow, len(deals))
	for _, deal := range deals {
		dealRows[deal.ID] = deal
	}
	for i := range signals {
		if signals[i].ContactID != nil {
			signals[i].ContactName = contactNames[*signals[i].ContactID]
		}
		if signals[i].DealID != nil {
			deal := dealRows[*signals[i].DealID]
			signals[i].DealName, signals[i].DealDisplayID = deal.Name, deal.DisplayID
		}
	}
	return nil
}

// DismissSignal hides a signal for the workspace until its evidence changes.
func (r *CRMSignalRepository) DismissSignal(ctx context.Context, workspaceID, id, memberID string, dismissedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]interface{}{"dismissed_at": dismissedAt, "dismissed_by_member_id": memberID})
	if result.Error != nil {
		return fmt.Errorf("dismiss buyer signal: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("buyer signal not found")
	}
	return nil
}

// DeleteSignal removes a buyer signal.
func (r *CRMSignalRepository) DeleteSignal(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMBuyerSignal{}).Error; err != nil {
		return fmt.Errorf("delete buyer signal: %w", err)
	}
	return nil
}

// HasRecentSignalForThread returns true when a same-type signal already exists
// for the same thread inside the provided window.
func (r *CRMSignalRepository) HasRecentSignalForThread(ctx context.Context, workspaceID, threadID, signalType, excludeSourceID string, since time.Time) (bool, error) {
	if workspaceID == "" || threadID == "" || signalType == "" {
		return false, nil
	}
	var count int64
	query := r.db.WithContext(ctx).
		Model(&model.CRMBuyerSignal{}).
		Where("workspace_id = ?", workspaceID).
		Where("source_thread_id = ?", threadID).
		Where("signal_type = ?", signalType).
		Where("detected_at >= ?", since)
	if excludeSourceID != "" {
		query = query.Where("source_id IS NULL OR source_id <> ?", excludeSourceID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("count recent thread signals: %w", err)
	}
	return count > 0, nil
}

// ReconcileAutomatedSignalsForSource removes stale model-created signals after
// a source has been analyzed successfully. Manually-created signals are never
// affected by source reconciliation.
func (r *CRMSignalRepository) ReconcileAutomatedSignalsForSource(ctx context.Context, workspaceID, sourceType, sourceID string, keepTypes []string) error {
	if workspaceID == "" || sourceType == "" || sourceID == "" {
		return nil
	}
	query := r.db.WithContext(ctx).Where(
		"workspace_id = ? AND source_type = ? AND source_id = ? AND source_type <> ?",
		workspaceID, sourceType, sourceID, model.CRMSignalSourceManual,
	)
	if len(keepTypes) > 0 {
		query = query.Where("signal_type NOT IN ?", keepTypes)
	}
	if err := query.Delete(&model.CRMBuyerSignal{}).Error; err != nil {
		return fmt.Errorf("reconcile buyer signals for source: %w", err)
	}
	return nil
}

// ListSignalsForDealSince returns verified, visible signals used by the
// deterministic deal-health scorer.
func (r *CRMSignalRepository) ListSignalsForDealSince(ctx context.Context, workspaceID, dealID string, since time.Time) ([]model.CRMBuyerSignal, error) {
	var signals []model.CRMBuyerSignal
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deal_id = ? AND dismissed_at IS NULL AND detected_at >= ?", workspaceID, dealID, since).
		Where("source_type = ? OR confidence >= ?", model.CRMSignalSourceManual, minimumAutomatedSignalConfidence).
		Order("detected_at DESC, id DESC").
		Find(&signals).Error
	if err != nil {
		return nil, fmt.Errorf("list deal signals for health score: %w", err)
	}
	return signals, nil
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint")
}

func ensureSignalEvidenceFingerprint(signal *model.CRMBuyerSignal) {
	if signal == nil || strings.TrimSpace(signal.EvidenceFingerprint) != "" {
		return
	}
	parts := []string{signal.SourceType, signal.SignalType, strings.TrimSpace(signal.Summary)}
	if signal.SourceID != nil {
		parts = append(parts, *signal.SourceID)
	}
	if signal.SourceThreadID != nil {
		parts = append(parts, *signal.SourceThreadID)
	}
	if signal.EvidenceExcerpt != nil {
		parts = append(parts, strings.TrimSpace(*signal.EvidenceExcerpt))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	signal.EvidenceFingerprint = fmt.Sprintf("%x", sum)
}

// ── Deal Health Scores ──

// CreateHealthScore inserts a deal health score.
func (r *CRMSignalRepository) CreateHealthScore(ctx context.Context, score *model.CRMDealHealthScore) error {
	if err := r.db.WithContext(ctx).Create(score).Error; err != nil {
		return fmt.Errorf("create deal health score: %w", err)
	}
	return nil
}

// SaveCalculatedHealthScore keeps one mutable snapshot per deal per day. This
// preserves useful history without letting hourly reconciliation grow the table
// without bound.
func (r *CRMSignalRepository) SaveCalculatedHealthScore(ctx context.Context, score *model.CRMDealHealthScore) error {
	if score == nil {
		return nil
	}
	latest, err := r.GetLatestHealthScore(ctx, score.WorkspaceID, score.DealID)
	if err != nil {
		return err
	}
	if latest != nil {
		snapshotAt := latest.CreatedAt
		if snapshotAt.IsZero() {
			snapshotAt = latest.CalculatedAt
		}
		if snapshotAt.After(score.CalculatedAt.Add(-24 * time.Hour)) {
			score.ID, score.CreatedAt = latest.ID, latest.CreatedAt
			if err := r.db.WithContext(ctx).Model(latest).Updates(map[string]interface{}{
				"score": score.Score, "factors": score.Factors, "calculated_at": score.CalculatedAt,
			}).Error; err != nil {
				return fmt.Errorf("update calculated deal health score: %w", err)
			}
			return nil
		}
	}
	return r.CreateHealthScore(ctx, score)
}

// GetLatestHealthScore returns the most recent health score for a deal.
func (r *CRMSignalRepository) GetLatestHealthScore(ctx context.Context, workspaceID, dealID string) (*model.CRMDealHealthScore, error) {
	var score model.CRMDealHealthScore
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND deal_id = ?", workspaceID, dealID).Order("calculated_at DESC, id DESC").First(&score).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get deal health score: %w", err)
	}
	return &score, nil
}

// ListHealthScores returns health scores for a workspace.
func (r *CRMSignalRepository) ListHealthScores(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.CRMDealHealthScore, int64, error) {
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PerPage < 1 || pagination.PerPage > 100 {
		pagination.PerPage = 20
	}
	query := r.db.WithContext(ctx).Model(&model.CRMDealHealthScore{}).
		Where("crm_deal_health_scores.workspace_id = ?", workspaceID).
		Where(`NOT EXISTS (
			SELECT 1 FROM crm_deal_health_scores newer
			WHERE newer.workspace_id = crm_deal_health_scores.workspace_id
			  AND newer.deal_id = crm_deal_health_scores.deal_id
			  AND (newer.calculated_at > crm_deal_health_scores.calculated_at
			    OR (newer.calculated_at = crm_deal_health_scores.calculated_at AND newer.id > crm_deal_health_scores.id))
		)`)

	var total int64
	if err := query.Distinct("crm_deal_health_scores.deal_id").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count health scores: %w", err)
	}
	query = query.Select("crm_deal_health_scores.*")

	var scores []model.CRMDealHealthScore
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("calculated_at DESC, id DESC").Offset(offset).Limit(pagination.PerPage).Find(&scores).Error; err != nil {
		return nil, 0, fmt.Errorf("list health scores: %w", err)
	}
	return scores, total, nil
}
