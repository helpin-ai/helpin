package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportAIRetrievalTraceBuildCompactsKnowledgeResults(t *testing.T) {
	longContent := strings.Repeat("refund policy details ", 80)
	results := make([]KnowledgeSearchResult, 0, 12)
	for i := 0; i < 12; i++ {
		results = append(results, KnowledgeSearchResult{
			ID:            "chunk-id",
			ReferenceID:   "ref-id",
			SourceType:    knowledgeSourceTypeDocs,
			DocumentID:    "doc-id",
			SourceID:      "source-id",
			ChunkIndex:    i,
			Title:         "Refunds",
			URL:           "https://example.com/refunds",
			Content:       longContent,
			LexicalScore:  0.1,
			VectorScore:   0.2,
			CombinedScore: 0.3,
		})
	}
	canAnswer := "true"
	canResolve := "partial"

	trace, err := BuildSupportAIRetrievalTrace(SupportAIRetrievalTraceInput{
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
		SearchQueries:  []string{strings.Repeat("refund ", 80)},
		SearchResults:  results,
		CitedSourceIDs: []string{"doc-id", "doc-id", "source-id"},
		AIConfidence:   0.72,
		CanAnswer:      &canAnswer,
		CanResolve:     &canResolve,
		FailureMode:    "weak_retrieval",
	})
	if err != nil {
		t.Fatalf("BuildSupportAIRetrievalTrace: %v", err)
	}
	if trace.WorkspaceID != "ws-1" || trace.ConversationID != "conversation-1" || trace.MessageID != "message-1" {
		t.Fatalf("trace identifiers were not preserved: %+v", trace)
	}
	if trace.AIConfidence != 0.72 || trace.CanAnswer == nil || *trace.CanAnswer != "true" || trace.CanResolve == nil || *trace.CanResolve != "partial" {
		t.Fatalf("trace evaluation fields were not preserved: %+v", trace)
	}

	var queries []string
	if err := json.Unmarshal(trace.SearchQueries, &queries); err != nil {
		t.Fatalf("unmarshal search queries: %v", err)
	}
	if len(queries) != 1 || len(queries[0]) > 203 {
		t.Fatalf("expected capped query, got %q", queries)
	}

	var compactResults []map[string]any
	if err := json.Unmarshal(trace.Results, &compactResults); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(compactResults) != 8 {
		t.Fatalf("expected 8 capped results, got %d", len(compactResults))
	}
	snippet, _ := compactResults[0]["snippet"].(string)
	if len(snippet) > 503 {
		t.Fatalf("expected capped snippet, got %d chars", len(snippet))
	}

	var cited []string
	if err := json.Unmarshal(trace.CitedSourceIDs, &cited); err != nil {
		t.Fatalf("unmarshal cited source ids: %v", err)
	}
	if strings.Join(cited, ",") != "doc-id,source-id" {
		t.Fatalf("expected deduped cited source ids, got %v", cited)
	}
}

func TestSupportAIRetrievalTraceServiceRecordsViaRepository(t *testing.T) {
	repo := &fakeSupportAIRetrievalTraceRepo{}
	svc := NewSupportCoverageRetrievalTraceService(repo)

	err := svc.RecordSupportAIRetrievalTrace(context.Background(), &model.SupportAIRetrievalTrace{
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	if err != nil {
		t.Fatalf("RecordSupportAIRetrievalTrace: %v", err)
	}
	if repo.recorded == nil || repo.recorded.MessageID != "message-1" {
		t.Fatalf("expected trace to be recorded, got %+v", repo.recorded)
	}
}

func TestSupportAIRetrievalTraceBestEffortDoesNotBlock(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	recorder := &blockingSupportAIRetrievalTraceRecorder{
		started: started,
		release: release,
		err:     errors.New("write failed"),
	}
	svc := &SupportAIService{}
	svc.SetSupportAIRetrievalTraceRecorder(recorder)

	start := time.Now()
	svc.recordSupportAIRetrievalTraceBestEffort(&model.SupportAIRetrievalTrace{
		WorkspaceID:    "ws-1",
		ConversationID: "conversation-1",
		MessageID:      "message-1",
	})
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("best-effort trace recording blocked caller")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("trace recorder was not invoked")
	}
	close(release)
}

type fakeSupportAIRetrievalTraceRepo struct {
	recorded *model.SupportAIRetrievalTrace
}

func (r *fakeSupportAIRetrievalTraceRepo) UpsertRetrievalTrace(_ context.Context, trace *model.SupportAIRetrievalTrace) error {
	r.recorded = trace
	return nil
}

type blockingSupportAIRetrievalTraceRecorder struct {
	started chan struct{}
	release chan struct{}
	err     error
}

func (r *blockingSupportAIRetrievalTraceRecorder) RecordSupportAIRetrievalTrace(_ context.Context, _ *model.SupportAIRetrievalTrace) error {
	close(r.started)
	<-r.release
	return r.err
}

func TestSupportRetrievalTraceExcludesPrivateMCPContent(t *testing.T) {
	results := []KnowledgeSearchResult{{ID: "mcp__logs__lookup", SourceType: "external_mcp", Content: "secret customer logs", Title: "private title", URL: "https://internal.example"}, {ID: "public", Content: "Public help"}}
	compact := compactKnowledgeSearchResults(results)
	encoded, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "private title") || strings.Contains(string(encoded), "internal.example") {
		t.Fatalf("private evidence leaked: %s", encoded)
	}
	if len(compact) != 1 || compact[0].ID != "public" {
		t.Fatalf("want public evidence only, got %s", encoded)
	}
}
