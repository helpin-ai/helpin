package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMExternalLinkService contains external link business logic.
type PMExternalLinkService struct {
	repo             *repository.PMExternalLinkRepository
	wsPublisher      *websocket.Publisher
	metadataResolver externalLinkMetadataResolver
}

// NewPMExternalLinkService creates a new PMExternalLinkService.
func NewPMExternalLinkService(repo *repository.PMExternalLinkRepository, wsPublisher *websocket.Publisher) *PMExternalLinkService {
	return &PMExternalLinkService{repo: repo, wsPublisher: wsPublisher}
}

type externalLinkMetadataResolver interface {
	Resolve(ctx context.Context, rawURL string) (*DocsResolvedEmbed, error)
}

// SetMetadataResolver configures page metadata fetching for external links.
func (s *PMExternalLinkService) SetMetadataResolver(resolver externalLinkMetadataResolver) {
	s.metadataResolver = resolver
}

var allowedExternalLinkEntityTypes = map[string]bool{
	"task": true,
	"epic": true,
}

// List returns external links for a task.
func (s *PMExternalLinkService) List(ctx context.Context, taskID string) ([]model.PMExternalLink, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	return s.repo.List(ctx, taskID)
}

// Create creates an external link, auto-deriving title from URL hostname if not provided.
func (s *PMExternalLinkService) Create(ctx context.Context, taskID string, req model.CreateExternalLinkRequest, userID, workspaceID string) (*model.PMExternalLink, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	resolvedURL, title := s.resolveLinkMetadata(ctx, rawURL, req.Title)

	link := &model.PMExternalLink{
		TaskID:      &taskID,
		EntityType:  "task",
		EntityID:    taskID,
		Title:       title,
		URL:         resolvedURL,
		CreatedByID: userID,
	}
	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "external_link", EntityID: link.ID, WorkspaceID: workspaceID, ActorID: userID, ParentType: "task", ParentID: taskID})
	return link, nil
}

// Update updates an external link.
func (s *PMExternalLinkService) Update(ctx context.Context, id string, req model.UpdateExternalLinkRequest, workspaceID, actorID string) (*model.PMExternalLink, error) {
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, fmt.Errorf("external link not found")
	}

	if req.URL != nil {
		rawURL := strings.TrimSpace(*req.URL)
		if rawURL == "" {
			return nil, fmt.Errorf("url cannot be empty")
		}
		// Re-derive title if URL changed and no explicit title given.
		if req.Title == nil {
			link.URL, link.Title = s.resolveLinkMetadata(ctx, rawURL, "")
		} else {
			link.URL = rawURL
		}
	}
	if req.Title != nil {
		link.Title = strings.TrimSpace(*req.Title)
	}

	if err := s.repo.Update(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: link.EntityType, ParentID: link.EntityID})
	return link, nil
}

// Delete deletes an external link.
func (s *PMExternalLinkService) Delete(ctx context.Context, id string, workspaceID, actorID string) error {
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("external link not found")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: link.EntityType, ParentID: link.EntityID})
	return nil
}

// ListByEntity returns external links for any supported entity type.
func (s *PMExternalLinkService) ListByEntity(ctx context.Context, entityType, entityID string) ([]model.PMExternalLink, error) {
	if !allowedExternalLinkEntityTypes[entityType] {
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	return s.repo.ListByEntity(ctx, entityType, entityID)
}

// CreateForEntity creates an external link for any supported entity type.
func (s *PMExternalLinkService) CreateForEntity(ctx context.Context, entityType, entityID string, req model.CreateExternalLinkRequest, userID, workspaceID string) (*model.PMExternalLink, error) {
	if !allowedExternalLinkEntityTypes[entityType] {
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	resolvedURL, title := s.resolveLinkMetadata(ctx, rawURL, req.Title)

	link := &model.PMExternalLink{
		EntityType:  entityType,
		EntityID:    entityID,
		Title:       title,
		URL:         resolvedURL,
		CreatedByID: userID,
	}
	if entityType == "task" {
		link.TaskID = &entityID
	}

	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{
		Action: "created", Entity: "external_link", EntityID: link.ID,
		WorkspaceID: workspaceID, ActorID: userID,
		ParentType: entityType, ParentID: entityID,
	})
	return link, nil
}

func (s *PMExternalLinkService) resolveLinkMetadata(ctx context.Context, rawURL, providedTitle string) (string, string) {
	rawURL = strings.TrimSpace(rawURL)
	title := strings.TrimSpace(providedTitle)
	if title != "" {
		return rawURL, title
	}

	if s.metadataResolver != nil {
		if resolved, err := s.metadataResolver.Resolve(ctx, rawURL); err == nil && resolved != nil {
			resolvedURL := strings.TrimSpace(resolved.URL)
			resolvedTitle := strings.TrimSpace(resolved.Title)
			if resolvedURL == "" {
				resolvedURL = rawURL
			}
			if resolvedTitle != "" {
				return resolvedURL, resolvedTitle
			}
			return resolvedURL, deriveTitle(resolvedURL)
		}
	}

	return rawURL, deriveTitle(rawURL)
}

// deriveTitle extracts hostname from a URL for use as the title.
func deriveTitle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return parsed.Host
}
