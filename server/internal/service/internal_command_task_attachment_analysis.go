package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var errIncompleteTaskAttachmentAnswer = errors.New("attachment analysis did not return a complete answer")

// Both task vision routes use OpenRouter's normalized finish reasons.
func validateTaskAttachmentAnswer(response *llm.ChatResponse) error {
	if response.FinishReason != "stop" || strings.TrimSpace(response.Content) == "" {
		return fmt.Errorf("%w (finish_reason=%q)", errIncompleteTaskAttachmentAnswer, response.FinishReason)
	}
	return nil
}

func (s *InternalCommandService) executeAnalyzeTaskAttachment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	in, file, err := s.resolveTaskAttachment(ctx, meta, input)
	if err != nil {
		return nil, err
	}
	question := strings.TrimSpace(in.Question)
	if question == "" || utf8.RuneCountInString(question) > 2000 {
		return nil, errCommandInput("question must contain 1 to 2000 characters")
	}
	kind := strings.ToLower(strings.TrimSpace(file.ContentType))
	if docType, _ := normalizeKnowledgeFileContentType(kind, file.FileName); docType == "application/pdf" {
		kind = docType
	}
	if !isDockChatMediaType(kind) && kind != "application/pdf" {
		return nil, errCommandInput("visual analysis supports images, videos and PDFs; use read_task_attachment for text documents")
	}
	if file.FileSize > 20<<20 {
		return nil, errCommandInput("visual analysis supports files up to 20 MiB; use the download_url from read_task_attachment")
	}
	if s.attachmentMediaProvider == nil {
		return nil, fmt.Errorf("task attachment analysis is not configured")
	}
	if err := s.signTaskAttachmentDownload(&file); err != nil {
		return nil, err
	}
	media := mediaAttachment{ID: file.AttachmentID, FileName: file.FileName, FileType: kind, FileSize: file.FileSize, URL: file.DownloadURL}
	analysis, err := analyzeAttachmentMedia(ctx, s.attachmentMediaProvider, meta.WorkspaceID, BillingFeatureAskChat, aipolicy.ActionTaskAttachmentAnalysis, question, []mediaAttachment{media})
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"task_id": file.TaskID, "attachment_id": file.AttachmentID, "file_name": file.FileName, "analysis": analysis}), nil
}
