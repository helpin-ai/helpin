//go:build integration

package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSimpleSalesPipelineIntegerWidthsPostgres(t *testing.T) {
	dsn := os.Getenv("AI_PROFILES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AI_PROFILES_TEST_DATABASE_URL is required")
	}
	fixture, err := os.ReadFile("testdata/simple_sales_pipeline.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(fixture), "\\ir ../sql/20260912000102_crm_simple_sales_pipeline.sql")
	if len(parts) != 3 {
		t.Fatal("expected fixture setup, two migration invocations, and assertions")
	}
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, positionType := range []string{"integer", "bigint"} {
		for _, probabilityType := range []string{"integer", "bigint"} {
			t.Run(positionType+"/"+probabilityType, func(t *testing.T) {
				db, err := sql.Open("pgx", dsn)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				db.SetMaxOpenConns(1)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				schema := fmt.Sprintf("pipeline_width_%d", time.Now().UnixNano())
				if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
						t.Error(err)
					}
				}()
				setup := strings.Replace(parts[0], "BEGIN;", "", 1)
				setup = strings.Replace(setup, "position integer, probability integer", "position "+positionType+", probability "+probabilityType, 1)
				if _, err := db.ExecContext(ctx, setup); err != nil {
					t.Fatal(err)
				}
				conn, err := db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				err = ensureSchemaMigrationsTable(ctx, conn)
				if closeErr := conn.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, migration := range core {
					if migration.Version == "202609120001" || migration.Version == "20260912000102" {
						continue
					}
					if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations(version,name,checksum) VALUES ($1,$2,$3)", migration.Version, migration.Name, migration.Checksum); err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					if err := Up(ctx, db); err != nil {
						t.Fatal(err)
					}
				}
				assertions := strings.Replace(parts[2], "ROLLBACK;", "", 1)
				if _, err := db.ExecContext(ctx, assertions); err != nil {
					t.Fatal(err)
				}
				issues, err := Validate(ctx, db)
				if err != nil || len(issues) != 0 {
					t.Fatalf("migration ledger: %v %v", issues, err)
				}
			})
		}
	}
}
