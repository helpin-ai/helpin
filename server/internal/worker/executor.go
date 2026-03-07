package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var ErrRunCancelled = errors.New("run was cancelled")

// Executor runs the Claude API tool loop for a single agent run.
type Executor struct {
	kind         string
	claude       *ClaudeClient
	tools        *ToolRegistry
	runRepo      *repository.AgentRunRepository
	artifactRepo *repository.AgentRunArtifactRepository
}

// NewExecutor creates a new executor.
func NewExecutor(
	kind string,
	claude *ClaudeClient,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *Executor {
	return &Executor{
		kind:         kind,
		claude:       claude,
		tools:        NewToolRegistry(),
		runRepo:      runRepo,
		artifactRepo: artifactRepo,
	}
}

// Kind identifies the runtime handled by this adapter.
func (e *Executor) Kind() string {
	return e.kind
}

// Execute runs the full tool loop for a given execution context.
func (e *Executor) Execute(execCtx *ExecutionContext, run *model.AgentRun) error {
	config := execCtx.Config
	if config == nil {
		config = DefaultWorkflowConfig()
	}

	if execCtx.RuntimeProfile.Name == "" {
		execCtx.RuntimeProfile = GetRuntimeProfile(execCtx.Agent.CapabilityProfile)
	}
	if len(execCtx.AllowedTools) == 0 {
		execCtx.AllowedTools = allowedToolSet(execCtx.RuntimeProfile)
	}

	// Build prompts.
	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Story, execCtx.Epic, execCtx.Ticket, config)

	var checklist []model.PMChecklistItem
	if execCtx.StoryID != "" && execCtx.Services != nil {
		var err error
		checklist, err = execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.StoryID)
		if err != nil {
			log.Printf("warning: failed to list checklist: %v", err)
		}
	}

	var ticketMessages []model.SupportMessage
	if execCtx.TicketID != "" && execCtx.Services != nil && execCtx.Services.ListTicketMessages != nil {
		var err error
		ticketMessages, err = execCtx.Services.ListTicketMessages(execCtx.Context, execCtx.WorkspaceID, execCtx.TicketID)
		if err != nil {
			log.Printf("warning: failed to list ticket messages: %v", err)
		}
	}

	userPrompt := BuildUserPrompt(
		execCtx.Story,
		execCtx.Epic,
		execCtx.EpicStories,
		execCtx.Ticket,
		ticketMessages,
		checklist,
		execCtx.InitialInstructions,
	)

	messages := []Message{
		{Role: "user", Content: userPrompt},
	}

	totalTokens := 0
	seqNo := 0

	timeout := time.Duration(config.TimeoutMinutes) * time.Minute
	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()

	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		if execCtx.Heartbeat != nil {
			_ = execCtx.Heartbeat(fmt.Sprintf("iteration_%d", iteration+1))
		}

		// Check cancellation.
		select {
		case <-ctx.Done():
			return fmt.Errorf("run timed out after %d minutes", config.TimeoutMinutes)
		default:
		}

		// Check if run was cancelled externally.
		currentRun, err := e.runRepo.GetByID(ctx, run.WorkspaceID, run.ID)
		if err == nil && currentRun != nil && currentRun.Status == "cancelled" {
			return ErrRunCancelled
		}

		// Check budget.
		if execCtx.Agent.MonthlyTokenBudget != nil {
			budget := *execCtx.Agent.MonthlyTokenBudget
			if execCtx.Agent.TokensUsedThisMonth+totalTokens > budget {
				return fmt.Errorf("token budget exceeded (%d/%d)", execCtx.Agent.TokensUsedThisMonth+totalTokens, budget)
			}
		}

		// Call Claude API.
		resp, err := e.claude.CreateMessage(ctx, CreateMessageRequest{
			Model:    derefOrEmpty(execCtx.Agent.Model),
			System:   systemPrompt,
			Messages: messages,
			Tools:    e.tools.DefinitionsFor(execCtx.AllowedTools),
		})
		if err != nil {
			return fmt.Errorf("claude API call: %w", err)
		}

		totalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens

		// Update run token count.
		run.TokensUsed = totalTokens
		_ = e.runRepo.Update(ctx, run)

		// Process response.
		hasToolUse := false
		var assistantContent []ContentBlock

		for _, block := range resp.Content {
			assistantContent = append(assistantContent, block)
			if block.Type == "text" {
				// Save text output as artifact.
				seqNo++
				e.saveArtifact(ctx, run, "agent_summary", "markdown", block.Text, seqNo)
			}
		}

		// Add assistant message.
		messages = append(messages, Message{
			Role:    "assistant",
			Content: assistantContent,
		})

		// Handle tool calls.
		var toolResults []ContentBlock
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				if execCtx.Heartbeat != nil {
					_ = execCtx.Heartbeat("tool_" + block.Name)
				}
				hasToolUse = true
				log.Printf("[run=%s] tool_use: %s", run.ID[:8], block.Name)

				result, err := e.tools.ExecuteAllowed(execCtx, block.Name, block.Input)
				isError := false
				if err != nil {
					result = err.Error()
					isError = true
				}

				toolResults = append(toolResults, ContentBlock{
					Type:      "tool_result",
					ToolUseID: block.ID,
					Content:   result,
					IsError:   isError,
				})

				// Save tool call as artifact.
				seqNo++
				toolLog := fmt.Sprintf("Tool: %s\nInput: %s\nResult: %s", block.Name, string(block.Input), truncate(result, 5000))
				e.saveArtifact(ctx, run, "tool_log", "text", toolLog, seqNo)

				if block.Name == "run_command" && looksLikeTestCommand(block.Input) {
					seqNo++
					e.saveArtifact(ctx, run, "test_report", "text", truncate(result, 50000), seqNo)
				}
			}
		}

		if hasToolUse {
			messages = append(messages, Message{
				Role:    "user",
				Content: toolResults,
			})
		}

		// If the model stopped without requesting tools, we're done.
		if resp.StopReason == "end_turn" && !hasToolUse {
			break
		}
	}

	// Save final conversation log.
	convLog, _ := json.MarshalIndent(messages, "", "  ")
	seqNo++
	e.saveArtifact(ctx, run, "conversation_log", "json", string(convLog), seqNo)

	if execCtx.PendingSupportDraft != nil {
		summary, _ := json.Marshal(map[string]any{
			"draft_reply": execCtx.PendingSupportDraft,
		})
		run.OutputSummary = json.RawMessage(summary)
		_ = e.runRepo.Update(ctx, run)
	}

	if execCtx.TargetType == "epic" && execCtx.Epic != nil {
		proposal, err := extractOrchestrationProposal(messages, execCtx.Epic.ID, totalTokens)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(proposal)
		run.OutputSummary = payload
		_ = e.runRepo.Update(ctx, run)

		seqNo++
		e.saveArtifact(ctx, run, "orchestration_proposal", "json", string(payload), seqNo)
	}

	if execCtx.LatestPRMetadata != nil {
		prPayload, _ := json.MarshalIndent(execCtx.LatestPRMetadata, "", "  ")
		seqNo++
		e.saveArtifact(ctx, run, "pr_metadata", "json", string(prPayload), seqNo)
	}

	// Harvest git diff if we're in a repo.
	if execCtx.WorkDir != "" {
		diff, err := runGit(execCtx, "diff")
		if err == nil && diff != "" {
			seqNo++
			e.saveArtifact(ctx, run, "diff", "patch", diff, seqNo)
		}

		files, err := runGit(execCtx, "diff", "--name-only")
		if err == nil && strings.TrimSpace(files) != "" {
			seqNo++
			e.saveArtifact(ctx, run, "file_bundle", "json", toJSONString(strings.Fields(files)), seqNo)
		}
	}

	return nil
}

func (e *Executor) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) {
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	if err := e.artifactRepo.Create(ctx, artifact); err != nil {
		log.Printf("warning: failed to save artifact: %v", err)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

func looksLikeTestCommand(input json.RawMessage) bool {
	var params struct {
		Program string   `json:"program"`
		Args    []string `json:"args"`
		Command string   `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(params.Command))
	if params.Program != "" {
		command = strings.ToLower(strings.TrimSpace(params.Program) + " " + strings.Join(params.Args, " "))
	}
	return strings.Contains(command, " test") || strings.HasPrefix(command, "test ") || strings.Contains(command, "go test") || strings.Contains(command, "npm test") || strings.Contains(command, "cargo test")
}

func toJSONString(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func derefOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
