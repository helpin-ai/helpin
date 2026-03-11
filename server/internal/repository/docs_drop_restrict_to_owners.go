package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateDropRestrictToOwners removes the restrict_to_owners column from docs_spaces.
// The flag was never enforced on the backend.
func MigrateDropRestrictToOwners(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='docs_spaces' AND column_name='restrict_to_owners') THEN
        ALTER TABLE docs_spaces DROP COLUMN restrict_to_owners;
    END IF;
END
$$;
`
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate drop restrict_to_owners: %w", err)
	}
	return nil
}
