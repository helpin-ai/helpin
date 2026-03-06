package repository

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// MigrateWorkspacesToOrganizations creates a default organization for each workspace
// that does not yet have an organization_id. This is a one-time idempotent migration.
func MigrateWorkspacesToOrganizations(db *gorm.DB) error {
	// Find workspaces without an organization.
	type ws struct {
		ID      string
		Name    string
		Slug    string
		OwnerID string
	}
	var orphans []ws
	if err := db.Raw(`SELECT id, name, slug, owner_id FROM workspaces WHERE organization_id IS NULL`).Scan(&orphans).Error; err != nil {
		return fmt.Errorf("find orphan workspaces: %w", err)
	}
	if len(orphans) == 0 {
		return nil
	}

	log.Printf("migrating %d workspace(s) to organizations...", len(orphans))

	for _, w := range orphans {
		if err := migrateOneWorkspace(db, w); err != nil {
			return err
		}
		log.Printf("  workspace %q -> organization (slug: %s)", w.Name, w.Slug)
	}

	log.Println("organization migration complete")
	return nil
}

func migrateOneWorkspace(db *gorm.DB, w struct {
	ID      string
	Name    string
	Slug    string
	OwnerID string
}) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Find a unique slug for the organization.
		orgSlug := w.Slug
		for suffix := 1; ; suffix++ {
			var count int64
			if err := tx.Raw(`SELECT COUNT(*) FROM organizations WHERE slug = ?`, orgSlug).Scan(&count).Error; err != nil {
				return fmt.Errorf("check slug uniqueness for workspace %s: %w", w.ID, err)
			}
			if count == 0 {
				break
			}
			orgSlug = fmt.Sprintf("%s-%d", w.Slug, suffix)
		}

		// Create the organization.
		var orgID string
		if err := tx.Raw(
			`INSERT INTO organizations (name, slug, owner_id) VALUES (?, ?, ?) RETURNING id`,
			w.Name, orgSlug, w.OwnerID,
		).Scan(&orgID).Error; err != nil {
			return fmt.Errorf("create org for workspace %s: %w", w.ID, err)
		}

		// Add all workspace members as org members (preserving owner role).
		if err := tx.Exec(
			`INSERT INTO organization_members (organization_id, user_id, role)
			 SELECT ?, user_id, CASE WHEN role = 'owner' THEN 'owner' ELSE 'member' END
			 FROM workspace_members WHERE workspace_id = ?
			 ON CONFLICT (organization_id, user_id) DO NOTHING`,
			orgID, w.ID,
		).Error; err != nil {
			return fmt.Errorf("add org members for workspace %s: %w", w.ID, err)
		}

		// Link the workspace to the organization.
		if err := tx.Exec(
			`UPDATE workspaces SET organization_id = ? WHERE id = ?`,
			orgID, w.ID,
		).Error; err != nil {
			return fmt.Errorf("link workspace %s to org: %w", w.ID, err)
		}

		return nil
	})
}
