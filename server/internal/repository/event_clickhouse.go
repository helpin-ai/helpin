package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maximumEventReadWindow = 366 * 24 * time.Hour

// EventRetentionPolicy is the deployment-owned TTL declared on usermaven.events.
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
	if err := db.QueryRowContext(ctx, `SELECT create_table_query FROM system.tables WHERE database = 'usermaven' AND name = 'events'`).Scan(&ddl); err != nil {
		return EventRetentionPolicy{}, fmt.Errorf("inspect usermaven.events retention: %w", err)
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

// ClickHouseEventRepository is a workspace-bound behavioral-event reader.
type ClickHouseEventRepository struct {
	db          *sql.DB
	projects    EventProjectResolver
	workspaceID string
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
	}, nil
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
	query := `SELECT uniqExact(event_id) FROM usermaven.events WHERE project_id IN (` +
		strings.Join(placeholders, ",") + `) AND _timestamp >= ? AND _timestamp < ?`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count workspace events: %w", err)
	}
	return count, nil
}
