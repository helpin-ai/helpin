package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PublicGetBootstrap returns the configuration and locale-specific spaces needed
// to render the public help-center shell in one request.
func (h *DocsHandler) PublicGetBootstrap(w http.ResponseWriter, r *http.Request) {
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}

	locale := publicDefaultLocale(cfg)
	requestPath := strings.TrimSpace(r.URL.Query().Get("path"))
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(segments) > 0 {
		candidate := strings.ToLower(strings.TrimSpace(segments[0]))
		if publicLocaleEnabled(cfg, candidate) {
			locale = candidate
		}
	}

	spaces, err := h.helpcenterSvc.ListPublicSpaces(r.Context(), cfg.WorkspaceID, locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "help center unavailable")
		return
	}

	resp := publicHelpcenterBootstrapResponse{
		Config: h.publicHelpcenterConfigResponse(r, cfg),
		Locale: locale,
		Spaces: spaces,
	}
	setHelpcenterCacheHeader(w, helpcenterCachePublicRead)
	writeJSONWithETag(w, r, http.StatusOK, resp)
}

func (h *DocsHandler) publicHelpcenterConfigResponse(
	r *http.Request,
	cfg *model.DocsHelpcenterConfig,
) publicHelpcenterConfigResponse {
	resp := publicHelpcenterConfigResponse{DocsHelpcenterConfig: cfg, PublicWidgetURL: h.publicWidgetURL, PublicSDKURL: h.publicSDKURL}
	if !cfg.ChatWidgetEnabled || h.supportWidgetConfig == nil {
		return resp
	}

	inst, _, err := h.supportWidgetConfig.GetInstallation(r.Context(), cfg.WorkspaceID)
	if err != nil {
		slog.WarnContext(
			r.Context(),
			"public helpcenter widget config unavailable",
			"error", err,
			"workspace_id", cfg.WorkspaceID,
		)
		return resp
	}
	if inst != nil && inst.Active && strings.TrimSpace(inst.WidgetKey) != "" {
		widgetKey := inst.WidgetKey
		resp.SupportWidgetKey = &widgetKey
	}
	return resp
}
