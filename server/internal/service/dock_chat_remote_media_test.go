package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
