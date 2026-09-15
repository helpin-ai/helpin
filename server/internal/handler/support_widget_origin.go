package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
	return requireWidgetOrigin(h.supportService, next)
}

func requireWidgetOrigin(authorizer widgetOriginAuthorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref, err := widgetOriginReference(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid widget request")
			return
		}
		if len(r.Header.Values("Origin")) != 1 || authorizer.AuthorizeWidgetOrigin(r.Context(), r.Header.Get("Origin"), ref) != nil {
			writeError(w, http.StatusForbidden, "Widget access denied. Add your site's origin in widget settings.")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Cache-Control", "no-store")
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
