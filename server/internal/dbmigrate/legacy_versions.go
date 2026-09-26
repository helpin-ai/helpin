package dbmigrate

import (
	"context"
	"database/sql"
	"fmt"
)

// These native-runtime/CLI migrations shared version numbers with migrations
// already released on develop. Their SQL is unchanged; the new
// versions sort immediately after the original numbers to preserve dependencies.
// Only an exact checksum match can identify a pre-merge native-runtime row.
var legacyNativeVersions = map[string]string{
	"20260912000101": "202609120001",
	"20260912000201": "202609120002",
	"20260914000101": "202609140001",
	"20260914000201": "202609140002",
	"20260915000101": "202609150001",
}

type migrationVersionMove struct {
	from string
	to   Migration
}

func legacyVersionMoves(migrations []Migration, applied map[string]appliedMigration) ([]migrationVersionMove, error) {
	var moves []migrationVersionMove
	for _, migration := range migrations {
		oldVersion, known := legacyNativeVersions[migration.Version]
		// The local pre-release fix briefly occupied the original CRM version.
		// Its unchanged SQL now has its own version; preserve that applied row
		// just like the native-runtime rows, without accepting arbitrary hashes.
		if migration.Version == correctedPipelineVersion {
			oldVersion, known = originalPipelineVersion, true
		}
		if !known {
			continue
		}
		old, exists := applied[oldVersion]
		if !exists || old.Checksum != migration.Checksum {
			continue
		}
		if _, exists := applied[migration.Version]; exists {
			return nil, fmt.Errorf("cannot relocate migration %s to occupied version %s", oldVersion, migration.Version)
		}
		moves = append(moves, migrationVersionMove{from: oldVersion, to: migration})
	}
	return moves, nil
}

const originalPipelineVersion = "202609120001"
const correctedPipelineVersion = "20260912000102"

// Keep the released SQL/checksum available for historical validation, but never
// execute its integer-array comparison on a database where it is still pending.
// The corrected migration runs in the same position before later CRM changes.
// Status omits this unapplied historical entry, as do Pending and Validate.
func skipSupersededPipeline(migration Migration, applied map[string]appliedMigration) bool {
	_, exists := applied[migration.Version]
	return migration.Version == originalPipelineVersion && !exists
}

// Read-only commands present the same effective versions that Up will persist.
// In particular, an applied native migration must not hide a pending CRM one.
func loadResolvedAppliedMigrations(ctx context.Context, conn *sql.Conn, migrations []Migration) (map[string]appliedMigration, error) {
	applied, err := loadAppliedMigrations(ctx, conn)
	if err != nil {
		return nil, err
	}
	moves, err := legacyVersionMoves(migrations, applied)
	if err != nil {
		return nil, err
	}
	for _, move := range moves {
		applied[move.to.Version] = applied[move.from]
		delete(applied, move.from)
	}
	return applied, nil
}

// Called under the migrator advisory lock before applying SQL. Preserve the
// original checksum, name, and applied_at; never replay the native migration.
func relocateLegacyMigrationVersions(ctx context.Context, conn *sql.Conn, migrations []Migration) error {
	applied, err := loadAppliedMigrations(ctx, conn)
	if err != nil {
		return err
	}
	moves, err := legacyVersionMoves(migrations, applied)
	if err != nil || len(moves) == 0 {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration version relocation: %w", err)
	}
	defer tx.Rollback()
	for _, move := range moves {
		result, err := tx.ExecContext(ctx,
			`UPDATE schema_migrations SET version = $1 WHERE version = $2 AND checksum = $3`,
			move.to.Version, move.from, move.to.Checksum)
		if err != nil {
			return fmt.Errorf("relocate migration %s: %w", move.from, err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count relocated migration %s: %w", move.from, err)
		}
		if count != 1 {
			return fmt.Errorf("migration %s changed during version relocation", move.from)
		}
	}
	return tx.Commit()
}
