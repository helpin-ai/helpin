package dbmigrate

import (
	"strings"
	"testing"
)

func TestCRMOutreachProductionMigration(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migrations {
		if m.Version != "202609140001" {
			continue
		}
		if m.NoTransaction() {
			t.Fatal("outreach schema must migrate atomically")
		}
		sql := strings.ToLower(m.SQL)
		for _, table := range []string{"crm_email_templates", "crm_email_sequences", "crm_sequence_enrollments", "crm_sequence_deliveries", "crm_email_suppressions"} {
			if !strings.Contains(sql, "create table if not exists "+table) {
				t.Errorf("missing production table %s", table)
			}
		}
		for _, clause := range []string{"add column if not exists signature", "crm_sequence_recipient", "crm_sequence_delivery_step", "unsubscribe_token"} {
			if !strings.Contains(sql, clause) {
				t.Errorf("missing schema requirement %s", clause)
			}
		}
		return
	}
	t.Fatal("CRM outreach has no embedded production migration")
}
