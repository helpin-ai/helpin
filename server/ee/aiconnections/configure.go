//go:build ee

package aiconnections

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/ee/pricing"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConfigureOptions struct {
	WorkspaceID string
	Enabled     bool
	Tariff      *aiusage.FlatTokenTariff
	Apply       bool
}

type ConfigureResult struct {
	Applied     bool                     `json:"applied"`
	WorkspaceID string                   `json:"workspace_id"`
	Enabled     bool                     `json:"byok_enabled"`
	Tariff      *aiusage.FlatTokenTariff `json:"tariff,omitempty"`
}

// Configure is an operator-only transaction. Rates are immutable across all
// workspaces; accepted executions are never updated. Preview rolls back writes.
func Configure(ctx context.Context, db *gorm.DB, options ConfigureOptions) (*ConfigureResult, error) {
	if strings.TrimSpace(options.WorkspaceID) == "" {
		return nil, errors.New("workspace is required")
	}
	if options.Enabled {
		if err := pricing.ValidateFlatTokenTariff(options.Tariff); err != nil {
			return nil, err
		}
		if len(options.Tariff.Version) > 128 || strings.TrimSpace(options.Tariff.Version) != options.Tariff.Version {
			return nil, errors.New("tariff version must be at most 128 characters without surrounding whitespace")
		}
	} else if options.Tariff != nil {
		return nil, errors.New("disable does not change the tariff")
	}
	preview := errors.New("rollback preview")
	result := &ConfigureResult{Applied: options.Apply, WorkspaceID: options.WorkspaceID, Enabled: options.Enabled}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize configuration for this workspace, including its first row.
		var workspace struct{ ID string }
		if err := tx.Table("workspaces").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", options.WorkspaceID).Take(&workspace).Error; err != nil {
			return fmt.Errorf("load workspace: %w", err)
		}
		settings := WorkspaceSettings{WorkspaceID: options.WorkspaceID}
		if err := tx.Where("workspace_id = ?", options.WorkspaceID).Take(&settings).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		settings.BYOKEnabled = options.Enabled
		if options.Enabled {
			tariff := Tariff{Version: options.Tariff.Version, Currency: options.Tariff.Currency, MicrousdPerMillion: *options.Tariff.MicrousdPerMillion, AccountingVersion: options.Tariff.AccountingVersion}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tariff).Error; err != nil {
				return err
			}
			var existing Tariff
			if err := tx.Where("version = ?", tariff.Version).Take(&existing).Error; err != nil {
				return err
			}
			if existing != tariff {
				return errors.New("tariff version already exists with different values; choose a new version")
			}
			settings.TariffVersion = &tariff.Version
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}}, DoUpdates: clause.AssignmentColumns([]string{"byok_enabled", "tariff_version"})}).Create(&settings).Error; err != nil {
			return err
		}
		if settings.TariffVersion != nil {
			var tariff Tariff
			if err := tx.Where("version = ?", *settings.TariffVersion).Take(&tariff).Error; err != nil {
				return err
			}
			result.Tariff = &aiusage.FlatTokenTariff{Version: tariff.Version, Currency: tariff.Currency, MicrousdPerMillion: &tariff.MicrousdPerMillion, AccountingVersion: tariff.AccountingVersion}
		}
		if !options.Apply {
			return preview
		}
		return nil
	})
	if err != nil && !errors.Is(err, preview) {
		return nil, err
	}
	return result, nil
}
