package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
)

// AgentRuntimeToolCatalog is the simple HTTP MCP provider list envelope.
type AgentRuntimeToolCatalog struct {
	Tools []agentruntime.Tool `json:"tools"`
}

// ListProviderTools returns the complete validated Helpin command-tool catalog.
func (s *AgentRuntimeHostService) ListProviderTools() (*AgentRuntimeToolCatalog, error) {
	if s == nil || s.commandService == nil {
		return nil, fmt.Errorf("command service is not configured")
	}
	if err := validateProviderCommandDefinitions(s.commandService); err != nil {
		return nil, err
	}
	definitions := s.commandService.ToolDefinitions()
	tools := make([]agentruntime.Tool, 0, len(definitions))
	for _, definition := range definitions {
		metadata := definition.Tool
		schema, err := json.Marshal(metadata.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal tool %q schema: %w", metadata.Alias, err)
		}
		tools = append(tools, agentruntime.Tool{
			Name:        strings.TrimSpace(metadata.Alias),
			Description: strings.TrimSpace(metadata.Description),
			Category:    strings.TrimSpace(metadata.Category),
			InputSchema: schema,
			Mutating:    definition.Mutating,
			RiskLevel:   definition.RiskLevel(),
			Aliases:     agentcontract.LegacyToolAliases(metadata.Alias),
			// SupportedTargetTypes is deliberately not published. Helpin's
			// field is context metadata, not an authorization boundary.
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return &AgentRuntimeToolCatalog{Tools: tools}, nil
}

// CallProviderTool resolves a model-facing alias and executes its backing command.
func (s *AgentRuntimeHostService) CallProviderTool(ctx context.Context, req agentruntime.ProviderToolCallRequest) (*agentruntime.ToolCallResult, error) {
	if s == nil || s.commandService == nil {
		return nil, fmt.Errorf("command service is not configured")
	}
	canonical := agentcontract.CanonicalToolName(req.ToolName)
	commandName := s.providerCommands[canonical]
	if commandName == "" {
		return &agentruntime.ToolCallResult{Content: []agentruntime.ContentItem{{Type: "text", Text: fmt.Sprintf("unknown Helpin tool: %s", strings.TrimSpace(req.ToolName))}}, IsError: true}, nil
	}
	response, err := s.ExecuteCommand(ctx, agentruntime.CommandExecutionRequest{Meta: req.Meta, CommandName: commandName, Input: req.Input})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Error) != "" {
		return &agentruntime.ToolCallResult{Content: []agentruntime.ContentItem{{Type: "text", Text: response.Error}}, IsError: true}, nil
	}
	structured := response.Output
	if len(structured) == 0 {
		structured = json.RawMessage(`{}`)
	}
	return &agentruntime.ToolCallResult{StructuredContent: structured}, nil
}

func validateProviderCommandDefinitions(commands *InternalCommandService) error {
	seenAliases := map[string]string{}
	for name, definition := range commands.definitions {
		if definition.Tool == nil {
			continue
		}
		alias := strings.TrimSpace(definition.Tool.Alias)
		if alias == "" {
			return fmt.Errorf("command %q has a tool block with an empty alias", name)
		}
		if definition.Execute == nil {
			return fmt.Errorf("tool %q has no executable command", alias)
		}
		if strings.HasPrefix(alias, "mcp__") || agentcontract.CanonicalToolName(alias) != alias {
			return fmt.Errorf("tool alias %q is not a bare canonical name", alias)
		}
		if definition.Tool.InputSchema == nil {
			return fmt.Errorf("tool %q has no input schema", alias)
		}
		if _, err := json.Marshal(definition.Tool.InputSchema); err != nil {
			return fmt.Errorf("tool %q has an invalid input schema: %w", alias, err)
		}
		if commandName := strings.TrimSpace(definition.Tool.CommandName); commandName != "" && commandName != name {
			return fmt.Errorf("tool %q points to command %q instead of %q", alias, commandName, name)
		}
		if existing := seenAliases[alias]; existing != "" && existing != name {
			return fmt.Errorf("tool alias %q is shared by commands %q and %q", alias, existing, name)
		}
		seenAliases[alias] = name
		for _, legacyAlias := range agentcontract.LegacyToolAliases(alias) {
			if existing := seenAliases[legacyAlias]; existing != "" && existing != name {
				return fmt.Errorf("legacy tool alias %q is shared by commands %q and %q", legacyAlias, existing, name)
			}
			seenAliases[legacyAlias] = name
		}
	}
	return nil
}

func providerCommandNames(commands *InternalCommandService) map[string]string {
	byAlias := map[string]string{}
	if commands == nil || validateProviderCommandDefinitions(commands) != nil {
		return byAlias
	}
	for name, definition := range commands.definitions {
		if definition.Tool == nil {
			continue
		}
		alias := agentcontract.CanonicalToolName(definition.Tool.Alias)
		if alias != "" {
			byAlias[alias] = name
		}
	}
	return byAlias
}
