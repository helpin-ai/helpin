package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
)

func TestJevCoverageClassificationPreservesGroundedGeneration(t *testing.T) {
	for _, scenario := range []string{"primary", "shadow", "off", "provider failure", "uncertain", "contradictory narrative"} {
		t.Run(scenario, func(t *testing.T) {
			mode := "primary"
			if scenario == "shadow" || scenario == "off" {
				mode = scenario
			}
			decisions, p, _, _ := setupJevDecisionTest(t, mode)
			p.choices = map[string]string{"conversation_type": "support_query", "gap_category": "action", "gap_kind": "action"}
			if scenario == "provider failure" {
				p.err = errors.New("unavailable")
			}
			if scenario == "uncertain" {
				p.choices["gap_category"] = "uncertain"
			}
			category, kind := "knowledge", "content"
			if scenario == "primary" {
				category, kind = "action", "action"
			}
			response := `{"conversation_type":"support_query","has_gap":true,"gap_category":"CATEGORY","gap_kind":"KIND","customer_need":"Cancel a scheduled job","ai_failure":"Agent lacked the cancellation action","decision_reason":"The human cancelled the job","recommended_fixes":[{"type":"add_action","target_type":"agent","rationale":"Cancellation needs an operation","suggested_change":"Add a guarded cancellation tool","priority":"primary"}],"confidence":0.81}`
			response = strings.ReplaceAll(strings.ReplaceAll(response, "CATEGORY", category), "KIND", kind)
			generator := &scriptedSupportPlannerLLM{responses: []llm.ChatResponse{{Content: response}}}
			analyzer := NewSupportCoverageDailyAnalyzer(generator, "test", "test").SetJevDecisions(decisions)
			result, _, err := analyzer.AnalyzeConversation(context.Background(), CoverageConversationAnalysisInput{WorkspaceID: "workspace", ConversationID: "conversation", Messages: []CoverageConversationMessage{{ID: "message", SenderType: "customer", MessageType: "reply", Content: "Cancel my scheduled job"}}})
			if scenario == "contradictory narrative" {
				if !errors.Is(err, errCoverageLLMContract) || result != nil {
					t.Fatalf("contradictory classification persisted: %+v %v", result, err)
				}
				return
			}
			if err != nil || result == nil || result.GapCategory != category || result.Confidence != .81 || result.RecommendedFixes[0].Priority != "primary" {
				t.Fatalf("generation changed unexpectedly: %+v %v", result, err)
			}
			if len(generator.requests) != 1 {
				t.Fatal("narrative generation skipped or duplicated")
			}
			constrained := strings.Contains(generator.requests[0].SystemPrompt, "server has classified")
			if constrained != (scenario == "primary") {
				t.Fatalf("unexpected primary guidance: %v", constrained)
			}
			if scenario == "off" && p.calls != 0 {
				t.Fatal("disabled classifier called")
			}
		})
	}
}
