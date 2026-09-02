package service

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentRuntimeProviderCatalogIncludesAllCommandTools(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil), nil)
	catalog, err := host.ListProviderTools()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]agentruntime.Tool{}
	for _, tool := range catalog.Tools {
		byName[tool.Name] = tool
		if len(tool.SupportedTargetTypes) != 0 {
			t.Fatalf("provider must not publish non-authoritative target metadata: %#v", tool)
		}
	}
	if _, ok := byName["create_collection"]; !ok {
		t.Fatal("create_collection is missing from the provider catalog")
	}
	crmSignals, ok := byName["list_crm_signals"]
	if !ok || !slices.Contains(crmSignals.Aliases, "list_buyer_signals") {
		t.Fatalf("legacy CRM alias is missing: %#v", crmSignals)
	}
}

func TestAgentRuntimeProviderCallReturnsStructuredCommandOutput(t *testing.T) {
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commands.register(InternalCommandDefinition{
		Name: "test.provider_echo",
		Tool: &commandtools.RuntimeToolMetadata{
			CommandName: "test.provider_echo", Alias: "provider_echo", Category: "Test",
			Description: "Echo input.", InputSchema: map[string]any{"type": "object"},
		},
		Execute: func(_ context.Context, _ model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			return input, nil
		},
	})
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commands, nil)
	result, err := host.CallProviderTool(context.Background(), agentruntime.ProviderToolCallRequest{
		ToolName: "provider_echo",
		Input:    json.RawMessage(`{"ok":true}`),
		Meta:     agentruntime.CommandExecutionContext{AppID: "helpin", WorkspaceID: "workspace-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || string(result.StructuredContent) != `{"ok":true}` {
		t.Fatalf("unexpected provider result: %#v", result)
	}
}

func TestProviderCatalogValidationChecksUnfilteredToolBlocks(t *testing.T) {
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commands.register(InternalCommandDefinition{
		Name: "test.invalid_tool", Tool: &commandtools.RuntimeToolMetadata{CommandName: "test.invalid_tool"},
		Execute: func(context.Context, model.InternalCommandContext, json.RawMessage) (json.RawMessage, error) {
			return nil, nil
		},
	})
	if err := validateProviderCommandDefinitions(commands); err == nil {
		t.Fatal("expected an empty alias in a non-exposed tool block to fail validation")
	}
}

func TestProviderCatalogValidationRejectsInvalidModelContracts(t *testing.T) {
	executor := func(context.Context, model.InternalCommandContext, json.RawMessage) (json.RawMessage, error) {
		return nil, nil
	}
	tests := map[string]*commandtools.RuntimeToolMetadata{
		"missing schema": {CommandName: "test.invalid_tool", Alias: "invalid_tool"},
		"invalid schema": {CommandName: "test.invalid_tool", Alias: "invalid_tool", InputSchema: map[string]any{"type": "object", "invalid": make(chan int)}},
		"legacy alias":   {CommandName: "test.invalid_tool", Alias: "add_story_comment", InputSchema: map[string]any{"type": "object"}},
		"prefixed alias": {CommandName: "test.invalid_tool", Alias: "mcp__helpin__invalid_tool", InputSchema: map[string]any{"type": "object"}},
	}
	for name, metadata := range tests {
		t.Run(name, func(t *testing.T) {
			commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
			commands.register(InternalCommandDefinition{Name: "test.invalid_tool", Tool: metadata, Execute: executor})
			if err := validateProviderCommandDefinitions(commands); err == nil {
				t.Fatal("expected invalid model contract to fail validation")
			}
		})
	}
}

func TestBuiltInPresetHelpinToolsResolveAgainstProviderCatalog(t *testing.T) {
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commands, nil)
	catalog, err := host.ListProviderTools()
	if err != nil {
		t.Fatal(err)
	}
	providerNames := make(map[string]bool, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		providerNames[tool.Name] = true
	}
	helpinOwned := map[string]bool{}
	for _, metadata := range commandtools.AllRuntimeToolMetadata() {
		helpinOwned[agentcontract.CanonicalToolName(metadata.Alias)] = true
	}
	for _, preset := range agentPresetDefinitions() {
		for _, allowed := range preset.AllowedTools {
			canonical := agentcontract.CanonicalToolName(allowed)
			if helpinOwned[canonical] && !providerNames[canonical] {
				t.Errorf("preset %q allows Helpin tool %q, but the provider catalog does not expose it", preset.Key, canonical)
			}
		}
	}
}
