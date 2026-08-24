package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// EvaluatePostgresSignalRule evaluates one bounded cross-module rule.
func (r *CRMSignalRepository) EvaluatePostgresSignalRule(
	ctx context.Context,
	config model.CRMSignalRuleConfig,
	windowStartedAt, windowEndedAt time.Time,
) ([]model.CRMSignalRuleCandidate, error) {
	switch config.RuleKey {
	case model.CRMSignalRuleSupportVolumeSpike:
		return r.supportVolumeSpikeCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleUrgentIssueOpenDeal:
		return r.urgentIssueOpenDealCandidates(ctx, windowEndedAt)
	case model.CRMSignalRuleSupportAIEscalation:
		return r.supportEscalationCandidates(ctx, windowStartedAt, windowEndedAt)
	case model.CRMSignalRuleSupportCSATDeterioration:
		return r.supportCSATCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleRequestedFeatureShipped:
		return r.featureShippedCandidates(ctx, windowStartedAt, windowEndedAt)
	case model.CRMSignalRuleDealStageStalled:
		return r.stageStalledCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleDealGoneDark:
		return r.goneDarkCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleChampionQuiet:
		return r.championQuietCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleTimelineFollowupLapsed:
		return r.timelineLapsedCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleDealSingleThreaded:
		return r.singleThreadedCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleRenewalApproaching:
		return r.renewalCandidates(ctx, config, windowEndedAt)
	case model.CRMSignalRuleBuyingCommitteeExpanded:
		return r.committeeExpandedCandidates(ctx, config, windowStartedAt, windowEndedAt)
	case model.CRMSignalRuleBuyingCommitteeShrank:
		return r.committeeShrankCandidates(ctx, config, windowEndedAt)
	default:
		return nil, fmt.Errorf("unsupported Postgres signal rule %q", config.RuleKey)
	}
}

type supportVolumeSpikeRow struct {
	WorkspaceID string
	CompanyID   string
	RecentCount int
	Baseline    int
	ObservedAt  time.Time
}

func (r *CRMSignalRepository) supportVolumeSpikeCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	recentDays := ruleInt(config.Thresholds, "recent_days", 7)
	baselineDays := ruleInt(config.Thresholds, "baseline_days", 28)
	minimumRecent := ruleInt(config.Thresholds, "minimum_recent", 3)
	multiplier := ruleFloat(config.Thresholds, "multiplier", 2)
	var rows []supportVolumeSpikeRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT workspace_id, crm_company_id AS company_id,
			COUNT(*) FILTER (WHERE created_at >= ?) AS recent_count,
			COUNT(*) FILTER (WHERE created_at < ?) AS baseline,
			MAX(created_at) AS observed_at
		FROM support_conversations
		WHERE crm_company_id IS NOT NULL AND created_at >= ? AND created_at < ? AND status <> 'spam'
		GROUP BY workspace_id, crm_company_id
		HAVING COUNT(*) FILTER (WHERE created_at >= ?) >= ?
			AND COUNT(*) FILTER (WHERE created_at >= ?) >= GREATEST(1, COUNT(*) FILTER (WHERE created_at < ?) * ?)
	`, end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -(recentDays+baselineDays)), end,
		end.AddDate(0, 0, -recentDays), minimumRecent, end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -recentDays), multiplier*float64(recentDays)/float64(baselineDays)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate support volume spike: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		companyID := row.CompanyID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleSupportVolumeSpike, model.CRMSignalRiskSignal,
			model.CRMSignalDomainSupport, model.CRMSignalPolarityNegative, model.CRMSignalSourceSupport,
			nil, nil, &companyID, row.ObservedAt,
			fmt.Sprintf("Support volume spiked to %d conversations", row.RecentCount),
			fmt.Sprintf("%d recent conversations versus %d in the baseline window", row.RecentCount, row.Baseline),
			model.JSONB{"recent_count": row.RecentCount, "baseline_count": row.Baseline}))
	}
	return result, nil
}

type linkedDealEvidenceRow struct {
	WorkspaceID string
	CompanyID   *string
	ContactID   *string
	DealID      string
	SourceID    string
	Subject     string
	ObservedAt  time.Time
	Value       float64
	Count       int
}

func (r *CRMSignalRepository) urgentIssueOpenDealCandidates(ctx context.Context, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.workspace_id, c.crm_company_id AS company_id, c.crm_contact_id AS contact_id,
			d.id AS deal_id, c.id AS source_id, c.subject, GREATEST(c.updated_at, c.created_at) AS observed_at
		FROM support_conversations c
		JOIN crm_deals d ON d.workspace_id = c.workspace_id
		JOIN crm_pipeline_stages ps ON ps.id = d.stage_id AND ps.stage_type = 'open'
		WHERE c.crm_company_id IS NOT NULL AND c.priority IN ('urgent', 'high')
			AND c.status IN ('open', 'waiting_on_customer') AND EXISTS (
				SELECT 1 FROM crm_associations a WHERE a.workspace_id = c.workspace_id AND (
					(a.from_object_type = 'deal' AND a.from_object_id = d.id AND a.to_object_type = 'company' AND a.to_object_id = c.crm_company_id)
					OR (a.to_object_type = 'deal' AND a.to_object_id = d.id AND a.from_object_type = 'company' AND a.from_object_id = c.crm_company_id)
				)
			) AND c.updated_at < ?
	`, end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate urgent support issues: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		sourceID, dealID := row.SourceID, row.DealID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleUrgentIssueOpenDeal, model.CRMSignalRiskSignal,
			model.CRMSignalDomainSupport, model.CRMSignalPolarityNegative, model.CRMSignalSourceSupport,
			row.ContactID, &dealID, row.CompanyID, row.ObservedAt,
			"Urgent support issue is open during an active deal", row.Subject,
			model.JSONB{"source_refs": []string{"support_conversation:" + sourceID, "deal:" + dealID}}))
		result[len(result)-1].SourceID = &sourceID
	}
	return result, nil
}

func (r *CRMSignalRepository) supportEscalationCandidates(ctx context.Context, start, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	var rows []struct {
		WorkspaceID string
		CompanyID   *string
		ContactID   *string
		SourceID    string
		Subject     string
		ObservedAt  time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT workspace_id, crm_company_id AS company_id, crm_contact_id AS contact_id,
			id AS source_id, subject, ai_escalated_at AS observed_at
		FROM support_conversations
		WHERE ai_escalated_at >= ? AND ai_escalated_at < ?
	`, start, end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate support escalations: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		sourceID := row.SourceID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleSupportAIEscalation, model.CRMSignalRiskSignal,
			model.CRMSignalDomainSupport, model.CRMSignalPolarityNegative, model.CRMSignalSourceSupport,
			row.ContactID, nil, row.CompanyID, row.ObservedAt, "Support conversation escalated to a human", row.Subject,
			model.JSONB{"source_refs": []string{"support_conversation:" + sourceID}}))
		result[len(result)-1].SourceID = &sourceID
	}
	return result, nil
}

func (r *CRMSignalRepository) supportCSATCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	recentDays := ruleInt(config.Thresholds, "recent_days", 30)
	baselineDays := ruleInt(config.Thresholds, "baseline_days", 90)
	minimumRatings := ruleInt(config.Thresholds, "minimum_ratings", 2)
	minimumDrop := ruleFloat(config.Thresholds, "minimum_drop", 1)
	var rows []struct {
		WorkspaceID string
		CompanyID   string
		RecentAvg   float64
		BaselineAvg float64
		RatingCount int
		ObservedAt  time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
		WITH ratings AS (
			SELECT c.workspace_id, c.crm_company_id AS company_id, m.created_at,
				CASE WHEN COALESCE(m.metadata::jsonb ->> 'rating','') ~ '^[0-9]+([.][0-9]+)?$'
					THEN (m.metadata::jsonb ->> 'rating')::numeric END AS rating
			FROM support_messages m JOIN support_conversations c ON c.id = m.conversation_id
			WHERE c.crm_company_id IS NOT NULL AND m.message_type = 'csat_survey'
				AND m.created_at >= ? AND m.created_at < ?
		)
		SELECT workspace_id, company_id,
			AVG(rating) FILTER (WHERE created_at >= ?) AS recent_avg,
			AVG(rating) FILTER (WHERE created_at < ?) AS baseline_avg,
			COUNT(rating) FILTER (WHERE created_at >= ?) AS rating_count,
			MAX(created_at) AS observed_at
		FROM ratings GROUP BY workspace_id, company_id
		HAVING COUNT(rating) FILTER (WHERE created_at >= ?) >= ?
			AND AVG(rating) FILTER (WHERE created_at < ?) - AVG(rating) FILTER (WHERE created_at >= ?) >= ?
	`, end.AddDate(0, 0, -(recentDays+baselineDays)), end, end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -recentDays),
		end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -recentDays), minimumRatings,
		end.AddDate(0, 0, -recentDays), end.AddDate(0, 0, -recentDays), minimumDrop).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate CSAT deterioration: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		companyID := row.CompanyID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleSupportCSATDeterioration, model.CRMSignalRiskSignal,
			model.CRMSignalDomainSupport, model.CRMSignalPolarityNegative, model.CRMSignalSourceSupport,
			nil, nil, &companyID, row.ObservedAt, "Customer satisfaction deteriorated",
			fmt.Sprintf("Recent CSAT %.1f fell from %.1f", row.RecentAvg, row.BaselineAvg),
			model.JSONB{"recent_average": row.RecentAvg, "baseline_average": row.BaselineAvg, "rating_count": row.RatingCount}))
	}
	return result, nil
}

func (r *CRMSignalRepository) featureShippedCandidates(ctx context.Context, start, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	var rows []struct {
		WorkspaceID string
		CompanyID   string
		SourceID    string
		Name        string
		ObservedAt  time.Time
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.workspace_id, CASE WHEN a.from_object_type = 'company' THEN a.from_object_id ELSE a.to_object_id END AS company_id,
			t.id AS source_id, t.name, t.completed_at AS observed_at
		FROM pm_tasks t JOIN crm_associations a ON a.workspace_id = t.workspace_id AND (
			(a.from_object_type = 'task' AND a.from_object_id = t.id AND a.to_object_type = 'company')
			OR (a.to_object_type = 'task' AND a.to_object_id = t.id AND a.from_object_type = 'company'))
		WHERE t.completed = true AND t.completed_at >= ? AND t.completed_at < ?
	`, start, end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate shipped requested features: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		companyID, sourceID := row.CompanyID, row.SourceID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleRequestedFeatureShipped, model.CRMSignalBuyingIntent,
			model.CRMSignalDomainDelivery, model.CRMSignalPolarityPositive, model.CRMSignalSourcePM,
			nil, nil, &companyID, row.ObservedAt, "A requested feature shipped", row.Name,
			model.JSONB{"source_refs": []string{"task:" + sourceID}}))
		result[len(result)-1].SourceID = &sourceID
	}
	return result, nil
}

func (r *CRMSignalRepository) stageStalledCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	days := ruleInt(config.Thresholds, "default_stage_days", 30)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT d.workspace_id, d.id AS deal_id, d.id AS source_id, ps.name AS subject,
			COALESCE(MAX(a.created_at) FILTER (WHERE a.event_type IN ('deal.stage_changed','deal.created')), d.created_at) AS observed_at
		FROM crm_deals d JOIN crm_pipeline_stages ps ON ps.id = d.stage_id AND ps.stage_type = 'open'
		LEFT JOIN pm_activity_log a ON a.workspace_id = d.workspace_id AND a.entity_type = 'deal' AND a.entity_id = d.id
		GROUP BY d.workspace_id, d.id, ps.name, d.created_at
		HAVING COALESCE(MAX(a.created_at) FILTER (WHERE a.event_type IN ('deal.stage_changed','deal.created')), d.created_at) < ?
	`, end.AddDate(0, 0, -days)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate stalled deals: %w", err)
	}
	return dealRiskCandidates(rows, model.CRMSignalRuleDealStageStalled, "Deal is stalled in its current stage", func(row linkedDealEvidenceRow) string {
		return fmt.Sprintf("No stage movement for at least %d days in %s", days, row.Subject)
	}), nil
}

func (r *CRMSignalRepository) goneDarkCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	days := ruleInt(config.Thresholds, "default_silence_days", 21)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH activity AS (
			SELECT d.workspace_id, d.id AS deal_id, d.id AS source_id, d.name AS subject,
			GREATEST(
				COALESCE((SELECT MAX(m.sent_at) FROM crm_email_messages m WHERE m.workspace_id=d.workspace_id AND m.deal_id=d.id AND m.direction='inbound'), d.created_at),
				COALESCE((SELECT MAX(a.occurred_at) FROM crm_activities a WHERE a.workspace_id=d.workspace_id AND a.deal_id=d.id), d.created_at),
				COALESCE((SELECT MAX(e.start_time) FROM crm_calendar_events e WHERE e.workspace_id=d.workspace_id AND e.deal_id=d.id AND e.status='confirmed' AND e.start_time < ?), d.created_at),
				COALESCE((SELECT MAX(c.updated_at) FROM support_conversations c WHERE c.workspace_id=d.workspace_id
					AND c.crm_company_id IS NOT NULL AND EXISTS (SELECT 1 FROM crm_associations ca WHERE ca.workspace_id=d.workspace_id AND
						((ca.from_object_type='deal' AND ca.from_object_id=d.id AND ca.to_object_type='company' AND ca.to_object_id=c.crm_company_id)
						OR (ca.to_object_type='deal' AND ca.to_object_id=d.id AND ca.from_object_type='company' AND ca.from_object_id=c.crm_company_id)))), d.created_at)
			) AS observed_at
			FROM crm_deals d JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		)
		SELECT * FROM activity WHERE observed_at < ?
	`, end, end.AddDate(0, 0, -days)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate gone-dark deals: %w", err)
	}
	return dealRiskCandidates(rows, model.CRMSignalRuleDealGoneDark, "Deal has gone dark", func(row linkedDealEvidenceRow) string {
		return fmt.Sprintf("No inbound deal activity for at least %d days", days)
	}), nil
}

func (r *CRMSignalRepository) championQuietCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	days := ruleInt(config.Thresholds, "silence_days", 21)
	lookback := ruleInt(config.Thresholds, "lookback_days", 180)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT s.workspace_id, s.contact_id, s.deal_id, s.id AS source_id, s.summary AS subject,
			COALESCE((SELECT MAX(m.sent_at) FROM crm_email_messages m
				JOIN crm_email_message_contacts mc ON mc.message_id=m.id AND mc.workspace_id=m.workspace_id
				WHERE m.workspace_id=s.workspace_id AND mc.contact_id=s.contact_id AND m.direction='inbound'), s.detected_at) AS observed_at
		FROM crm_buyer_signals s
		JOIN crm_deals d ON d.id=s.deal_id AND d.workspace_id=s.workspace_id
		JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		WHERE s.signal_type='champion_signal' AND s.contact_id IS NOT NULL AND s.detected_at >= ?
			AND COALESCE((SELECT MAX(m.sent_at) FROM crm_email_messages m
				JOIN crm_email_message_contacts mc ON mc.message_id=m.id AND mc.workspace_id=m.workspace_id
				WHERE m.workspace_id=s.workspace_id AND mc.contact_id=s.contact_id AND m.direction='inbound'), s.detected_at) < ?
	`, end.AddDate(0, 0, -lookback), end.AddDate(0, 0, -days)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate quiet champions: %w", err)
	}
	result := dealRiskCandidates(rows, model.CRMSignalRuleChampionQuiet, "Champion went quiet", func(row linkedDealEvidenceRow) string {
		return fmt.Sprintf("No inbound response from the champion for at least %d days", days)
	})
	for index := range result {
		result[index].ContactID = rows[index].ContactID
	}
	return result, nil
}

func (r *CRMSignalRepository) timelineLapsedCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	graceDays := ruleInt(config.Thresholds, "grace_days", 3)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH timelines AS (
			SELECT s.*, CASE WHEN COALESCE(NULLIF(s.metadata->>'timeline_date',''), NULLIF(s.metadata->>'date',''))
				~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}' THEN COALESCE(NULLIF(s.metadata->>'timeline_date',''),
				NULLIF(s.metadata->>'date',''))::timestamptz END AS promised_at
			FROM crm_buyer_signals s WHERE s.signal_type='timeline_signal'
		)
		SELECT s.workspace_id, s.contact_id, s.deal_id, s.id AS source_id, s.summary AS subject,
			s.promised_at AS observed_at
		FROM timelines s JOIN crm_deals d ON d.id=s.deal_id AND d.workspace_id=s.workspace_id
		JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		WHERE s.promised_at < ?
			AND NOT EXISTS (SELECT 1 FROM crm_email_messages m WHERE m.workspace_id=s.workspace_id AND m.deal_id=s.deal_id
				AND m.sent_at > s.promised_at)
	`, end.AddDate(0, 0, -graceDays)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate lapsed timeline follow-ups: %w", err)
	}
	result := dealRiskCandidates(rows, model.CRMSignalRuleTimelineFollowupLapsed, "Promised follow-up lapsed", func(row linkedDealEvidenceRow) string { return row.Subject })
	for index := range result {
		result[index].ContactID = rows[index].ContactID
	}
	return result, nil
}

func (r *CRMSignalRepository) singleThreadedCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	minimumAmount := ruleFloat(config.Thresholds, "minimum_amount", 10000)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH participants AS (
			SELECT m.workspace_id,m.deal_id,mc.contact_id::text AS contact_id,m.sent_at AS occurred_at
			FROM crm_email_messages m JOIN crm_email_message_contacts mc ON mc.message_id=m.id AND mc.workspace_id=m.workspace_id
			WHERE m.deal_id IS NOT NULL AND m.sent_at < ?
			UNION ALL
			SELECT e.workspace_id,e.deal_id,jsonb_array_elements_text(e.contact_ids::jsonb),e.start_time
			FROM crm_calendar_events e WHERE e.deal_id IS NOT NULL AND e.start_time < ?
		)
		SELECT d.workspace_id, d.id AS deal_id, d.id AS source_id, d.name AS subject,
			COALESCE(MAX(p.occurred_at), d.updated_at) AS observed_at, COUNT(DISTINCT p.contact_id) AS count
		FROM crm_deals d JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		LEFT JOIN participants p ON p.workspace_id=d.workspace_id AND p.deal_id=d.id
		WHERE COALESCE(d.amount,0) >= ?
		GROUP BY d.workspace_id, d.id, d.name, d.updated_at
		HAVING COUNT(DISTINCT p.contact_id) = 1
	`, end, end, minimumAmount).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate single-threaded deals: %w", err)
	}
	return dealRiskCandidates(rows, model.CRMSignalRuleDealSingleThreaded, "Deal is single-threaded", func(row linkedDealEvidenceRow) string {
		return "Only one external contact has engaged on this high-value deal"
	}), nil
}

func (r *CRMSignalRepository) renewalCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	leadDays := ruleInt(config.Thresholds, "lead_days", 90)
	engagementDays := ruleInt(config.Thresholds, "engagement_days", 30)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH renewals AS (
			SELECT d.*, CASE WHEN COALESCE(NULLIF(d.custom_properties->>'renewal_date',''),
				NULLIF(d.custom_properties->>'contract_end_date','')) ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}'
				THEN COALESCE(NULLIF(d.custom_properties->>'renewal_date',''),
				NULLIF(d.custom_properties->>'contract_end_date',''))::timestamptz END AS renewal_at
			FROM crm_deals d
		)
		SELECT d.workspace_id, d.id AS deal_id, d.id AS source_id, d.name AS subject, d.renewal_at AS observed_at
		FROM renewals d JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		WHERE d.renewal_at BETWEEN ? AND ?
			AND NOT EXISTS (SELECT 1 FROM crm_email_messages m WHERE m.workspace_id=d.workspace_id AND m.deal_id=d.id
				AND m.direction='inbound' AND m.sent_at >= ?)
	`, end, end.AddDate(0, 0, leadDays), end.AddDate(0, 0, -engagementDays)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate renewal windows: %w", err)
	}
	return dealRiskCandidates(rows, model.CRMSignalRuleRenewalApproaching, "Renewal window is approaching without recent engagement", func(row linkedDealEvidenceRow) string {
		return fmt.Sprintf("Renewal is within %d days and no inbound engagement was recorded in %d days", leadDays, engagementDays)
	}), nil
}

func (r *CRMSignalRepository) committeeExpandedCandidates(ctx context.Context, config model.CRMSignalRuleConfig, start, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	baselineDays := ruleInt(config.Thresholds, "baseline_days", 90)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH participants AS (
			SELECT m.workspace_id, m.deal_id, mc.contact_id::text AS contact_id, m.sent_at AS occurred_at
			FROM crm_email_messages m JOIN crm_email_message_contacts mc ON mc.message_id=m.id AND mc.workspace_id=m.workspace_id
			WHERE m.deal_id IS NOT NULL AND m.sent_at >= ? AND m.sent_at < ?
			UNION ALL
			SELECT e.workspace_id, e.deal_id, jsonb_array_elements_text(e.contact_ids::jsonb), e.start_time
			FROM crm_calendar_events e WHERE e.deal_id IS NOT NULL AND e.start_time >= ? AND e.start_time < ?
		)
		SELECT p.workspace_id, p.deal_id, p.deal_id AS source_id, d.name AS subject, MAX(p.occurred_at) AS observed_at,
			COUNT(DISTINCT p.contact_id) FILTER (WHERE p.occurred_at >= ?) AS count
		FROM participants p JOIN crm_deals d ON d.id=p.deal_id AND d.workspace_id=p.workspace_id
		JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		GROUP BY p.workspace_id,p.deal_id,d.name
		HAVING COUNT(DISTINCT p.contact_id) FILTER (WHERE p.occurred_at >= ?) >
			COUNT(DISTINCT p.contact_id) FILTER (WHERE p.occurred_at < ?)
	`, start.AddDate(0, 0, -baselineDays), end, start.AddDate(0, 0, -baselineDays), end, start, start, start).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate committee expansion: %w", err)
	}
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		dealID := row.DealID
		result = append(result, candidate(row.WorkspaceID, model.CRMSignalRuleBuyingCommitteeExpanded, model.CRMSignalBuyingIntent,
			model.CRMSignalDomainRelationship, model.CRMSignalPolarityPositive, model.CRMSignalSourceCRM,
			nil, &dealID, nil, row.ObservedAt, "Buying committee expanded",
			fmt.Sprintf("%d contacts engaged in the current window", row.Count), model.JSONB{"participant_count": row.Count}))
	}
	return result, nil
}

func (r *CRMSignalRepository) committeeShrankCandidates(ctx context.Context, config model.CRMSignalRuleConfig, end time.Time) ([]model.CRMSignalRuleCandidate, error) {
	silenceDays := ruleInt(config.Thresholds, "silence_days", 30)
	lookbackDays := ruleInt(config.Thresholds, "lookback_days", 180)
	var rows []linkedDealEvidenceRow
	err := r.db.WithContext(ctx).Raw(`
		WITH engagement AS (
			SELECT m.workspace_id,m.deal_id,mc.contact_id,MAX(m.sent_at) AS last_seen
			FROM crm_email_messages m JOIN crm_email_message_contacts mc ON mc.message_id=m.id AND mc.workspace_id=m.workspace_id
			WHERE m.deal_id IS NOT NULL AND m.sent_at >= ? AND m.sent_at < ?
			GROUP BY m.workspace_id,m.deal_id,mc.contact_id
		)
		SELECT e.workspace_id,e.deal_id,e.deal_id AS source_id,d.name AS subject,MAX(e.last_seen) AS observed_at,
			COUNT(*) FILTER (WHERE e.last_seen < ?) AS count
		FROM engagement e JOIN crm_deals d ON d.id=e.deal_id AND d.workspace_id=e.workspace_id
		JOIN crm_pipeline_stages ps ON ps.id=d.stage_id AND ps.stage_type='open'
		GROUP BY e.workspace_id,e.deal_id,d.name
		HAVING COUNT(*) >= 2 AND COUNT(*) FILTER (WHERE e.last_seen >= ?) <= 1
	`, end.AddDate(0, 0, -lookbackDays), end, end.AddDate(0, 0, -silenceDays), end.AddDate(0, 0, -silenceDays)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("evaluate committee shrinkage: %w", err)
	}
	return dealRiskCandidates(rows, model.CRMSignalRuleBuyingCommitteeShrank, "Buying committee engagement shrank", func(row linkedDealEvidenceRow) string {
		return fmt.Sprintf("Previously engaged contacts have been absent for %d days", silenceDays)
	}), nil
}

func dealRiskCandidates(rows []linkedDealEvidenceRow, ruleKey, summary string, evidence func(linkedDealEvidenceRow) string) []model.CRMSignalRuleCandidate {
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		dealID := row.DealID
		item := candidate(row.WorkspaceID, ruleKey, model.CRMSignalRiskSignal,
			model.CRMSignalDomainRelationship, model.CRMSignalPolarityNegative, model.CRMSignalSourceCRM,
			nil, &dealID, row.CompanyID, row.ObservedAt, summary, evidence(row),
			model.JSONB{"source_refs": []string{"deal:" + dealID}})
		item.SourceID = stringPointer(row.SourceID)
		result = append(result, item)
	}
	return result
}

func candidate(
	workspaceID, ruleKey, signalType, domain, polarity, sourceType string,
	contactID, dealID, companyID *string,
	observedAt time.Time,
	summary, evidence string,
	metadata model.JSONB,
) model.CRMSignalRuleCandidate {
	item := model.CRMSignalRuleCandidate{
		WorkspaceID: workspaceID, ContactID: contactID, DealID: dealID, CompanyID: companyID,
		RuleKey: ruleKey, SignalType: signalType, SignalDomain: domain, Polarity: polarity, SourceType: sourceType,
		Summary: summary, EvidenceExcerpt: evidence, ObservedAt: observedAt.UTC(), Metadata: metadata,
		EvidenceIdentityMethod: model.IdentityMethodConnectedMailbox, EvidenceIdentityTrust: model.IdentityTrustVerified,
	}
	if domain == model.CRMSignalDomainSupport {
		item.EvidenceIdentityMethod = model.IdentityMethodVerifiedSupport
	}
	item.EvidenceFingerprint = fingerprintRuleCandidate(item)
	return item
}

func fingerprintRuleCandidate(item model.CRMSignalRuleCandidate) string {
	parts := []string{item.WorkspaceID, item.RuleKey, item.Summary, item.EvidenceExcerpt,
		item.ObservedAt.UTC().Format(time.RFC3339Nano), item.AnonymousID, item.ExternalUserID, item.CompanyExternalID}
	for _, value := range []*string{item.ContactID, item.DealID, item.CompanyID, item.SourceID} {
		if value != nil {
			parts = append(parts, *value)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum)
}

func ruleInt(values model.JSONB, key string, fallback int) int {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func ruleFloat(values model.JSONB, key string, fallback float64) float64 {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func ruleStrings(values model.JSONB, key string, fallback []string) []string {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	items, ok := value.([]interface{})
	if !ok {
		if strings, ok := value.([]string); ok {
			return strings
		}
		return fallback
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && text != "" {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
