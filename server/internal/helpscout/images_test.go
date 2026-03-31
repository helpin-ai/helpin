package helpscout

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type testImageUploader struct {
	fail bool
}

func (u *testImageUploader) UploadImage(_ context.Context, _ string, _ string, data io.Reader, _ string) (string, error) {
	if u.fail {
		return "", errors.New("upload failed")
	}
	if _, err := io.ReadAll(data); err != nil {
		return "", err
	}
	return "https://cdn.helpin.ai/imported/image.png", nil
}

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
		{"data src fallback", `<img data-src="https://a.com/lazy.png">`, []string{"https://a.com/lazy.png"}},
		{"single quoted src", `<img src='https://a.com/single.png'>`, []string{"https://a.com/single.png"}},
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

func TestReplaceImageURLs_RewritesDataSrc(t *testing.T) {
	html := `<p><img data-src="https://old.com/lazy.png"></p>`
	result := ReplaceImageURLs(html, map[string]string{
		"https://old.com/lazy.png": "https://new.com/lazy.png",
	})
	if !strings.Contains(result, `data-src="https://new.com/lazy.png"`) {
		t.Fatalf("expected data-src to be rewritten, got: %s", result)
	}
}

func TestProcessImagesDetailed_ReportsFailedURLsAndKeepsOriginal(t *testing.T) {
	originalDownload := downloadAndUploadImage
	downloadAndUploadImage = func(_ context.Context, srcURL string, _ ImageUploader, _ string) (string, error) {
		if strings.Contains(srcURL, "bad.png") {
			return "", errors.New("upload failed")
		}
		return "https://cdn.helpin.ai/imported/" + filenameFromURL(srcURL), nil
	}
	defer func() {
		downloadAndUploadImage = originalDownload
	}()

	html := `<p><img src="https://old.com/ok.png"><img src="https://old.com/bad.png"></p>`
	result, kept, err := ProcessImagesDetailed(context.Background(), html, &testImageUploader{fail: true}, "ws-1")
	if err != nil {
		t.Fatalf("ProcessImagesDetailed returned unexpected error: %v", err)
	}
	if len(kept) != 1 || kept[0] != "https://old.com/bad.png" {
		t.Fatalf("expected only the failed original URL to be kept, got %v", kept)
	}
	if !strings.Contains(result, "https://cdn.helpin.ai/imported/ok.png") {
		t.Fatalf("expected successful image rewrite in HTML, got: %s", result)
	}
	if !strings.Contains(result, "https://old.com/bad.png") {
		t.Fatalf("expected failed image URL to remain in HTML, got: %s", result)
	}
}
