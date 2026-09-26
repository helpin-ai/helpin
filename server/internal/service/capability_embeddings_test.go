package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestWorkspaceEmbeddingCapabilityReflectsSource(t *testing.T) {
	workspaceOpenRouter := func(context.Context, string) (EmbeddingSourceInfo, error) {
		return EmbeddingSourceInfo{Source: EmbeddingSourceWorkspace, Provider: "openrouter", Model: defaultDocsEmbeddingModel}, nil
	}
	serverOpenAI := func(context.Context, string) (EmbeddingSourceInfo, error) {
		return EmbeddingSourceInfo{Source: EmbeddingSourceServer, Provider: "openai", Model: defaultDocsEmbeddingModel}, nil
	}
	none := func(context.Context, string) (EmbeddingSourceInfo, error) { return EmbeddingSourceInfo{}, nil }
	tests := []struct {
		name       string
		cfg        CapabilityConfig
		embedded   bool
		wantStatus string
		wantDetail string
		wantAction string
	}{
		{
			name: "no source anywhere", cfg: CapabilityConfig{EmbeddingSource: none},
			wantStatus: model.CapabilityNeedsSetup, wantDetail: "keywords only", wantAction: model.CapabilityActionOpenSettings,
		},
		{
			name: "workspace connection before indexing", cfg: CapabilityConfig{EmbeddingSource: workspaceOpenRouter},
			wantStatus: model.CapabilityUnableToVerify, wantDetail: "Using this workspace's OpenRouter connection",
		},
		{
			name: "workspace connection after indexing", cfg: CapabilityConfig{EmbeddingSource: workspaceOpenRouter}, embedded: true,
			wantStatus: model.CapabilityReady, wantDetail: "Using this workspace's OpenRouter connection; knowledge has been indexed",
		},
		{
			name:       "server key",
			cfg:        CapabilityConfig{EmbeddingModel: defaultDocsEmbeddingModel, EmbeddingSource: serverOpenAI},
			embedded:   true,
			wantStatus: model.CapabilityReady, wantDetail: "Using the server's OpenAI key",
		},
		{
			name: "server key without a resolver", cfg: CapabilityConfig{EmbeddingModel: "m"},
			wantStatus: model.CapabilityUnableToVerify, wantDetail: "Embeddings are configured (m)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newCapabilityService(tt.cfg, &fakeCapabilityEvidence{embedded: tt.embedded}, fakeCapabilitySetup{}, nil)
			got, err := s.aiEmbeddings(context.Background(), "workspace")
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.wantStatus || !strings.Contains(got.Detail, tt.wantDetail) {
				t.Fatalf("capability = %s %q, want %s containing %q", got.Status, got.Detail, tt.wantStatus, tt.wantDetail)
			}
			if tt.wantAction != "" && (got.Action == nil || got.Action.Kind != tt.wantAction || got.Action.Path != "settings/ai") {
				t.Fatalf("action = %+v, want %s to settings/ai", got.Action, tt.wantAction)
			}
		})
	}
}

func TestInstanceEmbeddingCapabilityIgnoresWorkspaceConnections(t *testing.T) {
	called := false
	cfg := CapabilityConfig{EmbeddingSource: func(context.Context, string) (EmbeddingSourceInfo, error) {
		called = true
		return EmbeddingSourceInfo{Source: EmbeddingSourceWorkspace, Provider: "openai"}, nil
	}}
	s := newCapabilityService(cfg, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, nil)
	got, err := s.aiEmbeddings(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if called || got.Status != model.CapabilityNeedsSetup || got.Action == nil || got.Action.Kind != model.CapabilityActionServerConfig {
		t.Fatalf("instance capability = %+v (resolver called: %v)", got, called)
	}
}

func TestWorkspaceEmbeddingCapabilityReturnsResolverError(t *testing.T) {
	failure := errors.New("lookup failed")
	cfg := CapabilityConfig{EmbeddingSource: func(context.Context, string) (EmbeddingSourceInfo, error) {
		return EmbeddingSourceInfo{}, failure
	}}
	s := newCapabilityService(cfg, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, nil)
	if _, err := s.aiEmbeddings(context.Background(), "workspace"); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}
}
