package handler

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func portalSlugRequest(method, target, slug string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("slug", slug)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeContext))
}

func TestPortalSessionCookieIsHttpOnlyAndScopedToSlug(t *testing.T) {
	request := portalSlugRequest(http.MethodPost, "https://app.example.com/api/public/portal/acme/auth/exchange", "acme")
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	setPortalCookie(recorder, request, "session-secret", 7*24*60*60)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != "/api/public/portal/acme" || cookies[0].Value != "session-secret" {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), "SameSite=Lax") {
		t.Fatal("missing SameSite=Lax")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse("https://app.example.com/api/public/portal/acme/auth/exchange")
	jar.SetCookies(origin, cookies)
	for target, want := range map[string]int{
		"https://app.example.com/api/public/portal/acme/session":       1,
		"https://app.example.com/api/public/portal/acme/requests/r1":   1,
		"https://app.example.com/api/public/portal/other/session":      0,
		"https://app.example.com/api/public/portal/acme-other/session": 0,
	} {
		u, _ := url.Parse(target)
		if got := len(jar.Cookies(u)); got != want {
			t.Errorf("%s receives %d portal cookies, want %d", target, got, want)
		}
	}
}

func TestPortalSessionCookieClearsSlugAndLegacyPaths(t *testing.T) {
	request := portalSlugRequest(http.MethodDelete, "/api/public/portal/acme/session", "acme")
	recorder := httptest.NewRecorder()
	clearPortalCookie(recorder, request, portalCookiePath(request))
	clearPortalCookie(recorder, request, portalCookieRoot)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Path != "/api/public/portal/acme" || cookies[1].Path != "/api/public/portal/" {
		t.Fatalf("unexpected cleared cookies: %+v", cookies)
	}
	for _, cookie := range cookies {
		if cookie.MaxAge != -1 || cookie.Value != "" {
			t.Fatalf("cookie not cleared: %+v", cookie)
		}
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
