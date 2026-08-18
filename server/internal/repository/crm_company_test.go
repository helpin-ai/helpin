package repository

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLockCRMCompanyExternalIDUsesSeparatePostgresTextParameters(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	if err != nil {
		t.Fatalf("open dry-run postgres database: %v", err)
	}

	result := lockCRMCompanyExternalID(db, context.Background(), "workspace-1", " Account-42 ")
	if result.Error != nil {
		t.Fatalf("build company advisory lock: %v", result.Error)
	}
	if got, want := result.Statement.SQL.String(), "SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))"; got != want {
		t.Fatalf("lock SQL = %q, want %q", got, want)
	}
	if len(result.Statement.Vars) != 2 {
		t.Fatalf("lock variables = %v, want workspace and external ID separately", result.Statement.Vars)
	}
	for _, value := range result.Statement.Vars {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("lock variable %v is not a string", value)
		}
		if strings.ContainsRune(text, '\x00') {
			t.Fatalf("lock variable contains a PostgreSQL-rejected NUL byte: %q", text)
		}
	}
	if got, want := result.Statement.Vars[1], "account-42"; got != want {
		t.Fatalf("normalized external ID = %q, want %q", got, want)
	}
}
