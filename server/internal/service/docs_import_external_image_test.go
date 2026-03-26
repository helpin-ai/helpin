package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeImageUploader struct {
	filename    string
	contentType string
	body        string
}

func (f *fakeImageUploader) UploadImage(_ context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error) {
	payload, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}
	f.filename = filename
	f.contentType = contentType
	f.body = string(payload)
	return "https://cdn.helpin.ai/docs-import/ws-1/" + filename, nil
}

func TestDocsImportService_ImportExternalImageWithUploader(t *testing.T) {
	t.Parallel()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-binary"))
	}))
	defer imageServer.Close()

	svc := &DocsImportService{}
	uploader := &fakeImageUploader{}

	got, err := svc.importExternalImageWithUploader(context.Background(), "ws-1", imageServer.URL+"/assets/diagram.png?cache=1", uploader)
	if err != nil {
		t.Fatalf("importExternalImageWithUploader: %v", err)
	}
	if got != "https://cdn.helpin.ai/docs-import/ws-1/diagram.png" {
		t.Fatalf("url = %q", got)
	}
	if uploader.filename != "diagram.png" {
		t.Fatalf("filename = %q", uploader.filename)
	}
	if uploader.contentType != "image/png" {
		t.Fatalf("contentType = %q", uploader.contentType)
	}
	if uploader.body != "png-binary" {
		t.Fatalf("body = %q", uploader.body)
	}
}

func TestDocsImportService_ImportExternalImageWithUploader_RejectsNonImages(t *testing.T) {
	t.Parallel()

	textServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not-an-image"))
	}))
	defer textServer.Close()

	svc := &DocsImportService{}
	uploader := &fakeImageUploader{}

	_, err := svc.importExternalImageWithUploader(context.Background(), "ws-1", textServer.URL+"/readme.txt", uploader)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not an image") {
		t.Fatalf("unexpected error: %v", err)
	}
}
