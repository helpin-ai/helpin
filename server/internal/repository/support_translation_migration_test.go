package repository

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateSupportTranslationCustomerLanguage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:support_translation_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSupportTranslationCustomerLanguage(db); err != nil {
		t.Fatalf("missing table: %v", err)
	}
	if err := db.Exec("CREATE TABLE support_translation_conversations (id INTEGER PRIMARY KEY, customer_language TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO support_translation_conversations (id, customer_language) VALUES (1, NULL), (2, 'fr')").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateSupportTranslationCustomerLanguage(db); err != nil {
			t.Fatalf("backfill pass %d: %v", i, err)
		}
	}
	var rows []struct {
		ID               int
		CustomerLanguage string
	}
	if err := db.Raw("SELECT id, customer_language FROM support_translation_conversations ORDER BY id").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].CustomerLanguage != "" || rows[1].CustomerLanguage != "fr" {
		t.Fatalf("unexpected languages after backfill: %+v", rows)
	}
}
