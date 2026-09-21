package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestTaskActivityReadableLabels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:activity_labels_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE pm_activity_log (id text PRIMARY KEY, workspace_id text, entity_type text, entity_id text, actor_id text, event_type text, action text, field_name text, old_value text, new_value text, metadata text, created_at datetime)`,
		`CREATE TABLE pm_labels (id text, workspace_id text, name text)`,
		`CREATE TABLE workspace_members (id text, user_id text, workspace_id text, display_name text, email text)`,
		`INSERT INTO pm_labels VALUES ('label-1','ws','Bug'),('foreign-label','other','Secret')`,
		`INSERT INTO workspace_members VALUES ('member-1','user-1','ws','Alex','alex@example.com')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPMActivityRepository(db)
	field, value := "label", "label-1"
	entry := model.PMActivityLog{ID: "new", WorkspaceID: "ws", EntityType: "task", EntityID: "task", Action: "label_added", FieldName: &field, NewValue: &value}
	if err := repo.Create(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE pm_labels SET name='Renamed' WHERE id='label-1'`).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, field, value string }{{"legacy", "label", "label-1"}, {"owner", "owner", "member-1"}, {"follower", "follower", "user-1"}, {"foreign", "label", "foreign-label"}} {
		if err := db.Exec(`INSERT INTO pm_activity_log (id, workspace_id, entity_type, entity_id, action, field_name, new_value, metadata) VALUES (?, 'ws', 'task', 'task', 'updated', ?, ?, CAST('{}' AS BLOB))`, row.id, row.field, row.value).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := repo.List(context.Background(), "task", "task", model.PMPagination{})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"new": "Bug", "legacy": "Renamed", "owner": "Alex", "follower": "Alex", "foreign": ""}
	for _, row := range rows {
		meta := map[string]any{}
		_ = json.Unmarshal(row.Activity.Metadata, &meta)
		actual, _ := meta["new_label"].(string)
		if actual != expected[row.Activity.ID] {
			t.Errorf("%s label=%q want %q", row.Activity.ID, actual, expected[row.Activity.ID])
		}
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 activities, got %d", len(rows))
	}
}
