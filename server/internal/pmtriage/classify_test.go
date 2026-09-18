package pmtriage

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/decision"
)

type providerFunc func(context.Context, string, map[string]decision.Question) (*decision.Result, error)

func (f providerFunc) DecideMany(ctx context.Context, state string, questions map[string]decision.Question) (*decision.Result, error) {
	return f(ctx, state, questions)
}

func testInput() Input {
	return Input{SourceKind: "task", SourceID: "source", Text: "CSV export crashes when exporting invoices.", Teams: []Option{{ID: "payments", Name: "Payments"}}, Labels: []Option{{ID: "export", Name: "Export"}}, Candidates: []Candidate{{ID: "same", Name: "Invoice CSV crash"}, {ID: "related", Name: "Add invoice export formats"}}}
}
func resultFor(questions map[string]decision.Question, choices map[string]string, probability float64) *decision.Result {
	result := &decision.Result{Model: decision.Model, Answers: map[string]decision.Answer{}, InputTokens: 100, OutputTokens: 20}
	for key, question := range questions {
		choice := choices[key]
		probabilities := map[string]float64{}
		for id := range question.Choices {
			probabilities[id] = (1 - probability) / float64(len(question.Choices)-1)
		}
		probabilities[choice] = probability
		result.Answers[key] = decision.Answer{Choice: choice, Probabilities: probabilities, ProviderConfidence: 1}
	}
	return result
}
func positiveChoices() map[string]string {
	return map[string]string{"actionable": "yes", "task_type": "bug", "team": "payments", "label_0": "yes", "candidate_0": "duplicates", "candidate_1": "relates_to"}
}

func TestAssessmentKeepsRelationshipsDistinct(t *testing.T) {
	request, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	assessment, usage, err := request.Evaluate(context.Background(), providerFunc(func(_ context.Context, state string, q map[string]decision.Question) (*decision.Result, error) {
		calls++
		if !strings.Contains(state, "CSV export") {
			t.Fatal("source evidence missing")
		}
		return resultFor(q, positiveChoices(), .98), nil
	}), .95)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || usage.InputTokens != 100 || !assessment.Actionable || assessment.TaskType.ID != "bug" || assessment.Team.ID != "payments" {
		t.Fatalf("unexpected assessment: %+v", assessment)
	}
	if len(assessment.Labels) != 1 || assessment.Labels[0].ID != "export" || len(assessment.Matches) != 2 || assessment.Matches[0].Relationship != "duplicates" || assessment.Matches[1].Relationship != "relates_to" {
		t.Fatalf("lost taxonomy or relationship: %+v", assessment)
	}
}
func TestAssessmentAbstainsDespiteProviderConfidence(t *testing.T) {
	request, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	assessment, _, err := request.Evaluate(context.Background(), providerFunc(func(_ context.Context, _ string, q map[string]decision.Question) (*decision.Result, error) {
		result := resultFor(q, positiveChoices(), .8)
		result.Answers["actionable"] = decision.Answer{Choice: "yes", Probabilities: map[string]float64{"yes": .99, "no": .01}}
		return result, nil
	}), .95)
	if err != nil {
		t.Fatal(err)
	}
	if !assessment.Actionable || assessment.TaskType != nil || assessment.Team != nil || len(assessment.Labels) != 0 || len(assessment.Matches) != 0 {
		t.Fatalf("low probability decisions escaped review threshold: %+v", assessment)
	}
}
func TestNonActionableSourceHasNoSuggestions(t *testing.T) {
	request, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	assessment, _, err := request.Evaluate(context.Background(), providerFunc(func(_ context.Context, _ string, q map[string]decision.Question) (*decision.Result, error) {
		choices := positiveChoices()
		choices["actionable"] = "no"
		return resultFor(q, choices, .99), nil
	}), .95)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Actionable || assessment.TaskType != nil || assessment.Team != nil || len(assessment.Labels) != 0 || len(assessment.Matches) != 0 {
		t.Fatalf("non-actionable source generated work: %+v", assessment)
	}
}
func TestBuildRejectsInvalidContext(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Input)
	}{
		{"unknown source", func(i *Input) { i.SourceKind = "email" }},
		{"empty evidence", func(i *Input) { i.Text = " " }},
		{"oversized evidence", func(i *Input) { i.Text = strings.Repeat("x", 8001) }},
		{"oversized context", func(i *Input) { i.Candidates[0].Description = strings.Repeat("x", 16000) }},
		{"duplicate candidate", func(i *Input) { i.Candidates = append(i.Candidates, i.Candidates[0]) }},
		{"self candidate", func(i *Input) { i.Candidates[0].ID = i.SourceID }},
		{"reserved team", func(i *Input) { i.Teams[0].ID = "unknown" }},
		{"duplicate label", func(i *Input) { i.Labels = append(i.Labels, i.Labels[0]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := testInput()
			tc.change(&input)
			if _, err := Build(input); err == nil {
				t.Fatal("accepted invalid context")
			}
		})
	}
}
func TestAssessmentRejectsMalformedProviderResults(t *testing.T) {
	cases := []struct {
		name   string
		change func(*decision.Result)
	}{
		{"wrong model", func(r *decision.Result) { r.Model = "other" }},
		{"missing answer", func(r *decision.Result) { delete(r.Answers, "team") }},
		{"invented ID", func(r *decision.Result) { a := r.Answers["team"]; a.Choice = "other-team"; r.Answers["team"] = a }},
		{"nan", func(r *decision.Result) { r.Answers["team"].Probabilities["payments"] = math.NaN() }},
		{"bad sum", func(r *decision.Result) { r.Answers["team"].Probabilities["payments"] = .5 }},
		{"not highest", func(r *decision.Result) { a := r.Answers["team"]; a.Choice = "unknown"; r.Answers["team"] = a }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request, err := Build(testInput())
			if err != nil {
				t.Fatal(err)
			}
			assessment, usage, err := request.Evaluate(context.Background(), providerFunc(func(_ context.Context, _ string, q map[string]decision.Question) (*decision.Result, error) {
				result := resultFor(q, positiveChoices(), .99)
				tc.change(result)
				return result, nil
			}), .95)
			if err == nil || assessment != nil || usage == nil {
				t.Fatal("malformed result accepted or usage lost")
			}
		})
	}
}
func TestAssessmentProviderFailure(t *testing.T) {
	request, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("provider unavailable")
	assessment, _, err := request.Evaluate(context.Background(), providerFunc(func(context.Context, string, map[string]decision.Question) (*decision.Result, error) {
		return nil, failure
	}), .95)
	if !errors.Is(err, failure) || assessment != nil {
		t.Fatal("provider failure converted into a decision")
	}
}
