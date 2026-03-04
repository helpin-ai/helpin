package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// PMExternalLinkService contains external link business logic.
type PMExternalLinkService struct {
	repo *repository.PMExternalLinkRepository
}

// NewPMExternalLinkService creates a new PMExternalLinkService.
func NewPMExternalLinkService(repo *repository.PMExternalLinkRepository) *PMExternalLinkService {
	return &PMExternalLinkService{repo: repo}
}

// List returns external links for a story.
func (s *PMExternalLinkService) List(ctx context.Context, storyID string) ([]model.PMExternalLink, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	return s.repo.List(ctx, storyID)
}

// Create creates an external link, auto-deriving title from URL hostname if not provided.
func (s *PMExternalLinkService) Create(ctx context.Context, storyID string, req model.CreateExternalLinkRequest, userID string) (*model.PMExternalLink, error) {
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
		StoryID:     storyID,
		Title:       title,
		URL:         rawURL,
		CreatedByID: userID,
	}
	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// Update updates an external link.
func (s *PMExternalLinkService) Update(ctx context.Context, id string, req model.UpdateExternalLinkRequest) (*model.PMExternalLink, error) {
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
	return link, nil
}

// Delete deletes an external link.
func (s *PMExternalLinkService) Delete(ctx context.Context, id string) error {
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("external link not found")
	}
	return s.repo.Delete(ctx, id)
}

// deriveTitle extracts hostname from a URL for use as the title.
func deriveTitle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return parsed.Host
}
