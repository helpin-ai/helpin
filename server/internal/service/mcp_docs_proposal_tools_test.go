package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestProposeDocumentChange(t *testing.T) {
	t.Run("runs the proposal command as the connected user", func(t *testing.T) {
		env, _, commands := setupMCPDocsProposalTest(t)
		result, err := env.run("propose_document_change",
			`{"scope":"document","document_id":"document-1","content":"# Guide","summary":"Refresh"}`)
		if err != nil {
			t.Fatalf("propose_document_change error = %v", err)
		}
		if commands.name != _mcpProposalCommand || commands.meta.ActorID != "user-1" ||
			commands.meta.WorkspaceID != "workspace-1" || commands.meta.RunID != "" {
			t.Fatalf("command = %s meta = %#v", commands.name, commands.meta)
		}
		data := result.Data.(map[string]any)
		if data["proposal_id"] != "proposal-new" || data["status"] != model.DocsChangeProposalStatusPending {
			t.Fatalf("data = %#v", data)
		}
	})
	t.Run("maps a stale block revision to PROPOSAL_CONFLICT", func(t *testing.T) {
		env, _, commands := setupMCPDocsProposalTest(t)
		commands.err = fmt.Errorf("%w; fetch the latest block revision", ErrDocsStaleBlockRevision)
		_, err := env.run("propose_document_change",
			`{"scope":"block","document_id":"document-1","block_id":"b1","revision":2,"content":"x","summary":"y"}`)
		assertMCPToolErrorCode(t, err, MCPErrorCodeProposalConflict)
	})
	t.Run("hides another workspace's document", func(t *testing.T) {
		env, _, commands := setupMCPDocsProposalTest(t)
		env.documents.documents["document-1"].WorkspaceID = "workspace-other"
		_, err := env.run("propose_document_change",
			`{"scope":"document","document_id":"document-1","content":"x","summary":"y"}`)
		if !errors.Is(err, ErrMCPNotFound) || commands.name != "" {
			t.Fatalf("error = %v, command = %q", err, commands.name)
		}
	})
	t.Run("refuses an archived document", func(t *testing.T) {
		env, _, _ := setupMCPDocsProposalTest(t)
		env.documents.documents["document-1"].Status = model.DocStatusArchived
		_, err := env.run("propose_document_change",
			`{"scope":"document","document_id":"document-1","content":"x","summary":"y"}`)
		assertMCPToolErrorCode(t, err, MCPErrorCodeDocumentArchived)
	})
}

func TestListDocumentChangeProposals(t *testing.T) {
	env, proposals, _ := setupMCPDocsProposalTest(t)
	result, err := env.run("list_document_change_proposals", `{"document_id":"document-1"}`)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	items := result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["proposal_id"] != "proposal-1" || items[0]["content_markdown"] != "# New" {
		t.Fatalf("items = %#v", items)
	}
	if proposals.listDocumentID != "document-1" {
		t.Fatalf("listed document = %q", proposals.listDocumentID)
	}
}

func TestApplyDocumentChangeProposal(t *testing.T) {
	tests := []struct {
		name        string
		arguments   string
		prepare     func(*fakeMCPDocsProposalService, *mcpDocsLifecycleTestEnv)
		wantCode    string
		wantApplied bool
	}{
		{name: "applies a pending proposal", arguments: `{"document_id":"document-1","proposal_id":"proposal-1"}`, wantApplied: true},
		{
			name: "reports PROPOSAL_CONFLICT when the base changed", arguments: `{"document_id":"document-1","proposal_id":"proposal-1"}`,
			prepare:  func(f *fakeMCPDocsProposalService, _ *mcpDocsLifecycleTestEnv) { f.baseChanged = true },
			wantCode: MCPErrorCodeProposalConflict,
		},
		{
			name: "force applies despite a changed base", arguments: `{"document_id":"document-1","proposal_id":"proposal-1","force":true}`,
			prepare:     func(f *fakeMCPDocsProposalService, _ *mcpDocsLifecycleTestEnv) { f.baseChanged = true },
			wantApplied: true,
		},
		{
			name: "maps a stale block revision to PROPOSAL_CONFLICT", arguments: `{"document_id":"document-1","proposal_id":"proposal-1"}`,
			prepare: func(f *fakeMCPDocsProposalService, _ *mcpDocsLifecycleTestEnv) {
				f.applyErr = ErrDocsStaleBlockRevision
			},
			wantCode: MCPErrorCodeProposalConflict,
		},
		{
			name: "reports PROPOSAL_NOT_FOUND for an unknown proposal", arguments: `{"document_id":"document-1","proposal_id":"missing"}`,
			wantCode: MCPErrorCodeProposalNotFound,
		},
		{
			name: "reports PROPOSAL_NOT_FOUND for a resolved proposal", arguments: `{"document_id":"document-1","proposal_id":"proposal-1"}`,
			prepare: func(f *fakeMCPDocsProposalService, _ *mcpDocsLifecycleTestEnv) {
				f.proposal.Status = model.DocsChangeProposalStatusDiscarded
			},
			wantCode: MCPErrorCodeProposalNotFound,
		},
		{
			name: "refuses a locked document", arguments: `{"document_id":"document-1","proposal_id":"proposal-1"}`,
			prepare: func(_ *fakeMCPDocsProposalService, env *mcpDocsLifecycleTestEnv) {
				env.documents.documents["document-1"].IsLocked = true
			},
			wantCode: MCPErrorCodeDocumentLocked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, proposals, _ := setupMCPDocsProposalTest(t)
			if tt.prepare != nil {
				tt.prepare(proposals, env)
			}
			result, err := env.run("apply_document_change_proposal", tt.arguments)
			if tt.wantCode != "" {
				assertMCPToolErrorCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("apply error = %v", err)
			}
			if (proposals.appliedBy != "") != tt.wantApplied && tt.wantCode == "" {
				t.Fatalf("applied by %q, want applied %v", proposals.appliedBy, tt.wantApplied)
			}
			if tt.wantApplied {
				if proposals.appliedBy != "user-1" || result.Data.(map[string]any)["status"] != model.DocsChangeProposalStatusApplied {
					t.Fatalf("applied by %q, result = %#v", proposals.appliedBy, result.Data)
				}
				if env.embeddings.queued != 1 {
					t.Fatalf("embedding sync queued %d times, want 1", env.embeddings.queued)
				}
			}
		})
	}
}

func TestDiscardDocumentChangeProposal(t *testing.T) {
	t.Run("discards a pending proposal", func(t *testing.T) {
		env, proposals, _ := setupMCPDocsProposalTest(t)
		result, err := env.run("discard_document_change_proposal", `{"document_id":"document-1","proposal_id":"proposal-1"}`)
		if err != nil {
			t.Fatalf("discard error = %v", err)
		}
		if proposals.discardedBy != "user-1" || result.Data.(map[string]any)["status"] != model.DocsChangeProposalStatusDiscarded {
			t.Fatalf("discarded by %q, result = %#v", proposals.discardedBy, result.Data)
		}
	})
	t.Run("hides a document in an inaccessible space", func(t *testing.T) {
		env, proposals, _ := setupMCPDocsProposalTest(t)
		env.documents.documents["document-1"].SpaceID = "space-hidden"
		_, err := env.run("discard_document_change_proposal", `{"document_id":"document-1","proposal_id":"proposal-1"}`)
		if !errors.Is(err, ErrMCPNotFound) || proposals.discardedBy != "" {
			t.Fatalf("error = %v, discarded by %q", err, proposals.discardedBy)
		}
	})
}

func TestDocsProposalMCPToolDefinitions(t *testing.T) {
	definitions := map[string]MCPToolDefinition{}
	for _, definition := range docsProposalMCPToolDefinitions() {
		definitions[definition.Name] = definition
	}
	if definitions["list_document_change_proposals"].Mutating {
		t.Errorf("list_document_change_proposals must be read-only")
	}
	for _, name := range []string{"propose_document_change", "apply_document_change_proposal", "discard_document_change_proposal"} {
		definition := definitions[name]
		if definition.Scope != MCPScopeDocsWrite || !definition.Mutating || !definition.IdempotentHint {
			t.Errorf("%s = %#v, want a mutating docs.write tool", name, definition)
		}
	}
	if _, err := resolveMCPToolSchema(definitions["propose_document_change"].InputSchema); err != nil {
		t.Fatalf("propose_document_change schema: %v", err)
	}
}

func setupMCPDocsProposalTest(t *testing.T) (*mcpDocsLifecycleTestEnv, *fakeMCPDocsProposalService, *fakeMCPCommandExecutor) {
	t.Helper()
	env := setupMCPDocsLifecycleTest(t, model.DocStatusPublished, model.SpaceTypeExternalCapable)
	proposals := &fakeMCPDocsProposalService{proposal: &model.DocsChangeProposal{
		ID: "proposal-1", WorkspaceID: "workspace-1", DocumentID: "document-1", Scope: "document",
		Status: model.DocsChangeProposalStatusPending, Summary: "Refresh", ContentMarkdown: "# New", BaseMarkdown: "# Old",
	}}
	commands := &fakeMCPCommandExecutor{}
	env.service.docsProposals = proposals
	env.service.proposalCommands = commands
	return env, proposals, commands
}

type fakeMCPCommandExecutor struct {
	name string
	meta model.InternalCommandContext
	err  error
}

func (f *fakeMCPCommandExecutor) Execute(_ context.Context, meta model.InternalCommandContext, name string, _ json.RawMessage) (json.RawMessage, error) {
	f.name, f.meta = name, meta
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(`{"status":"submitted","proposal_id":"proposal-new","scope":"document","document_id":"document-1","block_id":""}`), nil
}

type fakeMCPDocsProposalService struct {
	proposal       *model.DocsChangeProposal
	baseChanged    bool
	applyErr       error
	listDocumentID string
	appliedBy      string
	discardedBy    string
}

func (f *fakeMCPDocsProposalService) ListPending(_ context.Context, _, documentID string) ([]model.DocsChangeProposal, error) {
	f.listDocumentID = documentID
	return []model.DocsChangeProposal{*f.proposal}, nil
}

func (f *fakeMCPDocsProposalService) Get(_ context.Context, _, documentID, proposalID string) (*model.DocsChangeProposal, error) {
	if proposalID != f.proposal.ID || documentID != f.proposal.DocumentID {
		return nil, ErrDocsChangeProposalNotFound
	}
	copied := *f.proposal
	return &copied, nil
}

func (f *fakeMCPDocsProposalService) Apply(_ context.Context, _, _, _, actorID string) (*model.DocsChangeProposal, *model.DocsContent, error) {
	if f.applyErr != nil {
		return nil, nil, f.applyErr
	}
	f.appliedBy = actorID
	applied := *f.proposal
	applied.Status, applied.ResolvedBy = model.DocsChangeProposalStatusApplied, &actorID
	return &applied, &model.DocsContent{}, nil
}

func (f *fakeMCPDocsProposalService) Discard(_ context.Context, _, _, _, actorID string) (*model.DocsChangeProposal, error) {
	f.discardedBy = actorID
	discarded := *f.proposal
	discarded.Status, discarded.ResolvedBy = model.DocsChangeProposalStatusDiscarded, &actorID
	return &discarded, nil
}

func (f *fakeMCPDocsProposalService) BaseChanged(context.Context, *model.DocsChangeProposal) bool {
	return f.baseChanged
}
