package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestGitHubCapabilityReportsUnreachableAppBaseURL(t *testing.T) {
	reason := GitHubAppBaseURLBlockedReason("http://localhost:8080")
	svc := newCapabilityService(CapabilityConfig{Edition: "community", Modules: allModules(), GitHubReachabilityProblem: reason},
		&fakeCapabilityEvidence{}, fakeCapabilitySetup{}, fakeCapabilityAI{})

	instance, err := svc.Instance(context.Background())
	if err != nil {
		t.Fatalf("Instance: %v", err)
	}
	workspace, err := svc.Workspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("Workspace: %v", err)
	}
	for name, response := range map[string]model.CapabilitiesResponse{"instance": instance, "workspace": workspace} {
		var github *model.Capability
		for i := range response.Capabilities {
			if response.Capabilities[i].Key == model.CapabilityKeyGitHub {
				github = &response.Capabilities[i]
			}
		}
		if github == nil {
			t.Fatalf("%s: github capability missing", name)
		}
		if github.Status != model.CapabilityNeedsSetup || !strings.Contains(github.Detail, "Set APP_BASE_URL to a public https address (currently http://localhost:8080)") {
			t.Fatalf("%s: unexpected github capability %+v", name, github)
		}
	}
}
