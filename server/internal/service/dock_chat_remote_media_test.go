package service

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDockChatExternalImageDownloadPreservesValidation(t *testing.T) {
	var requests atomic.Int32
	client := newDockMediaDownloadTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/status":
			http.Error(w, "unavailable", http.StatusNotFound)
		case "/type":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>not an image</html>"))
		case "/signature":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("not a PNG"))
		case "/large":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(make([]byte, dockChatExternalImageMaxBytes+1))
		case "/private":
			http.Redirect(w, r, "https://127.0.0.1/valid", http.StatusFound)
		case "/insecure":
			http.Redirect(w, r, "http://example.com/valid", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "https://example.com/loop", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(dockDownloadPNG("valid"))
		}
	}))
	for _, test := range []struct{ name, path, wantError string }{
		{"success", "/valid", ""},
		{"HTTP failure", "/status", "external media returned an error"},
		{"content type", "/type", "external URL is not an image"},
		{"signature", "/signature", "signature does not match content type"},
		{"size limit", "/large", "external image is too large"},
		{"private redirect", "/private", "unsafe redirect"},
		{"insecure redirect", "/insecure", "unsafe redirect"},
		{"redirect limit", "/loop", "unsafe redirect"},
	} {
		t.Run(test.name, func(t *testing.T) {
			contentType, dataURL, err := fetchDockChatExternalImage(context.Background(), client, "https://example.com"+test.path)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil || contentType != "image/png" || !strings.HasPrefix(dataURL, "data:image/png;base64,") {
				t.Fatalf("download type/error = %q/%v", contentType, err)
			}
		})
	}
	before := requests.Load()
	if _, _, err := fetchDockChatExternalImage(context.Background(), client, "https://localhost/valid"); err == nil {
		t.Fatal("private URL was accepted")
	}
	if requests.Load() != before {
		t.Fatal("private URL reached the HTTP transport")
	}
}

func TestDockChatExternalMediaDialRejectsPrivateAddresses(t *testing.T) {
	for _, address := range []string{"127.0.0.1:443", "10.0.0.1:443", "169.254.169.254:443", "[::1]:443", "[::ffff:127.0.0.1]:443"} {
		t.Run(address, func(t *testing.T) {
			conn, err := dockChatSafeDialContext(context.Background(), "tcp", address)
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "media host address is not allowed") {
				t.Fatalf("dial error = %v, want private address rejection", err)
			}
		})
	}
}

func TestDockChatExternalMediaURLRejectsUnsafeTargets(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/image.png",
		"https://localhost/image.png",
		"https://127.0.0.1/image.png",
		"file:///etc/passwd",
	} {
		if isDockChatExternalMediaURL(raw) {
			t.Fatalf("isDockChatExternalMediaURL(%q) = true, want false", raw)
		}
	}
	if !isDockChatExternalMediaURL("https://images.example.com/screenshot.png") {
		t.Fatal("expected public HTTPS image URL to be eligible")
	}
}

func TestDockChatHostedImageURLsUsesDirectImagesAndLoomPreview(t *testing.T) {
	loomImage := "https://cdn.loom.com/thumb.png"
	message := model.SupportMessage{
		Content:  "Screenshot: https://i.imgur.com/error.png and private: http://127.0.0.1/nope.png",
		Metadata: `{"link_previews":[{"url":"https://www.loom.com/share/abc","image_url":"` + loomImage + `"}]}`,
	}
	urls := dockChatHostedImageURLs(message)
	if len(urls) != 2 || urls[0] != "https://i.imgur.com/error.png" || urls[1] != loomImage {
		t.Fatalf("dockChatHostedImageURLs() = %#v", urls)
	}
}
