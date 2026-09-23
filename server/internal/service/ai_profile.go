package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AIProfileService manages workspace-scoped profiles and resolves their routes.
type AIProfileService struct {
	checkRuntimeReadiness bool
	repo                  *repository.AIProfileRepository
	connections           *AIConnectionService
	admissionPolicy       AIConnectionAdmissionPolicy
	chatClients           AIChatClientFactory
}

func NewAIProfileService(repo *repository.AIProfileRepository, connections *AIConnectionService) *AIProfileService {
	return &AIProfileService{repo: repo, connections: connections}
}

func (s *AIProfileService) requireMember(ctx context.Context, workspace, user string) error {
	if s == nil || !s.connections.Enabled() || user == "" {
		return ErrAIConnection
	}
	active, err := s.connections.repo.ActiveMember(ctx, workspace, user)
	if err != nil || !active {
		return ErrAIConnection
	}
	return nil
}

func (s *AIProfileService) List(ctx context.Context, workspace, user string) ([]model.AIProfile, error) {
	if err := s.requireMember(ctx, workspace, user); err != nil {
		return nil, err
	}
	profiles, err := s.repo.List(ctx, workspace, user)
	if err != nil {
		return nil, err
	}
	visible, err := s.connections.List(ctx, workspace, user)
	if err != nil {
		return nil, err
	}
	connections := make(map[string]model.AIConnection, len(visible))
	for _, c := range visible {
		connections[c.ID] = c
	}
	for i := range profiles {
		p := &profiles[i]
		p.PrimaryPolicy = profileRoutePolicy(p.Primary, p.Scope, connections)
		if p.Fallback != nil {
			p.FallbackPolicy = profileRoutePolicy(*p.Fallback, p.Scope, connections)
		}
	}
	return profiles, nil
}

func (s *AIProfileService) authorizeProfile(ctx context.Context, workspace, user string, p *model.AIProfile, manage bool) error {
	if p == nil || p.WorkspaceID != workspace {
		return ErrAIConnection
	}
	if p.Scope == "personal" {
		if user == "" || derefString(p.UserID) != user {
			return ErrAIConnection
		}
		return s.requireMember(ctx, workspace, user)
	}
	if p.Scope != "workspace" || p.UserID != nil {
		return ErrAIConnection
	}
	if manage {
		return s.connections.requireConnectionManager(ctx, workspace, user)
	}
	return nil
}

func (s *AIProfileService) validateRoute(ctx context.Context, workspace, user, scope string, route *model.AIProfileRoute) error {
	if route.ConnectionID == "" {
		return errors.New("select an AI connection")
	}
	c, err := s.connections.repo.Get(ctx, route.ConnectionID)
	if err != nil {
		return err
	}
	if c == nil || c.WorkspaceID != workspace || c.SupersededBy != nil {
		return ErrAIConnection
	}
	if scope == "workspace" && (c.Scope != "workspace" || c.UserID != nil) {
		return errors.New("shared profiles require shared connections")
	}
	if err := accessAIConnection(c, workspace, user); err != nil {
		return err
	}
	if route.Model.Provider != c.Provider {
		return errors.New("profile provider must match its connection")
	}
	if route.Model.Endpoint != nil && !sameModelEndpoint(route.Model.Endpoint, c.Endpoint) {
		return errors.New("profile endpoint must match its connection")
	}
	route.Model.Endpoint = c.Endpoint
	route.Model.Model = strings.TrimSpace(route.Model.Model)
	if route.Model.Controls == nil {
		route.Model.Controls = &sdk.ModelControls{}
	}
	// Custom model names are valid without a commercial catalog entry.
	return sdk.ValidateRunModel(&route.Model)
}

// validatePrimaryRoute keeps personal profile primaries owner-bound. Fallbacks
// use validateRoute and may reference an authorized shared connection.
func (s *AIProfileService) validatePrimaryRoute(ctx context.Context, workspace, user, scope string, route *model.AIProfileRoute) error {
	if err := s.validateRoute(ctx, workspace, user, scope, route); err != nil {
		return err
	}
	if scope == "personal" {
		c, err := s.connections.repo.Get(ctx, route.ConnectionID)
		if err != nil {
			return err
		}
		if c == nil || c.Scope != "personal" || derefString(c.UserID) != user {
			return errors.New("personal profiles require your personal primary connection")
		}
	}
	return nil
}

func (s *AIProfileService) Save(ctx context.Context, workspace, user, id string, req model.SaveAIProfileRequest) (*model.AIProfile, error) {
	if err := s.requireMember(ctx, workspace, user); err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		return nil, errors.New("profile name is required and must not exceed 100 characters")
	}
	if req.Scope != "personal" && req.Scope != "workspace" {
		return nil, errors.New("unsupported profile scope")
	}
	p := &model.AIProfile{ID: uuid.NewString(), WorkspaceID: workspace, Scope: req.Scope, Revision: 1}
	if req.Scope == "personal" {
		p.UserID = &user
	}
	if id != "" {
		var err error
		p, err = s.repo.Get(ctx, workspace, id)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, ErrAIConnection
		}
		if p.Scope != req.Scope {
			return nil, errors.New("profile ownership cannot be changed")
		}
		if p.Revision != req.Revision {
			return nil, repository.ErrAIProfileChanged
		}
	}
	if err := s.authorizeProfile(ctx, workspace, user, p, true); err != nil {
		return nil, err
	}
	if err := s.validatePrimaryRoute(ctx, workspace, user, p.Scope, &req.Primary); err != nil {
		return nil, err
	}
	if req.Fallback != nil {
		if err := s.validateRoute(ctx, workspace, user, p.Scope, req.Fallback); err != nil {
			return nil, err
		}
		if req.Fallback.ConnectionID == req.Primary.ConnectionID {
			return nil, errors.New("fallback requires a different connection")
		}
	}
	p.Name, p.Primary, p.Fallback = req.Name, req.Primary, req.Fallback
	p.UpdatedAt = time.Now().UTC()
	var err error
	if id == "" {
		err = s.repo.Create(ctx, p)
	} else {
		p.Revision++
		err = s.repo.Update(ctx, p, req.Revision)
	}
	return p, err
}

func (s *AIProfileService) Delete(ctx context.Context, workspace, user, id string, revision int64) error {
	if err := s.requireMember(ctx, workspace, user); err != nil {
		return err
	}
	p, err := s.repo.Get(ctx, workspace, id)
	if err != nil {
		return err
	}
	if err := s.authorizeProfile(ctx, workspace, user, p, true); err != nil {
		return err
	}
	return s.repo.Delete(ctx, workspace, id, revision)
}

func (s *AIProfileService) Settings(ctx context.Context, workspace, user string) (*model.AIWorkspaceSettings, error) {
	if err := s.requireMember(ctx, workspace, user); err != nil {
		return nil, err
	}
	return s.repo.Settings(ctx, workspace)
}

func (s *AIProfileService) SetDefault(ctx context.Context, workspace, user, id string) error {
	if err := s.connections.requireConnectionManager(ctx, workspace, user); err != nil {
		return err
	}
	var p *model.AIProfile
	if id != "" {
		var err error
		p, err = s.repo.Get(ctx, workspace, id)
		if err != nil {
			return err
		}
		if p == nil || p.Scope != "workspace" {
			return errors.New("workspace default requires a shared profile")
		}
	}
	return s.repo.SetDefault(ctx, workspace, p)
}
