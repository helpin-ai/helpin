package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/joho/godotenv"

	"github.com/helpin-ai/helpin/server/internal/chmigrate"
)

const usage = `Usage: clickhouse-migrate <command>

Commands:
  up          Apply all pending migrations
  status      Show applied and pending migrations
  validate    Fail if migrations are pending or modified

Environment:
  CLICKHOUSE_DSN    ClickHouse connection string
`

func main() {
	_ = godotenv.Load()
	if len(os.Args) != 2 {
		fmt.Print(usage)
		os.Exit(1)
	}
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		log.Fatal("CLICKHOUSE_DSN environment variable is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatalf("open ClickHouse: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping ClickHouse: %v", err)
	}

	switch os.Args[1] {
	case "up":
		if err := chmigrate.Up(ctx, db); err != nil {
			log.Fatalf("run ClickHouse migrations: %v", err)
		}
		fmt.Println("ClickHouse migrations applied")
	case "status":
		rows, err := chmigrate.Status(ctx, db)
		if err != nil {
			log.Fatalf("ClickHouse migration status: %v", err)
		}
		for _, row := range rows {
			status := "pending"
			appliedAt := ""
			if row.Applied {
				status = "applied"
				appliedAt = row.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", row.Version, status, row.Name, appliedAt)
		}
	case "validate":
		issues, err := chmigrate.Validate(ctx, db)
		if err != nil {
			log.Fatalf("validate ClickHouse migrations: %v", err)
		}
		if len(issues) == 0 {
			fmt.Println("ok — all ClickHouse migrations clean")
			return
		}
		for _, issue := range issues {
			fmt.Printf("%s\t%s\t%s\n", issue.Kind, issue.Version, issue.Name)
		}
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		fmt.Print(usage)
		os.Exit(1)
	}
}
