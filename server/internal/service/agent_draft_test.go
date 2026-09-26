package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeAgentDraftLLM struct {
	content string
	err     error
	calls   int
}

func (f *fakeAgentDraftLLM) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.content}, nil
}

func TestValidateCustomAgentDraftDropsUnknownCatalogValuesAndAddsRequiredSkillTools(t *testing.T) {
	raw := model.CustomAgentDraft{
		Name:                  " Support agent ",
		Role:                  " ",
		SystemPrompt:          "Help customers.",
		AllowedTargets:        []string{"support_conversation", "made_up_target"},
		AllowedTools:          []string{"search_documents", "made_up_tool"},
		Skills:                model.AgentSkillRefs{{Key: "support_style"}, {Key: "missing_skill"}},
		ApprovalMode:          "banana",
		ModelTier:             "banana_tier",
		DefaultInvocationMode: "banana_mode",
		MaxConcurrentRuns:     0,
	}
	tools := []model.ToolCatalogEntry{
		{Name: "search_documents", Description: "Search docs"},
		{Name: "read_document", Description: "Read docs"},
	}
	skills := []model.SkillCatalogEntry{
		{Key: "support_style", Title: "Support style", RequiredTools: []string{"read_document"}},
	}

	draft, warnings := validateCustomAgentDraft(raw, tools, skills)

	if draft.Name != "Support agent" {
		t.Fatalf("expected trimmed name, got %q", draft.Name)
	}
	if draft.Role != "Custom Agent" {
		t.Fatalf("expected default role, got %q", draft.Role)
	}
	if !slices.Equal(draft.AllowedTargets, []string{"support_conversation"}) {
		t.Fatalf("expected valid target only, got %v", draft.AllowedTargets)
	}
	if !slices.Equal(draft.AllowedTools, []string{"search_documents", "read_document"}) {
		t.Fatalf("expected selected plus required tools, got %v", draft.AllowedTools)
	}
	if len(draft.Skills) != 1 || draft.Skills[0].Key != "support_style" {
		t.Fatalf("expected valid skill only, got %#v", draft.Skills)
	}
	if draft.ApprovalMode != "risk_based" {
		t.Fatalf("expected risk-based approval default, got %q", draft.ApprovalMode)
	}
	if draft.ModelTier != "small" {
		t.Fatalf("expected small model tier default, got %q", draft.ModelTier)
	}
	if draft.DefaultInvocationMode != "interactive" {
		t.Fatalf("expected interactive default, got %q", draft.DefaultInvocationMode)
	}
	if draft.MaxConcurrentRuns != 1 {
		t.Fatalf("expected max concurrent fallback, got %d", draft.MaxConcurrentRuns)
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"made_up_tool", "missing_skill", "made_up_target"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected warning mentioning %q, got %v", want, warnings)
		}
	}
}

func TestCommandBarAgentDraftApprovalDefault(t *testing.T) {
	for _, test := range []struct {
		name string
		mode string
		want string
	}{
		{name: "omitted", want: "risk_based"},
		{name: "explicit", mode: "mutating_tools", want: "mutating_tools"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := commandBarCreateAgentRequestFromDraft("workspace-1", model.CustomAgentDraft{
				Name: "Custom agent", ApprovalMode: test.mode,
			}, model.ConfirmCommandBarChatProposalRequest{})
			if req.ApprovalMode == nil || *req.ApprovalMode != test.want {
				t.Fatalf("approval mode = %v, want %q", req.ApprovalMode, test.want)
			}
		})
	}
}

func TestValidateCustomAgentDraftRetainsPMTargets(t *testing.T) {
	raw := model.CustomAgentDraft{
		AllowedTargets: []string{"sprint", "objective"},
	}

	draft, warnings := validateCustomAgentDraft(raw, nil, nil)

	if !slices.Equal(draft.AllowedTargets, []string{"sprint", "objective"}) {
		t.Fatalf("expected sprint and objective targets to be retained, got %v", draft.AllowedTargets)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no target normalization warnings, got %v", warnings)
	}
}

func TestDraftCustomAgentRejectsShortDescriptionBeforeCallingLLM(t *testing.T) {
	llmClient := &fakeAgentDraftLLM{content: `{}`}
	svc := (&AgentService{}).SetAgentDraftLLM(llmClient)

	_, err := svc.DraftCustomAgent(context.Background(), "ws-1", model.CustomAgentDraftRequest{Description: "hi"})

	if err == nil {
		t.Fatal("expected validation error")
	}
	if llmClient.calls != 0 {
		t.Fatalf("expected no LLM call, got %d", llmClient.calls)
	}
}

func TestDraftCustomAgentReturnsValidatedDraftWithoutCreatingAgent(t *testing.T) {
	raw := model.CustomAgentDraftLLMResponse{
		Draft: model.CustomAgentDraft{
			Name:                  "Docs Helper",
			SystemPrompt:          "Answer from docs.",
			AllowedTargets:        []string{"document"},
			AllowedTools:          []string{"search_documents", "unknown_tool"},
			ApprovalMode:          "never",
			ModelTier:             "small",
			DefaultInvocationMode: "interactive",
			MaxConcurrentRuns:     2,
		},
		Reasons: []model.CustomAgentDraftReason{{Field: "allowed_tools", Value: "search_documents", Reason: "Needs docs search."}},
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	llmClient := &fakeAgentDraftLLM{content: string(payload)}
	svc := (&AgentService{}).SetAgentDraftLLM(llmClient)

	result, err := svc.DraftCustomAgentWithCatalog(
		context.Background(),
		"ws-1",
		model.CustomAgentDraftRequest{Description: "Create an agent that answers questions from docs."},
		[]model.ToolCatalogEntry{{Name: "search_documents", Description: "Search docs"}},
		nil,
	)

	if err != nil {
		t.Fatalf("draft failed: %v", err)
	}
	if result.Draft.Name != "Docs Helper" {
		t.Fatalf("expected draft name, got %q", result.Draft.Name)
	}
	if !slices.Equal(result.Draft.AllowedTools, []string{"search_documents"}) {
		t.Fatalf("expected unknown tool removed, got %v", result.Draft.AllowedTools)
	}
	if len(result.Reasons) != 1 {
		t.Fatalf("expected reasons preserved, got %#v", result.Reasons)
	}
	if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, "\n"), "unknown_tool") {
		t.Fatalf("expected unknown tool warning, got %v", result.Warnings)
	}
	if llmClient.calls != 1 {
		t.Fatalf("expected one LLM call, got %d", llmClient.calls)
	}
}
