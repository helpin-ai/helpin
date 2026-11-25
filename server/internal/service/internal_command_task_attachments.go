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

func (s *InternalCommandService) executeReadTaskAttachment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	in, err := s.taskAttachmentRequest(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.AttachmentID) == "" {
		return nil, errCommandInput("attachment_id is required")
	}
	files, err := s.taskRepo.TaskAttachments(ctx, meta.WorkspaceID, []model.PMTask{in.task}, in.AttachmentID, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errCommandNotFound("attachment")
	}
	file := files[0]
	if err := s.signTaskAttachmentDownload(&file); err != nil {
		return nil, err
	}
	const chunkSize = 24 * 1024
	data, err := s.taskFileStore.ReadObjectChunk(ctx, file.StorageKey, int64(in.Offset), chunkSize+1)
	if err != nil {
		return nil, err
	}
	more := len(data) > chunkSize
	if more {
		data = data[:chunkSize]
	}
	result := struct {
		model.TaskAttachment
		Content    string `json:"content"`
		Encoding   string `json:"encoding"`
		Offset     int    `json:"offset"`
		NextOffset int    `json:"next_offset,omitempty"`
	}{TaskAttachment: file, Offset: in.Offset}
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
		result.NextOffset = in.Offset + len(data)
	}
	return mustJSON(result), nil
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
