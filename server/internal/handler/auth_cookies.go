package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/helpin-ai/helpin/server/internal/auth"
)

const (
	accessTokenCookieName  = "helpin_access_token"
	refreshTokenCookieName = "helpin_refresh_token"
)

func setAuthCookies(w http.ResponseWriter, r *http.Request, accessToken, refreshToken string) {
	if accessToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     accessTokenCookieName,
			Value:    accessToken,
			Path:     "/api",
			MaxAge:   maxAgeSeconds(tokenTTL(accessToken, auth.AccessTokenTTL)),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}

	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     refreshTokenCookieName,
			Value:    refreshToken,
			Path:     "/api/auth/refresh",
			MaxAge:   maxAgeSeconds(tokenTTL(refreshToken, auth.RefreshTokenTTL)),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range []http.Cookie{
		{Name: accessTokenCookieName, Path: "/api"},
		{Name: refreshTokenCookieName, Path: "/api/auth/refresh"},
	} {
		http.SetCookie(w, &http.Cookie{
			Name:     cookie.Name,
			Value:    "",
			Path:     cookie.Path,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   secureCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func tokenTTL(tokenString string, fallback time.Duration) time.Duration {
	claims := auth.Claims{}
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(tokenString, &claims); err == nil && claims.ExpiresAt != nil {
		if ttl := time.Until(claims.ExpiresAt.Time); ttl > 0 {
			return ttl
		}
	}
	return fallback
}

func maxAgeSeconds(ttl time.Duration) int {
	seconds := int(ttl.Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func secureCookie(r *http.Request) bool {
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}

	host := strings.ToLower(r.Host)
	return !(strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]"))
}
