package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MCPRepository persists MCP policy, credentials, connections, and audit events.
type MCPRepository struct {
	db *gorm.DB
}

// NewMCPRepository creates an MCPRepository.
func NewMCPRepository(db *gorm.DB) *MCPRepository {
	return &MCPRepository{db: db}
}

// GetPolicy returns the workspace policy or nil when it has not been configured.
func (r *MCPRepository) GetPolicy(ctx context.Context, workspaceID string) (*model.MCPWorkspacePolicy, error) {
	var policy model.MCPWorkspacePolicy
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP workspace policy: %w", err)
	}
	return &policy, nil
}

// UpsertPolicy creates or updates the workspace policy.
func (r *MCPRepository) UpsertPolicy(ctx context.Context, policy *model.MCPWorkspacePolicy) error {
	if err := r.db.WithContext(ctx).Save(policy).Error; err != nil {
		return fmt.Errorf("upsert MCP workspace policy: %w", err)
	}
	return nil
}

// CreateClient registers an OAuth client.
func (r *MCPRepository) CreateClient(ctx context.Context, client *model.MCPClientRegistration) error {
	if err := r.db.WithContext(ctx).Create(client).Error; err != nil {
		return fmt.Errorf("create MCP client: %w", err)
	}
	return nil
}

// GetClient returns an OAuth client by public client ID.
func (r *MCPRepository) GetClient(ctx context.Context, clientID string) (*model.MCPClientRegistration, error) {
	var client model.MCPClientRegistration
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP client: %w", err)
	}
	return &client, nil
}

// CreateConnection persists an authorized user connection.
func (r *MCPRepository) CreateConnection(ctx context.Context, connection *model.MCPConnection) error {
	if err := r.db.WithContext(ctx).Create(connection).Error; err != nil {
		return fmt.Errorf("create MCP connection: %w", err)
	}
	return nil
}

// GetConnection returns a connection by ID.
func (r *MCPRepository) GetConnection(ctx context.Context, id string) (*model.MCPConnection, error) {
	var connection model.MCPConnection
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&connection).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP connection: %w", err)
	}
	return &connection, nil
}

// ListConnections lists active and revoked connections in reverse chronological order.
func (r *MCPRepository) ListConnections(ctx context.Context, workspaceID, userID string, all bool) ([]model.MCPConnection, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if !all {
		query = query.Where("user_id = ?", userID)
	}
	var connections []model.MCPConnection
	if err := query.Order("created_at DESC").Find(&connections).Error; err != nil {
		return nil, fmt.Errorf("list MCP connections: %w", err)
	}
	return connections, nil
}

// RevokeConnection revokes a connection and invalidates all issued access tokens.
func (r *MCPRepository) RevokeConnection(ctx context.Context, id, revokedBy string) error {
	now := time.Now()
	updates := map[string]any{
		"status":        model.MCPConnectionStatusRevoked,
		"revoked_at":    now,
		"token_version": gorm.Expr("token_version + 1"),
	}
	if revokedBy != "" {
		updates["revoked_by"] = revokedBy
	}
	result := r.db.WithContext(ctx).Model(&model.MCPConnection{}).
		Where("id = ? AND status = ?", id, model.MCPConnectionStatusActive).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("revoke MCP connection: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return r.RevokeRefreshTokensForConnection(ctx, id)
}

// RevokeWorkspaceAccess revokes all user connections, service principals, and their refresh/service tokens.
func (r *MCPRepository) RevokeWorkspaceAccess(ctx context.Context, workspaceID, revokedBy string) (int64, error) {
	var revoked int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		connections := tx.Model(&model.MCPConnection{}).
			Where("workspace_id = ? AND status = ?", workspaceID, model.MCPConnectionStatusActive).
			Updates(map[string]any{
				"status": model.MCPConnectionStatusRevoked, "revoked_at": now,
				"revoked_by": revokedBy, "token_version": gorm.Expr("token_version + 1"),
			})
		if connections.Error != nil {
			return fmt.Errorf("revoke workspace MCP connections: %w", connections.Error)
		}
		revoked += connections.RowsAffected
		if err := tx.Model(&model.MCPRefreshToken{}).
			Where("connection_id IN (?) AND revoked_at IS NULL", tx.Model(&model.MCPConnection{}).Select("id").Where("workspace_id = ?", workspaceID)).
			Update("revoked_at", now).Error; err != nil {
			return fmt.Errorf("revoke workspace MCP refresh tokens: %w", err)
		}
		principals := tx.Model(&model.MCPServicePrincipal{}).
			Where("workspace_id = ? AND status = ?", workspaceID, model.MCPServicePrincipalStatusActive).
			Updates(map[string]any{"status": model.MCPServicePrincipalStatusRevoked, "revoked_at": now})
		if principals.Error != nil {
			return fmt.Errorf("revoke workspace MCP service principals: %w", principals.Error)
		}
		revoked += principals.RowsAffected
		if err := tx.Model(&model.MCPServiceToken{}).
			Where("service_principal_id IN (?) AND revoked_at IS NULL", tx.Model(&model.MCPServicePrincipal{}).Select("id").Where("workspace_id = ?", workspaceID)).
			Update("revoked_at", now).Error; err != nil {
			return fmt.Errorf("revoke workspace MCP service tokens: %w", err)
		}
		return nil
	})
	return revoked, err
}

// TouchConnection updates connection last-used time.
func (r *MCPRepository) TouchConnection(ctx context.Context, id string, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&model.MCPConnection{}).
		Where("id = ?", id).Update("last_used_at", now).Error; err != nil {
		return fmt.Errorf("touch MCP connection: %w", err)
	}
	return nil
}

// CreateAuthorizationCode persists a hashed OAuth authorization code.
func (r *MCPRepository) CreateAuthorizationCode(ctx context.Context, code *model.MCPOAuthAuthorizationCode) error {
	if err := r.db.WithContext(ctx).Create(code).Error; err != nil {
		return fmt.Errorf("create MCP authorization code: %w", err)
	}
	return nil
}

// ConsumeAuthorizationCode atomically consumes a valid authorization code.
func (r *MCPRepository) ConsumeAuthorizationCode(ctx context.Context, codeHash string, now time.Time) (*model.MCPOAuthAuthorizationCode, error) {
	var code model.MCPOAuthAuthorizationCode
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code_hash = ?", codeHash).First(&code).Error; err != nil {
			return err
		}
		if code.ConsumedAt != nil || !code.ExpiresAt.After(now) {
			return gorm.ErrRecordNotFound
		}
		result := tx.Model(&model.MCPOAuthAuthorizationCode{}).
			Where("id = ? AND consumed_at IS NULL", code.ID).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("consume MCP authorization code: %w", err)
	}
	code.ConsumedAt = &now
	return &code, nil
}

// CreateRefreshToken persists a hashed refresh token.
func (r *MCPRepository) CreateRefreshToken(ctx context.Context, token *model.MCPRefreshToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create MCP refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken returns a refresh token record by hash.
func (r *MCPRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*model.MCPRefreshToken, error) {
	var token model.MCPRefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP refresh token: %w", err)
	}
	return &token, nil
}

// RotateRefreshToken atomically consumes an old token and creates its replacement.
func (r *MCPRepository) RotateRefreshToken(ctx context.Context, oldID string, replacement *model.MCPRefreshToken, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.MCPRefreshToken{}).
			Where("id = ? AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at > ?", oldID, now).
			Updates(map[string]any{"consumed_at": now, "replaced_by_id": replacement.ID})
		if result.Error != nil {
			return fmt.Errorf("consume MCP refresh token: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Create(replacement).Error; err != nil {
			return fmt.Errorf("create replacement MCP refresh token: %w", err)
		}
		return nil
	})
}

// RevokeRefreshFamily revokes all refresh tokens in one rotation family.
func (r *MCPRepository) RevokeRefreshFamily(ctx context.Context, familyID string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.MCPRefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke MCP refresh family: %w", err)
	}
	return nil
}

// RevokeRefreshTokensForConnection revokes all refresh tokens for a connection.
func (r *MCPRepository) RevokeRefreshTokensForConnection(ctx context.Context, connectionID string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.MCPRefreshToken{}).
		Where("connection_id = ? AND revoked_at IS NULL", connectionID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke MCP connection refresh tokens: %w", err)
	}
	return nil
}

// CreateServicePrincipal persists a service identity.
func (r *MCPRepository) CreateServicePrincipal(ctx context.Context, principal *model.MCPServicePrincipal) error {
	if err := r.db.WithContext(ctx).Create(principal).Error; err != nil {
		return fmt.Errorf("create MCP service principal: %w", err)
	}
	return nil
}

// GetServicePrincipal returns a service identity by ID.
func (r *MCPRepository) GetServicePrincipal(ctx context.Context, id string) (*model.MCPServicePrincipal, error) {
	var principal model.MCPServicePrincipal
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&principal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP service principal: %w", err)
	}
	return &principal, nil
}

// ListServicePrincipals lists service identities in a workspace.
func (r *MCPRepository) ListServicePrincipals(ctx context.Context, workspaceID string) ([]model.MCPServicePrincipal, error) {
	var principals []model.MCPServicePrincipal
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").Find(&principals).Error; err != nil {
		return nil, fmt.Errorf("list MCP service principals: %w", err)
	}
	return principals, nil
}

// RevokeServicePrincipal revokes a service identity and all of its tokens.
func (r *MCPRepository) RevokeServicePrincipal(ctx context.Context, workspaceID, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		result := tx.Model(&model.MCPServicePrincipal{}).
			Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, id, model.MCPServicePrincipalStatusActive).
			Updates(map[string]any{"status": model.MCPServicePrincipalStatusRevoked, "revoked_at": now})
		if result.Error != nil {
			return fmt.Errorf("revoke MCP service principal: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&model.MCPServiceToken{}).
			Where("service_principal_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error; err != nil {
			return fmt.Errorf("revoke MCP service tokens: %w", err)
		}
		return nil
	})
}

// CreateServiceToken persists a hashed service token.
func (r *MCPRepository) CreateServiceToken(ctx context.Context, token *model.MCPServiceToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create MCP service token: %w", err)
	}
	return nil
}

// GetServiceTokenByHash returns an unrevoked service token by hash.
func (r *MCPRepository) GetServiceTokenByHash(ctx context.Context, tokenHash string) (*model.MCPServiceToken, error) {
	var token model.MCPServiceToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP service token: %w", err)
	}
	return &token, nil
}

// ListServiceTokens lists token metadata for a service identity.
func (r *MCPRepository) ListServiceTokens(ctx context.Context, principalID string) ([]model.MCPServiceToken, error) {
	var tokens []model.MCPServiceToken
	if err := r.db.WithContext(ctx).Where("service_principal_id = ?", principalID).
		Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("list MCP service tokens: %w", err)
	}
	return tokens, nil
}

// RevokeServiceToken revokes one token belonging to the service identity.
func (r *MCPRepository) RevokeServiceToken(ctx context.Context, principalID, tokenID string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&model.MCPServiceToken{}).
		Where("service_principal_id = ? AND id = ? AND revoked_at IS NULL", principalID, tokenID).
		Update("revoked_at", now)
	if result.Error != nil {
		return fmt.Errorf("revoke MCP service token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// TouchServiceUse updates last-used timestamps for a token and principal.
func (r *MCPRepository) TouchServiceUse(ctx context.Context, tokenID, principalID string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MCPServiceToken{}).Where("id = ?", tokenID).
			Update("last_used_at", now).Error; err != nil {
			return fmt.Errorf("touch MCP service token: %w", err)
		}
		if err := tx.Model(&model.MCPServicePrincipal{}).Where("id = ?", principalID).
			Update("last_used_at", now).Error; err != nil {
			return fmt.Errorf("touch MCP service principal: %w", err)
		}
		return nil
	})
}

// CreateAuditEvent appends a sanitized audit event.
func (r *MCPRepository) CreateAuditEvent(ctx context.Context, event *model.MCPAuditEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create MCP audit event: %w", err)
	}
	return nil
}

// ListAuditEvents lists recent sanitized workspace events.
func (r *MCPRepository) ListAuditEvents(ctx context.Context, workspaceID string, limit int) ([]model.MCPAuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []model.MCPAuditEvent
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list MCP audit events: %w", err)
	}
	return events, nil
}

// CountRecentToolCalls counts successful calls for a user inside one workspace window.
func (r *MCPRepository) CountRecentToolCalls(ctx context.Context, workspaceID, userID, toolName string, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.MCPAuditEvent{}).
		Where("workspace_id = ? AND user_id = ? AND tool_name = ? AND outcome = ? AND created_at >= ?", workspaceID, userID, toolName, "success", since).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count recent MCP tool calls: %w", err)
	}
	return count, nil
}

// CountActiveMCPAgentRuns returns active MCP-started run counts for a user and workspace.
func (r *MCPRepository) CountActiveMCPAgentRuns(ctx context.Context, workspaceID, userID string) (int64, int64, error) {
	var counts struct {
		UserCount      int64 `gorm:"column:user_count"`
		WorkspaceCount int64 `gorm:"column:workspace_count"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN connection.user_id = ? OR service.actor_user_id = ? THEN 1 ELSE 0 END), 0) AS user_count,
			COUNT(*) AS workspace_count
		FROM mcp_agent_run_attributions AS attribution
		JOIN agent_runs AS run ON run.id = attribution.run_id
		LEFT JOIN mcp_connections AS connection ON connection.id = attribution.connection_id
		LEFT JOIN mcp_service_principals AS service ON service.id = attribution.service_principal_id
		WHERE attribution.workspace_id = ? AND run.status IN ?
	`, userID, userID, workspaceID, []string{model.AgentRunStatusQueued, model.AgentRunStatusRunning, model.AgentRunStatusPaused}).Scan(&counts).Error
	if err != nil {
		return 0, 0, fmt.Errorf("count active MCP agent runs: %w", err)
	}
	return counts.UserCount, counts.WorkspaceCount, nil
}

// GetIdempotencyRecord finds a non-expired record for a principal and key.
func (r *MCPRepository) GetIdempotencyRecord(ctx context.Context, principalKey, key string, now time.Time) (*model.MCPIdempotencyRecord, error) {
	var record model.MCPIdempotencyRecord
	err := r.db.WithContext(ctx).
		Where("principal_key = ? AND key = ? AND expires_at > ?", principalKey, key, now).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get MCP idempotency record: %w", err)
	}
	return &record, nil
}

// CreateIdempotencyRecord stores a completed mutation result.
func (r *MCPRepository) CreateIdempotencyRecord(ctx context.Context, record *model.MCPIdempotencyRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create MCP idempotency record: %w", err)
	}
	return nil
}

// CreateRunAttribution records which MCP principal started a normal agent run.
func (r *MCPRepository) CreateRunAttribution(ctx context.Context, attribution *model.MCPAgentRunAttribution) error {
	if err := r.db.WithContext(ctx).Create(attribution).Error; err != nil {
		return fmt.Errorf("create MCP run attribution: %w", err)
	}
	return nil
}

// CleanupExpired removes replay state and applies MCP audit retention windows.
func (r *MCPRepository) CleanupExpired(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&model.MCPIdempotencyRecord{}).Error; err != nil {
			return fmt.Errorf("delete expired MCP idempotency records: %w", err)
		}
		if err := tx.Where("expires_at <= ?", now.Add(-24*time.Hour)).Delete(&model.MCPOAuthAuthorizationCode{}).Error; err != nil {
			return fmt.Errorf("delete expired MCP authorization codes: %w", err)
		}
		if err := tx.Where("expires_at <= ?", now.Add(-30*24*time.Hour)).Delete(&model.MCPRefreshToken{}).Error; err != nil {
			return fmt.Errorf("delete expired MCP refresh tokens: %w", err)
		}
		if err := tx.Where(
			"(outcome = ? AND created_at < ?) OR (outcome IN ? AND created_at < ?)",
			"success", now.Add(-90*24*time.Hour), []string{"denied", "error"}, now.Add(-365*24*time.Hour),
		).Delete(&model.MCPAuditEvent{}).Error; err != nil {
			return fmt.Errorf("apply MCP audit retention: %w", err)
		}
		return nil
	})
}

// ListRunAttributions returns MCP origins for the requested workspace runs.
func (r *MCPRepository) ListRunAttributions(ctx context.Context, workspaceID string, runIDs []string) ([]model.MCPAgentRunAttribution, error) {
	if len(runIDs) == 0 {
		return nil, nil
	}
	var attributions []model.MCPAgentRunAttribution
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id IN ?", workspaceID, runIDs).
		Find(&attributions).Error; err != nil {
		return nil, fmt.Errorf("list MCP run attributions: %w", err)
	}
	return attributions, nil
}
