package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const cliClientID = "agent-runtime-cli"
const cliScope = "agent:local"

// ErrCLIDisabled identifies a disabled local execution rollout.
var ErrCLIDisabled = errors.New("local CLI access is disabled")

// ErrCLIUnauthorized identifies invalid, expired, revoked, or mismatched credentials.
var ErrCLIUnauthorized = errors.New("CLI authorization is invalid or expired")

// ErrCLIForbidden identifies a current membership or policy denial.
var ErrCLIForbidden = errors.New("CLI access is not permitted")

// ErrCLIInvalid identifies an unsupported authorization or admission request.
var ErrCLIInvalid = errors.New("invalid CLI request")

// ErrCLIConflict identifies an admission or execution identity conflict.
var ErrCLIConflict = errors.New("CLI execution conflict; refresh its state before retrying")

// CLIConfig separates this resource and issuer from MCP OAuth and cloud workers.
type CLIConfig struct {
	Enabled                   bool
	GatewayEnabled            bool
	PublicBaseURL, AppBaseURL string
}
type cliWorkspaceLister interface {
	List(context.Context, string, string) ([]model.WorkspaceWithRole, error)
}
type cliAuthorizer interface {
	ResolveActor(context.Context, string, string) (*authorization.Actor, error)
	Can(*authorization.Actor, authorization.Permission) bool
	CanAccessModule(context.Context, *authorization.Actor, model.ModuleID) (bool, error)
	WorkspaceMFAPolicy(context.Context, string, string) (model.WorkspaceMFAPolicy, error)
}

// CLIService owns workspace-scoped consent and local run admission.
type CLIService struct {
	repo           *repository.CLIRepository
	workspaces     cliWorkspaceLister
	authz          cliAuthorizer
	agents         *AgentService
	config         CLIConfig
	validateNative func(context.Context, *model.AgentRun) error
	generate       func(context.Context, *model.AgentRun, model.CLINativeRequest) (*model.CLINativeResponse, error)
}

// NewCLIService constructs the separately gated CLI API.
func NewCLIService(repo *repository.CLIRepository, workspaces cliWorkspaceLister, authz cliAuthorizer, agents *AgentService, cfg CLIConfig) (*CLIService, error) {
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
	cfg.AppBaseURL = strings.TrimRight(cfg.AppBaseURL, "/")
	if cfg.Enabled {
		for index, raw := range []string{cfg.PublicBaseURL, cfg.AppBaseURL} {
			u, err := url.Parse(raw)
			if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
				return nil, ErrCLIInvalid
			}
			if u.Scheme != "https" {
				ip := net.ParseIP(u.Hostname())
				if u.Scheme != "http" || !((ip != nil && ip.IsLoopback()) || (index == 1 && u.Hostname() == "localhost")) {
					return nil, ErrCLIInvalid
				}
			}
		}
	}
	s := &CLIService{repo: repo, workspaces: workspaces, authz: authz, agents: agents, config: cfg}
	s.generate = s.generateNative
	s.validateNative = func(ctx context.Context, run *model.AgentRun) error { _, _, err := s.modelRoute(ctx, run); return err }
	return s, nil
}

// Enabled reports the deployment rollout gate.
func (s *CLIService) Enabled() bool { return s != nil && s.config.Enabled }

// Discovery describes the admission capabilities implemented by this phase.
func (s *CLIService) Discovery() map[string]any {
	capabilities := []string{"admission", "execution_leases"}
	if s.config.GatewayEnabled {
		capabilities = append(capabilities, "model_gateway", "event_sync", "artifacts")
	}
	return map[string]any{"protocol_version": "agent-runtime-cli/v1alpha1", "app_id": "helpin", "name": "Helpin", "api_base_url": s.resource(), "issuer": s.issuer(), "client_id": cliClientID, "resource": s.resource(), "scopes": []string{cliScope}, "capabilities": capabilities}
}

// OAuthMetadata exposes this issuer without modifying the existing MCP issuer.
func (s *CLIService) OAuthMetadata() map[string]any {
	return map[string]any{"issuer": s.issuer(), "authorization_endpoint": s.issuer() + "/authorize", "token_endpoint": s.issuer() + "/token", "revocation_endpoint": s.issuer() + "/revoke", "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"}, "scopes_supported": []string{cliScope}}
}

// AuthorizationURL returns the consent page only after validating the client request.
func (s *CLIService) AuthorizationURL(q model.CLIAuthorizationQuery) (string, error) {
	if err := s.validateQuery(q); err != nil {
		return "", err
	}
	values := url.Values{"client_id": {q.ClientID}, "redirect_uri": {q.RedirectURI}, "response_type": {q.ResponseType}, "scope": {q.Scope}, "state": {q.State}, "code_challenge": {q.CodeChallenge}, "code_challenge_method": {q.CodeChallengeMethod}, "resource": {q.Resource}}
	return s.config.AppBaseURL + "/oauth/cli/authorize?" + values.Encode(), nil
}

// ConsentRequest returns only permitted workspace display data to the signed-in browser.
func (s *CLIService) ConsentRequest(ctx context.Context, userID string, q model.CLIAuthorizationQuery, mfaSatisfied bool) (map[string]any, error) {
	if err := s.validateQuery(q); err != nil {
		return nil, err
	}
	workspaces, err := s.workspaces.List(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	options := make([]map[string]string, 0, len(workspaces))
	for _, w := range workspaces {
		if _, err := s.actor(ctx, w.ID, userID); err == nil && s.requireMFA(ctx, w.ID, userID, mfaSatisfied) == nil {
			options = append(options, map[string]string{"id": w.ID, "name": w.Name, "role": w.Role})
		}
	}
	return map[string]any{"client_name": "Agent Runtime CLI", "query": q, "workspaces": options}, nil
}

// Authenticate validates resource-bound opaque access tokens and current workspace access.
func (s *CLIService) Authenticate(ctx context.Context, raw string) (*model.CLIConnection, error) {
	if !s.Enabled() {
		return nil, ErrCLIDisabled
	}
	if raw == "" || len(raw) > 256 {
		return nil, ErrCLIUnauthorized
	}
	token, err := s.repo.Token(ctx, mcpHash(raw))
	if err != nil {
		return nil, err
	}
	if token == nil || token.Kind != "access" || token.UsedAt != nil || !token.ExpiresAt.After(time.Now()) {
		return nil, ErrCLIUnauthorized
	}
	return s.connection(ctx, token.ConnectionID)
}
func (s *CLIService) connection(ctx context.Context, id string) (*model.CLIConnection, error) {
	c, err := s.repo.Connection(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil || c.RevokedAt != nil || c.ClientID != cliClientID || c.Resource != s.resource() || c.Scope != cliScope {
		return nil, ErrCLIUnauthorized
	}
	if err = s.requireMFA(ctx, c.WorkspaceID, c.UserID, c.MFASatisfied); err != nil {
		return nil, err
	}
	if _, err = s.actor(ctx, c.WorkspaceID, c.UserID); err != nil {
		return nil, err
	}
	return c, nil
}
func (s *CLIService) actor(ctx context.Context, workspaceID, userID string) (*authorization.Actor, error) {
	if s.authz == nil {
		return nil, ErrCLIForbidden
	}
	a, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return nil, ErrCLIForbidden
	}
	if !s.authz.Can(a, authorization.PermPMEdit) {
		return nil, ErrCLIForbidden
	}
	allowed, err := s.authz.CanAccessModule(ctx, a, model.ModulePM)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrCLIForbidden
	}
	return a, nil
}
func (s *CLIService) issuer() string   { return s.config.PublicBaseURL + "/api/cli/oauth" }
func (s *CLIService) resource() string { return s.config.PublicBaseURL + "/api/cli/v1" }

func (s *CLIService) requireMFA(ctx context.Context, workspaceID, userID string, satisfied bool) error {
	policy, err := s.authz.WorkspaceMFAPolicy(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if policy.EnforceTwoFactor && !satisfied {
		return ErrCLIForbidden
	}
	return nil
}
