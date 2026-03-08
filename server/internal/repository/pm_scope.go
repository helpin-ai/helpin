package repository

import (
	"strings"

	"gorm.io/gorm"
)

// ScopeFilterOptions contains common team/workspace scope filtering options.
type ScopeFilterOptions struct {
	TeamID        *string
	IncludeShared bool
	Archived      *bool
}

// ApplyScopeFilter applies team/workspace scope and archived filtering to a GORM query.
func ApplyScopeFilter(query *gorm.DB, opts ScopeFilterOptions) *gorm.DB {
	if opts.Archived != nil {
		query = query.Where("archived = ?", *opts.Archived)
	}
	if opts.TeamID == nil || strings.TrimSpace(*opts.TeamID) == "" {
		if !opts.IncludeShared {
			query = query.Where("team_id IS NULL")
		}
		return query
	}
	if opts.IncludeShared {
		return query.Where("(team_id = ? OR team_id IS NULL)", *opts.TeamID)
	}
	return query.Where("team_id = ?", *opts.TeamID)
}
