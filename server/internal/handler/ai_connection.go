package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// KnowledgeAIConfiguration describes how the workspace's knowledge search is
// wired. EmbeddingSource is "server", "workspace", or "" when no provider
// serves the workspace; EmbeddingDetail names the source for display.
type KnowledgeAIConfiguration struct {
	EmbeddingsConfigured bool     `json:"embeddings_configured"`
	EmbeddingModel       string   `json:"embedding_model"`
	EmbeddingDimensions  int      `json:"embedding_dimensions"`
	EmbeddingSource      string   `json:"embedding_source"`
	EmbeddingProvider    string   `json:"embedding_provider,omitempty"`
	EmbeddingDetail      string   `json:"embedding_detail,omitempty"`
	ChatProviders        []string `json:"chat_providers"`
}

// KnowledgeEmbeddingSourceFunc resolves a workspace's embedding source.
type KnowledgeEmbeddingSourceFunc func(ctx context.Context, workspaceID string) (service.EmbeddingSourceInfo, error)

type AIConnectionHandler struct {
	service         *service.AIConnectionService
	knowledge       KnowledgeAIConfiguration
	embeddingSource KnowledgeEmbeddingSourceFunc
}

// Configuration describes wiring, not a successful provider health check.
func (h *AIConnectionHandler) SetKnowledgeConfiguration(embeddings bool, embeddingModel string, chatProviders []string) {
	if embeddingModel == "" {
		embeddingModel = "text-embedding-3-small"
	}
	if chatProviders == nil {
		chatProviders = []string{}
	}
	h.knowledge = KnowledgeAIConfiguration{
		EmbeddingsConfigured: embeddings, EmbeddingModel: embeddingModel,
		EmbeddingDimensions: 1536, ChatProviders: chatProviders,
	}
	if embeddings {
		h.knowledge.EmbeddingSource = service.EmbeddingSourceServer
	}
}

// SetKnowledgeEmbeddingSource makes the knowledge configuration reflect each
// workspace's own embedding source (server key or workspace connection).
func (h *AIConnectionHandler) SetKnowledgeEmbeddingSource(resolve KnowledgeEmbeddingSourceFunc) {
	h.embeddingSource = resolve
}

// knowledgeFor returns the knowledge configuration as seen by one workspace.
func (h *AIConnectionHandler) knowledgeFor(ctx context.Context, workspaceID string) KnowledgeAIConfiguration {
	knowledge := h.knowledge
	if h.embeddingSource == nil {
		return knowledge
	}
	source, err := h.embeddingSource(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "resolve knowledge embedding source failed", "workspace_id", workspaceID, "error", err)
		return knowledge
	}
	knowledge.EmbeddingsConfigured = source.Available()
	knowledge.EmbeddingSource = source.Source
	knowledge.EmbeddingProvider = source.Provider
	if source.Available() {
		knowledge.EmbeddingDetail = source.Detail()
		if source.Model != "" {
			knowledge.EmbeddingModel = source.Model
		}
		if source.Dimensions > 0 {
			knowledge.EmbeddingDimensions = source.Dimensions
		}
	}
	return knowledge
}

func NewAIConnectionHandler(s *service.AIConnectionService) *AIConnectionHandler {
	return &AIConnectionHandler{service: s}
}
func (h *AIConnectionHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := middleware.GetWorkspaceID(r.Context())
	connections, err := h.service.List(r.Context(), workspaceID, middleware.GetUserID(r.Context()))
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": h.service.Enabled(), "connections": connections, "models": h.service.Models(), "knowledge": h.knowledgeFor(r.Context(), workspaceID)})
}
func decodeAIConnection(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("unexpected request data")
	}
	return nil
}
func (h *AIConnectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAIConnectionRequest
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, 400, "invalid connection request")
		return
	}
	result, err := h.service.Create(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 201, result)
}
func (h *AIConnectionHandler) Poll(w http.ResponseWriter, r *http.Request) {
	workspace, user, id := middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID")
	result, err := h.service.Poll(r.Context(), workspace, user, id)
	if err != nil {
		h.failure(w, err)
		return
	}
	if result.Connection.Status == "connected" {
		if err := h.service.ReauthorizeRuns(r.Context(), workspace, user, id); err != nil {
			writeError(w, 409, "Connected. Retry to resume affected runs.")
			return
		}
	}
	writeJSON(w, 200, result)
}
func (h *AIConnectionHandler) Reconnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key,omitempty"`
	}
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, 400, "invalid connection request")
		return
	}
	workspace, user, id := middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID")
	result, err := h.service.Reconnect(r.Context(), workspace, user, id, req.APIKey)
	if err != nil {
		h.failure(w, err)
		return
	}
	if result.Connection.Status == "connected" {
		if err := h.service.ReauthorizeRuns(r.Context(), workspace, user, id); err != nil {
			writeError(w, 409, "Connected. Retry to resume affected runs.")
			return
		}
	}
	writeJSON(w, 200, result)
}
func (h *AIConnectionHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Disconnect(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID")); err != nil {
		h.failure(w, err)
		return
	}
	w.WriteHeader(204)
}

// Test runs a minimal live completion with the connection's credential. The
// body is optional; {"model": "..."} selects a specific model.
func (h *AIConnectionHandler) Test(w http.ResponseWriter, r *http.Request) {
	var req model.TestAIConnectionRequest
	if err := decodeAIConnection(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, 400, "invalid connection test request")
		return
	}
	workspace, user, id := middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID")
	result, err := h.service.TestConnection(r.Context(), workspace, user, id, req.Model)
	if err != nil {
		slog.ErrorContext(r.Context(), "AI connection test failed", "error", err, "workspace_id", workspace, "connection_id", id)
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, result)
}

// Refresh is mounted behind RequireInternalAPISecret, never user-supplied auth.
func (h *AIConnectionHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req sdk.ModelCredentialRefreshRequest
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, 400, "invalid credential request")
		return
	}
	response, err := h.service.RefreshRun(r.Context(), req)
	if err != nil {
		writeError(w, 401, "AI connection requires reconnection")
		return
	}
	writeJSON(w, 200, response)
}
func (h *AIConnectionHandler) failure(w http.ResponseWriter, err error) {
	var authErr *chatgptauth.AuthError
	if errors.As(err, &authErr) {
		writeError(w, 400, authErr.Error())
		return
	}
	if errors.Is(err, service.ErrAIConnection) {
		writeError(w, 403, "AI connection is unavailable or requires reconnection")
		return
	}
	writeError(w, 400, "Unable to update AI connection. Check the provider, model, and credentials, then retry.")
}

// Endpoints exposes approved destinations to authenticated workspace members.
func (h *AIConnectionHandler) Endpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.service.AvailableEndpoints(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()))
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, endpoints)
}

func (h *AIConnectionHandler) Models(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ConnectionModels(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID"), r.Method == http.MethodPost)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
