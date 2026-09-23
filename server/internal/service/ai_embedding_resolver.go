package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Embedding sources reported by WorkspaceEmbeddingResolver.
const (
	// EmbeddingSourceServer means the server's OPENAI_API_KEY or
	// OPENROUTER_API_KEY serves embeddings for every workspace.
	EmbeddingSourceServer = "server"
	// EmbeddingSourceWorkspace means one of the workspace's shared AI
	// connections serves its embeddings.
	EmbeddingSourceWorkspace = "workspace"
)

const (
	defaultEmbeddingSourceTTL = 30 * time.Second
	embeddingBackfillTimeout  = 2 * time.Minute
)

// ErrEmbeddingsUnavailable means neither the server nor the workspace has an
// embedding provider. Callers fall back to keyword retrieval.
var ErrEmbeddingsUnavailable = errors.New("no embedding provider is configured for this workspace")

// ErrEmbeddingDimensionMismatch means a provider returned vectors that cannot
// share an index with the stored 1,536-dimension knowledge vectors.
var ErrEmbeddingDimensionMismatch = errors.New("embedding dimension mismatch")

// workspaceEmbeddingProviders lists the connection providers that can serve
// text-embedding-3-small, in preference order. Anthropic has no embeddings API,
// and OpenAI-compatible endpoints carry no embedding model configuration, so
// neither is ever selected.
var workspaceEmbeddingProviders = []string{"openai", "openrouter"}

// EmbeddingSourceInfo describes where a workspace's embeddings come from. It
// never carries credential material.
type EmbeddingSourceInfo struct {
	// Source is EmbeddingSourceServer, EmbeddingSourceWorkspace, or "" when
	// no provider is available.
	Source string
	// Provider is the transport that serves the vectors: "openai" or "openrouter".
	Provider       string
	ConnectionID   string
	ConnectionName string
	// Model is the logical embedding model; stored vectors are keyed by it.
	Model      string
	Dimensions int
	// FundingMode is set for workspace sources: customer_unbilled in
	// Community, or the Enterprise policy's funding mode.
	FundingMode aiusage.FundingMode
}

// Available reports whether any embedding provider serves the workspace.
func (i EmbeddingSourceInfo) Available() bool { return i.Source != "" }

// Detail is a short, user-facing sentence naming the source.
func (i EmbeddingSourceInfo) Detail() string {
	provider := embeddingProviderLabel(i.Provider)
	switch i.Source {
	case EmbeddingSourceServer:
		return "Using the server's " + provider + " key"
	case EmbeddingSourceWorkspace:
		return "Using this workspace's " + provider + " connection"
	default:
		return "No embedding provider is configured"
	}
}

// EmbeddingSelection is a resolved embedding source with a governed client.
// It is process-local and must never be persisted: the client holds a key.
type EmbeddingSelection struct {
	Info   EmbeddingSourceInfo
	Client llm.EmbeddingProvider
}

// EmbeddingClientFactory builds a raw embeddings client for a workspace
// connection's provider and API key. Tests replace it with a fake.
type EmbeddingClientFactory func(provider, apiKey string) (llm.EmbeddingProvider, error)

// WorkspaceEmbeddingBackfiller queues indexing of knowledge that has no vectors yet.
type WorkspaceEmbeddingBackfiller interface {
	QueueMissingEmbeddings(ctx context.Context, workspaceID string) error
}

type embeddingSourceEntry struct {
	selection *EmbeddingSelection
	expires   time.Time
}

// WorkspaceEmbeddingResolver chooses the embedding provider for each
// workspace: the server's configured key first (instance-wide), then the
// workspace's own OpenAI connection, then its OpenRouter connection. Every
// option requests the same logical model, so stored vectors stay comparable.
//
// It implements llm.EmbeddingProvider by reading the workspace from the AI
// usage metering context, so existing call sites route through it unchanged.
type WorkspaceEmbeddingResolver struct {
	server      llm.EmbeddingProvider
	serverInfo  EmbeddingSourceInfo
	model       string
	connections *AIConnectionService
	meter       *AIUsageMeter
	registry    *aipolicy.Registry
	audit       aipolicy.ExecutionAudit
	clients     EmbeddingClientFactory
	backfill    WorkspaceEmbeddingBackfiller
	ttl         time.Duration
	now         func() time.Time

	mu      sync.Mutex
	cache   map[string]embeddingSourceEntry
	pending sync.WaitGroup
}

// WorkspaceEmbeddingResolverConfig wires a WorkspaceEmbeddingResolver.
type WorkspaceEmbeddingResolverConfig struct {
	// Server is the already governed server embedding provider, or nil.
	Server llm.EmbeddingProvider
	// ServerProvider names the server transport ("openai" or "openrouter").
	ServerProvider string
	// Model is the logical embedding model; empty means text-embedding-3-small.
	Model       string
	Connections *AIConnectionService
	Meter       *AIUsageMeter
	Registry    *aipolicy.Registry
	Audit       aipolicy.ExecutionAudit
}

// NewWorkspaceEmbeddingResolver creates a resolver. It is always non-nil so
// that workspaces can gain embeddings after startup.
func NewWorkspaceEmbeddingResolver(cfg WorkspaceEmbeddingResolverConfig) *WorkspaceEmbeddingResolver {
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = defaultDocsEmbeddingModel
	}
	r := &WorkspaceEmbeddingResolver{
		server: cfg.Server, model: modelName, connections: cfg.Connections,
		meter: cfg.Meter, registry: cfg.Registry, audit: cfg.Audit,
		clients: defaultEmbeddingClient, ttl: defaultEmbeddingSourceTTL, now: time.Now,
		cache: map[string]embeddingSourceEntry{},
	}
	if cfg.Server != nil {
		provider := strings.TrimSpace(cfg.ServerProvider)
		if provider == "" {
			provider = "openai"
		}
		r.serverInfo = EmbeddingSourceInfo{
			Source: EmbeddingSourceServer, Provider: provider, Model: modelName,
			Dimensions: docsEmbeddingDimensions,
		}
	}
	return r
}

// SetClientFactory replaces the raw clients built for workspace connections.
func (r *WorkspaceEmbeddingResolver) SetClientFactory(factory EmbeddingClientFactory) *WorkspaceEmbeddingResolver {
	if factory != nil {
		r.clients = factory
	}
	return r
}

// SetBackfiller registers the indexer queued when a workspace gains embeddings.
func (r *WorkspaceEmbeddingResolver) SetBackfiller(backfill WorkspaceEmbeddingBackfiller) *WorkspaceEmbeddingResolver {
	r.backfill = backfill
	return r
}

// ServerConfigured reports whether a server key serves every workspace.
func (r *WorkspaceEmbeddingResolver) ServerConfigured() bool { return r != nil && r.server != nil }

// ServerSource describes the server embedding source; it is unavailable when
// no server key is configured.
func (r *WorkspaceEmbeddingResolver) ServerSource() EmbeddingSourceInfo {
	if r == nil {
		return EmbeddingSourceInfo{}
	}
	return r.serverInfo
}

// EmbeddingProviderFor returns the workspace's embedding source, or nil when
// none is available. Lookups are cached briefly per workspace.
func (r *WorkspaceEmbeddingResolver) EmbeddingProviderFor(ctx context.Context, workspaceID string) (*EmbeddingSelection, error) {
	if r == nil {
		return nil, nil
	}
	if r.server != nil {
		return &EmbeddingSelection{Info: r.serverInfo, Client: r.server}, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, nil
	}
	now := r.now()
	r.mu.Lock()
	entry, ok := r.cache[workspaceID]
	r.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.selection, nil
	}
	selection, err := r.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[workspaceID] = embeddingSourceEntry{selection: selection, expires: now.Add(r.ttl)}
	r.mu.Unlock()
	return selection, nil
}

// EmbeddingSource describes the workspace's embedding source without the client.
func (r *WorkspaceEmbeddingResolver) EmbeddingSource(ctx context.Context, workspaceID string) (EmbeddingSourceInfo, error) {
	selection, err := r.EmbeddingProviderFor(ctx, workspaceID)
	if err != nil || selection == nil {
		return EmbeddingSourceInfo{}, err
	}
	return selection.Info, nil
}

// EmbeddingsAvailable reports whether the workspace can create embeddings.
// Lookup failures report false so callers keep keyword retrieval.
func (r *WorkspaceEmbeddingResolver) EmbeddingsAvailable(ctx context.Context, workspaceID string) bool {
	selection, err := r.EmbeddingProviderFor(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "resolve workspace embedding source failed", "workspace_id", workspaceID, "error", err)
		return false
	}
	return selection != nil
}

// InvalidateEmbeddingSource drops the cached source for a workspace.
func (r *WorkspaceEmbeddingResolver) InvalidateEmbeddingSource(workspaceID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.cache, strings.TrimSpace(workspaceID))
	r.mu.Unlock()
}

// CreateEmbeddings routes one embedding request to the workspace named by the
// AI usage metering context. Workspace-funded requests are audited with their
// source, connection, and funding mode.
func (r *WorkspaceEmbeddingResolver) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	if r == nil {
		return nil, ErrEmbeddingsUnavailable
	}
	metering, ok := AIUsageMeteringFromContext(ctx)
	if !ok {
		if r.server != nil {
			// The governed server provider reports the missing context.
			return r.server.CreateEmbeddings(ctx, req)
		}
		return nil, ErrAIUsageMeteringRequired
	}
	selection, err := r.EmbeddingProviderFor(ctx, metering.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve embedding source: %w", err)
	}
	if selection == nil || selection.Client == nil {
		return nil, ErrEmbeddingsUnavailable
	}
	if selection.Info.Source == EmbeddingSourceWorkspace {
		metadata := make(map[string]interface{}, len(metering.Metadata)+4)
		for key, value := range metering.Metadata {
			metadata[key] = value
		}
		metadata["embedding_source"] = selection.Info.Source
		metadata["embedding_provider"] = selection.Info.Provider
		metadata["connection_id"] = selection.Info.ConnectionID
		metadata["funding_mode"] = string(selection.Info.FundingMode)
		metering.Metadata = metadata
		ctx = WithAIUsageMetering(ctx, metering)
	}
	return selection.Client.CreateEmbeddings(ctx, req)
}

// AIConnectionsChanged drops the cached source and, when the workspace now
// has a workspace-funded source, queues indexing of knowledge without vectors
// in the background. It never blocks the connection change.
func (r *WorkspaceEmbeddingResolver) AIConnectionsChanged(ctx context.Context, workspaceID string) error {
	if r == nil {
		return nil
	}
	r.InvalidateEmbeddingSource(workspaceID)
	if r.server != nil || r.backfill == nil {
		return nil
	}
	selection, err := r.EmbeddingProviderFor(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("resolve embedding source: %w", err)
	}
	if selection == nil {
		return nil
	}
	r.pending.Add(1)
	go r.runBackfill(context.WithoutCancel(ctx), workspaceID)
	return nil
}

// WaitForBackfills blocks until queued backfill requests have finished.
func (r *WorkspaceEmbeddingResolver) WaitForBackfills() {
	if r != nil {
		r.pending.Wait()
	}
}

func (r *WorkspaceEmbeddingResolver) runBackfill(ctx context.Context, workspaceID string) {
	defer r.pending.Done()
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "embedding backfill panicked", "workspace_id", workspaceID, "panic", recovered)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, embeddingBackfillTimeout)
	defer cancel()
	if err := r.backfill.QueueMissingEmbeddings(ctx, workspaceID); err != nil {
		slog.ErrorContext(ctx, "queue knowledge embedding backfill failed", "workspace_id", workspaceID, "error", err)
		return
	}
	slog.InfoContext(ctx, "knowledge embedding backfill queued", "workspace_id", workspaceID)
}

// resolveWorkspace picks the workspace's best shared connection: OpenAI before
// OpenRouter, and the generated standard connection before other connections
// of the same provider. Connections the edition policy does not admit are skipped.
func (r *WorkspaceEmbeddingResolver) resolveWorkspace(ctx context.Context, workspaceID string) (*EmbeddingSelection, error) {
	// Only the encryption key is needed: embeddings never run through Agent
	// Runtime, so a worker without a Runtime client still reads the connections.
	if r.connections == nil || r.connections.repo == nil || len(r.connections.key) != 32 {
		return nil, nil
	}
	candidates, err := r.connections.repo.SharedEmbeddingConnections(ctx, workspaceID, workspaceEmbeddingProviders)
	if err != nil {
		return nil, fmt.Errorf("list workspace AI connections: %w", err)
	}
	sortEmbeddingCandidates(workspaceID, candidates)
	for i := range candidates {
		c := &candidates[i]
		if c.WorkspaceID != workspaceID || c.Scope != "workspace" || c.UserID != nil || c.SupersededBy != nil ||
			c.Status != "connected" || len(c.EncryptedSecret) == 0 {
			continue
		}
		funding, admitted, err := r.embeddingFunding(ctx, workspaceID, c)
		if err != nil {
			return nil, err
		}
		if !admitted {
			continue
		}
		secret, err := r.connections.open(c)
		if err != nil || strings.TrimSpace(secret.APIKey) == "" {
			slog.WarnContext(ctx, "skip AI connection for embeddings: credential unavailable",
				"workspace_id", workspaceID, "connection_id", c.ID, "provider", c.Provider)
			continue
		}
		raw, err := r.clients(c.Provider, secret.APIKey)
		if err != nil || raw == nil {
			continue
		}
		info := EmbeddingSourceInfo{
			Source: EmbeddingSourceWorkspace, Provider: c.Provider, ConnectionID: c.ID,
			ConnectionName: c.Name, Model: r.model, Dimensions: docsEmbeddingDimensions, FundingMode: funding,
		}
		client := NewGovernedEmbeddingProvider(&dimensionCheckedEmbeddingProvider{base: raw}, r.meter, r.registry, r.audit)
		return &EmbeddingSelection{Info: info, Client: client}, nil
	}
	return nil, nil
}

// embeddingFunding applies the edition's admission policy. Community admits
// every workspace connection as customer_unbilled; Enterprise admits a
// connection only when its policy resolves (for example, BYOK is enabled).
func (r *WorkspaceEmbeddingResolver) embeddingFunding(ctx context.Context, workspaceID string, c *model.AIConnection) (aiusage.FundingMode, bool, error) {
	policy := r.connections.admissionPolicy
	if policy == nil {
		return aiusage.FundingCustomerUnbilled, true, nil
	}
	snapshot, err := policy.ResolveConnectionPolicy(ctx, workspaceID, c)
	if err != nil {
		if errors.Is(err, ErrAIConnectionPolicyUnavailable) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve AI connection policy: %w", err)
	}
	if snapshot == nil || snapshot.FundingMode == "" {
		return "", false, nil
	}
	return snapshot.FundingMode, true, nil
}

func sortEmbeddingCandidates(workspaceID string, candidates []model.AIConnection) {
	rank := func(c model.AIConnection) int {
		score := len(workspaceEmbeddingProviders) * 2
		for i, provider := range workspaceEmbeddingProviders {
			if c.Provider == provider {
				score = i * 2
				break
			}
		}
		if c.ID != model.StandardAIConnectionID(workspaceID, c.Provider) {
			score++
		}
		return score
	}
	sort.SliceStable(candidates, func(i, j int) bool { return rank(candidates[i]) < rank(candidates[j]) })
}

func defaultEmbeddingClient(provider, apiKey string) (llm.EmbeddingProvider, error) {
	switch provider {
	case "openai":
		if p := llm.NewOpenAIProvider(apiKey, "", ""); p != nil {
			return p, nil
		}
	case "openrouter":
		if p := llm.NewOpenRouterEmbeddingProvider(apiKey, ""); p != nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("provider %q cannot serve embeddings", provider)
}

func embeddingProviderLabel(provider string) string {
	switch provider {
	case "openai":
		return "OpenAI"
	case "openrouter":
		return "OpenRouter"
	case "":
		return "embedding"
	default:
		return provider
	}
}

// dimensionCheckedEmbeddingProvider rejects vectors that could not share the
// 1,536-dimension knowledge index, so a misbehaving route never mixes models.
type dimensionCheckedEmbeddingProvider struct {
	base llm.EmbeddingProvider
}

func (p *dimensionCheckedEmbeddingProvider) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	response, err := p.base.CreateEmbeddings(ctx, req)
	if err != nil || response == nil {
		return response, err
	}
	for _, vector := range response.Vectors {
		if len(vector) != docsEmbeddingDimensions {
			return nil, fmt.Errorf("%w: got %d want %d", ErrEmbeddingDimensionMismatch, len(vector), docsEmbeddingDimensions)
		}
	}
	return response, nil
}

// workspaceEmbeddingAvailability is implemented by embedding providers whose
// availability depends on the workspace.
type workspaceEmbeddingAvailability interface {
	EmbeddingsAvailable(ctx context.Context, workspaceID string) bool
}

// workspaceEmbeddingInvalidator is implemented by providers that cache a
// per-workspace source.
type workspaceEmbeddingInvalidator interface {
	InvalidateEmbeddingSource(workspaceID string)
}

// embeddingsAvailable reports whether provider can embed for the workspace.
// Providers that do not depend on the workspace are available when non-nil.
func embeddingsAvailable(ctx context.Context, provider llm.EmbeddingProvider, workspaceID string) bool {
	if provider == nil {
		return false
	}
	if aware, ok := provider.(workspaceEmbeddingAvailability); ok {
		return aware.EmbeddingsAvailable(ctx, workspaceID)
	}
	return true
}

// refreshEmbeddingSource drops a cached workspace source before long-running
// indexing, so a worker process sees connection changes made by the API.
func refreshEmbeddingSource(provider llm.EmbeddingProvider, workspaceID string) {
	if invalidator, ok := provider.(workspaceEmbeddingInvalidator); ok {
		invalidator.InvalidateEmbeddingSource(workspaceID)
	}
}

var (
	_ llm.EmbeddingProvider          = (*WorkspaceEmbeddingResolver)(nil)
	_ AIConnectionChangeObserver     = (*WorkspaceEmbeddingResolver)(nil)
	_ workspaceEmbeddingAvailability = (*WorkspaceEmbeddingResolver)(nil)
)
