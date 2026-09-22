package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CapabilityRepository reads the evidence behind capability statuses and
// stores explicit instance capability checks.
type CapabilityRepository struct {
	db *gorm.DB
}

// NewCapabilityRepository creates a CapabilityRepository.
func NewCapabilityRepository(db *gorm.DB) *CapabilityRepository {
	return &CapabilityRepository{db: db}
}

// AIConnectionSummary counts shared workspace AI connections across the instance.
type AIConnectionSummary struct {
	Connected int64
	Verified  int64
}

// WorkspaceOrganizationID returns the workspace's organization, or "" when it has none.
func (r *CapabilityRepository) WorkspaceOrganizationID(ctx context.Context, workspaceID string) (string, error) {
	var orgID *string
	if err := r.db.WithContext(ctx).Table("workspaces").Select("organization_id").
		Where("id = ?", workspaceID).Limit(1).Scan(&orgID).Error; err != nil {
		return "", fmt.Errorf("read workspace organization: %w", err)
	}
	if orgID == nil {
		return "", nil
	}
	return *orgID, nil
}

// ActiveWidgetInstallationCount counts the workspace's active widget installations.
func (r *CapabilityRepository) ActiveWidgetInstallationCount(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("support_widget_installations").
		Where("workspace_id = ? AND active = true", workspaceID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count widget installations: %w", err)
	}
	return count, nil
}

// HasEmbeddedChunks reports whether knowledge has been embedded. An empty
// workspaceID checks the whole instance. docs_chunks rows always carry an embedding.
func (r *CapabilityRepository) HasEmbeddedChunks(ctx context.Context, workspaceID string) (bool, error) {
	if workspaceID == "" {
		return r.exists(ctx, "SELECT EXISTS (SELECT 1 FROM docs_chunks)")
	}
	return r.exists(ctx, "SELECT EXISTS (SELECT 1 FROM docs_chunks WHERE workspace_id = ?)", workspaceID)
}

// HasInboundSupportEmail reports whether the workspace has received support email.
func (r *CapabilityRepository) HasInboundSupportEmail(ctx context.Context, workspaceID string) (bool, error) {
	return r.exists(ctx, "SELECT EXISTS (SELECT 1 FROM support_email_logs WHERE workspace_id = ? AND direction = 'inbound')", workspaceID)
}

// HasGitHubInstallation reports whether the organization has an active GitHub App installation.
func (r *CapabilityRepository) HasGitHubInstallation(ctx context.Context, organizationID string) (bool, error) {
	if organizationID == "" {
		return false, nil
	}
	return r.exists(ctx, "SELECT EXISTS (SELECT 1 FROM git_integrations WHERE organization_id = ? AND provider = 'github' AND active = true AND deleted_at IS NULL AND COALESCE(installation_id, '') <> '')", organizationID)
}

// ConnectedRepositoryCount counts repositories selected for the workspace.
func (r *CapabilityRepository) ConnectedRepositoryCount(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("git_repositories").
		Where("workspace_id = ? AND active = true AND selected = true AND deleted_at IS NULL", workspaceID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count connected repositories: %w", err)
	}
	return count, nil
}

// SharedAIConnectionSummary counts current shared AI connections and those whose
// last explicit test succeeded.
func (r *CapabilityRepository) SharedAIConnectionSummary(ctx context.Context) (AIConnectionSummary, error) {
	var summary AIConnectionSummary
	base := r.db.WithContext(ctx).Table("ai_connections").
		Where("scope = 'workspace' AND status = 'connected' AND superseded_by IS NULL")
	if err := base.Session(&gorm.Session{}).Count(&summary.Connected).Error; err != nil {
		return summary, fmt.Errorf("count shared AI connections: %w", err)
	}
	if err := base.Session(&gorm.Session{}).
		Where("last_verified_at IS NOT NULL AND last_verification_error IS NULL").
		Count(&summary.Verified).Error; err != nil {
		return summary, fmt.Errorf("count verified AI connections: %w", err)
	}
	return summary, nil
}

// GetCheck returns the last recorded check for key, or nil.
func (r *CapabilityRepository) GetCheck(ctx context.Context, key string) (*model.InstanceCapabilityCheck, error) {
	var check model.InstanceCapabilityCheck
	err := r.db.WithContext(ctx).Where("key = ?", key).Take(&check).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read capability check %q: %w", key, err)
	}
	return &check, nil
}

// RecordCheck stores the latest check for its key, replacing the previous one.
func (r *CapabilityRepository) RecordCheck(ctx context.Context, check *model.InstanceCapabilityCheck) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"ok", "error", "config_fingerprint", "checked_by", "checked_at"}),
	}).Create(check).Error
	if err != nil {
		return fmt.Errorf("record capability check %q: %w", check.Key, err)
	}
	return nil
}

func (r *CapabilityRepository) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var found bool
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&found).Error; err != nil {
		return false, fmt.Errorf("read capability evidence: %w", err)
	}
	return found, nil
}
