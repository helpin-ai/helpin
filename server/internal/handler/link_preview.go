package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/linkpreview"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// LinkPreviewHandler serves the same metadata contract to edge functions and
// Community's HTML proxy, and serves PNGs after a fresh eligibility check.
type LinkPreviewHandler struct {
	service      *service.LinkPreviewService
	renderer     *linkpreview.Renderer
	appOrigin    string
	apiOrigin    string
	indexURL     string
	client       *http.Client
	indexMu      sync.Mutex
	indexHTML    string
	indexExpires time.Time
	requests     chan struct{}
}

// NewLinkPreviewHandler configures a fixed SPA-shell origin. indexURL is trusted
// operator configuration, never a URL supplied by a preview request.
func NewLinkPreviewHandler(s *service.LinkPreviewService, r *linkpreview.Renderer, appOrigin, apiOrigin, indexURL string) *LinkPreviewHandler {
	appOrigin = strings.TrimRight(appOrigin, "/")
	if apiOrigin == "" {
		apiOrigin = appOrigin
	}
	if indexURL == "" {
		indexURL = appOrigin + "/index.html"
	}
	return &LinkPreviewHandler{service: s, renderer: r, appOrigin: appOrigin, apiOrigin: apiOrigin, indexURL: indexURL, requests: make(chan struct{}, 32), client: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Limit bounds concurrent anonymous database lookups and render waiters.
func (h *LinkPreviewHandler) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case h.requests <- struct{}{}:
			defer func() { <-h.requests }()
			next.ServeHTTP(w, r)
		default:
			previewHeaders(w)
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusTooManyRequests, "Preview temporarily unavailable")
		}
	})
}

type linkPreviewResponse struct {
	model.LinkPreview
	Head string `json:"head"`
}

// Metadata returns only allowed fields and their escaped HTML representation.
func (h *LinkPreviewHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	previewHeaders(w)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	card := h.service.Resolve(ctx, r.URL.Query().Get("path"), r.URL.Query().Get("query"))
	card, head, err := linkpreview.Head(card, h.appOrigin, h.apiOrigin)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Preview unavailable")
		return
	}
	writeJSON(w, http.StatusOK, linkPreviewResponse{LinkPreview: card, Head: head})
}

// Image checks current settings/visibility before consulting the renderer cache.
func (h *LinkPreviewHandler) Image(w http.ResponseWriter, r *http.Request) {
	previewHeaders(w)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card := h.service.Resolve(ctx, r.URL.Query().Get("path"), r.URL.Query().Get("query"))
	data, err := h.renderer.PNG(ctx, card)
	if err != nil {
		slog.WarnContext(ctx, "render link preview", "error", err)
		writeError(w, http.StatusServiceUnavailable, "Preview unavailable")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(data); err != nil {
		slog.DebugContext(ctx, "write link preview image", "error", err)
	}
}

// HTML serves the existing SPA shell with initial metadata. Nginx overwrites
// X-Helpin-App-URI with the original path; it carries no authentication authority.
func (h *LinkPreviewHandler) HTML(w http.ResponseWriter, r *http.Request) {
	previewHeaders(w)
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	uri, err := url.ParseRequestURI(r.Header.Get("X-Helpin-App-URI"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid app path")
		return
	}
	shell, err := h.shell(ctx)
	if err != nil {
		slog.WarnContext(ctx, "load preview app shell", "error", err)
		writeError(w, http.StatusServiceUnavailable, "App shell unavailable")
		return
	}
	card := h.service.Resolve(ctx, uri.Path, uri.RawQuery)
	_, head, err := linkpreview.Head(card, h.appOrigin, h.apiOrigin)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "Preview unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.WriteString(w, linkpreview.Inject(shell, head)); err != nil {
		slog.DebugContext(ctx, "write preview app shell", "error", err)
	}
}

func (h *LinkPreviewHandler) shell(ctx context.Context) (string, error) {
	h.indexMu.Lock()
	defer h.indexMu.Unlock()
	if h.indexHTML != "" && time.Now().Before(h.indexExpires) {
		return h.indexHTML, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.indexURL, nil)
	if err != nil {
		return "", err
	}
	response, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
		return "", fmt.Errorf("app shell returned status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 1024*1024 || !strings.Contains(string(data), linkpreview.MetaStart) || !strings.Contains(string(data), linkpreview.MetaEnd) {
		return "", fmt.Errorf("app shell has no preview metadata block")
	}
	h.indexHTML = string(data)
	h.indexExpires = time.Now().Add(5 * time.Second)
	return h.indexHTML, nil
}

func previewHeaders(w http.ResponseWriter) {
	// This also prevents an upstream CDN from serving an image after opt-out.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("CDN-Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")
}
