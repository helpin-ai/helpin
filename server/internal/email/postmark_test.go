package email

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestSendEmailContextCancelsInFlightRequest(t *testing.T) {
	client := NewClient("test-only", "support@example.com")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	client.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(entered)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(time.Second):
			return nil, errors.New("request did not receive cancellation")
		}
	})})
	done := make(chan error, 1)
	go func() {
		_, err := client.SendEmailWithHeadersAttachmentsAndOptionsContext(ctx, "support@example.com", "customer@example.com", "Subject", "Body", "Body", "", nil, nil, SendEmailOptions{})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("send did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("send error = %v, want cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("send did not stop")
	}
}
