//go:build ee

package service

import (
	"context"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *BillingService) trackStripeSubscriptionEvents(
	ctx context.Context,
	previous billingAnalyticsState,
	update BillingStripeSubscriptionUpdate,
	billing *model.WorkspaceBilling,
) {
	if billing == nil || strings.TrimSpace(update.EventID) == "" {
		return
	}
	occurredAt := update.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now().UTC()
	}
	attributes := map[string]any{
		"plan": billing.Plan, "billing_interval": billing.BillingInterval,
		"billing_status": billing.Status, "subscription_id": update.StripeSubscriptionID,
		"cancel_at_period_end": billing.CancelAtPeriodEnd,
	}
	track := func(name string, extra map[string]any) {
		properties := make(map[string]any, len(attributes)+len(extra))
		for key, value := range attributes {
			properties[key] = value
		}
		for key, value := range extra {
			properties[key] = value
		}
		s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
			SemanticKey: "stripe:" + update.EventID + ":" + name,
			UserID:      update.UserID, WorkspaceID: billing.WorkspaceID,
			Name: name, Source: "stripe", OccurredAt: occurredAt, Attributes: properties,
		})
	}

	created := previous.SubscriptionID == "" && update.StripeSubscriptionID != ""
	if created && (update.EventType == "checkout.session.completed" ||
		update.EventType == "customer.subscription.created") {
		track("subscription_started", nil)
	}
	if previous.SubscriptionID != "" &&
		(previous.Plan != billing.Plan || previous.Interval != billing.BillingInterval) {
		track("subscription_plan_changed", map[string]any{
			"previous_plan": previous.Plan, "previous_billing_interval": previous.Interval,
		})
	}
	if !previous.CancelAtPeriodEnd && billing.CancelAtPeriodEnd {
		track("subscription_cancel_scheduled", nil)
	}
	if previous.CancelAtPeriodEnd && !billing.CancelAtPeriodEnd &&
		billing.Status != model.BillingStatusCanceled {
		track("subscription_resumed", nil)
	}
	if previous.Status != model.BillingStatusCanceled &&
		billing.Status == model.BillingStatusCanceled {
		track("subscription_canceled", map[string]any{"canceled_at": analyticsTime(billing.CanceledAt)})
	}
	if previous.Status == model.BillingStatusTrialing {
		switch billing.Status {
		case model.BillingStatusActive:
			track("trial_converted", nil)
		case model.BillingStatusTrialExpired, model.BillingStatusCanceled:
			track("trial_expired", nil)
		}
	}
}

func analyticsTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
