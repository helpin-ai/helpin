package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/externala2a"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Byte limits for files external agents deliver. They are variables only so
// tests can exercise the limits without large fixtures.
var (
	// externalA2AMaxFileBytes caps one file.
	externalA2AMaxFileBytes int64 = 200 << 20
	// externalA2AMaxRunBytes caps everything one run may upload.
	externalA2AMaxRunBytes int64 = 2 << 30
)

// externalA2AFileTypes maps accepted extensions to the stored content type.
var externalA2AFileTypes = map[string]string{
	".mp4": "video/mp4", ".webm": "video/webm",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".pdf": "application/pdf", ".zip": "application/zip",
	".txt": "text/plain", ".log": "text/plain", ".json": "application/json", ".csv": "text/csv",
}

var errExternalA2AUploadToken = &ExternalA2AUploadError{Status: http.StatusUnauthorized, Message: "upload link is invalid or has expired"}

// ExternalA2AUploadError is an upload rejection with its HTTP status.
type ExternalA2AUploadError struct {
	Status  int
	Message string
}

func (e *ExternalA2AUploadError) Error() string { return e.Message }

func externalA2AUploadExtensions() []string {
	names := make([]string, 0, len(externalA2AFileTypes))
	for ext := range externalA2AFileTypes {
		if ext != ".jpg" {
			names = append(names, strings.TrimPrefix(ext, "."))
		}
	}
	sort.Strings(names)
	return names
}

// ExternalA2AUploadResult is returned to the uploading agent.
type ExternalA2AUploadResult struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// AuthorizeUpload validates an upload bearer token before any body is read.
func (s *ExternalA2AService) AuthorizeUpload(ctx context.Context, bearer string) (*model.A2ARunUploadToken, error) {
	if !s.Enabled() || s.attachments == nil {
		return nil, ErrExternalA2ADisabled
	}
	bearer = strings.TrimSpace(bearer)
	if !strings.HasPrefix(bearer, externalA2AUploadPrefix) || len(bearer) > 256 {
		return nil, errExternalA2AUploadToken
	}
	row, err := s.repo.GetUploadTokenByHash(ctx, sha256Hex(bearer))
	if err != nil {
		return nil, err
	}
	if row == nil || row.RevokedAt != nil || !s.now().Before(row.ExpiresAt) {
		return nil, errExternalA2AUploadToken
	}
	run, err := s.agents.runRepo.GetByIDAny(ctx, row.AgentRunID)
	if err != nil {
		return nil, err
	}
	if run == nil || run.WorkspaceID != row.WorkspaceID || !model.IsAgentRunActiveStatus(run.Status) || externalA2ARunCancelling(run) {
		return nil, errExternalA2AUploadToken
	}
	return row, nil
}

// ReceiveUpload stores one uploaded file on the run's task, attributed to the
// external agent. The body is spooled to a temporary file, never fully held
// in memory, and is rejected past the per-file cap.
func (s *ExternalA2AService) ReceiveUpload(ctx context.Context, grant *model.A2ARunUploadToken, fileName string, body io.Reader) (*ExternalA2AUploadResult, error) {
	fileName = sanitizeExternalA2AFileName(fileName)
	contentType, ok := externalA2AContentType(fileName)
	if !ok {
		return nil, &ExternalA2AUploadError{Status: http.StatusUnsupportedMediaType, Message: "file type is not allowed; use " + strings.Join(externalA2AUploadExtensions(), ", ")}
	}
	record, err := s.repo.Get(ctx, grant.WorkspaceID, grant.ExternalA2AAgentID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Status != model.ExternalA2AStatusActive {
		return nil, errExternalA2AUploadToken
	}
	spool, size, err := spoolExternalA2AFile(body, contentType)
	if err != nil {
		return nil, err
	}
	defer closeExternalA2ASpool(spool)
	reserved, err := s.repo.ReserveUploadBytes(ctx, grant.ID, size, externalA2AMaxRunBytes)
	if err != nil {
		return nil, err
	}
	if !reserved {
		return nil, &ExternalA2AUploadError{Status: http.StatusRequestEntityTooLarge, Message: "this run's upload allowance is used up"}
	}
	run, err := s.agents.runRepo.GetByIDAny(ctx, grant.AgentRunID)
	if err != nil || run == nil {
		_ = s.repo.ReleaseUploadBytes(ctx, grant.ID, size)
		return nil, errors.Join(errExternalA2AUploadToken, err)
	}
	attachment, err := s.storeAgentFile(ctx, run, record, grant.PMTaskID, fileName, contentType, size, spool)
	if err != nil {
		if releaseErr := s.repo.ReleaseUploadBytes(ctx, grant.ID, size); releaseErr != nil {
			slog.ErrorContext(ctx, "release external agent upload allowance", "run_id", grant.AgentRunID, "error", releaseErr)
		}
		return nil, err
	}
	slog.InfoContext(ctx, "external agent file uploaded", "workspace_id", grant.WorkspaceID, "run_id", grant.AgentRunID, "task_id", grant.PMTaskID, "attachment_id", attachment.ID, "size", size)
	return &ExternalA2AUploadResult{ID: attachment.ID, Filename: attachment.FileName, Size: attachment.FileSize}, nil
}

func (s *ExternalA2AService) storeAgentFile(ctx context.Context, run *model.AgentRun, record *model.ExternalA2AAgent, taskID, fileName, contentType string, size int64, body io.Reader) (*model.PMAttachment, error) {
	if s.attachments == nil {
		return nil, fmt.Errorf("file storage is not configured")
	}
	return s.attachments.CreateAgentTaskAttachment(ctx, AgentAttachmentUpload{
		WorkspaceID: run.WorkspaceID, TaskID: taskID, FileName: fileName, ContentType: contentType,
		Size: size, MaxSize: externalA2AMaxFileBytes, UserID: externalA2AAccountableUser(run, record),
		AgentID: record.AgentID, Body: body,
	})
}

// externalA2AAccountableUser is the member recorded as the author of content
// an external agent delivers: whoever started the run, else the connector.
func externalA2AAccountableUser(run *model.AgentRun, record *model.ExternalA2AAgent) string {
	if run != nil && strings.TrimSpace(derefString(run.TriggeredByUserID)) != "" {
		return strings.TrimSpace(*run.TriggeredByUserID)
	}
	return record.CreatedBy
}

// spoolExternalA2AFile copies body to a temporary file, enforcing the size cap
// and checking that the content matches the declared type.
func spoolExternalA2AFile(body io.Reader, contentType string) (*os.File, int64, error) {
	file, err := os.CreateTemp("", "helpin-a2a-*")
	if err != nil {
		return nil, 0, fmt.Errorf("create upload spool: %w", err)
	}
	size, err := io.Copy(file, io.LimitReader(body, externalA2AMaxFileBytes+1))
	if err != nil {
		closeExternalA2ASpool(file)
		if errors.Is(err, externala2a.ErrTooLarge) {
			return nil, 0, externalA2ATooLarge()
		}
		return nil, 0, &ExternalA2AUploadError{Status: http.StatusBadRequest, Message: "could not read the file"}
	}
	if size > externalA2AMaxFileBytes {
		closeExternalA2ASpool(file)
		return nil, 0, externalA2ATooLarge()
	}
	if size == 0 {
		closeExternalA2ASpool(file)
		return nil, 0, &ExternalA2AUploadError{Status: http.StatusBadRequest, Message: "file is empty"}
	}
	head := make([]byte, 512)
	n, _ := file.ReadAt(head, 0)
	if !externalA2AContentMatches(contentType, http.DetectContentType(head[:n])) {
		closeExternalA2ASpool(file)
		return nil, 0, &ExternalA2AUploadError{Status: http.StatusUnsupportedMediaType, Message: "file content does not match its extension"}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		closeExternalA2ASpool(file)
		return nil, 0, fmt.Errorf("rewind upload spool: %w", err)
	}
	return file, size, nil
}

func externalA2ATooLarge() error {
	return &ExternalA2AUploadError{Status: http.StatusRequestEntityTooLarge, Message: fmt.Sprintf("file exceeds the %d MB limit", externalA2AMaxFileBytes>>20)}
}

func closeExternalA2ASpool(file *os.File) {
	if file == nil {
		return
	}
	name := file.Name()
	_ = file.Close()
	if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("remove external agent upload spool", "error", err)
	}
}

// externalA2AContentMatches rejects files whose bytes contradict their
// extension, such as HTML or an executable named .png.
func externalA2AContentMatches(declared, sniffed string) bool {
	sniffed = strings.TrimSpace(strings.Split(sniffed, ";")[0])
	switch declared {
	case "text/plain", "text/csv", "application/json":
		return strings.HasPrefix(sniffed, "text/") && sniffed != "text/html" && sniffed != "text/xml"
	case "video/mp4", "video/webm", "application/zip":
		return sniffed == declared || sniffed == "application/octet-stream"
	default:
		return sniffed == declared
	}
}

func externalA2AContentType(fileName string) (string, bool) {
	contentType, ok := externalA2AFileTypes[strings.ToLower(path.Ext(fileName))]
	return contentType, ok
}

// sanitizeExternalA2AFileName keeps a safe base name for storage keys.
func sanitizeExternalA2AFileName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '"' {
			return -1
		}
		return r
	}, name)
	if runes := []rune(name); len(runes) > 200 {
		ext := path.Ext(name)
		name = string(runes[:200-len([]rune(ext))]) + ext
	}
	if name == "." || name == "" || strings.HasPrefix(name, ".") {
		name = "file" + name
	}
	return name
}
