package handler

import (
	"log/slog"
	"net/http"

	"github.com/helpin-ai/helpin/server/internal/storage"
)

// HealthHandler handles health check and system setup requests.
type HealthHandler struct {
	s3Client *storage.S3Client
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(s3Client *storage.S3Client) *HealthHandler {
	return &HealthHandler{s3Client: s3Client}
}

// Check returns a 200 OK health check response.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// EnsureStorageCORS sets the S3 bucket CORS policy so browsers on any origin
// can upload files via presigned PUT URLs. Call once after deployment instead
// of blocking server startup.
// GET /api/system/ensure-cors
func (h *HealthHandler) EnsureStorageCORS(w http.ResponseWriter, r *http.Request) {
	if h.s3Client == nil {
		writeError(w, http.StatusServiceUnavailable, "S3 storage not configured")
		return
	}
	if err := h.s3Client.EnsureCORS(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "ensure storage CORS failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set storage CORS policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
