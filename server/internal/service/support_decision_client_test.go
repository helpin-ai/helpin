package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportDecisionClientPolicyAndFallback(t *testing.T) {
	options := []supportTriageMailboxOption{{Handle: "sales", Prompt: "Handles purchasing questions"}, {Handle: "shared", Prompt: "General support"}}
	_, hash := decisionChoices(options)
	fingerprint := strings.Repeat("a", 64)
	tests := []struct {
		name       string
		mode       string
		body       string
		status     int
		policyHash string
		accepted   bool
		reason     string
	}{
		{"accepted", "primary", `{"choice":"sales","confidence":0.95,"confidence_kind":"temperature_scaled","query_truncated":false,"deployment_fingerprint":"` + fingerprint + `","scores":[{"id":"sales","probability":0.95},{"id":"shared","probability":0.05}]}`, 200, hash, true, "accepted"},
		{"shadow", "shadow", `{"choice":"sales","confidence":0.95,"scores":[{"id":"sales","probability":0.95},{"id":"shared","probability":0.05}]}`, 200, hash, false, "shadow"},
		{"description change", "primary", `{"choice":"sales","confidence":0.95,"confidence_kind":"temperature_scaled","query_truncated":false,"deployment_fingerprint":"` + fingerprint + `","scores":[{"id":"sales","probability":0.95},{"id":"shared","probability":0.05}]}`, 200, strings.Repeat("b", 64), false, "policy_mismatch"},
		{"unknown choice", "primary", `{"choice":"invented","confidence":0.95}`, 200, hash, false, "invalid_scores"},
		{"busy", "primary", `{}`, 503, hash, false, "service_error"},
		{"bad JSON", "primary", `{`, 200, hash, false, "invalid_response"},
		{"abstained", "primary", `{"choice":null,"abstained":true}`, 200, hash, false, "abstained"},
		{"uncalibrated", "primary", `{"choice":"sales","confidence":0.95,"confidence_kind":"uncalibrated_probability","query_truncated":false,"deployment_fingerprint":"` + fingerprint + `","scores":[{"id":"sales","probability":0.95},{"id":"shared","probability":0.05}]}`, 200, hash, false, "uncalibrated_or_truncated"},
		{"truncated", "primary", `{"choice":"sales","confidence":0.95,"confidence_kind":"temperature_scaled","query_truncated":true,"deployment_fingerprint":"` + fingerprint + `","scores":[{"id":"sales","probability":0.95},{"id":"shared","probability":0.05}]}`, 200, hash, false, "uncalibrated_or_truncated"},
		{"low confidence", "primary", `{"choice":"sales","confidence":0.6,"confidence_kind":"temperature_scaled","query_truncated":false,"deployment_fingerprint":"` + fingerprint + `","scores":[{"id":"sales","probability":0.6},{"id":"shared","probability":0.4}]}`, 200, hash, false, "low_confidence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("missing authentication")
				}
				w.WriteHeader(tt.status)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: tt.mode, URL: server.URL, Token: "secret", Timeout: time.Second, WorkspaceIDs: []string{"ws"}, Policies: []SupportDecisionPolicy{{WorkspaceID: "ws", DeploymentFingerprint: fingerprint, ChoicesHash: tt.policyHash, Question: supportDecisionQuestion, MinConfidence: .9, ExpiresAt: time.Now().Add(time.Hour)}}})
			if err != nil {
				t.Fatal(err)
			}
			_, accepted, reason, _ := client.decide(context.Background(), "ws", "Pricing please", options)
			if accepted != tt.accepted || reason != tt.reason {
				t.Fatalf("accepted=%v reason=%s", accepted, reason)
			}
		})
	}
}

func TestSupportDecisionClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()
	client, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: "shadow", URL: server.URL, Token: "secret", Timeout: 20 * time.Millisecond, WorkspaceIDs: []string{"ws"}})
	if err != nil {
		t.Fatal(err)
	}
	_, accepted, reason, err := client.decide(context.Background(), "ws", "x", []supportTriageMailboxOption{{Handle: "a", Prompt: "a"}, {Handle: "b", Prompt: "b"}})
	if err == nil || accepted || reason != "unavailable" {
		t.Fatalf("accepted=%v reason=%s error=%v", accepted, reason, err)
	}
}

func TestSupportDecisionTriageFallsBackToLLM(t *testing.T) {
	for _, status := range []int{503, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			fake := &scriptedSupportTriageLLM{response: `{"intent":"sales","target_mailbox_handle":"sales","confidence":0.8,"reason":"Purchase question"}`}
			fixture := newSupportTriageTestFixture(t, fake, nil)
			sales := fixture.createMailbox(t, "Sales", "sales", true)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				if err := json.NewEncoder(w).Encode(map[string]any{"abstained": true}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: "shadow", URL: server.URL, Token: "secret", Timeout: time.Second, WorkspaceIDs: []string{fixture.workspaceID}})
			if err != nil {
				t.Fatal(err)
			}
			fixture.triageSvc.SetLocalDecisionClient(client)
			result, err := fixture.triageSvc.evaluateAI(fixture.ctx, fixture.workspaceID, model.SupportInboxSettings{}, &model.SupportConversation{ID: "test-conversation", WorkspaceID: fixture.workspaceID}, "Pricing please", "hash")
			if err != nil {
				t.Fatal(err)
			}
			if fake.calls != 1 || result == nil || result.SuggestedMailboxID == nil || *result.SuggestedMailboxID != sales.ID {
				t.Fatalf("LLM fallback did not route to sales: calls=%d result=%+v", fake.calls, result)
			}
		})
	}
}

func TestSupportDecisionPrimaryRequiresPolicy(t *testing.T) {
	_, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: "primary", URL: "http://localhost:8091", Token: "secret", Timeout: time.Second, WorkspaceIDs: []string{"ws"}})
	if err == nil {
		t.Fatal("primary without policy must fail startup")
	}
}

func TestSupportDecisionTriageAcceptedSkipsLLM(t *testing.T) {
	fake := &scriptedSupportTriageLLM{}
	fixture := newSupportTriageTestFixture(t, fake, nil)
	sales := fixture.createMailbox(t, "Sales", "sales", true)
	options, _, err := fixture.triageSvc.aiMailboxOptions(fixture.ctx, fixture.workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	_, hash := decisionChoices(options)
	fingerprint := strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{"choice": "sales", "confidence": .95, "confidence_kind": "temperature_scaled", "query_truncated": false, "deployment_fingerprint": fingerprint, "scores": []map[string]any{{"id": "sales", "probability": .95}, {"id": "shared", "probability": .05}}}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: "primary", URL: server.URL, Token: "secret", Timeout: time.Second, WorkspaceIDs: []string{fixture.workspaceID}, Policies: []SupportDecisionPolicy{{WorkspaceID: fixture.workspaceID, DeploymentFingerprint: fingerprint, ChoicesHash: hash, Question: supportDecisionQuestion, MinConfidence: .9, ExpiresAt: time.Now().Add(time.Hour)}}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.triageSvc.SetLocalDecisionClient(client)
	result, err := fixture.triageSvc.evaluateAI(fixture.ctx, fixture.workspaceID, model.SupportInboxSettings{}, &model.SupportConversation{ID: "test-conversation", WorkspaceID: fixture.workspaceID}, "Pricing please", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 || result == nil || result.SuggestedMailboxID == nil || *result.SuggestedMailboxID != sales.ID {
		t.Fatalf("local acceptance failed: LLM calls=%d result=%+v", fake.calls, result)
	}
}

// TestSupportDecisionLiveService verifies the Go/Python contract when an isolated service is supplied.
func TestSupportDecisionLiveService(t *testing.T) {
	endpoint := os.Getenv("SUPPORT_DECISION_LIVE_URL")
	if endpoint == "" {
		t.Skip("isolated local service not configured")
	}
	client, err := NewSupportDecisionClient(SupportDecisionConfig{Mode: "shadow", URL: endpoint, Token: os.Getenv("SUPPORT_DECISION_LIVE_TOKEN"), Timeout: time.Second, WorkspaceIDs: []string{"test"}})
	if err != nil {
		t.Fatal(err)
	}
	result, accepted, reason, err := client.decide(context.Background(), "test", "I was charged twice. Please refund the duplicate payment.", []supportTriageMailboxOption{{Handle: "billing", Prompt: "Handles charges, payments, invoices and refunds."}, {Handle: "support", Prompt: "Handles software bugs and technical support."}})
	if err != nil {
		t.Fatal(err)
	}
	if accepted || reason != "shadow" || result == nil || result.Choice == nil || *result.Choice != "billing" || result.QueryTruncated == nil || *result.QueryTruncated || len(result.DeploymentFingerprint) != 64 {
		t.Fatalf("invalid live service result: accepted=%v reason=%s result=%+v", accepted, reason, result)
	}
}
