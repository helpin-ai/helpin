package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubKnowledgeSearcher struct {
	lastQueries []string
	lastConv    string
	results     []KnowledgeSearchResult
	err         error
}

func (s *stubKnowledgeSearcher) SearchKnowledgeForConversation(_ context.Context, _, conversationID, _ string, queries []string) (*SupportKnowledgeSearchOutcome, error) {
	s.lastConv = conversationID
	s.lastQueries = queries
	if s.err != nil {
		return nil, s.err
	}
	return &SupportKnowledgeSearchOutcome{AgentID: "agent-1", Results: s.results}, nil
}

func setupSupportKnowledgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_knowledge_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_run_evidence (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		evidence_id TEXT NOT NULL,
		reference_id TEXT,
		source_type TEXT NOT NULL,
		source_id TEXT,
		document_id TEXT,
		title TEXT,
		url TEXT,
		is_internal INTEGER NOT NULL DEFAULT 0,
		content TEXT NOT NULL,
		lexical_score REAL NOT NULL DEFAULT 0,
		vector_score REAL NOT NULL DEFAULT 0,
		combined_score REAL NOT NULL DEFAULT 0,
		created_at DATETIME,
		UNIQUE(run_id, evidence_id)
	)`).Error; err != nil {
		t.Fatalf("create evidence table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_runs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		model_tier TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		external_runtime TEXT,
		external_runtime_id TEXT,
		input BLOB NOT NULL DEFAULT '{}',
		output_summary BLOB NOT NULL DEFAULT '{}'
	)`).Error; err != nil {
		t.Fatalf("create runs table: %v", err)
	}
	return db
}

func TestSearchKnowledgeCommand(t *testing.T) {
	db := setupSupportKnowledgeTestDB(t)
	searcher := &stubKnowledgeSearcher{
		results: []KnowledgeSearchResult{
			{ID: "chunk-1", ReferenceID: "docs:chunk-1", SourceType: "docs", Title: "Install guide", URL: "https://x/install", Content: strings.Repeat("a", 1500), LexicalScore: 0.31, VectorScore: 0.78, CombinedScore: 0.9},
			{ID: "g-1", ReferenceID: "guidance:g-1", SourceType: "curated_guidance", IsInternal: true, Content: "internal note", CombinedScore: 0.8},
		},
	}
	if err := db.Exec(`INSERT INTO agent_runs (id, workspace_id, agent_id, target_type, target_id, status, external_runtime, external_runtime_id, input, output_summary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-1", "ws-1", "agent-1", "support_conversation", "conv-1", "running", "agent-runtime", "rt-run-1",
		[]byte(`{}`), []byte(`{}`)).Error; err != nil {
		t.Fatalf("insert run: %v", err)
	}

	svc := &InternalCommandService{
		definitions:  make(map[string]InternalCommandDefinition),
		agentRunRepo: repository.NewAgentRunRepository(db),
	}
	svc.SetSupportKnowledgeDependencies(searcher, repository.NewSupportRunEvidenceRepository(db))
	svc.registerSupportKnowledgeCommands()

	meta := model.InternalCommandContext{
		WorkspaceID: "ws-1",
		RunID:       "rt-run-1",
		TargetType:  "support_conversation",
		TargetID:    "conv-1",
	}

	t.Run("returns compact rows and persists evidence", func(t *testing.T) {
		output, err := svc.Execute(context.Background(), meta, "support.search_knowledge", json.RawMessage(`{"queries":["how do i install", "installation steps"]}`))
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		var resp struct {
			Results []struct {
				EvidenceID                string  `json:"evidence_id"`
				URL                       string  `json:"url"`
				IsInternal                bool    `json:"is_internal"`
				Content                   string  `json:"content"`
				GroundedConfidenceCeiling float64 `json:"grounded_confidence_ceiling"`
			} `json:"results"`
			Total                          int     `json:"total"`
			RequiredConfidence             float64 `json:"required_confidence"`
			BestPossibleGroundedConfidence float64 `json:"best_possible_grounded_confidence"`
		}
		if err := json.Unmarshal(output, &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Total != 2 || resp.Results[0].EvidenceID != "chunk-1" {
			t.Fatalf("unexpected results: %+v", resp)
		}
		if resp.RequiredConfidence != 0.7 {
			t.Fatalf("required confidence = %v, want default 0.7", resp.RequiredConfidence)
		}
		if resp.BestPossibleGroundedConfidence < resp.RequiredConfidence {
			t.Fatalf("confidence ceiling = %v, want at least %v", resp.BestPossibleGroundedConfidence, resp.RequiredConfidence)
		}
		if resp.Results[0].GroundedConfidenceCeiling < resp.RequiredConfidence {
			t.Fatalf("first result confidence ceiling = %v, want at least %v", resp.Results[0].GroundedConfidenceCeiling, resp.RequiredConfidence)
		}
		if resp.Results[0].URL != "https://x/install" {
			t.Fatalf("result URL = %q, want source chunk URL", resp.Results[0].URL)
		}
		if len(resp.Results[0].Content) > supportKnowledgeContentExcerpt+len("…") {
			t.Errorf("content not excerpted: %d chars", len(resp.Results[0].Content))
		}
		if !resp.Results[1].IsInternal {
			t.Error("expected is_internal flag on guidance chunk")
		}
		if searcher.lastConv != "conv-1" || len(searcher.lastQueries) != 2 {
			t.Errorf("searcher called with conv=%q queries=%v", searcher.lastConv, searcher.lastQueries)
		}
		var count int64
		db.Table("support_run_evidence").Where("run_id = ?", "run-1").Count(&count)
		if count != 2 {
			t.Errorf("evidence rows = %d, want 2", count)
		}
		// Full (non-excerpted) content is stored for numeric grounding.
		var stored string
		db.Table("support_run_evidence").Where("run_id = ? AND evidence_id = ?", "run-1", "chunk-1").Pluck("content", &stored)
		if len(stored) != 1500 {
			t.Errorf("stored content length = %d, want 1500", len(stored))
		}
		var scores struct {
			LexicalScore  float64
			VectorScore   float64
			CombinedScore float64
		}
		if err := db.Table("support_run_evidence").Where("run_id = ? AND evidence_id = ?", "run-1", "chunk-1").Take(&scores).Error; err != nil {
			t.Fatalf("load stored evidence scores: %v", err)
		}
		if scores.LexicalScore != 0.31 || scores.VectorScore != 0.78 || scores.CombinedScore != 0.9 {
			t.Fatalf("stored evidence scores = %+v", scores)
		}
	})

	t.Run("repeated search stays idempotent per run", func(t *testing.T) {
		if _, err := svc.Execute(context.Background(), meta, "support.search_knowledge", json.RawMessage(`{"queries":["again"]}`)); err != nil {
			t.Fatalf("execute: %v", err)
		}
		var count int64
		db.Table("support_run_evidence").Where("run_id = ?", "run-1").Count(&count)
		if count != 2 {
			t.Errorf("evidence rows after repeat = %d, want 2", count)
		}
	})

	t.Run("requires a conversation target", func(t *testing.T) {
		badMeta := meta
		badMeta.TargetType = "workspace"
		badMeta.TargetID = "ws-1"
		if _, err := svc.Execute(context.Background(), badMeta, "support.search_knowledge", json.RawMessage(`{"queries":["x"]}`)); err == nil {
			t.Fatal("expected target error")
		}
	})

	t.Run("requires a non-empty query", func(t *testing.T) {
		if _, err := svc.Execute(context.Background(), meta, "support.search_knowledge", json.RawMessage(`{"queries":["   "]}`)); err == nil {
			t.Fatal("expected query error")
		}
	})

	t.Run("caps queries at three", func(t *testing.T) {
		if _, err := svc.Execute(context.Background(), meta, "support.search_knowledge", json.RawMessage(`{"queries":["a","b","c","d","e"]}`)); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if len(searcher.lastQueries) != supportKnowledgeMaxQueries {
			t.Errorf("queries passed = %d, want %d", len(searcher.lastQueries), supportKnowledgeMaxQueries)
		}
	})
}
