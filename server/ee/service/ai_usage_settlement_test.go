package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageSettlementWorkerChargesExactRoundedAmountIdempotently(t *testing.T) {
	store := &fakeSettlementStore{pending: []model.AIUsageSettlement{{
		ID: "settlement", WorkspaceID: "ws", PeriodID: "period", RoundedInvoiceCents: 34,
		IdempotencyKey: "ai-usage:ws:period:2026-08-13:v1", Status: model.AIUsageSettlementPending,
		PricingVersion: "2026-08-13", BillingInterval: "annual",
		StripeCustomerID: settlementString("cus_1"), StripeSubscriptionID: settlementString("sub_1"),
	}}}
	gateway := &fakeSettlementGateway{}
	worker := NewAIUsageSettlementWorker(store, gateway)
	processed, err := worker.ProcessPendingSettlements(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 || gateway.charge.AmountCents != 34 || gateway.charge.IdempotencyKey != "ai-usage:ws:period:2026-08-13:v1" {
		t.Fatalf("processed/charge = %d/%#v", processed, gateway.charge)
	}
	if gateway.charge.CustomerID != "cus_1" || gateway.charge.SubscriptionID != "sub_1" || gateway.charge.BillingInterval != "annual" {
		t.Fatalf("settlement identity = %#v", gateway.charge)
	}
	if store.completedID != "settlement" || store.invoiceItemID != "ii_1" {
		t.Fatalf("completion = %#v", store)
	}
}

func TestAIUsageSettlementWorkerSkipsZeroCentSettlement(t *testing.T) {
	store := &fakeSettlementStore{pending: []model.AIUsageSettlement{{ID: "zero", RoundedInvoiceCents: 0}}}
	gateway := &fakeSettlementGateway{}
	processed, err := NewAIUsageSettlementWorker(store, gateway).ProcessPendingSettlements(context.Background(), 10)
	if err != nil || processed != 1 || gateway.calls != 0 || store.completedID != "zero" {
		t.Fatalf("processed/error/calls/store = %d/%v/%d/%#v", processed, err, gateway.calls, store)
	}
}

type fakeSettlementStore struct {
	pending                               []model.AIUsageSettlement
	completedID, invoiceItemID, invoiceID string
}

func (f *fakeSettlementStore) ListPendingSettlements(context.Context, int) ([]model.AIUsageSettlement, error) {
	return f.pending, nil
}

func (f *fakeSettlementStore) CompleteSettlement(_ context.Context, id, item, invoice string) error {
	f.completedID, f.invoiceItemID, f.invoiceID = id, item, invoice
	return nil
}

func (f *fakeSettlementStore) FailSettlement(context.Context, string, string) error { return nil }

type fakeSettlementGateway struct {
	calls  int
	charge AIUsageSettlementCharge
}

func settlementString(value string) *string { return &value }

func (f *fakeSettlementGateway) SettleAIUsage(_ context.Context, charge AIUsageSettlementCharge) (StripeSettlementResult, error) {
	f.calls++
	f.charge = charge
	return StripeSettlementResult{InvoiceItemID: "ii_1", InvoiceID: "in_1"}, nil
}
