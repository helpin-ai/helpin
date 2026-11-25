package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type taskFileStore interface {
	ReadObjectChunk(context.Context, string, int64, int) ([]byte, error)
	GeneratePresignedGetURL(string, string) (string, error)
}

// SetTaskFileStore wires the same attachment storage used by the web UI.
func (s *InternalCommandService) SetTaskFileStore(store taskFileStore) {
	s.taskFileStore = store
}

type taskAttachmentRequest struct {
	task         model.PMTask
	TaskID       string `json:"task_id"`
	AttachmentID string `json:"attachment_id"`
	Offset       int    `json:"offset"`
	Raw          bool   `json:"raw"`
	Question     string `json:"question"`
}

// Resolve the task under the same actor, workspace and run-target checks as get_task.
func (s *InternalCommandService) taskAttachmentRequest(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (taskAttachmentRequest, error) {
	var in taskAttachmentRequest
	if err := json.Unmarshal(input, &in); err != nil || in.Offset < 0 {
		return in, errCommandInput("invalid task attachment arguments")
	}
	if searchTaskKeyPattern.MatchString(strings.ToUpper(strings.TrimSpace(in.TaskID))) {
		taskID, err := s.resolveCommandTaskKey(ctx, meta, in.TaskID)
		if err != nil {
			return in, err
		}
		in.TaskID = taskID
	}
	detail, err := s.readCommandTask(ctx, meta, in.TaskID)
	if err != nil {
		return in, err
	}
	in.task = detail.Task
	return in, nil
}

func (s *InternalCommandService) executeListTaskAttachments(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	in, err := s.taskAttachmentRequest(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	const pageSize = 50
	files, err := s.taskRepo.TaskAttachments(ctx, meta.WorkspaceID, []model.PMTask{in.task}, "", in.Offset, pageSize+1)
	if err != nil {
		return nil, err
	}
	result := struct {
		Attachments []model.TaskAttachment `json:"attachments"`
		NextOffset  int                    `json:"next_offset,omitempty"`
	}{Attachments: files}
	if len(files) > pageSize {
		result.Attachments = files[:pageSize]
		result.NextOffset = in.Offset + pageSize
	}
	return mustJSON(result), nil
}

// resolveTaskAttachment verifies both the caller's task scope and the file's relationship to the task.
func (s *InternalCommandService) resolveTaskAttachment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (taskAttachmentRequest, model.TaskAttachment, error) {
	in, err := s.taskAttachmentRequest(ctx, meta, input)
	if err != nil {
		return in, model.TaskAttachment{}, err
	}
	if strings.TrimSpace(in.AttachmentID) == "" {
		return in, model.TaskAttachment{}, errCommandInput("attachment_id is required")
	}
	files, err := s.taskRepo.TaskAttachments(ctx, meta.WorkspaceID, []model.PMTask{in.task}, in.AttachmentID, 0, 1)
	if err != nil {
		return in, model.TaskAttachment{}, err
	}
	if len(files) == 0 {
		return in, model.TaskAttachment{}, errCommandNotFound("attachment")
	}
	return in, files[0], nil
}

const taskAttachmentChunkSize = 24 * 1024
const maxTaskDocumentBytes = 8 << 20
const taskDOCXType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

type taskAttachmentReadResult struct {
	model.TaskAttachment
	Content        string `json:"content"`
	Encoding       string `json:"encoding,omitempty"`
	Representation string `json:"representation,omitempty"`
	Notice         string `json:"notice,omitempty"`
	Offset         int    `json:"offset"`
	NextOffset     int    `json:"next_offset,omitempty"`
}

func (s *InternalCommandService) executeReadTaskAttachment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	in, file, err := s.resolveTaskAttachment(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	if err := s.signTaskAttachmentDownload(&file); err != nil {
		return nil, err
	}
	result := taskAttachmentReadResult{TaskAttachment: file, Offset: in.Offset}
	docType, _ := normalizeKnowledgeFileContentType(file.ContentType, file.FileName)
	if !in.Raw && (docType == "application/pdf" || docType == taskDOCXType) {
		if err := s.readTaskDocument(ctx, docType, &result); err != nil {
			return nil, err
		}
	} else {
		data, err := s.taskFileStore.ReadObjectChunk(ctx, file.StorageKey, int64(in.Offset), taskAttachmentChunkSize+1)
		if err != nil {
			return nil, err
		}
		result.setChunk(data)
	}
	return mustJSON(result), nil
}

func (s *InternalCommandService) readTaskDocument(ctx context.Context, docType string, result *taskAttachmentReadResult) error {
	result.Representation = "extracted_text"
	const oversized = "Document exceeds the 8 MiB extraction limit. Use download_url or request raw bytes."
	if result.FileSize > maxTaskDocumentBytes {
		result.Notice = oversized
		return nil
	}
	data, err := s.taskFileStore.ReadObjectChunk(ctx, result.StorageKey, 0, maxTaskDocumentBytes+1)
	if err != nil {
		return err
	}
	if len(data) > maxTaskDocumentBytes {
		result.Notice = oversized
		return nil
	}
	var text string
	if docType == "application/pdf" {
		text, err = extractPDFPages(ctx, data, 500, maxTaskDocumentBytes)
	} else {
		text, err = extractDOCXText(data)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		result.Notice = "Text extraction was unsuccessful. Use download_url or request raw bytes."
		if docType == "application/pdf" {
			result.Notice += " PDF extraction supports up to 500 pages and 8 MiB of text."
		}
		return nil
	}
	text = strings.TrimSpace(strings.ReplaceAll(strings.ToValidUTF8(text, "�"), "\x00", "�"))
	if text == "" {
		result.Notice = "The document contains no extractable text."
		if docType == "application/pdf" {
			result.Notice += " Use analyze_task_attachment to inspect the scanned pages."
		}
		return nil
	}
	if result.Offset > len(text) || (result.Offset < len(text) && !utf8.RuneStart(text[result.Offset])) {
		return errCommandInput("offset must be a valid byte boundary in the extracted text")
	}
	start := result.Offset
	result.setChunk([]byte(text[start : start+min(taskAttachmentChunkSize+1, len(text)-start)]))
	return nil
}

func (result *taskAttachmentReadResult) setChunk(data []byte) {
	more := len(data) > taskAttachmentChunkSize
	if more {
		data = data[:taskAttachmentChunkSize]
	}
	// Leave a split UTF-8 character for the next chunk.
	end := len(data)
	if more && end > 0 {
		i := end - 1
		for i > 0 && end-i < utf8.UTFMax && !utf8.RuneStart(data[i]) {
			i--
		}
		if !utf8.FullRune(data[i:]) {
			end = i
		}
	}
	if utf8.Valid(data[:end]) && bytes.IndexByte(data[:end], 0) < 0 {
		data = data[:end]
		result.Encoding = "utf-8"
		result.Content = string(data)
	} else {
		result.Encoding = "base64"
		result.Content = base64.StdEncoding.EncodeToString(data)
	}
	if more {
		result.NextOffset = result.Offset + len(data)
	}
}

func (s *InternalCommandService) signTaskAttachmentDownload(file *model.TaskAttachment) error {
	if s.taskFileStore == nil {
		return fmt.Errorf("file storage is unavailable")
	}
	url, err := s.taskFileStore.GeneratePresignedGetURL(file.StorageKey, file.FileName)
	if err != nil {
		return fmt.Errorf("sign task attachment download: %w", err)
	}
	file.DownloadURL = url
	return nil
}
