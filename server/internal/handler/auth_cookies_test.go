package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureCookie(t *testing.T) {
	t.Run("false for plain http remote dev host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://203.0.113.10:8080/api/auth/signin", nil)

		if secureCookie(req) {
			t.Fatal("expected non-secure cookie for plain HTTP remote dev host")
		}
	})

	t.Run("true for direct https", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "https://app.helpin.ai/api/auth/signin", nil)
		req.TLS = &tls.ConnectionState{}

		if !secureCookie(req) {
			t.Fatal("expected secure cookie for direct HTTPS request")
		}
	})

	t.Run("true for proxied https", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://api:8080/api/auth/signin", nil)
		req.Header.Set("X-Forwarded-Proto", "https")

		if !secureCookie(req) {
			t.Fatal("expected secure cookie for HTTPS request behind proxy")
		}
	})
}
