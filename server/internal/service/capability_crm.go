package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GoogleOAuthCallbackPath is the API path Google returns to after a Gmail and
// Calendar connection; the OAuth client's redirect URI must end with it.
const GoogleOAuthCallbackPath = "/api/crm/email/oauth/callback"

// MeetingCaptureWebhookURL is where a capture provider must deliver events
// for this server.
func MeetingCaptureWebhookURL(appBaseURL, provider string) string {
	return strings.TrimRight(strings.TrimSpace(appBaseURL), "/") + "/api/webhooks/meeting-capture/" + provider
}

// GoogleOAuthRedirectURL is the redirect URI to register with Google for appBaseURL.
func GoogleOAuthRedirectURL(appBaseURL string) string {
	return strings.TrimRight(strings.TrimSpace(appBaseURL), "/") + GoogleOAuthCallbackPath
}

// GoogleOAuthRedirectProblem explains why Google cannot return people to this
// server through redirectURL, or returns "" when it can: the redirect must be
// APP_BASE_URL plus the callback path, and Google accepts plain http only for
// localhost.
func GoogleOAuthRedirectProblem(appBaseURL, redirectURL string) string {
	expected := GoogleOAuthRedirectURL(appBaseURL)
	current := strings.TrimSpace(redirectURL)
	if current == "" {
		current = "not set"
	}
	if current != expected {
		return fmt.Sprintf("GMAIL_OAUTH_REDIRECT_URL must be %s so Google returns people to this server (currently %s).", expected, current)
	}
	parsed, err := url.Parse(expected)
	if err != nil {
		return "APP_BASE_URL is not a valid address, so Google cannot return people to this server."
	}
	host := strings.ToLower(parsed.Hostname())
	loopback := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if !strings.EqualFold(parsed.Scheme, "https") && !loopback {
		return fmt.Sprintf("Google accepts http redirect addresses only for localhost. Set APP_BASE_URL to an https address (currently %s).", strings.TrimSpace(appBaseURL))
	}
	return ""
}

// MeetingProviderTitle is the display name of a capture provider key.
func MeetingProviderTitle(provider string) string {
	switch provider {
	case model.CRMMeetingProviderVexa:
		return "Vexa"
	case model.CRMMeetingProviderRecall:
		return "Recall"
	default:
		return provider
	}
}

func meetingCaptureEnv(provider string) (detail, fix string) {
	if provider == model.CRMMeetingProviderVexa {
		return "VEXA_API_KEY and VEXA_WEBHOOK_SECRET are not both set.",
			"Set VEXA_API_KEY and VEXA_WEBHOOK_SECRET on the server (and VEXA_BASE_URL for a self-hosted Vexa)"
	}
	return "RECALL_API_KEY and RECALL_WEBHOOK_SECRET are not both set.",
		"Set RECALL_API_KEY and RECALL_WEBHOOK_SECRET on the server, or set CRM_MEETING_CAPTURE_PROVIDER=vexa with VEXA_API_KEY and VEXA_WEBHOOK_SECRET"
}

// meetingCapture reports the capture provider for a workspace, or for the
// whole instance when workspaceID is empty. Configuration alone is never
// ready: a provider webhook must have arrived.
func (s *CapabilityService) meetingCapture(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeyMeetingCapture
	if !s.moduleEnabled(model.ModuleCRM) {
		return capability(key, model.CapabilityUnavailable, "CRM is not enabled on this server.", nil), nil
	}
	provider := s.cfg.MeetingCaptureProvider
	name := MeetingProviderTitle(provider)
	webhook := s.cfg.MeetingCaptureWebhookURL
	if !s.cfg.MeetingCaptureConfigured {
		missing, fix := meetingCaptureEnv(provider)
		return capability(key, model.CapabilityNeedsSetup, "Meeting capture uses "+name+", but "+missing, serverAction(fix)), nil
	}
	if problem := s.cfg.MeetingCaptureReachabilityProblem; problem != "" {
		return capability(key, model.CapabilityNeedsSetup, "Meeting capture with "+name+" is configured. "+problem,
			serverAction("Set APP_BASE_URL to a public https address, then register "+webhook+" with "+name)), nil
	}
	delivered, err := s.evidence.HasMeetingProviderEvent(ctx, workspaceID, provider)
	if err != nil {
		return model.Capability{}, err
	}
	if delivered {
		return capability(key, model.CapabilityReady, name+" has delivered meeting events to this server.", nil), nil
	}
	return capability(key, model.CapabilityUnableToVerify, "Meeting capture with "+name+" is configured; it is confirmed when the first meeting event arrives.",
		serverAction("Register "+webhook+" as the "+name+" webhook, then capture a meeting")), nil
}

// googleWorkspace reports the Google OAuth client behind Gmail and Google
// Calendar connections, for a workspace or the instance (empty workspaceID).
func (s *CapabilityService) googleWorkspace(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeyGoogleWorkspace
	if !s.moduleEnabled(model.ModuleCRM) {
		return capability(key, model.CapabilityUnavailable, "CRM is not enabled on this server.", nil), nil
	}
	redirect := s.cfg.GoogleOAuthRedirectURL
	if !s.cfg.GoogleOAuthConfigured {
		return capability(key, model.CapabilityNeedsSetup, "Connecting Gmail and Google Calendar needs a Google OAuth client; GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET are not set.",
			serverAction("Create a Google OAuth client with redirect URI "+redirect+", then set GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET on the server")), nil
	}
	if problem := s.cfg.GoogleOAuthProblem; problem != "" {
		return capability(key, model.CapabilityNeedsSetup, problem,
			serverAction("Leave GMAIL_OAUTH_REDIRECT_URL empty (or set it to "+redirect+") and add "+redirect+" to the Google OAuth client's redirect URIs")), nil
	}
	connected, err := s.evidence.HasConnectedGoogleAccount(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if connected {
		detail := "A Google account is connected in this workspace."
		if workspaceID == "" {
			detail = "A Google account is connected on this server."
		}
		return capability(key, model.CapabilityReady, detail, nil), nil
	}
	const detail = "The Google OAuth client is configured; it is confirmed when someone connects a Google account."
	if workspaceID == "" {
		return capability(key, model.CapabilityUnableToVerify, detail, serverAction("Connect a Google account in Settings → Email & calendar")), nil
	}
	return capability(key, model.CapabilityUnableToVerify, detail, settingsAction("Connect a Google account", "settings/crm-email")), nil
}
