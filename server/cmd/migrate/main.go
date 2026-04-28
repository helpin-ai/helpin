package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
		if err := dbmigrate.Up(ctx, db); err != nil {
			fatalf("run migrations: %v", err)
		}
		fmt.Println("migrations applied")

	case "status":
		rows, err := dbmigrate.Status(ctx, db)
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
		rows, err := dbmigrate.Pending(ctx, db)
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
		issues, err := dbmigrate.Validate(ctx, db)
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
		repaired, err := dbmigrate.Repair(ctx, db)
		if err != nil {
			fatalf("repair migrations: %v", err)
		}
		fmt.Printf("repaired %d migration checksum(s)\n", repaired)

	case "cluster-rebuild":
		if err := runClusterRebuild(ctx, db); err != nil {
			fatalf("coverage cluster rebuild: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
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
