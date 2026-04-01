package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeImageUploader struct {
	filename    string
	contentType string
	body        string
}

type fakeDocsImageStore struct {
	key         string
	contentType string
	size        int64
	body        []byte
	publicRead  bool
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

func (f *fakeDocsImageStore) PutObject(_ context.Context, key, contentType string, size int64, body io.Reader, publicRead bool) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.key = key
	f.contentType = contentType
	f.size = size
	f.body = payload
	f.publicRead = publicRead
	return nil
}

func (f *fakeDocsImageStore) PublicURL(key string) string {
	return "https://cdn.helpin.ai/" + key
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestDocsImportService_ImportExternalImageWithUploader(t *testing.T) {
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://images.example.com/assets/diagram.png?cache=1" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(strings.NewReader("png-binary")),
			}, nil
		}),
	}
	defer func() {
		http.DefaultClient = originalClient
	}()

	svc := &DocsImportService{}
	uploader := &fakeImageUploader{}

	got, err := svc.importExternalImageWithUploader(context.Background(), "ws-1", "https://images.example.com/assets/diagram.png?cache=1", uploader)
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
	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://images.example.com/readme.txt" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("not-an-image")),
			}, nil
		}),
	}
	defer func() {
		http.DefaultClient = originalClient
	}()

	svc := &DocsImportService{}
	uploader := &fakeImageUploader{}

	_, err := svc.importExternalImageWithUploader(context.Background(), "ws-1", "https://images.example.com/readme.txt", uploader)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not an image") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3ImageUploader_UsesActualPayloadLength(t *testing.T) {
	t.Parallel()

	store := &fakeDocsImageStore{}
	uploader := &s3ImageUploader{store: store}

	got, err := uploader.UploadImage(context.Background(), "ws-1", "diagram.png", bytes.NewBufferString("png-binary"), "image/png")
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if !strings.HasPrefix(got, "https://cdn.helpin.ai/docs-import/ws-1/") {
		t.Fatalf("public url = %q", got)
	}
	if store.contentType != "image/png" {
		t.Fatalf("contentType = %q", store.contentType)
	}
	if store.size != int64(len("png-binary")) {
		t.Fatalf("size = %d", store.size)
	}
	if string(store.body) != "png-binary" {
		t.Fatalf("body = %q", string(store.body))
	}
	if !store.publicRead {
		t.Fatal("expected publicRead to be true")
	}
	if !strings.HasPrefix(store.key, "docs-import/ws-1/") {
		t.Fatalf("key = %q", store.key)
	}
}
