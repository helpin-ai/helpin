package repository

import (
	"context"
	"fmt"
	"time"
)

// Behavior identity key prefixes. Each event contributes one key per
// identifier it carries, so one person can surface under several keys.
const (
	BehaviorKeyUserID      = "u:"
	BehaviorKeyEmail       = "e:"
	BehaviorKeyAnonymousID = "a:"
	BehaviorKeyCompanyID   = "c:"
)

// _behaviorMaxSessionIDs caps the session ids returned per key. Session
// counts above the cap are reported as the cap.
const _behaviorMaxSessionIDs = 500

// BehaviorKeyStats aggregates product events for one identity key.
type BehaviorKeyStats struct {
	Key         string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	// RecentSessionIDs lists distinct sessions inside the recent window, so
	// callers can union sessions across the keys of one CRM record.
	RecentSessionIDs []string
}

// ListBehaviorKeyStats reads first and last activity plus recent sessions
// for every identified user, email, company, and the given anonymous ids,
// over non-bot events in [now - maximum read window, now).
func (r *ClickHouseEventRepository) ListBehaviorKeyStats(
	ctx context.Context, now time.Time, recentWindow time.Duration, anonymousIDs []string,
) ([]BehaviorKeyStats, error) {
	projects, err := r.projects.ResolveProjectSet(ctx, r.workspaceID)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, nil
	}
	end := now.UTC()
	start := end.Add(-maximumEventReadWindow)
	recentStart := end.Add(-recentWindow)
	projectSQL, projectArgs := clickHouseProjectFilter(projects)
	anonymousSQL := "''"
	var anonymousArgs []any
	if len(anonymousIDs) > 0 {
		anonymousSQL, anonymousArgs = clickHouseProjectFilter(anonymousIDs)
	}
	query := `SELECT key, min(_timestamp) AS first_seen, max(_timestamp) AS last_seen,
		groupUniqArrayIf(` + fmt.Sprint(_behaviorMaxSessionIDs) + `)(session_id, _timestamp >= ? AND session_id != '') AS recent_sessions
	FROM helpin.events FINAL
	ARRAY JOIN arrayFilter(k -> k != '', [
		if(user_id != '', concat('` + BehaviorKeyUserID + `', user_id), ''),
		if(user_email != '', concat('` + BehaviorKeyEmail + `', lower(trimBoth(user_email))), ''),
		if(user_anonymous_id != '' AND user_anonymous_id IN (` + anonymousSQL + `), concat('` + BehaviorKeyAnonymousID + `', user_anonymous_id), ''),
		if(company_id != '', concat('` + BehaviorKeyCompanyID + `', company_id), '')
	]) AS key
	WHERE project_id IN (` + projectSQL + `)
		AND _timestamp >= ? AND _timestamp < ?
		AND parsed_ua_bot != 'true'
	GROUP BY key`
	args := []any{recentStart}
	args = append(args, anonymousArgs...)
	args = append(args, projectArgs...)
	args = append(args, start, end)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list behavior key stats: %w", err)
	}
	defer rows.Close()
	var result []BehaviorKeyStats
	for rows.Next() {
		var stats BehaviorKeyStats
		if err := rows.Scan(&stats.Key, &stats.FirstSeenAt, &stats.LastSeenAt, &stats.RecentSessionIDs); err != nil {
			return nil, fmt.Errorf("scan behavior key stats: %w", err)
		}
		stats.FirstSeenAt, stats.LastSeenAt = stats.FirstSeenAt.UTC(), stats.LastSeenAt.UTC()
		result = append(result, stats)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate behavior key stats: %w", err)
	}
	return result, nil
}
