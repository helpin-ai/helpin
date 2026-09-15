//go:build ee

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/ee/pricing"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func flatBYOKRepositoryFixture(t *testing.T, allowance, used int64) (*AIUsageService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:flat-byok-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.AutoMigrate(&model.AIUsagePeriod{}, &model.AIUsageReservation{}, &model.AIUsageLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.AIUsagePeriod{ID: "period", WorkspaceID: "ws", PeriodStart: now, PeriodEnd: now.Add(time.Hour),
		AllowanceMicrousd: allowance, UsedMicrousd: used, EnforcementMode: model.AIUsageEnforcementStrict, Status: model.AIUsagePeriodOpen}).Error; err != nil {
		t.Fatal(err)
	}
	catalog := &pricing.Catalog{Tools: []pricing.ToolRate{{Key: "search", CustomerMicrousd: 7500}}}
	return NewAIUsageService(catalog, eerepository.NewAIUsageRepository(db), nil), db
}

func zeroRateChatGPTRequest() PreflightRequest {
	return PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", Provider: "openai_chatgpt", Model: "gpt-5.6-sol", IdempotencyKey: "chatgpt-run",
		FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(0), InputTokensEstimate: 2000, MaximumOutputTokens: 1000,
	}}
}

func TestFlatBYOKZeroRateRepositoryLifecycle(t *testing.T) {
	for _, used := range []int64{0, 10001} {
		t.Run(fmt.Sprintf("used_%d", used), func(t *testing.T) {
			svc, db := flatBYOKRepositoryFixture(t, 10000, used)
			ctx := context.Background()
			metering, err := svc.Preflight(ctx, zeroRateChatGPTRequest())
			if err != nil {
				t.Fatal(err)
			}
			if metering.ReservationID == "" || metering.MaxBillableMicrousd != 0 {
				t.Fatal("free BYOK requires a tracked zero-cost reservation")
			}
			retry, err := svc.Preflight(ctx, zeroRateChatGPTRequest())
			if err != nil || retry.ReservationID != metering.ReservationID {
				t.Fatalf("admission retry lost its reservation: %v", err)
			}
			turn := *metering
			turn.IdempotencyKey += ":turn-1"
			if _, err := svc.Checkpoint(ctx, CompletionUsage{Context: turn, Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 1000}, MeasurementStatus: "actual"}); err != nil {
				t.Fatal(err)
			}
			if err := svc.SuspendReservation(ctx, *metering); err != nil {
				t.Fatal(err)
			}
			if err := svc.Heartbeat(ctx, *metering); err != nil {
				t.Fatal(err)
			}
			completion := CompletionUsage{Context: *metering, Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 500, CompletionTokensTotal: 100}, MeasurementStatus: "actual"}
			for range 2 {
				result, err := svc.Reconcile(ctx, completion)
				if err != nil || result.ChargedMicrousd != 0 {
					t.Fatalf("free BYOK settlement: %v", err)
				}
			}
			var period model.AIUsagePeriod
			if err := db.First(&period, "id = ?", "period").Error; err != nil || period.UsedMicrousd != used || period.ReservedMicrousd != 0 {
				t.Fatal("free tokens consumed allowance")
			}
			var entries []model.AIUsageLedgerEntry
			if err := db.Find(&entries).Error; err != nil || len(entries) != 2 {
				t.Fatal("checkpoint and settlement were not recorded exactly once")
			}
			var tokens int64
			for _, entry := range entries {
				tokens += entry.InputTokensTotal + entry.OutputTokens
				if entry.FinalChargedMicrousd != 0 || entry.Provider != "openai_chatgpt" {
					t.Fatal("incorrect free ChatGPT ledger entry")
				}
			}
			if tokens != 1600 {
				t.Fatalf("raw token usage = %d, want 1600", tokens)
			}
			var reservation model.AIUsageReservation
			if err := db.First(&reservation, "id = ?", metering.ReservationID).Error; err != nil || reservation.Status != model.AIUsageReservationReconciled {
				t.Fatal("free reservation did not finish")
			}
		})
	}
}

func TestFlatBYOKZeroRateRepositoryPaidTools(t *testing.T) {
	for _, allowance := range []int64{0, 7500} {
		t.Run(fmt.Sprintf("allowance_%d", allowance), func(t *testing.T) {
			svc, db := flatBYOKRepositoryFixture(t, allowance, 0)
			req := zeroRateChatGPTRequest()
			req.Metering.AllowedPaidTools = []string{"search"}
			metering, err := svc.Preflight(context.Background(), req)
			if allowance == 0 {
				if !errors.Is(err, model.ErrAIUsageExhausted) {
					t.Fatalf("paid tool must still require allowance: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if metering.MaxBillableMicrousd != 7500 {
				t.Fatal("paid tool hold missing")
			}
			result, err := svc.Reconcile(context.Background(), CompletionUsage{Context: *metering,
				Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 1000}, PaidTools: []aiusage.PaidToolUsage{{Key: "search", Count: 1}}, MeasurementStatus: "actual"})
			if err != nil || result.ChargedMicrousd != 7500 {
				t.Fatalf("paid tool charge incorrect: %v", err)
			}
			var period model.AIUsagePeriod
			if err := db.First(&period, "id = ?", "period").Error; err != nil || period.UsedMicrousd != 7500 || period.ReservedMicrousd != 0 {
				t.Fatal("paid tool charge was not settled")
			}
		})
	}
}
