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
	"github.com/helpin-ai/helpin/server/internal/querybuilder"
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
	if !r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "commercial_motion") {
		ensureSignalDimensions(signal)
		ensureSignalEvidenceFingerprint(signal)
		if err := legacySignalCreateDB(r.db.WithContext(ctx)).Create(signal).Error; err != nil {
			return fmt.Errorf("create buyer signal: %w", err)
		}
		return nil
	}
	rows, err := r.prepareSignalInterpretations(ctx, signal)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("signal has no applicable commercial interpretation")
	}
	created, primary, err := insertInterpretedSignals(r.db.WithContext(ctx), rows)
	if err != nil {
		return fmt.Errorf("create buyer signal: %w", err)
	}
	if !created || primary == nil {
		return fmt.Errorf("buyer signal already exists")
	}
	*signal = *primary
	return nil
}

// ValidateSignalReferences prevents a manually entered signal from linking to
// CRM records outside its workspace.
func (r *CRMSignalRepository) ValidateSignalReferences(ctx context.Context, workspaceID string, contactID, dealID, companyID *string) error {
	checks := []struct {
		label string
		table string
		id    *string
	}{
		{label: "contact", table: "crm_contacts", id: contactID},
		{label: "deal", table: "crm_deals", id: dealID},
		{label: "company", table: "crm_companies", id: companyID},
	}
	for _, check := range checks {
		if check.id == nil || strings.TrimSpace(*check.id) == "" {
			continue
		}
		var count int64
		if err := r.db.WithContext(ctx).Table(check.table).
			Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(*check.id)).
			Count(&count).Error; err != nil {
			return fmt.Errorf("validate signal %s: %w", check.label, err)
		}
		if count == 0 {
			return fmt.Errorf("%s not found in workspace", check.label)
		}
	}
	return nil
}

// CreateSignalIfAbsent inserts a buyer signal once for a given source and signal type.
func (r *CRMSignalRepository) CreateSignalIfAbsent(ctx context.Context, signal *model.CRMBuyerSignal) (bool, error) {
	if signal == nil {
		return false, nil
	}
	if !r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "commercial_motion") {
		return r.createLegacySignalIfAbsent(ctx, signal)
	}
	rows, err := r.prepareSignalInterpretations(ctx, signal)
	if err != nil {
		return false, err
	}
	created, primary, err := insertInterpretedSignals(r.db.WithContext(ctx), rows)
	if err != nil {
		return false, fmt.Errorf("create interpreted signal if absent: %w", err)
	}
	if primary != nil {
		*signal = *primary
	}
	return created, nil
}

func (r *CRMSignalRepository) createLegacySignalIfAbsent(ctx context.Context, signal *model.CRMBuyerSignal) (bool, error) {
	ensureSignalDimensions(signal)
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
			"detector_kind": signal.DetectorKind, "signal_domain": signal.SignalDomain,
			"polarity": signal.Polarity, "rule_key": signal.RuleKey,
			"rule_version": signal.RuleVersion, "window_started_at": signal.WindowStartedAt,
			"window_ended_at":          signal.WindowEndedAt,
			"evidence_identity_method": signal.EvidenceIdentityMethod,
			"evidence_identity_trust":  signal.EvidenceIdentityTrust,
			"evidence_fingerprint":     signal.EvidenceFingerprint,
			"dismissed_at":             nil, "dismissed_by_member_id": nil,
		}
		if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
			return false, fmt.Errorf("refresh buyer signal evidence: %w", err)
		}
		return true, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("lookup buyer signal by source: %w", err)
	}

	if err := legacySignalCreateDB(r.db.WithContext(ctx)).Create(signal).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("create buyer signal if absent: %w", err)
	}
	return true, nil
}

func legacySignalCreateDB(db *gorm.DB) *gorm.DB {
	return db.Omit(
		"observation_id", "commercial_motion", "interpretation_version",
		"business_weight_snapshot", "half_life_days_snapshot", "interpretation_snapshot",
		"meaning_fingerprint", "recommended_action_key", "recommended_action_label",
		"replay_calibration_excluded", "superseded_at", "superseded_reason",
		"direction_changed_by_supersession",
	)
}

// ListSignals returns buyer signals with optional filters.
func (r *CRMSignalRepository) ListSignals(ctx context.Context, workspaceID string, filters model.CRMBuyerSignalListFilters, pagination model.PMPagination) ([]model.CRMBuyerSignal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).Where("crm_buyer_signals.workspace_id = ?", workspaceID)
	hasSupersession := r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "superseded_at")
	if filters.Query != nil {
		var err error
		query, err = querybuilder.ApplyGORM(query, filters.Query, crmSignalFilterDefinitions)
		if err != nil {
			return nil, 0, err
		}
	}
	if filters.Status != nil && *filters.Status == "dismissed" {
		query = query.Where("crm_buyer_signals.dismissed_at IS NOT NULL")
	} else if hasSupersession && filters.Status != nil && *filters.Status == "superseded" {
		query = query.Where("crm_buyer_signals.superseded_at IS NOT NULL")
	} else if filters.Status != nil && *filters.Status == "all" {
		// Include both active and dismissed signals.
	} else if !filters.IncludeDismissed {
		query = query.Where("crm_buyer_signals.dismissed_at IS NULL")
		if hasSupersession {
			query = query.Where("crm_buyer_signals.superseded_at IS NULL")
		}
	}
	if !filters.IncludeLowConfidence {
		query = query.Where(
			"detector_kind = ? OR source_type = ? OR confidence >= ?",
			model.CRMSignalDetectorRuleDerived,
			model.CRMSignalSourceManual,
			minimumAutomatedSignalConfidence,
		)
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
	if filters.SignalDomain != nil && *filters.SignalDomain != "" {
		query = query.Where("signal_domain = ?", *filters.SignalDomain)
	}
	if filters.Polarity != nil && *filters.Polarity != "" {
		query = query.Where("polarity = ?", *filters.Polarity)
	}
	if r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "commercial_motion") && filters.CommercialMotion != nil && *filters.CommercialMotion != "" {
		query = query.Where("commercial_motion = ?", *filters.CommercialMotion)
	}
	if filters.EvidenceIdentityTrust != nil && *filters.EvidenceIdentityTrust != "" {
		query = query.Where("evidence_identity_trust = ?", *filters.EvidenceIdentityTrust)
	}
	if filters.MaxAgeDays != nil && *filters.MaxAgeDays > 0 {
		query = query.Where("detected_at >= ?", time.Now().UTC().Add(-time.Duration(*filters.MaxAgeDays)*24*time.Hour))
	}
	ownerFiltered := filters.OwnerMemberID != nil && strings.TrimSpace(*filters.OwnerMemberID) != ""
	var signals []model.CRMBuyerSignal
	offset := (pagination.Page - 1) * pagination.PerPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
	listQuery := query.Order("detected_at DESC")
	if !ownerFiltered {
		listQuery = listQuery.Offset(offset).Limit(pagination.PerPage)
	}
	if err := listQuery.Find(&signals).Error; err != nil {
		return nil, 0, fmt.Errorf("list buyer signals: %w", err)
	}
	if err := r.hydrateSignalContext(ctx, workspaceID, signals); err != nil {
		return nil, 0, err
	}
	if ownerFiltered {
		signals = filterSignalsByResolvedOwner(signals, strings.TrimSpace(*filters.OwnerMemberID))
		total := int64(len(signals))
		start := min(offset, len(signals))
		end := min(start+pagination.PerPage, len(signals))
		return signals[start:end], total, nil
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count buyer signals: %w", err)
	}
	return signals, total, nil
}

func filterSignalsByResolvedOwner(signals []model.CRMBuyerSignal, ownerMemberID string) []model.CRMBuyerSignal {
	filtered := signals[:0]
	for _, signal := range signals {
		if signal.OwnerMemberID != nil && *signal.OwnerMemberID == ownerMemberID {
			filtered = append(filtered, signal)
		}
	}
	return filtered
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
	if r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "superseded_at") {
		query = query.Where("crm_buyer_signals.superseded_at IS NULL")
	}

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
	type dealRow struct {
		ID, Name, DisplayID string
	}
	var deals []dealRow
	if len(dealIDs) > 0 {
		if err := r.db.WithContext(ctx).Table("crm_deals d").
			Select("d.id, d.name, d.display_id").
			Where("d.workspace_id = ? AND d.id IN ?", workspaceID, dealIDs).Scan(&deals).Error; err != nil {
			return fmt.Errorf("hydrate signal deals: %w", err)
		}
	}
	type dealRankingRow struct {
		ID            string
		Amount        *float64
		Probability   *int
		OwnerMemberID *string
	}
	var dealRankings []dealRankingRow
	if len(dealIDs) > 0 && r.db.Migrator().HasColumn("crm_deals", "probability") {
		if err := r.db.WithContext(ctx).Table("crm_deals").
			Select("id, amount, probability, owner_member_id").
			Where("workspace_id = ? AND id IN ?", workspaceID, dealIDs).Scan(&dealRankings).Error; err != nil {
			return fmt.Errorf("hydrate signal deal ranking context: %w", err)
		}
	}
	companyIDs := make([]string, 0, len(signals))
	for i := range signals {
		if signals[i].CompanyID != nil {
			companyIDs = append(companyIDs, *signals[i].CompanyID)
		}
	}
	type companyRow struct {
		ID, Name                     string
		Domain                       *string
		OwnerMemberID                *string
		CustomerSuccessOwnerMemberID *string
	}
	var companies []companyRow
	if len(companyIDs) > 0 && r.db.Migrator().HasTable("crm_companies") {
		companySelect := "id, name, domain, owner_member_id, NULL AS customer_success_owner_member_id"
		if r.db.Migrator().HasColumn(&model.CRMCompany{}, "customer_success_owner_member_id") {
			companySelect = "id, name, domain, owner_member_id, customer_success_owner_member_id"
		}
		if err := r.db.WithContext(ctx).Table("crm_companies").
			Select(companySelect).
			Where("workspace_id = ? AND id IN ?", workspaceID, companyIDs).Scan(&companies).Error; err != nil {
			return fmt.Errorf("hydrate signal companies: %w", err)
		}
	}
	activeMemberIDs := map[string]bool{}
	hasWorkspaceMembers := r.db.Migrator().HasTable(&model.WorkspaceMember{})
	if hasWorkspaceMembers {
		var members []model.WorkspaceMember
		if err := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).
			Select("id, user_id, role").
			Where("workspace_id = ? AND status = ?", workspaceID, model.WorkspaceMemberStatusActive).
			Find(&members).Error; err != nil {
			return fmt.Errorf("hydrate active signal owners: %w", err)
		}
		for _, member := range members {
			activeMemberIDs[member.ID] = true
		}
	}
	var defaultOwnerMemberID *string
	if r.db.Migrator().HasTable(&model.CRMSignalRoutingSettings{}) {
		var settings model.CRMSignalRoutingSettings
		err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&settings).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("hydrate signal routing settings: %w", err)
		}
		defaultOwnerMemberID = settings.DefaultSignalOwnerMemberID
	}
	var workspaceOwnerMemberID *string
	if hasWorkspaceMembers {
		var owner model.WorkspaceMember
		ownerQuery := r.db.WithContext(ctx).Model(&model.WorkspaceMember{}).
			Where("workspace_id = ? AND status = ?", workspaceID, model.WorkspaceMemberStatusActive)
		if r.db.Migrator().HasTable(&model.Workspace{}) {
			var workspace model.Workspace
			if err := r.db.WithContext(ctx).Select("owner_id").Where("id = ?", workspaceID).First(&workspace).Error; err == nil {
				ownerQuery = ownerQuery.Where("user_id = ?", workspace.OwnerID)
			} else {
				ownerQuery = ownerQuery.Where("role = ?", model.RoleOwner)
			}
		} else {
			ownerQuery = ownerQuery.Where("role = ?", model.RoleOwner)
		}
		if err := ownerQuery.Order("created_at ASC").First(&owner).Error; err == nil {
			workspaceOwnerMemberID = &owner.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("hydrate workspace signal owner: %w", err)
		}
	}
	eligibleOwner := func(memberID *string) *string {
		if memberID == nil || strings.TrimSpace(*memberID) == "" {
			return nil
		}
		if hasWorkspaceMembers && !activeMemberIDs[*memberID] {
			return nil
		}
		return memberID
	}
	contactNames := make(map[string]string, len(contacts))
	for _, contact := range contacts {
		contactNames[contact.ID] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
	}
	dealRows := make(map[string]dealRow, len(deals))
	for _, deal := range deals {
		dealRows[deal.ID] = deal
	}
	dealRankingRows := make(map[string]dealRankingRow, len(dealRankings))
	for _, deal := range dealRankings {
		dealRankingRows[deal.ID] = deal
	}
	companyRows := make(map[string]companyRow, len(companies))
	for _, company := range companies {
		companyRows[company.ID] = company
	}
	for i := range signals {
		if signals[i].ContactID != nil {
			signals[i].ContactName = contactNames[*signals[i].ContactID]
		}
		if signals[i].DealID != nil {
			deal := dealRows[*signals[i].DealID]
			ranking := dealRankingRows[*signals[i].DealID]
			signals[i].DealName, signals[i].DealDisplayID = deal.Name, deal.DisplayID
			signals[i].DealAmount = ranking.Amount
			signals[i].OwnerMemberID = eligibleOwner(ranking.OwnerMemberID)
			signals[i].DealStageProbability = ranking.Probability
		}
		if signals[i].CompanyID != nil {
			company := companyRows[*signals[i].CompanyID]
			signals[i].AccountName = company.Name
			if company.Domain != nil {
				signals[i].AccountDomain = *company.Domain
			}
			customerMotion := signals[i].CommercialMotion == model.CRMCommercialMotionOnboarding ||
				signals[i].CommercialMotion == model.CRMCommercialMotionAdoption ||
				signals[i].CommercialMotion == model.CRMCommercialMotionRetention
			if customerMotion {
				signals[i].OwnerMemberID = eligibleOwner(company.CustomerSuccessOwnerMemberID)
			}
			if signals[i].OwnerMemberID == nil {
				signals[i].OwnerMemberID = eligibleOwner(company.OwnerMemberID)
			}
		}
		if signals[i].OwnerMemberID == nil {
			signals[i].OwnerMemberID = eligibleOwner(defaultOwnerMemberID)
		}
		if signals[i].OwnerMemberID == nil {
			signals[i].OwnerMemberID = workspaceOwnerMemberID
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
func (r *CRMSignalRepository) DeleteSignal(ctx context.Context, workspaceID, id string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.CRMBuyerSignal{})
	if result.Error != nil {
		return fmt.Errorf("delete buyer signal: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("buyer signal not found")
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

// ReconcileAutomatedSignalsForSource supersedes stale model-created signals after
// a source has been analyzed successfully. Manually-created signals are never
// affected by source reconciliation.
func (r *CRMSignalRepository) ReconcileAutomatedSignalsForSource(ctx context.Context, workspaceID, sourceType, sourceID string, keepTypes []string) error {
	if workspaceID == "" || sourceType == "" || sourceID == "" {
		return nil
	}
	baseQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).Where(
			"workspace_id = ? AND source_type = ? AND source_id = ? AND source_type <> ?",
			workspaceID, sourceType, sourceID, model.CRMSignalSourceManual,
		)
		if len(keepTypes) > 0 {
			query = query.Where("signal_type NOT IN ?", keepTypes)
		}
		return query
	}
	if r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "superseded_at") {
		now := time.Now().UTC()
		var stale []model.CRMBuyerSignal
		if err := baseQuery().Find(&stale).Error; err != nil {
			return fmt.Errorf("load buyer signals to supersede: %w", err)
		}
		directionFlipSignalIDs := make([]string, 0)
		seenGroups := map[string]bool{}
		for _, signal := range stale {
			groupKey := signalEntityMotionKey(signal)
			if seenGroups[groupKey] {
				continue
			}
			seenGroups[groupKey] = true
			active := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
				Where("workspace_id = ? AND commercial_motion = ? AND dismissed_at IS NULL AND superseded_at IS NULL",
					signal.WorkspaceID, signal.CommercialMotion)
			active = nullableSignalID(active, "contact_id", signal.ContactID)
			active = nullableSignalID(active, "deal_id", signal.DealID)
			active = nullableSignalID(active, "company_id", signal.CompanyID)
			var groupSignals []model.CRMBuyerSignal
			if err := active.Find(&groupSignals).Error; err != nil {
				return fmt.Errorf("load signal direction before supersession: %w", err)
			}
			staleIDs := map[string]bool{}
			for _, candidate := range stale {
				if signalEntityMotionKey(candidate) == groupKey {
					staleIDs[candidate.ID] = true
				}
			}
			beforeDirection := storedSignalGroupDirection(groupSignals, nil)
			afterDirection := storedSignalGroupDirection(groupSignals, staleIDs)
			if beforeDirection != 0 && afterDirection != 0 && beforeDirection != afterDirection {
				var target *model.CRMBuyerSignal
				for index := range groupSignals {
					candidate := &groupSignals[index]
					if staleIDs[candidate.ID] {
						continue
					}
					if target == nil || storedSignalMagnitude(*candidate) > storedSignalMagnitude(*target) {
						target = candidate
					}
				}
				if target != nil {
					directionFlipSignalIDs = append(directionFlipSignalIDs, target.ID)
				}
			}
		}
		if err := baseQuery().Model(&model.CRMBuyerSignal{}).Updates(map[string]interface{}{
			"superseded_at": now, "superseded_reason": "source_reinterpreted",
		}).Error; err != nil {
			return fmt.Errorf("supersede buyer signals for source: %w", err)
		}
		if len(directionFlipSignalIDs) > 0 {
			if err := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
				Where("workspace_id = ? AND id IN ?", workspaceID, directionFlipSignalIDs).
				Update("direction_changed_by_supersession", true).Error; err != nil {
				return fmt.Errorf("mark supersession direction change: %w", err)
			}
		}
		return nil
	}
	if err := baseQuery().Delete(&model.CRMBuyerSignal{}).Error; err != nil {
		return fmt.Errorf("reconcile legacy buyer signals for source: %w", err)
	}
	return nil
}

func storedSignalMagnitude(signal model.CRMBuyerSignal) float64 {
	strength := signal.BusinessWeightSnapshot
	if strength <= 0 {
		strength = 1
	}
	if signal.Confidence > 0 {
		strength *= signal.Confidence
	}
	return strength
}

func signalEntityMotionKey(signal model.CRMBuyerSignal) string {
	parts := []string{signal.WorkspaceID, signal.CommercialMotion}
	for _, id := range []*string{signal.ContactID, signal.DealID, signal.CompanyID} {
		if id == nil {
			parts = append(parts, "")
		} else {
			parts = append(parts, *id)
		}
	}
	return strings.Join(parts, "\x00")
}

func storedSignalGroupDirection(signals []model.CRMBuyerSignal, excluded map[string]bool) int {
	impact := 0.0
	for _, signal := range signals {
		if excluded[signal.ID] {
			continue
		}
		strength := storedSignalMagnitude(signal)
		switch signal.Polarity {
		case model.CRMSignalPolarityNegative:
			impact -= strength
		case model.CRMSignalPolarityPositive:
			impact += strength
		}
	}
	if impact > 0 {
		return 1
	}
	if impact < 0 {
		return -1
	}
	return 0
}

// ListSignalsForDealSince returns verified, visible signals used by the
// deterministic deal-health scorer.
func (r *CRMSignalRepository) ListSignalsForDealSince(ctx context.Context, workspaceID, dealID string, since time.Time) ([]model.CRMBuyerSignal, error) {
	var signals []model.CRMBuyerSignal
	query := r.db.WithContext(ctx).
		Where("workspace_id = ? AND deal_id = ? AND dismissed_at IS NULL AND detected_at >= ?", workspaceID, dealID, since)
	if r.db.Migrator().HasColumn(&model.CRMBuyerSignal{}, "superseded_at") {
		query = query.Where("superseded_at IS NULL")
	}
	err := query.
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

func ensureSignalDimensions(signal *model.CRMBuyerSignal) {
	if signal == nil {
		return
	}
	if signal.DetectorKind == "" {
		signal.DetectorKind = model.CRMSignalDetectorLLMExtracted
	}
	if signal.SignalDomain == "" {
		signal.SignalDomain = model.CRMSignalDomainConversation
	}
	// Detectors may omit observation polarity. This fallback normalizes only the
	// motion-agnostic observation; a commercial signal is still impossible
	// without an explicit interpretation mapping, which may override it.
	if signal.Polarity == "" {
		switch signal.SignalType {
		case model.CRMSignalBuyingIntent, model.CRMSignalBudgetSignal,
			model.CRMSignalTimelineSignal, model.CRMSignalChampionSignal:
			signal.Polarity = model.CRMSignalPolarityPositive
		case model.CRMSignalObjection, model.CRMSignalCompetitorMention, model.CRMSignalRiskSignal:
			signal.Polarity = model.CRMSignalPolarityNegative
		default:
			signal.Polarity = model.CRMSignalPolarityNeutral
		}
	}
	if signal.EvidenceIdentityMethod == "" {
		if signal.SourceType == model.CRMSignalSourceManual {
			signal.EvidenceIdentityMethod = model.IdentityMethodManualEntry
		} else if signal.SourceType == model.CRMSignalSourceSupport {
			signal.EvidenceIdentityMethod = model.IdentityMethodVerifiedSupport
		} else {
			signal.EvidenceIdentityMethod = model.IdentityMethodConnectedMailbox
		}
	}
	if signal.EvidenceIdentityTrust == "" {
		if signal.SourceType == model.CRMSignalSourceManual {
			signal.EvidenceIdentityTrust = model.IdentityTrustUntrusted
		} else {
			signal.EvidenceIdentityTrust = model.IdentityTrustVerified
		}
	}
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
