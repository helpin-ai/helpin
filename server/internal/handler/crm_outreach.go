package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMOutreachHandler serves CRM email workflow endpoints and public opt-out links.
type CRMOutreachHandler struct{ service *service.CRMOutreachService }

// NewCRMOutreachHandler creates the HTTP workflow adapter.
func NewCRMOutreachHandler(s *service.CRMOutreachService) *CRMOutreachHandler {
	return &CRMOutreachHandler{service: s}
}

// Templates lists templates visible to the requesting workspace member.
func (h *CRMOutreachHandler) Templates(w http.ResponseWriter, r *http.Request) {
	ws, user := getWorkspaceID(r), middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	switch r.Method {
	case http.MethodGet:
		rows, err := h.service.Templates(r.Context(), ws, user)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	case http.MethodDelete:
		if err := h.service.DeleteTemplate(r.Context(), ws, user, id); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"deleted": true})
	default:
		var req model.CRMEmailTemplate
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, 400, "invalid template")
			return
		}
		row, err := h.service.SaveTemplate(r.Context(), ws, user, id, req)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, row)
	}
}

// Sequences lists sequence definitions in a workspace.
func (h *CRMOutreachHandler) Sequences(w http.ResponseWriter, r *http.Request) {
	ws, user := getWorkspaceID(r), middleware.GetUserID(r.Context())
	if r.Method == http.MethodGet {
		rows, err := h.service.Sequences(r.Context(), ws)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, rows)
		return
	}
	var req model.CRMEmailSequence
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid sequence")
		return
	}
	row, err := h.service.SaveSequence(r.Context(), ws, user, chi.URLParam(r, "id"), req)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, row)
}

// Enroll creates recipient snapshots after validating the published version.
func (h *CRMOutreachHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	var req model.CRMSequenceEnrollRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid enrollment")
		return
	}
	ws, user, id := getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "id")
	if r.URL.Query().Get("preview") == "true" {
		rows, err := h.service.Preview(r.Context(), ws, user, id, req)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, rows)
		return
	}
	rows, err := h.service.Enroll(r.Context(), ws, user, id, req)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

// Enrollments lists paginated recipient activity without full email bodies.
func (h *CRMOutreachHandler) Enrollments(w http.ResponseWriter, r *http.Request) {
	ws := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	if r.Method == http.MethodPost {
		var req struct {
			Action   string `json:"action"`
			Subject  string `json:"subject"`
			BodyHTML string `json:"body_html"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, 400, "invalid action")
			return
		}
		if err := h.service.Control(r.Context(), ws, middleware.GetUserID(r.Context()), id, req.Action, req.Subject, req.BodyHTML); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"updated": true})
		return
	}
	if id != "" {
		row, deliveries, err := h.service.Detail(r.Context(), ws, id)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"enrollment": row, "deliveries": deliveries})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	rows, err := h.service.Enrollments(r.Context(), ws, r.URL.Query().Get("sequence_id"), r.URL.Query().Get("contact_id"), r.URL.Query().Get("deal_id"), model.CRMSequenceEnrollmentFilter{Page: page, Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status")})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

// Unsubscribe confirms or applies a recipient’s workspace opt-out.
func (h *CRMOutreachHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 72 {
		http.Error(w, "Link unavailable", 404)
		return
	}
	if err := h.service.Unsubscribe(r.Context(), token, r.Method == http.MethodPost); err != nil {
		http.Error(w, "Link unavailable", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	content := `<h1>Stop follow-up emails?</h1><p>You will no longer receive automated email sequences from this workspace.</p><form method="post"><button type="submit">Unsubscribe</button></form>`
	if r.Method == http.MethodPost {
		content = `<h1>You’re unsubscribed</h1><p>You won’t receive further automated email sequences from this workspace.</p>`
	}
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Email preferences</title><body style="font:16px system-ui;max-width:480px;margin:12vh auto;padding:24px;line-height:1.6">%s</body></html>`, content)
}

// RenderTemplate handles the corresponding CRM email workflow operation.
func (h *CRMOutreachHandler) RenderTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string `json:"email"`
		AccountID string `json:"account_id"`
		DealID    string `json:"deal_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid template preview")
		return
	}
	row, err := h.service.RenderTemplate(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "id"), req.Email, req.AccountID, req.DealID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, row)
}
