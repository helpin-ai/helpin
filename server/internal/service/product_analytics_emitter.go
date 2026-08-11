package service

import "context"

type productAnalyticsEmitter struct {
	analytics *ProductAnalyticsService
}

// SetProductAnalyticsService enables canonical backend product events.
func (e *productAnalyticsEmitter) SetProductAnalyticsService(analytics *ProductAnalyticsService) {
	e.analytics = analytics
}

func (e *productAnalyticsEmitter) trackProductEvent(
	ctx context.Context,
	event ProductAnalyticsEvent,
) {
	if e != nil && e.analytics != nil {
		e.analytics.Track(ctx, event)
	}
}
