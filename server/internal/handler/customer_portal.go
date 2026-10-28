package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	portalCookie = "helpin_portal_session"
	// portalCookieRoot is the pre-release cookie path shared by every slug. It
	// is only cleared on sign-in and sign-out so old sessions cannot linger.
	portalCookieRoot = "/api/public/portal/"
)

// CustomerPortalHandler adapts the public slug-based portal contract to the
// customer portal service. The session cookie is never exposed to scripts.
type CustomerPortalHandler struct {
	portal    *service.CustomerPortalService
	assistant *service.HelpAssistantService
}

// NewCustomerPortalHandler creates the customer portal handler.
func NewCustomerPortalHandler(portal *service.CustomerPortalService) *CustomerPortalHandler {
	return &CustomerPortalHandler{portal: portal}
}

type portalAuthBody struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

type portalRequestBody struct {
	Subject       string   `json:"subject"`
	Message       string   `json:"message"`
	AttachmentIDs []string `json:"attachment_ids"`
	// Intake is what the customer saw while writing; optional.
	Intake *model.PortalIntake `json:"intake"`
}

// workspace resolves the enabled portal for the slug or writes a 404.
func (h *CustomerPortalHandler) workspace(w http.ResponseWriter, r *http.Request) *service.PortalWorkspace {
	ws, err := h.portal.ResolveWorkspace(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "portal unavailable")
		return nil
	}
	return ws
}

// authorized resolves the portal and its session cookie or writes an error.
func (h *CustomerPortalHandler) authorized(w http.ResponseWriter, r *http.Request) *service.PortalAccess {
	ws := h.workspace(w, r)
	if ws == nil {
		return nil
	}
	access, err := h.portal.Authenticate(r.Context(), ws, portalSessionCookie(r))
	if errors.Is(err, service.ErrPortalAuthInvalid) {
		slog.WarnContext(r.Context(), "portal access denied", "workspace_id", ws.ID, "action", r.Method, "reason", "invalid_session")
		writeError(w, http.StatusUnauthorized, "invalid portal session")
		return nil
	}
	if err != nil {
		// A lookup failure is not proof the customer lost access: keep the
		// session and let the client retry instead of signing them out.
		slog.ErrorContext(r.Context(), "portal session check failed", "workspace_id", ws.ID, "action", r.Method, "error", err)
		writePortalTemporarilyUnavailable(w)
		return nil
	}
	return access
}

// Config returns the public portal configuration.
func (h *CustomerPortalHandler) Config(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	configuration := h.portal.Configuration(r.Context(), ws)
	configuration.AssistantEnabled = h.assistant.Available(r.Context(), ws.ID)
	writeJSON(w, http.StatusOK, configuration)
}

// HelpcenterPortal handles GET /api/hc/{subdomain}/portal: the help center
// server asks which portal it serves at /requests on a help center host.
func (h *CustomerPortalHandler) HelpcenterPortal(w http.ResponseWriter, r *http.Request) {
	slug, err := h.portal.HelpcenterPortalSlug(r.Context(), chi.URLParam(r, "subdomain"))
	if errors.Is(err, service.ErrPortalUnavailable) {
		writeError(w, http.StatusNotFound, "portal not found")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "help center portal lookup failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "portal temporarily unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"slug": slug})
}

// RequestLink emails a sign-in link without disclosing whether the slug,
// address, or workspace exists.
func (h *CustomerPortalHandler) RequestLink(w http.ResponseWriter, r *http.Request) {
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if ws, err := h.portal.ResolveWorkspace(r.Context(), chi.URLParam(r, "slug")); err == nil {
		h.portal.RequestLink(r.Context(), ws, body.Email)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Exchange trades a sign-in link for a session cookie.
func (h *CustomerPortalHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token, identity, err := h.portal.Exchange(r.Context(), ws, body.Token)
	if errors.Is(err, service.ErrPortalAuthInvalid) {
		writeError(w, http.StatusUnauthorized, "invalid or expired link")
		return
	}
	if err != nil {
		// The exchange rolled back, so the link stays usable for a retry.
		slog.ErrorContext(r.Context(), "portal link exchange failed", "workspace_id", ws.ID, "error", err)
		writePortalTemporarilyUnavailable(w)
		return
	}
	clearPortalCookie(w, r, portalCookieRoot)
	setPortalCookie(w, r, token, int(service.PortalSessionTTL/time.Second))
	response := portalCustomer(identity)
	// Answer like Session, so company views appear without a second request.
	if access, err := h.portal.Authenticate(r.Context(), ws, token); err == nil {
		response = h.portalSession(r, access)
	} else {
		slog.WarnContext(r.Context(), "portal session lookup after exchange failed", "workspace_id", ws.ID, "error", err)
		response["companies"] = []repository.PortalCompanyMembership{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, response)
}

// Session returns the signed-in customer and the companies whose portal they
// belong to. A failed company lookup degrades to a personal portal.
func (h *CustomerPortalHandler) Session(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, h.portalSession(r, access))
}

// portalSession is the signed-in customer with their portal companies.
func (h *CustomerPortalHandler) portalSession(r *http.Request, access *service.PortalAccess) map[string]any {
	response := portalCustomer(&access.Identity)
	companies, err := h.portal.Companies(r.Context(), access)
	if err != nil {
		slog.ErrorContext(r.Context(), "portal company lookup failed", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "error", err)
		companies = []repository.PortalCompanyMembership{}
	}
	response["companies"] = companies
	return response
}

// Logout revokes the session and clears its cookie.
func (h *CustomerPortalHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if ws, err := h.portal.ResolveWorkspace(r.Context(), chi.URLParam(r, "slug")); err == nil {
		h.portal.Logout(r.Context(), ws, portalSessionCookie(r))
	}
	clearPortalCookie(w, r, portalCookiePath(r))
	clearPortalCookie(w, r, portalCookieRoot)
	w.WriteHeader(http.StatusNoContent)
}

// StartAnonymousIntake issues an intake token for an unverified address.
func (h *CustomerPortalHandler) StartAnonymousIntake(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body portalAuthBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	token, err := h.portal.StartAnonymousIntake(r.Context(), ws, body.Email)
	switch {
	case errors.Is(err, service.ErrPortalAnonymousIntakeDisabled):
		writeError(w, http.StatusForbidden, "anonymous intake unavailable")
	case errors.Is(err, service.ErrPortalAuthInvalid):
		writeError(w, http.StatusBadRequest, "enter a valid email address")
	case err != nil:
		slog.ErrorContext(r.Context(), "portal intake start failed", "workspace_id", ws.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "unable to start request")
	default:
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, map[string]string{"intake_token": token})
	}
}

// UploadAnonymousAttachment starts an upload scoped to an intake token.
func (h *CustomerPortalHandler) UploadAnonymousAttachment(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body model.CreateSupportAttachmentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.portal.UploadAnonymousAttachment(r.Context(), ws, portalBearer(r), body)
	if err != nil {
		writePortalError(w, r, ws.ID, err, "attachment unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}

// ConfirmAnonymousAttachment confirms an intake upload.
func (h *CustomerPortalHandler) ConfirmAnonymousAttachment(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	if err := h.portal.ConfirmAnonymousAttachment(r.Context(), ws, portalBearer(r), chi.URLParam(r, "attachmentId")); err != nil {
		writePortalError(w, r, ws.ID, err, "attachment unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateAnonymousRequest creates a request from an intake token. The request
// is published to the portal once the address owner confirms it by email.
func (h *CustomerPortalHandler) CreateAnonymousRequest(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body portalRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.portal.CreateAnonymousRequest(r.Context(), ws, portalBearer(r), body.Subject, body.Message, body.AttachmentIDs, body.Intake); err != nil {
		writePortalError(w, r, ws.ID, err, "unable to create request")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusAccepted)
}

// Suggestions returns help center articles for a request being written. The
// forms for signed-in and anonymous customers both use it; only a valid
// session adds articles for signed-in customers. The portal rate limiter
// bounds it.
func (h *CustomerPortalHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	suggestions, err := h.portal.SuggestArticles(r.Context(), ws, r.URL.Query().Get("q"), h.signedIn(r, ws))
	if errors.Is(err, context.Canceled) {
		// The portal cancels a search when the customer keeps typing.
		return
	}
	if err != nil {
		writePortalError(w, r, ws.ID, err, "unable to suggest articles")
		return
	}
	// Signed-in customers also get articles for signed-in customers.
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, suggestions)
}

// SuggestionEvent records that suggestions were shown, or one was opened,
// while a request was written. Like suggestions it needs no sign-in.
func (h *CustomerPortalHandler) SuggestionEvent(w http.ResponseWriter, r *http.Request) {
	ws := h.workspace(w, r)
	if ws == nil {
		return
	}
	var body struct {
		IntakeID   string   `json:"intake_id"`
		Event      string   `json:"event"`
		ArticleIDs []string `json:"article_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.portal.RecordIntakeEvent(r.Context(), ws, body.IntakeID, body.Event, body.ArticleIDs); err != nil {
		writePortalError(w, r, ws.ID, err, "unable to record the event")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateRequest creates a request for the signed-in customer.
func (h *CustomerPortalHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	var body portalRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	request, err := h.portal.CreateRequest(r.Context(), access, body.Subject, body.Message, body.AttachmentIDs, body.Intake)
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to create request")
		return
	}
	slog.InfoContext(r.Context(), "portal request created", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "reference", request.Reference, "attachment_count", len(body.AttachmentIDs))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, request)
}

// Requests lists the signed-in customer's requests.
func (h *CustomerPortalHandler) Requests(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "active" && status != model.SupportConversationStatusWaitingOnCustomer && status != model.SupportConversationStatusResolved {
		writeError(w, http.StatusBadRequest, "invalid request status")
		return
	}
	query := r.URL.Query()
	var requests []model.SupportPortalRequest
	var err error
	switch scope := query.Get("scope"); scope {
	case "", service.PortalScopeMine:
		requests, err = h.portal.Requests(r.Context(), access, status)
	case service.PortalScopeCompany:
		requests, err = h.portal.CompanyRequests(r.Context(), access, strings.TrimSpace(query.Get("company_id")), status)
	default:
		writeError(w, http.StatusBadRequest, service.ErrPortalScopeInvalid.Error())
		return
	}
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to load requests")
		return
	}
	response := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		response = append(response, portalRequestListRow(request, access.Identity.ID))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, response)
}

// portalRequestListRow is the customer-safe list projection of a request:
// only the fields named here reach the portal.
func portalRequestListRow(request model.SupportPortalRequest, viewerIdentityID string) map[string]any {
	row := map[string]any{
		"reference": request.Reference, "number": request.Number, "subject": request.Subject, "status": request.Status,
		"last_activity_at": request.LastActivityAt, "last_message_preview": request.LastMessagePreview,
		"last_message_from": request.LastMessageFrom, "unread": request.Unread,
	}
	if request.ReplyExpectedBy != nil {
		row["reply_expected_by"] = request.ReplyExpectedBy
	}
	if request.CompanyID != nil {
		row["own"] = request.OwnerIdentityID == viewerIdentityID
		row["requested_by"] = portalRequestedBy(request)
		row["company"] = map[string]any{"id": *request.CompanyID, "name": request.CompanyName}
	}
	return row
}

// RequestDetail returns one of the signed-in customer's requests.
func (h *CustomerPortalHandler) RequestDetail(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	detail, err := h.portal.RequestDetail(r.Context(), access, chi.URLParam(r, "reference"))
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to load request")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, detail)
}

// Reply adds a customer reply and returns the refreshed request.
func (h *CustomerPortalHandler) Reply(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	var body struct {
		Content       string   `json:"content"`
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "enter a reply")
		return
	}
	reference := chi.URLParam(r, "reference")
	detail, err := h.portal.Reply(r.Context(), access, reference, body.Content, body.AttachmentIDs)
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to send reply")
		return
	}
	slog.InfoContext(r.Context(), "portal reply created", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "reference", reference, "attachment_count", len(body.AttachmentIDs))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, detail)
}

// Rate records the customer's satisfaction rating of a resolved request.
func (h *CustomerPortalHandler) Rate(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	var body struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, service.ErrSupportCSATInvalid.Error())
		return
	}
	detail, err := h.portal.RateRequest(r.Context(), access, chi.URLParam(r, "reference"), body.Rating, body.Comment)
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "unable to save your rating")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, detail)
}

// UploadAttachment starts an upload for a new request or a reply.
func (h *CustomerPortalHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	var body model.CreateSupportAttachmentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.portal.UploadAttachment(r.Context(), access, chi.URLParam(r, "reference"), body)
	if err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "attachment unavailable")
		return
	}
	slog.InfoContext(r.Context(), "portal attachment uploaded", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "reference", chi.URLParam(r, "reference"), "attachment_id", result.Attachment.ID)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}

// ConfirmAttachment confirms an upload made by the same session.
func (h *CustomerPortalHandler) ConfirmAttachment(w http.ResponseWriter, r *http.Request) {
	access := h.authorized(w, r)
	if access == nil {
		return
	}
	if err := h.portal.ConfirmAttachment(r.Context(), access, chi.URLParam(r, "reference"), chi.URLParam(r, "attachmentId")); err != nil {
		writePortalError(w, r, access.Workspace.ID, err, "attachment unavailable")
		return
	}
	slog.InfoContext(r.Context(), "portal attachment confirmed", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "reference", chi.URLParam(r, "reference"), "attachment_id", chi.URLParam(r, "attachmentId"))
	w.WriteHeader(http.StatusNoContent)
}

// writePortalError maps service errors to public responses. Unknown errors
// are logged and reported with fallback, never with internal detail.
func writePortalError(w http.ResponseWriter, r *http.Request, workspaceID string, err error, fallback string) {
	if message, rejected := service.IsSupportAttachmentRejected(err); rejected {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	switch {
	case errors.Is(err, service.ErrPortalAuthInvalid):
		writeError(w, http.StatusUnauthorized, "invalid intake session")
	case errors.Is(err, service.ErrPortalRequestNotFound):
		writeError(w, http.StatusNotFound, "request not found")
	case errors.Is(err, service.ErrPortalReplyUnavailable):
		writeError(w, http.StatusConflict, service.ErrPortalReplyUnavailable.Error())
	case errors.Is(err, service.ErrSupportCSATUnavailable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrSupportCSATInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrPortalIntakeDisabled), errors.Is(err, service.ErrPortalAnonymousIntakeDisabled), errors.Is(err, service.ErrPortalAttachmentsUnavailable):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrPortalRequestInvalid), errors.Is(err, service.ErrPortalReplyInvalid), errors.Is(err, service.ErrPortalIntakeInvalid), errors.Is(err, service.ErrPortalAttachmentsInvalid), errors.Is(err, service.ErrPortalTooManyAttachments):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.ErrorContext(r.Context(), "portal request failed", "workspace_id", workspaceID, "action", r.Method, "error", err)
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

func writePortalTemporarilyUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	writeError(w, http.StatusServiceUnavailable, "the support portal is temporarily unavailable; please try again")
}

// portalRequestedBy names who asked for a company request.
func portalRequestedBy(request model.SupportPortalRequest) string {
	for _, value := range []*string{request.RequesterName, request.RequesterEmail} {
		if name := strings.TrimSpace(derefPortalString(value)); name != "" {
			return name
		}
	}
	return ""
}

func derefPortalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func portalCustomer(identity *model.SupportPortalIdentity) map[string]any {
	return map[string]any{"customer": map[string]any{"id": identity.ID, "email": identity.Email, "name": identity.DisplayName}}
}

func portalSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(portalCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// portalCookiePath scopes the session to one portal, so signing in to another
// workspace's portal neither replaces nor receives this session. The /api
// prefix matches the staff session cookie policy.
func portalCookiePath(r *http.Request) string {
	return portalCookieRoot + url.PathEscape(chi.URLParam(r, "slug"))
}

func setPortalCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: portalCookie, Value: token, Path: portalCookiePath(r), HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func clearPortalCookie(w http.ResponseWriter, r *http.Request, path string) {
	http.SetCookie(w, &http.Cookie{Name: portalCookie, Value: "", Path: path, HttpOnly: true, Secure: secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func portalBearer(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
