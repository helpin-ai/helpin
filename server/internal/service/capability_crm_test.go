package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMeetingCaptureCapability(t *testing.T) {
	const publicURL = "https://helpin.example.com"
	webhook := MeetingCaptureWebhookURL(publicURL, "recall")
	configured := CapabilityConfig{MeetingCaptureProvider: "recall", MeetingCaptureConfigured: true, MeetingCaptureWebhookURL: webhook}
	tests := []struct {
		name       string
		cfg        CapabilityConfig
		evidence   *fakeCapabilityEvidence
		want       string
		detailHas  string
		actionKind string
	}{
		{"crm disabled", CapabilityConfig{Modules: []model.ModuleID{model.ModulePM}, MeetingCaptureProvider: "recall", MeetingCaptureConfigured: true},
			&fakeCapabilityEvidence{meetingEvent: true}, model.CapabilityUnavailable, "CRM is not enabled", ""},
		{"recall not configured", CapabilityConfig{MeetingCaptureProvider: "recall"}, &fakeCapabilityEvidence{},
			model.CapabilityNeedsSetup, "RECALL_API_KEY and RECALL_WEBHOOK_SECRET", model.CapabilityActionServerConfig},
		{"vexa not configured", CapabilityConfig{MeetingCaptureProvider: "vexa"}, &fakeCapabilityEvidence{},
			model.CapabilityNeedsSetup, "Vexa, but VEXA_API_KEY and VEXA_WEBHOOK_SECRET", model.CapabilityActionServerConfig},
		{"webhooks cannot arrive", CapabilityConfig{MeetingCaptureProvider: "recall", MeetingCaptureConfigured: true,
			MeetingCaptureReachabilityProblem: PublicBaseURLBlockedReason("Recall", "http://localhost:8085")}, &fakeCapabilityEvidence{meetingEvent: true},
			model.CapabilityNeedsSetup, "Recall must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://localhost:8085)", model.CapabilityActionServerConfig},
		{"configured, no event yet", configured, &fakeCapabilityEvidence{}, model.CapabilityUnableToVerify, "configured", model.CapabilityActionServerConfig},
		{"event delivered", configured, &fakeCapabilityEvidence{meetingEvent: true}, model.CapabilityReady, "Recall has delivered", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.Modules == nil {
				tt.cfg.Modules = allModules()
			}
			svc := newCapabilityService(tt.cfg, tt.evidence, fakeCapabilitySetup{}, fakeCapabilityAI{})
			assertCRMCapability(t, svc, model.CapabilityKeyMeetingCapture, tt.want, tt.detailHas, tt.actionKind)
		})
	}
	configured.Modules = allModules()
	svc := newCapabilityService(configured, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, fakeCapabilityAI{})
	response, err := svc.Workspace(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	if got := capabilityByKey(t, response, model.CapabilityKeyMeetingCapture); got.Action == nil || !strings.Contains(got.Action.Label, webhook) {
		t.Fatalf("action does not name the webhook address: %+v", got.Action)
	}
}

func TestGoogleWorkspaceCapability(t *testing.T) {
	const publicURL = "https://helpin.example.com"
	redirect := GoogleOAuthRedirectURL(publicURL)
	configured := CapabilityConfig{GoogleOAuthConfigured: true, GoogleOAuthRedirectURL: redirect}
	tests := []struct {
		name          string
		cfg           CapabilityConfig
		evidence      *fakeCapabilityEvidence
		want          string
		detailHas     string
		workspaceKind string
	}{
		{"crm disabled", CapabilityConfig{Modules: []model.ModuleID{model.ModuleSupport}, GoogleOAuthConfigured: true},
			&fakeCapabilityEvidence{google: true}, model.CapabilityUnavailable, "CRM is not enabled", ""},
		{"client missing", CapabilityConfig{GoogleOAuthRedirectURL: redirect}, &fakeCapabilityEvidence{},
			model.CapabilityNeedsSetup, "GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET", model.CapabilityActionServerConfig},
		{"redirect elsewhere", CapabilityConfig{GoogleOAuthConfigured: true, GoogleOAuthRedirectURL: redirect,
			GoogleOAuthProblem: GoogleOAuthRedirectProblem(publicURL, "https://old.example.com/api/crm/email/oauth/callback")},
			&fakeCapabilityEvidence{google: true}, model.CapabilityNeedsSetup, "GMAIL_OAUTH_REDIRECT_URL must be " + redirect, model.CapabilityActionServerConfig},
		{"configured, nobody connected", configured, &fakeCapabilityEvidence{}, model.CapabilityUnableToVerify, "configured", model.CapabilityActionOpenSettings},
		{"account connected", configured, &fakeCapabilityEvidence{google: true}, model.CapabilityReady, "connected", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.Modules == nil {
				tt.cfg.Modules = allModules()
			}
			svc := newCapabilityService(tt.cfg, tt.evidence, fakeCapabilitySetup{}, fakeCapabilityAI{})
			assertCRMCapability(t, svc, model.CapabilityKeyGoogleWorkspace, tt.want, tt.detailHas, tt.workspaceKind)
		})
	}
}

// assertCRMCapability checks the workspace view (status, detail, action kind)
// and that the instance view reports the same status.
func assertCRMCapability(t *testing.T, svc *CapabilityService, key, want, detailHas, actionKind string) {
	t.Helper()
	workspace, err := svc.Workspace(context.Background(), "ws")
	if err != nil {
		t.Fatal(err)
	}
	got := capabilityByKey(t, workspace, key)
	if got.Status != want || !strings.Contains(got.Detail, detailHas) {
		t.Fatalf("status=%s detail=%q, want %s containing %q", got.Status, got.Detail, want, detailHas)
	}
	if (got.Action == nil) != (actionKind == "") || (got.Action != nil && got.Action.Kind != actionKind) {
		t.Fatalf("action=%+v, want %q", got.Action, actionKind)
	}
	instance, err := svc.Instance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status := capabilityByKey(t, instance, key).Status; status != want {
		t.Fatalf("instance status=%s, want %s", status, want)
	}
}

func TestGoogleOAuthRedirectProblem(t *testing.T) {
	tests := []struct {
		name, base, redirect, want string
	}{
		{"matches public https", "https://helpin.example.com", "https://helpin.example.com/api/crm/email/oauth/callback", ""},
		{"trailing slash on base", "https://helpin.example.com/", "https://helpin.example.com/api/crm/email/oauth/callback", ""},
		{"localhost http is allowed", "http://localhost:8085", "http://localhost:8085/api/crm/email/oauth/callback", ""},
		{"loopback ip http is allowed", "http://127.0.0.1:8085", "http://127.0.0.1:8085/api/crm/email/oauth/callback", ""},
		{"other host", "https://helpin.example.com", "https://api.example.com/api/crm/email/oauth/callback", "must be https://helpin.example.com/api/crm/email/oauth/callback"},
		{"not set", "https://helpin.example.com", "", "(currently not set)"},
		{"lan http", "http://192.168.1.20:8085", "http://192.168.1.20:8085/api/crm/email/oauth/callback", "only for localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GoogleOAuthRedirectProblem(tt.base, tt.redirect)
			if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
				t.Fatalf("GoogleOAuthRedirectProblem() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicBaseURLBlockedReasonNamesTheSender(t *testing.T) {
	if got := PublicBaseURLBlockedReason("Vexa", "http://helpin.lan"); !strings.HasPrefix(got, "Vexa must reach this server") {
		t.Fatal(got)
	}
	if got := PublicBaseURLBlockedReason("Vexa", "https://helpin.example.com"); got != "" {
		t.Fatal(got)
	}
	if GitHubAppBaseURLBlockedReason("http://localhost") != PublicBaseURLBlockedReason("GitHub", "http://localhost") {
		t.Fatal("GitHub reason must reuse the shared rule")
	}
}
