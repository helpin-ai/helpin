// Command crm-behavior-coverage reports, without writing anything, how many
// CRM contacts and companies product events can be tied to. It runs the same
// matching as the hourly behavior-profile sync, so its numbers are what the
// sync would store.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const usage = `Usage: crm-behavior-coverage [workspace-id ...]

Reports CRM behavior coverage for the given workspaces, or for every
workspace collecting events. Read-only: no profiles or leases are written.

Environment:
  DATABASE_URL      PostgreSQL connection string (a read replica is enough)
  CLICKHOUSE_DSN    ClickHouse connection string (read-only user is enough)
`

func main() {
	_ = godotenv.Load()
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-") {
		fmt.Print(usage)
		os.Exit(2)
	}
	databaseURL, clickHouseDSN := os.Getenv("DATABASE_URL"), os.Getenv("CLICKHOUSE_DSN")
	if databaseURL == "" || clickHouseDSN == "" {
		fmt.Print(usage)
		log.Fatal("DATABASE_URL and CLICKHOUSE_DSN are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("open PostgreSQL: %v", err)
	}
	events, err := sql.Open("clickhouse", clickHouseDSN)
	if err != nil {
		log.Fatalf("open ClickHouse: %v", err)
	}
	defer events.Close()
	if err := events.PingContext(ctx); err != nil {
		log.Fatalf("ping ClickHouse: %v", err)
	}

	projects := repository.NewEventProjectRepository(db)
	workspaceIDs := os.Args[1:]
	if len(workspaceIDs) == 0 {
		if workspaceIDs, err = projects.ListAllActiveEventWorkspaceIDs(ctx); err != nil {
			log.Fatalf("list event workspaces: %v", err)
		}
	}
	// Leases are never taken: ComputeWorkspace only reads.
	sync := service.NewCRMBehaviorProfileSynchronizer(repository.NewCRMBehaviorProfileRepository(db), nil, projects, events)
	if err := report(ctx, sync, workspaceIDs, time.Now().UTC()); err != nil {
		log.Fatal(err)
	}
}

func report(ctx context.Context, sync *service.CRMBehaviorProfileSynchronizer, workspaceIDs []string, now time.Time) error {
	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(out, "WORKSPACE\tCONTACTS\tWITH EVENTS\tEMAIL ONLY\tCOMPANIES\tWITH EVENTS\tIDENTIFIED KEYS\tUNMATCHED\tMATCH METHODS")
	var failures []string
	for _, workspaceID := range workspaceIDs {
		_, result, err := sync.ComputeWorkspace(ctx, workspaceID, now)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", workspaceID, err))
			continue
		}
		fmt.Fprintf(out, "%s\t%d\t%s\t%d\t%d\t%s\t%d\t%d\t%s\n", workspaceID,
			result.TotalContacts, share(result.Contacts, result.TotalContacts), result.EmailOnlyContacts,
			result.TotalCompanies, share(result.Companies, result.TotalCompanies),
			result.IdentifiedKeys, result.UnmatchedKeys, methods(result.MatchMethods))
	}
	if err := out.Flush(); err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("coverage failed for %d workspace(s):\n%s", len(failures), strings.Join(failures, "\n"))
	}
	return nil
}

func share(part, total int) string {
	if total == 0 {
		return "0"
	}
	return fmt.Sprintf("%d (%.0f%%)", part, 100*float64(part)/float64(total))
}

func methods(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, " ")
}
