package crmsituationtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// PlaybookActionStorage applies proposal authority only to the isolated fixture database.
func PlaybookActionStorage(t testing.TB, db *gorm.DB) {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(current), "../dbmigrate/sql/202609080002_crm_playbook_action_intents.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statement := string(data)
	if db.Dialector.Name() == "sqlite" {
		statement = strings.ReplaceAll(statement, "timestamptz", "datetime")
	}
	Exec(t, db, statement)
	Exec(t, db, statement)
	for _, statement := range []string{
		`ALTER TABLE crm_contacts ADD COLUMN email text`,
		`ALTER TABLE crm_contacts ADD COLUMN email_status text NOT NULL DEFAULT 'valid'`,
		`CREATE TABLE crm_email_messages (id uuid PRIMARY KEY, workspace_id uuid, contact_id uuid, direction text, sent_at timestamp)`,
		`CREATE TABLE crm_email_message_contacts (message_id uuid, contact_id uuid)`,
	} {
		Exec(t, db, statement)
	}
	Exec(t, db, "UPDATE crm_contacts SET email = 'customer@example.test' WHERE id = ?", Contact)
}
