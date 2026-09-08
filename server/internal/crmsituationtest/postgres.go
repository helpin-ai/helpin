package crmsituationtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openPostgres(t testing.TB, dsn string) *gorm.DB {
	t.Helper()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid disposable PostgreSQL test configuration")
	}
	// Never fall back to the application DSN or mutate an application schema.
	if config.Database != "crm_situation_test" {
		t.Fatal("PostgreSQL fixtures require the disposable crm_situation_test database")
	}
	schema := "situation_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	config.RuntimeParams["search_path"] = schema
	config.ConnectTimeout = 5 * time.Second
	sqlDB := stdlib.OpenDB(*config)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close disposable PostgreSQL connection: %v", err)
		}
	})
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open disposable PostgreSQL fixture: %v", err)
	}
	// The identifier is generated here, never derived from a request or DSN.
	Exec(t, db, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.WithContext(ctx).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("remove isolated test schema: %v", err)
		}
	})
	initialize(t, db)
	return db
}
