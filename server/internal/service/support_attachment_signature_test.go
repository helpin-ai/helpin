package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

// testSupportObjectStore serves stored objects to an S3 client and records deletions.
type testSupportObjectStore struct {
	client  *storage.S3Client
	mu      sync.Mutex
	objects map[string][]byte
	deleted []string
}

func newTestSupportObjectStore(t *testing.T, objects map[string][]byte) *testSupportObjectStore {
	t.Helper()
	store := &testSupportObjectStore{objects: objects}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		store.mu.Lock()
		defer store.mu.Unlock()
		switch r.Method {
		case http.MethodDelete:
			store.deleted = append(store.deleted, key)
			delete(store.objects, key)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			data, ok := store.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code></Error>`))
				return
			}
			_, _ = w.Write(data)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	store.client = storage.NewS3Client("key", "secret", "bucket", "us-east-1", server.URL, "")
	return store
}

func TestSupportAttachmentConfirmUploadChecksContent(t *testing.T) {
	exe := append([]byte("MZ\x90\x00\x03\x00\x00\x00"), make([]byte, 64)...)
	tests := []struct {
		name        string
		stored      []byte
		wantError   string
		wantRemoved bool
	}{
		{name: "real photo", stored: []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")},
		{name: "program named jpg", stored: exe, wantError: "photo.jpg can't be sent because its contents don't match its file type", wantRemoved: true},
		{name: "upload never arrived", wantError: "photo.jpg did not finish uploading. Please try again."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			createSupportAttachmentTestTable(t, db)
			repo := repository.NewSupportAttachmentRepository(db)
			objects := map[string][]byte{}
			if tt.stored != nil {
				objects["uploads/photo.jpg"] = tt.stored
			}
			store := newTestSupportObjectStore(t, objects)
			svc := NewSupportAttachmentService(repo, store.client)
			sessionID := "visitor-session"
			attachment := &model.SupportAttachment{
				WorkspaceID: "workspace-1", FileName: "photo.jpg", FileSize: int64(len(tt.stored)), ContentType: "image/jpeg",
				StorageKey: "uploads/photo.jpg", UploadedByType: "customer", SessionID: &sessionID,
			}
			if err := repo.Create(context.Background(), attachment); err != nil {
				t.Fatal(err)
			}

			err := svc.ConfirmUpload(context.Background(), attachment.ID, "customer", nil, &sessionID)

			stored, getErr := repo.GetByID(context.Background(), attachment.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("ConfirmUpload: %v", err)
				}
				if stored == nil || !stored.IsUploaded {
					t.Fatal("real file was not confirmed")
				}
				return
			}
			if message, ok := IsSupportAttachmentRejected(err); !ok || message != tt.wantError {
				t.Fatalf("error = %v, want rejection %q", err, tt.wantError)
			}
			if stored != nil && stored.IsUploaded {
				t.Fatal("refused file was confirmed")
			}
			if tt.wantRemoved && (stored != nil || len(store.deleted) != 1 || store.deleted[0] != "uploads/photo.jpg") {
				t.Fatalf("refused file kept: record=%v deleted=%v", stored != nil, store.deleted)
			}
		})
	}
}

func TestInboundEmailAttachmentWithMismatchedContentIsRefusedOnce(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	store := newTestSupportObjectStore(t, map[string][]byte{})
	env.service.SetAttachmentService(NewSupportAttachmentService(repository.NewSupportAttachmentRepository(env.convRepo.DB()), store.client))
	ctx := context.Background()
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Invoice", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	program := append([]byte("MZ\x90\x00\x03\x00\x00\x00"), make([]byte, 64)...)
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: "conv-" + conv.ID + "@replies.helpin.ai", TextBody: "Invoice attached"}
	p.Attachments = append(p.Attachments, model.PostmarkInboundAttachment{Name: "invoice.pdf", ContentType: "application/pdf", Content: base64.StdEncoding.EncodeToString(program), ContentLength: int64(len(program))})
	if err := env.service.ProcessInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if err := env.convRepo.DB().Model(&model.SupportInboundJob{}).Where("kind = ?", "attachment").Update("available_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "attachment"); err != nil {
		t.Fatal(err)
	}

	var attachment model.SupportAttachment
	if err := env.convRepo.DB().First(&attachment, "conversation_id = ?", conv.ID).Error; err != nil {
		t.Fatal(err)
	}
	if attachment.ProcessingStatus != supportAttachmentStatusRejected || attachment.IsUploaded {
		t.Fatalf("attachment status = %q uploaded = %v, want rejected", attachment.ProcessingStatus, attachment.IsUploaded)
	}
	if attachment.ProcessingError != "invoice.pdf was not saved because its contents don't match its file type" {
		t.Fatalf("processing error = %q", attachment.ProcessingError)
	}
	var job model.SupportInboundJob
	if err := env.convRepo.DB().First(&job, "id = ?", attachment.ID).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "completed" || job.Payload != "" {
		t.Fatalf("job status = %q with payload kept = %v, want completed without the file", job.Status, job.Payload != "")
	}
}

// isoMedia builds an ISO base media "ftyp" box with a major and compatible brands.
func isoMedia(major string, compatible ...string) []byte {
	box := []byte{0, 0, 0, 0}
	box = append(box, "ftyp"+major+"\x00\x00\x00\x00"...)
	for _, brand := range compatible {
		box = append(box, brand...)
	}
	box[3] = byte(len(box))
	return append(box, make([]byte, 16)...)
}

func tarHeader() []byte {
	head := make([]byte, 512)
	copy(head, "notes.txt")
	copy(head[257:], "ustar\x0000")
	return head
}

func TestSupportAttachmentContentMatches(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpeg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	exe := append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"), make([]byte, 64)...)
	elf := append([]byte("\x7fELF\x02\x01\x01"), make([]byte, 32)...)
	html := []byte("<!DOCTYPE html><html><body><script>alert(1)</script>")
	zip := []byte("PK\x03\x04\x14\x00\x06\x00")
	ole := []byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1\x00\x00")

	tests := []struct {
		name     string
		declared string
		head     []byte
		want     bool
	}{
		{"png", "image/png", png, true},
		{"jpeg", "image/jpeg", jpeg, true},
		{"png named jpg", "image/jpeg", png, true},
		{"gif", "image/gif", []byte("GIF89a\x01\x00"), true},
		{"webp", "image/webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), true},
		{"heic photo", "image/heic", isoMedia("heic", "mif1", "heic"), true},
		{"heif by compatible brand", "image/heif", isoMedia("msf1", "mif1"), true},
		{"heic named jpg", "image/jpeg", isoMedia("heic", "mif1"), true},
		{"program named jpg", "image/jpeg", exe, false},
		{"web page named png", "image/png", html, false},
		{"video named jpg", "image/jpeg", isoMedia("isom", "mp41"), false},
		{"svg", "image/svg+xml", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`), true},
		{"png named svg", "image/svg+xml", png, false},
		{"web page named svg", "image/svg+xml", html, false},
		{"mp4", "video/mp4", isoMedia("isom", "mp41"), true},
		{"mov named mp4", "video/mp4", isoMedia("qt  "), true},
		{"old quicktime", "video/quicktime", append([]byte("\x00\x00\x00\x08wide"), make([]byte, 8)...), true},
		{"webm", "video/webm", []byte("\x1a\x45\xdf\xa3\x9f\x42\x86\x81"), true},
		{"matroska", "video/x-matroska", []byte("\x1a\x45\xdf\xa3\x93\x42\x82\x88matroska"), true},
		{"avi", "video/x-msvideo", []byte("RIFF\x24\x00\x00\x00AVI LIST"), true},
		{"mpeg", "video/mpeg", []byte("\x00\x00\x01\xba\x44\x00"), true},
		{"program named mp4", "video/mp4", exe, false},
		{"photo named mp4", "video/mp4", png, false},
		{"pdf", "application/pdf", []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3"), true},
		{"pdf after junk", "application/pdf", append(bytes.Repeat([]byte{' '}, 100), "%PDF-1.4"...), true},
		{"program named pdf", "application/pdf", exe, false},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", zip, true},
		{"password-protected docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ole, true},
		{"xls", "application/vnd.ms-excel", ole, true},
		{"rtf named doc", "application/msword", []byte(`{\rtf1\ansi`), true},
		{"program named docx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", exe, false},
		{"zip", "application/zip", zip, true},
		{"empty zip", "application/zip", []byte("PK\x05\x06\x00\x00"), true},
		{"program named zip", "application/zip", exe, false},
		{"gzip", "application/gzip", []byte("\x1f\x8b\x08\x00"), true},
		{"tar", "application/x-tar", tarHeader(), true},
		{"text", "text/plain", []byte("Hello,\r\nmy order is late.\tThanks"), true},
		{"windows csv", "text/csv", []byte("name,city\nJos\xe9,M\xfcnchen\n"), true},
		{"utf-16 export", "text/plain", []byte("\xff\xfeh\x00i\x00"), true},
		{"markdown", "text/markdown", []byte("# Steps\n\n1. Open settings\n"), true},
		{"json", "application/json", []byte(`{"error":"timeout"}`), true},
		{"program named txt", "text/plain", exe, false},
		{"linux program named csv", "text/csv", elf, false},
		{"photo named txt", "text/plain", png, false},
		{"empty file", "image/png", nil, false},
		{"unknown type", "application/x-msdownload", exe, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportAttachmentContentMatches(tt.declared, tt.head); got != tt.want {
				t.Fatalf("supportAttachmentContentMatches(%q) = %v, want %v", tt.declared, got, tt.want)
			}
		})
	}
}

func TestSupportAttachmentContentMatchesCoversEveryAllowedType(t *testing.T) {
	for contentType := range supportAttachmentContentTypes {
		if !supportAttachmentKnownContentType(contentType) {
			t.Errorf("%s is accepted for upload but has no content check", contentType)
		}
	}
}
