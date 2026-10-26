package helpscout

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var _pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func imageResponse(contentType string, body []byte) *http.Response {
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{Header: header, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: -1}
}

func TestImageBody(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		filename    string
		want        string
		wantErr     error
	}{
		{name: "declared image", contentType: "image/jpeg; charset=binary", body: []byte("x"), want: "image/jpeg"},
		{name: "generic type sniffed", contentType: "application/octet-stream", body: _pngHeader, want: "image/png"},
		{name: "missing type from name", body: []byte("<svg/>"), filename: "logo.svg", want: "image/svg+xml"},
		{name: "html page", contentType: "text/html", body: []byte("<html>"), wantErr: ErrNotImage},
		{name: "generic non-image", contentType: "application/octet-stream", body: []byte("plain"), filename: "notes.txt", wantErr: ErrNotImage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType, err := ImageBody(imageResponse(tt.contentType, tt.body), tt.filename)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			read, err := io.ReadAll(body)
			if contentType != tt.want || err != nil || !bytes.Equal(read, tt.body) {
				t.Fatalf("got %q %q %v", contentType, read, err)
			}
		})
	}
}

func TestImageBodyRejectsOversizedImages(t *testing.T) {
	declared := imageResponse("image/png", nil)
	declared.ContentLength = MaxImageBytes + 1
	if _, _, err := ImageBody(declared, ""); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("declared size: %v", err)
	}
	streamed := imageResponse("image/png", nil)
	streamed.Body = io.NopCloser(io.MultiReader(bytes.NewReader(_pngHeader), strings.NewReader(strings.Repeat("x", MaxImageBytes))))
	body, _, err := ImageBody(streamed, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, body); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("streamed size: %v", err)
	}
}
