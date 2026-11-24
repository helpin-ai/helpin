package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type mcpTaskFileStore interface {
	ReadObjectChunk(context.Context, string, int64, int) ([]byte, error)
}

func (s *MCPService) SetTaskAttachments(files *repository.PMAttachmentRepository, store mcpTaskFileStore) {
	s.taskFiles, s.taskFileStore = files, store
}

func (s *MCPService) executeTaskAttachment(ctx context.Context, principal *model.MCPPrincipal, name string, args json.RawMessage) (*MCPToolResult, error) {
	var in struct {
		TaskID       string `json:"task_id"`
		AttachmentID string `json:"attachment_id"`
		Offset       int    `json:"offset"`
	}
	if err := decodeMCPArguments(args, &in); err != nil {
		return nil, err
	}
	task, err := s.accessibleMCPTask(ctx, principal, in.TaskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrMCPNotFound
	}
	if s.taskFiles == nil {
		return nil, fmt.Errorf("task attachments are unavailable")
	}
	const pageSize = 50
	read := name == "read_task_attachment"
	limit, start := pageSize+1, in.Offset
	if read {
		if in.AttachmentID == "" {
			return nil, ErrMCPInvalidArguments
		}
		limit, start = 1, 0
	}
	files, err := s.taskFiles.UploadedTaskFiles(ctx, principal.WorkspaceID, task.Task.ID, in.AttachmentID, start, limit)
	if err != nil {
		return nil, err
	}
	if !read {
		items := make([]map[string]any, 0, len(files))
		result := map[string]any{}
		if len(files) > pageSize {
			files = files[:pageSize]
			result["next_offset"] = start + pageSize
		}
		for _, f := range files {
			items = append(items, map[string]any{"attachment_id": f.ID, "file_name": f.FileName, "file_size": f.FileSize, "content_type": f.ContentType, "entity_type": f.EntityType, "entity_id": f.EntityID})
		}
		result["attachments"] = items
		return &MCPToolResult{Summary: "Task attachments loaded.", Data: result}, nil
	}
	if len(files) == 0 {
		return nil, ErrMCPNotFound
	}
	if s.taskFileStore == nil {
		return nil, fmt.Errorf("file storage is unavailable")
	}
	file := files[0]
	const chunkSize = 24 * 1024
	data, err := s.taskFileStore.ReadObjectChunk(ctx, file.StorageKey, int64(in.Offset), chunkSize+1)
	if err != nil {
		return nil, err
	}
	more := len(data) > chunkSize
	if more {
		data = data[:chunkSize]
	}
	result := map[string]any{"file_name": file.FileName, "content_type": file.ContentType, "offset": in.Offset, "attachment_id": file.ID}
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
		result["encoding"] = "utf-8"
		result["content"] = string(data)
	} else {
		result["encoding"] = "base64"
		result["content"] = base64.StdEncoding.EncodeToString(data)
	}
	if more {
		result["next_offset"] = in.Offset + len(data)
	}
	return &MCPToolResult{Summary: "Task attachment content loaded.", Data: result}, nil
}
