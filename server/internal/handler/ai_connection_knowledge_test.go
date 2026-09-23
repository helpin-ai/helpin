package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestAIConnectionKnowledgeReflectsWorkspaceEmbeddingSource(t *testing.T) {
	tests := []struct {
		name       string
		server     bool
		source     service.EmbeddingSourceInfo
		err        error
		want       bool
		wantSource string
		wantDetail string
	}{
		{name: "no source", want: false},
		{
			name:   "workspace connection",
			source: service.EmbeddingSourceInfo{Source: service.EmbeddingSourceWorkspace, Provider: "openrouter", Model: "text-embedding-3-small", Dimensions: 1536},
			want:   true, wantSource: service.EmbeddingSourceWorkspace, wantDetail: "Using this workspace's OpenRouter connection",
		},
		{
			name: "server key", server: true,
			source: service.EmbeddingSourceInfo{Source: service.EmbeddingSourceServer, Provider: "openai", Model: "text-embedding-3-small", Dimensions: 1536},
			want:   true, wantSource: service.EmbeddingSourceServer, wantDetail: "Using the server's OpenAI key",
		},
		{name: "lookup failure keeps server wiring", server: true, err: errors.New("db down"), want: true, wantSource: service.EmbeddingSourceServer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AIConnectionHandler{}
			model := ""
			if tt.server {
				model = "text-embedding-3-small"
			}
			h.SetKnowledgeConfiguration(tt.server, model, nil)
			h.SetKnowledgeEmbeddingSource(func(context.Context, string) (service.EmbeddingSourceInfo, error) { return tt.source, tt.err })
			got := h.knowledgeFor(context.Background(), "workspace")
			if got.EmbeddingsConfigured != tt.want || got.EmbeddingSource != tt.wantSource || got.EmbeddingDetail != tt.wantDetail {
				t.Fatalf("knowledge = %+v", got)
			}
			if got.EmbeddingModel != "text-embedding-3-small" || got.EmbeddingDimensions != 1536 || got.ChatProviders == nil {
				t.Fatalf("knowledge shape changed: %+v", got)
			}
		})
	}
}
