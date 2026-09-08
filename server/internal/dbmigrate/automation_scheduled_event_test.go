package dbmigrate

import (
	"strings"
	"testing"
)

func TestAutomationScheduledEventMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202609060003" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("outbox migration must be atomic")
		}
		sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
		for _, clause := range []string{
			"create table if not exists automation_scheduled_events", "unique (workspace_id, event_key)",
			"check (attempts <= max_attempts)", "lease_token is not null and lease_until is not null",
			"references workspaces(id) on delete cascade", "expected_revision bigint not null check (expected_revision > 0)",
			"where lifecycle = 'open' and next_checkpoint_at is not null", "on conflict (workspace_id, event_key) do nothing",
		} {
			if !strings.Contains(sql, clause) {
				t.Errorf("missing contract: %s", clause)
			}
		}
		for _, clause := range []string{"update crm_", "delete from", "alter table", "drop table", "insert into agent", "insert into automation_rules", "insert into pm_"} {
			if strings.Contains(sql, clause) {
				t.Errorf("unexpected existing workflow mutation: %s", clause)
			}
		}
		return
	}
	t.Fatal("scheduled event migration is not embedded")
}
