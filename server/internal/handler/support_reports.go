package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportReportsHandler serves support metrics and exports.
type SupportReportsHandler struct {
	reports *service.SupportReportService
}

func NewSupportReportsHandler(reports *service.SupportReportService) *SupportReportsHandler {
	return &SupportReportsHandler{reports: reports}
}

func supportReportRequestFromQuery(r *http.Request) (model.SupportReportRequest, error) {
	query := r.URL.Query()
	req := model.SupportReportRequest{
		WorkspaceID: middleware.GetWorkspaceID(r.Context()),
		Timezone:    strings.TrimSpace(query.Get("tz")),
		MailboxIDs:  splitSupportReportList(query.Get("mailbox_ids")),
		Channels:    splitSupportReportList(query.Get("channels")),
	}
	var err error
	if req.From, err = time.Parse(time.RFC3339, strings.TrimSpace(query.Get("from"))); err != nil {
		return req, fmt.Errorf("from must be an RFC 3339 timestamp")
	}
	if req.To, err = time.Parse(time.RFC3339, strings.TrimSpace(query.Get("to"))); err != nil {
		return req, fmt.Errorf("to must be an RFC 3339 timestamp")
	}
	return req, nil
}

func splitSupportReportList(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func writeSupportReportError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, service.ErrSupportReportInvalidRange) || errors.Is(err, service.ErrSupportReportInvalidSLAOutcome) || strings.HasPrefix(err.Error(), "unknown timezone") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	slog.ErrorContext(r.Context(), "support report failed", "error", err)
	writeError(w, http.StatusInternalServerError, "unable to build the support report")
}

// Overview handles GET /api/support/reports/overview.
func (h *SupportReportsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	req, err := supportReportRequestFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	overview, err := h.reports.Overview(r.Context(), req)
	if err != nil {
		writeSupportReportError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, overview)
}

// SLAConversations handles GET /api/support/reports/sla/conversations: the
// conversations behind one SLA count (metric, outcome, page).
func (h *SupportReportsHandler) SLAConversations(w http.ResponseWriter, r *http.Request) {
	req, err := supportReportRequestFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	query := r.URL.Query()
	page := 1
	if raw := strings.TrimSpace(query.Get("page")); raw != "" {
		if page, err = strconv.Atoi(raw); err != nil || page < 1 || page > 1000 {
			writeError(w, http.StatusBadRequest, "page must be a number from 1 to 1000")
			return
		}
	}
	result, err := h.reports.SLAConversations(r.Context(), req, strings.TrimSpace(query.Get("metric")), strings.TrimSpace(query.Get("outcome")), page)
	if err != nil {
		writeSupportReportError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

// ExportConversations handles GET /api/support/reports/conversations.csv.
func (h *SupportReportsHandler) ExportConversations(w http.ResponseWriter, r *http.Request) {
	req, err := supportReportRequestFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var body strings.Builder
	if err := h.reports.ExportConversationsCSV(r.Context(), req, &body); err != nil {
		writeSupportReportError(w, r, err)
		return
	}
	filename := fmt.Sprintf("support-conversations-%s-to-%s.csv", req.From.UTC().Format("2006-01-02"), req.To.UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body.String()))
}
