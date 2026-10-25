package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// supportHandoffFinding is one "what I checked" line as the AI wrote it.
type supportHandoffFinding struct {
	Tool    string `json:"tool"`
	Finding string `json:"finding"`
}

// supportHandoffChecks ties the AI's findings to the sources this run really
// used, so a teammate never sees a check the AI did not do. Findings that name
// an unused tool are dropped; sources used without a finding are listed alone.
// known is false when the run's tool calls could not be read.
func (s *InternalCommandService) supportHandoffChecks(ctx context.Context, meta model.InternalCommandContext, findings []supportHandoffFinding) (checks []SupportHandoffCheck, used map[string]bool, known bool) {
	sources, known := s.supportHandoffSources(ctx, meta)
	covered := map[string]bool{}
	for _, item := range findings {
		finding := strings.TrimSpace(item.Finding)
		if finding == "" {
			continue
		}
		tool := supportHandoffToolAlias(item.Tool)
		if tool == "" || !known {
			checks = append(checks, SupportHandoffCheck{Finding: finding})
			continue
		}
		label, ok := sources[tool]
		if !ok {
			continue
		}
		checks = append(checks, SupportHandoffCheck{Source: label, Finding: finding})
		covered[label] = true
	}
	used = map[string]bool{}
	var uncovered []string
	for tool, label := range sources {
		used[tool] = true
		if label != "" && !covered[label] {
			uncovered = append(uncovered, label)
			covered[label] = true
		}
	}
	sort.Strings(uncovered)
	for _, label := range uncovered {
		checks = append(checks, SupportHandoffCheck{Source: label})
	}
	return checks, used, known
}

// supportHandoffSources maps each tool this run called successfully to the
// source name a teammate sees. Tools that only read this conversation map to "".
func (s *InternalCommandService) supportHandoffSources(ctx context.Context, meta model.InternalCommandContext) (map[string]string, bool) {
	if s.agentService == nil {
		return nil, false
	}
	client, ok := s.agentService.agentRuntimeClient.(supportMCPToolCallReader)
	if !ok {
		return nil, false
	}
	run, err := s.resolveCommandRun(ctx, meta)
	if err != nil || run == nil {
		return nil, false
	}
	runtimeID, ok := agentRuntimeRunID(run)
	if !ok {
		return nil, false
	}
	calls, err := client.ListToolCalls(ctx, runtimeID)
	if err != nil {
		slog.WarnContext(ctx, "load handoff tool calls", "run_id", run.ID, "error", err)
		return nil, false
	}
	var servers map[string]string
	sources := map[string]string{}
	for _, call := range calls {
		if call.RunID != runtimeID || call.Error != "" {
			continue
		}
		tool := supportHandoffToolAlias(call.ToolName)
		if _, seen := sources[tool]; seen || tool == "" {
			continue
		}
		label := supportHandoffSourceLabel(tool)
		if serverName, ok := externalMCPToolServer(tool); ok {
			if servers == nil {
				servers = s.externalMCPServerLabels(ctx, meta.WorkspaceID)
			}
			label = servers[serverName]
			if label == "" {
				label = strings.ReplaceAll(serverName, "_", " ")
			}
		}
		sources[tool] = label
	}
	return sources, true
}

func (s *InternalCommandService) externalMCPServerLabels(ctx context.Context, workspaceID string) map[string]string {
	labels := map[string]string{}
	if s.agentService.externalMCPService == nil || s.agentService.externalMCPService.repo == nil {
		return labels
	}
	servers, err := s.agentService.externalMCPService.repo.ListServers(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "load MCP server names for handoff", "workspace_id", workspaceID, "error", err)
		return labels
	}
	for _, server := range servers {
		labels[server.ServerName] = strings.TrimSpace(server.Name)
	}
	return labels
}

// supportHandoffToolAlias accepts both the runtime name of a built-in tool
// (mcp__helpin__search_knowledge) and the alias the AI sees (search_knowledge).
func supportHandoffToolAlias(tool string) string {
	return strings.TrimPrefix(strings.TrimSpace(tool), "mcp__helpin__")
}

func externalMCPToolServer(tool string) (string, bool) {
	rest, ok := strings.CutPrefix(tool, "mcp__")
	if !ok {
		return "", false
	}
	server, _, ok := strings.Cut(rest, "__")
	return server, ok && server != ""
}

func supportHandoffSourceLabel(tool string) string {
	switch tool {
	case "search_knowledge":
		return "Help articles"
	case "list_tasks", "get_task_context":
		return "Tasks"
	case "list_support_conversations":
		return "Past conversations"
	}
	return ""
}
