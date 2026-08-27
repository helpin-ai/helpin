package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const excludeDetectedBotsSQL = " AND parsed_ua_bot != 'true'"

type behavioralRuleRow struct {
	AnonymousID       string
	ExternalUserID    string
	CompanyExternalID string
	IdentityMethod    string
	IdentityTrust     string
	ObservedAt        time.Time
	EventCount        int
	SessionCount      int
	Evidence          string
}

// EvaluateBehavioralSignalRule returns aggregate, tenant-bound event evidence.
func (r *ClickHouseEventRepository) EvaluateBehavioralSignalRule(
	ctx context.Context,
	config model.CRMSignalRuleConfig,
	windowStartedAt, windowEndedAt time.Time,
) ([]model.CRMSignalRuleCandidate, error) {
	if windowStartedAt.IsZero() || !windowEndedAt.After(windowStartedAt) {
		return nil, fmt.Errorf("valid behavioral evaluation window is required")
	}
	lookbackDays := ruleInt(config.Thresholds, "lookback_days", 7)
	lookbackStart := windowStartedAt.AddDate(0, 0, -lookbackDays)
	if windowEndedAt.Sub(lookbackStart) > maximumEventReadWindow {
		lookbackStart = windowEndedAt.Add(-maximumEventReadWindow)
	}
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("event project set is empty")
	}

	var rows []behavioralRuleRow
	switch config.RuleKey {
	case model.CRMSignalRuleRepeatedPricingActivity:
		rows, err = r.pathActivityRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleStrings(config.Thresholds, "paths", []string{"/pricing"}),
			ruleInt(config.Thresholds, "minimum_sessions", 2), true)
	case model.CRMSignalRuleProcurementPageActivity:
		rows, err = r.pathActivityRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleStrings(config.Thresholds, "paths", []string{"/security", "/compliance", "/procurement"}), 1, true)
	case model.CRMSignalRuleKnownContactReturned:
		rows, err = r.returnedAfterDormancyRows(ctx, projects, lookbackStart, windowStartedAt, windowEndedAt,
			ruleInt(config.Thresholds, "dormancy_days", 30))
	case model.CRMSignalRuleHighIntentProductEvent:
		rows, err = r.highIntentEventRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleStrings(config.Thresholds, "event_names", nil))
	case model.CRMSignalRuleSessionDepthSpike:
		rows, err = r.sessionDepthRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleInt(config.Thresholds, "minimum_pageviews", 5))
	case model.CRMSignalRuleNewAccountStakeholder:
		rows, err = r.newStakeholderRows(ctx, projects, lookbackStart, windowStartedAt, windowEndedAt)
	case model.CRMSignalRuleAnonymousAccountTraffic:
		rows, err = r.anonymousAccountRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleInt(config.Thresholds, "minimum_events", 3))
	case model.CRMSignalRuleCampaignReturn:
		rows, err = r.campaignReturnRows(ctx, projects, lookbackStart, windowStartedAt, windowEndedAt)
	case model.CRMSignalRulePreIdentification:
		rows, err = r.preIdentificationRows(ctx, projects, lookbackStart, windowStartedAt, windowEndedAt)
	case model.CRMSignalRuleConfiguredForm:
		rows, err = r.capturedFormRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleStrings(config.Thresholds, "form_ids", nil))
	case model.CRMSignalRuleIdentifiedArticleView:
		rows, err = r.identifiedArticleRows(ctx, projects, windowStartedAt, windowEndedAt,
			ruleStrings(config.Thresholds, "article_ids", nil))
	case model.CRMSignalRuleVersionedInteraction:
		rows, err = r.versionedInteractionRows(ctx, projects, windowStartedAt, windowEndedAt, config.RuleKey, config.Version)
	default:
		return nil, fmt.Errorf("unsupported behavioral signal rule %q", config.RuleKey)
	}
	if err != nil {
		return nil, err
	}
	return behavioralCandidates(config, r.workspaceID, rows), nil
}

func (r *ClickHouseEventRepository) capturedFormRows(ctx context.Context, projects []string, start, end time.Time, formIDs []string) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC())
	formFilter := ""
	if len(formIDs) > 0 {
		formFilter = " AND has(?, JSONExtractString(event_attributes, 'form_id'))"
		args = append(args, formIDs)
	}
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(JSONExtractString(event_attributes, 'form_id'))), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND event_type = '$form' AND user_anonymous_id != ''` + formFilter + excludeDetectedBotsSQL + `
		GROUP BY user_anonymous_id`
	return r.queryBrowserBehavioralRows(ctx, "configured form submissions", query, args...)
}

func (r *ClickHouseEventRepository) identifiedArticleRows(ctx context.Context, projects []string, start, end time.Time, articleIDs []string) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC())
	articleFilter := ""
	if len(articleIDs) > 0 {
		articleFilter = " AND has(?, JSONExtractString(event_attributes, 'article_id'))"
		args = append(args, articleIDs)
	}
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(JSONExtractString(event_attributes, 'article_id'))), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND event_type = 'article_view' AND identity_trust != 'untrusted'
		AND (user_anonymous_id != '' OR user_id != '')` + articleFilter + excludeDetectedBotsSQL + `
		GROUP BY coalesce(nullIf(user_anonymous_id, ''), user_id), user_anonymous_id`
	return r.queryBrowserBehavioralRows(ctx, "identified article views", query, args...)
}

func (r *ClickHouseEventRepository) versionedInteractionRows(ctx context.Context, projects []string, start, end time.Time, ruleKey string, version int) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC(), ruleKey, version)
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(JSONExtractString(event_attributes, 'interaction_type'))), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND event_type = '$interaction'
		AND JSONExtractString(event_attributes, 'capture_rule_key') = ?
		AND JSONExtractInt(event_attributes, 'capture_rule_version') = ?
		AND user_anonymous_id != ''` + excludeDetectedBotsSQL + ` GROUP BY user_anonymous_id`
	return r.queryBrowserBehavioralRows(ctx, "versioned captured interactions", query, args...)
}

func (r *ClickHouseEventRepository) pathActivityRows(ctx context.Context, projects []string, start, end time.Time, paths []string, minimumSessions int, verifiedOnly bool) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC(), paths)
	trustFilter := ""
	if verifiedOnly {
		trustFilter = " AND identity_trust = 'verified'"
	}
	args = append(args, minimumSessions)
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(doc_path)), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `)
		AND _timestamp >= ? AND _timestamp < ? AND event_type = 'pageview'
		AND multiSearchAnyCaseInsensitive(doc_path, ?) > 0` + trustFilter + excludeDetectedBotsSQL + `
		AND user_anonymous_id != '' GROUP BY user_anonymous_id HAVING uniqExact(session_id) >= ?`
	return r.queryBrowserBehavioralRows(ctx, "behavioral path activity", query, args...)
}

func (r *ClickHouseEventRepository) returnedAfterDormancyRows(ctx context.Context, projects []string, lookbackStart, start, end time.Time, dormancyDays int) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC(), dormancyDays)
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, minIf(_timestamp, _timestamp >= ?) AS observed_at,
		countIf(_timestamp >= ?) AS event_count, uniqExactIf(session_id, _timestamp >= ?) AS session_count,
		concat('Returned after ', toString(dateDiff('day', maxIf(_timestamp, _timestamp < ?), minIf(_timestamp, _timestamp >= ?))), ' days') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND identity_trust='verified' AND user_anonymous_id != ''` + excludeDetectedBotsSQL + ` GROUP BY user_anonymous_id
		HAVING maxIf(_timestamp, _timestamp < ?) > toDateTime64(0,3)
		AND minIf(_timestamp, _timestamp >= ?) > toDateTime64(0,3)
		AND dateDiff('day', maxIf(_timestamp, _timestamp < ?), minIf(_timestamp, _timestamp >= ?)) >= ?`
	// The SELECT and HAVING repeat the same bounded timestamps; keep positional arguments explicit.
	args = []any{start.UTC(), start.UTC(), start.UTC(), start.UTC(), start.UTC()}
	for _, project := range projects {
		args = append(args, project)
	}
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC(), start.UTC(), start.UTC(), dormancyDays)
	return r.queryBrowserBehavioralRows(ctx, "known-contact returns", query, args...)
}

func (r *ClickHouseEventRepository) highIntentEventRows(ctx context.Context, projects []string, start, end time.Time, eventNames []string) ([]behavioralRuleRow, error) {
	if len(eventNames) == 0 {
		return nil, nil
	}
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC(), eventNames)
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(event_type)), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND has(?, event_type) AND identity_method='server_event' AND identity_trust='verified'
		AND (user_anonymous_id != '' OR user_id != '')
		GROUP BY coalesce(nullIf(user_anonymous_id, ''), user_id), user_anonymous_id`
	return r.queryBehavioralRows(ctx, "high-intent product events", query, args...)
}

func (r *ClickHouseEventRepository) sessionDepthRows(ctx context.Context, projects []string, start, end time.Time, minimumPageviews int) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC(), minimumPageviews)
	query := `SELECT any(user_anonymous_id) AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		countIf(event_type='pageview') AS event_count, 1 AS session_count,
		concat(toString(countIf(event_type='pageview')), ' pageviews in one session') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND session_id != ''` + excludeDetectedBotsSQL + ` GROUP BY session_id HAVING countIf(event_type='pageview') >= ?`
	return r.queryBrowserBehavioralRows(ctx, "session depth spikes", query, args...)
}

func (r *ClickHouseEventRepository) newStakeholderRows(ctx context.Context, projects []string, lookbackStart, start, end time.Time) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC())
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, min(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count, 'First activity from this account visitor' AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND company_id != '' AND user_anonymous_id != ''` + excludeDetectedBotsSQL + ` GROUP BY user_anonymous_id
		HAVING min(_timestamp) >= ? AND min(_timestamp) < ?`
	return r.queryBrowserBehavioralRows(ctx, "new account stakeholders", query, args...)
}

func (r *ClickHouseEventRepository) anonymousAccountRows(ctx context.Context, projects []string, start, end time.Time, minimumEvents int) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, start.UTC(), end.UTC(), minimumEvents)
	query := `SELECT any(user_anonymous_id) AS anonymous_id, '' AS external_user_id,
		company_id AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, max(_timestamp) AS observed_at,
		count() AS event_count, uniqExact(session_id) AS session_count, 'Anonymous activity from a known account' AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND company_id != '' AND user_id = ''` + excludeDetectedBotsSQL + ` GROUP BY company_id HAVING count() >= ?`
	return r.queryBrowserBehavioralRows(ctx, "anonymous account traffic", query, args...)
}

func (r *ClickHouseEventRepository) campaignReturnRows(ctx context.Context, projects []string, lookbackStart, start, end time.Time) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC())
	query := `SELECT user_anonymous_id AS anonymous_id, any(user_id) AS external_user_id,
		any(company_id) AS company_external_id, any(identity_method) AS identity_method,
		any(identity_trust) AS identity_trust, maxIf(_timestamp, _timestamp >= ?) AS observed_at,
		countIf(_timestamp >= ?) AS event_count, uniqExactIf(session_id, _timestamp >= ?) AS session_count,
		arrayStringConcat(arraySort(groupUniqArray(10)(utm_source)), ', ') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND user_anonymous_id != ''` + excludeDetectedBotsSQL + ` GROUP BY user_anonymous_id
		HAVING min(_timestamp) < ? AND countIf(_timestamp >= ? AND (utm_source != '' OR click_id_gclid != '')) > 0`
	args = []any{start.UTC(), start.UTC(), start.UTC()}
	for _, project := range projects {
		args = append(args, project)
	}
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC())
	return r.queryBrowserBehavioralRows(ctx, "campaign returns", query, args...)
}

func (r *ClickHouseEventRepository) preIdentificationRows(ctx context.Context, projects []string, lookbackStart, start, end time.Time) ([]behavioralRuleRow, error) {
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC())
	query := `SELECT user_anonymous_id AS anonymous_id, argMax(user_id, _timestamp) AS external_user_id,
		argMax(company_id, _timestamp) AS company_external_id, argMax(identity_method, _timestamp) AS identity_method,
		argMax(identity_trust, _timestamp) AS identity_trust, minIf(_timestamp, _timestamp >= ? AND user_id != '') AS observed_at,
		countIf(user_id='') AS event_count, uniqExactIf(session_id, user_id='') AS session_count,
		concat(toString(countIf(user_id='')), ' earlier anonymous events') AS evidence
		FROM usermaven.events FINAL WHERE project_id IN (` + projectSQL + `) AND _timestamp >= ? AND _timestamp < ?
		AND user_anonymous_id != ''` + excludeDetectedBotsSQL + ` GROUP BY user_anonymous_id
		HAVING countIf(_timestamp < ? AND user_id='') > 0 AND countIf(_timestamp >= ? AND user_id!='') > 0`
	args = []any{start.UTC()}
	for _, project := range projects {
		args = append(args, project)
	}
	args = append(args, lookbackStart.UTC(), end.UTC(), start.UTC(), start.UTC())
	return r.queryBrowserBehavioralRows(ctx, "pre-identification history", query, args...)
}

func (r *ClickHouseEventRepository) queryBrowserBehavioralRows(
	ctx context.Context,
	operation, query string,
	args ...any,
) ([]behavioralRuleRow, error) {
	if !strings.Contains(query, excludeDetectedBotsSQL) {
		return nil, fmt.Errorf("query %s must exclude detected bots", operation)
	}
	return r.queryBehavioralRows(ctx, operation, query, args...)
}

func (r *ClickHouseEventRepository) queryBehavioralRows(ctx context.Context, operation, query string, args ...any) ([]behavioralRuleRow, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", operation, err)
	}
	defer rows.Close()
	result := make([]behavioralRuleRow, 0)
	for rows.Next() {
		var row behavioralRuleRow
		if err := rows.Scan(&row.AnonymousID, &row.ExternalUserID, &row.CompanyExternalID,
			&row.IdentityMethod, &row.IdentityTrust, &row.ObservedAt, &row.EventCount,
			&row.SessionCount, &row.Evidence); err != nil {
			return nil, fmt.Errorf("scan %s: %w", operation, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", operation, err)
	}
	return result, nil
}

func clickHouseProjectFilter(projects []string) (string, []any) {
	placeholders := make([]string, 0, len(projects))
	args := make([]any, 0, len(projects))
	for _, project := range projects {
		placeholders = append(placeholders, "?")
		args = append(args, project)
	}
	return strings.Join(placeholders, ","), args
}

func behavioralCandidates(config model.CRMSignalRuleConfig, workspaceID string, rows []behavioralRuleRow) []model.CRMSignalRuleCandidate {
	signalType, domain, polarity, sourceType, summary := behavioralRuleDimensions(config.RuleKey)
	result := make([]model.CRMSignalRuleCandidate, 0, len(rows))
	for _, row := range rows {
		item := candidate(workspaceID, config.RuleKey, signalType, domain, polarity, sourceType,
			nil, nil, nil, row.ObservedAt, summary, row.Evidence,
			model.JSONB{"event_count": row.EventCount, "session_count": row.SessionCount, "shadow_mode": config.ShadowMode,
				"activation_eligible": config.ActivationEligible})
		item.AnonymousID, item.ExternalUserID, item.CompanyExternalID = row.AnonymousID, row.ExternalUserID, row.CompanyExternalID
		item.EvidenceIdentityMethod, item.EvidenceIdentityTrust = row.IdentityMethod, row.IdentityTrust
		item.EvidenceFingerprint = fingerprintRuleCandidate(item)
		result = append(result, item)
	}
	return result
}

func behavioralRuleDimensions(ruleKey string) (string, string, string, string, string) {
	switch ruleKey {
	case model.CRMSignalRuleRepeatedPricingActivity:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Repeated pricing-page activity"
	case model.CRMSignalRuleProcurementPageActivity:
		return model.CRMSignalTimelineSignal, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Security or procurement research on an active account"
	case model.CRMSignalRuleKnownContactReturned:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Known contact returned after dormancy"
	case model.CRMSignalRuleHighIntentProductEvent:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainProductUsage, model.CRMSignalPolarityPositive, model.CRMSignalSourceProduct, "High-intent product event"
	case model.CRMSignalRuleSessionDepthSpike:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Session depth spiked"
	case model.CRMSignalRuleNewAccountStakeholder:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainRelationship, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "New stakeholder appeared at a known account"
	case model.CRMSignalRuleAnonymousAccountTraffic:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityNeutral, model.CRMSignalSourceWeb, "Anonymous traffic from a known account"
	case model.CRMSignalRuleCampaignReturn:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Known visitor returned through a campaign"
	case model.CRMSignalRuleConfiguredForm:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityPositive, model.CRMSignalSourceWeb, "Configured high-intent form submitted"
	case model.CRMSignalRuleIdentifiedArticleView:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityNeutral, model.CRMSignalSourceWeb, "Identified contact viewed a relevant article"
	case model.CRMSignalRuleVersionedInteraction:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityNeutral, model.CRMSignalSourceWeb, "Versioned high-intent interaction observed"
	default:
		return model.CRMSignalBuyingIntent, model.CRMSignalDomainWebBehavior, model.CRMSignalPolarityNeutral, model.CRMSignalSourceWeb, "Pre-identification activity became attributable"
	}
}
