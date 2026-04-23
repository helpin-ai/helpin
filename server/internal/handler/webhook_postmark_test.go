package handler

import (
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

func TestPostmarkInbound_InvalidJSONReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPostmarkInbound_AuthorizedWithoutServiceReturnsOK(t *testing.T) {
	h := NewPostmarkInboundHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/postmark/inbound", strings.NewReader(`{"MailboxHash":"conv-123"}`))
	req.SetBasicAuth("postmark", "secret")
	rec := httptest.NewRecorder()

	h.PostmarkInbound(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
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

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
