package handler

import (
	"net/http"
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

func TestPortalSessionIgnoresOtherCredentials(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*http.Request)
		want   string
	}{
		{"anonymous", func(*http.Request) {}, ""},
		{"workspace bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer workspace-token") }, ""},
		{"portal bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer portal-token") }, ""},
		{"widget cookie", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "widget_session", Value: "widget-token"}) }, ""},
		{"portal cookie", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: portalCookie, Value: "portal-token"}) }, "portal-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/public/portal/slug/session", nil)
			tc.modify(r)
			if got := portalSessionCookie(r); got != tc.want {
				t.Fatalf("portal credential = %q, want %q", got, tc.want)
			}
		})
	}
}
