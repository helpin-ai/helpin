package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/docsimport"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

const shortcutDocsSourceSystem = "shortcut"

func (s *PMImportService) importShortcutDocs(ctx context.Context, client *ShortcutAPIClient, workspaceID, actorID string, docs []shortcutAPIDocSlim, options model.ShortcutImportOptions) (int, int, []string) {
	if client == nil {
		return 0, 0, []string{"Shortcut docs were requested but the API client is not configured"}
	}
	if s.docsDocumentSvc == nil || s.docsContentSvc == nil {
		return 0, 0, []string{"Shortcut docs were requested but Helpin Docs import is not configured"}
	}
	spaceID := strings.TrimSpace(options.DocsSpaceID)
	if spaceID == "" {
		return 0, 0, []string{"Shortcut docs were skipped because no Helpin Docs space was selected"}
	}
	var collectionID *string
	if trimmed := strings.TrimSpace(options.DocsCollectionID); trimmed != "" {
		collectionID = &trimmed
	}
	var updatedCutoff *time.Time
	if options.DocsLookbackMonths > 0 {
		cutoff := time.Now().UTC().AddDate(0, -options.DocsLookbackMonths, 0)
		updatedCutoff = &cutoff
	}

	created := 0
	skipped := 0
	warnings := []string{}
	for _, slim := range docs {
		publicID := strings.TrimSpace(slim.ID)
		title := strings.TrimSpace(slim.Title)
		if publicID == "" {
			skipped++
			continue
		}
		if existingID, err := s.findImportedShortcutDocID(ctx, workspaceID, publicID); err == nil && existingID != "" {
			skipped++
			continue
		} else if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q duplicate check failed: %v", fallbackName(title, publicID), err)})
			skipped++
			continue
		}
		doc, err := client.GetDoc(ctx, publicID)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q could not be fetched: %v", fallbackName(title, publicID), err)})
			skipped++
			continue
		}
		if doc.Archived && !options.ImportArchived {
			skipped++
			continue
		}
		if updatedCutoff != nil {
			if updatedAt := parseShortcutAPITimestamp(doc.UpdatedAt); updatedAt != nil && updatedAt.Before(*updatedCutoff) {
				skipped++
				continue
			}
		}
		if strings.TrimSpace(doc.Title) != "" {
			title = strings.TrimSpace(doc.Title)
		}
		if title == "" {
			title = "Shortcut Doc " + publicID
		}
		content, sourceHTML, convertWarnings, err := shortcutDocToTiptapContent(doc)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q could not be converted: %v", title, err)})
			skipped++
			continue
		}
		for _, warning := range convertWarnings {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q conversion warning: %s", title, warning.Message)})
		}

		createdDoc, err := s.docsDocumentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{
			SpaceID:      spaceID,
			CollectionID: collectionID,
			Title:        title,
		}, actorID)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q could not be created: %v", title, err)})
			skipped++
			continue
		}
		savedContent, err := s.docsContentSvc.Save(ctx, createdDoc.ID, content, actorID)
		if err != nil {
			_ = s.db.WithContext(ctx).Where("id = ?", createdDoc.ID).Delete(&model.DocsDocument{}).Error
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q content could not be saved: %v", title, err)})
			skipped++
			continue
		}
		s.docsContentSvc.SetImportProvenance(ctx, savedContent.ID, sourceHTML, shortcutDocsSourceSystem, publicID)
		created++
	}
	return created, skipped, warnings
}

func filterShortcutDocsByScope(ctx context.Context, client *ShortcutAPIClient, docs []shortcutAPIDocSlim, options model.ShortcutImportOptions) ([]shortcutAPIDocSlim, []string) {
	if client == nil || len(docs) == 0 || (options.ImportArchived && options.DocsLookbackMonths <= 0) {
		return docs, nil
	}
	var updatedCutoff *time.Time
	if options.DocsLookbackMonths > 0 {
		cutoff := time.Now().UTC().AddDate(0, -options.DocsLookbackMonths, 0)
		updatedCutoff = &cutoff
	}
	filtered := make([]shortcutAPIDocSlim, 0, len(docs))
	warnings := []string{}
	for _, slim := range docs {
		publicID := strings.TrimSpace(slim.ID)
		if publicID == "" {
			continue
		}
		doc, err := client.GetDoc(ctx, publicID)
		if err != nil {
			warnings = appendUniqueWarnings(warnings, []string{fmt.Sprintf("Shortcut doc %q could not be checked for scope: %v", fallbackName(slim.Title, publicID), err)})
			continue
		}
		if doc.Archived && !options.ImportArchived {
			continue
		}
		if updatedCutoff != nil {
			if updatedAt := parseShortcutAPITimestamp(doc.UpdatedAt); updatedAt != nil && updatedAt.Before(*updatedCutoff) {
				continue
			}
		}
		filtered = append(filtered, slim)
	}
	return filtered, warnings
}

func (s *PMImportService) findImportedShortcutDocID(ctx context.Context, workspaceID, shortcutDocID string) (string, error) {
	var row struct {
		ID string
	}
	err := s.db.WithContext(ctx).
		Table("docs_documents d").
		Select("d.id").
		Joins("JOIN docs_contents c ON c.document_id = d.id").
		Where("d.workspace_id = ? AND d.deleted_at IS NULL AND c.import_source_system = ? AND c.import_source_object_id = ?", workspaceID, shortcutDocsSourceSystem, strings.TrimSpace(shortcutDocID)).
		Limit(1).
		Scan(&row).Error
	return row.ID, err
}

func shortcutDocToTiptapContent(doc *shortcutAPIDoc) (json.RawMessage, string, []docsimport.Warning, error) {
	if doc == nil {
		return nil, "", nil, fmt.Errorf("doc is nil")
	}
	if strings.TrimSpace(doc.ContentHTML) != "" {
		sourceHTML := doc.ContentHTML
		convResult, err := docsimport.ConvertHTML(sourceHTML)
		if err != nil {
			return nil, sourceHTML, nil, err
		}
		contentJSON, err := json.Marshal(convResult.Doc)
		if err != nil {
			return nil, sourceHTML, convResult.Warnings, err
		}
		return json.RawMessage(contentJSON), sourceHTML, convResult.Warnings, nil
	}
	markdown := strings.TrimSpace(doc.ContentMarkdown)
	if markdown == "" {
		markdown = ""
	}
	return tiptap.MarkdownToJSON(markdown), "", nil, nil
}
