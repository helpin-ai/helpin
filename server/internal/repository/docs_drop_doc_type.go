package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateDropDocType removes the doc_type column from docs_documents.
// External publishability is now determined by the space type.
func MigrateDropDocType(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='docs_documents' AND column_name='doc_type') THEN
        ALTER TABLE docs_documents DROP COLUMN doc_type;
    END IF;
END
$$;
DROP INDEX IF EXISTS idx_docs_doc_ws_type_status;
`
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate drop doc_type: %w", err)
	}
	return nil
}
