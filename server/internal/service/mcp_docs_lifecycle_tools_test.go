package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestExecuteDocsLifecycleMCPToolPublish(t *testing.T) {
	tests := []struct {
		name            string
		spaceType       string
		wantExternal    bool
		wantHelpCenter  bool
		wantSummaryPart string
	}{
		{
			name:            "help center space goes live",
			spaceType:       model.SpaceTypeExternalCapable,
			wantExternal:    true,
			wantHelpCenter:  true,
			wantSummaryPart: "is live in the Help Center.",
		},
		{
			name:            "internal space publishes inside the workspace only",
			spaceType:       model.SpaceTypeInternal,
			wantExternal:    false,
			wantHelpCenter:  false,
			wantSummaryPart: "published inside the workspace.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, tt.spaceType)
			result, err := env.run("publish_document", `{"document_id":"document-1","slug":"getting-started"}`)
			if err != nil {
				t.Fatalf("publish_document error = %v", err)
			}
			if env.lifecycle.publishCalls != 1 {
				t.Fatalf("internal publish calls = %d, want 1", env.lifecycle.publishCalls)
			}
			if (env.helpcenter.publishSlug != "") != tt.wantExternal {
				t.Fatalf("external publish slug = %q, want external %v", env.helpcenter.publishSlug, tt.wantExternal)
			}
			if tt.wantExternal && env.helpcenter.publishSlug != "getting-started" {
				t.Fatalf("external publish slug = %q, want getting-started", env.helpcenter.publishSlug)
			}
			data := result.Data.(map[string]any)
			if data["help_center"] != tt.wantHelpCenter {
				t.Fatalf("help_center = %v, want %v", data["help_center"], tt.wantHelpCenter)
			}
			if !strings.Contains(result.Summary, tt.wantSummaryPart) {
				t.Fatalf("summary = %q, want it to contain %q", result.Summary, tt.wantSummaryPart)
			}
			if env.embeddings.queued != 1 {
				t.Fatalf("embedding sync queued %d times, want 1", env.embeddings.queued)
			}
		})
	}
}

func TestExecuteDocsLifecycleMCPToolPublishRejectsArchived(t *testing.T) {
	env := setupMCPDocsLifecycleTest(t, model.DocStatusArchived, model.SpaceTypeExternalCapable)
	_, err := env.run("publish_document", `{"document_id":"document-1"}`)
	assertMCPToolErrorCode(t, err, MCPErrorCodeDocumentArchived)
	if env.lifecycle.publishCalls != 0 {
		t.Fatalf("internal publish calls = %d, want 0", env.lifecycle.publishCalls)
	}
}

func TestExecuteDocsLifecycleMCPToolRejectsLockedDocument(t *testing.T) {
	env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeExternalCapable)
	env.documents.documents["document-1"].IsLocked = true
	_, err := env.run("update_document", `{"document_id":"document-1","title":"New"}`)
	assertMCPToolErrorCode(t, err, MCPErrorCodeDocumentLocked)
}

func TestExecuteDocsLifecycleMCPToolUnpublishLiveArticle(t *testing.T) {
	env := setupMCPDocsLifecycleTest(t, model.DocStatusPublished, model.SpaceTypeExternalCapable)
	env.helpcenter.markLive()
	result, err := env.run("unpublish_document", `{"document_id":"document-1"}`)
	if err != nil {
		t.Fatalf("unpublish_document error = %v", err)
	}
	if env.helpcenter.unpublishCalls != 1 || env.lifecycle.unpublishCalls != 1 {
		t.Fatalf("unpublish calls external=%d internal=%d, want 1 and 1",
			env.helpcenter.unpublishCalls, env.lifecycle.unpublishCalls)
	}
	if data := result.Data.(map[string]any); data["status"] != model.DocStatusDraft {
		t.Fatalf("status = %v, want draft", data["status"])
	}
}

func TestExecuteDocsLifecycleMCPToolUnpublishRejectsDraft(t *testing.T) {
	env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeExternalCapable)
	_, err := env.run("unpublish_document", `{"document_id":"document-1"}`)
	assertMCPToolErrorCode(t, err, MCPErrorCodeNotPublished)
}

func TestExecuteDocsLifecycleMCPToolArchive(t *testing.T) {
	t.Run("archives a draft", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeExternalCapable)
		if _, err := env.run("archive_document", `{"document_id":"document-1"}`); err != nil {
			t.Fatalf("archive_document error = %v", err)
		}
		if env.lifecycle.archiveCalls != 1 {
			t.Fatalf("archive calls = %d, want 1", env.lifecycle.archiveCalls)
		}
	})
	t.Run("refuses a live help center article", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusPublished, model.SpaceTypeExternalCapable)
		env.helpcenter.markLive()
		_, err := env.run("archive_document", `{"document_id":"document-1"}`)
		assertMCPToolErrorCode(t, err, MCPErrorCodeDocumentPublished)
		if env.lifecycle.archiveCalls != 0 {
			t.Fatalf("archive calls = %d, want 0", env.lifecycle.archiveCalls)
		}
	})
}

func TestExecuteDocsLifecycleMCPToolRestore(t *testing.T) {
	t.Run("restores an archived document", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusArchived, model.SpaceTypeInternal)
		if _, err := env.run("restore_document", `{"document_id":"document-1"}`); err != nil {
			t.Fatalf("restore_document error = %v", err)
		}
		if env.lifecycle.unarchiveCalls != 1 {
			t.Fatalf("unarchive calls = %d, want 1", env.lifecycle.unarchiveCalls)
		}
	})
	t.Run("rejects a document that is not archived", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
		_, err := env.run("restore_document", `{"document_id":"document-1"}`)
		assertMCPToolErrorCode(t, err, MCPErrorCodeNotArchived)
	})
}

func TestExecuteDocsLifecycleMCPToolUpdateDocument(t *testing.T) {
	t.Run("renames and clears the excerpt", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
		_, err := env.run("update_document", `{"document_id":"document-1","title":"  Task links  ","excerpt":null}`)
		if err != nil {
			t.Fatalf("update_document error = %v", err)
		}
		request := env.lifecycle.updateRequest
		if request.Title == nil || *request.Title != "Task links" || !request.ClearExcerpt || request.Excerpt != nil {
			t.Fatalf("update request = %#v", request)
		}
	})
	t.Run("requires at least one field", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
		_, err := env.run("update_document", `{"document_id":"document-1"}`)
		if !errors.Is(err, ErrMCPInvalidArguments) {
			t.Fatalf("update_document error = %v, want ErrMCPInvalidArguments", err)
		}
	})
	t.Run("rejects a document in another workspace", func(t *testing.T) {
		env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
		env.documents.documents["document-1"].WorkspaceID = "workspace-other"
		_, err := env.run("update_document", `{"document_id":"document-1","title":"x"}`)
		if !errors.Is(err, ErrMCPNotFound) {
			t.Fatalf("update_document error = %v, want ErrMCPNotFound", err)
		}
	})
}

func TestExecuteDocsLifecycleMCPToolIgnoresOtherTools(t *testing.T) {
	env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
	_, handled, err := env.service.executeDocsLifecycleMCPTool(
		context.Background(), env.principal, env.actor, "list_spaces", json.RawMessage(`{}`))
	if handled || err != nil {
		t.Fatalf("handled = %v, err = %v; want false, nil", handled, err)
	}
}

func TestDocsLifecycleMCPToolDefinitions(t *testing.T) {
	definitions := map[string]MCPToolDefinition{}
	for _, definition := range docsLifecycleMCPToolDefinitions() {
		definitions[definition.Name] = definition
	}
	for _, name := range []string{"publish_document", "unpublish_document"} {
		if definitions[name].Scope != MCPScopeDocsPublish {
			t.Errorf("%s scope = %q, want %q", name, definitions[name].Scope, MCPScopeDocsPublish)
		}
	}
	for _, name := range []string{"unpublish_document", "archive_document"} {
		if !definitions[name].Destructive {
			t.Errorf("%s must be destructive", name)
		}
	}
	for name, definition := range definitions {
		if !definition.Mutating || !definition.IdempotentHint {
			t.Errorf("%s must be a mutating, idempotent tool", name)
		}
	}
}

func TestMCPDocsPublishScopeIsWriteScope(t *testing.T) {
	if !isMCPWriteScope(MCPScopeDocsPublish) {
		t.Fatalf("publish scope must be treated as a write scope")
	}
	filtered := filterMCPReadScopes([]string{MCPScopeDocsRead, MCPScopeDocsPublish})
	if len(filtered) != 1 || filtered[0] != MCPScopeDocsRead {
		t.Fatalf("read-only scopes = %v, want only docs read", filtered)
	}
}

func TestMCPPrincipalCanSeeRun(t *testing.T) {
	owner := "user-1"
	other := "user-2"
	chat := "chat-1"
	principal := &model.MCPPrincipal{UserID: owner}
	tests := []struct {
		name string
		run  *model.AgentRun
		want bool
	}{
		{name: "nil run", run: nil, want: false},
		{name: "workspace run without chat", run: &model.AgentRun{TriggeredByUserID: &other}, want: true},
		{name: "own chat run", run: &model.AgentRun{DockChatID: &chat, TriggeredByUserID: &owner}, want: true},
		{name: "another user's chat run", run: &model.AgentRun{DockChatID: &chat, TriggeredByUserID: &other}, want: false},
		{name: "chat run without owner", run: &model.AgentRun{DockChatID: &chat}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mcpPrincipalCanSeeRun(principal, tt.run); got != tt.want {
				t.Fatalf("mcpPrincipalCanSeeRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mcpDocsLifecycleTestEnv struct {
	t          *testing.T
	service    *MCPService
	principal  *model.MCPPrincipal
	actor      *authorization.Actor
	documents  *fakeMCPDocsDocumentService
	lifecycle  *fakeMCPDocsLifecycleService
	helpcenter *fakeMCPHelpcenterPublisher
	embeddings *fakeMCPDocsEmbeddingQueue
}

func setupMCPDocsLifecycleTest(t *testing.T, status, spaceType string) *mcpDocsLifecycleTestEnv {
	t.Helper()
	service, principal, actor, spaces, _ := setupMCPDocsToolTest()
	spaces.getResults["space-1"].Type = spaceType
	documents := &fakeMCPDocsDocumentService{documents: map[string]*model.DocsDocument{
		"document-1": {ID: "document-1", WorkspaceID: principal.WorkspaceID, SpaceID: "space-1", Title: "Guide", Status: status},
	}}
	lifecycle := &fakeMCPDocsLifecycleService{documents: documents}
	helpcenter := &fakeMCPHelpcenterPublisher{}
	embeddings := &fakeMCPDocsEmbeddingQueue{}
	service.documents = documents
	service.docsLifecycle = lifecycle
	service.helpcenter = helpcenter
	service.docsEmbedding = embeddings
	return &mcpDocsLifecycleTestEnv{
		t: t, service: service, principal: principal, actor: actor, documents: documents,
		lifecycle: lifecycle, helpcenter: helpcenter, embeddings: embeddings,
	}
}

func (e *mcpDocsLifecycleTestEnv) run(name, arguments string) (*MCPToolResult, error) {
	e.t.Helper()
	return e.service.executeSpecialMCPTool(context.Background(), e.principal, e.actor, name, json.RawMessage(arguments))
}

func assertMCPToolErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var toolErr *MCPToolError
	if !errors.As(err, &toolErr) || toolErr.Code != code {
		t.Fatalf("error = %v, want MCPToolError code %s", err, code)
	}
}

type fakeMCPDocsLifecycleService struct {
	documents      *fakeMCPDocsDocumentService
	updateRequest  model.UpdateDocsDocumentRequest
	publishCalls   int
	unpublishCalls int
	archiveCalls   int
	unarchiveCalls int
}

func (f *fakeMCPDocsLifecycleService) setStatus(id, status string) *model.DocsDocument {
	document := f.documents.documents[id]
	document.Status = status
	copied := *document
	return &copied
}

func (f *fakeMCPDocsLifecycleService) Update(_ context.Context, id string, request model.UpdateDocsDocumentRequest) (*model.DocsDocument, error) {
	f.updateRequest = request
	document := *f.documents.documents[id]
	if request.Title != nil {
		document.Title = *request.Title
	}
	return &document, nil
}

func (f *fakeMCPDocsLifecycleService) Publish(_ context.Context, id string) (*model.DocsDocument, error) {
	f.publishCalls++
	return f.setStatus(id, model.DocStatusPublished), nil
}

func (f *fakeMCPDocsLifecycleService) Unpublish(_ context.Context, id string) (*model.DocsDocument, error) {
	f.unpublishCalls++
	return f.setStatus(id, model.DocStatusDraft), nil
}

func (f *fakeMCPDocsLifecycleService) Archive(_ context.Context, id string) (*model.DocsDocument, error) {
	f.archiveCalls++
	return f.setStatus(id, model.DocStatusArchived), nil
}

func (f *fakeMCPDocsLifecycleService) Unarchive(_ context.Context, id string) (*model.DocsDocument, error) {
	f.unarchiveCalls++
	return f.setStatus(id, model.DocStatusDraft), nil
}

type fakeMCPHelpcenterPublisher struct {
	article        *model.DocsHelpcenterArticle
	publishSlug    string
	unpublishCalls int
}

func (f *fakeMCPHelpcenterPublisher) markLive() {
	now := time.Now()
	f.article = &model.DocsHelpcenterArticle{DocumentID: "document-1", PublicID: "pub-1", Slug: "guide", PublicPublishedAt: &now}
}

func (f *fakeMCPHelpcenterPublisher) PublishExternally(_ context.Context, documentID, slug string, _ json.RawMessage) error {
	f.publishSlug = slug
	if slug == "" {
		f.publishSlug = "derived"
	}
	now := time.Now()
	f.article = &model.DocsHelpcenterArticle{DocumentID: documentID, PublicID: "pub-1", Slug: f.publishSlug, PublicPublishedAt: &now}
	return nil
}

func (f *fakeMCPHelpcenterPublisher) UnpublishExternally(context.Context, string) error {
	f.unpublishCalls++
	if f.article != nil {
		f.article.PublicPublishedAt = nil
	}
	return nil
}

func (f *fakeMCPHelpcenterPublisher) GetArticle(context.Context, string) (*model.DocsHelpcenterArticle, error) {
	return f.article, nil
}

type fakeMCPDocsEmbeddingQueue struct{ queued int }

func (f *fakeMCPDocsEmbeddingQueue) QueueDocumentSync(context.Context, string) error {
	f.queued++
	return nil
}

func TestRequireMCPCommandDocumentAccess(t *testing.T) {
	docsTool := MCPToolDefinition{Name: "edit_document", Toolset: MCPToolsetDocs}
	tests := []struct {
		name      string
		tool      MCPToolDefinition
		arguments string
		spaceID   string
		wantErr   error
	}{
		{name: "accessible document passes", tool: docsTool, arguments: `{"document_id":"document-1"}`, spaceID: "space-1"},
		{name: "document in an inaccessible space is hidden", tool: docsTool, arguments: `{"document_id":"document-1"}`, spaceID: "space-hidden", wantErr: ErrMCPNotFound},
		{name: "unknown document is hidden", tool: docsTool, arguments: `{"document_id":"missing"}`, spaceID: "space-1", wantErr: ErrMCPNotFound},
		{name: "tools without a document id are unaffected", tool: docsTool, arguments: `{"query":"x"}`, spaceID: "space-hidden"},
		{name: "non-docs tools are unaffected", tool: MCPToolDefinition{Name: "list_tasks", Toolset: MCPToolsetPM}, arguments: `{"document_id":"document-1"}`, spaceID: "space-hidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeInternal)
			env.documents.documents["document-1"].SpaceID = tt.spaceID
			err := env.service.requireMCPCommandDocumentAccess(
				context.Background(), env.principal, env.actor, tt.tool, json.RawMessage(tt.arguments))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("requireMCPCommandDocumentAccess() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
