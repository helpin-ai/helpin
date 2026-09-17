package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestJevFlowConditionGatesAndRecordsActivity(t *testing.T) {
	for _, tc := range []struct {
		name, mode, choice, outcome                     string
		probability                                     float64
		providerError, malformed, stale, wrongWorkspace bool
	}{
		{name: "match", mode: "primary", choice: "match", outcome: "matched", probability: .99},
		{name: "no match", mode: "primary", choice: "no_match", outcome: "no_match", probability: .99},
		{name: "uncertain", mode: "primary", choice: "uncertain", outcome: "uncertain", probability: .99},
		{name: "low confidence", mode: "primary", choice: "match", outcome: "uncertain", probability: .6},
		{name: "shadow", mode: "shadow", choice: "match", outcome: "shadow", probability: .99},
		{name: "off", mode: "off", choice: "match", outcome: "unavailable", probability: .99},
		{name: "provider error", mode: "primary", choice: "match", outcome: "unavailable", probability: .99, providerError: true},
		{name: "malformed", mode: "primary", choice: "match", outcome: "unavailable", probability: .99, malformed: true},
		{name: "edited rule", mode: "primary", choice: "match", outcome: "stale", probability: .99, stale: true},
		{name: "wrong workspace", mode: "primary", choice: "match", outcome: "unavailable", probability: .99, wrongWorkspace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decisions, provider, _, _ := setupJevDecisionTest(t, tc.mode)
			provider.choices["condition"] = tc.choice
			provider.probability = tc.probability
			provider.malformed = tc.malformed
			if tc.providerError {
				provider.err = errors.New("provider unavailable")
			}
			db := setupRuleEngineTestDB(t)
			if err := db.Exec(`CREATE TABLE agent_trigger_executions(id TEXT PRIMARY KEY,workspace_id TEXT,agent_id TEXT,binding_id TEXT,binding_kind TEXT,trigger_type TEXT,reference_id TEXT,reference_type TEXT,target_type TEXT,target_id TEXT,status TEXT,condition_outcome TEXT,condition_assessment_id TEXT,fired_at DATETIME,completed_at DATETIME,created_at DATETIME,updated_at DATETIME)`).Error; err != nil {
				t.Fatal(err)
			}
			repo := repository.NewAutomationRuleRepository(db)
			engine := NewAutomationRuleEngine(repo, nil, nil, nil, nil, nil, nil, nil).SetJevDecisions(decisions).SetTriggerExecutionRepository(repository.NewAgentTriggerExecutionRepository(db))
			actions := 0
			engine.SetCommandService(&InternalCommandService{definitions: map[string]InternalCommandDefinition{"test": {Name: "test", Execute: func(context.Context, model.InternalCommandContext, json.RawMessage) (json.RawMessage, error) {
				actions++
				return json.RawMessage(`{}`), nil
			}}}})
			rule := &model.AutomationRule{ID: "rule", WorkspaceID: "workspace", Name: "Release flow", Enabled: true, TriggerType: model.TriggerGitHubReleasePub, TriggerConfig: json.RawMessage(`{"repo_full_name":"helpin/api","semantic_condition":{"text":"The release concerns authentication"}}`), ActionType: model.ActionRunCommand, ActionConfig: json.RawMessage(`{"command_name":"test"}`)}
			if err := repo.Create(context.Background(), rule); err != nil {
				t.Fatal(err)
			}
			if tc.stale {
				if err := db.Model(rule).UpdateColumn("enabled", false).Error; err != nil {
					t.Fatal(err)
				}
			}
			event := model.AutomationEvent{WorkspaceID: "workspace", TriggerType: rule.TriggerType, RepoFullName: "helpin/api", ReleaseName: "Authentication fixes"}
			if tc.wrongWorkspace {
				event.WorkspaceID = "other"
			}
			got := engine.matchesSemanticCondition(context.Background(), rule, event, nil)
			if got != (tc.outcome == "matched") {
				t.Fatalf("allowed=%v", got)
			}
			var activity model.AgentTriggerExecution
			if err := db.First(&activity).Error; err != nil {
				t.Fatal(err)
			}
			if activity.ConditionOutcome == nil || *activity.ConditionOutcome != tc.outcome || activity.RunID != nil || activity.AgentID != "" {
				t.Fatalf("activity=%+v", activity)
			}
			actionErr := engine.executeAction(context.Background(), rule, event, nil, &model.RuleExecutionContext{MaxDepth: 10})
			if got {
				if actionErr != nil || actions != 1 {
					t.Fatalf("matching action count=%d err=%v", actions, actionErr)
				}
			} else if !errors.Is(actionErr, errFlowConditionSkipped) || actions != 0 {
				t.Fatalf("skipped condition executed action: %d %v", actions, actionErr)
			}
			if tc.wrongWorkspace && provider.calls != 0 {
				t.Fatal("cross-workspace evidence disclosed")
			}
			if tc.name == "no match" {
				rule.StopOnMatch = true
				if err := repo.Update(context.Background(), rule); err != nil {
					t.Fatal(err)
				}
				next := &model.AutomationRule{ID: "next", WorkspaceID: "workspace", Name: "Next Flow", Enabled: true, Position: 1, TriggerType: rule.TriggerType, TriggerConfig: json.RawMessage(`{"repo_full_name":"helpin/api"}`), ActionType: rule.ActionType, ActionConfig: rule.ActionConfig}
				if err := repo.Create(context.Background(), next); err != nil {
					t.Fatal(err)
				}
				engine.EvaluateEvent(context.Background(), event, nil)
				if actions != 1 {
					t.Fatalf("unmatched StopOnMatch blocked the next Flow: actions=%d", actions)
				}
			}
			if got { // Identical checks reuse the Jev assessment; activity still represents each event.
				if !engine.matchesSemanticCondition(context.Background(), rule, event, nil) || provider.calls != 1 {
					t.Fatal("matching result was not cached")
				}
			}
		})
	}
}

func TestJevFlowConditionValidationAndLegacyParity(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, config string
		wantErr               bool
	}{
		{"legacy", model.TriggerTaskStateEntered, `{"state_id":"state"}`, false},
		{"empty disables", model.TriggerCron, `{"semantic_condition":{"text":" "}}`, false},
		{"schedule", model.TriggerCron, `{"semantic_condition":{"text":"a task is a bug"}}`, true},
		{"malformed", model.TriggerTaskStateEntered, `{"semantic_condition":"yes"}`, true},
		{"long", model.TriggerTaskStateEntered, `{"semantic_condition":{"text":"` + strings.Repeat("x", 501) + `"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSemanticFlowCondition(tc.trigger, json.RawMessage(tc.config))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
	engine := NewAutomationRuleEngine(nil, nil, nil, nil, nil, nil, nil, nil)
	if !engine.matchesSemanticCondition(context.Background(), &model.AutomationRule{TriggerType: model.TriggerTaskStateEntered, TriggerConfig: json.RawMessage(`{}`)}, model.AutomationEvent{}, nil) {
		t.Fatal("legacy Flow gated")
	}
}

func TestJevFlowConditionCRUDRoundTrip(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	repo := repository.NewAutomationRuleRepository(db)
	engine := NewAutomationRuleEngine(repo, nil, nil, nil, nil, nil, nil, nil)
	config := json.RawMessage(`{"repo_full_name":"helpin/api","semantic_condition":{"text":"Authentication release"}}`)
	created, err := engine.CreateRuleForActor(context.Background(), "workspace", "actor", model.CreateAutomationRuleRequest{Name: "Release", TriggerType: model.TriggerGitHubReleasePub, TriggerConfig: config, ActionType: model.ActionRunCommand, ActionConfig: json.RawMessage(`{"command_name":"test"}`)})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := engine.GetRule(context.Background(), "workspace", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := parseSemanticFlowCondition(saved.TriggerType, saved.TriggerConfig)
	if err != nil || condition == nil || condition.Text != "Authentication release" {
		t.Fatalf("saved condition=%+v %v", condition, err)
	}
	invalid := json.RawMessage(`{"repo_full_name":"helpin/api","semantic_condition":"invalid"}`)
	if _, err := engine.UpdateRule(context.Background(), "workspace", created.ID, model.UpdateAutomationRuleRequest{TriggerConfig: &invalid}); err == nil {
		t.Fatal("malformed condition saved")
	}
	saved, err = engine.GetRule(context.Background(), "workspace", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved.TriggerConfig) != string(config) {
		t.Fatal("failed update changed saved condition")
	}
	removed := json.RawMessage(`{"repo_full_name":"helpin/api"}`)
	if _, err := engine.UpdateRule(context.Background(), "workspace", created.ID, model.UpdateAutomationRuleRequest{TriggerConfig: &removed}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.UpdateRule(context.Background(), "other", created.ID, model.UpdateAutomationRuleRequest{TriggerConfig: &config}); err == nil {
		t.Fatal("cross-workspace update accepted")
	}
}
