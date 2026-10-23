package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type stubSupportInboxClearRecorder struct {
	stats *model.SupportInboxClearStats
	err   error
	got   model.RecordSupportInboxClearRequest
}

func (s *stubSupportInboxClearRecorder) Record(_ context.Context, _, _ string, req model.RecordSupportInboxClearRequest) (*model.SupportInboxClearStats, error) {
	s.got = req
	return s.stats, s.err
}

func TestSupportInboxClearHandler_RecordClear(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "returns stats", body: `{"conversation_id":"c","scope":"shared","local_date":"2026-10-23","day_start":"2026-10-23T00:00:00Z"}`, wantStatus: http.StatusOK, wantBody: `"streak_days":3`},
		{name: "malformed body", body: `{`, wantStatus: http.StatusBadRequest, wantBody: "invalid request body"},
		{name: "validation error names the field", body: `{}`, err: fmt.Errorf("%w: scope must be mailbox, all or mine", service.ErrSupportInboxClearInvalid), wantStatus: http.StatusBadRequest, wantBody: "scope must be mailbox, all or mine"},
		{name: "unknown conversation", body: `{}`, err: service.ErrSupportInboxClearNotFound, wantStatus: http.StatusNotFound, wantBody: "conversation not found"},
		{name: "conversation still open", body: `{}`, err: service.ErrSupportInboxClearStillWaiting, wantStatus: http.StatusConflict, wantBody: "conversation is not resolved"},
		{name: "storage failure stays generic", body: `{}`, err: errors.New("pq: relation does not exist"), wantStatus: http.StatusInternalServerError, wantBody: "could not record the inbox clear"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &stubSupportInboxClearRecorder{stats: &model.SupportInboxClearStats{StreakDays: 3, ClearsToday: 1, ResolvedToday: 9, ClearedCount: 4}, err: tc.err}
			h := NewSupportInboxClearHandler(recorder)
			req := httptest.NewRequest(http.MethodPost, "/api/support/inbox-clears?workspace_id=ws-1", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.RecordClear(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %s, want it to contain %q", rec.Body.String(), tc.wantBody)
			}
			if strings.Contains(rec.Body.String(), "pq:") {
				t.Fatalf("body leaked a storage error: %s", rec.Body.String())
			}
		})
	}
}

func TestSupportInboxClearHandler_DecodesTheContract(t *testing.T) {
	recorder := &stubSupportInboxClearRecorder{stats: &model.SupportInboxClearStats{ClearedCount: 1}}
	h := NewSupportInboxClearHandler(recorder)
	body := `{"conversation_id":"c-1","scope":"mailbox","mailbox_id":"mb-1","local_date":"2026-10-23","day_start":"2026-10-22T19:00:00Z"}`
	rec := httptest.NewRecorder()
	h.RecordClear(rec, httptest.NewRequest(http.MethodPost, "/api/support/inbox-clears", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := recorder.got
	if got.ConversationID != "c-1" || got.Scope != "mailbox" || got.MailboxID == nil || *got.MailboxID != "mb-1" || got.LocalDate != "2026-10-23" || got.DayStart != "2026-10-22T19:00:00Z" {
		t.Fatalf("decoded request = %+v", got)
	}
	var stats map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"streak_days", "clears_today", "resolved_today", "cleared_count"} {
		if _, ok := stats[key]; !ok {
			t.Fatalf("response %v is missing %s", stats, key)
		}
	}
}
