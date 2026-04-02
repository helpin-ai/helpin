package dbmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const advisoryLockID int64 = 2026040101

//go:embed sql/*.sql
var migrationFiles embed.FS

type Migration struct {
	Version  string
	Name     string
	Path     string
	SQL      string
	Checksum string
}

type StatusRow struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt *time.Time
}

func Up(ctx context.Context, db *sql.DB) error {
	return withLockedConn(ctx, db, func(conn *sql.Conn) error {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return err
		}

		migrations, err := loadMigrations()
		if err != nil {
			return err
		}

		applied, err := loadAppliedMigrations(ctx, conn)
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
			if err := applyMigration(ctx, conn, migration); err != nil {
				return fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Path, err)
			}
		}

		return nil
	})
}

func Status(ctx context.Context, db *sql.DB) ([]StatusRow, error) {
	return withLockedConnResult(ctx, db, func(conn *sql.Conn) ([]StatusRow, error) {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return nil, err
		}

		migrations, err := loadMigrations()
		if err != nil {
			return nil, err
		}

		applied, err := loadAppliedMigrations(ctx, conn)
		if err != nil {
			return nil, err
		}

		rows := make([]StatusRow, 0, len(migrations))
		for _, migration := range migrations {
			row := StatusRow{
				Version: migration.Version,
				Name:    migration.Name,
			}
			if existing, ok := applied[migration.Version]; ok {
				row.Applied = true
				row.AppliedAt = existing.AppliedAt
			}
			rows = append(rows, row)
		}

		return rows, nil
	})
}

// Head returns the latest applied migration, or nil if none have been applied.
func Head(ctx context.Context, db *sql.DB) (*StatusRow, error) {
	return withLockedConnResult(ctx, db, func(conn *sql.Conn) (*StatusRow, error) {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return nil, err
		}

		var version, name string
		var appliedAt time.Time
		err := conn.QueryRowContext(ctx,
			`SELECT version, name, applied_at FROM schema_migrations ORDER BY version DESC LIMIT 1`,
		).Scan(&version, &name, &appliedAt)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("query head migration: %w", err)
		}

		ts := appliedAt
		return &StatusRow{
			Version:   version,
			Name:      name,
			Applied:   true,
			AppliedAt: &ts,
		}, nil
	})
}

// Repair recalculates checksums for all applied migrations to match
// the current file contents. This fixes checksum mismatches caused by
// post-apply edits to migration files.
func Repair(ctx context.Context, db *sql.DB) (int, error) {
	return withLockedConnResult(ctx, db, func(conn *sql.Conn) (int, error) {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return 0, err
		}

		migrations, err := loadMigrations()
		if err != nil {
			return 0, err
		}

		applied, err := loadAppliedMigrations(ctx, conn)
		if err != nil {
			return 0, err
		}

		repaired := 0
		for _, migration := range migrations {
			existing, ok := applied[migration.Version]
			if !ok || existing.Checksum == migration.Checksum {
				continue
			}
			if _, err := conn.ExecContext(ctx,
				`UPDATE schema_migrations SET checksum = $1 WHERE version = $2`,
				migration.Checksum, migration.Version,
			); err != nil {
				return repaired, fmt.Errorf("update checksum for %s: %w", migration.Version, err)
			}
			repaired++
		}

		return repaired, nil
	})
}

// ValidationIssue describes a single problem found by Validate.
type ValidationIssue struct {
	Version string
	Name    string
	Kind    string // "checksum_mismatch" or "pending"
}

// Validate checks for checksum mismatches and pending migrations without
// modifying anything. Returns issues found; an empty slice means everything
// is clean.
func Validate(ctx context.Context, db *sql.DB) ([]ValidationIssue, error) {
	return withLockedConnResult(ctx, db, func(conn *sql.Conn) ([]ValidationIssue, error) {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return nil, err
		}

		migrations, err := loadMigrations()
		if err != nil {
			return nil, err
		}

		applied, err := loadAppliedMigrations(ctx, conn)
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
	})
}

// Pending returns only unapplied migrations.
func Pending(ctx context.Context, db *sql.DB) ([]StatusRow, error) {
	return withLockedConnResult(ctx, db, func(conn *sql.Conn) ([]StatusRow, error) {
		if err := ensureSchemaMigrationsTable(ctx, conn); err != nil {
			return nil, err
		}

		migrations, err := loadMigrations()
		if err != nil {
			return nil, err
		}

		applied, err := loadAppliedMigrations(ctx, conn)
		if err != nil {
			return nil, err
		}

		var pending []StatusRow
		for _, migration := range migrations {
			if _, ok := applied[migration.Version]; !ok {
				pending = append(pending, StatusRow{
					Version: migration.Version,
					Name:    migration.Name,
				})
			}
		}

		return pending, nil
	})
}

// Create scaffolds a new migration SQL file in the given directory using the
// naming convention YYYYMMDDNNNN_name.sql. It auto-increments the sequence
// number based on existing files for today's date. Returns the created file path.
func Create(dir string, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("migration name is required")
	}

	// Sanitize: lowercase, replace spaces/hyphens with underscores, strip invalid chars.
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if r == ' ' || r == '-' {
			return '_'
		}
		return -1
	}, name)
	if sanitized == "" {
		return "", fmt.Errorf("migration name %q contains no valid characters", name)
	}

	// Determine today's date prefix and next sequence number.
	datePrefix := time.Now().UTC().Format("20060102")
	seq := 1

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read migration directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, _, ok := parseMigrationName(entry.Name())
		if !ok || !strings.HasPrefix(version, datePrefix) {
			continue
		}
		// Extract sequence from version (last 4 digits).
		seqStr := version[len(datePrefix):]
		var n int
		if _, err := fmt.Sscanf(seqStr, "%d", &n); err == nil && n >= seq {
			seq = n + 1
		}
	}

	version := fmt.Sprintf("%s%04d", datePrefix, seq)
	filename := fmt.Sprintf("%s_%s.sql", version, sanitized)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf("-- Migration: %s\n", sanitized)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write migration file: %w", err)
	}

	return path, nil
}

type appliedMigration struct {
	Checksum  string
	AppliedAt *time.Time
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		version, name, ok := parseMigrationName(entry.Name())
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}

		path := filepath.Join("sql", entry.Name())
		contents, err := fs.ReadFile(migrationFiles, path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", path, err)
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

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].Version == migrations[i].Version {
			return nil, fmt.Errorf("duplicate migration version %s", migrations[i].Version)
		}
	}

	return migrations, nil
}

func parseMigrationName(filename string) (version string, name string, ok bool) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	idx := strings.IndexByte(base, '_')
	if idx <= 0 || idx == len(base)-1 {
		return "", "", false
	}

	version = base[:idx]
	for _, r := range version {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}

	return version, base[idx+1:], true
}

func ensureSchemaMigrationsTable(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, conn *sql.Conn) (map[string]appliedMigration, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT version, checksum, applied_at
		FROM schema_migrations
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]appliedMigration{}
	for rows.Next() {
		var (
			version   string
			checksum  string
			appliedAt time.Time
		)
		if err := rows.Scan(&version, &checksum, &appliedAt); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		ts := appliedAt
		applied[version] = appliedMigration{
			Checksum:  checksum,
			AppliedAt: &ts,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, migration Migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if strings.TrimSpace(migration.SQL) != "" {
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("execute sql: %w", err)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
		migration.Version,
		migration.Name,
		migration.Checksum,
	); err != nil {
		return fmt.Errorf("record schema migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func withLockedConn(ctx context.Context, db *sql.DB, fn func(conn *sql.Conn) error) error {
	_, err := withLockedConnResult(ctx, db, func(conn *sql.Conn) (struct{}, error) {
		return struct{}{}, fn(conn)
	})
	return err
}

func withLockedConnResult[T any](ctx context.Context, db *sql.DB, fn func(conn *sql.Conn) (T, error)) (T, error) {
	var zero T

	conn, err := db.Conn(ctx)
	if err != nil {
		return zero, fmt.Errorf("open db connection: %w", err)
	}
	defer conn.Close()

	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, advisoryLockID).Scan(&locked); err != nil {
		return zero, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !locked {
		return zero, fmt.Errorf("another migration process is already running")
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, advisoryLockID)

	return fn(conn)
}
