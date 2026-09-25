package repository

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateSupportTranslationConversationDefaults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:support_translation_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSupportTranslationConversationDefaults(db); err != nil {
		t.Fatalf("missing table: %v", err)
	}
	if err := db.Exec("CREATE TABLE support_translation_conversations (id INTEGER PRIMARY KEY, customer_language TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO support_translation_conversations (id, customer_language) VALUES (1, NULL), (2, 'fr')").Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSupportTranslationConversationDefaults(db); err != nil {
		t.Fatalf("legacy table without revision: %v", err)
	}
	if err := db.Exec("ALTER TABLE support_translation_conversations ADD COLUMN revision INTEGER").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE support_translation_conversations SET revision = 4 WHERE id = 2").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateSupportTranslationConversationDefaults(db); err != nil {
			t.Fatalf("backfill pass %d: %v", i, err)
		}
	}
	var rows []struct {
		ID               int
		CustomerLanguage string
		Revision         int64
	}
	if err := db.Raw("SELECT id, customer_language, revision FROM support_translation_conversations ORDER BY id").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].CustomerLanguage != "" || rows[0].Revision != 1 || rows[1].CustomerLanguage != "fr" || rows[1].Revision != 4 {
		t.Fatalf("unexpected policies after backfill: %+v", rows)
	}
}

func TestMigrateSupportTranslationPolicyRevision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:support_translation_policy_revision?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateSupportTranslationPolicyRevision(db); err != nil {
		t.Fatalf("missing table: %v", err)
	}
	if err := db.Exec("CREATE TABLE support_translations (id INTEGER PRIMARY KEY, policy_revision INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO support_translations (id, policy_revision) VALUES (1, NULL), (2, 7)").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateSupportTranslationPolicyRevision(db); err != nil {
			t.Fatalf("backfill pass %d: %v", i, err)
		}
	}
	var revisions []int64
	if err := db.Raw("SELECT policy_revision FROM support_translations ORDER BY id").Scan(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0] != 0 || revisions[1] != 7 {
		t.Fatalf("unexpected revisions after backfill: %v", revisions)
	}
}
