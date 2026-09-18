package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

type widgetOriginAuthorizer interface {
	AuthorizeWidgetOrigin(context.Context, string, widgetorigin.Reference) error
}

// RequireOrigin protects visitor endpoints after routing has resolved path IDs.
// CORS preflights are transport checks; every actual request is authorized here.
func (h *SupportInboxWidgetHandler) RequireOrigin(next http.Handler) http.Handler {
	if h == nil || h.supportService == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "widget service is unavailable")
		})
	}
	return requireWidgetOrigin(h.supportService, next, h.publicOrigin)
}

func requireWidgetOrigin(authorizer widgetOriginAuthorizer, next http.Handler, publicOrigins ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref, err := widgetOriginReference(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid widget request")
			return
		}
		origin := r.Header.Get("Origin")
		count := len(r.Header.Values("Origin"))
		// Browsers omit Origin on same-origin GETs. Fetch Metadata is browser-
		// owned; accept this narrow case only for the configured public host,
		// and still require that origin in the installation's allow-list.
		if count == 0 && len(publicOrigins) == 1 && r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Site") == "same-origin" && (r.Header.Get("Sec-Fetch-Mode") == "cors" || r.Header.Get("Sec-Fetch-Mode") == "same-origin") {
			if u, e := url.Parse(publicOrigins[0]); e == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host) {
				origin = publicOrigins[0]
				count = 1
			}
		}
		if count != 1 || authorizer.AuthorizeWidgetOrigin(r.Context(), origin, ref) != nil {
			writeError(w, http.StatusForbidden, "Widget access denied. Add your site's origin in widget settings.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin, Sec-Fetch-Site, Sec-Fetch-Mode")
		if r.Method != http.MethodGet || ref.SessionToken != "" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func widgetOriginReference(w http.ResponseWriter, r *http.Request) (widgetorigin.Reference, error) {
	ref := widgetorigin.Reference{InstallationID: chi.URLParam(r, "id")}
	merge := func(dst *string, values ...string) error {
		for _, v := range values {
			if v == "" {
				continue
			}
			if *dst != "" && *dst != v {
				return fmt.Errorf("conflicting widget references")
			}
			*dst = v
		}
		return nil
	}
	if err := merge(&ref.WidgetKey, r.URL.Query()["widget_key"]...); err != nil {
		return ref, err
	}
	if err := merge(&ref.SessionToken, r.URL.Query()["session_token"]...); err != nil {
		return ref, err
	}
	if err := merge(&ref.SessionToken, r.Header.Values("X-Session-Token")...); err != nil {
		return ref, err
	}
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			return ref, err
		}
		if err := r.Body.Close(); err != nil {
			return ref, err
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var credentials struct {
			WidgetKey    string `json:"widget_key"`
			APIKey       string `json:"api_key"`
			SessionToken string `json:"session_token"`
		}
		if len(body) == 0 {
			return ref, nil
		}
		if err := json.Unmarshal(body, &credentials); err != nil {
			return ref, err
		}
		if err := merge(&ref.WidgetKey, credentials.WidgetKey, credentials.APIKey); err != nil {
			return ref, err
		}
		if err := merge(&ref.SessionToken, credentials.SessionToken); err != nil {
			return ref, err
		}
	}
	return ref, nil
}

func (h *SupportInboxWidgetHandler) SetPublicOrigin(origin string) { h.publicOrigin = origin }
