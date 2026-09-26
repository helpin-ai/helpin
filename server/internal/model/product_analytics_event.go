package model

import "time"

type ProductAnalyticsEvent struct {
	SemanticKey string
	UserID      string
	AnonymousID string
	WorkspaceID string
	Name        string
	Source      string
	OccurredAt  time.Time
	Attributes  map[string]any
}
