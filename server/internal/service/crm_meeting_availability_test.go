package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMeetingCaptureAvailabilityFollowsSelectedProvider(t *testing.T) {
	recall := meetingcapture.NewRecallProvider(meetingcapture.RecallConfig{APIKey: "key"})
	vexa := meetingcapture.NewVexaProvider(meetingcapture.VexaConfig{APIKey: "key", WebhookSecret: "secret"})
	svc := NewCRMMeetingService(nil, nil, nil, recall, vexa)

	if provider, configured := svc.CaptureAvailability(); provider != model.CRMMeetingProviderRecall || configured {
		t.Fatalf("recall without a webhook secret: provider=%q configured=%v", provider, configured)
	}
	if _, err := svc.provider(model.CRMMeetingProviderRecall, model.CRMMeetingPlatformGoogleMeet); !IsMeetingCaptureNotConfigured(err) {
		t.Fatalf("unconfigured provider error = %v", err)
	}
	svc.SetCaptureProvider(model.CRMMeetingProviderVexa)
	if provider, configured := svc.CaptureAvailability(); provider != model.CRMMeetingProviderVexa || !configured {
		t.Fatalf("vexa with credentials: provider=%q configured=%v", provider, configured)
	}
}

func TestDescribeCaptureAddsAvailabilityToSettings(t *testing.T) {
	svc := NewCRMMeetingService(nil, nil, nil, meetingcapture.NewRecallProvider(meetingcapture.RecallConfig{}))
	settings := &model.CRMMeetingSettings{Enabled: true}
	svc.describeCapture(settings)
	if settings.CaptureProvider != model.CRMMeetingProviderRecall || settings.CaptureConfigured {
		t.Fatalf("settings = %+v", settings)
	}
}
