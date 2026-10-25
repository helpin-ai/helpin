package handler

import (
	"context"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/middleware"
)

type iconSuggester interface {
	Suggest(context.Context, string, string, string) (string, error)
}

type IconSuggestionHandler struct{ service iconSuggester }

func NewIconSuggestionHandler(service iconSuggester) *IconSuggestionHandler {
	return &IconSuggestionHandler{service: service}
}
func (h *IconSuggestionHandler) Collection(w http.ResponseWriter, r *http.Request) {
	h.suggest(w, r, "collection")
}
func (h *IconSuggestionHandler) Inbox(w http.ResponseWriter, r *http.Request) {
	h.suggest(w, r, "inbox")
}
func (h *IconSuggestionHandler) suggest(w http.ResponseWriter, r *http.Request, kind string) {
	var req struct {
		Name string `json:"name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if err := decodeJSON(r, &req); err != nil || utf8.RuneCountInString(req.Name) > 160 {
		writeError(w, http.StatusBadRequest, "Provide a name of at most 160 characters")
		return
	}
	workspaceID := middleware.GetWorkspaceID(r.Context())
	icon, err := h.service.Suggest(r.Context(), workspaceID, kind, req.Name)
	if err != nil {
		slog.WarnContext(r.Context(), "icon suggestion unavailable", "workspace_id", workspaceID, "kind", kind, "error", err)
		icon = ""
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"icon": icon})
}
