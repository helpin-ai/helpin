package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/helpin-ai/helpin/server/internal/dbmigrate"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		fatal("usage: migrate <up|status>")
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

	switch os.Args[1] {
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
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func fatal(message string) {
	log.Fatal(message)
}

func fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
