package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportQuickstartHandler serves the onboarding that sets up support from a
// company website.
type SupportQuickstartHandler struct {
	service *service.SupportQuickstartService
}

// NewSupportQuickstartHandler creates the handler.
func NewSupportQuickstartHandler(svc *service.SupportQuickstartService) *SupportQuickstartHandler {
	return &SupportQuickstartHandler{service: svc}
}

// Preview handles POST /api/onboarding/support/preview. The response is a
// stream of server-sent events, one JSON SupportQuickstartEvent each, written
// as the preview runs.
func (h *SupportQuickstartHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req model.SupportQuickstartPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	flusher := http.NewResponseController(w)
	started := false
	emit := func(event model.SupportQuickstartEvent) {
		if !started {
			started = true
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			// Tells nginx-style proxies not to hold events back.
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
		}
		payload, err := json.Marshal(event)
		if err != nil {
			slog.ErrorContext(r.Context(), "support quickstart preview event", "error", err)
			return
		}
		// A failed write means the visitor left; the request context ends the work.
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err == nil {
			if err := flusher.Flush(); err != nil {
				slog.DebugContext(r.Context(), "support quickstart preview flush", "error", err)
			}
		}
	}
	err := h.service.Preview(r.Context(), req, emit)
	if err == nil {
		return
	}
	if !started {
		writeQuickstartError(w, r, err)
		return
	}
	slog.WarnContext(r.Context(), "support quickstart preview", "error", err)
	emit(model.SupportQuickstartEvent{Type: model.QuickstartEventFailed, Text: "We couldn’t finish reading your site. Try again."})
}

// PreviewAnswer handles POST /api/onboarding/support/preview-answer.
func (h *SupportQuickstartHandler) PreviewAnswer(w http.ResponseWriter, r *http.Request) {
	var req model.SupportQuickstartAnswerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	answer, err := h.service.PreviewAnswer(r.Context(), req)
	if errors.Is(err, service.ErrSupportQuickstartBusy) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "So many people are trying Helpin right now that previews are paused. Create your account to keep going.")
		return
	}
	if err != nil && !errors.Is(err, service.ErrSupportQuickstartInvalid) {
		slog.WarnContext(r.Context(), "support quickstart preview answer", "error", err)
		writeError(w, http.StatusServiceUnavailable, "Your agent couldn’t answer just now. Try again.")
		return
	}
	if err != nil {
		writeQuickstartError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

// Start handles POST /api/workspaces/{id}/support/quickstart.
func (h *SupportQuickstartHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req model.SupportQuickstartStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	status, err := h.service.Start(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeQuickstartError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

// Status handles GET /api/workspaces/{id}/support/quickstart.
func (h *SupportQuickstartHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Status(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeQuickstartError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func writeQuickstartError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, service.ErrSupportQuickstartInvalid) {
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrSupportQuickstartInvalid.Error()+": "))
		return
	}
	slog.ErrorContext(r.Context(), "support quickstart", "error", err, "workspace_id", getWorkspaceID(r))
	writeError(w, http.StatusInternalServerError, "Support setup didn’t finish. Try again.")
}
