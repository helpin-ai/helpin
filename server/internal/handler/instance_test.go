package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type instanceHandlerFixture struct {
	db       *gorm.DB
	auth     *AuthHandler
	instance *InstanceHandler
	router   http.Handler
	mail     *bool
}

func newInstanceHandlerFixture(t *testing.T) *instanceHandlerFixture {
	t.Helper()
	dbName := fmt.Sprintf("file:instance-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, full_name TEXT NOT NULL,
			email_verified_at DATETIME, google_subject TEXT UNIQUE, avatar_url TEXT, avatar_style TEXT,
			avatar_seed TEXT, avatar_background_mode TEXT, avatar_background_color TEXT, default_workspace_id TEXT,
			totp_secret_encrypted TEXT, totp_verified BOOLEAN NOT NULL DEFAULT 0, recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0, is_server_admin BOOLEAN NOT NULL DEFAULT 0,
			signup_verification_pending BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE instance_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			singleton BOOLEAN NOT NULL DEFAULT 1 UNIQUE, signup_mode TEXT NOT NULL DEFAULT 'invite_only',
			signup_allowed_domains TEXT NOT NULL DEFAULT '', admin_bootstrapped_at DATETIME,
			smtp_host TEXT, smtp_port INTEGER, smtp_username TEXT, smtp_password_encrypted TEXT,
			smtp_from TEXT, smtp_tls_mode TEXT, smtp_updated_at DATETIME, updated_by TEXT,
			created_at DATETIME, updated_at DATETIME
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	mailReady := false
	f := &instanceHandlerFixture{db: db, mail: &mailReady}
	settings := repository.NewInstanceSettingsRepository(db)
	emailConfig := service.NewAppEmailConfigService(settings, service.AppEmailConfigOptions{EncryptionKey: []byte(testAuthEncryptionKey)})
	instance := service.NewInstanceService(settings, repository.NewUserRepository(db), service.InstanceServiceOptions{
		MailReady: func() bool { return *f.mail },
	})
	authService := service.NewAuthService(repository.NewUserRepository(db), nil, nil, nil, nil,
		auth.NewJWTManager("test-secret"), nil, nil, "http://localhost:5173", []byte(testAuthEncryptionKey))
	authService.SetSignupGate(instance)
	f.auth = NewAuthHandler(authService)
	f.auth.SetSignupPolicy(instance)
	f.instance = NewInstanceHandler(instance, emailConfig)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(f.instance.RequireServerAdmin)
		r.Get("/instance/signup-policy", f.instance.GetSignupPolicy)
		r.Put("/instance/signup-policy", f.instance.UpdateSignupPolicy)
		r.Get("/instance/admins", f.instance.ListAdmins)
		r.Delete("/instance/admins/{userID}", f.instance.RevokeAdmin)
		r.Get("/instance/email", f.instance.GetEmailSettings)
		r.Put("/instance/email", f.instance.UpdateEmailSettings)
	})
	f.router = r
	return f
}

func (f *instanceHandlerFixture) signup(t *testing.T, email string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(model.SignupRequest{Email: email, Password: "strongpass1", FullName: "Person"})
	rec := httptest.NewRecorder()
	f.auth.Signup(rec, httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body)))
	return rec
}

func (f *instanceHandlerFixture) config(t *testing.T) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	f.auth.GetConfig(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	var cfg map[string]any
	decodeJSONResponse(t, rec, &cfg)
	return cfg
}

func (f *instanceHandlerFixture) call(t *testing.T, userID, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req = req.WithContext(middleware.WithUserID(context.Background(), userID))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func TestAuthConfigAndSignupFollowSignupPolicy(t *testing.T) {
	f := newInstanceHandlerFixture(t)

	cfg := f.config(t)
	if cfg["signup_mode"] != "invite_only" || cfg["signup_first_user"] != true {
		t.Fatalf("fresh config = %v, want invite_only awaiting the first account", cfg)
	}

	first := f.signup(t, "founder@example.com")
	if first.Code != http.StatusCreated {
		t.Fatalf("first signup status = %d body=%s", first.Code, first.Body.String())
	}
	var created model.AuthResponse
	decodeJSONResponse(t, first, &created)
	if !created.User.IsServerAdmin {
		t.Fatal("first account should be reported as server admin")
	}
	if cfg := f.config(t); cfg["signup_first_user"] != false {
		t.Fatalf("config after first signup = %v", cfg)
	}

	second := f.signup(t, "second@example.com")
	if second.Code != http.StatusForbidden {
		t.Fatalf("invite-only signup status = %d", second.Code)
	}
	var apiErr model.APIError
	decodeJSONResponse(t, second, &apiErr)
	if apiErr.Code != "signup_restricted" || apiErr.Error != "Signup on this server is by invitation. Ask your admin for an invite." {
		t.Fatalf("invite-only error = %+v", apiErr)
	}

	*f.mail = true
	f.db.Exec(`UPDATE instance_settings SET signup_mode = 'domains', signup_allowed_domains = 'acme.com'`)
	if cfg := f.config(t); cfg["signup_mode"] != "domains" || fmt.Sprint(cfg["signup_allowed_domains"]) != "[acme.com]" {
		t.Fatalf("domains config = %v", cfg)
	}
	pending := f.signup(t, "dev@acme.com")
	if pending.Code != http.StatusAccepted || !strings.Contains(pending.Body.String(), `"verification_required":true`) {
		t.Fatalf("domain signup = %d %s", pending.Code, pending.Body.String())
	}
	if strings.Contains(pending.Body.String(), "access_token") {
		t.Fatal("pending signup must not return tokens")
	}
}

func TestAuthConfigWithoutSignupPolicyIsOpen(t *testing.T) {
	h, _ := newAuthHandlerTestFixture(t)
	rec := httptest.NewRecorder()
	h.GetConfig(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	var cfg map[string]any
	decodeJSONResponse(t, rec, &cfg)
	if cfg["signup_mode"] != "open" || cfg["signup_first_user"] != false {
		t.Fatalf("config without policy = %v, want open signup (Enterprise)", cfg)
	}
}

func TestInstanceRoutesRequireServerAdmin(t *testing.T) {
	f := newInstanceHandlerFixture(t)
	var admin, member model.AuthResponse
	decodeJSONResponse(t, f.signup(t, "founder@example.com"), &admin)
	f.db.Exec(`UPDATE instance_settings SET signup_mode = 'open'`)
	decodeJSONResponse(t, f.signup(t, "member@example.com"), &member)

	for _, path := range []string{"/instance/signup-policy", "/instance/admins", "/instance/email"} {
		if rec := f.call(t, member.User.ID, http.MethodGet, path, nil); rec.Code != http.StatusForbidden {
			t.Errorf("member GET %s = %d, want 403", path, rec.Code)
		}
		if rec := f.call(t, admin.User.ID, http.MethodGet, path, nil); rec.Code != http.StatusOK {
			t.Errorf("admin GET %s = %d, want 200", path, rec.Code)
		}
	}
	if rec := f.call(t, admin.User.ID, http.MethodDelete, "/instance/admins/"+admin.User.ID, nil); rec.Code != http.StatusConflict {
		t.Fatalf("removing the last admin = %d, want 409", rec.Code)
	}
	rec := f.call(t, admin.User.ID, http.MethodPut, "/instance/signup-policy", model.UpdateSignupPolicyRequest{Mode: "domains", AllowedDomains: []string{"acme.com"}})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "application email") {
		t.Fatalf("domains without email = %d %s", rec.Code, rec.Body.String())
	}
}

func TestInstanceEmailSettingsNeverReturnPassword(t *testing.T) {
	f := newInstanceHandlerFixture(t)
	var admin model.AuthResponse
	decodeJSONResponse(t, f.signup(t, "founder@example.com"), &admin)
	password := "very-secret-smtp-password"

	rec := f.call(t, admin.User.ID, http.MethodPut, "/instance/email", map[string]any{
		"host": "smtp.example.com", "port": 587, "username": "mailer", "password": password,
		"from": "helpin@example.com", "tls_mode": "starttls",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body.String())
	}
	for _, body := range []string{rec.Body.String(), f.call(t, admin.User.ID, http.MethodGet, "/instance/email", nil).Body.String()} {
		if strings.Contains(body, password) || !strings.Contains(body, `"password_set":true`) {
			t.Fatalf("email settings response leaked or lost the password state: %s", body)
		}
	}
	bad := f.call(t, admin.User.ID, http.MethodPut, "/instance/email", map[string]any{"host": "smtp.example.com", "from": "nope"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid settings = %d", bad.Code)
	}
}
