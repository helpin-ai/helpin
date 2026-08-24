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

// SmokeCount counts canonical and legacy events inside a required bounded window.
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
	query := `SELECT count() FROM usermaven.events WHERE project_id IN (` +
		strings.Join(placeholders, ",") + `) AND timestamp >= ? AND timestamp < ?`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count workspace events: %w", err)
	}
	return count, nil
}
