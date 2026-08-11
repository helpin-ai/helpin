package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type FixDocumentFormattingResult struct {
	Content              *model.DocsContent `json:"content"`
	SourceSystem         string             `json:"source_system"`
	Republished          bool               `json:"republished"`
	Warnings             int                `json:"warnings"`
	HTMLBlockFallbacks   int                `json:"html_block_fallbacks"`
	NormalizedNoteBlocks int                `json:"normalized_note_blocks"`
}

// FixDocumentFormatting refreshes one supported imported article through the
// same source-specific conversion path used by its original import.
func (s *DocsImportService) FixDocumentFormatting(
	ctx context.Context,
	workspaceID, documentID, actorID string,
) (*FixDocumentFormattingResult, error) {
	doc, err := s.documentSvc.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}
	if doc.IsLocked {
		return nil, fmt.Errorf("unlock the document before fixing formatting")
	}
	if doc.Status == model.DocStatusArchived {
		return nil, fmt.Errorf("unarchive the document before fixing formatting")
	}

	content, err := s.contentSvc.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if content == nil || content.ImportSourceHTML == nil || strings.TrimSpace(*content.ImportSourceHTML) == "" {
		return nil, fmt.Errorf("this document has no imported source to reformat")
	}
	sourceSystem := ""
	if content.ImportSourceSystem != nil {
		sourceSystem = strings.ToLower(strings.TrimSpace(*content.ImportSourceSystem))
	}
	if sourceSystem != "helpscout" && sourceSystem != "nextra" {
		return nil, fmt.Errorf("fix formatting is not available for this import source")
	}

	sourceHTML := *content.ImportSourceHTML
	sourceTitle := doc.Title
	sourceSlug := ""
	var payload json.RawMessage
	var warnings []docsimport.Warning

	switch sourceSystem {
	case "helpscout":
		payload, sourceHTML, sourceTitle, sourceSlug, warnings, err = s.latestHelpScoutFormatting(
			ctx, workspaceID, doc.SpaceID, doc.Title, content,
		)
		if err != nil {
			return nil, err
		}
	case "nextra":
		payload = nextraContentToTiptapJSON(sourceHTML)
	}

	saved, err := s.contentSvc.Save(ctx, documentID, payload, actorID)
	if err != nil {
		return nil, fmt.Errorf("save formatted article: %w", err)
	}
	sourceObjectID := ""
	if content.ImportSourceObjectID != nil {
		sourceObjectID = strings.TrimSpace(*content.ImportSourceObjectID)
	}
	if err := s.contentSvc.SetImportProvenance(
		ctx, saved.ID, sourceHTML, sourceSystem, sourceObjectID,
	); err != nil {
		return nil, fmt.Errorf("save refreshed import source: %w", err)
	}

	if sourceTitle != doc.Title {
		if _, err := s.documentSvc.Update(
			ctx, documentID, model.UpdateDocsDocumentRequest{Title: &sourceTitle},
		); err != nil {
			return nil, fmt.Errorf("update article title: %w", err)
		}
	}

	republished := false
	article, err := s.helpcenterSvc.GetArticle(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("load help center article: %w", err)
	}
	if article != nil {
		if sourceSlug != "" && sourceSlug != article.Slug {
			if err := s.helpcenterSvc.UpdateArticleSlug(ctx, workspaceID, documentID, sourceSlug); err != nil {
				return nil, fmt.Errorf("update article slug: %w", err)
			}
			article.Slug = sourceSlug
		}
		if article.PublicPublishedAt != nil {
			if err := s.helpcenterSvc.PublishExternally(ctx, documentID, article.Slug, nil); err != nil {
				return nil, fmt.Errorf("republish formatted article: %w", err)
			}
			republished = true
		}
	}

	stats := summarizeImportWarnings(warnings)
	return &FixDocumentFormattingResult{
		Content:              saved,
		SourceSystem:         sourceSystem,
		Republished:          republished,
		Warnings:             len(warnings),
		HTMLBlockFallbacks:   stats.HTMLBlockFallbacks,
		NormalizedNoteBlocks: stats.NormalizedNoteBlocks,
	}, nil
}

func (s *DocsImportService) latestHelpScoutFormatting(
	ctx context.Context,
	workspaceID, spaceID, fallbackTitle string,
	content *model.DocsContent,
) (json.RawMessage, string, string, string, []docsimport.Warning, error) {
	if content.ImportSourceObjectID == nil || strings.TrimSpace(*content.ImportSourceObjectID) == "" {
		return nil, "", "", "", nil, fmt.Errorf("this document has no Help Scout source ID")
	}
	sourceID := strings.TrimSpace(*content.ImportSourceObjectID)
	client, err := s.helpScoutClientForSpace(ctx, workspaceID, spaceID)
	if err != nil {
		return nil, "", "", "", nil, err
	}
	article, err := client.GetArticle(ctx, sourceID, false)
	if err != nil {
		return nil, "", "", "", nil, fmt.Errorf("fetch latest Help Scout article: %w", err)
	}

	sourceHTML := article.Text
	sourceTitle := strings.TrimSpace(article.Name)
	if sourceTitle == "" {
		sourceTitle = fallbackTitle
	}
	if s.s3Client != nil && sourceHTML != "" {
		processed, keptURLs, err := helpscout.ProcessImagesDetailed(
			ctx, sourceHTML, &s3ImageUploader{store: s.s3Client}, workspaceID,
		)
		if err != nil {
			return nil, "", "", "", nil, fmt.Errorf("process Help Scout images: %w", err)
		}
		sourceHTML = processed
		if len(keptURLs) > 0 {
			s.logger.WarnContext(ctx, "some Help Scout images could not be refreshed",
				"document_id", content.DocumentID,
				"count", len(keptURLs),
			)
		}
	}

	converted, warnings, err := s.convertHelpScoutHTML(
		ctx, workspaceID, sourceID, sourceTitle, sourceHTML,
	)
	if err != nil {
		return nil, "", "", "", nil, fmt.Errorf("format latest Help Scout article: %w", err)
	}
	payload, err := json.Marshal(converted.Doc)
	if err != nil {
		return nil, "", "", "", nil, fmt.Errorf("marshal formatted article: %w", err)
	}
	return payload, sourceHTML, sourceTitle, strings.TrimSpace(article.Slug), warnings, nil
}

func (s *DocsImportService) helpScoutClientForSpace(
	ctx context.Context,
	workspaceID, spaceID string,
) (*helpscout.Client, error) {
	if len(s.encryptionKey) != 32 {
		return nil, fmt.Errorf("Help Scout import credential is not configured")
	}
	jobs, err := s.importRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list Help Scout imports: %w", err)
	}
	for _, job := range jobs {
		if job.Source != "helpscout" || job.SpaceID == nil || *job.SpaceID != spaceID ||
			job.PayloadEncrypted == nil || strings.TrimSpace(*job.PayloadEncrypted) == "" {
			continue
		}
		raw, err := appcrypto.DecryptString(*job.PayloadEncrypted, s.encryptionKey)
		if err != nil {
			continue
		}
		var req model.DocsImportStartRequest
		if json.Unmarshal([]byte(raw), &req) == nil && strings.TrimSpace(req.APIKey) != "" {
			return helpscout.NewClient(req.APIKey), nil
		}
	}
	return nil, fmt.Errorf("no reusable Help Scout import credential was found for this space")
}
