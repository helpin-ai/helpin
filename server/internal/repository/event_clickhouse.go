package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CommercialStateEvent is a server-authenticated company-state patch retained
// by the canonical event pipeline.
type CommercialStateEvent struct {
	EventID           string
	CompanyExternalID string
	IdentityMethod    string
	StateUpdatedAt    time.Time
	ObservedAt        time.Time
	Patch             map[string]interface{}
}

// UsageWeekdayBaseline is one external company's weekday expectation.
type UsageWeekdayBaseline struct {
	CompanyExternalID string
	MetricKey         string
	Weekday           int
	MedianValue       float64
	ObservationCount  int
	CompleteWeeks     int
	IdentityMethod    string
	WindowStartedAt   time.Time
	WindowEndedAt     time.Time
}

// UsageDailyMetric is one company-local calendar day's observed event count.
type UsageDailyMetric struct {
	CompanyExternalID string
	Day               time.Time
	Value             float64
	IdentityMethod    string
}

const maximumEventReadWindow = 366 * 24 * time.Hour

// EventRetentionPolicy is the deployment-owned TTL declared on helpin.events.
type EventRetentionPolicy struct {
	TTLConfigured bool
	TTLClause     string
}

// InspectEventRetentionPolicy reads table metadata without scanning tenant events.
func InspectEventRetentionPolicy(ctx context.Context, db *sql.DB) (EventRetentionPolicy, error) {
	if db == nil {
		return EventRetentionPolicy{}, fmt.Errorf("ClickHouse connection is required")
	}
	var ddl string
	if err := db.QueryRowContext(ctx, `SELECT create_table_query FROM system.tables WHERE database = 'helpin' AND name = 'events'`).Scan(&ddl); err != nil {
		return EventRetentionPolicy{}, fmt.Errorf("inspect helpin.events retention: %w", err)
	}
	return parseEventRetentionDDL(ddl), nil
}

func parseEventRetentionDDL(ddl string) EventRetentionPolicy {
	upper := strings.ToUpper(ddl)
	index := strings.Index(upper, " TTL ")
	if index < 0 {
		return EventRetentionPolicy{}
	}
	clause := strings.TrimSpace(ddl[index+5:])
	if settings := strings.Index(strings.ToUpper(clause), " SETTINGS "); settings >= 0 {
		clause = strings.TrimSpace(clause[:settings])
	}
	return EventRetentionPolicy{TTLConfigured: true, TTLClause: clause}
}

// EventProjectResolver resolves the projects owned by one internal workspace.
type EventProjectResolver interface {
	ResolveProjectSet(ctx context.Context, workspaceID string) ([]string, error)
}

// EventWorkspaceResolver additionally enumerates every workspace currently
// collecting events. Shadow jobs use this set so history exists before live
// rollout while feed and activation remain gated separately.
type EventWorkspaceResolver interface {
	EventProjectResolver
	ListAllActiveEventWorkspaceIDs(ctx context.Context) ([]string, error)
}

// ClickHouseEventRepository is a workspace-bound behavioral-event reader.
type ClickHouseEventRepository struct {
	db          *sql.DB
	projects    EventProjectResolver
	workspaceID string
	timezone    string
}

// NewClickHouseEventRepository constructs a reader bound to one internal workspace UUID.
func NewClickHouseEventRepository(
	db *sql.DB,
	projects EventProjectResolver,
	workspaceID string,
) (*ClickHouseEventRepository, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(workspaceID))
	if err != nil || parsed == uuid.Nil {
		return nil, fmt.Errorf("valid workspace UUID is required")
	}
	if db == nil || projects == nil {
		return nil, fmt.Errorf("ClickHouse connection and project resolver are required")
	}
	return &ClickHouseEventRepository{
		db:          db,
		projects:    projects,
		workspaceID: strings.ToLower(parsed.String()),
		timezone:    "UTC",
	}, nil
}

// SetTimezone selects the workspace-local calendar used by weekday rules.
func (r *ClickHouseEventRepository) SetTimezone(timezone string) {
	if strings.TrimSpace(timezone) == "" {
		timezone = "UTC"
	}
	r.timezone = timezone
}

// SmokeCount counts unique events inside a required bounded window. ReplacingMergeTree
// convergence is asynchronous, so a plain count can expose redelivery duplicates.
func (r *ClickHouseEventRepository) SmokeCount(
	ctx context.Context,
	windowStartedAt, windowEndedAt time.Time,
) (int64, error) {
	if windowStartedAt.IsZero() || windowEndedAt.IsZero() || !windowEndedAt.After(windowStartedAt) {
		return 0, fmt.Errorf("valid event window is required")
	}
	if windowEndedAt.Sub(windowStartedAt) > maximumEventReadWindow {
		return 0, fmt.Errorf("event window exceeds %s", maximumEventReadWindow)
	}
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return 0, err
	}
	if len(projects) == 0 {
		return 0, fmt.Errorf("event project set is empty")
	}

	args := make([]any, 0, len(projects)+2)
	placeholders := make([]string, 0, len(projects))
	for _, projectID := range projects {
		if strings.TrimSpace(projectID) == "" {
			return 0, fmt.Errorf("event project set contains an empty project")
		}
		placeholders = append(placeholders, "?")
		args = append(args, projectID)
	}
	args = append(args, windowStartedAt.UTC(), windowEndedAt.UTC())
	query := `SELECT uniqExact(event_id) FROM helpin.events WHERE project_id IN (` +
		strings.Join(placeholders, ",") + `) AND _timestamp >= ? AND _timestamp < ?`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count workspace events: %w", err)
	}
	return count, nil
}

// ListCommercialStateEvents returns bounded server-origin group state patches.
func (r *ClickHouseEventRepository) ListCommercialStateEvents(
	ctx context.Context,
	windowStartedAt, windowEndedAt time.Time,
) ([]CommercialStateEvent, error) {
	if windowStartedAt.IsZero() || !windowEndedAt.After(windowStartedAt) || windowEndedAt.Sub(windowStartedAt) > maximumEventReadWindow {
		return nil, fmt.Errorf("valid bounded commercial-state event window is required")
	}
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return nil, err
	}
	projectSQL, args := clickHouseProjectFilter(projects)
	args = append(args, windowStartedAt.UTC(), windowEndedAt.UTC())
	rows, err := r.db.QueryContext(ctx, `SELECT event_id, company_id, identity_method, _timestamp,
		coalesce(parseDateTime64BestEffortOrNull(JSONExtractString(event_attributes, 'state_updated_at')), _timestamp) AS state_updated_at,
		JSONExtractRaw(company_custom, 'commercial_state')
		FROM helpin.events FINAL
		WHERE project_id IN (`+projectSQL+`) AND _timestamp >= ? AND _timestamp < ?
		AND event_type = 'commercial_state_updated'
		AND identity_method = 'server_event' AND identity_trust = 'verified'
		AND company_id != '' AND JSONHas(company_custom, 'commercial_state')
		ORDER BY state_updated_at ASC, event_id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query commercial-state events: %w", err)
	}
	defer rows.Close()
	result := []CommercialStateEvent{}
	for rows.Next() {
		var event CommercialStateEvent
		var rawPatch string
		if err := rows.Scan(&event.EventID, &event.CompanyExternalID, &event.IdentityMethod,
			&event.ObservedAt, &event.StateUpdatedAt, &rawPatch); err != nil {
			return nil, fmt.Errorf("scan commercial-state event: %w", err)
		}
		if err := json.Unmarshal([]byte(rawPatch), &event.Patch); err != nil {
			return nil, fmt.Errorf("decode commercial-state event %s: %w", event.EventID, err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate commercial-state events: %w", err)
	}
	return result, nil
}

// BuildUsageWeekdayBaselines computes eight complete pre-evaluation weeks and
// excludes accounts without enough history for every weekday.
func (r *ClickHouseEventRepository) BuildUsageWeekdayBaselines(
	ctx context.Context, metricKey string, windowEndedAt time.Time, timezone string,
) ([]UsageWeekdayBaseline, error) {
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(timezone) == "" {
		timezone = "UTC"
	}
	location := mustLoadLocation(timezone)
	windowStartedAt := windowEndedAt.In(location).AddDate(0, 0, -56).UTC()
	historyStartedAt := windowEndedAt.Add(-maximumEventReadWindow)
	localEnd := windowEndedAt.In(location).Format("2006-01-02")
	projectSQL, args := clickHouseProjectFilter(projects)
	queryArgs := []any{timezone}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, historyStartedAt.UTC(), windowEndedAt.UTC(), metricKey, localEnd, localEnd, localEnd, localEnd)
	rows, err := r.db.QueryContext(ctx, `WITH source AS (
		SELECT company_id, toDate(_timestamp, ?) AS day
		FROM helpin.events FINAL WHERE project_id IN (`+projectSQL+`)
		AND _timestamp >= ? AND _timestamp < ? AND event_type = ?
		AND identity_trust = 'verified' AND company_id != ''
	), history_accounts AS (
		SELECT company_id FROM source GROUP BY company_id
		HAVING min(day) <= addDays(toDate(?), -56)
	), activity_coverage AS (
		SELECT company_id, toDayOfWeek(day) % 7 AS weekday, uniqExact(day) AS active_days
		FROM source WHERE day >= addDays(toDate(?), -56)
		GROUP BY company_id, weekday
	), eligible_accounts AS (
		SELECT history_accounts.company_id FROM history_accounts
		JOIN activity_coverage USING (company_id)
		GROUP BY history_accounts.company_id
		HAVING count() = 7 AND min(active_days) >= 3
	), daily AS (
		SELECT company_id, day, count() AS value FROM source
		WHERE day >= addDays(toDate(?), -56) GROUP BY company_id, day
	), calendar AS (
		SELECT eligible_accounts.company_id, addDays(toDate(?), -56 + number) AS day
		FROM eligible_accounts CROSS JOIN numbers(56)
	), filled AS (
		SELECT calendar.company_id, calendar.day, toDayOfWeek(calendar.day) % 7 AS weekday,
			coalesce(any(daily.value), 0) AS value
		FROM calendar LEFT JOIN daily USING (company_id, day)
		GROUP BY calendar.company_id, calendar.day
	), provenance AS (
		SELECT company_id,
			if(uniqExact(identity_method) = 1, any(identity_method), 'mixed_verified') AS identity_method
		FROM helpin.events FINAL WHERE project_id IN (`+projectSQL+`)
		AND _timestamp >= ? AND _timestamp < ? AND event_type = ?
		AND identity_trust = 'verified' AND company_id != '' GROUP BY company_id
	), stats AS (
		SELECT company_id, weekday, quantileExact(0.5)(value) AS median_value, count() AS observation_count
		FROM filled GROUP BY company_id, weekday
	)
	SELECT stats.company_id, stats.weekday, stats.median_value, stats.observation_count, provenance.identity_method
	FROM stats JOIN provenance USING (company_id) ORDER BY stats.company_id, stats.weekday`, append(queryArgs, argsForProvenance(args, historyStartedAt, windowEndedAt, metricKey)...)...)
	if err != nil {
		return nil, fmt.Errorf("build usage weekday baselines: %w", err)
	}
	defer rows.Close()
	result := []UsageWeekdayBaseline{}
	for rows.Next() {
		var baseline UsageWeekdayBaseline
		if err := rows.Scan(&baseline.CompanyExternalID, &baseline.Weekday, &baseline.MedianValue, &baseline.ObservationCount, &baseline.IdentityMethod); err != nil {
			return nil, fmt.Errorf("scan usage weekday baseline: %w", err)
		}
		baseline.MetricKey, baseline.CompleteWeeks = metricKey, 8
		baseline.WindowStartedAt, baseline.WindowEndedAt = windowStartedAt, windowEndedAt
		result = append(result, baseline)
	}
	return result, rows.Err()
}

func argsForProvenance(projectArgs []any, start, end time.Time, metricKey string) []any {
	result := append([]any{}, projectArgs...)
	return append(result, start.UTC(), end.UTC(), metricKey)
}

// ListRecentUsageDays reads only the seven completed workspace-local days.
func (r *ClickHouseEventRepository) ListRecentUsageDays(
	ctx context.Context, metricKey string, windowEndedAt time.Time, timezone string,
) ([]UsageDailyMetric, error) {
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load workspace timezone: %w", err)
	}
	localNow := windowEndedAt.In(location)
	localEnd := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	start, end := localEnd.AddDate(0, 0, -7).UTC(), localEnd.UTC()
	projectSQL, projectArgs := clickHouseProjectFilter(projects)
	args := []any{timezone}
	args = append(args, projectArgs...)
	args = append(args, start, end, metricKey)
	rows, err := r.db.QueryContext(ctx, `SELECT company_id, toDate(_timestamp, ?) AS day, count() AS value,
		if(uniqExact(identity_method) = 1, any(identity_method), 'mixed_verified') AS identity_method
		FROM helpin.events FINAL WHERE project_id IN (`+projectSQL+`)
		AND _timestamp >= ? AND _timestamp < ? AND event_type = ?
		AND identity_trust = 'verified' AND company_id != ''
		GROUP BY company_id, day ORDER BY company_id, day`, args...)
	if err != nil {
		return nil, fmt.Errorf("list recent usage days: %w", err)
	}
	defer rows.Close()
	result := []UsageDailyMetric{}
	for rows.Next() {
		var row UsageDailyMetric
		if err := rows.Scan(&row.CompanyExternalID, &row.Day, &row.Value, &row.IdentityMethod); err != nil {
			return nil, fmt.Errorf("scan recent usage day: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}
