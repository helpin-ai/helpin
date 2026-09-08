package dbmigrate

import (
	"strings"
	"testing"
)

func TestCRMSituationFoundationMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var migration *Migration
	for i := range migrations {
		if migrations[i].Version == "202609050002" {
			migration = &migrations[i]
			break
		}
	}
	if migration == nil {
		t.Fatal("customer-work foundation is missing from embedded migrations")
	}
	if migration.NoTransaction() {
		t.Fatal("foundation must be applied atomically")
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
	for _, clause := range []string{
		"create table if not exists crm_situations",
		"create table if not exists crm_situation_references",
		"on crm_situations(workspace_id, creation_key)",
		"foreign key (workspace_id, situation_id) references crm_situations(workspace_id, id) on delete cascade",
		"primary key (workspace_id, situation_id, kind, source_id)",
		"lifecycle = 'closed' and outcome_kind is not null",
		"priority double precision",
	} {
		if !strings.Contains(normalized, clause) {
			t.Errorf("migration missing %q", clause)
		}
	}
	for _, forbidden := range []string{"insert into", "update crm_", "delete from", "alter table", "drop table"} {
		if strings.Contains(normalized, forbidden) {
			t.Errorf("additive foundation unexpectedly contains %q", forbidden)
		}
	}
}

func TestCRMSituationSourceMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202609060001" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("source migration must be atomic")
		}
		sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
		for _, clause := range []string{"create table if not exists crm_situation_source_links", "primary key (workspace_id, kind, source_id)", "crm_situation_origin_contract", "origin_kind = 'suggestion' and commercial_motion = 'needs_context'"} {
			if !strings.Contains(sql, clause) {
				t.Errorf("source migration missing %q", clause)
			}
		}
		for _, clause := range []string{"insert into", "update crm_suggestions", "update crm_signals", "delete from"} {
			if strings.Contains(sql, clause) {
				t.Errorf("source migration replays or rewrites source data: %q", clause)
			}
		}
		return
	}
	t.Fatal("source migration is not embedded")
}

func TestCRMSituationLifecycleMigrationContract(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if migration.Version != "202609060002" {
			continue
		}
		if migration.NoTransaction() {
			t.Fatal("lifecycle migration must be atomic")
		}
		sql := strings.ToLower(strings.Join(strings.Fields(migration.SQL), " "))
		for _, clause := range []string{
			"add column if not exists revision bigint not null default 1 check (revision > 0)",
			"create table if not exists crm_situation_changes", "unique (workspace_id, situation_id, revision)",
			"unique (workspace_id, situation_id, command_key)", "actor_kind = 'member' and actor_member_id is not null",
			"references crm_situations(workspace_id, id) on delete cascade",
		} {
			if !strings.Contains(sql, clause) {
				t.Errorf("lifecycle migration missing %q", clause)
			}
		}
		for _, clause := range []string{"insert into", "update crm_", "delete from", "drop table"} {
			if strings.Contains(sql, clause) {
				t.Errorf("lifecycle migration rewrites historical work: %q", clause)
			}
		}
		return
	}
	t.Fatal("lifecycle migration is not embedded")
}
