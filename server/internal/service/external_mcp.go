package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/externalmcp"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	ErrExternalMCPDisabled         = errors.New("external MCP servers are disabled")
	ErrExternalMCPNotFound         = errors.New("external MCP server not found")
	ErrExternalMCPReauthorize      = errors.New("external MCP server requires reauthorization")
	validExternalMCPName           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	validExternalMCPRemoteToolName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	validOAuthScope                = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._/-]{0,127}$`)
	validExternalMCPHeaderName     = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

var customerIOReadTools = map[string]bool{
	"cio_prime":       true,
	"cio_schema":      true,
	"cio_read_api":    true,
	"cio_skills_list": true,
	"cio_skills_read": true,
	"cio_auth_status": true,
}

const (
	customerIOMCPEndpointUS = "https://mcp.customer.io/mcp"
	customerIOMCPEndpointEU = "https://mcp-eu.customer.io/mcp"
)

// ExternalMCPService owns durable workspace installation state and browser
// OAuth. Decrypted secrets exist only within bounded request/run preparation.
type ExternalMCPService struct {
	repo          *repository.ExternalMCPRepository
	notifications *NotificationService
	cfg           ExternalMCPServiceConfig
	key           []byte
	now           func() time.Time
}

func NewExternalMCPService(repo *repository.ExternalMCPRepository, notifications *NotificationService, cfg ExternalMCPServiceConfig) (*ExternalMCPService, error) {
	key, err := parseExternalMCPEncryptionKey(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if cfg.Enabled {
		if repo == nil {
			return nil, fmt.Errorf("external MCP repository is required")
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("EXTERNAL_MCP_ENCRYPTION_KEY is required when external MCP is enabled")
		}
		if strings.TrimSpace(cfg.OAuthRedirectURL) == "" {
			return nil, fmt.Errorf("EXTERNAL_MCP_OAUTH_REDIRECT_URL is required when external MCP is enabled")
		}
		if !validExternalMCPOAuthRedirectURL(cfg.OAuthRedirectURL, cfg.AllowInsecureLocalhost) {
			return nil, fmt.Errorf("EXTERNAL_MCP_OAUTH_REDIRECT_URL must be an absolute HTTPS URL without a fragment")
		}
		if !validExternalMCPClientAuthMethod(cfg.OAuthClientAuthMethod) {
			return nil, fmt.Errorf("EXTERNAL_MCP_OAUTH_CLIENT_AUTH_METHOD must be none, client_secret_basic, or client_secret_post")
		}
		clientID := strings.TrimSpace(cfg.OAuthClientID)
		clientSecret := strings.TrimSpace(cfg.OAuthClientSecret)
		clientAuthMethod := strings.TrimSpace(cfg.OAuthClientAuthMethod)
		if clientID == "" && clientSecret != "" {
			return nil, fmt.Errorf("EXTERNAL_MCP_OAUTH_CLIENT_SECRET requires EXTERNAL_MCP_OAUTH_CLIENT_ID")
		}
		if clientID != "" && clientAuthMethod != "" && clientAuthMethod != "none" && clientSecret == "" {
			return nil, fmt.Errorf("EXTERNAL_MCP_OAUTH_CLIENT_SECRET is required for the configured client authentication method")
		}
	}
	return &ExternalMCPService{repo: repo, notifications: notifications, cfg: cfg, key: key, now: time.Now}, nil
}

func (s *ExternalMCPService) Enabled() bool { return s != nil && s.cfg.Enabled }

func (s *ExternalMCPService) Providers() []ExternalMCPProviderOption {
	return []ExternalMCPProviderOption{
		{Provider: model.ExternalMCPProviderCustom, Name: "Any MCP server"},
		{Provider: model.ExternalMCPProviderCustomerIO, Region: "us", Name: "Customer.io (US)", EndpointURL: customerIOMCPEndpointUS, AuthType: model.ExternalMCPAuthOAuth, DefaultScopes: []string{"read"}, OptionalScopes: []string{"read:sensitive", "write", "write:live", "configure"}},
		{Provider: model.ExternalMCPProviderCustomerIO, Region: "eu", Name: "Customer.io (EU)", EndpointURL: customerIOMCPEndpointEU, AuthType: model.ExternalMCPAuthOAuth, DefaultScopes: []string{"read"}, OptionalScopes: []string{"read:sensitive", "write", "write:live", "configure"}},
	}
}

func (s *ExternalMCPService) ListServers(ctx context.Context, workspaceID string) ([]model.ExternalMCPServer, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	return s.repo.ListServers(ctx, workspaceID)
}

func (s *ExternalMCPService) CreateServer(ctx context.Context, workspaceID, actorID string, req CreateExternalMCPServerRequest) (*model.ExternalMCPServer, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(actorID) == "" {
		return nil, fmt.Errorf("workspace and actor are required")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	name := strings.TrimSpace(req.Name)
	endpoint := strings.TrimSpace(req.EndpointURL)
	authType := strings.TrimSpace(req.AuthType)
	if provider == model.ExternalMCPProviderCustomerIO {
		option, ok := s.customerIOProvider(req.Region)
		if !ok {
			return nil, fmt.Errorf("Customer.io region must be us or eu")
		}
		if name == "" {
			name = option.Name
		}
		endpoint, authType = option.EndpointURL, option.AuthType
		if len(req.OAuthScopes) == 0 {
			req.OAuthScopes = append([]string(nil), option.DefaultScopes...)
		}
	} else if provider == model.ExternalMCPProviderCustom {
		if name == "" || endpoint == "" {
			return nil, fmt.Errorf("name and endpoint_url are required")
		}
		if authType == "" {
			authType = model.ExternalMCPAuthOAuth
		}
	} else {
		return nil, fmt.Errorf("unsupported external MCP provider")
	}
	if len(name) > 120 {
		return nil, fmt.Errorf("name cannot exceed 120 characters")
	}
	serverName := externalMCPServerName(name)
	if !validExternalMCPName.MatchString(serverName) {
		return nil, fmt.Errorf("name cannot produce a valid MCP server identifier")
	}
	if serverName == "helpin" {
		return nil, fmt.Errorf("server name %q is reserved", serverName)
	}
	if _, err := externalmcp.NewClient(endpoint, s.cfg.AllowedHosts, s.cfg.AllowInsecureLocalhost); err != nil {
		return nil, err
	}
	if !isExternalMCPAuthType(authType) {
		return nil, fmt.Errorf("auth_type must be oauth, bearer_token, headers, or none")
	}
	if authType == model.ExternalMCPAuthBearerToken && strings.TrimSpace(req.BearerToken) == "" {
		return nil, fmt.Errorf("bearer_token is required")
	}
	if len(req.BearerToken) > 64<<10 {
		return nil, fmt.Errorf("bearer_token cannot exceed 65536 bytes")
	}
	if authType == model.ExternalMCPAuthHeaders && len(req.Headers) == 0 {
		return nil, fmt.Errorf("headers are required")
	}
	if authType != model.ExternalMCPAuthBearerToken && strings.TrimSpace(req.BearerToken) != "" {
		return nil, fmt.Errorf("bearer_token is only valid with bearer_token authentication")
	}
	if authType != model.ExternalMCPAuthHeaders && len(req.Headers) > 0 {
		return nil, fmt.Errorf("headers are only valid with headers authentication")
	}
	if authType != model.ExternalMCPAuthOAuth && len(req.OAuthScopes) > 0 {
		return nil, fmt.Errorf("oauth_scopes are only valid with oauth authentication")
	}
	scopes, err := normalizeExternalMCPScopes(provider, req.OAuthScopes)
	if err != nil {
		return nil, err
	}
	status := model.ExternalMCPStatusDisconnected
	if authType == model.ExternalMCPAuthOAuth {
		status = model.ExternalMCPStatusPendingOAuth
	}
	server := &model.ExternalMCPServer{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Name: name, ServerName: serverName,
		Provider: provider, EndpointURL: endpoint, Transport: "streamable_http", AuthType: authType,
		Status: status, Enabled: true, OAuthScopes: marshalStringList(scopes), RemoteIdentity: []byte("{}"), CreatedBy: actorID,
	}
	if err := s.repo.CreateServer(ctx, server); err != nil {
		return nil, err
	}
	credential := &model.ExternalMCPCredential{ServerID: server.ID, WorkspaceID: workspaceID}
	switch authType {
	case model.ExternalMCPAuthBearerToken:
		credential.EncryptedAccessToken, err = s.encrypt(workspaceID, server.ID, "access_token", strings.TrimSpace(req.BearerToken))
	case model.ExternalMCPAuthHeaders:
		var encoded []byte
		if encoded, err = validateAndMarshalExternalMCPHeaders(req.Headers); err == nil {
			credential.EncryptedHeaders, err = s.encrypt(workspaceID, server.ID, "headers", string(encoded))
		}
	}
	if err != nil {
		_ = s.repo.DeleteServer(ctx, workspaceID, server.ID)
		return nil, err
	}
	if authType != model.ExternalMCPAuthNone || len(req.Headers) > 0 {
		if err := s.repo.UpsertCredential(ctx, credential); err != nil {
			_ = s.repo.DeleteServer(ctx, workspaceID, server.ID)
			return nil, err
		}
	}
	if authType != model.ExternalMCPAuthOAuth {
		_ = s.SyncTools(ctx, workspaceID, server.ID)
	}
	return s.repo.GetServer(ctx, workspaceID, server.ID)
}

func (s *ExternalMCPService) UpdateServer(ctx context.Context, workspaceID, serverID string, req UpdateExternalMCPServerRequest) (*model.ExternalMCPServer, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if _, err := s.getServer(ctx, workspaceID, serverID); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 120 {
			return nil, fmt.Errorf("name is required and cannot exceed 120 characters")
		}
		updates["name"] = name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return s.getServer(ctx, workspaceID, serverID)
	}
	if err := s.repo.UpdateServer(ctx, workspaceID, serverID, updates); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExternalMCPNotFound
	} else if err != nil {
		return nil, err
	}
	return s.getServer(ctx, workspaceID, serverID)
}

func (s *ExternalMCPService) DeleteServer(ctx context.Context, workspaceID, serverID string) error {
	if err := s.requireEnabled(); err != nil {
		return err
	}
	if err := s.repo.DeleteServer(ctx, workspaceID, serverID); errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrExternalMCPNotFound
	} else {
		return err
	}
}

func (s *ExternalMCPService) UpdateToolPolicies(ctx context.Context, workspaceID, serverID string, req UpdateExternalMCPToolsRequest) (*model.ExternalMCPServer, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if _, err := s.getServer(ctx, workspaceID, serverID); err != nil {
		return nil, err
	}
	if len(req.Tools) > 128 {
		return nil, fmt.Errorf("tools cannot contain more than 128 entries")
	}
	policies := make(map[string]struct {
		Enabled bool
		Access  string
	}, len(req.Tools))
	for _, input := range req.Tools {
		id := strings.TrimSpace(input.ID)
		access := strings.TrimSpace(input.Access)
		if id == "" || access != model.ExternalMCPToolAccessRead && access != model.ExternalMCPToolAccessWrite {
			return nil, fmt.Errorf("each tool requires id and access read or write")
		}
		if _, exists := policies[id]; exists {
			return nil, fmt.Errorf("tool %s is duplicated", id)
		}
		policies[id] = struct {
			Enabled bool
			Access  string
		}{input.Enabled, access}
	}
	if err := s.repo.UpdateToolPolicies(ctx, workspaceID, serverID, policies); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExternalMCPNotFound
	} else if err != nil {
		return nil, err
	}
	return s.getServer(ctx, workspaceID, serverID)
}

func (s *ExternalMCPService) ListToolCatalog(ctx context.Context, workspaceID string) ([]model.ToolCatalogEntry, error) {
	tools, err := s.repo.ListCatalogTools(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	entries := make([]model.ToolCatalogEntry, 0, len(tools))
	for _, tool := range tools {
		if !tool.Enabled {
			continue
		}
		var schema any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			schema = map[string]any{}
		}
		entries = append(entries, model.ToolCatalogEntry{Name: tool.RuntimeAlias, Description: tool.Description, Category: "External MCP", InputSchema: schema, Presets: []string{}})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (s *ExternalMCPService) requireEnabled() error {
	if s == nil || !s.cfg.Enabled {
		return ErrExternalMCPDisabled
	}
	return nil
}

func (s *ExternalMCPService) getServer(ctx context.Context, workspaceID, serverID string) (*model.ExternalMCPServer, error) {
	server, err := s.repo.GetServer(ctx, workspaceID, serverID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrExternalMCPNotFound
	}
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrExternalMCPNotFound
	}
	return server, nil
}

func (s *ExternalMCPService) customerIOProvider(region string) (ExternalMCPProviderOption, bool) {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		region = "us"
	}
	for _, provider := range s.Providers() {
		if provider.Provider == model.ExternalMCPProviderCustomerIO && provider.Region == region {
			return provider, true
		}
	}
	return ExternalMCPProviderOption{}, false
}

func externalMCPServerName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out strings.Builder
	lastUnderscore := false
	for _, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-'
		if valid {
			out.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			out.WriteByte('_')
			lastUnderscore = true
		}
	}
	value := strings.Trim(out.String(), "_-")
	if len(value) > 56 {
		value = strings.Trim(value[:56], "_-")
	}
	return value
}

func isExternalMCPAuthType(value string) bool {
	switch value {
	case model.ExternalMCPAuthOAuth, model.ExternalMCPAuthBearerToken, model.ExternalMCPAuthHeaders, model.ExternalMCPAuthNone:
		return true
	default:
		return false
	}
}

func validExternalMCPClientAuthMethod(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "none", "client_secret_basic", "client_secret_post":
		return true
	default:
		return false
	}
}

func validExternalMCPOAuthRedirectURL(value string, allowLocal bool) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !parsed.IsAbs() || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	host := strings.ToLower(parsed.Hostname())
	return allowLocal && parsed.Scheme == "http" && (host == "localhost" || strings.HasSuffix(host, ".localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}

func normalizeExternalMCPScopes(provider string, scopes []string) ([]string, error) {
	if len(scopes) > 32 {
		return nil, fmt.Errorf("oauth_scopes cannot contain more than 32 entries")
	}
	allowedCustomer := map[string]bool{"read": true, "read:sensitive": true, "write": true, "write:live": true, "configure": true}
	seen := map[string]bool{}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || !validOAuthScope.MatchString(scope) {
			return nil, fmt.Errorf("invalid OAuth scope")
		}
		if provider == model.ExternalMCPProviderCustomerIO && !allowedCustomer[scope] {
			return nil, fmt.Errorf("unsupported Customer.io OAuth scope %q", scope)
		}
		if !seen[scope] {
			seen[scope] = true
			result = append(result, scope)
		}
	}
	sort.Strings(result)
	return result, nil
}

func validateAndMarshalExternalMCPHeaders(headers map[string]string) ([]byte, error) {
	if len(headers) == 0 || len(headers) > 16 {
		return nil, fmt.Errorf("headers must contain between 1 and 16 entries")
	}
	clean := make(map[string]string, len(headers))
	totalBytes := 0
	for key, value := range headers {
		key = strings.TrimSpace(key)
		if !validExternalMCPHeaderName.MatchString(key) || strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") || len(value) > 8192 {
			return nil, fmt.Errorf("invalid credential header")
		}
		key = http.CanonicalHeaderKey(key)
		switch strings.ToLower(key) {
		case "host", "cookie", "connection", "content-length", "transfer-encoding", "proxy-authorization", "mcp-session-id", "mcp-protocol-version", "origin":
			return nil, fmt.Errorf("credential header %s is not allowed", key)
		}
		if _, exists := clean[key]; exists {
			return nil, fmt.Errorf("credential header %s is duplicated", key)
		}
		totalBytes += len(key) + len(value)
		if totalBytes > 64<<10 {
			return nil, fmt.Errorf("credential headers cannot exceed 65536 bytes")
		}
		clean[key] = value
	}
	return json.Marshal(clean)
}

func parseExternalMCPEncryptionKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len(value) == 32 {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("EXTERNAL_MCP_ENCRYPTION_KEY must be 32 raw bytes, 64 hex characters, or base64-encoded 32 bytes")
}

func (s *ExternalMCPService) aad(workspaceID, serverID, field string) []byte {
	return []byte("external-mcp|v1|" + workspaceID + "|" + serverID + "|" + field)
}

func (s *ExternalMCPService) encrypt(workspaceID, serverID, field, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return appcrypto.EncryptStringWithAAD(value, s.key, s.aad(workspaceID, serverID, field))
}

func (s *ExternalMCPService) decrypt(workspaceID, serverID, field, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return appcrypto.DecryptStringWithAAD(value, s.key, s.aad(workspaceID, serverID, field))
}
