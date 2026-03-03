package handler

import (
	"net/http"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// InviteHandler handles invite HTTP requests (stub).
type InviteHandler struct{}

// NewInviteHandler creates a new InviteHandler.
func NewInviteHandler() *InviteHandler {
	return &InviteHandler{}
}

// Send handles POST /api/invite. Currently a stub.
func (h *InviteHandler) Send(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "invite sent (stub)"})
}
