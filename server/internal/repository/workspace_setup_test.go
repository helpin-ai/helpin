package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkspaceSetupInvitationsExcludeExpiredRevokedAndOtherWorkspaces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`CREATE TABLE workspace_invitations (workspace_id TEXT,status TEXT,expires_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		workspace, status string
		expiry            time.Time
	}{
		{"ws-1", "pending", time.Now().Add(time.Hour)},
		{"ws-1", "pending", time.Now().Add(-time.Hour)},
		{"ws-1", "revoked", time.Now().Add(time.Hour)},
		{"ws-1", "accepted", time.Now().Add(-time.Hour)},
		{"ws-2", "pending", time.Now().Add(time.Hour)},
	} {
		if err = db.Exec("INSERT INTO workspace_invitations VALUES (?,?,?)", row.workspace, row.status, row.expiry).Error; err != nil {
			t.Fatal(err)
		}
	}
	count, err := NewSetupRepository(db).UsableSetupInvitationCount(context.Background(), "ws-1")
	if err != nil || count != 2 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
}
