//go:build integration

package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupAIUsagePostgres(t *testing.T) *AIUsageRepository {
	t.Helper()
	dsn := os.Getenv("BILLING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BILLING_TEST_DATABASE_URL must point to an isolated test database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schema := "billing_test_" + uuid.New().String()[:8]
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		conn, err := db.DB()
		if err != nil {
			t.Error(err)
		} else if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	scoped, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := scoped.DB()
		if err != nil {
			t.Error(err)
		} else if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := scoped.AutoMigrate(&model.AIUsagePeriod{}, &model.AIUsageReservation{}, &model.AIUsageLedgerEntry{}, &model.AIUsageSettlement{}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"CREATE UNIQUE INDEX one_open_period ON billing_ai_usage_periods(workspace_id) WHERE status = 'open'",
		"CREATE UNIQUE INDEX ledger_idempotency ON billing_ai_usage_ledger(idempotency_key)",
		"CREATE UNIQUE INDEX reservation_idempotency ON billing_ai_usage_reservations(idempotency_key)",
	} {
		if err := scoped.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if err := scoped.Create(&model.AIUsagePeriod{ID: "period", WorkspaceID: "ws", PeriodStart: now, PeriodEnd: now.Add(time.Hour), AllowanceMicrousd: 150_000_000, EnforcementMode: model.AIUsageEnforcementSoft, Status: "open", PricingVersion: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	return NewAIUsageRepository(scoped)
}

func TestAIUsagePostgresRolloverLedger(t *testing.T) {
	testAIUsageRolloverTransfersHolds(t, setupAIUsagePostgres(t))
}

func TestAIUsagePostgresConcurrentRolloverAndSettlement(t *testing.T) {
	repo := setupAIUsagePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "run", ReservedMicrousd: 100})
	if err != nil {
		t.Fatal(err)
	}
	boundary := time.Now().UTC().Add(-time.Minute)
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", boundary).Error; err != nil {
		t.Fatal(err)
	}
	schedule := AIUsagePeriodSchedule{WorkspaceID: "ws", Start: boundary, End: boundary.AddDate(0, 1, 0), AllowanceMicrousd: 150_000_000, EnforcementMode: model.AIUsageEnforcementSoft, PricingVersion: "test"}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var err error
			if i%2 == 0 {
				err = repo.RolloverPeriod(ctx, "period", schedule)
			} else {
				_, err = repo.Reconcile(ctx, AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 75, AllowLateUsage: true, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "terminal", EntryKind: "usage"}})
			}
			if err != nil {
				t.Errorf("concurrent operation: %v", err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	var totals struct{ Used, Reserved int64 }
	if err := repo.db.Model(&model.AIUsagePeriod{}).Select("SUM(used_microusd) AS used, SUM(reserved_microusd) AS reserved").Scan(&totals).Error; err != nil {
		t.Fatal(err)
	}
	if totals.Used != 75 || totals.Reserved != 0 {
		t.Fatalf("totals=%+v", totals)
	}
	var count int64
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("status = ?", "open").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("open periods=%d", count)
	}
}

func TestAIUsagePostgresTerminalRecovery(t *testing.T) {
	testAgentRuntimeRecovery(t, setupAIUsagePostgres(t))
}
