package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIUsageSettlementCharge is one exact, cent-rounded Stripe settlement.
type AIUsageSettlementCharge struct {
	WorkspaceID, PeriodID, PricingVersion, SettlementVersion    string
	CustomerID, SubscriptionID, DraftInvoiceID, BillingInterval string
	PeriodStart, PeriodEnd                                      time.Time
	AmountCents                                                 int64
	IdempotencyKey                                              string
}

// StripeSettlementResult identifies the durable Stripe objects created.
type StripeSettlementResult struct{ InvoiceItemID, InvoiceID string }

// AIUsageSettlementGateway settles exact extra usage without credit products.
type AIUsageSettlementGateway interface {
	SettleAIUsage(context.Context, AIUsageSettlementCharge) (StripeSettlementResult, error)
}

// AIUsageSettlementStore persists worker progress and retry state.
type AIUsageSettlementStore interface {
	ListPendingSettlements(context.Context, int) ([]model.AIUsageSettlement, error)
	CompleteSettlement(context.Context, string, string, string) error
	FailSettlement(context.Context, string, string) error
}

// AIUsageSettlementWorker processes durable settlement rows outside period locks.
type AIUsageSettlementWorker struct {
	store   AIUsageSettlementStore
	gateway AIUsageSettlementGateway
}

// NewAIUsageSettlementWorker creates the exact-overage worker.
func NewAIUsageSettlementWorker(store AIUsageSettlementStore, gateway AIUsageSettlementGateway) *AIUsageSettlementWorker {
	return &AIUsageSettlementWorker{store: store, gateway: gateway}
}

// ProcessPendingSettlements processes a stable batch idempotently.
func (w *AIUsageSettlementWorker) ProcessPendingSettlements(ctx context.Context, limit int) (int, error) {
	settlements, err := w.store.ListPendingSettlements(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, settlement := range settlements {
		if settlement.RoundedInvoiceCents == 0 {
			if err := w.store.CompleteSettlement(ctx, settlement.ID, "", ""); err != nil {
				return processed, err
			}
			processed++
			continue
		}
		result, err := w.gateway.SettleAIUsage(ctx, AIUsageSettlementCharge{
			WorkspaceID: settlement.WorkspaceID, PeriodID: settlement.PeriodID,
			AmountCents: settlement.RoundedInvoiceCents, IdempotencyKey: settlement.IdempotencyKey,
			SettlementVersion: "v1",
		})
		if err != nil {
			if storeErr := w.store.FailSettlement(ctx, settlement.ID, err.Error()); storeErr != nil {
				return processed, fmt.Errorf("settle AI usage: %v; record failure: %w", err, storeErr)
			}
			continue
		}
		if err := w.store.CompleteSettlement(ctx, settlement.ID, result.InvoiceItemID, result.InvoiceID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}
