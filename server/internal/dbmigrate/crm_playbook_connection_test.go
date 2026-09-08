package dbmigrate

import (
	"strings"
	"testing"
)

func TestCRMPlaybookConnectionMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202609070002" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("connection migration must be atomic")
		}
		sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
		for _, clause := range []string{"create table if not exists crm_playbook_connections", "unique (workspace_id, playbook_id, command_key)", "unique (workspace_id, playbook_id, version)", "foreign key (workspace_id, playbook_id, playbook_version_id)", "check (execution_enabled = false)"} {
			if !strings.Contains(sql, clause) {
				t.Errorf("missing contract: %s", clause)
			}
		}
		for _, forbidden := range []string{"insert into", "update ", "delete from", "alter table agents", "alter table automation_rules", "alter table crm_situations"} {
			if strings.Contains(sql, forbidden) {
				t.Errorf("migration mutates existing work: %s", forbidden)
			}
		}
		return
	}
	t.Fatal("connection migration is not embedded")
}
