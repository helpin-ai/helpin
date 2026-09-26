package handler

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMMeetingHandler handles meeting intelligence and capture webhooks.
type CRMMeetingHandler struct {
	meetingService *service.CRMMeetingService
}

// NewCRMMeetingHandler creates a meeting handler.
func NewCRMMeetingHandler(meetingService *service.CRMMeetingService) *CRMMeetingHandler {
	return &CRMMeetingHandler{meetingService: meetingService}
}

// List handles GET /api/crm/meetings.
func (h *CRMMeetingHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMMeetingListFilters{
		Status:          queryStringPtr(r, "status"),
		OwnerMemberID:   queryStringPtr(r, "owner_member_id"),
		CompanyID:       queryStringPtr(r, "company_id"),
		CompanyRollupID: queryStringPtr(r, "company_rollup_id"),
		ContactID:       queryStringPtr(r, "contact_id"),
		DealID:          queryStringPtr(r, "deal_id"),
		Search:          queryStringPtr(r, "search"),
	}
	var err error
	if filters.StartAfter, err = queryMeetingTime(r, "start_after"); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if filters.StartBefore, err = queryMeetingTime(r, "start_before"); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pagination := queryPagination(r)
	meetings, total, err := h.meetingService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to list meetings")
		return
	}
	if meetings == nil {
		meetings = []model.CRMMeeting{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": meetings, "total": total, "page": pagination.Page})
}

// Create handles POST /api/crm/meetings.
func (h *CRMMeetingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMMeetingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = getWorkspaceID(r)
	detail, err := h.meetingService.Create(r.Context(), req, middleware.GetUserID(r.Context()), r.Header.Get("Idempotency-Key"))
	if err != nil {
		if writeMeetingCaptureNotConfigured(w, err) {
			return
		}
		writeBillingAwareError(w, meetingErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// Get handles GET /api/crm/meetings/{id}.
func (h *CRMMeetingHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.meetingService.Get(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Update handles PUT /api/crm/meetings/{id}.
func (h *CRMMeetingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateCRMMeetingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	detail, err := h.meetingService.Update(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req)
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// Delete handles DELETE /api/crm/meetings/{id}.
func (h *CRMMeetingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.meetingService.Delete(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "meeting deleted"})
}

// StartCapture handles POST /api/crm/meetings/{id}/capture.
func (h *CRMMeetingHandler) StartCapture(w http.ResponseWriter, r *http.Request) {
	capture, err := h.meetingService.StartCapture(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		if writeMeetingCaptureNotConfigured(w, err) {
			return
		}
		writeBillingAwareError(w, meetingErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusAccepted, capture)
}

// StopCapture handles POST /api/crm/meetings/{id}/capture/stop.
func (h *CRMMeetingHandler) StopCapture(w http.ResponseWriter, r *http.Request) {
	capture, err := h.meetingService.StopCapture(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, capture)
}

// RetryProcessing handles POST /api/crm/meetings/{id}/process.
func (h *CRMMeetingHandler) RetryProcessing(w http.ResponseWriter, r *http.Request) {
	if err := h.meetingService.RetryProcessing(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id")); err != nil {
		writeBillingAwareError(w, meetingErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusAccepted, model.MessageResponse{Message: "meeting processing queued"})
}

// GetRecording handles GET /api/crm/meetings/{id}/recording.
func (h *CRMMeetingHandler) GetRecording(w http.ResponseWriter, r *http.Request) {
	recording, err := h.meetingService.GetRecording(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recording)
}

// DeleteRecording handles DELETE /api/crm/meetings/{id}/recording.
func (h *CRMMeetingHandler) DeleteRecording(w http.ResponseWriter, r *http.Request) {
	if err := h.meetingService.DeleteRecording(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id")); err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "meeting recording deleted"})
}

// AcceptActionItem handles POST /api/crm/meetings/{id}/action-items/{itemID}/accept.
func (h *CRMMeetingHandler) AcceptActionItem(w http.ResponseWriter, r *http.Request) {
	var req model.AcceptCRMMeetingActionItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.meetingService.AcceptActionItem(
		r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "itemID"),
		middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DismissActionItem handles POST /api/crm/meetings/{id}/action-items/{itemID}/dismiss.
func (h *CRMMeetingHandler) DismissActionItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.meetingService.DismissActionItem(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "itemID"))
	if err != nil {
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// GetSettings handles GET /api/crm/meeting-settings.
func (h *CRMMeetingHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.meetingService.GetSettings(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load meeting settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"settings": settings})
}

// UpdateSettings handles PUT /api/crm/meeting-settings.
func (h *CRMMeetingHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateCRMMeetingSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.meetingService.UpdateSettings(r.Context(), getWorkspaceID(r), req)
	if err != nil {
		if writeMeetingCaptureNotConfigured(w, err) {
			return
		}
		writeError(w, meetingErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// Webhook handles public authenticated provider webhooks.
func (h *CRMMeetingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook payload")
		return
	}
	if err := h.meetingService.HandleWebhook(r.Context(), chi.URLParam(r, "provider"), r.Header, payload); err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "signature") || strings.Contains(message, "verification") ||
			strings.Contains(message, "webhook authorization") || strings.Contains(message, "webhook timestamp") {
			writeError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}
		if strings.Contains(message, "decode") || strings.Contains(message, "missing a provider capture id") {
			writeError(w, http.StatusBadRequest, "invalid meeting webhook payload")
			return
		}
		if strings.Contains(message, "not found") {
			writeError(w, http.StatusNotFound, "meeting capture not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "unable to process meeting webhook")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func queryMeetingTime(r *http.Request, key string) (*time.Time, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, &meetingQueryError{field: key}
	}
	return &parsed, nil
}

type meetingQueryError struct {
	field string
}

func (e *meetingQueryError) Error() string {
	return e.field + " must be an RFC3339 timestamp"
}

// writeMeetingCaptureNotConfigured answers 409 with a user sentence when the
// server has no usable capture provider. It reports whether it wrote.
func writeMeetingCaptureNotConfigured(w http.ResponseWriter, err error) bool {
	if !service.IsMeetingCaptureNotConfigured(err) {
		return false
	}
	writeError(w, http.StatusConflict, service.MeetingCaptureNotConfiguredMessage)
	return true
}

func meetingErrorStatus(err error) int {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not found") {
		return http.StatusNotFound
	}
	if strings.Contains(message, "not configured") || strings.Contains(message, "unavailable") {
		return http.StatusServiceUnavailable
	}
	if strings.Contains(message, "already active") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}
