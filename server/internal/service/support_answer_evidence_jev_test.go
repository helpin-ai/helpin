package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestJevAnswerEvidenceUsesReturnedExcerpts(t *testing.T) {
	decisions, p, _, _ := setupJevDecisionTest(t, "primary")
	p.choices = map[string]string{"evidence_0": "supported", "evidence_1": "partial"}
	command := &InternalCommandService{}
	command.SetJevDecisions(decisions)
	long := strings.Repeat("é", 700) + "SECRET_UNRETURNED_TAIL"
	results := []KnowledgeSearchResult{{ID: "source-a", Content: long, CombinedScore: .8}, {ID: "source-b", Content: "Some steps are documented", CombinedScore: .6}}
	assessment := command.assessAnswerEvidence(context.Background(), "workspace", "run", "How do I set this up?", results)
	if len(assessment) != 2 || assessment["source-a"].Status != "supported" || assessment["source-b"].Status != "partial" {
		t.Fatalf("assessments %+v", assessment)
	}
	if assessment["source-a"].Scope != "returned_excerpt_only" || assessment["source-a"].AssessmentID == "" {
		t.Fatal("missing scope/provenance")
	}
	if strings.Contains(p.states[0], "SECRET_UNRETURNED_TAIL") || !utf8.ValidString(supportKnowledgeExcerpt(long)) {
		t.Fatal("assessed unseen or invalid excerpt")
	}
	if results[0].Content != long || results[0].CombinedScore != .8 {
		t.Fatal("assessment changed retrieval or evidence")
	}
}
func TestJevAnswerEvidencePreservesFallback(t *testing.T) {
	for _, scenario := range []string{"off", "shadow", "error", "oversize", "uncertain", "low probability"} {
		t.Run(scenario, func(t *testing.T) {
			mode := "primary"
			if scenario == "off" || scenario == "shadow" {
				mode = scenario
			}
			decisions, p, _, _ := setupJevDecisionTest(t, mode)
			p.choices = map[string]string{"evidence_0": "supported"}
			query := "How do I set this up?"
			switch scenario {
			case "error":
				p.err = errors.New("unavailable")
			case "oversize":
				query = strings.Repeat("x", 16001)
			case "uncertain":
				p.choices["evidence_0"] = "uncertain"
			case "low probability":
				p.probability = .7
			}
			command := &InternalCommandService{}
			command.SetJevDecisions(decisions)
			assessment := command.assessAnswerEvidence(context.Background(), "workspace", "run", query, []KnowledgeSearchResult{{ID: "source", Content: "Setup steps"}})
			if scenario == "uncertain" || scenario == "low probability" {
				if assessment["source"].Status != "uncertain" {
					t.Fatal("uncertain evidence presented as sufficient")
				}
			} else if len(assessment) != 0 {
				t.Fatal("fallback exposed a decision")
			}
		})
	}
}

func TestJevAnswerEvidenceIsReturnedByKnowledgeTool(t *testing.T) {
	decisions, p, _, _ := setupJevDecisionTest(t, "primary")
	p.choices = map[string]string{"evidence_0": "partial"}
	db := setupSupportKnowledgeTestDB(t)
	if err := db.Exec(`INSERT INTO agent_runs(id,workspace_id,agent_id,target_type,target_id,status,external_runtime,external_runtime_id,input,output_summary) VALUES ('run','workspace','agent-1','support_conversation','conversation','running','agent-runtime','runtime-run',?,?)`, []byte(`{}`), []byte(`{}`)).Error; err != nil {
		t.Fatal(err)
	}
	searcher := &stubKnowledgeSearcher{results: []KnowledgeSearchResult{{ID: "chunk", ReferenceID: "docs:chunk", SourceType: "docs", Content: "Install the package.", CombinedScore: .8}}}
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}, agentRunRepo: repository.NewAgentRunRepository(db)}
	svc.SetSupportKnowledgeDependencies(searcher, repository.NewSupportRunEvidenceRepository(db))
	svc.SetJevDecisions(decisions)
	svc.registerSupportKnowledgeCommands()
	result, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "workspace", RunID: "runtime-run", TargetType: "support_conversation", TargetID: "conversation"}, "support.search_knowledge", json.RawMessage(`{"queries":["How do I install and authenticate?"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Results []struct {
			EvidenceID string                           `json:"evidence_id"`
			Content    string                           `json:"content"`
			Support    *SupportAnswerEvidenceAssessment `json:"answer_support"`
		} `json:"results"`
	}
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Results) != 1 || output.Results[0].Support == nil || output.Results[0].Support.Status != "partial" || output.Results[0].Support.AssessmentID == "" || output.Results[0].EvidenceID != "chunk" {
		t.Fatalf("tool output=%s", result)
	}
	var persisted model.SupportRunEvidence
	if err := db.First(&persisted, "evidence_id = ?", "chunk").Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Content != "Install the package." || persisted.CombinedScore != .8 {
		t.Fatal("classification altered citation evidence")
	}
}
