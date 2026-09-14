package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var ErrAIConnection = errors.New("AI connection is unavailable or requires reconnection")

// ErrAIConnectionUnavailable permits a pre-execution fallback after authorization.
// Ownership, configuration, database, and encryption errors do not use this error.
var ErrAIConnectionUnavailable = fmt.Errorf("%w: connection is not ready", ErrAIConnection)

type AIConnectionConfig struct {
	EncryptionKey   string
	ChatGPTEnabled  bool
	ChatGPTClientID string
	AppID           string
}
type aiConnectionSecret struct {
	APIKey string                     `json:"api_key,omitempty"`
	Token  *chatgptauth.Token         `json:"token,omitempty"`
	Device *chatgptauth.DeviceSession `json:"device,omitempty"`
}
type AIConnectionService struct {
	admissionPolicy AIConnectionAdmissionPolicy
	repo            *repository.AIConnectionRepository
	key             []byte
	cfg             AIConnectionConfig
	oauth           *chatgptauth.Client
	catalog         *aimodel.Catalog
	runtime         *AgentRuntimeClient
	authz           *authorization.AuthzService
}

func NewAIConnectionService(repo *repository.AIConnectionRepository, catalog *aimodel.Catalog, runtime *AgentRuntimeClient, cfg AIConnectionConfig) (*AIConnectionService, error) {
	key, err := parseExternalMCPEncryptionKey(cfg.EncryptionKey)
	if err != nil {
		return nil, errors.New("invalid AI_CONNECTION_ENCRYPTION_KEY")
	}
	oauth, err := chatgptauth.NewClient(chatgptauth.Config{ClientID: cfg.ChatGPTClientID})
	if err != nil {
		return nil, err
	}
	return &AIConnectionService{repo: repo, key: key, cfg: cfg, oauth: oauth, catalog: catalog, runtime: runtime}, nil
}
func (s *AIConnectionService) Enabled() bool { return s != nil && len(s.key) == 32 && s.runtime != nil }
func (s *AIConnectionService) Models() []aimodel.PublicModel {
	out := []aimodel.PublicModel{}
	if s == nil || s.catalog == nil {
		return out
	}
	for _, m := range s.catalog.PublicModels() {
		if _, err := s.catalog.ResolveDefault(m.Provider, m.SelectionModel, "standard"); err != nil {
			continue
		}
		if !m.Enabled {
			continue
		}
		switch m.Provider {
		case "openai", "anthropic", "openrouter":
			out = append(out, m)
		}
		if m.Provider == "openai" && s.cfg.ChatGPTEnabled {
			copy := m
			copy.Provider = "openai_chatgpt"
			out = append(out, copy)
		}
	}
	return out
}
func (s *AIConnectionService) List(ctx context.Context, workspace, user string) ([]model.AIConnection, error) {
	if !s.Enabled() {
		return []model.AIConnection{}, nil
	}
	member, err := s.repo.ActiveMember(ctx, workspace, user)
	if err != nil || !member {
		return nil, ErrAIConnection
	}
	connections, err := s.repo.List(ctx, workspace, user)
	if err != nil {
		return nil, err
	}
	for i := range connections {
		connections[i].Policy, err = connectionPolicyView(ctx, s.admissionPolicy, workspace, &connections[i])
		if err != nil {
			return nil, err
		}
	}
	return connections, nil
}
func aiConnectionAAD(c *model.AIConnection) []byte {
	return []byte("ai-connection|v1|" + c.WorkspaceID + "|" + derefString(c.UserID) + "|" + c.ID + "|" + c.Provider)
}
func (s *AIConnectionService) seal(c *model.AIConnection, secret aiConnectionSecret) error {
	raw, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	c.EncryptedSecret, err = crypto.EncryptWithAAD(raw, s.key, aiConnectionAAD(c))
	return err
}
func (s *AIConnectionService) open(c *model.AIConnection) (aiConnectionSecret, error) {
	var secret aiConnectionSecret
	raw, err := crypto.DecryptWithAAD(c.EncryptedSecret, s.key, aiConnectionAAD(c))
	if err != nil {
		return secret, ErrAIConnection
	}
	if json.Unmarshal(raw, &secret) != nil {
		return secret, ErrAIConnection
	}
	return secret, nil
}
func ownAIConnection(c *model.AIConnection, workspace, user string) error {
	if c == nil || c.WorkspaceID != workspace || derefString(c.UserID) != user || user == "" {
		return ErrAIConnection
	}
	return nil
}

func (s *AIConnectionService) Create(ctx context.Context, workspace, user string, req model.CreateAIConnectionRequest) (*model.AIConnectionLogin, error) {
	if !s.Enabled() || workspace == "" || user == "" {
		return nil, ErrAIConnection
	}
	if req.Scope == "" {
		req.Scope = "personal"
	}
	if req.Scope != "personal" && req.Scope != "workspace" {
		return nil, errors.New("unsupported connection scope")
	}
	if req.Scope == "workspace" {
		if err := s.requireConnectionManager(ctx, workspace, user); err != nil {
			return nil, err
		}
		if req.Provider == "openai_chatgpt" {
			return nil, errors.New("ChatGPT connections must be personal")
		}
	}
	req.Name = strings.TrimSpace(req.Name)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.Name == "" || len(req.Name) > 100 {
		return nil, errors.New("connection name is required and must not exceed 100 characters")
	}
	switch req.Provider {
	case "openai", "anthropic", "openrouter":
		if req.APIKey == "" || len(req.APIKey) > 65536 || strings.ContainsAny(req.APIKey, "\r\n") {
			return nil, errors.New("a valid API key is required")
		}
	case "openai_chatgpt":
		if !s.cfg.ChatGPTEnabled || req.APIKey != "" {
			return nil, errors.New("ChatGPT device login is not enabled")
		}
	default:
		return nil, errors.New("unsupported AI provider")
	}
	c := &model.AIConnection{ID: uuid.NewString(), WorkspaceID: workspace, UserID: &user, Scope: req.Scope, Funding: "customer", Name: req.Name, Provider: req.Provider, Status: "connected"}
	if req.Scope == "workspace" {
		c.UserID = nil
	}
	secret := aiConnectionSecret{APIKey: req.APIKey}
	var session *chatgptauth.DeviceSession
	if req.Provider == "openai_chatgpt" {
		var err error
		session, err = s.oauth.StartDeviceLogin(ctx)
		if err != nil {
			return nil, err
		}
		secret.Device = session
		c.Status = "pending"
		c.ExpiresAt = &session.ExpiresAt
	}
	if err := s.seal(c, secret); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return aiLoginResult(c, session), nil
}
func aiLoginResult(c *model.AIConnection, session *chatgptauth.DeviceSession) *model.AIConnectionLogin {
	result := &model.AIConnectionLogin{Connection: *c}
	if session != nil {
		result.VerificationURL = session.VerificationURL
		result.UserCode = session.UserCode
		result.ExpiresAt = &session.ExpiresAt
		result.IntervalSeconds = session.IntervalSeconds
	}
	return result
}

func (s *AIConnectionService) Poll(ctx context.Context, workspace, user, id string) (*model.AIConnectionLogin, error) {
	if !s.Enabled() {
		return nil, ErrAIConnection
	}
	var result *model.AIConnectionLogin
	var publicErr error
	err := s.repo.WithLocked(ctx, id, func(c *model.AIConnection) error {
		if err := ownAIConnection(c, workspace, user); err != nil {
			return err
		}
		if c.Status != "pending" {
			result = aiLoginResult(c, nil)
			return nil
		}
		secret, err := s.open(c)
		if err != nil {
			return err
		}
		if secret.Device == nil {
			return ErrAIConnection
		}
		polled, err := s.oauth.PollDeviceLogin(ctx, secret.Device)
		if err != nil {
			var authErr *chatgptauth.AuthError
			if errors.As(err, &authErr) && (authErr.Code == "expired" || authErr.Code == "authorization_failed" || authErr.Code == "reconnect_required") {
				c.Status = "reauthorization_required"
				c.EncryptedSecret = nil
			}
			if c.Status == "pending" {
				if sealErr := s.seal(c, secret); sealErr != nil {
					return sealErr
				}
			}
			publicErr = err
			return nil
		}
		if polled.Token != nil {
			if c.AccountID != "" && c.AccountID != polled.Token.AccountID {
				c.Status = "reauthorization_required"
				c.EncryptedSecret = nil
				publicErr = errors.New("ChatGPT account changed; create a new connection and run")
				return nil
			}
			secret = aiConnectionSecret{Token: polled.Token}
			c.AccountID = polled.Token.AccountID
			c.ExpiresAt = &polled.Token.ExpiresAt
			c.Status = "connected"
		}
		if err := s.seal(c, secret); err != nil {
			return err
		}
		result = aiLoginResult(c, secret.Device)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if publicErr != nil {
		return nil, publicErr
	}
	return result, nil
}

func (s *AIConnectionService) Reconnect(ctx context.Context, workspace, user, id, apiKey string) (*model.AIConnectionLogin, error) {
	if !s.Enabled() {
		return nil, ErrAIConnection
	}
	if err := s.authorizeConnectionManagement(ctx, workspace, user, id); err != nil {
		return nil, err
	}
	var result *model.AIConnectionLogin
	err := s.repo.WithLocked(ctx, id, func(c *model.AIConnection) error {
		if err := accessAIConnection(c, workspace, user); err != nil {
			return err
		}
		secret := aiConnectionSecret{}
		if c.Provider == "openai_chatgpt" {
			if !s.cfg.ChatGPTEnabled {
				return ErrAIConnection
			}
			session, err := s.oauth.StartDeviceLogin(ctx)
			if err != nil {
				return err
			}
			secret.Device = session
			c.Status = "pending"
			c.ExpiresAt = &session.ExpiresAt
		} else {
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" || len(apiKey) > 65536 || strings.ContainsAny(apiKey, "\r\n") {
				return errors.New("a valid API key is required")
			}
			secret.APIKey = apiKey
			c.Status = "connected"
		}
		if err := s.seal(c, secret); err != nil {
			return err
		}
		result = aiLoginResult(c, secret.Device)
		return nil
	})
	return result, err
}

func (s *AIConnectionService) Credential(ctx context.Context, workspace, user, id string, force bool, rejectedFingerprint ...string) (*model.AIConnection, *sdk.ModelCredential, error) {
	if !s.Enabled() {
		return nil, nil, ErrAIConnection
	}
	member, err := s.repo.ActiveMember(ctx, workspace, user)
	if err != nil || !member {
		return nil, nil, ErrAIConnection
	}
	return s.loadConnectionCredential(ctx, workspace, user, id, force, rejectedFingerprint...)
}

func (s *AIConnectionService) loadConnectionCredential(ctx context.Context, workspace, user, id string, force bool, rejectedFingerprint ...string) (*model.AIConnection, *sdk.ModelCredential, error) {
	var connection *model.AIConnection
	var credential *sdk.ModelCredential
	var publicErr error
	err := s.repo.WithLocked(ctx, id, func(c *model.AIConnection) error {
		if err := accessAIConnection(c, workspace, user); err != nil {
			return err
		}
		if c.Status != "connected" {
			return ErrAIConnectionUnavailable
		}
		secret, err := s.open(c)
		if err != nil {
			return err
		}
		next := sdk.ModelCredential{Type: "api_key", APIKey: secret.APIKey, ConnectionID: c.ID}
		if c.Provider == "openai_chatgpt" {
			if !s.cfg.ChatGPTEnabled || secret.Token == nil {
				return ErrAIConnection
			}
			token := secret.Token
			if force && len(rejectedFingerprint) > 0 && rejectedFingerprint[0] != fmt.Sprintf("%x", sha256.Sum256([]byte(token.AccessToken))) {
				force = false
			}
			if force || !token.ExpiresAt.After(time.Now().Add(time.Minute)) {
				refreshed, err := s.oauth.Refresh(ctx, *token)
				if err != nil {
					var authErr *chatgptauth.AuthError
					if errors.As(err, &authErr) && (authErr.Code == "reconnect_required" || authErr.Code == "account_changed") {
						c.Status = "reauthorization_required"
					}
					publicErr = ErrAIConnectionUnavailable
					return nil
				}
				secret.Token = refreshed
				token = refreshed
				c.ExpiresAt = &token.ExpiresAt
				if err := s.seal(c, secret); err != nil {
					return err
				}
			}
			next = sdk.ModelCredential{Type: "oauth", AccessToken: token.AccessToken, ExpiresAt: &token.ExpiresAt, ConnectionID: c.ID, AccountID: token.AccountID}
		}
		copy := *c
		connection = &copy
		credential = &next
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if publicErr != nil {
		return nil, nil, publicErr
	}
	return connection, credential, nil
}

func (s *AIConnectionService) ResolveModel(provider, name string) (*sdk.RunModel, aimodel.Tier, error) {
	pricingProvider := provider
	if provider == "openai_chatgpt" {
		pricingProvider = "openai"
	}
	route, err := s.catalog.ResolveDefault(pricingProvider, name, "standard")
	if err != nil {
		return nil, "", errors.New("select a supported model for this AI connection")
	}
	return &sdk.RunModel{Provider: provider, Model: route.Route}, route.Tier, nil
}

func (s *AIConnectionService) RefreshRun(ctx context.Context, req sdk.ModelCredentialRefreshRequest) (*sdk.UpdateRunModelCredentialRequest, error) {
	if !s.Enabled() || req.AppID != s.cfg.AppID || req.HostRunID == "" {
		return nil, ErrAIConnection
	}
	c, err := s.repo.Get(ctx, req.ConnectionID)
	if err != nil || c == nil {
		return nil, ErrAIConnection
	}
	run, err := s.repo.Run(ctx, c.WorkspaceID, req.HostRunID)
	if err != nil || run == nil || !model.IsAgentRunActiveStatus(run.Status) || (c.UserID != nil && derefString(run.TriggeredByUserID) != *c.UserID) {
		return nil, ErrAIConnection
	}
	var input model.AgentRunInputPayload
	if json.Unmarshal(run.Input, &input) != nil || input.ModelConnectionID != c.ID || input.ModelProvider != req.Provider || c.Provider != req.Provider || c.AccountID != req.AccountID {
		return nil, ErrAIConnection
	}
	if err := s.authorizeAcceptedSelectionOwner(ctx, run, input.AISelection); err != nil {
		return nil, err
	}
	if id, mapped := agentRuntimeRunID(run); mapped && id != req.RunID {
		return nil, ErrAIConnection
	}
	var credential *sdk.ModelCredential
	if c.Scope == "workspace" {
		_, credential, err = s.sharedCredential(ctx, c.WorkspaceID, c.ID)
	} else {
		_, credential, err = s.Credential(ctx, c.WorkspaceID, derefString(c.UserID), c.ID, req.Reason == "unauthorized", req.CredentialFingerprint)
	}
	if err != nil {
		return nil, err
	}
	return &sdk.UpdateRunModelCredentialRequest{Credential: *credential}, nil
}

func (s *AIConnectionService) Disconnect(ctx context.Context, workspace, user, id string) error {
	if !s.Enabled() {
		return ErrAIConnection
	}
	if err := s.authorizeConnectionManagement(ctx, workspace, user, id); err != nil {
		return err
	}
	if err := s.repo.WithLocked(ctx, id, func(c *model.AIConnection) error {
		if err := accessAIConnection(c, workspace, user); err != nil {
			return err
		}
		c.Status = "disconnected"
		c.EncryptedSecret = nil
		c.ExpiresAt = nil
		return nil
	}); err != nil {
		return err
	}
	runs, err := s.boundConnectionRuns(ctx, workspace, user, id)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if runtimeID, ok := agentRuntimeRunID(&run); ok {
			if err := s.runtime.RevokeRunModelCredential(ctx, runtimeID); err != nil {
				return fmt.Errorf("connection disconnected; retry to revoke remaining runs")
			}
		}
	}
	return nil
}

func (s *AIConnectionService) ReauthorizeRuns(ctx context.Context, workspace, user, id string) error {
	if err := s.authorizeConnectionManagement(ctx, workspace, user, id); err != nil {
		return err
	}
	_, credential, err := s.Credential(ctx, workspace, user, id, false)
	if err != nil {
		return err
	}
	runs, err := s.boundConnectionRuns(ctx, workspace, user, id)
	if err != nil {
		return err
	}
	for _, run := range runs {
		runtimeID, ok := agentRuntimeRunID(&run)
		if !ok {
			continue
		}
		var input model.AgentRunInputPayload
		if err := decodeAIConnectionRunInput(run.Input, &input); err != nil {
			return err
		}
		if err := s.authorizeAcceptedSelectionOwner(ctx, &run, input.AISelection); err != nil {
			if errors.Is(err, ErrAIConnection) {
				continue
			}
			return err
		}
		if err := s.runtime.UpdateRunModelCredential(ctx, runtimeID, *credential); err != nil {
			current, lookupErr := s.runtime.GetRun(ctx, runtimeID)
			if lookupErr == nil && current != nil && !model.IsAgentRunActiveStatus(current.Status) {
				continue
			}
			return err
		}
		if run.Status == "paused" && run.PauseReason == model.AgentRunPauseReasonAuthentication {
			interactions, err := s.runtime.ListInteractions(ctx, runtimeID)
			if err != nil {
				return err
			}
			matches := false
			for _, interaction := range interactions {
				if interaction.InteractionKind != "authentication" || interaction.Status != "pending" {
					continue
				}
				var payload struct {
					ConnectionID string `json:"connection_id"`
				}
				if json.Unmarshal(interaction.RequestPayload, &payload) == nil && payload.ConnectionID == id {
					matches = true
				}
			}
			if !matches {
				continue
			}
			if _, err := s.runtime.ResumeRun(ctx, runtimeID, AgentRuntimeResumeRunRequest{Intent: "auth_completed", ExternalActorID: user}); err != nil {
				return err
			}
		}
	}
	return nil
}
