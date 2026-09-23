package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var _pngHeader = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}

func TestPrepareDocumentImageUpload(t *testing.T) {
	env, attachments := setupMCPUploadTest(t)
	result, err := env.run("prepare_document_image_upload",
		`{"document_id":"document-1","file_name":"../../shot.png","content_type":"image/png","size_bytes":2048}`)
	if err != nil {
		t.Fatalf("prepare_document_image_upload error = %v", err)
	}
	request := attachments.createRequest
	if request.EntityType != entityTypeEditorUpload || request.EntityID != "document-1" || !request.Private ||
		request.FileName != "shot.png" || request.FileSize != 2048 {
		t.Fatalf("create request = %#v", request)
	}
	data := result.Data.(map[string]any)
	if data["upload_url"] != "https://upload.example.com/att-1" || data["image_src"] != "/api/pm/attachments/att-1/content" {
		t.Fatalf("prepare result = %#v", data)
	}
}

func TestPrepareDocumentImageUploadRejectsUnsupportedType(t *testing.T) {
	env, attachments := setupMCPUploadTest(t)
	_, err := env.run("prepare_document_image_upload",
		`{"document_id":"document-1","file_name":"x.svg","content_type":"image/svg+xml","size_bytes":20}`)
	assertMCPToolErrorCode(t, err, MCPErrorCodeUnsupportedContentType)
	if attachments.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", attachments.createCalls)
	}
}

func TestCompleteDocumentImageUpload(t *testing.T) {
	tests := []struct {
		name       string
		attachment *model.PMAttachment
		verifyErr  error
		wantCode   string
		wantErr    error
	}{
		{name: "confirms own upload", attachment: mcpTestAttachment("user-1")},
		{name: "hides another user's upload", attachment: mcpTestAttachment("user-2"), wantErr: ErrMCPNotFound},
		{name: "hides a missing attachment", attachment: nil, wantErr: ErrMCPNotFound},
		{name: "reports a missing object", attachment: mcpTestAttachment("user-1"), verifyErr: ErrAttachmentUploadMissing, wantCode: MCPErrorCodeUploadNotFound},
		{name: "reports a size mismatch", attachment: mcpTestAttachment("user-1"), verifyErr: ErrAttachmentUploadSizeMismatch, wantCode: MCPErrorCodeUploadSizeMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, attachments := setupMCPUploadTest(t)
			attachments.attachment = tt.attachment
			attachments.verifyErr = tt.verifyErr
			result, err := env.run("complete_document_image_upload", `{"attachment_id":"att-1","alt_text":"Board [view]"}`)
			switch {
			case tt.wantCode != "":
				assertMCPToolErrorCode(t, err, tt.wantCode)
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if attachments.verifyCalls != 0 {
					t.Fatalf("verify calls = %d, want 0", attachments.verifyCalls)
				}
			default:
				if err != nil {
					t.Fatalf("complete error = %v", err)
				}
				if markdown := result.Data.(map[string]any)["markdown"]; markdown != "![Board view](/api/pm/attachments/att-1/content)" {
					t.Fatalf("markdown = %v", markdown)
				}
			}
		})
	}
}

func TestUploadDocumentImageFromURL(t *testing.T) {
	t.Run("stores a fetched public image", func(t *testing.T) {
		env, attachments := setupMCPUploadTest(t)
		env.service.fetchImage = func(context.Context, string) (string, []byte, error) {
			return "image/png", _pngHeader, nil
		}
		result, err := env.run("upload_document_image_from_url",
			`{"document_id":"document-1","url":"https://images.example.com/a/board.png?x=1"}`)
		if err != nil {
			t.Fatalf("upload_document_image_from_url error = %v", err)
		}
		if attachments.importRequest.FileName != "board.png" || attachments.importRequest.FileSize != int64(len(_pngHeader)) ||
			!attachments.importRequest.Private {
			t.Fatalf("import request = %#v", attachments.importRequest)
		}
		if result.Data.(map[string]any)["image_src"] != "/api/pm/attachments/att-imported/content" {
			t.Fatalf("result = %#v", result.Data)
		}
	})
	for _, rawURL := range []string{
		"http://images.example.com/a.png",
		"https://127.0.0.1/a.png",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.5/a.png",
		"https://localhost/a.png",
		"https://user:pass@images.example.com/a.png",
	} {
		t.Run("refuses "+rawURL, func(t *testing.T) {
			env, _ := setupMCPUploadTest(t)
			env.service.fetchImage = func(context.Context, string) (string, []byte, error) {
				t.Fatalf("fetch must not run for %s", rawURL)
				return "", nil, nil
			}
			arguments, _ := json.Marshal(map[string]string{"document_id": "document-1", "url": rawURL})
			_, err := env.run("upload_document_image_from_url", string(arguments))
			assertMCPToolErrorCode(t, err, MCPErrorCodeURLNotPublic)
		})
	}
}

func TestFetchMCPPublicImageRefusesLoopbackAtDialTime(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(_pngHeader)
	}))
	defer server.Close()
	client := newDockChatExternalMediaClient()
	client.Transport.(*http.Transport).TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig
	_, _, err := fetchMCPPublicImage(context.Background(), client, server.URL)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("fetch to a loopback address error = %v, want dial-time refusal", err)
	}
}

func TestSanitizeMCPFileName(t *testing.T) {
	tests := map[string]string{
		"../../etc/passwd.png": "passwd.png",
		`C:\shots\a:b.png`:     "a-b.png",
		"":                     "image.png",
		"screenshot":           "screenshot.png",
	}
	for input, want := range tests {
		if got := sanitizeMCPFileName(input, "image/png"); got != want {
			t.Errorf("sanitizeMCPFileName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPMAttachmentVerifyAndConfirmUpload(t *testing.T) {
	tests := []struct {
		name     string
		headSize int64
		headErr  error
		wantErr  error
	}{
		{name: "confirms a stored object of the declared size", headSize: 2048},
		{name: "refuses a missing object", headErr: errors.New("not found"), wantErr: ErrAttachmentUploadMissing},
		{name: "refuses a size mismatch", headSize: 10, wantErr: ErrAttachmentUploadSizeMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			seedUser(t, db, "user-1", "u@test.com", "User", "hash")
			seedWorkspace(t, db, "ws-1", "Workspace", "ws-1", "user-1")
			repo := repository.NewPMAttachmentRepository(db)
			store := &fakeHeaderAttachmentStore{headSize: tt.headSize, headErr: tt.headErr}
			svc := NewPMAttachmentService(repo, store, nil)
			created, err := svc.Create(context.Background(), model.CreateAttachmentRequest{
				EntityType: entityTypeEditorUpload, EntityID: "00000000-0000-0000-0000-000000000001",
				FileName: "a.png", FileSize: 2048, ContentType: "image/png",
			}, "ws-1", "user-1")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			err = svc.VerifyAndConfirmUpload(context.Background(), created.Attachment.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("VerifyAndConfirmUpload error = %v, want %v", err, tt.wantErr)
			}
			stored, err := svc.Get(context.Background(), created.Attachment.ID)
			if err != nil || stored == nil {
				t.Fatalf("Get = %v, %v", stored, err)
			}
			if stored.IsUploaded != (tt.wantErr == nil) {
				t.Fatalf("IsUploaded = %v, want %v", stored.IsUploaded, tt.wantErr == nil)
			}
		})
	}
}

type fakeHeaderAttachmentStore struct {
	fakeAttachmentStore
	headSize int64
	headErr  error
}

func (f *fakeHeaderAttachmentStore) HeadObject(context.Context, string) (int64, error) {
	return f.headSize, f.headErr
}

func setupMCPUploadTest(t *testing.T) (*mcpDocsLifecycleTestEnv, *fakeMCPAttachmentService) {
	t.Helper()
	env := setupMCPDocsLifecycleTest(t, model.DocStatusDraft, model.SpaceTypeExternalCapable)
	attachments := &fakeMCPAttachmentService{attachment: mcpTestAttachment("user-1")}
	env.service.attachments = attachments
	return env, attachments
}

func mcpTestAttachment(uploadedBy string) *model.PMAttachment {
	return &model.PMAttachment{
		ID: "att-1", WorkspaceID: "workspace-1", EntityType: entityTypeEditorUpload,
		EntityID: "document-1", FileSize: 2048, UploadedByID: uploadedBy,
	}
}

type fakeMCPAttachmentService struct {
	attachment    *model.PMAttachment
	verifyErr     error
	createCalls   int
	verifyCalls   int
	createRequest model.CreateAttachmentRequest
	importRequest model.CreateAttachmentRequest
}

func (f *fakeMCPAttachmentService) Create(_ context.Context, request model.CreateAttachmentRequest, workspaceID, userID string) (*model.AttachmentResponse, error) {
	f.createCalls++
	f.createRequest = request
	return &model.AttachmentResponse{
		Attachment: model.PMAttachment{ID: "att-1", WorkspaceID: workspaceID, UploadedByID: userID},
		URL:        "https://upload.example.com/att-1",
	}, nil
}

func (f *fakeMCPAttachmentService) CreateImported(_ context.Context, request model.CreateAttachmentRequest, workspaceID, _ string, body io.Reader) (*model.AttachmentResponse, error) {
	f.importRequest = request
	if _, err := io.ReadAll(body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return &model.AttachmentResponse{Attachment: model.PMAttachment{ID: "att-imported", WorkspaceID: workspaceID}}, nil
}

func (f *fakeMCPAttachmentService) Get(context.Context, string) (*model.PMAttachment, error) {
	return f.attachment, nil
}

func (f *fakeMCPAttachmentService) VerifyAndConfirmUpload(context.Context, string) error {
	f.verifyCalls++
	return f.verifyErr
}
