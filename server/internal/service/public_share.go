package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrPublicShareNotFound avoids revealing whether a resource or token exists.
var ErrPublicShareNotFound = errors.New("public share not found")

type publicShareStore interface {
	CreateActive(ctx context.Context, workspaceID, resourceType, resourceID, actorID string) (*model.PublicShare, error)
	GetActiveByToken(ctx context.Context, token string) (*model.PublicShare, error)
	GetActiveByResource(ctx context.Context, workspaceID, resourceType, resourceID string) (*model.PublicShare, error)
	RevokeActive(ctx context.Context, workspaceID, resourceType, resourceID, actorID string) error
}

// GetLink returns the current public link after authorizing resource access.
func (s *PublicShareService) GetLink(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) (*model.PublicShareLink, error) {
	allowed, err := s.canAccess(ctx, workspaceID, actorID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrPublicShareNotFound
	}
	share, err := s.store.GetActiveByResource(ctx, workspaceID, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("get public share link: %w", err)
	}
	if share == nil {
		return nil, nil
	}
	return &model.PublicShareLink{Token: share.Token, URL: s.appBaseURL + "/shared/" + share.Token}, nil
}

type publicShareSource interface {
	CanAccessDockChat(ctx context.Context, workspaceID, actorID, chatID string) (bool, error)
	CanAccessAgentRun(ctx context.Context, workspaceID, actorID, runID string) (bool, error)
	PublicDockChat(ctx context.Context, workspaceID, chatID string) (*model.PublicSharedDockChat, error)
	PublicAgentRun(ctx context.Context, workspaceID, runID string) (*model.PublicSharedAgentRun, error)
}

// PublicShareService manages and resolves live public links.
type PublicShareService struct {
	store      publicShareStore
	source     publicShareSource
	appBaseURL string
}

// NewPublicShareService creates a PublicShareService.
func NewPublicShareService(store publicShareStore, source publicShareSource, appBaseURL string) *PublicShareService {
	return &PublicShareService{store: store, source: source, appBaseURL: strings.TrimRight(appBaseURL, "/")}
}

// Create authorizes access and returns one stable active public link.
func (s *PublicShareService) Create(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) (*model.PublicShareLink, error) {
	allowed, err := s.canAccess(ctx, workspaceID, actorID, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrPublicShareNotFound
	}
	share, err := s.store.CreateActive(ctx, workspaceID, resourceType, resourceID, actorID)
	if err != nil {
		return nil, fmt.Errorf("create public share: %w", err)
	}
	return &model.PublicShareLink{Token: share.Token, URL: s.appBaseURL + "/shared/" + share.Token}, nil
}

// Revoke authorizes access and disables the active public link.
func (s *PublicShareService) Revoke(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) error {
	allowed, err := s.canAccess(ctx, workspaceID, actorID, resourceType, resourceID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPublicShareNotFound
	}
	return s.store.RevokeActive(ctx, workspaceID, resourceType, resourceID, actorID)
}

// GetPublic resolves the latest safe projection for an active bearer token.
func (s *PublicShareService) GetPublic(ctx context.Context, token string) (*model.PublicSharedResource, error) {
	share, err := s.store.GetActiveByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, fmt.Errorf("get public share: %w", err)
	}
	if share == nil {
		return nil, ErrPublicShareNotFound
	}
	result := &model.PublicSharedResource{ResourceType: share.ResourceType}
	switch share.ResourceType {
	case model.PublicShareResourceDockChat:
		result.DockChat, err = s.source.PublicDockChat(ctx, share.WorkspaceID, share.ResourceID)
	case model.PublicShareResourceAgentRun:
		result.AgentRun, err = s.source.PublicAgentRun(ctx, share.WorkspaceID, share.ResourceID)
	default:
		return nil, ErrPublicShareNotFound
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PublicShareService) canAccess(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) (bool, error) {
	switch resourceType {
	case model.PublicShareResourceDockChat:
		return s.source.CanAccessDockChat(ctx, workspaceID, actorID, resourceID)
	case model.PublicShareResourceAgentRun:
		return s.source.CanAccessAgentRun(ctx, workspaceID, actorID, resourceID)
	default:
		return false, nil
	}
}
