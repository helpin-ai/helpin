package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/agent-runtime-go/chatgptauth"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCompatibleConnectionFreezesApprovedEndpointAndNoAuth(t *testing.T) {
	profiles, _, _, db := setupAIProfileTestDB(t)
	ctx := context.Background()
	endpoint := sdk.ModelEndpoint{ID: "local", BaseURL: "http://127.0.0.1:8181/v1", AuthMode: "none"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(sdk.Capabilities{RuntimeKinds: []string{"native_sdk"}, Providers: []sdk.ProviderCapability{{Name: "openai_compatible", RunCredentialsConfigured: true, Protocols: []string{"chat_completions"}, AuthModes: []string{"none", "api_key"}}}, Apps: []sdk.AppSummary{{AppID: "helpin", ModelEndpoints: []sdk.ModelEndpoint{endpoint}}}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewAgentRuntimeClient(server.URL, "helpin", "service-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	profiles.connections.runtime = client
	profiles.CheckRuntimeReadiness()
	request := model.CreateAIConnectionRequest{Name: "Local", Scope: "workspace", Provider: "openai_compatible", EndpointID: "local"}
	request.APIKey = "must-not-be-sent"
	if _, err := profiles.connections.Create(ctx, "workspace", "owner", request); err == nil {
		t.Fatal("no-auth accepted a key")
	}
	request.APIKey, request.EndpointID = "", "not-approved"
	if _, err := profiles.connections.Create(ctx, "workspace", "owner", request); err == nil {
		t.Fatal("unapproved endpoint accepted")
	}
	request.EndpointID = "local"
	connection, err := profiles.connections.Create(ctx, "workspace", "owner", request)
	if err != nil {
		t.Fatal(err)
	}
	route := model.AIProfileRoute{ConnectionID: connection.Connection.ID, Model: sdk.RunModel{Provider: "openai_compatible", Model: "custom-unpriced"}}
	profile, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Local", Scope: "workspace", Primary: route})
	if err != nil {
		t.Fatal(err)
	}
	selection, credential, err := profiles.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{ProfileID: profile.ID, Unattended: true})
	if err != nil {
		t.Fatal(err)
	}
	if credential.Type != "none" || credential.APIKey != "" || !sameModelEndpoint(selection.Route.Model.Endpoint, &endpoint) || selection.Policy.Mode != "community" {
		t.Fatal("missing no-auth binding or community policy")
	}
	badEndpoint := endpoint
	badEndpoint.BaseURL = "http://127.0.0.1:9999/v1"
	route.Model.Endpoint = &badEndpoint
	if _, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Redirect", Scope: "workspace", Primary: route}); err == nil {
		t.Fatal("profile redirected credential")
	}
	// Even a direct stored-binding change cannot decrypt the original secret.
	badJSON, err := json.Marshal(badEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AIConnection{}).Where("id = ?", connection.Connection.ID).Update("endpoint", string(badJSON)).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := profiles.connections.Credential(ctx, "workspace", "owner", connection.Connection.ID, false); err == nil {
		t.Fatal("changed binding decrypted original credential")
	}
}

func TestAIProfileRefreshInfrastructureFailureDoesNotChooseFallback(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			profiles, _, fallback := setupAIProfileTest(t)
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte(`{"error":"test"}`))
			}))
			defer server.Close()
			oauth, err := chatgptauth.NewClient(chatgptauth.Config{Issuer: server.URL, AllowLocalHTTP: true})
			if err != nil {
				t.Fatal(err)
			}
			profiles.connections.oauth = oauth
			c := &model.AIConnection{ID: "expired-personal", WorkspaceID: "workspace", UserID: strPtr("owner"), Scope: "personal", Provider: "openai_chatgpt", Funding: "customer", Status: "connected", Name: "ChatGPT"}
			token := chatgptauth.Token{AccessToken: "test-access", RefreshToken: "test-refresh", AccountID: "test-account", ExpiresAt: time.Now().Add(-time.Minute)}
			if err := profiles.connections.seal(c, aiConnectionSecret{Token: &token}); err != nil {
				t.Fatal(err)
			}
			if err := profiles.connections.repo.Create(ctx, c); err != nil {
				t.Fatal(err)
			}
			p, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Personal", Scope: "personal", Primary: model.AIProfileRoute{ConnectionID: c.ID, Model: sdk.RunModel{Provider: "openai_chatgpt", Model: "test-model"}}, Fallback: &fallback})
			if err != nil {
				t.Fatal(err)
			}
			selection, credential, err := profiles.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID})
			if status == http.StatusUnauthorized {
				if err != nil || credential.APIKey != "Fallback-key" || selection.FallbackReason == "" {
					t.Fatalf("known unavailable did not fall back: %v", err)
				}
			} else if err == nil || selection != nil || credential != nil {
				t.Fatalf("infrastructure failure switched route: %+v, %v", selection, err)
			}
		})
	}
}
