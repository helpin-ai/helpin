package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Typed public MCP error codes for document change proposals.
const (
	MCPErrorCodeProposalNotFound = "PROPOSAL_NOT_FOUND"
	MCPErrorCodeProposalConflict = "PROPOSAL_CONFLICT"
)

// _mcpProposalCommand is the internal command behind propose_document_change.
// Agents use the same command inside runs; outside a run it records the
// connected user as the author.
const _mcpProposalCommand = "docs.publish_document_change_proposal"

type mcpDocsChangeProposalService interface {
	ListPending(context.Context, string, string) ([]model.DocsChangeProposal, error)
	Get(context.Context, string, string, string) (*model.DocsChangeProposal, error)
	Apply(context.Context, string, string, string, string) (*model.DocsChangeProposal, *model.DocsContent, error)
	Discard(context.Context, string, string, string, string) (*model.DocsChangeProposal, error)
	BaseChanged(context.Context, *model.DocsChangeProposal) bool
}

type mcpCommandExecutor interface {
	Execute(context.Context, model.InternalCommandContext, string, json.RawMessage) (json.RawMessage, error)
}

// SetDocsChangeProposals wires the Docs change proposal review service used
// by the proposal tools.
func (s *MCPService) SetDocsChangeProposals(proposals *DocsChangeProposalService) {
	if proposals != nil {
		s.docsProposals = proposals
	}
}

func docsProposalMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	id := map[string]any{"type": "string", "minLength": 1}
	docs := func(definition MCPToolDefinition) MCPToolDefinition {
		definition.Toolset, definition.Module = MCPToolsetDocs, model.ModuleDocs
		return definition
	}
	proposeSchema := cloneMCPSchema(commandDocumentChangeProposalSchema())
	if properties, ok := proposeSchema["properties"].(map[string]any); ok {
		properties["document_id"] = map[string]any{"type": "string", "minLength": 1, "description": "The document to change."}
	}
	return []MCPToolDefinition{
		docs(MCPToolDefinition{
			Name:  "propose_document_change",
			Title: "Propose document change",
			Description: "Submit a proposed replacement for a whole document or one block for human review in Helpin Docs. " +
				"The document is not changed until a reviewer applies the proposal. For block scope, pass the block_id " +
				"and revision from get_document_blocks.",
			InputSchema: withMCPIdempotencyKey(proposeSchema),
			Scope:       MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Mutating: true, IdempotentHint: true,
		}),
		docs(MCPToolDefinition{
			Name:        "list_document_change_proposals",
			Title:       "List document change proposals",
			Description: "List pending change proposals for a document, newest first, with the proposed and current markdown.",
			InputSchema: object(map[string]any{"document_id": id}, "document_id"),
			Scope:       MCPScopeDocsRead, Permission: authorization.PermDocsRead,
		}),
		docs(MCPToolDefinition{
			Name:  "apply_document_change_proposal",
			Title: "Apply document change proposal",
			Description: "Apply a pending change proposal as the reviewer. Fails with PROPOSAL_CONFLICT when the content " +
				"it replaces changed after it was proposed; set force to apply anyway (block proposals always need the current revision).",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{
				"document_id": id,
				"proposal_id": id,
				"force":       map[string]any{"type": "boolean", "default": false, "description": "Apply a document proposal even if the document changed since it was proposed."},
			}, "document_id", "proposal_id")),
			Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Mutating: true, IdempotentHint: true,
		}),
		docs(MCPToolDefinition{
			Name:        "discard_document_change_proposal",
			Title:       "Discard document change proposal",
			Description: "Discard a pending change proposal as the reviewer. The document is not changed.",
			InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": id, "proposal_id": id}, "document_id", "proposal_id")),
			Scope:       MCPScopeDocsWrite, Permission: authorization.PermDocsEdit,
			Mutating: true, Destructive: true, IdempotentHint: true,
		}),
	}
}

func (s *MCPService) executeDocsProposalMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, bool, error) {
	switch name {
	case "propose_document_change", "list_document_change_proposals",
		"apply_document_change_proposal", "discard_document_change_proposal":
	default:
		return nil, false, nil
	}
	if name == "propose_document_change" {
		result, err := s.proposeMCPDocumentChange(ctx, principal, actor, arguments)
		return result, true, err
	}
	if s.docsProposals == nil {
		return nil, true, ErrMCPForbidden
	}
	var input struct {
		DocumentID string `json:"document_id"`
		ProposalID string `json:"proposal_id"`
		Force      bool   `json:"force"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, true, err
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, strings.TrimSpace(input.DocumentID))
	if err != nil {
		return nil, true, err
	}
	if document == nil {
		return nil, true, ErrMCPNotFound
	}
	var result *MCPToolResult
	switch name {
	case "list_document_change_proposals":
		result, err = s.listMCPDocumentChangeProposals(ctx, principal, document)
	case "apply_document_change_proposal":
		result, err = s.applyMCPDocumentChangeProposal(ctx, principal, document, strings.TrimSpace(input.ProposalID), input.Force)
	case "discard_document_change_proposal":
		result, err = s.discardMCPDocumentChangeProposal(ctx, principal, document, strings.TrimSpace(input.ProposalID))
	}
	return result, true, err
}

func (s *MCPService) proposeMCPDocumentChange(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	executor := s.proposalCommands
	if executor == nil && s.commands != nil {
		executor = s.commands
	}
	if executor == nil {
		return nil, ErrMCPForbidden
	}
	var input struct {
		DocumentID string `json:"document_id"`
	}
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, strings.TrimSpace(input.DocumentID))
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, ErrMCPNotFound
	}
	if document.Status == model.DocStatusArchived {
		return nil, newMCPToolError(MCPErrorCodeDocumentArchived, "Restore the document before proposing changes to it.")
	}
	output, err := executor.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: principal.WorkspaceID,
		ActorID:     principal.UserID,
		ActorRole:   actor.Role,
	}, _mcpProposalCommand, arguments)
	if err != nil {
		if errors.Is(err, ErrDocsStaleBlockRevision) {
			return nil, newMCPToolError(MCPErrorCodeProposalConflict, "The block changed. Call get_document_blocks and propose against the current revision.")
		}
		return nil, err
	}
	var created struct {
		ProposalID string `json:"proposal_id"`
		Scope      string `json:"scope"`
		BlockID    string `json:"block_id"`
	}
	if err := json.Unmarshal(output, &created); err != nil {
		return nil, fmt.Errorf("decode proposal command output: %w", err)
	}
	data := map[string]any{
		"proposal_id": created.ProposalID,
		"document_id": document.ID,
		"scope":       created.Scope,
		"status":      model.DocsChangeProposalStatusPending,
	}
	if created.BlockID != "" {
		data["block_id"] = created.BlockID
	}
	return &MCPToolResult{
		Summary: "Change to " + document.Title + " proposed. It waits in Helpin Docs for a reviewer to apply or discard it.",
		Data:    data,
	}, nil
}

func (s *MCPService) listMCPDocumentChangeProposals(
	ctx context.Context,
	principal *model.MCPPrincipal,
	document *model.DocsDocument,
) (*MCPToolResult, error) {
	proposals, err := s.docsProposals.ListPending(ctx, principal.WorkspaceID, document.ID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(proposals))
	for i := range proposals {
		items = append(items, mcpProposalState(&proposals[i], true))
	}
	return &MCPToolResult{
		Summary: strconv.Itoa(len(items)) + " pending change proposals for " + document.Title + ".",
		Data:    map[string]any{"document_id": document.ID, "items": items},
	}, nil
}

func (s *MCPService) applyMCPDocumentChangeProposal(
	ctx context.Context,
	principal *model.MCPPrincipal,
	document *model.DocsDocument,
	proposalID string,
	force bool,
) (*MCPToolResult, error) {
	if document.IsLocked {
		return nil, newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	}
	proposal, err := s.pendingMCPProposal(ctx, principal, document, proposalID)
	if err != nil {
		return nil, err
	}
	if !force && s.docsProposals.BaseChanged(ctx, proposal) {
		return nil, newMCPToolError(MCPErrorCodeProposalConflict,
			"The content this proposal replaces changed after it was proposed. Review it with list_document_change_proposals, then discard it or apply with force.")
	}
	applied, _, err := s.docsProposals.Apply(ctx, principal.WorkspaceID, document.ID, proposal.ID, principal.UserID)
	if err != nil {
		return nil, mapMCPProposalError(err)
	}
	s.queueMCPDocumentEmbedding(ctx, document.ID)
	return &MCPToolResult{
		Summary: "Proposal applied to " + document.Title + ". Publish the document to make the change public.",
		Data:    mcpProposalState(applied, false),
	}, nil
}

func (s *MCPService) discardMCPDocumentChangeProposal(
	ctx context.Context,
	principal *model.MCPPrincipal,
	document *model.DocsDocument,
	proposalID string,
) (*MCPToolResult, error) {
	proposal, err := s.pendingMCPProposal(ctx, principal, document, proposalID)
	if err != nil {
		return nil, err
	}
	discarded, err := s.docsProposals.Discard(ctx, principal.WorkspaceID, document.ID, proposal.ID, principal.UserID)
	if err != nil {
		return nil, mapMCPProposalError(err)
	}
	return &MCPToolResult{Summary: "Proposal for " + document.Title + " discarded.", Data: mcpProposalState(discarded, false)}, nil
}

func (s *MCPService) pendingMCPProposal(
	ctx context.Context,
	principal *model.MCPPrincipal,
	document *model.DocsDocument,
	proposalID string,
) (*model.DocsChangeProposal, error) {
	if proposalID == "" {
		return nil, fmt.Errorf("%w: proposal_id is required", ErrMCPInvalidArguments)
	}
	proposal, err := s.docsProposals.Get(ctx, principal.WorkspaceID, document.ID, proposalID)
	if err != nil {
		return nil, mapMCPProposalError(err)
	}
	if proposal == nil {
		return nil, mapMCPProposalError(ErrDocsChangeProposalNotFound)
	}
	if proposal.Status != model.DocsChangeProposalStatusPending {
		return nil, mapMCPProposalError(ErrDocsChangeProposalResolved)
	}
	return proposal, nil
}

func mapMCPProposalError(err error) error {
	switch {
	case errors.Is(err, ErrDocsChangeProposalNotFound):
		return newMCPToolError(MCPErrorCodeProposalNotFound, "No proposal with that id exists for this document.")
	case errors.Is(err, ErrDocsChangeProposalResolved):
		return newMCPToolError(MCPErrorCodeProposalNotFound, "The proposal was already applied or discarded.")
	case errors.Is(err, ErrDocsContentConflict), errors.Is(err, ErrDocsStaleBlockRevision):
		return newMCPToolError(MCPErrorCodeProposalConflict, "The document changed after the proposal was made. Discard it or propose again against the current content.")
	case errors.Is(err, ErrDocsDocumentLocked):
		return newMCPToolError(MCPErrorCodeDocumentLocked, "The document is locked. Unlock it in Helpin first.")
	default:
		return err
	}
}

func mcpProposalState(proposal *model.DocsChangeProposal, includeContent bool) map[string]any {
	state := map[string]any{
		"proposal_id": proposal.ID,
		"document_id": proposal.DocumentID,
		"scope":       proposal.Scope,
		"status":      proposal.Status,
		"summary":     proposal.Summary,
		"created_by":  proposal.CreatedBy,
		"created_at":  proposal.CreatedAt,
	}
	if proposal.BlockID != nil {
		state["block_id"] = *proposal.BlockID
		state["revision"] = proposal.Revision
	}
	if proposal.AgentID != nil {
		state["agent_id"] = *proposal.AgentID
	}
	if proposal.AgentRunID != nil {
		state["agent_run_id"] = *proposal.AgentRunID
	}
	if proposal.ResolvedBy != nil {
		state["resolved_by"] = *proposal.ResolvedBy
	}
	if includeContent {
		state["content_markdown"] = proposal.ContentMarkdown
		state["base_markdown"] = proposal.BaseMarkdown
		state["sources"] = model.NormalizeDocsChangeProposalSources(proposal.Sources)
	}
	return state
}
