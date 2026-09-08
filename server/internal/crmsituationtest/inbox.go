package crmsituationtest

import (
	"testing"

	"gorm.io/gorm"
)

// InboxTables installs the pipeline lookup used by read-only inbox classification.
func InboxTables(t testing.TB, db *gorm.DB) {
	t.Helper()
	Exec(t, db, `CREATE TABLE IF NOT EXISTS crm_pipelines (id uuid PRIMARY KEY, workspace_id uuid, default_commercial_motion text)`)
}
