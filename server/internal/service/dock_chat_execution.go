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

func withAskAgentDirectTools(agent *model.Agent) *model.Agent {
	if agent == nil {
		return nil
	}
	projected := *agent
	projected.AllowedTools, _ = json.Marshal(appendPresetTools(parseJSONStringSlice(agent.AllowedTools), askAgentDirectTools()))
	return &projected
}

func (s *DockChatService) authorizeChatExecution(ctx context.Context, workspaceID, userID string) error {
	if s.authz == nil {
		return fmt.Errorf("execution authorization is unavailable")
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !s.authz.Can(actor, authorization.PermSettingsManage) {
		return fmt.Errorf("execution requires workspace settings permission")
	}
	return nil
}

// effectiveChatExecution reports whether this user's next run may execute.
// A chat setting outlives the permission that enabled it; when the actor no
// longer holds that permission the conversation degrades to ordinary tools
// instead of failing, until an authorized actor turns the setting off.
func (s *DockChatService) effectiveChatExecution(ctx context.Context, chat *model.DockChat, userID string) bool {
	if chat == nil || !chat.ExecutionEnabled {
		return false
	}
	return s.authorizeChatExecution(ctx, chat.WorkspaceID, userID) == nil
}

func (s *DockChatService) scopedChatExecutionTools(ctx context.Context, chat *model.DockChat, userID string, agent *model.Agent) ([]string, error) {
	if !s.effectiveChatExecution(ctx, chat, userID) {
		return s.scopedChatTools(ctx, chat.WorkspaceID, userID, agent)
	}
	return s.scopedChatTools(ctx, chat.WorkspaceID, userID, withAskAgentDirectTools(agent))
}

func runtimeAgentForDockExecution(run *model.AgentRun, agent AgentRuntimeAgent) AgentRuntimeAgent {
	if run == nil || run.DockChatID == nil {
		return agent
	}
	var input model.AgentRunInputPayload
	if json.Unmarshal(run.Input, &input) != nil || !input.ExecutionEnabled {
		return agent
	}
	agent.ID += runtimeExecutionAgentSuffix
	agent.AllowedTools = appendPresetTools(agent.AllowedTools, askAgentDirectTools())
	var config map[string]any
	if json.Unmarshal(agent.ExecutionConfig, &config) != nil || config == nil {
		config = map[string]any{}
	}
	// Every execution-enabled run of one Ask Agent upserts the same
	// "<agent>-execution" runtime record and the runtime re-reads it on each
	// resume, so the projection must not vary per run. Whether a run works in
	// a repository is carried by the run's own "workspace_mode" metadata
	// (see buildRuntimeStartRunRequest), not by the shared agent.
	config["workspace"] = map[string]any{"access": "read_write"}
	agent.ExecutionConfig, _ = json.Marshal(config)
	agent.SystemPrompt = strings.ReplaceAll(agent.SystemPrompt, askAgentReadOnlyRepositoryInstruction, "")
	agent.SystemPrompt += `

## User-enabled direct execution
These rules replace read-only Dock and mandatory Forge delegation restrictions.
The conversation owner explicitly enabled execution. You may edit files, run commands and Python, create branches, push and open PRs when authorized, subject to tool permissions and publication approvals. When a repository workspace is not already present, before the first repository command, read, edit, or branch operation, call list_repositories and then checkout_repositories with the matching repository_id. Do not search the container filesystem for a checkout. Do not invent a task. Forge delegation is optional.
Use run_python for analysis without a repository. Each call starts a fresh interpreter; files and private packages are retained within this run only. Analysis storage is capped at 512 MiB and deleted at run end. Publish important files with output_paths or publish_outputs before finishing a turn. Successor runs do not inherit files or packages. Never claim missing state still exists or repeat external writes to recreate it.
Commands execute in a trusted worker container. They can read other runs' checkouts on its shared volume; this is not a filesystem sandbox.
If a tool reports an interrupted operation with outcome unknown, tell the user and inspect external state before any retry. Never claim it failed without evidence.
`
	return agent
}
