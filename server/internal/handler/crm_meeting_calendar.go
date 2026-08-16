package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListUpcomingCalendar handles GET /api/crm/meetings/calendar-upcoming.
func (h *CRMMeetingHandler) ListUpcomingCalendar(w http.ResponseWriter, r *http.Request) {
	candidates, err := h.meetingService.ListUpcomingCalendarMeetings(r.Context(), getWorkspaceID(r), time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unable to load upcoming calendar meetings")
		return
	}
	if candidates == nil {
		candidates = []model.CRMCalendarMeetingCandidate{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": candidates, "total": len(candidates)})
}

// UpdateCalendarCapture handles PUT /api/crm/meetings/calendar/{eventID}/capture.
func (h *CRMMeetingHandler) UpdateCalendarCapture(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateCRMCalendarMeetingCaptureRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	candidate, err := h.meetingService.UpdateCalendarMeetingCapture(
		r.Context(),
		getWorkspaceID(r),
		chi.URLParam(r, "eventID"),
		middleware.GetUserID(r.Context()),
		req,
	)
	if err != nil {
		writeBillingAwareError(w, meetingErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}
