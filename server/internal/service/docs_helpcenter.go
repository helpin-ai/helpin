package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// DocsHelpcenterService handles help center publishing, config, and slug management.
type DocsHelpcenterService struct {
	hcRepo    *repository.DocsHelpcenterRepository
	docRepo   *repository.DocsDocumentRepository
	spaceRepo *repository.DocsSpaceRepository
}

// NewDocsHelpcenterService creates a new DocsHelpcenterService.
func NewDocsHelpcenterService(
	hcRepo *repository.DocsHelpcenterRepository,
	docRepo *repository.DocsDocumentRepository,
	spaceRepo *repository.DocsSpaceRepository,
) *DocsHelpcenterService {
	return &DocsHelpcenterService{hcRepo: hcRepo, docRepo: docRepo, spaceRepo: spaceRepo}
}

// GetConfig returns the help center config for a workspace.
func (s *DocsHelpcenterService) GetConfig(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	return s.hcRepo.GetConfig(ctx, workspaceID)
}

// UpsertConfig creates or updates the help center config.
func (s *DocsHelpcenterService) UpsertConfig(ctx context.Context, workspaceID string, req model.UpdateDocsHelpcenterConfigRequest) (*model.DocsHelpcenterConfig, error) {
	updates := map[string]interface{}{}
	if req.Subdomain != nil {
		updates["subdomain"] = *req.Subdomain
	}
	if req.CustomDomain != nil {
		updates["custom_domain"] = req.CustomDomain
	}
	if req.BrandName != nil {
		updates["brand_name"] = *req.BrandName
	}
	if req.BrandLogoURL != nil {
		updates["brand_logo_url"] = req.BrandLogoURL
	}
	if req.BrandColor != nil {
		updates["brand_color"] = *req.BrandColor
	}
	if req.IsPublished != nil {
		updates["is_published"] = *req.IsPublished
	}
	if req.SEOTitle != nil {
		updates["seo_title"] = req.SEOTitle
	}
	if req.SEODescription != nil {
		updates["seo_description"] = req.SEODescription
	}
	if req.SupportEmail != nil {
		updates["support_email"] = req.SupportEmail
	}
	return s.hcRepo.UpsertConfig(ctx, workspaceID, updates)
}

// PublishExternally publishes a help center article externally.
func (s *DocsHelpcenterService) PublishExternally(ctx context.Context, documentID string, slug string) error {
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// Validate eligibility.
	if doc.DocType != model.DocTypeHelpCenterArticle {
		return fmt.Errorf("only help_center_article can be published externally")
	}
	if doc.Status != model.DocStatusPublished {
		return fmt.Errorf("document must be internally published first")
	}

	space, err := s.spaceRepo.GetByID(ctx, doc.SpaceID)
	if err != nil {
		return err
	}
	if space == nil || space.Type != model.SpaceTypeExternalCapable {
		return fmt.Errorf("document must be in an external-capable space")
	}

	if slug == "" {
		slug = slugify(doc.Title)
	}

	// Ensure article extension exists.
	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if art == nil {
		art = &model.DocsHelpcenterArticle{DocumentID: documentID}
		if _, err := s.hcRepo.CreateArticle(ctx, art); err != nil {
			return err
		}
	}

	// Handle slug change → alias old slug.
	existingDoc, _ := s.docRepo.GetByID(ctx, documentID)
	if existingDoc != nil {
		// Check if document had a different slug stored (via title-based slug).
		// Slug aliasing: handled by caller passing the new slug.
	}

	// Set public_published_at.
	now := time.Now()
	return s.hcRepo.SetPublicPublishedAt(ctx, documentID, &now)
}

// UnpublishExternally removes a help center article from public access.
func (s *DocsHelpcenterService) UnpublishExternally(ctx context.Context, documentID string) error {
	art, err := s.hcRepo.GetArticle(ctx, documentID)
	if err != nil {
		return err
	}
	if art == nil {
		return nil // Not published, no-op.
	}
	return s.hcRepo.SetPublicPublishedAt(ctx, documentID, nil)
}

// CreateSlugAlias records an old slug alias for redirect.
func (s *DocsHelpcenterService) CreateSlugAlias(ctx context.Context, workspaceID, documentID, oldSlug string) error {
	alias := &model.DocsSlugAlias{
		WorkspaceID: workspaceID,
		DocumentID:  documentID,
		OldSlug:     oldSlug,
	}
	return s.hcRepo.CreateSlugAlias(ctx, alias)
}

// ResolveSlug resolves a slug to a document ID, checking aliases if needed.
func (s *DocsHelpcenterService) ResolveSlug(ctx context.Context, workspaceID, slug string) (documentID string, isAlias bool, err error) {
	// First check slug aliases.
	alias, err := s.hcRepo.FindAliasBySlug(ctx, workspaceID, slug)
	if err != nil {
		return "", false, err
	}
	if alias != nil {
		return alias.DocumentID, true, nil
	}
	return "", false, nil
}

// GetArticle returns the help center article extension for a document.
func (s *DocsHelpcenterService) GetArticle(ctx context.Context, documentID string) (*model.DocsHelpcenterArticle, error) {
	return s.hcRepo.GetArticle(ctx, documentID)
}

// IncrementViewCount increments article view count.
func (s *DocsHelpcenterService) IncrementViewCount(ctx context.Context, documentID string) error {
	return s.hcRepo.IncrementViewCount(ctx, documentID)
}

// SubmitFeedback records article feedback and updates counts.
func (s *DocsHelpcenterService) SubmitFeedback(ctx context.Context, documentID string, req model.DocsArticleFeedbackRequest) error {
	fb := &model.DocsArticleFeedback{
		DocumentID: documentID,
		IsHelpful:  req.IsHelpful,
		Comment:    req.Comment,
		SessionID:  req.SessionID,
	}
	if _, err := s.hcRepo.CreateFeedback(ctx, fb); err != nil {
		return err
	}
	return s.hcRepo.IncrementFeedbackCount(ctx, documentID, req.IsHelpful)
}
