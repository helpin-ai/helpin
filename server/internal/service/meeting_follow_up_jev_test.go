package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestJevMeetingRoutingRequiresScopeAndExactEvidence(t *testing.T) {
	for _, scenario := range []string{"supported", "uncertain", "missing evidence", "low probability", "shadow", "provider failure", "long transcript"} {
		t.Run(scenario, func(t *testing.T) {
			mode := "primary"
			if scenario == "shadow" {
				mode = "shadow"
			}
			decisions, p, _, _ := setupJevDecisionTest(t, mode)
			p.choices = map[string]string{"scope": "internal", "internal_evidence": "evidence_00", "customer_evidence": "unknown"}
			draft := "Prepare the engineering sprint plan for our internal team."
			transcript := "We need to plan the next sprint together."
			switch scenario {
			case "uncertain":
				p.choices["scope"] = "uncertain"
			case "missing evidence":
				p.choices["internal_evidence"] = "unknown"
			case "low probability":
				p.probability = .7
			case "provider failure":
				p.err = errors.New("unavailable")
			case "long transcript":
				transcript = strings.Repeat("Full transcript. ", 1500)
			}
			svc := (&CRMMeetingProcessingService{}).SetJevDecisions(decisions)
			result := svc.classifyJevMeetingFollowUp(context.Background(), "workspace", "draft", draft, transcript)
			if scenario == "supported" {
				if result == nil || result.scope != "internal" || !strings.Contains(draft, result.evidence) || result.assessmentID == "" {
					t.Fatalf("missing grounded route: %+v", result)
				}
			} else if result != nil {
				t.Fatalf("invalid route accepted: %+v", result)
			}
			if scenario == "long transcript" && p.calls != 0 {
				t.Fatal("oversized transcript sent to provider")
			}
		})
	}
}
func TestJevMeetingRoutingExistingDraftUsesJevWithoutRegenerating(t *testing.T) {
	decisions, p, _, _ := setupJevDecisionTest(t, "primary")
	p.choices = map[string]string{"scope": "customer", "internal_evidence": "unknown", "customer_evidence": "evidence_00"}
	svc := (&CRMMeetingProcessingService{}).SetJevDecisions(decisions)
	meetingID := "meeting"
	item := model.CRMSuggestion{ID: "draft", WorkspaceID: "workspace", ObjectID: &meetingID, Title: "Review Acme's renewal contract with legal"}
	scope, err := svc.classifyExistingFollowUp(context.Background(), item, &model.CRMMeetingTranscript{PlainText: "We need legal to review Acme's contract."})
	if err != nil || scope != "customer" || p.calls != 1 {
		t.Fatalf("scope=%s calls=%d error=%v", scope, p.calls, err)
	}
}
func TestJevMeetingRoutingFailurePreservesExistingEvidenceValidation(t *testing.T) {
	decisions, p, _, _ := setupJevDecisionTest(t, "primary")
	p.err = errors.New("unavailable")
	legacy := &scriptedMeetingIntelligenceLLM{results: []meetingIntelligenceLLMResult{{response: &llm.ChatResponse{Content: `{"scope":"internal","scope_evidence":"Prepare the internal engineering sprint plan"}`}}}}
	svc := (&CRMMeetingProcessingService{llmProvider: legacy}).SetJevDecisions(decisions)
	meetingID := "meeting"
	item := model.CRMSuggestion{ID: "draft", WorkspaceID: "workspace", ObjectID: &meetingID, Title: "Prepare the internal engineering sprint plan"}
	scope, err := svc.classifyExistingFollowUp(context.Background(), item, &model.CRMMeetingTranscript{PlainText: "We need a sprint plan."})
	if err != nil || scope != "internal" || len(legacy.requests) != 1 {
		t.Fatalf("legacy fallback failed: %s %v", scope, err)
	}
}

func TestJevMeetingProjectionRoutesDraftAndRetainsProvenance(t *testing.T) {
	decisions, p, _, _ := setupJevDecisionTest(t, "primary")
	p.choices = map[string]string{"scope": "internal", "internal_evidence": "evidence_00", "customer_evidence": "unknown"}
	manager := &routingSuggestionManager{}
	processor := (&CRMMeetingProcessingService{suggestions: manager}).SetJevDecisions(decisions)
	meeting := &model.CRMMeeting{ID: "meeting", WorkspaceID: "workspace", Title: "Engineering planning"}
	transcript := &model.CRMMeetingTranscript{PlainText: "Prepare the internal engineering sprint plan."}
	output := &meetingIntelligenceOutput{FollowUpDraft: meetingFollowUpOutput{Subject: "Prepare the internal engineering sprint plan", Body: "Arrange our next engineering planning session.", Scope: "uncertain"}}
	if err := processor.projectFollowUp(context.Background(), meeting, transcript, output); err != nil {
		t.Fatal(err)
	}
	if len(manager.created) != 1 {
		t.Fatal("draft not projected")
	}
	created := manager.created[0]
	if *created.Description != output.FollowUpDraft.Body || created.Context[model.MeetingFollowUpScopeKey] != "internal" || created.Context["meeting_follow_up_jev_assessment_id"] == nil || created.Context["meeting_follow_up_scope_evidence"] == nil {
		t.Fatalf("projection=%+v", created)
	}
	manager.existing = []model.CRMSuggestion{{ID: "existing"}}
	if err := processor.projectFollowUp(context.Background(), meeting, transcript, output); err != nil {
		t.Fatal(err)
	}
	if len(manager.created) != 1 || p.calls != 1 {
		t.Fatal("existing draft was regenerated or reassessed")
	}
}
