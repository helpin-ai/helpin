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

// CLIHandler exposes the public OAuth issuer and user-scoped local admission API.
type CLIHandler struct{ service *service.CLIService }

// NewCLIHandler constructs the HTTP adapter.
func NewCLIHandler(s *service.CLIService) *CLIHandler { return &CLIHandler{service: s} }

// Discovery serves only enabled host capabilities.
func (h *CLIHandler) Discovery(w http.ResponseWriter, r *http.Request) {
	if h.enabled(w, r) {
		writeJSON(w, 200, h.service.Discovery())
	}
}

// Metadata serves the dedicated CLI issuer's authorization metadata.
func (h *CLIHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	if h.enabled(w, r) {
		writeJSON(w, 200, h.service.OAuthMetadata())
	}
}

// AuthorizeRedirect sends a validated request to the authenticated consent page.
func (h *CLIHandler) AuthorizeRedirect(w http.ResponseWriter, r *http.Request) {
	u, err := h.service.AuthorizationURL(cliQuery(r))
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, u, http.StatusFound)
}

// ConsentRequest returns the signed-in user's eligible workspaces.
func (h *CLIHandler) ConsentRequest(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ConsentRequest(r.Context(), middleware.GetUserID(r.Context()), cliQuery(r), cliBrowserMFA(r))
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

// Authorize records explicit browser consent and returns the exact callback URL.
func (h *CLIHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	var req model.CLIConsentDecision
	if err := decodeCLIJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	u, err := h.service.Authorize(r.Context(), middleware.GetUserID(r.Context()), req, cliBrowserMFA(r))
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]string{"redirect_url": u})
}

// Token consumes a PKCE authorization code or rotates a refresh token.
func (h *CLIHandler) Token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	f := r.PostForm
	raw := f.Get("code")
	if f.Get("grant_type") == "refresh_token" {
		raw = f.Get("refresh_token")
	}
	result, err := h.service.Exchange(r.Context(), f.Get("grant_type"), raw, f.Get("client_id"), f.Get("resource"), f.Get("redirect_uri"), f.Get("code_verifier"))
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, 200, result)
}

// RevokeToken revokes a connection's entire credential family.
func (h *CLIHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	if err := h.service.RevokeToken(r.Context(), r.PostForm.Get("token"), r.PostForm.Get("client_id")); err != nil {
		writeCLIError(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// Me returns the identity bound to the CLI consent, independent of browser JWTs.
func (h *CLIHandler) Me(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c != nil {
		writeJSON(w, 200, map[string]string{"user_id": c.UserID, "workspace_id": c.WorkspaceID, "connection_id": c.ID, "scope": c.Scope})
	}
}

// Agents lists locally compatible, actor-visible agents.
func (h *CLIHandler) Agents(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	agents, err := h.service.Agents(r.Context(), c)
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"agents": agents})
}

// Admit prepares a local run through the shared billing and authorization path.
func (h *CLIHandler) Admit(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	var req model.CLIAdmissionRequest
	if err := decodeCLIJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	result, err := h.service.Admit(r.Context(), c, req)
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 201, result)
}

// Execution reads or modifies a run-scoped lease under the current OAuth identity.
func (h *CLIHandler) Execution(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	runID := chi.URLParam(r, "run_id")
	action := chi.URLParam(r, "action")
	if r.Method == http.MethodGet {
		result, err := h.service.Execution(r.Context(), c, runID)
		if err != nil {
			writeCLIError(w, r, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if action == "revoke" {
		if err := h.service.RevokeExecution(r.Context(), c, runID); err != nil {
			writeCLIError(w, r, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	var req struct {
		Epoch      int64  `json:"epoch"`
		LocalRunID string `json:"local_run_id,omitempty"`
	}
	if err := decodeCLIJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	var result *model.CLIExecution
	var err error
	switch action {
	case "bind":
		result, err = h.service.BindExecution(r.Context(), c, runID, req.Epoch, req.LocalRunID)
	case "renew":
		result, err = h.service.RenewExecution(r.Context(), c, runID, req.Epoch)
	default:
		err = service.ErrCLIInvalid
	}
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}
func (h *CLIHandler) enabled(w http.ResponseWriter, r *http.Request) bool {
	if !h.service.Enabled() {
		writeCLIError(w, r, service.ErrCLIDisabled)
		return false
	}
	return true
}
func (h *CLIHandler) principal(w http.ResponseWriter, r *http.Request) *model.CLIConnection {
	if !h.enabled(w, r) {
		return nil
	}
	scheme, raw, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		writeCLIError(w, r, service.ErrCLIUnauthorized)
		return nil
	}
	c, err := h.service.Authenticate(r.Context(), raw)
	if err != nil {
		writeCLIError(w, r, err)
		return nil
	}
	w.Header().Set("Cache-Control", "no-store")
	return c
}
func cliQuery(r *http.Request) model.CLIAuthorizationQuery {
	q := r.URL.Query()
	return model.CLIAuthorizationQuery{ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"), ResponseType: q.Get("response_type"), Scope: q.Get("scope"), State: q.Get("state"), CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"), Resource: q.Get("resource")}
}
func decodeCLIJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 96<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return service.ErrCLIInvalid
	}
	return nil
}
func writeCLIError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := 500, "CLI request could not be completed"
	var entitlement *service.EntitlementError
	switch {
	case errors.As(err, &entitlement) || errors.Is(err, model.ErrAIUsageExhausted) || errors.Is(err, model.ErrBillingWorkspaceLocked) || errors.Is(err, model.ErrExtraAIUsageUnavailable) || errors.Is(err, model.ErrExtraAIUsageDisabled):
		status, message = 402, "AI usage is unavailable; check your workspace billing"
	case errors.Is(err, service.ErrCLIDisabled):
		status, message = 404, "Local CLI access is disabled"
	case errors.Is(err, service.ErrCLIUnauthorized):
		status, message = 401, "CLI authorization is invalid or expired"
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	case errors.Is(err, service.ErrCLIForbidden):
		status, message = 403, "CLI access is not permitted"
	case errors.Is(err, service.ErrCLIInvalid):
		status, message = 400, "Invalid or unsupported CLI request"
	case errors.Is(err, service.ErrCLIConflict):
		status, message = 409, "CLI execution conflict; refresh its state before retrying"
	default:
		slog.ErrorContext(r.Context(), "CLI request failed", "error", err)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeError(w, status, message)
}

func cliBrowserMFA(r *http.Request) bool {
	claims := middleware.ClaimsFrom(r.Context())
	return claims != nil && claims.MFASatisfied
}

// Model executes one idempotent, fenced model request under the current user.
func (h *CLIHandler) Model(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	var req model.CLIModelRequest
	if err := decodeCLIExecutionJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	result, err := h.service.Generate(r.Context(), c, chi.URLParam(r, "run_id"), req)
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

// Report ingests bounded local reports without trusting their usage claims.
func (h *CLIHandler) Report(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	var req model.CLIResultRequest
	if err := decodeCLIExecutionJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	if err := h.service.Report(r.Context(), c, chi.URLParam(r, "run_id"), req); err != nil {
		writeCLIError(w, r, err)
		return
	}
	w.WriteHeader(204)
}

// Artifact persists a private local evidence attachment.
func (h *CLIHandler) Artifact(w http.ResponseWriter, r *http.Request) {
	c := h.principal(w, r)
	if c == nil {
		return
	}
	var req model.CLIArtifactRequest
	if err := decodeCLIExecutionJSON(w, r, &req); err != nil {
		writeCLIError(w, r, service.ErrCLIInvalid)
		return
	}
	id, err := h.service.Artifact(r.Context(), c, chi.URLParam(r, "run_id"), req)
	if err != nil {
		writeCLIError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"artifact_id": id, "provenance": "local_report"})
}
func decodeCLIExecutionJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return service.ErrCLIInvalid
	}
	return nil
}
