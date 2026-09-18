package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDemoReadOnly(t *testing.T) {
	isDemo := func(email string) bool { return email == "demo@example.com" }
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := DemoReadOnly(isDemo)(next)

	cases := []struct {
		name   string
		method string
		email  string
		want   int
	}{
		{"demo user read", http.MethodGet, "demo@example.com", http.StatusNoContent},
		{"demo user write", http.MethodPost, "demo@example.com", http.StatusForbidden},
		{"demo user delete", http.MethodDelete, "demo@example.com", http.StatusForbidden},
		{"regular user write", http.MethodPost, "alice@example.com", http.StatusNoContent},
		{"anonymous write", http.MethodPut, "", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/anything", nil)
			req = req.WithContext(WithUserEmail(req.Context(), tc.email))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
