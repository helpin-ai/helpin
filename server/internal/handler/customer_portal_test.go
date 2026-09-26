package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPortalSessionCookieIsHttpOnlyScopedAndCleared(t *testing.T) {
	recorder := httptest.NewRecorder()
	setPortalCookie(recorder, "session-secret", 7*24*60*60)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != "/api/public/portal/" || cookies[0].Value != "session-secret" {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), "SameSite=Lax") {
		t.Fatal("missing SameSite=Lax")
	}
	request := httptest.NewRequest("GET", "/api/public/portal/slug/session", nil)
	request.AddCookie(cookies[0])
	if got := portalSessionCookie(request); got != "session-secret" {
		t.Fatalf("session cookie: %q", got)
	}
	cleared := httptest.NewRecorder()
	setPortalCookie(cleared, "", -1)
	cookie := cleared.Result().Cookies()[0]
	if cookie.MaxAge != -1 || cookie.Value != "" || cookie.Path != cookies[0].Path {
		t.Fatalf("cookie not cleared: %+v", cookie)
	}
}
