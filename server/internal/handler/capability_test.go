package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type recordingMailer struct{ to []string }

func (m *recordingMailer) SendEmail(to, _, _, _ string) error {
	m.to = append(m.to, to)
	return nil
}

func setupCapabilityHandler(t *testing.T, emailConfigured bool) (*CapabilityHandler, *recordingMailer) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:capability_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL)`,
		`INSERT INTO users VALUES ('u1', 'owner@example.com')`,
		`CREATE TABLE instance_capability_checks (key TEXT PRIMARY KEY, ok BOOLEAN NOT NULL, error TEXT, config_fingerprint TEXT NOT NULL, checked_by TEXT, checked_at DATETIME NOT NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	mailer := &recordingMailer{}
	svc := service.NewCapabilityService(service.CapabilityConfig{AppEmailConfigured: emailConfigured, AppEmailFingerprint: "fp"},
		repository.NewCapabilityRepository(db), repository.NewSetupRepository(db), repository.NewAIConnectionRepository(db))
	var sender service.TestEmailSender
	if emailConfigured {
		sender = mailer
	}
	svc.SetTestEmail(sender, repository.NewUserRepository(db))
	return NewCapabilityHandler(svc), mailer
}

func sendTestEmailRequest(h *CapabilityHandler, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/ws/email/test", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "ws")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	if userID != "" {
		ctx = middleware.WithUserID(ctx, userID)
	}
	rec := httptest.NewRecorder()
	h.SendTestEmail(rec, req.WithContext(ctx))
	return rec
}

func TestSendTestEmailHandlerSendsToRequesterAndRateLimits(t *testing.T) {
	h, mailer := setupCapabilityHandler(t, true)
	rec := sendTestEmailRequest(h, "u1")
	var result model.TestEmailResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !result.OK || result.Recipient != "owner@example.com" || len(mailer.to) != 1 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = sendTestEmailRequest(h, "u1")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(mailer.to) != 1 {
		t.Fatal("rate-limited request sent mail")
	}
}

func TestSendTestEmailHandlerWithoutMailOrUser(t *testing.T) {
	h, _ := setupCapabilityHandler(t, false)
	if rec := sendTestEmailRequest(h, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	rec := sendTestEmailRequest(h, "u1")
	var result model.TestEmailResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || result.OK || result.Error == "" {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
