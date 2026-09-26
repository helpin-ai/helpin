package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestRuntimeRouteReadinessUsesRunCredentialsAndSeparateChatGPTCapabilities(t *testing.T) {
	caps := &sdk.Capabilities{RuntimeKinds: []string{"native_sdk"}, Providers: []sdk.ProviderCapability{{Name: "openai", AuthModes: []string{"api_key"}, RunCredentialsConfigured: true, LosslessResponseReplay: true}}}
	if err := validateRuntimeRoute(caps, "helpin", "openai"); err != nil {
		t.Fatalf("environment key should be unnecessary: %v", err)
	}
	if err := validateRuntimeRoute(caps, "helpin", "openai_chatgpt"); err == nil {
		t.Fatal("ChatGPT inherited direct OpenAI readiness")
	}
	caps.Providers = append(caps.Providers, sdk.ProviderCapability{Name: "openai_chatgpt", AuthModes: []string{"oauth"}, RunCredentialsConfigured: true})
	if err := validateRuntimeRoute(caps, "helpin", "openai_chatgpt"); err == nil {
		t.Fatal("ChatGPT without callback ready")
	}
	caps.Apps = []sdk.AppSummary{{AppID: "other", Components: []sdk.AppComponent{{Kind: "model_credentials", Configured: true, AuthConfigured: true}}}}
	if err := validateRuntimeRoute(caps, "helpin", "openai_chatgpt"); err == nil {
		t.Fatal("used another app's callback")
	}
	caps.Apps[0].AppID = "helpin"
	if err := validateRuntimeRoute(caps, "helpin", "openai_chatgpt"); err != nil {
		t.Fatal(err)
	}
	caps.Providers[0].Configured, caps.Providers[0].RunCredentialsConfigured = true, false
	if err := validateRuntimeRoute(caps, "helpin", "openai"); err == nil {
		t.Fatal("environment key masked missing credential encryption")
	}
}

func TestAIProfileRuntimeReadinessFailureNeverFallsBack(t *testing.T) {
	profiles, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if err := profiles.connections.repo.WithLocked(ctx, primary.ConnectionID, func(c *model.AIConnection) error { c.Status = "disconnected"; return nil }); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/capabilities" || r.URL.Query().Get("app_id") != "helpin" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(sdk.Capabilities{RuntimeKinds: []string{"native_sdk"}, Providers: []sdk.ProviderCapability{{Name: "openai", Configured: true}}})
	}))
	defer server.Close()
	profiles.connections.runtime, err = NewAgentRuntimeClient(server.URL, "helpin", "test", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	profiles.CheckRuntimeReadiness()
	selection, _, err := profiles.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID})
	if err == nil || errors.Is(err, ErrAIConnectionUnavailable) || selection != nil {
		t.Fatalf("capability failure became fallback: %+v, %v", selection, err)
	}
}
