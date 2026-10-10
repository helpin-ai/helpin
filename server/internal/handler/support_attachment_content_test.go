package handler

import (
	"io"
	"mime"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestWriteSupportAttachmentContent(t *testing.T) {
	request := httptest.NewRequest("GET", "/content", nil)
	response := httptest.NewRecorder()
	writeSupportAttachmentContent(response, request, &service.SupportAttachmentContent{
		Body: io.NopCloser(strings.NewReader("image bytes")), Size: 11,
		ContentType: "image/png", FileName: "screenshot é.png",
	})
	if response.Code != 200 || response.Body.String() != "image bytes" {
		t.Fatalf("unexpected response: %d %q", response.Code, response.Body.String())
	}
	for key, want := range map[string]string{
		"Content-Type": "image/png", "Content-Length": "11", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff",
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := response.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	disposition, params, err := mime.ParseMediaType(response.Header().Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || params["filename"] != "screenshot é.png" {
		t.Fatalf("invalid download disposition: %s %v %v", disposition, params, err)
	}
}
