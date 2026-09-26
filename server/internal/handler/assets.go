package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"net/http"
)

// AssetHandler exposes explicit public media from a private S3 bucket. Imported
// Docs images use the separate authenticated Docs route and workspace policy.
type AssetHandler struct{ store *storage.S3Client }

func NewAssetHandler(store *storage.S3Client) *AssetHandler { return &AssetHandler{store: store} }

func (h *AssetHandler) Public(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "*")
	if h.store == nil || !h.store.PrivateBucket() || !storage.PublicAssetKey(key) {
		http.NotFound(w, r)
		return
	}
	h.redirect(w, r, key)
}

// DocsImage runs behind authentication, workspace access, and docs.read.
func (h *AssetHandler) DocsImage(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if h.store == nil || !storage.DocsImageKey(key, getWorkspaceID(r)) {
		http.NotFound(w, r)
		return
	}
	h.redirect(w, r, key)
}

func (h *AssetHandler) redirect(w http.ResponseWriter, r *http.Request, key string) {
	target, err := h.store.GeneratePresignedInlineGetURL(key)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "asset storage unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}
