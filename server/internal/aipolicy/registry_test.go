package aipolicy

import "testing"

func TestDefaultRegistryIsValid(t *testing.T) {
	registry := DefaultRegistry()
	if err := registry.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(registry.Actions()) < 30 {
		t.Fatalf("Actions() count = %d, want app-wide registry", len(registry.Actions()))
	}
}

func TestCoverageActionsAreSupportAI(t *testing.T) {
	registry := DefaultRegistry()
	keys := []string{
		ActionSupportCoverageAnalyze,
		ActionSupportCoverageRefine,
		ActionSupportCoverageEmbed,
		ActionSupportCoverageAssignTopic,
	}
	for _, key := range keys {
		action, ok := registry.Lookup(key)
		if !ok {
			t.Fatalf("Lookup(%q) missing", key)
		}
		if action.Category != CategorySupportAI || action.Origin != "coverage" {
			t.Fatalf("Lookup(%q) category/origin = %q/%q, want %q/coverage",
				key, action.Category, action.Origin, CategorySupportAI)
		}
	}
}

func TestRegistryRejectsCoverageCategorizedAsDocsAI(t *testing.T) {
	registry := NewRegistry([]Action{{
		Key:             ActionSupportCoverageAnalyze,
		PolicyVersion:   "v1",
		FeatureKey:      "coverage_gap_analysis",
		Label:           "Coverage gap analysis",
		Category:        CategoryDocsAI,
		Origin:          "coverage",
		Modality:        ModalityChat,
		DefaultProvider: "anthropic",
		DefaultModel:    "claude-sonnet-4-6",
		AllowedModels:   map[string][]string{"anthropic": {"claude-sonnet-4-6"}},
		Timeout:         30,
		MaxOutputTokens: 1800,
		RetryClass:      RetryTransient,
		Autonomy:        AutonomyAnalyze,
		DataClass:       DataClassCustomerContent,
	}})
	if err := registry.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want coverage/Docs AI rejection")
	}
}

func TestRegistryRejectsInvalidActionDefinitions(t *testing.T) {
	tests := []struct {
		name   string
		action Action
	}{
		{name: "missing policy version", action: validTestAction(func(action *Action) { action.PolicyVersion = "" })},
		{name: "unsupported modality", action: validTestAction(func(action *Action) { action.Modality = "magic" })},
		{name: "default model not allowed", action: validTestAction(func(action *Action) { action.DefaultModel = "other" })},
		{name: "missing timeout", action: validTestAction(func(action *Action) { action.Timeout = 0 })},
		{name: "missing output ceiling", action: validTestAction(func(action *Action) { action.MaxOutputTokens = 0 })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewRegistry([]Action{tt.action}).Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func validTestAction(mutate func(*Action)) Action {
	action := Action{
		Key:             "support.test.v1",
		PolicyVersion:   "v1",
		FeatureKey:      "support_test",
		Label:           "Support test",
		Category:        CategorySupportAI,
		Origin:          "support",
		Modality:        ModalityChat,
		DefaultProvider: "anthropic",
		DefaultModel:    "claude-sonnet-4-6",
		AllowedModels:   map[string][]string{"anthropic": {"claude-sonnet-4-6"}},
		Timeout:         30,
		MaxOutputTokens: 1000,
		RetryClass:      RetryTransient,
		Autonomy:        AutonomyAssist,
		DataClass:       DataClassCustomerContent,
	}
	mutate(&action)
	return action
}
