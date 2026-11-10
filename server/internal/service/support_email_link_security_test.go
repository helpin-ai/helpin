package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportEmailPhishingURL = "https://login-example.test/verify"

// flaggingSupportLinkScanner reports one URL as phishing and records every lookup.
type flaggingSupportLinkScanner struct {
	malicious string
	urls      []string
	onScan    func()
}

func (s *flaggingSupportLinkScanner) Scan(_ context.Context, normalizedURL string) model.SupportLinkSecurity {
	s.urls = append(s.urls, normalizedURL)
	if s.onScan != nil {
		s.onScan()
	}
	now := time.Now().UTC()
	if normalizedURL == s.malicious {
		return model.SupportLinkSecurity{
			URL: normalizedURL, Status: supportLinkSecurityMalicious, ThreatTypes: []string{"SOCIAL_ENGINEERING"},
			CheckedAt: now, ExpiresAt: now.Add(time.Hour),
		}
	}
	return model.SupportLinkSecurity{URL: normalizedURL, Status: supportLinkSecurityNoMatch, CheckedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
}

// newSupportEmailLinkScanTestService fails the test if an email link is ever fetched.
func newSupportEmailLinkScanTestService(t *testing.T, scanner SupportLinkScanner) *SupportLinkPreviewService {
	t.Helper()
	svc := NewSupportLinkPreviewService("")
	svc.SetLinkScanner(scanner)
	svc.clients = []*http.Client{{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("email link was fetched: %s", req.URL)
		return nil, errors.New("email links must not be fetched")
	})}}
	return svc
}

func TestExtractSupportEmailURLs(t *testing.T) {
	html := `<p>Your account is locked. <a href="https://Login-Example.test/verify?id=1&amp;t=2#top">Verify now</a></p>
<a href="mailto:help@example.com">Email us</a> <a href="javascript:alert(1)">Open</a>
<map><area href="https://map.example.com/a"></map>
<a href="https://login-example.test/verify?id=1&t=2">Verify again</a>`
	text := "Or open https://text.example.com/path. Thanks"

	got := extractSupportEmailURLs(html, text, 10)
	want := []string{"https://login-example.test/verify?id=1&t=2", "https://map.example.com/a", "https://text.example.com/path"}
	if !slices.Equal(got, want) {
		t.Fatalf("urls = %#v, want %#v", got, want)
	}
	if got := extractSupportEmailURLs(html, text, 1); !slices.Equal(got, want[:1]) {
		t.Fatalf("limited urls = %#v, want %#v", got, want[:1])
	}
}

func TestScanEmailLinksChecksOnlyCustomerEmails(t *testing.T) {
	scanner := &flaggingSupportLinkScanner{malicious: supportEmailPhishingURL}
	svc := newSupportEmailLinkScanTestService(t, scanner)
	html := `<a href="` + supportEmailPhishingURL + `">Verify</a>`

	for _, msg := range []*model.SupportMessage{
		{SenderType: "user", HTMLBody: html},
		{SenderType: "customer", IsInternal: true, HTMLBody: html},
	} {
		if got := svc.ScanEmailLinks(context.Background(), msg); got != nil {
			t.Fatalf("scanned %s message: %#v", msg.SenderType, got)
		}
	}
	if len(scanner.urls) != 0 {
		t.Fatalf("unexpected lookups: %#v", scanner.urls)
	}

	got := svc.ScanEmailLinks(context.Background(), &model.SupportMessage{SenderType: "customer", HTMLBody: html})
	if len(got) != 1 || got[0].Status != supportLinkSecurityMalicious || got[0].URL != supportEmailPhishingURL {
		t.Fatalf("verdicts = %#v", got)
	}
}

func TestInboundEmailLinksAreChecked(t *testing.T) {
	workspaceID := "11111111-1111-1111-1111-111111111111"
	html := `<p>Your mailbox is full. <a href="` + supportEmailPhishingURL + `">Fix it now</a></p>`

	tests := []struct {
		name    string
		payload func(t *testing.T, env *emailFallbackTestEnv) model.PostmarkInboundPayload
	}{
		{
			name: "new conversation",
			payload: func(t *testing.T, env *emailFallbackTestEnv) model.PostmarkInboundPayload {
				route := &model.SupportEmailRoute{
					ID: uuid.NewString(), WorkspaceID: workspaceID, RouteKey: "route-links123",
					InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true,
					CreatedByID: "22222222-2222-2222-2222-222222222222",
				}
				if err := env.routeRepo.Create(context.Background(), route); err != nil {
					t.Fatalf("create route: %v", err)
				}
				return model.PostmarkInboundPayload{
					MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
					To: route.InboundAddress, OriginalRecipient: route.InboundAddress, Subject: "Mailbox full",
					TextBody: "Your mailbox is full. Fix it now", HtmlBody: html,
				}
			},
		},
		{
			name: "reply",
			payload: func(t *testing.T, env *emailFallbackTestEnv) model.PostmarkInboundPayload {
				conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: workspaceID, Subject: "Help", Status: "open", CustomerEmail: strPtr("customer@example.com")}
				if err := env.convRepo.Create(context.Background(), conv); err != nil {
					t.Fatal(err)
				}
				return model.PostmarkInboundPayload{
					MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
					To: "conv-" + conv.ID + "@replies.helpin.ai", TextBody: "Your mailbox is full. Fix it now", HtmlBody: html,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
			scanner := &flaggingSupportLinkScanner{malicious: supportEmailPhishingURL}
			// Jev can record a decision on the message while the lookup runs.
			scanner.onScan = func() {
				if err := env.convRepo.DB().Model(&model.SupportMessage{}).
					Where("sender_type = ? AND via_channel = ?", "customer", "email").
					Update("metadata", `{"email_ai_request":false,"email_ai_suppression_reason":"spam"}`).Error; err != nil {
					t.Errorf("record AI decision: %v", err)
				}
			}
			env.service.SetEmailLinkScanner(newSupportEmailLinkScanTestService(t, scanner))
			env.service.runAsync = func(task func()) { task() }

			if err := env.service.ProcessInboundEmail(context.Background(), tt.payload(t, env), ""); err != nil {
				t.Fatalf("process inbound email: %v", err)
			}

			var msg model.SupportMessage
			if err := env.convRepo.DB().Where("sender_type = ? AND via_channel = ?", "customer", "email").First(&msg).Error; err != nil {
				t.Fatalf("load email message: %v", err)
			}
			var metadata struct {
				LinkSecurity             []model.SupportLinkSecurity `json:"link_security"`
				EmailAIRequest           *bool                       `json:"email_ai_request"`
				EmailAISuppressionReason string                      `json:"email_ai_suppression_reason"`
			}
			if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err != nil {
				t.Fatalf("decode metadata %q: %v", msg.Metadata, err)
			}
			if len(metadata.LinkSecurity) != 1 || metadata.LinkSecurity[0].URL != supportEmailPhishingURL || metadata.LinkSecurity[0].Status != supportLinkSecurityMalicious {
				t.Fatalf("link security = %#v", metadata.LinkSecurity)
			}
			if metadata.EmailAIRequest == nil || *metadata.EmailAIRequest || metadata.EmailAISuppressionReason != "spam" {
				t.Fatalf("link check overwrote the AI decision: %s", msg.Metadata)
			}
		})
	}
}

func TestInboundEmailWithoutVerdictsKeepsMetadata(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	scanner := &unknownSupportLinkScanner{}
	env.service.SetEmailLinkScanner(newSupportEmailLinkScanTestService(t, scanner))
	env.service.runAsync = func(task func()) { task() }
	conv := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: "11111111-1111-1111-1111-111111111111", Subject: "Help", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(context.Background(), conv); err != nil {
		t.Fatal(err)
	}
	payload := model.PostmarkInboundPayload{
		MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"},
		To: "conv-" + conv.ID + "@replies.helpin.ai", TextBody: "See https://example.com/docs",
	}
	if err := env.service.ProcessInboundEmail(context.Background(), payload, ""); err != nil {
		t.Fatal(err)
	}
	var msg model.SupportMessage
	if err := env.convRepo.DB().Where("conversation_id = ? AND sender_type = ?", conv.ID, "customer").First(&msg).Error; err != nil {
		t.Fatal(err)
	}
	if scanner.calls != 1 {
		t.Fatalf("lookups = %d, want 1", scanner.calls)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(msg.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if _, ok := metadata[supportLinkSecurityMetadataKey]; ok {
		t.Fatalf("stored verdicts that carry no warning: %s", msg.Metadata)
	}
}

// unknownSupportLinkScanner behaves like a deployment without a Web Risk key.
type unknownSupportLinkScanner struct {
	calls int
}

func (s *unknownSupportLinkScanner) Scan(_ context.Context, normalizedURL string) model.SupportLinkSecurity {
	s.calls++
	now := time.Now().UTC()
	return model.SupportLinkSecurity{URL: normalizedURL, Status: supportLinkSecurityUnknown, CheckedAt: now, ExpiresAt: now.Add(30 * time.Second)}
}
