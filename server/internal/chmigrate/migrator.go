package chmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// Migration is one versioned ClickHouse schema change.
type Migration struct {
	Version  string
	Name     string
	Path     string
	SQL      string
	Checksum string
}

// StatusRow describes whether a ClickHouse migration has been applied.
type StatusRow struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

// ValidationIssue describes a pending or modified ClickHouse migration.
type ValidationIssue struct {
	Version string
	Name    string
	Kind    string
}

type appliedMigration struct {
	Checksum  string
	AppliedAt time.Time
}

// Up applies all pending ClickHouse migrations in version order.
//
// ClickHouse DDL is not transactional. Migration SQL must therefore be
// idempotent so an interrupted migration can be safely retried.
func Up(ctx context.Context, db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if existing, ok := applied[migration.Version]; ok {
			if existing.Checksum != migration.Checksum {
				return fmt.Errorf("migration %s checksum mismatch for %s", migration.Version, migration.Path)
			}
			continue
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Path, err)
		}
	}

	return nil
}

// Status returns every embedded migration and its applied state.
func Status(ctx context.Context, db *sql.DB) ([]StatusRow, error) {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return nil, err
	}
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return nil, err
	}

	rows := make([]StatusRow, 0, len(migrations))
	for _, migration := range migrations {
		row := StatusRow{Version: migration.Version, Name: migration.Name}
		if existing, ok := applied[migration.Version]; ok {
			appliedAt := existing.AppliedAt
			row.Applied = true
			row.AppliedAt = &appliedAt
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// Validate reports pending migrations and checksum mismatches without changing
// event tables.
func Validate(ctx context.Context, db *sql.DB) ([]ValidationIssue, error) {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return nil, err
	}
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return nil, err
	}

	var issues []ValidationIssue
	for _, migration := range migrations {
		existing, ok := applied[migration.Version]
		if !ok {
			issues = append(issues, ValidationIssue{
				Version: migration.Version,
				Name:    migration.Name,
				Kind:    "pending",
			})
			continue
		}
		if existing.Checksum != migration.Checksum {
			issues = append(issues, ValidationIssue{
				Version: migration.Version,
				Name:    migration.Name,
				Kind:    "checksum_mismatch",
			})
		}
	}
	return issues, nil
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("read embedded ClickHouse migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, name, ok := parseMigrationName(entry.Name())
		if !ok {
			return nil, fmt.Errorf("invalid ClickHouse migration filename %q", entry.Name())
		}
		path := filepath.Join("sql", entry.Name())
		contents, err := fs.ReadFile(migrationFiles, path)
		if err != nil {
			return nil, fmt.Errorf("read ClickHouse migration %s: %w", path, err)
		}
		sum := sha256.Sum256(contents)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			Path:     path,
			SQL:      string(contents),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no ClickHouse migrations found")
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	for index := 1; index < len(migrations); index++ {
		if migrations[index-1].Version == migrations[index].Version {
			return nil, fmt.Errorf("duplicate ClickHouse migration version %s", migrations[index].Version)
		}
	}
	return migrations, nil
}

func parseMigrationName(filename string) (version string, name string, ok bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	separator := strings.IndexByte(base, '_')
	if separator <= 0 || separator == len(base)-1 {
		return "", "", false
	}
	version = base[:separator]
	for _, character := range version {
		if character < '0' || character > '9' {
			return "", "", false
		}
	}
	return version, base[separator+1:], true
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations
		(
			version String,
			name String,
			checksum FixedString(64),
			applied_at DateTime64(6, 'UTC')
		)
		ENGINE = ReplacingMergeTree(applied_at)
		ORDER BY version
	`)
	if err != nil {
		return fmt.Errorf("create ClickHouse schema_migrations table: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			version,
			argMax(checksum, applied_at),
			max(applied_at)
		FROM schema_migrations
		GROUP BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("query applied ClickHouse migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]appliedMigration)
	for rows.Next() {
		var (
			version   string
			checksum  string
			appliedAt time.Time
		)
		if err := rows.Scan(&version, &checksum, &appliedAt); err != nil {
			return nil, fmt.Errorf("scan applied ClickHouse migration: %w", err)
		}
		applied[version] = appliedMigration{Checksum: checksum, AppliedAt: appliedAt}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied ClickHouse migrations: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration Migration) error {
	rendered, err := renderMigrationSQL(migration.SQL)
	if err != nil {
		return fmt.Errorf("render SQL: %w", err)
	}
	statements, err := splitStatements(rendered)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("execute SQL: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, checksum, applied_at)
		VALUES (?, ?, ?, now64(6))
	`, migration.Version, migration.Name, migration.Checksum); err != nil {
		return fmt.Errorf("record ClickHouse schema migration: %w", err)
	}
	return nil
}

func splitStatements(contents string) ([]string, error) {
	var (
		statements   []string
		start        int
		quote        byte
		lineComment  bool
		blockComment bool
	)
	for index := 0; index < len(contents); index++ {
		current := contents[index]
		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && index+1 < len(contents) && contents[index+1] == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if quote != 0 {
			if current == '\\' {
				index++
				continue
			}
			if current == quote {
				if index+1 < len(contents) && contents[index+1] == quote {
					index++
					continue
				}
				quote = 0
			}
			continue
		}

		switch {
		case current == '-' && index+1 < len(contents) && contents[index+1] == '-':
			lineComment = true
			index++
		case current == '/' && index+1 < len(contents) && contents[index+1] == '*':
			blockComment = true
			index++
		case current == '\'' || current == '"' || current == '`':
			quote = current
		case current == ';':
			statement := strings.TrimSpace(contents[start:index])
			if statement != "" {
				statements = append(statements, statement)
			}
			start = index + 1
		}
	}
	if quote != 0 || blockComment {
		return nil, fmt.Errorf("unterminated SQL quote or comment")
	}
	if statement := strings.TrimSpace(contents[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}
