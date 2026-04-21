package worker

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildAndLoadSkillArchiveRoundTrip(t *testing.T) {
	allowImplicit := false
	original := SkillDefinition{
		Key:               "repo_planner",
		Title:             "Repo Planner",
		Description:       "Use when planning implementation work for a repository.",
		Instructions:      "Follow the repository planning workflow and stop at review checkpoints.",
		RequiredTools:     []string{"request_review_checkpoint", "publish_task_plan"},
		SupportedRuntimes: []string{"codex", "native_sdk"},
		Interface: SkillInterface{
			DisplayName:      "Repository Planner",
			ShortDescription: "Guides implementation planning for repository work.",
			DefaultPrompt:    "Plan the work before coding.",
		},
		Policy: SkillPolicy{
			AllowImplicitInvocation: &allowImplicit,
			CompletionRequiresInteractionKinds: []string{
				"review_checkpoint",
				"request_user_input",
			},
			InteractionContracts: []SkillInteractionContract{
				{
					Kind:   "review_checkpoint",
					Schema: "review_checkpoint_v1",
					Transports: map[string]SkillInteractionTransport{
						"native_sdk": {Type: InteractionTransportTypeToolCall, ToolName: "request_review_checkpoint"},
						"codex":      {Type: InteractionTransportTypeFencedJSON, BlockLabel: "helpin-review"},
					},
				},
				{
					Kind:   "request_user_input",
					Schema: "request_user_input_v1",
					Transports: map[string]SkillInteractionTransport{
						"native_sdk": {Type: InteractionTransportTypeToolCall, ToolName: "request_user_input"},
						"codex":      {Type: InteractionTransportTypeRuntimeBridge},
					},
				},
			},
		},
	}

	archive, checksum, filename, err := BuildSkillArchive(original)
	if err != nil {
		t.Fatalf("BuildSkillArchive returned error: %v", err)
	}
	if checksum == "" {
		t.Fatal("expected checksum")
	}
	if filename != "repo_planner.zip" {
		t.Fatalf("expected repo_planner.zip, got %q", filename)
	}

	loaded, err := LoadSkillArchive(archive, model.WorkspaceSkillSourceImported)
	if err != nil {
		t.Fatalf("LoadSkillArchive returned error: %v", err)
	}

	if loaded.Key != original.Key {
		t.Fatalf("expected key %q, got %q", original.Key, loaded.Key)
	}
	if loaded.Title != "Repository Planner" {
		t.Fatalf("expected display name override, got %q", loaded.Title)
	}
	if loaded.Description != original.Interface.ShortDescription {
		t.Fatalf("expected short description override, got %q", loaded.Description)
	}
	if loaded.Instructions != original.Instructions {
		t.Fatalf("expected instructions %q, got %q", original.Instructions, loaded.Instructions)
	}
	if loaded.SourceKind != model.WorkspaceSkillSourceImported {
		t.Fatalf("expected source kind %q, got %q", model.WorkspaceSkillSourceImported, loaded.SourceKind)
	}
	if !containsString(loaded.RequiredTools, "request_review_checkpoint") || !containsString(loaded.RequiredTools, "publish_task_plan") {
		t.Fatalf("expected required tools to round-trip, got %v", loaded.RequiredTools)
	}
	if !containsString(loaded.SupportedRuntimes, "codex") || !containsString(loaded.SupportedRuntimes, "native_sdk") {
		t.Fatalf("expected runtimes to round-trip, got %v", loaded.SupportedRuntimes)
	}
	if loaded.Interface.DisplayName != original.Interface.DisplayName {
		t.Fatalf("expected interface display name %q, got %q", original.Interface.DisplayName, loaded.Interface.DisplayName)
	}
	if loaded.Interface.DefaultPrompt != original.Interface.DefaultPrompt {
		t.Fatalf("expected default prompt %q, got %q", original.Interface.DefaultPrompt, loaded.Interface.DefaultPrompt)
	}
	if loaded.Policy.AllowImplicitInvocation == nil || *loaded.Policy.AllowImplicitInvocation != false {
		t.Fatalf("expected allow_implicit_invocation=false, got %+v", loaded.Policy.AllowImplicitInvocation)
	}
	if !containsString(loaded.Policy.CompletionRequiresInteractionKinds, "review_checkpoint") || !containsString(loaded.Policy.CompletionRequiresInteractionKinds, "request_user_input") {
		t.Fatalf("expected completion interaction policy to round-trip, got %+v", loaded.Policy.CompletionRequiresInteractionKinds)
	}
	contract, ok := loaded.Policy.InteractionContract("review_checkpoint")
	if !ok {
		t.Fatal("expected review_checkpoint interaction contract to round-trip")
	}
	if contract.Schema != "review_checkpoint_v1" {
		t.Fatalf("expected review checkpoint schema to round-trip, got %q", contract.Schema)
	}
	if contract.Transports["native_sdk"].ToolName != "request_review_checkpoint" {
		t.Fatalf("expected native_sdk tool transport to round-trip, got %+v", contract.Transports["native_sdk"])
	}
	if contract.Transports["codex"].BlockLabel != "helpin-review" {
		t.Fatalf("expected codex fenced block label to round-trip, got %+v", contract.Transports["codex"])
	}
	inputContract, ok := loaded.Policy.InteractionContract("request_user_input")
	if !ok {
		t.Fatal("expected request_user_input interaction contract to round-trip")
	}
	if inputContract.Schema != "request_user_input_v1" {
		t.Fatalf("expected request user input schema to round-trip, got %q", inputContract.Schema)
	}
	if inputContract.Transports["native_sdk"].ToolName != "request_user_input" {
		t.Fatalf("expected native_sdk request user input tool transport to round-trip, got %+v", inputContract.Transports["native_sdk"])
	}
	if inputContract.Transports["codex"].Type != InteractionTransportTypeRuntimeBridge {
		t.Fatalf("expected codex request user input runtime bridge transport to round-trip, got %+v", inputContract.Transports["codex"])
	}
}
