package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostmarkInbound_AuthFailure(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
		t.Fatalf("body = %q, want unauthorized error", body)
	}
}

func TestPostmarkInbound_InvalidJSONIsNotAcknowledged(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPostmarkInbound_UnavailableServiceIsRetryable(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{"MailboxHash":"conv-123"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestPostmarkOpen_AuthFailure(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/open", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PostmarkOpen(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
		t.Fatalf("body = %q, want unauthorized error", body)
	}
}

func TestPostmarkOpen_InvalidJSONReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/open", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkOpen(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkOpen_AuthorizedWithoutServiceReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/open", strings.NewReader(`{"RecordType":"Open","MessageID":"pm-1"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkOpen(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkDelivery_AuthFailure(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/delivery", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PostmarkDelivery(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
		t.Fatalf("body = %q, want unauthorized error", body)
	}
}

func TestPostmarkDelivery_InvalidJSONReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/delivery", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkDelivery(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkDelivery_AuthorizedWithoutServiceReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/delivery", strings.NewReader(`{"RecordType":"Delivery","MessageID":"pm-1"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkDelivery(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkBounce_AuthFailure(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/bounce", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PostmarkBounce(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
		t.Fatalf("body = %q, want unauthorized error", body)
	}
}

func TestPostmarkBounce_InvalidJSONReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/bounce", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkBounce(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkBounce_AuthorizedWithoutServiceReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/bounce", strings.NewReader(`{"RecordType":"Bounce","MessageID":"pm-1"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkBounce(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkSpamComplaint_AuthFailure(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/spam-complaint", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PostmarkSpamComplaint(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"error":"unauthorized"`) {
		t.Fatalf("body = %q, want unauthorized error", body)
	}
}

func TestPostmarkSpamComplaint_InvalidJSONReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/spam-complaint", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkSpamComplaint(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkSpamComplaint_AuthorizedWithoutServiceReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/spam-complaint", strings.NewReader(`{"RecordType":"SpamComplaint","MessageID":"pm-1"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkSpamComplaint(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkInbound_AllowsAnyConfiguredSecret(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "reply-secret", "route-secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{"MailboxHash":"route-123"}`))
	req.SetBasicAuth("postmark", "route-secret")
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

type retryingPostmarkProcessor struct {
	*service.EmailFallbackService
	nextError error
	calls     int
}

func (p *retryingPostmarkProcessor) AcceptInboundEmail(_ context.Context, _ model.PostmarkInboundPayload, _ string) error {
	p.calls++
	err := p.nextError
	p.nextError = nil
	return err
}

func TestPostmarkInboundRetryableDispatchHTTPStatus(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want int
	}{
		{"dispatch failure", fmt.Errorf("dispatch failed: %w", service.ErrInboundEmailAIDispatchRetry), http.StatusServiceUnavailable},
		{"receipt persistence failure", errors.New("database unavailable"), http.StatusServiceUnavailable},
		{"success", nil, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			processor := &retryingPostmarkProcessor{nextError: tt.err}
			h := NewPostmarkInboundHandler(nil, "secret")
			h.emailFallbackService = processor
			request := func() *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{"MessageID":"saved-customer-message"}`))
				req.SetBasicAuth("postmark", "secret")
				rec := httptest.NewRecorder()
				h.PostmarkInbound(rec, req)
				return rec
			}
			rec := request()
			if rec.Code != tt.want {
				t.Fatalf("first status = %d, want %d", rec.Code, tt.want)
			}
			rec = request()
			if rec.Code != http.StatusOK {
				t.Fatalf("retry status = %d, want 200", rec.Code)
			}
			if processor.calls != 2 {
				t.Fatalf("processing calls = %d", processor.calls)
			}
		})
	}
}
