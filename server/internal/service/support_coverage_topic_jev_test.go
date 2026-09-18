package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type captureSemanticAssignment struct {
	attempt *model.CoverageAssignmentAttempt
	topic   *model.CoverageTopicV2
}

func (c *captureSemanticAssignment) ApplySemanticTopicAssignment(_ context.Context, _ model.CoverageFinding, topic *model.CoverageTopicV2, attempt *model.CoverageAssignmentAttempt) error {
	c.topic, c.attempt = topic, attempt
	return nil
}
func TestJevCoverageTopicMatchesAreScopedAndReversible(t *testing.T) {
	for _, scenario := range []string{"same", "related", "ambiguous", "uncertain candidate", "shadow"} {
		t.Run(scenario, func(t *testing.T) {
			mode := "primary"
			if scenario == "shadow" {
				mode = scenario
			}
			decisions, p, _, _ := setupJevDecisionTest(t, mode)
			p.choices = map[string]string{"topic_0": "same_need", "topic_1": "distinct"}
			if scenario == "related" {
				p.choices["topic_0"] = "related"
			}
			if scenario == "uncertain candidate" {
				p.choices["topic_1"] = "uncertain"
			}
			if scenario == "ambiguous" {
				p.choices["topic_1"] = "same_need"
			}
			finding := &model.CoverageFinding{ID: "finding", WorkspaceID: "workspace", CustomerNeed: "発注をキャンセルする", Confidence: .9}
			topics := []model.CoverageTopicV2{{ID: "a", WorkspaceID: "workspace", Status: "open", CustomerNeed: "Cancel an order"}, {ID: "b", WorkspaceID: "workspace", Status: "open", CustomerNeed: "Update an account"}, {ID: "private", WorkspaceID: "other", Status: "open", CustomerNeed: "PRIVATE_OTHER_WORKSPACE"}, {ID: "archived", WorkspaceID: "workspace", Status: "archived", CustomerNeed: "ARCHIVED_TOPIC"}}
			writer := &captureSemanticAssignment{}
			handled, err := assignCoverageTopicWithJev(context.Background(), decisions, writer, finding, topics)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(p.states[0], "PRIVATE_OTHER_WORKSPACE") || strings.Contains(p.states[0], "ARCHIVED_TOPIC") {
				t.Fatal("inaccessible candidate sent")
			}
			if scenario == "shadow" {
				if handled || writer.attempt != nil {
					t.Fatal("shadow assigned a topic")
				}
				return
			}
			if !handled || writer.attempt == nil {
				t.Fatal("semantic assignment not handled")
			}
			want := model.CoverageAssignmentAttach
			if scenario == "related" {
				want = model.CoverageAssignmentCreate
			}
			if scenario == "ambiguous" || scenario == "uncertain candidate" {
				want = model.CoverageAssignmentReview
			}
			if writer.attempt.Outcome != want {
				t.Fatalf("outcome %s want %s", writer.attempt.Outcome, want)
			}
			if scenario == "same" && writer.topic.ID != "a" {
				t.Fatal("incorrect topic selected")
			}
			if scenario == "related" && writer.topic.CanonicalKey == aiUsageStableHash("jev-topic-v1:") {
				t.Fatal("non-Latin source collapsed into an empty key")
			}
			if !strings.Contains(string(writer.attempt.Metadata), "jev_assessment_id") {
				t.Fatal("assignment provenance missing")
			}
		})
	}
}
