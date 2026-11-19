package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// Only call this for attachments from messages already admitted to the transcript.
// Copy the payloads so signing a mailed copy cannot alter the live thread's URLs.
func (s *SupportInboxService) prepareTranscriptDownloadLinks(ctx context.Context, attachments []model.SupportAttachmentPayload) []model.SupportAttachmentPayload {
	prepared := slices.Clone(attachments)
	if s.attachmentService == nil || s.attachmentService.s3Client == nil {
		return prepared
	}
	for i, attachment := range prepared {
		if strings.TrimSpace(attachment.FileKey) == "" {
			continue
		}
		link, err := s.attachmentService.s3Client.GeneratePresignedGetURLWithExpiry(attachment.FileKey, attachment.FileName, 7*24*time.Hour)
		if err != nil {
			slog.WarnContext(ctx, "sign transcript attachment download", "error", err, "attachment_id", attachment.ID)
			continue
		}
		prepared[i].URL = link
	}
	return prepared
}

// Postmark's 10 MB limit includes encoded attachments and the email body. Reserve
// transport overhead and count JSON escaping as well as each file's metadata.
// Linked files remain available when a long transcript reduces the file budget.
func supportTranscriptAttachmentBudget(htmlBody, textBody string, messages []model.SupportMessage) (int64, error) {
	var headers []email.Attachment
	for _, msg := range messages {
		for _, attachment := range msg.Attachments {
			name, contentType := strings.TrimSpace(attachment.FileName), strings.TrimSpace(attachment.FileType)
			if name == "" {
				name = "attachment"
			}
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			headers = append(headers, email.Attachment{Name: name, ContentType: contentType})
		}
	}
	encoded, err := json.Marshal(struct {
		HtmlBody, TextBody string
		Attachments        []email.Attachment
	}{HtmlBody: htmlBody, TextBody: textBody, Attachments: headers})
	if err != nil {
		return 0, fmt.Errorf("calculate transcript attachment budget: %w", err)
	}
	const maxMessageBytes = 10_000_000
	const transportReserve = 64 * 1024
	remaining := maxMessageBytes - transportReserve - int64(len(encoded)) - int64(len(headers))*4
	if remaining <= 0 {
		return 0, nil
	}
	rawBudget := remaining * 3 / 4
	if rawBudget > supportEmailAttachmentMaxTotalRawBytes {
		rawBudget = supportEmailAttachmentMaxTotalRawBytes
	}
	return rawBudget, nil
}

func supportTranscriptDownloadExpiry(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	if query.Get("X-Amz-Signature") == "" {
		return ""
	}
	signedAt, err := time.Parse("20060102T150405Z", query.Get("X-Amz-Date"))
	if err != nil {
		return ""
	}
	seconds, err := strconv.ParseInt(query.Get("X-Amz-Expires"), 10, 64)
	if err != nil || seconds < 1 || seconds > 604800 {
		return ""
	}
	return formatSupportTranscriptTimestamp(signedAt.Add(time.Duration(seconds) * time.Second))
}
