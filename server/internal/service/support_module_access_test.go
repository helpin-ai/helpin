package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func ensureSupportModuleGrantsTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS workspace_module_grants (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		module TEXT NOT NULL,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		access_level TEXT NOT NULL DEFAULT 'member',
		created_by_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
}

func seedSupportModuleGrant(t *testing.T, db *gorm.DB, id, workspaceID string, subjectType model.ModuleGrantSubjectType, subjectID string) {
	t.Helper()
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, model.ModuleSupport, subjectType, subjectID, model.ModuleGrantAccessLevelMember, now, now)
}
