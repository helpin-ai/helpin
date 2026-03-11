package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

type OpenCodeExecutor struct {
	kind              string
	commandPath       string
	anthropicAPIKey   string
	openAIAPIKey      string
	openAIBaseURL     string
	openRouterAPIKey  string
	openRouterBaseURL string
	runRepo           *repository.AgentRunRepository
	artifactRepo      *repository.AgentRunArtifactRepository
}

func NewOpenCodeExecutor(
	kind string,
	commandPath string,
	anthropicAPIKey string,
	openAIAPIKey string,
	openAIBaseURL string,
	openRouterAPIKey string,
	openRouterBaseURL string,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
) *OpenCodeExecutor {
	if strings.TrimSpace(commandPath) == "" {
		commandPath = "opencode"
	}
	return &OpenCodeExecutor{
		kind:              kind,
		commandPath:       commandPath,
		anthropicAPIKey:   strings.TrimSpace(anthropicAPIKey),
		openAIAPIKey:      strings.TrimSpace(openAIAPIKey),
		openAIBaseURL:     strings.TrimSpace(openAIBaseURL),
		openRouterAPIKey:  strings.TrimSpace(openRouterAPIKey),
		openRouterBaseURL: strings.TrimSpace(openRouterBaseURL),
		runRepo:           runRepo,
		artifactRepo:      artifactRepo,
	}
}

func (e *OpenCodeExecutor) Kind() string {
	return e.kind
}

func (e *OpenCodeExecutor) Execute(execCtx *ExecutionContext, run *model.AgentRun) error {
	config := execCtx.Config
	if config == nil {
		config = DefaultWorkflowConfig()
	}

	var checklist []model.PMChecklistItem
	if execCtx.StoryID != "" && execCtx.Services != nil && execCtx.Services.ListChecklist != nil {
		items, err := execCtx.Services.ListChecklist(execCtx.Context, execCtx.WorkspaceID, execCtx.StoryID)
		if err != nil {
			log.Printf("warning: failed to list checklist for opencode run: %v", err)
		} else {
			checklist = items
		}
	}

	var ticketMessages []model.SupportMessage
	if execCtx.TicketID != "" && execCtx.Services != nil && execCtx.Services.ListTicketMessages != nil {
		messages, err := execCtx.Services.ListTicketMessages(execCtx.Context, execCtx.WorkspaceID, execCtx.TicketID)
		if err != nil {
			log.Printf("warning: failed to list ticket messages for opencode run: %v", err)
		} else {
			ticketMessages = messages
		}
	}

	systemPrompt := BuildSystemPrompt(execCtx.Agent, execCtx.Story, execCtx.Epic, execCtx.Ticket, execCtx.PlanningStage, execCtx.PlanningMethodology, config)
	userPrompt := BuildUserPrompt(
		execCtx.Story,
		execCtx.Epic,
		execCtx.EpicStories,
		execCtx.Ticket,
		ticketMessages,
		checklist,
		execCtx.PlanningStage,
		execCtx.InitialInstructions,
	)
	if execCtx.Ticket != nil {
		systemPrompt += "\nFor support tickets, respond with valid JSON only in this shape: " +
			`{"status":"open|in_progress|pending|resolved|closed","draft_reply":{"content":"...","is_internal":false,"sender_display_name":"optional","approval_required":true}}.`
	}

	fullPrompt := buildOpenCodePrompt(systemPrompt, userPrompt)
	args := []string{"run"}
	if agentName := openCodeAgentName(execCtx); agentName != "" {
		args = append(args, "--agent", agentName)
	}
	if modelID := e.resolveModelID(execCtx.Agent); modelID != "" {
		args = append(args, "--model", modelID)
	}
	args = append(args, fullPrompt)

	cmd := exec.CommandContext(execCtx.Context, e.commandPath, args...)
	cmd.Dir = execCtx.WorkDir
	cmd.Env = e.buildEnv(execCtx.Agent)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("opencode_starting")
	}

	done := make(chan struct{})
	defer close(done)
	if execCtx.Heartbeat != nil {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = execCtx.Heartbeat("opencode_running")
				case <-done:
					return
				case <-execCtx.Context.Done():
					return
				}
			}
		}()
	}

	if err := cmd.Run(); err != nil {
		e.saveProcessArtifacts(execCtx.Context, run, stdout.String(), stderr.String())
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("opencode executable %q was not found on PATH", e.commandPath)
		}
		return fmt.Errorf("opencode run failed: %s", strings.TrimSpace(firstNonEmptyText(stderr.String(), err.Error())))
	}

	responseText := sanitizeOpenCodeOutput(firstNonEmptyText(stdout.String(), stderr.String()))
	e.saveProcessArtifacts(execCtx.Context, run, stdout.String(), stderr.String())
	if strings.TrimSpace(responseText) == "" {
		return fmt.Errorf("opencode returned no response")
	}

	seqNo := 2
	e.saveArtifact(execCtx.Context, run, "agent_summary", "markdown", responseText, seqNo)
	seqNo++

	run.TokensUsed = 0

	switch {
	case execCtx.TargetType == "epic" && execCtx.Epic != nil:
		switch execCtx.PlanningStage {
		case model.PlanningStageDraftSpec:
			draft, err := extractProductSpecDraftFromResponseText(responseText)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(draft)
			run.OutputSummary = payload
			_ = e.runRepo.Update(execCtx.Context, run)
			e.saveArtifact(execCtx.Context, run, "product_spec_draft", "json", string(payload), seqNo)
		case model.PlanningStagePlanStories:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, 0)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			_ = e.runRepo.Update(execCtx.Context, run)
			e.saveArtifact(execCtx.Context, run, "story_plan_proposal", "json", string(payload), seqNo)
		default:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, 0)
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			_ = e.runRepo.Update(execCtx.Context, run)
			e.saveArtifact(execCtx.Context, run, "orchestration_proposal", "json", string(payload), seqNo)
		}
	case execCtx.TargetType == "support_ticket" && execCtx.Ticket != nil:
		summary, err := extractSupportRunSummaryFromResponseText(responseText)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(summary)
		run.OutputSummary = payload
		_ = e.runRepo.Update(execCtx.Context, run)
		e.saveArtifact(execCtx.Context, run, "support_draft", "json", string(payload), seqNo)
		if summary.Status != nil && execCtx.Services != nil && execCtx.Services.UpdateTicketStatus != nil {
			if err := execCtx.Services.UpdateTicketStatus(execCtx.Context, execCtx.WorkspaceID, execCtx.TicketID, *summary.Status); err != nil {
				log.Printf("warning: failed to update support ticket status: %v", err)
			}
		}
	default:
		run.OutputSummary = json.RawMessage(`{"status":"success"}`)
		_ = e.runRepo.Update(execCtx.Context, run)
	}

	return nil
}

func (e *OpenCodeExecutor) buildEnv(agent *model.Agent) []string {
	env := os.Environ()
	switch strings.TrimSpace(derefOpenCodeString(agent.Provider)) {
	case "", model.AgentModelProviderAnthropic:
		env = appendIfMissingEnv(env, "ANTHROPIC_API_KEY", e.anthropicAPIKey)
	case model.AgentModelProviderOpenAI:
		env = appendIfMissingEnv(env, "OPENAI_API_KEY", e.openAIAPIKey)
		env = appendIfMissingEnv(env, "OPENAI_BASE_URL", e.openAIBaseURL)
	case model.AgentModelProviderOpenRouter:
		env = appendIfMissingEnv(env, "OPENROUTER_API_KEY", e.openRouterAPIKey)
		env = appendIfMissingEnv(env, "OPENROUTER_BASE_URL", e.openRouterBaseURL)
	}
	return env
}

func (e *OpenCodeExecutor) resolveModelID(agent *model.Agent) string {
	if agent == nil {
		return ""
	}
	provider := strings.TrimSpace(derefOpenCodeString(agent.Provider))
	modelName := strings.TrimSpace(derefOpenCodeString(agent.Model))
	if provider == "" && modelName == "" {
		return ""
	}
	if provider == "" {
		provider = model.AgentModelProviderAnthropic
	}
	if modelName == "" {
		modelName = defaultOpenCodeModelForProvider(provider)
	}
	if modelName == "" {
		return ""
	}
	return provider + "/" + modelName
}

func (e *OpenCodeExecutor) saveProcessArtifacts(ctx context.Context, run *model.AgentRun, stdout, stderr string) {
	seqNo := 0
	if strings.TrimSpace(stdout) != "" {
		seqNo++
		e.saveArtifact(ctx, run, "opencode_stdout", "text", stdout, seqNo)
	}
	if strings.TrimSpace(stderr) != "" {
		seqNo++
		e.saveArtifact(ctx, run, "opencode_stderr", "text", stderr, seqNo)
	}
}

func (e *OpenCodeExecutor) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) {
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

func buildOpenCodePrompt(systemPrompt, userPrompt string) string {
	return strings.TrimSpace("System instructions:\n" + systemPrompt + "\n\nUser task:\n" + userPrompt)
}

func openCodeAgentName(execCtx *ExecutionContext) string {
	if execCtx == nil || execCtx.Agent == nil {
		return ""
	}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassProductPlanner:
		return "plan"
	case model.AgentClassEngineer, model.AgentClassReviewer:
		return "build"
	default:
		return ""
	}
}

func sanitizeOpenCodeOutput(value string) string {
	value = ansiEscapePattern.ReplaceAllString(value, "")
	return strings.TrimSpace(value)
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultOpenCodeModelForProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case model.AgentModelProviderOpenAI:
		return "gpt-5-mini"
	case model.AgentModelProviderOpenRouter:
		return "openai/gpt-5-mini"
	default:
		return "claude-sonnet-4-20250514"
	}
}

func derefOpenCodeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type openCodeSupportDraftReply struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

type openCodeSupportRunSummary struct {
	Status     *string                    `json:"status,omitempty"`
	DraftReply *openCodeSupportDraftReply `json:"draft_reply,omitempty"`
}

func extractSupportRunSummaryFromResponseText(responseText string) (*openCodeSupportRunSummary, error) {
	var summary openCodeSupportRunSummary
	if err := unmarshalLatestJSON(responseText, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse support summary: %w", err)
	}
	if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
		return nil, fmt.Errorf("support run did not return a draft reply")
	}
	if !summary.DraftReply.ApprovalRequired {
		summary.DraftReply.ApprovalRequired = true
	}
	return &summary, nil
}

func appendIfMissingEnv(env []string, key, value string) []string {
	if strings.TrimSpace(value) == "" {
		return env
	}
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return env
		}
	}
	return append(env, prefix+value)
}
