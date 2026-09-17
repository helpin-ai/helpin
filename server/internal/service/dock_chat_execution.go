package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func askAgentDirectTools() []string {
	return []string{"write_file", "edit_file", "apply_patch", "run_command", "run_python", "publish_outputs", "create_branch", "commit_and_push", "open_pr"}
}

func (s *DockChatService) authorizeChatExecution(ctx context.Context, workspaceID, userID string) error {
	if s.authz == nil {
		return fmt.Errorf("execution authorization is unavailable")
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !s.authz.CanAny(actor, authorization.PermPMEdit, authorization.PermSettingsManage) {
		return fmt.Errorf("execution requires project editing or workspace settings permission")
	}
	return nil
}

func (s *DockChatService) scopedChatExecutionTools(ctx context.Context, chat *model.DockChat, userID string, agent *model.Agent) ([]string, error) {
	if !chat.ExecutionEnabled {
		return s.scopedChatTools(ctx, chat.WorkspaceID, userID, agent)
	}
	if err := s.authorizeChatExecution(ctx, chat.WorkspaceID, userID); err != nil {
		return nil, err
	}
	copy := *agent
	copy.AllowedTools, _ = json.Marshal(appendPresetTools(parseJSONStringSlice(agent.AllowedTools), askAgentDirectTools()))
	return s.scopedChatTools(ctx, chat.WorkspaceID, userID, &copy)
}

func runtimeAgentForDockExecution(run *model.AgentRun, agent AgentRuntimeAgent) AgentRuntimeAgent {
	if run == nil || run.DockChatID == nil {
		return agent
	}
	var input model.AgentRunInputPayload
	if json.Unmarshal(run.Input, &input) != nil || !input.ExecutionEnabled {
		return agent
	}
	agent.ID += "-execution"
	agent.AllowedTools = appendPresetTools(agent.AllowedTools, askAgentDirectTools())
	var config map[string]any
	if json.Unmarshal(agent.ExecutionConfig, &config) != nil || config == nil {
		config = map[string]any{}
	}
	config["workspace"] = map[string]any{"access": "read_write"}
	agent.ExecutionConfig, _ = json.Marshal(config)
	const readOnly = "- Repository inspection is read-only: discover the repository, check out its default branch, and use read/search/symbol/commit-history tools. Never attempt file edits, shell commands, branches, commits, pushes, merges, or pull requests from the Dock."
	agent.SystemPrompt = strings.ReplaceAll(agent.SystemPrompt, readOnly, "")
	agent.SystemPrompt += `

## User-enabled direct execution
These rules replace read-only Dock and mandatory Forge delegation restrictions.
The conversation owner explicitly enabled execution. You may edit files, run commands and Python, create branches, push and open PRs when authorized, subject to tool permissions and publication approvals. Attach repositories with list_repositories and checkout_repositories; do not invent a task. Forge delegation is optional.
Use run_python for analysis without a repository. Each call starts a fresh interpreter; files and private packages are retained within this run only. Analysis storage is capped at 512 MiB and deleted at run end. Publish important files with output_paths or publish_outputs before finishing a turn. Successor runs do not inherit files or packages. Never claim missing state still exists or repeat external writes to recreate it.
Commands execute in a trusted worker container. They can read other runs' checkouts on its shared volume; this is not a filesystem sandbox.
If a tool reports an interrupted operation with outcome unknown, tell the user and inspect external state before any retry. Never claim it failed without evidence.
`
	return agent
}
