package dbmigrate

import (
	"strings"
	"testing"
)

func TestCRMPlaybookMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202609070001" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("Playbook migration must be atomic")
		}
		sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
		for _, clause := range []string{
			"create table if not exists crm_playbooks", "create table if not exists crm_playbook_versions", "create table if not exists crm_playbook_changes",
			"unique (workspace_id, creation_key)", "unique (workspace_id, playbook_id, version)", "unique (workspace_id, playbook_id, command_key)",
			"foreign key (workspace_id, playbook_id, playbook_version_id)", "references crm_playbook_versions(workspace_id, playbook_id, id)",
			"accepting_customers boolean not null default false", "add column if not exists playbook_milestones jsonb",
		} {
			if !strings.Contains(sql, clause) {
				t.Errorf("missing migration contract: %s", clause)
			}
		}
		for _, forbidden := range []string{"insert into", "update crm_", "delete from", "create table if not exists crm_enrollment", "alter table agents", "alter table automation_rules"} {
			if strings.Contains(sql, forbidden) {
				t.Errorf("migration mutates unrelated work: %s", forbidden)
			}
		}
		return
	}
	t.Fatal("Playbook migration is not embedded")
}
