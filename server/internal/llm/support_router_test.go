package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSupportRouterSelectsEmbeddingProvider(t *testing.T) {
	tests := []struct {
		name          string
		openAIKey     string
		openRouterKey string
		wantType      string
		wantDefault   string
	}{
		{name: "openai preferred when both are configured", openAIKey: "openai-key", openRouterKey: "router-key", wantType: "openai", wantDefault: "openai"},
		{name: "openai only", openAIKey: "openai-key", wantType: "openai", wantDefault: "openai"},
		{name: "openrouter fallback", openRouterKey: "router-key", wantType: "openrouter", wantDefault: "openrouter"},
		{name: "no embedding provider", wantType: "", wantDefault: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, embeddings := NewSupportRouter("", tt.openAIKey, "", tt.openRouterKey, "")
			gotType := ""
			switch embeddings.(type) {
			case *OpenAIProvider:
				gotType = "openai"
			case *openRouterEmbeddingProvider:
				gotType = "openrouter"
			}
			if gotType != tt.wantType {
				t.Fatalf("embedding provider = %q, want %q", gotType, tt.wantType)
			}
			gotDefault := ""
			if router != nil {
				gotDefault = router.defaultEmbeddingProvider
			}
			if gotDefault != tt.wantDefault {
				t.Fatalf("default embedding provider = %q, want %q", gotDefault, tt.wantDefault)
			}
		})
	}
}

func TestOpenRouterEmbeddingsUseVendorModelNames(t *testing.T) {
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer router-key" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		models = append(models, body.Model)
		if err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"embedding": make([]float32, 1536)}}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	_, embeddings := NewSupportRouter("", "", "", "router-key", server.URL)
	for _, model := range []string{"", "text-embedding-3-small", "openai/text-embedding-3-large"} {
		response, err := embeddings.CreateEmbeddings(context.Background(), EmbeddingRequest{Model: model, Inputs: []string{"hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Vectors) != 1 || len(response.Vectors[0]) != 1536 {
			t.Fatalf("unexpected vectors: %d", len(response.Vectors))
		}
	}
	want := []string{"openai/text-embedding-3-small", "openai/text-embedding-3-small", "openai/text-embedding-3-large"}
	for i := range want {
		if i >= len(models) || models[i] != want[i] {
			t.Fatalf("models = %v, want %v", models, want)
		}
	}
}

func TestSupportEmbeddingModel(t *testing.T) {
	tests := []struct {
		name, openAI, openRouter, configured, want string
	}{
		{name: "openai default", openAI: "k", want: "text-embedding-3-small"},
		{name: "openai configured", openAI: "k", configured: "text-embedding-3-large", want: "text-embedding-3-large"},
		{name: "openrouter default", openRouter: "k", want: DefaultOpenRouterEmbeddingModel},
		{name: "openrouter configured", openRouter: "k", configured: "text-embedding-3-small", want: "openai/text-embedding-3-small"},
		{name: "none", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportEmbeddingModel(tt.openAI, tt.openRouter, tt.configured); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
