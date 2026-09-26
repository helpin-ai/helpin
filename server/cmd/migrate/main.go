package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
)

const usage = `Usage: migrate <command> [args]

Commands:
  up          Apply all pending migrations
  status      Show all migrations and their applied/pending state
  head        Show the latest applied migration
  pending     List only unapplied migrations
  validate    Check for checksum mismatches and pending migrations (exit 1 if issues found)
  repair      Recalculate checksums for applied migrations whose files were edited
  create      Scaffold a new migration file: migrate create <name>
  cluster-rebuild
              One-shot: rebuild coverage gaps under the v2 clusterer
  support-inbox-backfill [batch-size]
              Idempotently backfill projected support inbox state (default 250)
  support-inbox-mode <workspace-id> <legacy|shadow|v2>
              Change one workspace's support state read mode after readiness checks

Environment:
  DATABASE_URL    PostgreSQL connection string (required for all commands except create)
`

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	cmd := os.Args[1]

	// create does not need a database connection.
	if cmd == "create" {
		runCreate()
		return
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatal("DATABASE_URL environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fatalf("ping database: %v", err)
	}

	switch cmd {
	case "up":
		if err := dbmigrate.Up(ctx, db, editionMigrationSources()...); err != nil {
			fatalf("run migrations: %v", err)
		}
		fmt.Println("migrations applied")

	case "status":
		rows, err := dbmigrate.Status(ctx, db, editionMigrationSources()...)
		if err != nil {
			fatalf("migration status: %v", err)
		}
		if len(rows) == 0 {
			fmt.Println("no migrations found")
			return
		}
		for _, row := range rows {
			status := "pending"
			appliedAt := ""
			if row.Applied {
				status = "applied"
				if row.AppliedAt != nil {
					appliedAt = row.AppliedAt.UTC().Format(time.RFC3339)
				}
			}
			if appliedAt == "" {
				fmt.Printf("%s\t%s\t%s\n", row.Version, status, row.Name)
				continue
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", row.Version, status, row.Name, appliedAt)
		}

	case "head":
		row, err := dbmigrate.Head(ctx, db)
		if err != nil {
			fatalf("migration head: %v", err)
		}
		if row == nil {
			fmt.Println("no migrations applied")
			return
		}
		appliedAt := ""
		if row.AppliedAt != nil {
			appliedAt = row.AppliedAt.UTC().Format(time.RFC3339)
		}
		fmt.Printf("%s\t%s\t%s\n", row.Version, row.Name, appliedAt)

	case "pending":
		rows, err := dbmigrate.Pending(ctx, db, editionMigrationSources()...)
		if err != nil {
			fatalf("pending migrations: %v", err)
		}
		if len(rows) == 0 {
			fmt.Println("no pending migrations")
			return
		}
		for _, row := range rows {
			fmt.Printf("%s\t%s\n", row.Version, row.Name)
		}

	case "validate":
		issues, err := dbmigrate.Validate(ctx, db, editionMigrationSources()...)
		if err != nil {
			fatalf("validate migrations: %v", err)
		}
		if len(issues) == 0 {
			fmt.Println("ok — all migrations clean")
			return
		}
		for _, issue := range issues {
			fmt.Printf("%s\t%s\t%s\n", issue.Kind, issue.Version, issue.Name)
		}
		os.Exit(1)

	case "repair":
		repaired, err := dbmigrate.Repair(ctx, db, editionMigrationSources()...)
		if err != nil {
			fatalf("repair migrations: %v", err)
		}
		fmt.Printf("repaired %d migration checksum(s)\n", repaired)

	case "cluster-rebuild":
		if err := runClusterRebuild(ctx, db); err != nil {
			fatalf("coverage cluster rebuild: %v", err)
		}

	case "support-inbox-backfill":
		backfillCtx, backfillCancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer backfillCancel()
		batchSize := 250
		if len(os.Args) >= 3 {
			parsed, parseErr := strconv.Atoi(os.Args[2])
			if parseErr != nil || parsed < 1 || parsed > 5000 {
				fatal("support inbox backfill batch size must be between 1 and 5000")
			}
			batchSize = parsed
		}

		var total int
		for {
			var processed int
			if err := db.QueryRowContext(backfillCtx, `SELECT support_inbox_backfill_batch($1)`, batchSize).Scan(&processed); err != nil {
				fatalf("support inbox backfill: %v", err)
			}
			total += processed
			fmt.Printf("support inbox backfill processed=%d total=%d\n", processed, total)
			if processed == 0 {
				break
			}
		}

	case "support-inbox-mode":
		if len(os.Args) != 4 {
			fatal("usage: migrate support-inbox-mode <workspace-id> <legacy|shadow|v2>")
		}
		if err := runSupportInboxMode(ctx, db, strings.TrimSpace(os.Args[2]), strings.TrimSpace(os.Args[3])); err != nil {
			fatalf("support inbox mode: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}

func runSupportInboxMode(ctx context.Context, db *sql.DB, workspaceID, mode string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace id is required")
	}
	if mode != "legacy" && mode != "shadow" && mode != "v2" {
		return fmt.Errorf("mode must be legacy, shadow, or v2")
	}
	if mode == "v2" {
		var currentMode string
		err := db.QueryRowContext(ctx, `
			SELECT COALESCE((SELECT mode FROM support_inbox_state_rollouts WHERE workspace_id = $1), 'legacy')`, workspaceID).
			Scan(&currentMode)
		if err != nil {
			return fmt.Errorf("load current rollout mode: %w", err)
		}
		if currentMode != "shadow" {
			return fmt.Errorf("workspace must be in shadow mode before v2 cutover (current=%s)", currentMode)
		}
	}
	if mode != "legacy" {
		var uninitialized, missingContributions int64
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM support_conversations conversation
			LEFT JOIN support_inbox_conversation_projection_states projection
			  ON projection.conversation_id = conversation.id AND projection.generation = 1
			WHERE conversation.workspace_id = $1 AND projection.conversation_id IS NULL`, workspaceID).Scan(&uninitialized); err != nil {
			return fmt.Errorf("check projection readiness: %w", err)
		}
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM support_conversations conversation
			WHERE conversation.workspace_id = $1
			  AND 5 <> (
				SELECT COUNT(*)
				FROM support_inbox_counter_contributions contribution
				WHERE contribution.conversation_id = conversation.id
				  AND contribution.audience_type = 'shared'
				  AND contribution.audience_id = 'shared'
			  )`, workspaceID).Scan(&missingContributions); err != nil {
			return fmt.Errorf("check counter readiness: %w", err)
		}
		if uninitialized != 0 || missingContributions != 0 {
			return fmt.Errorf("workspace is not ready: uninitialized_conversations=%d missing_counter_contributions=%d", uninitialized, missingContributions)
		}
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO support_inbox_state_rollouts (workspace_id, mode, generation, updated_at)
		SELECT id, $2, 1, NOW() FROM workspaces WHERE id = $1
		ON CONFLICT (workspace_id) DO UPDATE SET mode = EXCLUDED.mode, updated_at = NOW()`, workspaceID, mode)
	if err != nil {
		return fmt.Errorf("set rollout mode: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rollout result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("workspace not found")
	}
	fmt.Printf("support inbox workspace=%s mode=%s\n", workspaceID, mode)
	return nil
}

// runCreate scaffolds a new migration file. It resolves the sql/ directory
// relative to the dbmigrate package source so it works from any working
// directory within the repo.
func runCreate() {
	if len(os.Args) < 3 {
		fatal("usage: migrate create <name>")
	}
	name := strings.Join(os.Args[2:], "_")

	sqlDir := migrationSQLDir()
	path, err := dbmigrate.Create(sqlDir, name)
	if err != nil {
		fatalf("create migration: %v", err)
	}
	fmt.Printf("created %s\n", path)
}

// migrationSQLDir finds the dbmigrate/sql directory relative to this source file.
func migrationSQLDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fatal("cannot determine source file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "internal", "dbmigrate", "sql")
}

func fatal(message string) {
	log.Fatal(message)
}

func fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
