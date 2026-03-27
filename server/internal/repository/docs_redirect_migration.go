package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MigrateDocsRedirectPaths normalizes malformed redirect paths and slug parts
// created by older slug-change code.
func MigrateDocsRedirectPaths(db *gorm.DB) error {
	var workspaceIDs []string
	if err := db.Model(&model.DocsRedirect{}).Distinct().Pluck("workspace_id", &workspaceIDs).Error; err != nil {
		return fmt.Errorf("list docs redirect workspaces: %w", err)
	}

	repo := NewDocsRedirectRepository(db)
	ctx := context.Background()
	for _, workspaceID := range workspaceIDs {
		if err := repo.repairMalformedPaths(ctx, workspaceID); err != nil {
			return err
		}
	}

	return nil
}
