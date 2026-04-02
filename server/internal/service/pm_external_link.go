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
	repo        *repository.PMExternalLinkRepository
	wsPublisher *websocket.Publisher
}

// NewPMExternalLinkService creates a new PMExternalLinkService.
func NewPMExternalLinkService(repo *repository.PMExternalLinkRepository, wsPublisher *websocket.Publisher) *PMExternalLinkService {
	return &PMExternalLinkService{repo: repo, wsPublisher: wsPublisher}
}

// List returns external links for a story.
func (s *PMExternalLinkService) List(ctx context.Context, storyID string) ([]model.PMExternalLink, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	return s.repo.List(ctx, storyID)
}

// Create creates an external link, auto-deriving title from URL hostname if not provided.
func (s *PMExternalLinkService) Create(ctx context.Context, storyID string, req model.CreateExternalLinkRequest, userID, workspaceID string) (*model.PMExternalLink, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = deriveTitle(rawURL)
	}

	link := &model.PMExternalLink{
		TaskID:      storyID,
		Title:       title,
		URL:         rawURL,
		CreatedByID: userID,
	}
	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "external_link", EntityID: link.ID, WorkspaceID: workspaceID, ActorID: userID, ParentType: "task", ParentID: storyID})
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
		link.URL = rawURL
		// Re-derive title if URL changed and no explicit title given.
		if req.Title == nil {
			link.Title = deriveTitle(rawURL)
		}
	}
	if req.Title != nil {
		link.Title = strings.TrimSpace(*req.Title)
	}

	if err := s.repo.Update(ctx, link); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: link.TaskID})
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
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "external_link", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: link.TaskID})
	return nil
}

// deriveTitle extracts hostname from a URL for use as the title.
func deriveTitle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return parsed.Host
}
