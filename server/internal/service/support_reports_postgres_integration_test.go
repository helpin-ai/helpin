//go:build integration

package service

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openSupportOpsPostgres opens a migrated PostgreSQL database and returns a
// transaction that is rolled back when the test ends.
func openSupportOpsPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SUPPORT_OPS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SUPPORT_OPS_TEST_DATABASE_URL to a migrated disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		tx.Rollback()
		_ = sqlDB.Close()
	})
	return tx
}

func TestSupportReportsPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	f := seedSupportReportFixture(t, db)
	t.Run("metrics", func(t *testing.T) { checkSupportReportOverviewMetrics(t, f) })
	t.Run("topics", func(t *testing.T) { checkSupportReportTopics(t, f) })
	t.Run("inbox access", func(t *testing.T) { checkSupportReportRespectsInboxAccess(t, f) })
	t.Run("timezone", func(t *testing.T) { checkSupportReportDailyUsesTimezone(t, f) })
	t.Run("csv", func(t *testing.T) { checkSupportReportCSVExport(t, f) })
	t.Run("sla drill-down", func(t *testing.T) { checkSupportReportSLAConversations(t, f) })
}
