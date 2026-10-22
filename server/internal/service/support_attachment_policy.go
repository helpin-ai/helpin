package service

import (
	"errors"
	"fmt"
	"sort"
)

// supportAttachmentContentTypes are the file types support conversations
// accept from customers and agents. HEIC/HEIF photos are accepted, but most
// browsers cannot display them, so clients offer them as downloads.
var supportAttachmentContentTypes = func() map[string]bool {
	types := map[string]bool{"image/heic": true, "image/heif": true}
	for contentType := range allowedMIMETypes {
		types[contentType] = true
	}
	return types
}()

// SupportAttachmentRejectedError reports a file that support attachment
// rules refuse. Its message is safe to show to the uploader.
type SupportAttachmentRejectedError struct{ Message string }

func (e *SupportAttachmentRejectedError) Error() string { return e.Message }

func rejectSupportAttachment(format string, args ...any) error {
	return &SupportAttachmentRejectedError{Message: fmt.Sprintf(format, args...)}
}

// IsSupportAttachmentRejected reports whether err is a rule violation and
// returns the message for the uploader.
func IsSupportAttachmentRejected(err error) (string, bool) {
	var rejected *SupportAttachmentRejectedError
	if errors.As(err, &rejected) {
		return rejected.Message, true
	}
	return "", false
}

// SupportAttachmentPolicy is the upload rules clients use to filter and
// check files before uploading; the server enforces the same rules.
type SupportAttachmentPolicy struct {
	MaxBytes     int64    `json:"max_bytes"`
	ContentTypes []string `json:"content_types"`
}

// CurrentSupportAttachmentPolicy returns the support attachment rules.
func CurrentSupportAttachmentPolicy() SupportAttachmentPolicy {
	types := make([]string, 0, len(supportAttachmentContentTypes))
	for contentType := range supportAttachmentContentTypes {
		types = append(types, contentType)
	}
	sort.Strings(types)
	return SupportAttachmentPolicy{MaxBytes: maxSupportFileSize, ContentTypes: types}
}
