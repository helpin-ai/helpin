package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// proposalVersionSnapshotter is the small consumer interface used by
// DocsChangeProposalService to record an applied proposal in version history.
// Defined here (where it is consumed) so tests can inject fakes without
// pulling in the full DocsVersionService.
type proposalVersionSnapshotter interface {
	SnapshotOnProposalApply(ctx context.Context, documentID, userID string, proposal *model.DocsChangeProposal) (*model.DocsVersion, error)
}

type DocsChangeProposalService struct {
	proposalRepo *repository.DocsChangeProposalRepository
	docRepo      *repository.DocsDocumentRepository
	contentSvc   *DocsContentService
	blockSvc     *DocsBlockService
	versionSvc   proposalVersionSnapshotter
	wsPublisher  *websocket.Publisher
}

func NewDocsChangeProposalService(proposalRepo *repository.DocsChangeProposalRepository, docRepo *repository.DocsDocumentRepository, contentSvc *DocsContentService, blockSvc *DocsBlockService, versionSvc proposalVersionSnapshotter, wsPublisher *websocket.Publisher) *DocsChangeProposalService {
	return &DocsChangeProposalService{
		proposalRepo: proposalRepo,
		docRepo:      docRepo,
		contentSvc:   contentSvc,
		blockSvc:     blockSvc,
		versionSvc:   versionSvc,
		wsPublisher:  wsPublisher,
	}
}

func (s *DocsChangeProposalService) Create(ctx context.Context, workspaceID string, req model.CreateDocsChangeProposalRequest) (*model.DocsChangeProposal, error) {
	if s == nil || s.proposalRepo == nil {
		return nil, fmt.Errorf("docs change proposal service is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	documentID := strings.TrimSpace(req.DocumentID)
	summary := strings.TrimSpace(req.Summary)
	contentMarkdown := strings.TrimSpace(req.ContentMarkdown)
	createdBy := strings.TrimSpace(req.CreatedBy)
	if workspaceID == "" || documentID == "" {
		return nil, fmt.Errorf("workspace_id and document_id are required")
	}
	switch scope {
	case "document", "block":
	default:
		return nil, fmt.Errorf("scope must be document or block")
	}
	if summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if contentMarkdown == "" {
		return nil, fmt.Errorf("content_markdown is required")
	}
	if len(req.Content) == 0 || strings.TrimSpace(string(req.Content)) == "" || strings.TrimSpace(string(req.Content)) == "null" {
		return nil, fmt.Errorf("content is required")
	}
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}
	var blockID *string
	if scope == "block" {
		trimmedBlockID := strings.TrimSpace(derefString(req.BlockID))
		if trimmedBlockID == "" {
			return nil, fmt.Errorf("block_id is required for block proposals")
		}
		if req.Revision <= 0 {
			return nil, fmt.Errorf("revision is required for block proposals")
		}
		blockID = &trimmedBlockID
	} else if req.BlockID != nil && strings.TrimSpace(*req.BlockID) != "" {
		trimmedBlockID := strings.TrimSpace(*req.BlockID)
		blockID = &trimmedBlockID
	}
	sources := req.Sources
	if len(sources) == 0 || strings.TrimSpace(string(sources)) == "" || strings.TrimSpace(string(sources)) == "null" {
		sources = json.RawMessage(`[]`)
	}
	proposal := &model.DocsChangeProposal{
		WorkspaceID:     workspaceID,
		DocumentID:      documentID,
		BlockID:         blockID,
		AgentID:         trimPtr(req.AgentID),
		AgentRunID:      trimPtr(req.AgentRunID),
		Scope:           scope,
		Status:          model.DocsChangeProposalStatusPending,
		Revision:        req.Revision,
		Summary:         summary,
		ContentMarkdown: contentMarkdown,
		Content:         append(json.RawMessage(nil), req.Content...),
		Sources:         append(json.RawMessage(nil), sources...),
		CreatedBy:       createdBy,
	}
	created, err := s.proposalRepo.Create(ctx, proposal)
	if err != nil {
		return nil, err
	}
	s.publishProposalEvent("created", created, createdBy)
	return created, nil
}

func (s *DocsChangeProposalService) ListPending(ctx context.Context, workspaceID, documentID string) ([]model.DocsChangeProposal, error) {
	if s == nil || s.proposalRepo == nil {
		return nil, fmt.Errorf("docs change proposal service is not configured")
	}
	doc, err := s.docRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if doc == nil || doc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("document not found")
	}
	return s.proposalRepo.ListPendingByDocument(ctx, workspaceID, documentID)
}

func (s *DocsChangeProposalService) Apply(ctx context.Context, workspaceID, documentID, proposalID, actorID string) (*model.DocsChangeProposal, *model.DocsContent, error) {
	proposal, err := s.loadPending(ctx, workspaceID, proposalID)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(proposal.DocumentID) != strings.TrimSpace(documentID) {
		return nil, nil, fmt.Errorf("proposal not found")
	}
	var content *model.DocsContent
	switch proposal.Scope {
	case "document":
		content, err = s.contentSvc.Save(ctx, proposal.DocumentID, proposal.Content, actorID)
	case "block":
		if proposal.BlockID == nil || strings.TrimSpace(*proposal.BlockID) == "" {
			return nil, nil, fmt.Errorf("block_id is required")
		}
		content, err = s.blockSvc.Patch(ctx, proposal.DocumentID, strings.TrimSpace(*proposal.BlockID), proposal.Revision, proposal.Content, actorID)
	default:
		err = fmt.Errorf("unsupported proposal scope %q", proposal.Scope)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := s.proposalRepo.Resolve(ctx, workspaceID, proposal.ID, model.DocsChangeProposalStatusApplied, actorID); err != nil {
		return nil, nil, err
	}
	proposal.Status = model.DocsChangeProposalStatusApplied
	proposal.ResolvedBy = &actorID
	// Record the applied change in version history. Failures here are
	// non-fatal: the proposal has already been applied and the content has
	// already been written, so we log and continue rather than unwinding.
	if s.versionSvc != nil {
		if _, err := s.versionSvc.SnapshotOnProposalApply(ctx, proposal.DocumentID, actorID, proposal); err != nil {
			slog.WarnContext(ctx, "snapshot on proposal apply failed",
				"error", err,
				"workspace_id", proposal.WorkspaceID,
				"document_id", proposal.DocumentID,
				"proposal_id", proposal.ID,
				"agent_id", derefString(proposal.AgentID),
				"agent_run_id", derefString(proposal.AgentRunID),
			)
		}
	}
	s.publishProposalEvent("updated", proposal, actorID)
	return proposal, content, nil
}

func (s *DocsChangeProposalService) Discard(ctx context.Context, workspaceID, documentID, proposalID, actorID string) (*model.DocsChangeProposal, error) {
	proposal, err := s.loadPending(ctx, workspaceID, proposalID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(proposal.DocumentID) != strings.TrimSpace(documentID) {
		return nil, fmt.Errorf("proposal not found")
	}
	if err := s.proposalRepo.Resolve(ctx, workspaceID, proposal.ID, model.DocsChangeProposalStatusDiscarded, actorID); err != nil {
		return nil, err
	}
	proposal.Status = model.DocsChangeProposalStatusDiscarded
	proposal.ResolvedBy = &actorID
	s.publishProposalEvent("updated", proposal, actorID)
	return proposal, nil
}

func (s *DocsChangeProposalService) loadPending(ctx context.Context, workspaceID, proposalID string) (*model.DocsChangeProposal, error) {
	if s == nil || s.proposalRepo == nil {
		return nil, fmt.Errorf("docs change proposal service is not configured")
	}
	proposal, err := s.proposalRepo.GetByID(ctx, workspaceID, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, fmt.Errorf("proposal not found")
	}
	if proposal.Status != model.DocsChangeProposalStatusPending {
		return nil, fmt.Errorf("proposal is already resolved")
	}
	return proposal, nil
}

func (s *DocsChangeProposalService) publishProposalEvent(action string, proposal *model.DocsChangeProposal, actorID string) {
	if proposal == nil {
		return
	}
	publishWorkspaceEventWithParent(s.wsPublisher, action, "docs_change_proposal", proposal.ID, proposal.WorkspaceID, actorID, "docs_document", proposal.DocumentID, map[string]any{
		"document_id": proposal.DocumentID,
		"block_id":    derefString(proposal.BlockID),
		"status":      proposal.Status,
		"scope":       proposal.Scope,
	})
}
