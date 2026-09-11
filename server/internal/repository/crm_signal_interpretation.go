package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const crmSignalInterpretationVersion = 1

// prepareSignalInterpretations resolves detection-time motions and snapshots commercial meaning.
// Test databases that still model the legacy schema keep the legacy single-row behavior.
func (r *CRMSignalRepository) prepareSignalInterpretations(
	ctx context.Context,
	signal *model.CRMSignal,
) ([]model.CRMSignal, error) {
	ensureSignalDimensions(signal)
	ensureSignalEvidenceFingerprint(signal)
	if !r.db.Migrator().HasColumn(&model.CRMSignal{}, "commercial_motion") {
		return []model.CRMSignal{*signal}, nil
	}

	motions, snapshot, err := r.resolveSignalMotions(ctx, signal)
	if err != nil {
		return nil, err
	}
	if err := r.persistSignalMotionState(ctx, signal, motions, snapshot); err != nil {
		return nil, err
	}
	if isCommerciallyQualifiedConversation(*signal) {
		snapshot["baseline_motions_at_detection"] = motions
		motions = []string{signal.CommercialMotion}
		snapshot["motions_at_detection"] = motions
		snapshot["commercial_event"] = signal.Metadata["commercial_event"]
		snapshot["customer_relationship"] = signal.Metadata["customer_relationship"]
		snapshot["needs_customer_context"] = signal.Metadata["needs_customer_context"]
	}
	observationID, err := r.persistSignalObservation(ctx, signal, motions, snapshot)
	if err != nil {
		return nil, err
	}

	rows := make([]model.CRMSignal, 0, len(motions))
	for _, motion := range motions {
		config, err := r.findSignalInterpretation(ctx, signal, motion)
		if err != nil {
			return nil, err
		}
		interpreted, interpretationVersion := *signal, crmSignalInterpretationVersion
		if config == nil {
			// No mapping means no commercial meaning. The observation remains
			// available to the shadow-coverage gate without entering a lane.
			continue
		}
		if err := r.snapshotSignalScoringMeaning(ctx, &interpreted); err != nil {
			return nil, err
		}
		if config != nil {
			if config.SignalType != "inherit" {
				interpreted.SignalType = config.SignalType
			}
			if config.Polarity != "inherit" {
				interpreted.Polarity = config.Polarity
			}
			interpreted.RecommendedActionKey = config.RecommendedActionKey
			interpreted.RecommendedActionLabel = config.RecommendedActionLabel
			interpreted.BusinessWeightSnapshot = config.BusinessWeight
			interpreted.HalfLifeDaysSnapshot = config.HalfLifeDays
			interpretationVersion = config.Version
		}
		interpreted.ID = ""
		interpreted.ObservationID = observationID
		interpreted.CommercialMotion = motion
		interpreted.InterpretationVersion = interpretationVersion
		interpreted.InterpretationSnapshot = cloneJSONB(snapshot)
		interpreted.InterpretationSnapshot["motion"] = motion
		interpreted.InterpretationSnapshot["signal_type"] = interpreted.SignalType
		interpreted.InterpretationSnapshot["polarity"] = interpreted.Polarity
		interpreted.InterpretationSnapshot["interpretation_version"] = interpretationVersion
		interpreted.InterpretationSnapshot["business_weight"] = interpreted.BusinessWeightSnapshot
		interpreted.InterpretationSnapshot["half_life_days"] = interpreted.HalfLifeDaysSnapshot
		if interpreted.RecommendedActionKey != nil {
			interpreted.InterpretationSnapshot["recommended_action_key"] = *interpreted.RecommendedActionKey
		}
		if isCommerciallyQualifiedConversation(interpreted) {
			actionKey, _ := signal.Metadata["commercial_action_key"].(string)
			actionLabel, _ := signal.Metadata["commercial_action_label"].(string)
			interpreted.RecommendedActionKey, interpreted.RecommendedActionLabel = &actionKey, &actionLabel
			interpreted.InterpretationSnapshot["recommended_action_key"] = actionKey
		}
		interpreted.MeaningFingerprint = signalMeaningFingerprint(interpreted)
		rows = append(rows, interpreted)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, nil
}

func (r *CRMSignalRepository) persistSignalMotionState(
	ctx context.Context,
	signal *model.CRMSignal,
	motions []string,
	snapshot model.JSONB,
) error {
	if !r.db.Migrator().HasTable(&model.CRMSignalMotionState{}) {
		return nil
	}
	entityType, entityID := "", ""
	if signal.CompanyID != nil {
		entityType, entityID = "company", strings.TrimSpace(*signal.CompanyID)
	} else if signal.DealID != nil {
		entityType, entityID = "deal", strings.TrimSpace(*signal.DealID)
	} else if signal.ContactID != nil {
		entityType, entityID = "contact", strings.TrimSpace(*signal.ContactID)
	}
	if entityID == "" {
		return nil
	}
	state := model.CRMSignalMotionState{
		ID: uuid.NewString(), WorkspaceID: signal.WorkspaceID, EntityType: entityType,
		EntityID: entityID, ResolverVersion: 1, Motions: model.JSONB{"values": motions},
		InputSnapshot: cloneJSONB(snapshot), EffectiveAt: signal.DetectedAt,
	}
	var latest model.CRMSignalMotionState
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND entity_type = ? AND entity_id = ? AND resolver_version = ?",
			state.WorkspaceID, state.EntityType, state.EntityID, state.ResolverVersion).
		Order("effective_at DESC").First(&latest).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("load latest signal motion state: %w", err)
	}
	if err == nil && equalSignalJSON(latest.Motions, state.Motions) &&
		equalSignalJSON(latest.InputSnapshot, state.InputSnapshot) {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error
}

func equalSignalJSON(left, right model.JSONB) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

// PruneSignalMotionStates bounds the append-only resolver audit. Signals keep
// their immutable detection-time snapshot, so older resolver audit rows are
// not required to preserve signal meaning.
func (r *CRMSignalRepository) PruneSignalMotionStates(ctx context.Context, before time.Time) (int64, error) {
	if !r.db.Migrator().HasTable(&model.CRMSignalMotionState{}) {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Where("effective_at < ?", before).
		Delete(&model.CRMSignalMotionState{})
	if result.Error != nil {
		return 0, fmt.Errorf("prune signal motion states: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *CRMSignalRepository) findSignalInterpretation(
	ctx context.Context,
	signal *model.CRMSignal,
	motion string,
) (*model.CRMSignalInterpretationConfig, error) {
	if !r.db.Migrator().HasTable(&model.CRMSignalInterpretationConfig{}) {
		return nil, nil
	}
	ruleKey := signalInterpretationRuleKey(signal)
	ruleVersion := 1
	if signal.RuleVersion != nil {
		ruleVersion = *signal.RuleVersion
	}
	var config model.CRMSignalInterpretationConfig
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND rule_key = ? AND rule_version = ? AND motion = ?", true, ruleKey, ruleVersion, motion).
		Where("observation_signal_type IN (?, '*')", signal.SignalType).
		Where("workspace_id = ? OR workspace_id IS NULL", signal.WorkspaceID).
		Order("CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END, CASE WHEN observation_signal_type = '*' THEN 1 ELSE 0 END, version DESC").
		First(&config).Error
	if err == nil {
		return &config, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, fmt.Errorf("load signal interpretation: %w", err)
}

func (r *CRMSignalRepository) snapshotSignalScoringMeaning(ctx context.Context, signal *model.CRMSignal) error {
	if signal.BusinessWeightSnapshot <= 0 || signal.HalfLifeDaysSnapshot <= 0 {
		weight, halfLife := defaultSignalMeaning(signal.SignalType)
		if signal.BusinessWeightSnapshot <= 0 {
			signal.BusinessWeightSnapshot = weight
		}
		if signal.HalfLifeDaysSnapshot <= 0 {
			signal.HalfLifeDaysSnapshot = halfLife
		}
	}
	if signal.RuleKey == nil || !r.db.Migrator().HasTable(&model.CRMSignalRuleConfig{}) {
		return nil
	}
	query := r.db.WithContext(ctx).
		Where("rule_key = ? AND (workspace_id = ? OR workspace_id IS NULL)", *signal.RuleKey, signal.WorkspaceID)
	if signal.RuleVersion != nil {
		query = query.Where("version = ?", *signal.RuleVersion)
	}
	var rule model.CRMSignalRuleConfig
	err := query.Order("CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END, version DESC").First(&rule).Error
	if err == nil {
		if rule.BusinessWeight > 0 {
			signal.BusinessWeightSnapshot = rule.BusinessWeight
		}
		if rule.HalfLifeDays > 0 {
			signal.HalfLifeDaysSnapshot = rule.HalfLifeDays
		}
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return fmt.Errorf("snapshot signal rule scoring meaning: %w", err)
}

func defaultSignalMeaning(signalType string) (float64, float64) {
	return defaultSignalWeight(signalType), defaultSignalHalfLife(signalType)
}

func defaultSignalWeight(signalType string) float64 {
	switch signalType {
	case model.CRMSignalBuyingIntent:
		return 15
	case model.CRMSignalBudgetSignal, model.CRMSignalChampionSignal, model.CRMSignalObjection:
		return 12
	case model.CRMSignalTimelineSignal:
		return 10
	case model.CRMSignalCompetitorMention:
		return 8
	case model.CRMSignalRiskSignal:
		return 20
	default:
		return 8
	}
}

func defaultSignalHalfLife(signalType string) float64 {
	switch signalType {
	case model.CRMSignalBuyingIntent:
		return 7
	case model.CRMSignalBudgetSignal:
		return 21
	case model.CRMSignalTimelineSignal:
		return 10
	case model.CRMSignalChampionSignal:
		return 180
	case model.CRMSignalObjection:
		return 30
	case model.CRMSignalCompetitorMention:
		return 45
	case model.CRMSignalRiskSignal:
		return 30
	default:
		return 30
	}
}

// ReconcileDealMotionSignals supersedes active interpretations when a deal leaves its motion.
func (r *CRMSignalRepository) ReconcileDealMotionSignals(ctx context.Context, workspaceID, dealID string) error {
	if !r.db.Migrator().HasColumn(&model.CRMSignal{}, "superseded_at") ||
		!r.db.Migrator().HasColumn(&model.CRMDeal{}, "commercial_motion") {
		return nil
	}
	var deal struct {
		CommercialMotion        *string
		DefaultCommercialMotion string
		StageType               string
	}
	result := r.db.WithContext(ctx).Table("crm_deals d").
		Select("d.commercial_motion, p.default_commercial_motion, s.stage_type").
		Joins("JOIN crm_pipelines p ON p.id = d.pipeline_id").
		Joins("JOIN crm_pipeline_stages s ON s.id = d.stage_id").
		Where("d.workspace_id = ? AND d.id = ?", workspaceID, dealID).Scan(&deal)
	if result.Error != nil {
		return fmt.Errorf("resolve deal motion exit: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("resolve deal motion exit: deal pipeline or stage not found")
	}
	dealMotion := deal.DefaultCommercialMotion
	if deal.CommercialMotion != nil && strings.TrimSpace(*deal.CommercialMotion) != "" {
		dealMotion = strings.TrimSpace(*deal.CommercialMotion)
	}
	activeMotion := commercialMotionForDealMotion(dealMotion)
	query := r.db.WithContext(ctx).Model(&model.CRMSignal{}).
		Where("workspace_id = ? AND deal_id = ? AND superseded_at IS NULL", workspaceID, dealID)
	if deal.StageType == model.CRMStageTypeOpen {
		if dealMotion == model.CRMDealMotionExistingBusiness {
			// A broader customer classification does not invalidate specific evidence
			// already captured for expansion, renewal, onboarding, or retention.
			query = query.Where("commercial_motion IN ?", []string{model.CRMCommercialMotionProspecting, model.CRMCommercialMotionConversion})
		} else {
			query = query.Where("commercial_motion <> ?", activeMotion)
		}
		query = excludeQualifiedCommercialEvents(query)
	}
	now := time.Now().UTC()
	if err := query.Updates(map[string]interface{}{
		"superseded_at": now, "superseded_reason": "motion_exit",
	}).Error; err != nil {
		return fmt.Errorf("supersede signals after deal motion exit: %w", err)
	}
	return nil
}

// ReconcilePipelineMotionSignals applies a changed pipeline default to inheriting deals.
func (r *CRMSignalRepository) ReconcilePipelineMotionSignals(ctx context.Context, workspaceID, pipelineID string) error {
	if !r.db.Migrator().HasColumn(&model.CRMDeal{}, "commercial_motion") {
		return nil
	}
	var dealIDs []string
	if err := r.db.WithContext(ctx).Model(&model.CRMDeal{}).
		Where("workspace_id = ? AND pipeline_id = ? AND (commercial_motion IS NULL OR commercial_motion = ?)", workspaceID, pipelineID, "").
		Pluck("id", &dealIDs).Error; err != nil {
		return fmt.Errorf("list pipeline deals for motion reconciliation: %w", err)
	}
	for _, dealID := range dealIDs {
		if err := r.ReconcileDealMotionSignals(ctx, workspaceID, dealID); err != nil {
			return err
		}
	}
	return nil
}

// RefreshEntityMotionSignals resolves current applicability independently of
// signal detection and supersedes immutable interpretations for exited motions.
func (r *CRMSignalRepository) RefreshEntityMotionSignals(
	ctx context.Context,
	workspaceID, entityType, entityID string,
	effectiveAt time.Time,
) error {
	if !r.db.Migrator().HasTable(&model.CRMSignalMotionState{}) ||
		!r.db.Migrator().HasColumn(&model.CRMSignal{}, "superseded_at") {
		return nil
	}
	signal := model.CRMSignal{WorkspaceID: workspaceID, DetectedAt: effectiveAt}
	switch entityType {
	case "company":
		signal.CompanyID = &entityID
	case "contact":
		signal.ContactID = &entityID
	case "deal":
		signal.DealID = &entityID
	default:
		return fmt.Errorf("unsupported CRM signal motion entity %q", entityType)
	}
	motions, snapshot, err := r.resolveSignalMotions(ctx, &signal)
	if err != nil {
		return err
	}
	if err := r.persistSignalMotionState(ctx, &signal, motions, snapshot); err != nil {
		return err
	}
	entityScope := func(query *gorm.DB) *gorm.DB {
		switch entityType {
		case "company":
			return query.Where("company_id = ?", entityID)
		case "contact":
			// Deal-scoped interpretations are owned by deal reconciliation. A
			// contact lifecycle edit must not invalidate an otherwise-open deal.
			return query.Where("contact_id = ? AND deal_id IS NULL", entityID)
		case "deal":
			return query.Where("deal_id = ?", entityID)
		default:
			return query
		}
	}
	query := entityScope(r.db.WithContext(ctx).Model(&model.CRMSignal{}).
		Where("workspace_id = ? AND dismissed_at IS NULL AND superseded_at IS NULL", workspaceID)).
		Where("commercial_motion NOT IN ?", motions)
	if hasExistingBusinessContext(snapshot) {
		// Generic customer context broadens applicability. It is not evidence
		// that an already-observed renewal, expansion, or onboarding has ended.
		query = query.Where("commercial_motion IN ?", []string{model.CRMCommercialMotionProspecting, model.CRMCommercialMotionConversion})
	}
	query = excludeQualifiedCommercialEvents(query)
	var stale []model.CRMSignal
	if err := query.Find(&stale).Error; err != nil {
		return fmt.Errorf("load motion-exit signals: %w", err)
	}
	if len(stale) == 0 {
		return nil
	}
	staleIDs := make(map[string]bool, len(stale))
	staleIDList := make([]string, 0, len(stale))
	for _, candidate := range stale {
		staleIDs[candidate.ID] = true
		staleIDList = append(staleIDList, candidate.ID)
	}
	var allActive []model.CRMSignal
	active := entityScope(r.db.WithContext(ctx).Model(&model.CRMSignal{}).Where(
		"workspace_id = ? AND dismissed_at IS NULL AND superseded_at IS NULL", workspaceID,
	))
	if err := active.Find(&allActive).Error; err != nil {
		return fmt.Errorf("load motion direction before supersession: %w", err)
	}
	beforeDirection := storedSignalGroupDirection(allActive, nil)
	afterDirection := storedSignalGroupDirection(allActive, staleIDs)
	if err := r.db.WithContext(ctx).Model(&model.CRMSignal{}).
		Where("workspace_id = ? AND id IN ? AND superseded_at IS NULL", workspaceID, staleIDList).
		Updates(map[string]interface{}{
			"superseded_at": effectiveAt, "superseded_reason": "motion_exit",
		}).Error; err != nil {
		return fmt.Errorf("supersede signals after motion exit: %w", err)
	}
	if beforeDirection != 0 && afterDirection != 0 && beforeDirection != afterDirection {
		var target *model.CRMSignal
		for index := range allActive {
			candidate := &allActive[index]
			if staleIDs[candidate.ID] {
				continue
			}
			if target == nil || storedSignalMagnitude(*candidate) > storedSignalMagnitude(*target) {
				target = candidate
			}
		}
		if target != nil {
			if updateErr := r.db.WithContext(ctx).Model(&model.CRMSignal{}).
				Where("workspace_id = ? AND id = ?", workspaceID, target.ID).
				Update("direction_changed_by_supersession", true).Error; updateErr != nil {
				return fmt.Errorf("mark motion supersession direction change: %w", updateErr)
			}
		}
	}
	return nil
}

// ListContactCompanyIDs returns companies whose motion can change when a
// contact lifecycle changes.
func (r *CRMSignalRepository) ListContactCompanyIDs(ctx context.Context, workspaceID, contactID string) ([]string, error) {
	var companyIDs []string
	err := r.db.WithContext(ctx).Table("crm_associations").
		Distinct("CASE WHEN from_object_type = 'company' THEN from_object_id ELSE to_object_id END").
		Where("workspace_id = ?", workspaceID).
		Where(`(from_object_type = 'contact' AND from_object_id = ? AND to_object_type = 'company')
			OR (to_object_type = 'contact' AND to_object_id = ? AND from_object_type = 'company')`, contactID, contactID).
		Pluck("CASE WHEN from_object_type = 'company' THEN from_object_id ELSE to_object_id END", &companyIDs).Error
	return companyIDs, err
}

func (r *CRMSignalRepository) resolveSignalMotions(
	ctx context.Context,
	signal *model.CRMSignal,
) ([]string, model.JSONB, error) {
	snapshot := model.JSONB{}
	motions := map[string]struct{}{}
	if signal.DealID != nil && strings.TrimSpace(*signal.DealID) != "" &&
		r.db.Migrator().HasColumn(&model.CRMDeal{}, "commercial_motion") {
		var deal struct {
			CommercialMotion        *string
			DefaultCommercialMotion string
			StageType               string
		}
		result := r.db.WithContext(ctx).Table("crm_deals d").
			Select("d.commercial_motion, p.default_commercial_motion, s.stage_type").
			Joins("JOIN crm_pipelines p ON p.id = d.pipeline_id").
			Joins("JOIN crm_pipeline_stages s ON s.id = d.stage_id").
			Where("d.workspace_id = ? AND d.id = ?", signal.WorkspaceID, *signal.DealID).
			Scan(&deal)
		if result.Error != nil {
			return nil, nil, fmt.Errorf("resolve deal commercial motion: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return nil, nil, fmt.Errorf("resolve deal commercial motion: deal pipeline or stage not found")
		}
		dealMotion := deal.DefaultCommercialMotion
		if deal.CommercialMotion != nil && strings.TrimSpace(*deal.CommercialMotion) != "" {
			dealMotion = strings.TrimSpace(*deal.CommercialMotion)
		}
		addDealMotions(motions, dealMotion)
		snapshot["deal_commercial_motion"] = dealMotion
		snapshot["deal_stage_type"] = deal.StageType
	}

	if signal.ContactID != nil && strings.TrimSpace(*signal.ContactID) != "" {
		var contact struct {
			LifecycleStage string
			LeadStatus     string
		}
		result := r.db.WithContext(ctx).Table("crm_contacts").
			Select("lifecycle_stage, lead_status").
			Where("workspace_id = ? AND id = ?", signal.WorkspaceID, *signal.ContactID).
			Scan(&contact)
		if result.Error != nil {
			return nil, nil, fmt.Errorf("resolve contact commercial motion: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return nil, nil, fmt.Errorf("resolve contact commercial motion: contact not found")
		}
		if contact.LifecycleStage != "" {
			snapshot["lifecycle_stage"] = contact.LifecycleStage
			snapshot["lead_status"] = contact.LeadStatus
			addLifecycleMotions(motions, contact.LifecycleStage)
		}
	}

	if signal.CompanyID != nil && strings.TrimSpace(*signal.CompanyID) != "" &&
		r.db.Migrator().HasTable(&model.CRMCompanyCommercialState{}) {
		var state model.CRMCompanyCommercialState
		err := r.db.WithContext(ctx).
			Where("workspace_id = ? AND company_id = ?", signal.WorkspaceID, *signal.CompanyID).
			First(&state).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("resolve company commercial motion: %w", err)
		}
		if err == nil {
			addCommercialStateMotions(motions, state.State, signal.DetectedAt)
			snapshot["commercial_state"] = cloneJSONB(state.State)
			snapshot["commercial_state_updated_at"] = state.StateUpdatedAt
		}
	}
	if signal.CompanyID != nil && strings.TrimSpace(*signal.CompanyID) != "" {
		if err := r.addCompanyRelationshipMotions(ctx, signal.WorkspaceID, *signal.CompanyID, motions, snapshot); err != nil {
			return nil, nil, err
		}
	}

	if snapshot["deal_commercial_motion"] == model.CRMDealMotionExistingBusiness {
		// Deal-specific customer classification takes precedence over a stale lead
		// lifecycle or other new-business opportunities on the same account.
		delete(motions, model.CRMCommercialMotionProspecting)
		delete(motions, model.CRMCommercialMotionConversion)
	}
	if len(motions) == 0 {
		motions[model.CRMCommercialMotionProspecting] = struct{}{}
	}
	values := make([]string, 0, len(motions))
	for motion := range motions {
		values = append(values, motion)
	}
	sort.Strings(values)
	snapshot["motions_at_detection"] = values
	snapshot["resolver_version"] = 1
	return values, snapshot, nil
}

func (r *CRMSignalRepository) addCompanyRelationshipMotions(
	ctx context.Context,
	workspaceID, companyID string,
	motions map[string]struct{},
	snapshot model.JSONB,
) error {
	var contacts []struct{ LifecycleStage string }
	if err := r.db.WithContext(ctx).Table("crm_contacts").
		Select("lifecycle_stage").
		Where("workspace_id = ?", workspaceID).
		Where(`EXISTS (SELECT 1 FROM crm_associations ca WHERE ca.workspace_id = crm_contacts.workspace_id AND
			((ca.from_object_type = 'contact' AND ca.from_object_id = crm_contacts.id AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
			 OR (ca.to_object_type = 'contact' AND ca.to_object_id = crm_contacts.id AND ca.from_object_type = 'company' AND ca.from_object_id = ?)))`, companyID, companyID).
		Scan(&contacts).Error; err != nil {
		return fmt.Errorf("resolve company contact motions: %w", err)
	}
	for _, contact := range contacts {
		addLifecycleMotions(motions, contact.LifecycleStage)
	}
	var deals []struct {
		CommercialMotion        *string
		DefaultCommercialMotion string
		StageType               string
	}
	if err := r.db.WithContext(ctx).Table("crm_deals d").
		Select("d.commercial_motion, p.default_commercial_motion, s.stage_type").
		Joins("JOIN crm_pipelines p ON p.id = d.pipeline_id").
		Joins("JOIN crm_pipeline_stages s ON s.id = d.stage_id").
		Where("d.workspace_id = ? AND s.stage_type = ?", workspaceID, model.CRMStageTypeOpen).
		Where(`EXISTS (SELECT 1 FROM crm_associations ca WHERE ca.workspace_id = d.workspace_id AND
			((ca.from_object_type = 'deal' AND ca.from_object_id = d.id AND ca.to_object_type = 'company' AND ca.to_object_id = ?)
			 OR (ca.to_object_type = 'deal' AND ca.to_object_id = d.id AND ca.from_object_type = 'company' AND ca.from_object_id = ?)
			 OR (ca.from_object_type = 'deal' AND ca.from_object_id = d.id AND ca.to_object_type = 'contact' AND ca.to_object_id IN (
				SELECT CASE WHEN cca.from_object_type = 'contact' THEN cca.from_object_id ELSE cca.to_object_id END
				FROM crm_associations cca WHERE cca.workspace_id = d.workspace_id
				AND ((cca.from_object_type = 'contact' AND cca.to_object_type = 'company' AND cca.to_object_id = ?)
				 OR (cca.to_object_type = 'contact' AND cca.from_object_type = 'company' AND cca.from_object_id = ?))
			 ))
			 OR (ca.to_object_type = 'deal' AND ca.to_object_id = d.id AND ca.from_object_type = 'contact' AND ca.from_object_id IN (
				SELECT CASE WHEN cca.from_object_type = 'contact' THEN cca.from_object_id ELSE cca.to_object_id END
				FROM crm_associations cca WHERE cca.workspace_id = d.workspace_id
				AND ((cca.from_object_type = 'contact' AND cca.to_object_type = 'company' AND cca.to_object_id = ?)
				 OR (cca.to_object_type = 'contact' AND cca.from_object_type = 'company' AND cca.from_object_id = ?))
			 ))))`, companyID, companyID, companyID, companyID, companyID, companyID).
		Scan(&deals).Error; err != nil {
		return fmt.Errorf("resolve company deal motions: %w", err)
	}
	dealMotions := make([]string, 0, len(deals))
	for _, deal := range deals {
		dealMotion := deal.DefaultCommercialMotion
		if deal.CommercialMotion != nil && strings.TrimSpace(*deal.CommercialMotion) != "" {
			dealMotion = strings.TrimSpace(*deal.CommercialMotion)
		}
		addDealMotions(motions, dealMotion)
		dealMotions = append(dealMotions, dealMotion)
	}
	if len(dealMotions) > 0 {
		snapshot["open_deal_motions"] = dealMotions
	}
	return nil
}

func addCommercialStateMotions(motions map[string]struct{}, state model.JSONB, detectedAt time.Time) {
	status, _ := state["subscription_status"].(string)
	switch status {
	case "trialing", "active":
		motions[model.CRMCommercialMotionAdoption] = struct{}{}
		motions[model.CRMCommercialMotionRetention] = struct{}{}
	case "past_due", "cancel_scheduled":
		motions[model.CRMCommercialMotionRetention] = struct{}{}
	case "canceled":
		if canceledAt, ok := commercialStateTime(state["canceled_at"]); ok && !canceledAt.Before(detectedAt.Add(-90*24*time.Hour)) {
			motions[model.CRMCommercialMotionRetention] = struct{}{}
		}
	}
	if _, onboardingStarted := commercialStateTime(state["onboarding_started_at"]); onboardingStarted {
		if _, firstValue := commercialStateTime(state["first_value_at"]); !firstValue {
			motions[model.CRMCommercialMotionOnboarding] = struct{}{}
		}
	}
	if renewalAt, ok := commercialStateTime(state["renewal_at"]); ok {
		untilRenewal := renewalAt.Sub(detectedAt)
		if untilRenewal >= -30*24*time.Hour && untilRenewal <= 120*24*time.Hour {
			motions[model.CRMCommercialMotionRenewal] = struct{}{}
		}
	}
}

func commercialStateTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, !typed.IsZero()
	case string:
		parsed, err := time.Parse(time.RFC3339, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}

func addLifecycleMotions(motions map[string]struct{}, lifecycle string) {
	switch lifecycle {
	case model.CRMLifecycleCustomer, model.CRMLifecycleEvangelist:
		motions[model.CRMCommercialMotionAdoption] = struct{}{}
		motions[model.CRMCommercialMotionRetention] = struct{}{}
	case model.CRMLifecycleOpportunity:
		motions[model.CRMCommercialMotionConversion] = struct{}{}
	default:
		motions[model.CRMCommercialMotionProspecting] = struct{}{}
	}
}

func hasExistingBusinessContext(snapshot model.JSONB) bool {
	if snapshot["deal_commercial_motion"] == model.CRMDealMotionExistingBusiness {
		return true
	}
	motions, _ := snapshot["open_deal_motions"].([]string)
	for _, motion := range motions {
		if motion == model.CRMDealMotionExistingBusiness {
			return true
		}
	}
	return false
}

func addDealMotions(motions map[string]struct{}, dealMotion string) {
	if dealMotion == model.CRMDealMotionExistingBusiness {
		// A customer relationship is applicability, not evidence of an upsell or renewal.
		addLifecycleMotions(motions, model.CRMLifecycleCustomer)
		return
	}
	motions[commercialMotionForDealMotion(dealMotion)] = struct{}{}
}

func commercialMotionForDealMotion(dealMotion string) string {
	switch dealMotion {
	case model.CRMDealMotionExistingBusiness:
		return model.CRMCommercialMotionNeedsContext
	case model.CRMDealMotionExpansion:
		return model.CRMCommercialMotionExpansion
	case model.CRMDealMotionRenewal:
		return model.CRMCommercialMotionRenewal
	default:
		return model.CRMCommercialMotionConversion
	}
}

func (r *CRMSignalRepository) persistSignalObservation(
	ctx context.Context,
	signal *model.CRMSignal,
	motions []string,
	snapshot model.JSONB,
) (*string, error) {
	if !r.db.Migrator().HasTable(&model.CRMSignalObservation{}) {
		return nil, nil
	}
	ruleKey, ruleVersion := "manual_signal", 1
	if signal.RuleKey != nil && strings.TrimSpace(*signal.RuleKey) != "" {
		ruleKey = strings.TrimSpace(*signal.RuleKey)
	}
	if signal.RuleVersion != nil {
		ruleVersion = *signal.RuleVersion
	}
	observation := model.CRMSignalObservation{
		ID: uuid.NewString(), WorkspaceID: signal.WorkspaceID, ContactID: signal.ContactID,
		DealID: signal.DealID, CompanyID: signal.CompanyID, RuleKey: ruleKey,
		RuleVersion: ruleVersion, DetectorKind: signal.DetectorKind,
		SignalDomain: signal.SignalDomain, SourceType: signal.SourceType,
		SourceID: signal.SourceID, SourceThreadID: signal.SourceThreadID,
		Summary: signal.Summary, EvidenceExcerpt: signal.EvidenceExcerpt,
		Metadata: cloneJSONB(signal.Metadata), Confidence: signal.Confidence,
		ObservedAt: signal.DetectedAt, WindowStartedAt: signal.WindowStartedAt,
		WindowEndedAt: signal.WindowEndedAt, IdentityMethod: signal.EvidenceIdentityMethod,
		IdentityTrust:       signal.EvidenceIdentityTrust,
		EvidenceFingerprint: signal.EvidenceFingerprint,
		MotionsAtDetection:  model.JSONB{"values": motions}, ContextSnapshot: cloneJSONB(snapshot),
		ReplayCalibrationExcluded: signal.ReplayCalibrationExcluded,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&observation)
	if result.Error != nil {
		return nil, fmt.Errorf("create signal observation: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return &observation.ID, nil
	}
	query := r.db.WithContext(ctx).Model(&model.CRMSignalObservation{}).
		Where("workspace_id = ? AND rule_key = ? AND rule_version = ? AND evidence_fingerprint = ?",
			signal.WorkspaceID, ruleKey, ruleVersion, signal.EvidenceFingerprint)
	query = nullableSignalID(query, "contact_id", signal.ContactID)
	query = nullableSignalID(query, "deal_id", signal.DealID)
	query = nullableSignalID(query, "company_id", signal.CompanyID)
	var existing model.CRMSignalObservation
	if err := query.First(&existing).Error; err != nil {
		return nil, fmt.Errorf("load signal observation: %w", err)
	}
	return &existing.ID, nil
}

func signalMeaningFingerprint(signal model.CRMSignal) string {
	parts := []string{
		signal.WorkspaceID, signal.EvidenceFingerprint, signal.CommercialMotion, signal.SignalType, signal.Polarity,
		fmt.Sprintf("%d", signal.InterpretationVersion),
	}
	for _, entityID := range []*string{signal.ContactID, signal.DealID, signal.CompanyID} {
		if entityID == nil {
			parts = append(parts, "")
		} else {
			parts = append(parts, strings.TrimSpace(*entityID))
		}
	}
	if signal.RecommendedActionKey != nil {
		parts = append(parts, *signal.RecommendedActionKey)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum)
}

func signalInterpretationRuleKey(signal *model.CRMSignal) string {
	if signal != nil && signal.RuleKey != nil && strings.TrimSpace(*signal.RuleKey) != "" {
		return strings.TrimSpace(*signal.RuleKey)
	}
	return "manual_signal"
}

func cloneJSONB(source model.JSONB) model.JSONB {
	target := model.JSONB{}
	for key, value := range source {
		target[key] = value
	}
	return target
}

func insertInterpretedSignals(
	tx *gorm.DB,
	rows []model.CRMSignal,
) (bool, *model.CRMSignal, error) {
	created := false
	var primary *model.CRMSignal
	for index := range rows {
		if rows[index].ID == "" {
			rows[index].ID = uuid.NewString()
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows[index])
		if result.Error != nil {
			return false, nil, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		created = true
		if primary == nil {
			copy := rows[index]
			primary = &copy
		}
	}
	return created, primary, nil
}
