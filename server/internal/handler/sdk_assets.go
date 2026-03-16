package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SDKAssetsHandler serves the built SDK files (lib.js, helpin.[hash].js)
// with appropriate cache headers.
type SDKAssetsHandler struct {
	distDir string
}

// NewSDKAssetsHandler creates a handler that serves SDK assets from the given dist directory.
func NewSDKAssetsHandler(distDir string) *SDKAssetsHandler {
	return &SDKAssetsHandler{distDir: distDir}
}

// ServeSDK handles GET /sdk/{file} — serves SDK dist files with proper caching.
func (h *SDKAssetsHandler) ServeSDK(w http.ResponseWriter, r *http.Request) {
	// Extract filename from path: /sdk/lib.js → lib.js
	reqPath := strings.TrimPrefix(r.URL.Path, "/sdk/")
	reqPath = strings.TrimPrefix(reqPath, "/")

	if reqPath == "" || strings.Contains(reqPath, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	filePath := filepath.Join(h.distDir, reqPath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Content type
	switch {
	case strings.HasSuffix(reqPath, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(reqPath, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}

	// Cache headers:
	// lib.js = loader (mutable, short cache)
	// helpin.[hash].js = full SDK (immutable, long cache)
	if reqPath == "lib.js" {
		w.Header().Set("Cache-Control", "public, max-age=300") // 5 min
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year
	}

	// CORS — SDK must be loadable from any origin
	w.Header().Set("Access-Control-Allow-Origin", "*")

	http.ServeFile(w, r, filePath)
}
