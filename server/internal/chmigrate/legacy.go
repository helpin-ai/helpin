package chmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	eventsTable       = "events"
	eventsNextTable   = "events_next"
	eventsBackupTable = "events_compact_migration"
)

func reconcileLegacyEventsTable(ctx context.Context, db *sql.DB, schemaSQL string) (string, error) {
	exists, err := tableExists(ctx, db, eventsTable)
	if err != nil || !exists {
		return "", err
	}
	current, err := isCurrentEventsTable(ctx, db, eventsTable)
	if err != nil {
		return "", err
	}
	backupExists, err := tableExists(ctx, db, eventsBackupTable)
	if err != nil {
		return "", err
	}
	if current {
		if backupExists {
			return eventsBackupTable, nil
		}
		return "", nil
	}
	if backupExists {
		return "", fmt.Errorf("legacy ClickHouse backup exists but events schema is not current")
	}

	createNext, err := replacementTableStatement(schemaSQL)
	if err != nil {
		return "", err
	}
	if err := prepareNextTable(ctx, db, createNext); err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS usermaven.events_queue_mv"); err != nil {
		return "", fmt.Errorf("pause ClickHouse event ingestion: %w", err)
	}

	sourceCount, err := tableCount(ctx, db, eventsTable)
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO usermaven.events_next (raw_event, _partition, _offset, _kafka_timestamp_ms)
		SELECT raw_event, _partition, _offset, toNullable(_timestamp)
		FROM usermaven.events
	`); err != nil {
		return "", fmt.Errorf("copy legacy ClickHouse events: %w", err)
	}
	nextCount, err := tableCount(ctx, db, eventsNextTable)
	if err != nil {
		return "", err
	}
	if sourceCount != nextCount {
		return "", fmt.Errorf(
			"legacy ClickHouse event count mismatch: source=%d replacement=%d",
			sourceCount,
			nextCount,
		)
	}

	if _, err := db.ExecContext(ctx, `
		RENAME TABLE
			usermaven.events TO usermaven.events_compact_migration,
			usermaven.events_next TO usermaven.events
	`); err != nil {
		return "", fmt.Errorf("swap legacy ClickHouse events table: %w", err)
	}
	return eventsBackupTable, nil
}

func prepareNextTable(ctx context.Context, db *sql.DB, createNext string) error {
	exists, err := tableExists(ctx, db, eventsNextTable)
	if err != nil {
		return err
	}
	if exists {
		if _, err := db.ExecContext(ctx, "DROP TABLE usermaven.events_next"); err != nil {
			return fmt.Errorf("remove incomplete ClickHouse replacement table: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, createNext); err != nil {
		return fmt.Errorf("create ClickHouse replacement events table: %w", err)
	}
	return nil
}

func replacementTableStatement(schemaSQL string) (string, error) {
	statements, err := splitStatements(schemaSQL)
	if err != nil {
		return "", err
	}
	const tableName = "usermaven.events"
	for _, statement := range statements {
		if strings.Contains(statement, "CREATE TABLE IF NOT EXISTS "+tableName) {
			return strings.Replace(statement, tableName, "usermaven.events_next", 1), nil
		}
	}
	return "", fmt.Errorf("events table definition missing from baseline ClickHouse migration")
}

func removeVerifiedLegacyBackup(ctx context.Context, db *sql.DB, backup string) error {
	currentCount, err := tableCount(ctx, db, eventsTable)
	if err != nil {
		return err
	}
	backupCount, err := tableCount(ctx, db, backup)
	if err != nil {
		return err
	}
	if currentCount != backupCount {
		return fmt.Errorf(
			"ClickHouse migration backup mismatch: current=%d backup=%d",
			currentCount,
			backupCount,
		)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE usermaven."+backup); err != nil {
		return fmt.Errorf("remove verified ClickHouse migration backup: %w", err)
	}
	return nil
}

func isCurrentEventsTable(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var current bool
	err := db.QueryRowContext(ctx, `
		SELECT
			countIf(name = 'company_name') > 0
			AND countIf(name = 'parsed_referer_channel') > 0
			AND countIf(name = 'parsed_ua_bot' AND type LIKE 'Enum8%') > 0
		FROM system.columns
		WHERE database = 'usermaven' AND table = ?
	`, table).Scan(&current)
	if err != nil {
		return false, fmt.Errorf("inspect ClickHouse events schema: %w", err)
	}
	return current, nil
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count uint64
	err := db.QueryRowContext(ctx, `
		SELECT count()
		FROM system.tables
		WHERE database = 'usermaven' AND name = ?
	`, table).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check ClickHouse table %s: %w", table, err)
	}
	return count > 0, nil
}

func tableCount(ctx context.Context, db *sql.DB, table string) (uint64, error) {
	var count uint64
	switch table {
	case eventsTable, eventsNextTable, eventsBackupTable:
	default:
		return 0, fmt.Errorf("invalid ClickHouse table name %q", table)
	}
	if err := db.QueryRowContext(ctx, "SELECT count() FROM usermaven."+table).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ClickHouse table %s: %w", table, err)
	}
	return count, nil
}
