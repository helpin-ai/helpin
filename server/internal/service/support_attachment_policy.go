package service

import (
	"errors"
	"fmt"
	"sort"
)

// SupportAttachmentRefusedCode is the API error code for a file refused for
// good. Retrying fails the same way, so clients offer no retry.
const SupportAttachmentRefusedCode = "attachment_refused"

// supportUploaderCustomer is the uploader type of files from customers.
const supportUploaderCustomer = "customer"

// supportAttachmentContentTypes are the file types support conversations
// accept from agents. HEIC/HEIF photos are accepted, but most
// browsers cannot display them, so clients offer them as downloads.
var supportAttachmentContentTypes = func() map[string]bool {
	types := map[string]bool{"image/heic": true, "image/heif": true}
	for contentType := range allowedMIMETypes {
		types[contentType] = true
	}
	return types
}()

// supportCustomerRefusedContentTypes are accepted from agents only. An SVG
// can carry script that runs when its file link is opened.
var supportCustomerRefusedContentTypes = map[string]bool{"image/svg+xml": true}

// supportAttachmentTypeAllowed reports whether an uploader of uploaderType may
// send a file of contentType.
func supportAttachmentTypeAllowed(contentType, uploaderType string) bool {
	if uploaderType == supportUploaderCustomer && supportCustomerRefusedContentTypes[contentType] {
		return false
	}
	return supportAttachmentContentTypes[contentType]
}

// SupportAttachmentRejectedError reports a file that support attachment
// rules refuse. Its message is safe to show to the uploader.
type SupportAttachmentRejectedError struct {
	Message string
	// Retryable marks a refusal a fresh upload may get past, such as an
	// upload that never reached storage.
	Retryable bool
}

func (e *SupportAttachmentRejectedError) Error() string { return e.Message }

func rejectSupportAttachment(format string, args ...any) error {
	return &SupportAttachmentRejectedError{Message: fmt.Sprintf(format, args...)}
}

// refuseSupportAttachmentType explains why a file's type is refused.
func refuseSupportAttachmentType(fileName, contentType string) error {
	if contentType == "image/svg+xml" {
		return rejectSupportAttachment("%s can't be sent. Please send it as a PNG or PDF.", fileName)
	}
	return rejectSupportAttachment("%s can't be sent because this file type isn't accepted", fileName)
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

// SupportAttachmentRejectionCode returns SupportAttachmentRefusedCode when err
// refuses a file for good, and "" otherwise.
func SupportAttachmentRejectionCode(err error) string {
	var rejected *SupportAttachmentRejectedError
	if errors.As(err, &rejected) && !rejected.Retryable {
		return SupportAttachmentRefusedCode
	}
	return ""
}

// SupportAttachmentPolicy is the upload rules clients use to filter and
// check files before uploading; the server enforces the same rules.
type SupportAttachmentPolicy struct {
	MaxBytes     int64    `json:"max_bytes"`
	ContentTypes []string `json:"content_types"`
}

// CurrentSupportAttachmentPolicy returns the rules for files agents send.
func CurrentSupportAttachmentPolicy() SupportAttachmentPolicy {
	return supportAttachmentPolicyFor("user")
}

// CustomerSupportAttachmentPolicy returns the rules for files customers send.
func CustomerSupportAttachmentPolicy() SupportAttachmentPolicy {
	return supportAttachmentPolicyFor(supportUploaderCustomer)
}

func supportAttachmentPolicyFor(uploaderType string) SupportAttachmentPolicy {
	types := make([]string, 0, len(supportAttachmentContentTypes))
	for contentType := range supportAttachmentContentTypes {
		if supportAttachmentTypeAllowed(contentType, uploaderType) {
			types = append(types, contentType)
		}
	}
	sort.Strings(types)
	return SupportAttachmentPolicy{MaxBytes: maxSupportFileSize, ContentTypes: types}
}
