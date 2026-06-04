package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

func TestAdminAuditLogger_LogsDeniedRequest(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	defer slog.SetDefault(previous)

	jwtManager := auth.NewJWTManager("test-secret")
	accessToken, _, err := jwtManager.GenerateTokenPair("user-1", "admin@example.com", false)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/email-queue", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	AdminAuditLogger(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})).ServeHTTP(rec, req)

	logged := out.String()
	for _, want := range []string{`"msg":"admin_request"`, `"user_id":"user-1"`, `"path":"/api/admin/email-queue"`, `"status":403`} {
		if !strings.Contains(logged, want) {
			t.Fatalf("audit log missing %s in %s", want, logged)
		}
	}
}
