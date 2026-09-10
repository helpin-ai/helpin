package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// ErrDocsInvalidContent identifies TipTap JSON that cannot be safely loaded by
// the document editor.
var ErrDocsInvalidContent = errors.New("invalid document content")

// DocsContentService handles business logic for document content.
type DocsContentService struct {
	contentRepo         *repository.DocsContentRepository
	docRepo             *repository.DocsDocumentRepository
	translationSvc      *DocsHelpcenterTranslationService
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	workspaceRepo       *repository.WorkspaceRepository
	agentService        *AgentService
}

// NewDocsContentService creates a new DocsContentService.
func NewDocsContentService(contentRepo *repository.DocsContentRepository, docRepo *repository.DocsDocumentRepository, wsPublisher *websocket.Publisher) *DocsContentService {
	return &DocsContentService{contentRepo: contentRepo, docRepo: docRepo, wsPublisher: wsPublisher}
}

func (s *DocsContentService) SetTranslationService(translationSvc *DocsHelpcenterTranslationService) {
	s.translationSvc = translationSvc
}

func (s *DocsContentService) SetMentionNotificationDependencies(notificationService *NotificationService, workspaceRepo *repository.WorkspaceRepository) {
	s.notificationService = notificationService
	s.workspaceRepo = workspaceRepo
}

func (s *DocsContentService) SetAgentMentionDependencies(agentService *AgentService) {
	s.agentService = agentService
}

// Get returns the content for a document.
func (s *DocsContentService) Get(ctx context.Context, documentID string) (*model.DocsContent, error) {
	return s.contentRepo.GetByDocumentID(ctx, documentID)
}

// Save creates or updates document content.
// Automatically extracts content_text and computes word_count in the repository layer.
func (s *DocsContentService) Save(ctx context.Context, documentID string, content json.RawMessage, actorID string) (*model.DocsContent, error) {
	var previousText string
	if s.notificationService != nil && s.workspaceRepo != nil && actorID != "" {
		if existing, err := s.contentRepo.GetByDocumentID(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to load previous docs content before mention diff", "document_id", documentID, "error", err)
		} else if existing != nil {
			previousText = existing.ContentText
		}
	}

	content = s.restoreImportedToggleAttrs(ctx, documentID, content)
	if !isMarkdownSourceEnvelope(content) {
		if err := tiptap.ValidateDocument(content); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDocsInvalidContent, err)
		}
	}

	saved, err := s.contentRepo.UpsertWithActor(ctx, documentID, content, actorID)
	if err != nil {
		return nil, err
	}
	s.afterContentSave(ctx, documentID, actorID, previousText, saved)
	return saved, nil
}

// SaveVersioned replaces content only if the snapshot still matches.
func (s *DocsContentService) SaveVersioned(ctx context.Context, documentID string, content json.RawMessage, actorID, version string) (*model.DocsContent, error) {
	if version == "" {
		return s.Save(ctx, documentID, content, actorID)
	}
	content = s.restoreImportedToggleAttrs(ctx, documentID, content)
	return s.mutateContent(ctx, documentID, actorID, version, func(json.RawMessage) (json.RawMessage, error) { return content, nil })
}

func (s *DocsContentService) mutateContent(ctx context.Context, documentID, actorID, version string, transform func(json.RawMessage) (json.RawMessage, error)) (*model.DocsContent, error) {
	before, saved, err := s.contentRepo.MutateWithActor(ctx, documentID, version, actorID, func(raw json.RawMessage) (json.RawMessage, error) {
		next, err := transform(raw)
		if err != nil {
			return nil, err
		}
		if err := tiptap.ValidateDocument(next); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDocsInvalidContent, err)
		}
		return next, nil
	})
	if err != nil {
		return nil, err
	}
	s.afterContentSave(ctx, documentID, actorID, before.ContentText, saved)
	return saved, nil
}

func (s *DocsContentService) afterContentSave(ctx context.Context, documentID, actorID, previousText string, saved *model.DocsContent) {
	if s.translationSvc != nil {
		if err := s.translationSvc.RefreshArticleSource(ctx, documentID); err != nil {
			slog.WarnContext(ctx, "failed to refresh helpcenter article translation source after content save", "document_id", documentID, "error", err)
		}
	}
	if s.docRepo != nil {
		if doc, err := s.docRepo.GetByID(ctx, documentID); err == nil && doc != nil {
			publishWorkspaceEvent(s.wsPublisher, "updated", "docs_document", documentID, doc.WorkspaceID, actorID)
			s.emitNewMentionNotifications(ctx, doc, previousText, saved.ContentText, actorID)
			s.startNewAgentMentionRuns(ctx, doc, previousText, saved.ContentText, actorID)
		}
	}
}

func isMarkdownSourceEnvelope(content json.RawMessage) bool {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(content, &envelope); err != nil {
		return false
	}
	_, ok := envelope["_markdown_source"]
	return ok
}

func (s *DocsContentService) emitNewMentionNotifications(ctx context.Context, doc *model.DocsDocument, previousText, currentText, actorID string) {
	if s.notificationService == nil || s.workspaceRepo == nil || doc == nil || actorID == "" {
		return
	}

	addedMentions := diffMentionHandles(extractMentions(previousText), extractMentions(currentText))
	if len(addedMentions) == 0 {
		return
	}
	readableTeamIDs := s.readableTeamIDsForDoc(ctx, doc)

	if _, err := emitMentionNotificationForHandles(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
		WorkspaceID:      doc.WorkspaceID,
		ActorID:          actorID,
		Body:             currentText,
		EventType:        "doc.mention",
		EntityType:       "doc",
		EntityID:         doc.ID,
		Title:            "mentioned you in " + doc.Title,
		TeamID:           derefString(doc.TeamID),
		ReadableTeamIDs:  readableTeamIDs,
		EntitySnapshot:   model.JSONB{"title": doc.Title},
		NotificationBody: truncate(currentText, 200),
	}, addedMentions); err != nil {
		slog.ErrorContext(ctx, "failed to emit docs mention notification", "error", err, "document_id", doc.ID)
	}
}

func (s *DocsContentService) readableTeamIDsForDoc(ctx context.Context, doc *model.DocsDocument) []string {
	if doc == nil {
		return nil
	}
	if scoped := mentionScopeForTeamID(doc.TeamID); len(scoped) > 0 {
		return scoped
	}
	if s.workspaceRepo == nil {
		return nil
	}
	teams, err := s.workspaceRepo.ListTeams(ctx, doc.WorkspaceID)
	if err != nil {
		slog.WarnContext(ctx, "failed to load docs mention team scope", "workspace_id", doc.WorkspaceID, "document_id", doc.ID, "error", err)
		return nil
	}
	teamIDs := make([]string, 0, len(teams))
	for _, team := range teams {
		if team.ID != "" {
			teamIDs = append(teamIDs, team.ID)
		}
	}
	return teamIDs
}

func (s *DocsContentService) startNewAgentMentionRuns(ctx context.Context, doc *model.DocsDocument, previousText, currentText, actorID string) {
	if s.agentService == nil || doc == nil || actorID == "" {
		return
	}
	addedMentions := diffMentionHandles(extractMentions(previousText), extractMentions(currentText))
	if len(addedMentions) == 0 {
		return
	}
	mentionSet := make(map[string]struct{}, len(addedMentions))
	for _, mention := range addedMentions {
		if mention != "" {
			mentionSet[strings.ToLower(mention)] = struct{}{}
		}
	}
	if len(mentionSet) == 0 {
		return
	}

	agents, err := s.agentService.ListAgents(ctx, doc.WorkspaceID)
	if err != nil {
		slog.WarnContext(ctx, "failed to resolve docs agent mentions", "workspace_id", doc.WorkspaceID, "document_id", doc.ID, "error", err)
		return
	}
	for _, agent := range agents {
		if !agentMentionAllowsTarget(agent.AllowedTargets, "document") {
			continue
		}
		matched := false
		for _, handle := range mentionHandleVariants(agent.Name) {
			if _, ok := mentionSet[handle]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		contextText := "You were mentioned in the document \"" + doc.Title + "\". Review the document and respond with the next useful action."
		if _, err := s.agentService.StartTargetRun(ctx, doc.WorkspaceID, "document", doc.ID, model.StartAgentRunRequest{
			AgentID:           agent.ID,
			AdditionalContext: &contextText,
		}, actorID); err != nil {
			slog.WarnContext(ctx, "failed to start docs agent mention run", "workspace_id", doc.WorkspaceID, "document_id", doc.ID, "agent_id", agent.ID, "error", err)
			continue
		}
		slog.InfoContext(ctx, "started docs agent mention run", "workspace_id", doc.WorkspaceID, "document_id", doc.ID, "agent_id", agent.ID)
	}
}

func agentMentionAllowsTarget(raw json.RawMessage, target string) bool {
	for _, allowed := range parseJSONStringSlice(raw) {
		if allowed == target {
			return true
		}
	}
	return false
}

// ListBySpaceWithImportHTML returns content records that have stored import HTML for a space.
func (s *DocsContentService) ListBySpaceWithImportHTML(ctx context.Context, spaceID string) ([]model.DocsContent, error) {
	return s.contentRepo.ListBySpaceWithImportHTML(ctx, spaceID)
}

// SetImportProvenance stores the original import HTML and source metadata on a content record.
// This is a snapshot for reconversion/debugging — not the live source of truth.
func (s *DocsContentService) SetImportProvenance(ctx context.Context, contentID, sourceHTML, sourceSystem, sourceObjectID string) error {
	return s.contentRepo.UpdateImportProvenance(ctx, contentID, sourceHTML, sourceSystem, sourceObjectID)
}
