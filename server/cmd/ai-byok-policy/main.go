//go:build ee

// Command ai-byok-policy configures the SaaS workspace flag and immutable flat
// token tariff. It previews by default and never reprices accepted executions.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/ee/aiconnections"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	workspace := flag.String("workspace", "", "workspace UUID")
	mode := flag.String("mode", "", "enable or disable")
	version := flag.String("tariff-version", "", "immutable tariff version (required to enable)")
	rate := flag.String("microusd-per-million-tokens", "", "flat micro-USD per million tokens; 1000000 means $1, explicit 0 is allowed")
	apply := flag.Bool("apply", false, "commit changes (default is a rolled-back preview)")
	flag.Parse()
	if _, err := uuid.Parse(*workspace); err != nil {
		return fmt.Errorf("a workspace UUID is required")
	}
	if *mode != "enable" && *mode != "disable" {
		return fmt.Errorf("mode must be enable or disable")
	}
	options := aiconnections.ConfigureOptions{WorkspaceID: *workspace, Enabled: *mode == "enable", Apply: *apply}
	if options.Enabled {
		parsed, err := strconv.ParseInt(*rate, 10, 64)
		if err != nil || parsed < 0 || *version == "" {
			return fmt.Errorf("enabling requires a tariff version and explicit nonnegative integer rate")
		}
		options.Tariff = &aiusage.FlatTokenTariff{Version: *version, Currency: "USD", MicrousdPerMillion: &parsed, AccountingVersion: aiusage.FlatTokenAccountingVersion}
	} else if *rate != "" || *version != "" {
		return fmt.Errorf("disable preserves the tariff; omit tariff flags")
	}
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load configuration: %w", err)
	}
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := aiconnections.Configure(ctx, db, options)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
