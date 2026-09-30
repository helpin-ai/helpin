package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	externalA2ARequestBodyLimit = 64 << 10
	// Multipart framing around the largest accepted file.
	externalA2AUploadBodyLimit = 201 << 20
)

// ExternalA2AHandler manages workspace connections to external A2A agents and
// receives files those agents upload with a per-run upload link.
type ExternalA2AHandler struct {
	service *service.ExternalA2AService
}

// NewExternalA2AHandler creates the external agents handler.
func NewExternalA2AHandler(externalA2A *service.ExternalA2AService) *ExternalA2AHandler {
	return &ExternalA2AHandler{service: externalA2A}
}

// List returns the workspace's external agents.
func (h *ExternalA2AHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), middleware.GetWorkspaceID(r.Context()))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	if items == nil {
		items = []model.ExternalA2AAgent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Preview fetches and validates an agent card without saving it.
func (h *ExternalA2AHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req model.PreviewExternalA2AAgentRequest
	if err := decodeExternalA2AJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	card, err := h.service.Preview(r.Context(), req)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"card": card})
}

// Create connects an external agent and creates its Helpin agent.
func (h *ExternalA2AHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateExternalA2AAgentRequest
	if err := decodeExternalA2AJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agent, err := h.service.Create(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

// Update edits an external agent's name, token, team scope or status.
func (h *ExternalA2AHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateExternalA2AAgentRequest
	if err := decodeExternalA2AJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agent, err := h.service.Update(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "externalAgentID"), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// RefreshCard re-fetches the external agent's card.
func (h *ExternalA2AHandler) RefreshCard(w http.ResponseWriter, r *http.Request) {
	agent, err := h.service.RefreshCard(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "externalAgentID"), middleware.GetUserID(r.Context()))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// Delete removes the connection and its linked Helpin agent.
func (h *ExternalA2AHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "externalAgentID"), middleware.GetUserID(r.Context())); err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Upload receives one multipart "file" from an external agent authenticated
// by its run's upload token, and attaches it to the run's task.
func (h *ExternalA2AHandler) Upload(w http.ResponseWriter, r *http.Request) {
	bearer, ok := strings.CutPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ")
	if !ok || strings.TrimSpace(bearer) == "" {
		writeError(w, http.StatusUnauthorized, "upload token is required")
		return
	}
	grant, err := h.service.AuthorizeUpload(r.Context(), bearer)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, externalA2AUploadBodyLimit)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "send the file as multipart/form-data in the \"file\" field")
		return
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "could not read the multipart body")
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		result, err := h.service.ReceiveUpload(r.Context(), grant, part.FileName(), part)
		_ = part.Close()
		if err != nil {
			var maxBytes *http.MaxBytesError
			if errors.As(err, &maxBytes) {
				writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the 200 MB limit")
				return
			}
			h.writeServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
		return
	}
	writeError(w, http.StatusBadRequest, "multipart field \"file\" is required")
}

func (h *ExternalA2AHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var entitlementErr *service.EntitlementError
	if errors.As(err, &entitlementErr) {
		writeBillingAwareError(w, http.StatusPaymentRequired, err)
		return
	}
	status := service.ExternalA2AErrorStatus(err)
	if status >= http.StatusInternalServerError && status != http.StatusServiceUnavailable {
		slog.ErrorContext(r.Context(), "external agent request failed", "error", err, "path", r.URL.Path)
		writeError(w, status, "external agent request failed")
		return
	}
	writeError(w, status, err.Error())
}

func decodeExternalA2AJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, externalA2ARequestBodyLimit)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
