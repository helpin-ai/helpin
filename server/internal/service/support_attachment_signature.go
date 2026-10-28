package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportAttachmentSniffBytes covers every signature below; a tar marker ends at byte 262.
const supportAttachmentSniffBytes = 4096

// supportAttachmentStatusRejected marks an inbound file refused for its content.
// Unlike a failed file, retrying cannot help, so clients offer no retry.
const supportAttachmentStatusRejected = "rejected"

// checkUploadedContent reads the start of a direct-to-storage upload and refuses
// a file whose bytes contradict its type. A refused file is removed.
func (s *SupportAttachmentService) checkUploadedContent(ctx context.Context, attachment *model.SupportAttachment) error {
	if s.s3Client == nil {
		return fmt.Errorf("file storage is not configured")
	}
	body, _, err := s.s3Client.OpenObject(ctx, attachment.StorageKey)
	if err != nil {
		slog.WarnContext(ctx, "open uploaded support attachment failed",
			"workspace_id", attachment.WorkspaceID, "attachment_id", attachment.ID, "error", err)
		return &SupportAttachmentRejectedError{
			Message:   fmt.Sprintf("%s did not finish uploading. Please try again.", attachment.FileName),
			Retryable: true,
		}
	}
	head, err := io.ReadAll(io.LimitReader(body, supportAttachmentSniffBytes))
	if closeErr := body.Close(); closeErr != nil {
		slog.WarnContext(ctx, "close uploaded support attachment failed", "attachment_id", attachment.ID, "error", closeErr)
	}
	if err != nil {
		return fmt.Errorf("read uploaded support attachment: %w", err)
	}
	if supportAttachmentContentMatches(attachment.ContentType, head) {
		return nil
	}
	slog.WarnContext(ctx, "support upload refused: content does not match its type",
		"workspace_id", attachment.WorkspaceID, "attachment_id", attachment.ID,
		"content_type", attachment.ContentType, "uploaded_by_type", attachment.UploadedByType)
	if err := s.s3Client.DeleteObject(ctx, attachment.StorageKey); err != nil {
		slog.WarnContext(ctx, "delete refused support upload failed", "attachment_id", attachment.ID, "error", err)
	}
	if err := s.attachmentRepo.Delete(ctx, attachment.ID); err != nil {
		slog.WarnContext(ctx, "delete refused support attachment record failed", "attachment_id", attachment.ID, "error", err)
	}
	return rejectSupportAttachment("%s can't be sent because its contents don't match its file type", attachment.FileName)
}

// supportAttachmentContentCheckers maps each accepted upload type to a check of
// the file's first bytes. Close relatives pass: a PNG named .jpg is still an
// image, and a QuickTime file named .mp4 is still a video.
var supportAttachmentContentCheckers = map[string]func([]byte) bool{
	"image/jpeg":       isRasterImage,
	"image/png":        isRasterImage,
	"image/gif":        isRasterImage,
	"image/webp":       isRasterImage,
	"image/heic":       isRasterImage,
	"image/heif":       isRasterImage,
	"image/svg+xml":    isSVGImage,
	"video/mp4":        isVideo,
	"video/quicktime":  isVideo,
	"video/webm":       isVideo,
	"video/mpeg":       isVideo,
	"video/x-msvideo":  isVideo,
	"video/x-matroska": isVideo,
	"application/pdf":  isPDF,
	"application/msword": func(head []byte) bool {
		return isOfficeDocument(head) || bytes.HasPrefix(head, []byte(`{\rtf`))
	},
	"application/vnd.ms-excel": isOfficeDocument,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": isOfficeDocument,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       isOfficeDocument,
	"application/zip":  isZipArchive,
	"application/gzip": func(head []byte) bool { return bytes.HasPrefix(head, []byte{0x1f, 0x8b}) },
	"application/x-tar": func(head []byte) bool {
		return len(head) >= 262 && string(head[257:262]) == "ustar"
	},
	"text/plain":       isPlainText,
	"text/csv":         isPlainText,
	"text/markdown":    isPlainText,
	"application/json": isPlainText,
}

// supportAttachmentContentMatches reports whether a file's first bytes fit the
// type it was uploaded as. It refuses renamed files, such as a program or a web
// page named .jpg, which browsers and storage would otherwise trust.
func supportAttachmentContentMatches(declared string, head []byte) bool {
	check, ok := supportAttachmentContentCheckers[declared]
	return ok && len(head) > 0 && check(head)
}

func supportAttachmentKnownContentType(contentType string) bool {
	_, ok := supportAttachmentContentCheckers[contentType]
	return ok
}

func isRasterImage(head []byte) bool {
	switch {
	case bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}),
		bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")),
		bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")),
		isRIFF(head, "WEBP"):
		return true
	}
	for _, brand := range isoMediaBrands(head) {
		switch brand {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "avif", "avis":
			return true
		}
	}
	return false
}

// isSVGImage accepts text that opens an svg element; HTML is not an image.
func isSVGImage(head []byte) bool {
	lower := bytes.ToLower(head)
	return isPlainText(head) && bytes.Contains(lower, []byte("<svg")) && !bytes.Contains(lower, []byte("<html"))
}

func isVideo(head []byte) bool {
	if len(isoMediaBrands(head)) > 0 {
		return true
	}
	if len(head) >= 8 {
		// QuickTime files written before "ftyp" start with one of these atoms.
		switch string(head[4:8]) {
		case "moov", "mdat", "wide", "free", "skip", "pnot":
			return true
		}
	}
	return bytes.HasPrefix(head, []byte{0x1a, 0x45, 0xdf, 0xa3}) || // WebM and Matroska
		bytes.HasPrefix(head, []byte{0x00, 0x00, 0x01, 0xba}) || // MPEG program stream
		bytes.HasPrefix(head, []byte{0x00, 0x00, 0x01, 0xb3}) || // MPEG video stream
		(len(head) > 188 && head[0] == 0x47 && head[188] == 0x47) || // MPEG transport stream
		isRIFF(head, "AVI ")
}

// isPDF allows bytes before the header, as PDF readers do within the first kilobyte.
func isPDF(head []byte) bool {
	return bytes.Contains(head[:min(len(head), 1024)], []byte("%PDF-"))
}

// isOfficeDocument accepts both Office formats: modern files are zip archives,
// while older and password-protected files are compound documents.
func isOfficeDocument(head []byte) bool {
	return isZipArchive(head) || bytes.HasPrefix(head, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
}

func isZipArchive(head []byte) bool {
	return bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06")) ||
		bytes.HasPrefix(head, []byte("PK\x07\x08"))
}

// isPlainText refuses binary content. Text in legacy encodings, such as a CSV
// saved by Excel, passes; so does UTF-16 text marked with a byte-order mark.
func isPlainText(head []byte) bool {
	if bytes.HasPrefix(head, []byte{0xff, 0xfe}) || bytes.HasPrefix(head, []byte{0xfe, 0xff}) {
		return true
	}
	for _, b := range head {
		// Tab, line breaks, form feed and escape appear in real text files.
		if b < 0x20 && b != '\t' && b != '\n' && b != '\v' && b != '\f' && b != '\r' && b != 0x1b {
			return false
		}
	}
	return true
}

func isRIFF(head []byte, form string) bool {
	return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == form
}

// isoMediaBrands returns the major and compatible brands of an ISO base media
// file (MP4, QuickTime, HEIF), or nil when the file has no "ftyp" box.
func isoMediaBrands(head []byte) []string {
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		return nil
	}
	end := min(int(binary.BigEndian.Uint32(head[:4])), len(head))
	brands := []string{string(head[8:12])}
	for offset := 16; offset+4 <= end; offset += 4 {
		brands = append(brands, string(head[offset:offset+4]))
	}
	return brands
}
