package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ExternalMCPRepository persists workspace-owned outbound MCP installations.
type ExternalMCPRepository struct {
	db *gorm.DB
}

func NewExternalMCPRepository(db *gorm.DB) *ExternalMCPRepository {
	return &ExternalMCPRepository{db: db}
}

func (r *ExternalMCPRepository) CreateServer(ctx context.Context, server *model.ExternalMCPServer) error {
	if len(server.OAuthScopes) == 0 {
		server.OAuthScopes = []byte("[]")
	}
	if len(server.RemoteIdentity) == 0 {
		server.RemoteIdentity = []byte("{}")
	}
	if err := r.db.WithContext(ctx).Create(server).Error; err != nil {
		return fmt.Errorf("create external MCP server: %w", err)
	}
	return nil
}

func (r *ExternalMCPRepository) GetServer(ctx context.Context, workspaceID, serverID string) (*model.ExternalMCPServer, error) {
	var server model.ExternalMCPServer
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, serverID).
		First(&server).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get external MCP server: %w", err)
	}
	tools, err := r.ListTools(ctx, workspaceID, serverID)
	if err != nil {
		return nil, err
	}
	server.Tools = tools
	return &server, nil
}

func (r *ExternalMCPRepository) ListServers(ctx context.Context, workspaceID string) ([]model.ExternalMCPServer, error) {
	var servers []model.ExternalMCPServer
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").Find(&servers).Error; err != nil {
		return nil, fmt.Errorf("list external MCP servers: %w", err)
	}
	var tools []model.ExternalMCPTool
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("remote_name ASC").Find(&tools).Error; err != nil {
		return nil, fmt.Errorf("list external MCP tools: %w", err)
	}
	byServer := make(map[string][]model.ExternalMCPTool)
	for _, tool := range tools {
		byServer[tool.ServerID] = append(byServer[tool.ServerID], tool)
	}
	for i := range servers {
		servers[i].Tools = byServer[servers[i].ID]
	}
	return servers, nil
}

func (r *ExternalMCPRepository) UpdateServer(ctx context.Context, workspaceID, serverID string, updates map[string]any) error {
	updates["updated_at"] = time.Now()
	result := r.db.WithContext(ctx).Model(&model.ExternalMCPServer{}).
		Where("workspace_id = ? AND id = ?", workspaceID, serverID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update external MCP server: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ExternalMCPRepository) DeleteServer(ctx context.Context, workspaceID, serverID string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, serverID).
		Delete(&model.ExternalMCPServer{})
	if result.Error != nil {
		return fmt.Errorf("delete external MCP server: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ExternalMCPRepository) UpsertCredential(ctx context.Context, credential *model.ExternalMCPCredential) error {
	if err := upsertExternalMCPCredential(r.db.WithContext(ctx), credential); err != nil {
		return fmt.Errorf("upsert external MCP credential: %w", err)
	}
	return nil
}

func upsertExternalMCPCredential(db *gorm.DB, credential *model.ExternalMCPCredential) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "server_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"encrypted_access_token", "encrypted_refresh_token", "encrypted_headers",
			"encrypted_client_secret", "client_id", "token_type", "authorization_endpoint",
			"token_endpoint", "registration_endpoint", "resource_url", "token_endpoint_auth_method",
			"access_token_expires_at", "updated_at",
		}),
	}).Create(credential).Error
}

func (r *ExternalMCPRepository) GetCredential(ctx context.Context, workspaceID, serverID string) (*model.ExternalMCPCredential, error) {
	var credential model.ExternalMCPCredential
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND server_id = ?", workspaceID, serverID).
		First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get external MCP credential: %w", err)
	}
	return &credential, nil
}

// WithCredentialLock serializes refresh-token rotation across application
// instances. The callback must remain bounded by its context.
func (r *ExternalMCPRepository) WithCredentialLock(
	ctx context.Context,
	workspaceID, serverID string,
	fn func(*model.ExternalMCPCredential) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential model.ExternalMCPCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND server_id = ?", workspaceID, serverID).
			First(&credential).Error; err != nil {
			return err
		}
		if err := fn(&credential); err != nil {
			return err
		}
		return upsertExternalMCPCredential(tx, &credential)
	})
}

func (r *ExternalMCPRepository) CreateOAuthState(ctx context.Context, state *model.ExternalMCPOAuthState) error {
	if len(state.RequestedScopes) == 0 {
		state.RequestedScopes = []byte("[]")
	}
	if err := r.db.WithContext(ctx).Create(state).Error; err != nil {
		return fmt.Errorf("create external MCP OAuth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState atomically consumes one unexpired browser OAuth state.
func (r *ExternalMCPRepository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (*model.ExternalMCPOAuthState, error) {
	var state model.ExternalMCPOAuthState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("state_hash = ?", stateHash).First(&state).Error; err != nil {
			return err
		}
		if state.ConsumedAt != nil || !state.ExpiresAt.After(now) {
			return gorm.ErrRecordNotFound
		}
		result := tx.Model(&model.ExternalMCPOAuthState{}).
			Where("id = ? AND consumed_at IS NULL", state.ID).
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
		return nil, fmt.Errorf("consume external MCP OAuth state: %w", err)
	}
	state.ConsumedAt = &now
	return &state, nil
}

func (r *ExternalMCPRepository) ListTools(ctx context.Context, workspaceID, serverID string) ([]model.ExternalMCPTool, error) {
	var tools []model.ExternalMCPTool
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}
	if err := query.Order("remote_name ASC").Find(&tools).Error; err != nil {
		return nil, fmt.Errorf("list external MCP tools: %w", err)
	}
	return tools, nil
}

// ListCatalogTools returns only tools that an agent can currently select.
func (r *ExternalMCPRepository) ListCatalogTools(ctx context.Context, workspaceID string) ([]model.ExternalMCPTool, error) {
	var tools []model.ExternalMCPTool
	if err := r.db.WithContext(ctx).Table("external_mcp_tools AS t").
		Select("t.*").
		Joins("JOIN external_mcp_servers AS s ON s.id = t.server_id AND s.workspace_id = t.workspace_id").
		Where("t.workspace_id = ? AND t.enabled = ? AND s.enabled = ? AND s.status = ?", workspaceID, true, true, model.ExternalMCPStatusConnected).
		Order("t.runtime_alias ASC").Find(&tools).Error; err != nil {
		return nil, fmt.Errorf("list external MCP catalog tools: %w", err)
	}
	return tools, nil
}

// ReplaceTools upserts the current discovery result while retaining each
// existing tool's enabled/access policy, then removes tools no longer exposed.
func (r *ExternalMCPRepository) ReplaceTools(ctx context.Context, workspaceID, serverID string, tools []model.ExternalMCPTool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := make([]string, 0, len(tools))
		for i := range tools {
			tool := &tools[i]
			seen = append(seen, tool.RemoteName)
			if len(tool.InputSchema) == 0 {
				tool.InputSchema = []byte("{}")
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "server_id"}, {Name: "remote_name"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"runtime_alias", "description", "input_schema", "schema_hash", "last_seen_at", "updated_at",
				}),
			}).Create(tool).Error; err != nil {
				return fmt.Errorf("upsert external MCP tool %q: %w", tool.RemoteName, err)
			}
		}
		query := tx.Where("workspace_id = ? AND server_id = ?", workspaceID, serverID)
		if len(seen) > 0 {
			query = query.Where("remote_name NOT IN ?", seen)
		}
		if err := query.Delete(&model.ExternalMCPTool{}).Error; err != nil {
			return fmt.Errorf("remove stale external MCP tools: %w", err)
		}
		return nil
	})
}

func (r *ExternalMCPRepository) UpdateToolPolicies(ctx context.Context, workspaceID, serverID string, policies map[string]struct {
	Enabled bool
	Access  string
}) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, policy := range policies {
			result := tx.Model(&model.ExternalMCPTool{}).
				Where("workspace_id = ? AND server_id = ? AND id = ?", workspaceID, serverID, id).
				Updates(map[string]any{"enabled": policy.Enabled, "access": policy.Access, "updated_at": time.Now()})
			if result.Error != nil {
				return fmt.Errorf("update external MCP tool policy: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

func (r *ExternalMCPRepository) ListEnabledToolsByAliases(ctx context.Context, workspaceID string, aliases []string) ([]model.ExternalMCPTool, error) {
	if len(aliases) == 0 {
		return []model.ExternalMCPTool{}, nil
	}
	var tools []model.ExternalMCPTool
	if err := r.db.WithContext(ctx).Table("external_mcp_tools AS t").
		Select("t.*").
		Joins("JOIN external_mcp_servers AS s ON s.id = t.server_id AND s.workspace_id = t.workspace_id").
		Where("t.workspace_id = ? AND t.runtime_alias IN ? AND t.enabled = ? AND s.enabled = ?", workspaceID, aliases, true, true).
		Find(&tools).Error; err != nil {
		return nil, fmt.Errorf("resolve external MCP tools: %w", err)
	}
	return tools, nil
}

func (r *ExternalMCPRepository) CreateRunBindings(ctx context.Context, bindings []model.AgentRunExternalMCPBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&bindings).Error; err != nil {
		return fmt.Errorf("create external MCP run bindings: %w", err)
	}
	return nil
}

func (r *ExternalMCPRepository) ListRunBindings(ctx context.Context, workspaceID, agentRunID string) ([]model.AgentRunExternalMCPBinding, error) {
	var bindings []model.AgentRunExternalMCPBinding
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_run_id = ?", workspaceID, agentRunID).
		Order("created_at ASC").Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("list external MCP run bindings: %w", err)
	}
	return bindings, nil
}

func (r *ExternalMCPRepository) ListRunBindingsForServer(ctx context.Context, workspaceID, serverID string) ([]model.AgentRunExternalMCPBinding, error) {
	var bindings []model.AgentRunExternalMCPBinding
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND server_id = ?", workspaceID, serverID).
		Order("created_at DESC").Limit(100).Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("list external MCP server run bindings: %w", err)
	}
	return bindings, nil
}
