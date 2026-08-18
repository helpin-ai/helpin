// Command backfill-helpcenter-icons inventories and normalizes persisted Docs
// icon values against the generated Helpin icon catalog.
//
// Usage:
//
//	go run ./cmd/backfill-helpcenter-icons [--apply] [--workspace-id=UUID]
//	  [--clear-unresolved] [--clear-ascii-display-text]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/iconcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type options struct {
	apply                 bool
	workspaceID           string
	clearUnresolved       bool
	clearASCIIDisplayText bool
}

type iconRow struct {
	ID          string
	WorkspaceID string
	Icon        *string
}

type configRow struct {
	ID             string
	WorkspaceID    string
	HomepageConfig json.RawMessage
}

type normalization struct {
	value   *string
	kind    string
	changed bool
}

type inventory struct {
	counts  map[string]int
	values  map[string]int
	updated int
}

func main() {
	var opts options
	flag.BoolVar(&opts.apply, "apply", false, "write canonicalized values (default is dry-run)")
	flag.StringVar(&opts.workspaceID, "workspace-id", "", "process one workspace (empty = all)")
	flag.BoolVar(&opts.clearUnresolved, "clear-unresolved", false, "clear unknown identifier-shaped values when applying")
	flag.BoolVar(&opts.clearASCIIDisplayText, "clear-ascii-display-text", false, "clear reviewed legacy ASCII display text when applying")
	flag.Parse()

	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	report := &inventory{counts: map[string]int{}, values: map[string]int{}}
	for _, table := range []string{"docs_spaces", "docs_collections", "docs_documents"} {
		if err := processIconTable(ctx, db, table, opts, report); err != nil {
			log.Fatalf("%s: %v", table, err)
		}
	}
	if err := processHomepageConfig(ctx, db, opts, report); err != nil {
		log.Fatalf("docs_helpcenter_configs: %v", err)
	}

	keys := make([]string, 0, len(report.counts))
	for key := range report.counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		log.Printf("inventory %s=%d", key, report.counts[key])
	}
	valueKeys := make([]string, 0, len(report.values))
	for key := range report.values {
		valueKeys = append(valueKeys, key)
	}
	sort.Strings(valueKeys)
	for _, key := range valueKeys {
		log.Printf("review %s count=%d", key, report.values[key])
	}
	mode := "DRY-RUN"
	if opts.apply {
		mode = "APPLY"
	}
	log.Printf("[%s] complete: %d fields updated; catalog=%s hash=%s", mode, report.updated, iconcatalog.Version(), iconcatalog.Hash())
}

func processIconTable(ctx context.Context, db *gorm.DB, table string, opts options, report *inventory) error {
	query := db.WithContext(ctx).Table(table).
		Select("id, workspace_id, icon").
		Where("deleted_at IS NULL")
	if opts.workspaceID != "" {
		query = query.Where("workspace_id = ?", opts.workspaceID)
	}
	var rows []iconRow
	if err := query.Find(&rows).Error; err != nil {
		return fmt.Errorf("load icons: %w", err)
	}
	for _, row := range rows {
		next := normalizePersistedIcon(row.Icon, opts)
		recordInventory(report, next, row.Icon)
		if !next.changed || !opts.apply {
			continue
		}
		if err := db.WithContext(ctx).Table(table).Where("id = ?", row.ID).
			UpdateColumn("icon", databaseIconValue(next.value)).Error; err != nil {
			return fmt.Errorf("update %s: %w", row.ID, err)
		}
		report.updated++
	}
	return nil
}

func processHomepageConfig(ctx context.Context, db *gorm.DB, opts options, report *inventory) error {
	query := db.WithContext(ctx).Table("docs_helpcenter_configs").
		Select("id, workspace_id, homepage_config")
	if opts.workspaceID != "" {
		query = query.Where("workspace_id = ?", opts.workspaceID)
	}
	var rows []configRow
	if err := query.Find(&rows).Error; err != nil {
		return fmt.Errorf("load configs: %w", err)
	}
	for _, row := range rows {
		var homepage model.HelpcenterHomepageConfig
		if err := json.Unmarshal(row.HomepageConfig, &homepage); err != nil {
			report.counts["homepage-invalid-json"]++
			continue
		}
		changed := false
		for index := range homepage.FeaturedCards {
			current := &homepage.FeaturedCards[index].Icon
			next := normalizePersistedIcon(current, opts)
			recordInventory(report, next, current)
			if !next.changed {
				continue
			}
			changed = true
			if next.value == nil {
				homepage.FeaturedCards[index].Icon = ""
			} else {
				homepage.FeaturedCards[index].Icon = *next.value
			}
		}
		if !changed || !opts.apply {
			continue
		}
		encoded, err := json.Marshal(homepage)
		if err != nil {
			return fmt.Errorf("encode config %s: %w", row.ID, err)
		}
		if err := db.WithContext(ctx).Table("docs_helpcenter_configs").Where("id = ?", row.ID).
			UpdateColumn("homepage_config", encoded).Error; err != nil {
			return fmt.Errorf("update config %s: %w", row.ID, err)
		}
		report.updated++
	}
	return nil
}

func normalizePersistedIcon(value *string, opts options) normalization {
	if value == nil || strings.TrimSpace(*value) == "" {
		return normalization{kind: "empty"}
	}
	resolved := iconcatalog.ResolveStored(value)
	switch resolved.Kind {
	case iconcatalog.ResolutionAsset:
		next := resolved.Value
		return normalization{value: &next, kind: "canonical", changed: *value != next}
	case iconcatalog.ResolutionDisplayText:
		if isASCII(resolved.Value) {
			if opts.clearASCIIDisplayText {
				return normalization{kind: "ascii-display-cleared", changed: true}
			}
			return normalization{value: value, kind: "ascii-display-review"}
		}
		next := resolved.Value
		return normalization{value: &next, kind: "unicode-display", changed: *value != next}
	default:
		if opts.clearUnresolved {
			return normalization{kind: "unresolved-cleared", changed: true}
		}
		return normalization{value: value, kind: "unresolved-review"}
	}
}

func recordInventory(report *inventory, next normalization, current *string) {
	report.counts[next.kind]++
	if next.kind != "ascii-display-review" && next.kind != "unresolved-review" {
		return
	}
	value := "<nil>"
	if current != nil {
		value = strings.TrimSpace(*current)
	}
	report.values[next.kind+":"+value]++
}

func databaseIconValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func isASCII(value string) bool {
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		if r > 127 {
			return false
		}
		value = value[size:]
	}
	return true
}
