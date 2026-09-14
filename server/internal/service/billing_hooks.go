package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Billing depends only on the side effects it emits. This keeps its edition
// implementation independent from the host's concrete product services.
type billingIdentity interface {
	SyncWorkspace(context.Context, string, string)
	TrackWorkspaceEvent(context.Context, string, string, time.Time, map[string]any)
}

type billingAnalytics interface {
	Track(context.Context, model.ProductAnalyticsEvent)
}
