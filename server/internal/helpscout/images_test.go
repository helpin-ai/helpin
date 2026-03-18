package helpscout

import (
	"strings"
	"testing"
)

func TestExtractImageURLs(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{"no images", "<p>text</p>", nil},
		{"single image", `<img src="https://cdn.helpscout.net/img1.png">`, []string{"https://cdn.helpscout.net/img1.png"}},
		{"multiple images", `<img src="https://a.com/1.png"><img src="https://b.com/2.png">`, []string{"https://a.com/1.png", "https://b.com/2.png"}},
		{"duplicate URLs", `<img src="https://a.com/1.png"><img src="https://a.com/1.png">`, []string{"https://a.com/1.png"}},
		{"with alt", `<img src="https://a.com/pic.jpg" alt="photo">`, []string{"https://a.com/pic.jpg"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImageURLs(tt.html)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractImageURLs() = %v, want %v", got, tt.want)
			}
			for i, url := range got {
				if url != tt.want[i] {
					t.Errorf("ExtractImageURLs()[%d] = %q, want %q", i, url, tt.want[i])
				}
			}
		})
	}
}

func TestReplaceImageURLs(t *testing.T) {
	html := `<p><img src="https://old.com/img.png"> text <img src="https://old.com/img2.png"></p>`
	urlMap := map[string]string{
		"https://old.com/img.png":  "https://new.com/abc.png",
		"https://old.com/img2.png": "https://new.com/def.png",
	}
	result := ReplaceImageURLs(html, urlMap)
	if !strings.Contains(result, "https://new.com/abc.png") {
		t.Errorf("expected new URL abc.png in result: %s", result)
	}
	if !strings.Contains(result, "https://new.com/def.png") {
		t.Errorf("expected new URL def.png in result: %s", result)
	}
	if strings.Contains(result, "https://old.com/") {
		t.Errorf("old URLs should be replaced: %s", result)
	}
}
