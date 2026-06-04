package email

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSendEmailWithHeadersAndAttachmentsSerializesPostmarkAttachments(t *testing.T) {
	client := NewClient("postmark-token", "noreply@example.com")

	var captured struct {
		Attachments []Attachment `json:"Attachments"`
	}
	client.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-attachment-1",
					"SubmittedAt": "2026-06-04T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	_, err := client.SendEmailWithHeadersAndAttachments(
		"Support <support@example.com>",
		"customer@example.com",
		"Attached report",
		"<p>See attached.</p>",
		"See attached.",
		"reply@example.com",
		nil,
		[]Attachment{
			{
				Name:        "report.pdf",
				ContentType: "application/pdf",
				Content:     "cGRmLWJ5dGVz",
			},
		},
	)
	if err != nil {
		t.Fatalf("send email: %v", err)
	}
	if len(captured.Attachments) != 1 {
		t.Fatalf("attachments len = %d, want 1", len(captured.Attachments))
	}
	if captured.Attachments[0].Name != "report.pdf" {
		t.Fatalf("attachment name = %q", captured.Attachments[0].Name)
	}
	if captured.Attachments[0].Content != "cGRmLWJ5dGVz" {
		t.Fatalf("attachment content = %q", captured.Attachments[0].Content)
	}
	if captured.Attachments[0].ContentType != "application/pdf" {
		t.Fatalf("attachment content type = %q", captured.Attachments[0].ContentType)
	}
}
