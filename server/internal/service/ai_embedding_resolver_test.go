package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const embeddingTestWorkspace = "workspace"

// fakeWorkspaceEmbedder records requests and returns vectors of a fixed size.
type fakeWorkspaceEmbedder struct {
	mu         sync.Mutex
	provider   string
	apiKey     string
	dimensions int
	requests   []llm.EmbeddingRequest
}

func (f *fakeWorkspaceEmbedder) CreateEmbeddings(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	vectors := make([][]float32, len(req.Inputs))
	for i := range vectors {
		vectors[i] = make([]float32, f.dimensions)
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

// fakeEmbeddingClients builds fakeWorkspaceEmbedder clients and remembers them.
type fakeEmbeddingClients struct {
	dimensions int
	built      []*fakeWorkspaceEmbedder
}

func (f *fakeEmbeddingClients) factory(provider, apiKey string) (llm.EmbeddingProvider, error) {
	dimensions := f.dimensions
	if dimensions == 0 {
		dimensions = docsEmbeddingDimensions
	}
	client := &fakeWorkspaceEmbedder{provider: provider, apiKey: apiKey, dimensions: dimensions}
	f.built = append(f.built, client)
	return client, nil
}

func (f *fakeEmbeddingClients) last(t *testing.T) *fakeWorkspaceEmbedder {
	t.Helper()
	if len(f.built) == 0 {
		t.Fatal("no workspace embedding client was built")
	}
	return f.built[len(f.built)-1]
}

type recordingEmbeddingAudit struct {
	mu       sync.Mutex
	started  []model.AIActionExecution
	finished []aipolicy.ExecutionResult
}

func (a *recordingEmbeddingAudit) Start(_ context.Context, execution *model.AIActionExecution) (*model.AIActionExecution, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	copy := *execution
	copy.ID = fmt.Sprintf("execution-%d", len(a.started)+1)
	a.started = append(a.started, copy)
	return &copy, nil
}

func (a *recordingEmbeddingAudit) Finish(_ context.Context, _ string, result aipolicy.ExecutionResult) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.finished = append(a.finished, result)
	return nil
}

type recordingBackfiller struct {
	mu         sync.Mutex
	workspaces []string
}

func (b *recordingBackfiller) QueueMissingEmbeddings(_ context.Context, workspaceID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.workspaces = append(b.workspaces, workspaceID)
	return nil
}

type fakeAdmissionPolicy struct {
	snapshot *model.AIExecutionPolicySnapshot
	err      error
}

func (p fakeAdmissionPolicy) ResolveConnectionPolicy(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
	return p.snapshot, p.err
}

type embeddingConnectionSpec struct {
	id       string
	provider string
	status   string
	personal bool
	noSecret bool
}

func addEmbeddingConnection(t *testing.T, s *AIConnectionService, spec embeddingConnectionSpec, created time.Time) *model.AIConnection {
	t.Helper()
	status := spec.status
	if status == "" {
		status = "connected"
	}
	c := &model.AIConnection{
		ID: spec.id, WorkspaceID: embeddingTestWorkspace, Scope: "workspace", Funding: "customer",
		Name: spec.provider + " " + spec.id, Provider: spec.provider, Status: status, CreatedAt: created, UpdatedAt: created,
	}
	if spec.personal {
		owner := "owner"
		c.Scope, c.UserID = "personal", &owner
	}
	if !spec.noSecret {
		if err := s.seal(c, aiConnectionSecret{APIKey: "key-" + spec.id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.repo.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func newTestEmbeddingResolver(t *testing.T, s *AIConnectionService, server llm.EmbeddingProvider, clients *fakeEmbeddingClients, audit aipolicy.ExecutionAudit) *WorkspaceEmbeddingResolver {
	t.Helper()
	return NewWorkspaceEmbeddingResolver(WorkspaceEmbeddingResolverConfig{
		Server: server, ServerProvider: "openai", Connections: s,
		Registry: aipolicy.DefaultRegistry(), Audit: audit,
	}).SetClientFactory(clients.factory)
}

func TestWorkspaceEmbeddingResolverPrecedence(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		server       bool
		connections  []embeddingConnectionSpec
		wantSource   string
		wantProvider string
		wantID       string
	}{
		{
			name: "server key wins over workspace connections", server: true,
			connections: []embeddingConnectionSpec{{id: "c-openai", provider: "openai"}},
			wantSource:  EmbeddingSourceServer, wantProvider: "openai",
		},
		{
			name: "workspace OpenAI before OpenRouter",
			connections: []embeddingConnectionSpec{
				{id: "c-openrouter", provider: "openrouter"}, {id: "c-openai", provider: "openai"},
			},
			wantSource: EmbeddingSourceWorkspace, wantProvider: "openai", wantID: "c-openai",
		},
		{
			name: "workspace OpenRouter when no OpenAI connection",
			connections: []embeddingConnectionSpec{
				{id: "c-anthropic", provider: "anthropic"}, {id: "c-openrouter", provider: "openrouter"},
			},
			wantSource: EmbeddingSourceWorkspace, wantProvider: "openrouter", wantID: "c-openrouter",
		},
		{
			name:        "anthropic is never used",
			connections: []embeddingConnectionSpec{{id: "c-anthropic", provider: "anthropic"}},
		},
		{
			name:        "compatible endpoints are never used",
			connections: []embeddingConnectionSpec{{id: "c-compatible", provider: "openai_compatible"}},
		},
		{
			name: "disconnected, secretless and personal connections are ignored",
			connections: []embeddingConnectionSpec{
				{id: "c-disconnected", provider: "openai", status: "disconnected"},
				{id: "c-empty", provider: "openai", noSecret: true},
				{id: "c-personal", provider: "openai", personal: true},
			},
		},
		{
			name: "standard connection preferred within a provider",
			connections: []embeddingConnectionSpec{
				{id: "c-openai-custom", provider: "openai"},
				{id: model.StandardAIConnectionID(embeddingTestWorkspace, "openai"), provider: "openai"},
			},
			wantSource: EmbeddingSourceWorkspace, wantProvider: "openai",
			wantID: model.StandardAIConnectionID(embeddingTestWorkspace, "openai"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := setupAIConnectionTest(t)
			for i, spec := range tt.connections {
				addEmbeddingConnection(t, s, spec, base.Add(time.Duration(i)*time.Minute))
			}
			var server llm.EmbeddingProvider
			if tt.server {
				server = &fakeWorkspaceEmbedder{dimensions: docsEmbeddingDimensions}
			}
			clients := &fakeEmbeddingClients{}
			resolver := newTestEmbeddingResolver(t, s, server, clients, nil)
			selection, err := resolver.EmbeddingProviderFor(context.Background(), embeddingTestWorkspace)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantSource == "" {
				if selection != nil {
					t.Fatalf("selection = %+v, want none", selection.Info)
				}
				if resolver.EmbeddingsAvailable(context.Background(), embeddingTestWorkspace) {
					t.Fatal("EmbeddingsAvailable = true, want false")
				}
				return
			}
			if selection == nil {
				t.Fatal("selection = nil, want a source")
			}
			info := selection.Info
			if info.Source != tt.wantSource || info.Provider != tt.wantProvider || info.ConnectionID != tt.wantID {
				t.Fatalf("source = %+v, want %s/%s/%s", info, tt.wantSource, tt.wantProvider, tt.wantID)
			}
			if info.Model != defaultDocsEmbeddingModel || info.Dimensions != docsEmbeddingDimensions {
				t.Fatalf("model = %s/%d, want %s/%d", info.Model, info.Dimensions, defaultDocsEmbeddingModel, docsEmbeddingDimensions)
			}
			if tt.wantSource == EmbeddingSourceWorkspace {
				if got := clients.last(t); got.provider != tt.wantProvider || got.apiKey != "key-"+tt.wantID {
					t.Fatalf("client built for %s with key %q", got.provider, got.apiKey)
				}
				if info.FundingMode != aiusage.FundingCustomerUnbilled {
					t.Fatalf("funding = %q, want customer_unbilled", info.FundingMode)
				}
			}
		})
	}
}

func TestWorkspaceEmbeddingResolverDetails(t *testing.T) {
	tests := []struct {
		info EmbeddingSourceInfo
		want string
	}{
		{EmbeddingSourceInfo{Source: EmbeddingSourceServer, Provider: "openai"}, "Using the server's OpenAI key"},
		{EmbeddingSourceInfo{Source: EmbeddingSourceServer, Provider: "openrouter"}, "Using the server's OpenRouter key"},
		{EmbeddingSourceInfo{Source: EmbeddingSourceWorkspace, Provider: "openrouter"}, "Using this workspace's OpenRouter connection"},
		{EmbeddingSourceInfo{}, "No embedding provider is configured"},
	}
	for _, tt := range tests {
		if got := tt.info.Detail(); got != tt.want {
			t.Errorf("Detail() = %q, want %q", got, tt.want)
		}
	}
}

func TestWorkspaceEmbeddingResolverCachesAndInvalidatesOnConnectionChange(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	clients := &fakeEmbeddingClients{}
	resolver := newTestEmbeddingResolver(t, s, nil, clients, nil)
	backfill := &recordingBackfiller{}
	resolver.SetBackfiller(backfill)
	s.AddChangeObserver(resolver)
	ctx := context.Background()

	if resolver.EmbeddingsAvailable(ctx, embeddingTestWorkspace) {
		t.Fatal("workspace without connections has embeddings")
	}
	connection := addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openrouter", provider: "openrouter"}, time.Now())
	if resolver.EmbeddingsAvailable(ctx, embeddingTestWorkspace) {
		t.Fatal("cached negative result was not used before the change notification")
	}

	s.notifyConnectionsChanged(ctx, connection)
	resolver.WaitForBackfills()
	info, err := resolver.EmbeddingSource(ctx, embeddingTestWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if info.Source != EmbeddingSourceWorkspace || info.Provider != "openrouter" {
		t.Fatalf("source after change = %+v, want workspace openrouter", info)
	}
	if len(backfill.workspaces) != 1 || backfill.workspaces[0] != embeddingTestWorkspace {
		t.Fatalf("backfills = %v, want one for the workspace", backfill.workspaces)
	}

	// Disconnecting drops the source once the change is observed.
	if err := s.repo.WithLocked(ctx, connection.ID, func(c *model.AIConnection) error {
		c.Status, c.EncryptedSecret = "disconnected", nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.notifyConnectionsChanged(ctx, connection)
	resolver.WaitForBackfills()
	if resolver.EmbeddingsAvailable(ctx, embeddingTestWorkspace) {
		t.Fatal("disconnected connection still serves embeddings")
	}
	if len(backfill.workspaces) != 1 {
		t.Fatalf("backfill queued without an embedding source: %v", backfill.workspaces)
	}
}

func TestWorkspaceEmbeddingResolverCacheExpires(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{}, nil)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	resolver.now = func() time.Time { return now }
	ctx := context.Background()
	if resolver.EmbeddingsAvailable(ctx, embeddingTestWorkspace) {
		t.Fatal("unexpected source")
	}
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, now)
	now = now.Add(defaultEmbeddingSourceTTL + time.Second)
	if !resolver.EmbeddingsAvailable(ctx, embeddingTestWorkspace) {
		t.Fatal("expired cache entry was reused")
	}
}

func TestWorkspaceEmbeddingResolverSkipsBackfillWithServerKey(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	resolver := newTestEmbeddingResolver(t, s, &fakeWorkspaceEmbedder{dimensions: docsEmbeddingDimensions}, &fakeEmbeddingClients{}, nil)
	backfill := &recordingBackfiller{}
	resolver.SetBackfiller(backfill)
	connection := addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, time.Now())
	if err := resolver.AIConnectionsChanged(context.Background(), connection.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	resolver.WaitForBackfills()
	if len(backfill.workspaces) != 0 {
		t.Fatalf("server-key instance queued a workspace backfill: %v", backfill.workspaces)
	}
}

func TestWorkspaceEmbeddingResolverAuditsWorkspaceFundedEmbeddings(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openrouter", provider: "openrouter"}, time.Now())
	clients := &fakeEmbeddingClients{}
	audit := &recordingEmbeddingAudit{}
	resolver := newTestEmbeddingResolver(t, s, nil, clients, audit)

	ctx := withAIActionMetering(context.Background(), embeddingTestWorkspace, aipolicy.ActionSupportKnowledgeEmbed,
		"support_knowledge_embed", "query", map[string]interface{}{"surface": "support_knowledge"})
	response, err := resolver.CreateEmbeddings(ctx, llm.EmbeddingRequest{Model: defaultDocsEmbeddingModel, Inputs: []string{"reset password"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Vectors) != 1 || len(response.Vectors[0]) != docsEmbeddingDimensions {
		t.Fatalf("vectors = %d, want one %d-dimension vector", len(response.Vectors), docsEmbeddingDimensions)
	}
	client := clients.last(t)
	if len(client.requests) != 1 || client.requests[0].Model != defaultDocsEmbeddingModel {
		t.Fatalf("workspace client requests = %+v, want model %s", client.requests, defaultDocsEmbeddingModel)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("audit started=%d finished=%d, want 1/1", len(audit.started), len(audit.finished))
	}
	execution := audit.started[0]
	if execution.WorkspaceID != embeddingTestWorkspace || execution.ActionKey != aipolicy.ActionSupportKnowledgeEmbed ||
		execution.Provider != "openai" || execution.Model != defaultDocsEmbeddingModel || execution.Modality != string(aipolicy.ModalityEmbedding) {
		t.Fatalf("audit execution = %+v", execution)
	}
	metadata := string(execution.Metadata)
	for _, want := range []string{`"embedding_source":"workspace"`, `"embedding_provider":"openrouter"`, `"connection_id":"c-openrouter"`, `"funding_mode":"customer_unbilled"`, `"surface":"support_knowledge"`} {
		if !strings.Contains(metadata, want) {
			t.Errorf("audit metadata %s missing %s", metadata, want)
		}
	}
	if strings.Contains(metadata, "key-c-openrouter") {
		t.Fatal("audit metadata contains the API key")
	}
	if audit.finished[0].Status != model.AIActionExecutionSucceeded {
		t.Fatalf("audit status = %s", audit.finished[0].Status)
	}
}

func TestWorkspaceEmbeddingResolverRequiresMeteringContext(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, time.Now())
	resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{}, nil)
	_, err := resolver.CreateEmbeddings(context.Background(), llm.EmbeddingRequest{Inputs: []string{"x"}})
	if !errors.Is(err, ErrAIUsageMeteringRequired) {
		t.Fatalf("err = %v, want ErrAIUsageMeteringRequired", err)
	}
}

func TestWorkspaceEmbeddingResolverReportsUnavailable(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{}, nil)
	ctx := withAIActionMetering(context.Background(), embeddingTestWorkspace, aipolicy.ActionDocsEmbed, "docs", "x", nil)
	_, err := resolver.CreateEmbeddings(ctx, llm.EmbeddingRequest{Inputs: []string{"x"}})
	if !errors.Is(err, ErrEmbeddingsUnavailable) {
		t.Fatalf("err = %v, want ErrEmbeddingsUnavailable", err)
	}
}

func TestWorkspaceEmbeddingResolverRejectsWrongDimensions(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, time.Now())
	resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{dimensions: 3072}, &recordingEmbeddingAudit{})
	ctx := withAIActionMetering(context.Background(), embeddingTestWorkspace, aipolicy.ActionDocsEmbed, "docs", "x", nil)
	_, err := resolver.CreateEmbeddings(ctx, llm.EmbeddingRequest{Provider: "openai", Model: defaultDocsEmbeddingModel, Inputs: []string{"x"}})
	if !errors.Is(err, ErrEmbeddingDimensionMismatch) {
		t.Fatalf("err = %v, want ErrEmbeddingDimensionMismatch", err)
	}
}

func TestWorkspaceEmbeddingResolverAppliesEditionPolicy(t *testing.T) {
	flat := &model.AIExecutionPolicySnapshot{Mode: "ee", FundingMode: aiusage.FundingCustomerFlat}
	tests := []struct {
		name        string
		policy      fakeAdmissionPolicy
		wantSource  bool
		wantFunding aiusage.FundingMode
		wantErr     bool
	}{
		{name: "BYOK admitted", policy: fakeAdmissionPolicy{snapshot: flat}, wantSource: true, wantFunding: aiusage.FundingCustomerFlat},
		{name: "BYOK unavailable skips the connection", policy: fakeAdmissionPolicy{err: fmt.Errorf("%w: disabled", ErrAIConnectionPolicyUnavailable)}},
		{name: "policy failure is terminal", policy: fakeAdmissionPolicy{err: errors.New("database down")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := setupAIConnectionTest(t)
			s.admissionPolicy = tt.policy
			addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, time.Now())
			resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{}, nil)
			info, err := resolver.EmbeddingSource(context.Background(), embeddingTestWorkspace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if info.Available() != tt.wantSource || info.FundingMode != tt.wantFunding {
				t.Fatalf("source = %+v, want available=%v funding=%q", info, tt.wantSource, tt.wantFunding)
			}
		})
	}
}

func TestDocsIndexingAndHelpcenterQueryUseWorkspaceConnection(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openrouter", provider: "openrouter"}, time.Now())
	clients := &fakeEmbeddingClients{}
	audit := &recordingEmbeddingAudit{}
	resolver := newTestEmbeddingResolver(t, s, nil, clients, audit)

	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()
	insertKnowledgeSpace(t, db, embeddingTestWorkspace, "public-space", model.SpaceTypeExternalCapable)
	for _, statement := range []string{
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, updated_at) VALUES ('public-doc', 'workspace', 'public-space', 'Pricing', 'published', CURRENT_TIMESTAMP)`,
		`INSERT INTO docs_helpcenter_articles (document_id, public_published_at) VALUES ('public-doc', CURRENT_TIMESTAMP)`,
		`INSERT INTO docs_contents (id, document_id, content, content_text, word_count, created_at, updated_at) VALUES ('content-1', 'public-doc', X'7B7D', 'The Growth plan costs $84 per month.', 7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	chunkRepo := repository.NewDocsChunkRepository(db)
	docs := NewDocsEmbeddingService(chunkRepo, nil, repository.NewAgentKnowledgeSourceRepository(db),
		repository.NewDocsContentRepository(db), repository.NewDocsSpaceRepository(db),
		repository.NewDocsHelpcenterRepository(db, false), repository.NewDocsDocumentRepository(db),
		resolver, "", nil)
	if err := docs.RunSpaceSync(ctx, embeddingTestWorkspace, "public-space"); err != nil {
		t.Fatalf("RunSpaceSync: %v", err)
	}
	var chunks []model.DocsChunk
	if err := db.Find(&chunks).Error; err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].EmbeddingModel != defaultDocsEmbeddingModel || chunks[0].EmbeddingProvider != "openai" ||
		chunks[0].EmbeddingDimensions != docsEmbeddingDimensions {
		t.Fatalf("chunks = %+v, want one openai/%s/%d chunk", chunks, defaultDocsEmbeddingModel, docsEmbeddingDimensions)
	}

	search := NewHelpcenterAISearchService(chunkRepo, nil, nil, resolver, "", nil, "", "", nil)
	if _, err := search.retrievePublicChunks(ctx, embeddingTestWorkspace, "growth plan", 5); err != nil {
		t.Fatalf("retrievePublicChunks: %v", err)
	}

	var requests []llm.EmbeddingRequest
	for _, client := range clients.built {
		if client.provider != "openrouter" {
			t.Fatalf("client provider = %s, want openrouter", client.provider)
		}
		requests = append(requests, client.requests...)
	}
	if len(requests) != 2 {
		t.Fatalf("workspace embedding requests = %d, want index + query", len(requests))
	}
	for _, req := range requests {
		if req.Model != defaultDocsEmbeddingModel {
			t.Fatalf("request model = %q, want %q", req.Model, defaultDocsEmbeddingModel)
		}
	}
	if requests[1].Inputs[0] != "growth plan" {
		t.Fatalf("query input = %v", requests[1].Inputs)
	}
	actions := []string{}
	for _, execution := range audit.started {
		actions = append(actions, execution.ActionKey)
	}
	if strings.Join(actions, ",") != aipolicy.ActionDocsEmbed+","+aipolicy.ActionHelpcenterSearchEmbed {
		t.Fatalf("audited actions = %v", actions)
	}
}

func TestQueueSpaceSyncDisabledWithoutEmbeddingSource(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	resolver := newTestEmbeddingResolver(t, s, nil, &fakeEmbeddingClients{}, nil)
	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()
	insertKnowledgeSpace(t, db, embeddingTestWorkspace, "internal-space", model.SpaceTypeInternal)
	if err := db.Exec(`INSERT INTO agent_knowledge_sources (id, agent_id, space_id, workspace_id, sync_status, created_at, updated_at)
		VALUES ('source-1', 'agent-1', 'internal-space', 'workspace', 'queued', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	starter := &recordingEmbeddingSyncStarter{}
	knowledgeRepo := repository.NewAgentKnowledgeSourceRepository(db)
	docs := NewDocsEmbeddingService(repository.NewDocsChunkRepository(db), nil, knowledgeRepo,
		repository.NewDocsContentRepository(db), repository.NewDocsSpaceRepository(db),
		repository.NewDocsHelpcenterRepository(db, false), repository.NewDocsDocumentRepository(db), resolver, "", starter)

	if err := docs.QueueSpaceSync(ctx, embeddingTestWorkspace, "internal-space"); err != nil {
		t.Fatal(err)
	}
	source, err := knowledgeRepo.GetByID(ctx, "source-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(starter.queued) != 0 || source.SyncStatus != model.KnowledgeSourceSyncDisabled || source.LastSyncError == nil ||
		!strings.Contains(*source.LastSyncError, "connect OpenAI or OpenRouter") {
		t.Fatalf("queued=%v source=%+v, want disabled with setup guidance", starter.queued, source)
	}

	// The backfill picks the disabled source up once a connection exists.
	addEmbeddingConnection(t, s, embeddingConnectionSpec{id: "c-openai", provider: "openai"}, time.Now())
	resolver.InvalidateEmbeddingSource(embeddingTestWorkspace)
	if err := NewKnowledgeEmbeddingBackfill(docs, nil).QueueMissingEmbeddings(ctx, embeddingTestWorkspace); err != nil {
		t.Fatal(err)
	}
	if len(starter.queued) != 1 || starter.queued[0] != "workspace/internal-space" {
		t.Fatalf("backfill queued %v, want workspace/internal-space", starter.queued)
	}
}

func TestQueueMissingEmbeddingsSkipsIndexedKnowledge(t *testing.T) {
	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()
	insertKnowledgeSpace(t, db, "ws-1", "ready-space", model.SpaceTypeInternal)
	insertKnowledgeSpace(t, db, "ws-1", "failed-space", model.SpaceTypeInternal)
	insertKnowledgeSpace(t, db, "ws-1", "public-space", model.SpaceTypeExternalCapable)
	if err := db.Exec(`INSERT INTO agent_knowledge_sources (id, agent_id, space_id, workspace_id, sync_status, indexed_chunks, created_at, updated_at) VALUES
		('ready', 'agent-1', 'ready-space', 'ws-1', 'ready', 4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('failed', 'agent-1', 'failed-space', 'ws-1', 'failed', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	starter := &recordingEmbeddingSyncStarter{}
	docs := NewDocsEmbeddingService(repository.NewDocsChunkRepository(db), nil, repository.NewAgentKnowledgeSourceRepository(db),
		repository.NewDocsContentRepository(db), repository.NewDocsSpaceRepository(db),
		repository.NewDocsHelpcenterRepository(db, false), repository.NewDocsDocumentRepository(db),
		internalKnowledgeEmbeddingProvider{}, "", starter)
	if err := docs.QueueMissingEmbeddings(ctx, "ws-1"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(starter.queued, ",") != "ws-1/failed-space,ws-1/public-space" {
		t.Fatalf("queued = %v, want failed and unindexed public spaces only", starter.queued)
	}
}

func TestAddChangeObserverNotifiesEveryObserver(t *testing.T) {
	s, _ := setupAIConnectionTest(t)
	first, second := &countingObserver{err: errors.New("first failed")}, &countingObserver{}
	s.AddChangeObserver(first).AddChangeObserver(second)
	s.notifyConnectionsChanged(context.Background(), &model.AIConnection{ID: "c", WorkspaceID: embeddingTestWorkspace, Scope: "workspace"})
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("observer calls = %d/%d, want 1/1", first.calls, second.calls)
	}
}

type countingObserver struct {
	calls int
	err   error
}

func (o *countingObserver) AIConnectionsChanged(context.Context, string) error {
	o.calls++
	return o.err
}
