package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	openCodeChunkFlushInterval = 2 * time.Second
	openCodeChunkFlushBytes    = 4 * 1024
	openCodePostRunTimeout     = 2 * time.Minute
	openCodeScannerBufferSize  = 1024 * 1024
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

	userPrompt = buildOpenCodeUserPrompt(execCtx, userPrompt)
	modelID := e.resolveModelID(execCtx.Agent)
	configContent, err := buildOpenCodeConfigContent(execCtx, modelID, systemPrompt)
	if err != nil {
		return fmt.Errorf("build opencode config: %w", err)
	}

	args := []string{"run"}
	if agentName := openCodeAgentName(execCtx); agentName != "" {
		args = append(args, "--agent", agentName)
	}
	if modelID != "" {
		args = append(args, "--model", modelID)
	}
	args = append(args, "--format", "json")
	args = append(args, userPrompt)

	cmd := exec.CommandContext(execCtx.Context, e.commandPath, args...)
	cmd.Dir = execCtx.WorkDir
	cmd.Env = e.buildEnv(execCtx.Agent, configContent)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create opencode stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create opencode stderr pipe: %w", err)
	}

	artifactWriter := newOpenCodeArtifactWriter(e, run)
	artifactWriter.Save(execCtx.Context, "opencode_config", "json", configContent, true)

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("opencode_starting")
	}

	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("opencode executable %q was not found on PATH", e.commandPath)
		}
		return fmt.Errorf("start opencode run: %w", err)
	}

	done := make(chan struct{})
	var stopOnce sync.Once
	stopHeartbeat := func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
	defer stopHeartbeat()
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

	streamCollector := newOpenCodeStreamCollector(run.ID, artifactWriter)
	streamErrs := make(chan error, 2)
	var streamWG sync.WaitGroup
	streamWG.Add(2)
	go consumeOpenCodeJSONStream(stdoutPipe, execCtx.Context, streamCollector, &streamWG, streamErrs)
	go consumeOpenCodeTextStream(stderrPipe, "stderr", execCtx.Context, streamCollector, &streamWG, streamErrs)

	flushDone := make(chan struct{})
	go streamCollector.FlushLoop(execCtx.Context, flushDone)

	waitErr := cmd.Wait()
	stopHeartbeat()
	close(flushDone)

	streamWG.Wait()
	close(streamErrs)

	artifactCtx, cancelArtifacts := backgroundContextOnCancel(execCtx.Context)
	defer cancelArtifacts()

	streamCollector.FlushPending(artifactCtx, true)
	stdoutText, stderrText := streamCollector.Outputs()
	artifactWriter.Save(artifactCtx, "opencode_stdout", "text", stdoutText, false)
	artifactWriter.Save(artifactCtx, "opencode_stderr", "text", stderrText, false)

	for streamErr := range streamErrs {
		if streamErr == nil {
			continue
		}
		if waitErr == nil {
			waitErr = streamErr
			continue
		}
		log.Printf("warning: failed to stream opencode output: %v", streamErr)
	}

	if waitErr != nil {
		if execCtx.Context.Err() != nil {
			return ErrRunCancelled
		}
		return fmt.Errorf("opencode run failed: %s", strings.TrimSpace(firstNonEmptyText(stderrText, waitErr.Error())))
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("opencode_finished")
	}

	if eventErr := streamCollector.EventError(); eventErr != "" {
		return fmt.Errorf("opencode reported an error: %s", eventErr)
	}

	responseText := sanitizeOpenCodeOutput(firstNonEmptyText(streamCollector.ResponseText(), stdoutText, stderrText))
	if strings.TrimSpace(responseText) == "" {
		return fmt.Errorf("opencode returned no response")
	}

	postRunCtx, cancelPostRun := context.WithTimeout(execCtx.Context, openCodePostRunTimeout)
	defer cancelPostRun()
	postRunExecCtx := cloneExecutionContext(execCtx, postRunCtx)

	if isEngineerStoryRun(postRunExecCtx) {
		if err := e.persistEngineerWorkspace(postRunExecCtx, run, artifactWriter); err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
	}

	if postRunExecCtx.Heartbeat != nil {
		_ = postRunExecCtx.Heartbeat("finalizing")
	}
	artifactWriter.Save(postRunCtx, "agent_summary", "markdown", responseText, false)

	run.TokensUsed = streamCollector.TokensUsed()

	switch {
	case execCtx.TargetType == "epic" && execCtx.Epic != nil:
		switch execCtx.PlanningStage {
		case model.PlanningStageDraftSpec:
			draft, err := extractProductSpecDraftFromResponseText(responseText)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			payload, _ := json.Marshal(draft)
			run.OutputSummary = payload
			if err := e.runRepo.Update(postRunCtx, run); err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			artifactWriter.Save(postRunCtx, "product_spec_draft", "json", string(payload), false)
		case model.PlanningStagePlanStories:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, run.TokensUsed)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			if err := e.runRepo.Update(postRunCtx, run); err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			artifactWriter.Save(postRunCtx, "story_plan_proposal", "json", string(payload), false)
		default:
			proposal, err := extractPlanningProposalFromResponseText(responseText, execCtx.Epic.ID, execCtx.PlanningSpecVersionID, run.TokensUsed)
			if err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			payload, _ := json.Marshal(proposal)
			run.OutputSummary = payload
			if err := e.runRepo.Update(postRunCtx, run); err != nil {
				return normalizeOpenCodePostRunError(postRunCtx, err)
			}
			artifactWriter.Save(postRunCtx, "orchestration_proposal", "json", string(payload), false)
		}
	case execCtx.TargetType == "support_ticket" && execCtx.Ticket != nil:
		summary, err := extractSupportRunSummaryFromResponseText(responseText)
		if err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
		payload, _ := json.Marshal(summary)
		run.OutputSummary = payload
		if err := e.runRepo.Update(postRunCtx, run); err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
		artifactWriter.Save(postRunCtx, "support_draft", "json", string(payload), false)
		if summary.Status != nil && execCtx.Services != nil && execCtx.Services.UpdateTicketStatus != nil {
			if err := execCtx.Services.UpdateTicketStatus(postRunCtx, execCtx.WorkspaceID, execCtx.TicketID, *summary.Status); err != nil {
				log.Printf("warning: failed to update support ticket status: %v", err)
			}
		}
	default:
		run.OutputSummary = json.RawMessage(`{"status":"success"}`)
		if err := e.runRepo.Update(postRunCtx, run); err != nil {
			return normalizeOpenCodePostRunError(postRunCtx, err)
		}
	}

	return normalizeOpenCodePostRunError(postRunCtx, nil)
}

func (e *OpenCodeExecutor) buildEnv(agent *model.Agent, configContent string) []string {
	env := os.Environ()
	provider := model.AgentModelProviderAnthropic
	if agent != nil {
		if resolvedProvider := strings.TrimSpace(derefOpenCodeString(agent.Provider)); resolvedProvider != "" {
			provider = resolvedProvider
		}
	}
	switch provider {
	case "", model.AgentModelProviderAnthropic:
		env = appendIfMissingEnv(env, "ANTHROPIC_API_KEY", e.anthropicAPIKey)
	case model.AgentModelProviderOpenAI:
		env = appendIfMissingEnv(env, "OPENAI_API_KEY", e.openAIAPIKey)
		env = appendIfMissingEnv(env, "OPENAI_BASE_URL", e.openAIBaseURL)
	case model.AgentModelProviderOpenRouter:
		env = appendIfMissingEnv(env, "OPENROUTER_API_KEY", e.openRouterAPIKey)
		env = appendIfMissingEnv(env, "OPENROUTER_BASE_URL", e.openRouterBaseURL)
	}
	env = upsertEnv(env, "NO_COLOR", "1")
	env = upsertEnv(env, "OPENCODE_CONFIG_CONTENT", configContent)
	return env
}

func (e *OpenCodeExecutor) resolveModelID(agent *model.Agent) string {
	provider := model.AgentModelProviderAnthropic
	modelName := ""
	if agent != nil {
		if resolvedProvider := strings.TrimSpace(derefOpenCodeString(agent.Provider)); resolvedProvider != "" {
			provider = resolvedProvider
		}
		modelName = strings.TrimSpace(derefOpenCodeString(agent.Model))
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

func (e *OpenCodeExecutor) notifyRun(ctx context.Context, run *model.AgentRun) {
	if e.runRepo == nil || run == nil {
		return
	}
	notifyCtx, cancel := backgroundContextOnCancel(ctx)
	defer cancel()
	e.runRepo.Notify(notifyCtx, run)
}

type openCodeArtifactWriter struct {
	executor *OpenCodeExecutor
	run      *model.AgentRun
	mu       sync.Mutex
	seqNo    int
}

func newOpenCodeArtifactWriter(executor *OpenCodeExecutor, run *model.AgentRun) *openCodeArtifactWriter {
	return &openCodeArtifactWriter{executor: executor, run: run}
}

func (w *openCodeArtifactWriter) Save(ctx context.Context, artifactType, format, content string, notify bool) {
	if w == nil || w.executor == nil || w.run == nil || content == "" {
		return
	}
	w.mu.Lock()
	w.seqNo++
	seqNo := w.seqNo
	w.mu.Unlock()
	w.executor.saveArtifact(ctx, w.run, artifactType, format, content, seqNo)
	if notify {
		w.executor.notifyRun(ctx, w.run)
	}
}

type openCodeStreamCollector struct {
	runID  string
	writer *openCodeArtifactWriter
	mu     sync.Mutex

	stdoutFull  strings.Builder
	stderrFull  strings.Builder
	stdoutChunk strings.Builder
	stderrChunk strings.Builder
	response    strings.Builder
	eventError  string
	tokensUsed  int
}

func newOpenCodeStreamCollector(runID string, writer *openCodeArtifactWriter) *openCodeStreamCollector {
	return &openCodeStreamCollector{
		runID:  runID,
		writer: writer,
	}
}

func (c *openCodeStreamCollector) AppendLine(ctx context.Context, stream, line string) {
	logLine := strings.TrimSpace(line)
	if logLine != "" {
		log.Printf("[run=%s][opencode.%s] %s", shortRunID(c.runID), stream, truncate(logLine, 1000))
	}

	c.mu.Lock()
	full, chunk := c.buffersFor(stream)
	full.WriteString(line)
	full.WriteByte('\n')
	chunk.WriteString(line)
	chunk.WriteByte('\n')
	shouldFlush := chunk.Len() >= openCodeChunkFlushBytes
	c.mu.Unlock()

	if shouldFlush {
		c.FlushStream(ctx, stream, true)
	}
}

func (c *openCodeStreamCollector) AppendResponseText(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.response.Len() > 0 {
		c.response.WriteString("\n")
	}
	c.response.WriteString(text)
}

func (c *openCodeStreamCollector) AddTokens(tokens int) {
	if tokens <= 0 {
		return
	}
	c.mu.Lock()
	c.tokensUsed += tokens
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) SetEventError(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	c.mu.Lock()
	if c.eventError == "" {
		c.eventError = message
	}
	c.mu.Unlock()
}

func (c *openCodeStreamCollector) FlushLoop(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(openCodeChunkFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.FlushPending(ctx, true)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *openCodeStreamCollector) FlushPending(ctx context.Context, notify bool) {
	c.FlushStream(ctx, "stdout", notify)
	c.FlushStream(ctx, "stderr", notify)
}

func (c *openCodeStreamCollector) FlushStream(ctx context.Context, stream string, notify bool) {
	c.mu.Lock()
	_, chunk := c.buffersFor(stream)
	content := chunk.String()
	chunk.Reset()
	c.mu.Unlock()

	if content == "" {
		return
	}
	c.writer.Save(ctx, "opencode_"+stream+"_chunk", "text", content, notify)
}

func (c *openCodeStreamCollector) Outputs() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdoutFull.String(), c.stderrFull.String()
}

func (c *openCodeStreamCollector) ResponseText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.response.String()
}

func (c *openCodeStreamCollector) EventError() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eventError
}

func (c *openCodeStreamCollector) TokensUsed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokensUsed
}

func (c *openCodeStreamCollector) buffersFor(stream string) (*strings.Builder, *strings.Builder) {
	if stream == "stderr" {
		return &c.stderrFull, &c.stderrChunk
	}
	return &c.stdoutFull, &c.stdoutChunk
}

func consumeOpenCodeTextStream(reader io.Reader, stream string, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		collector.AppendLine(ctx, stream, stripANSI(scanner.Text()))
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read %s: %w", stream, err)
		return
	}
	errCh <- nil
}

func consumeOpenCodeJSONStream(reader io.Reader, ctx context.Context, collector *openCodeStreamCollector, wg *sync.WaitGroup, errCh chan<- error) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), openCodeScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(stripANSI(scanner.Text()))
		if line == "" {
			continue
		}

		displayLine, responseText, tokensUsed, eventErr, err := parseOpenCodeJSONEvent(line)
		if err != nil {
			log.Printf("warning: treating non-json opencode stdout line as plain text: %v", err)
			collector.AppendLine(ctx, "stdout", line)
			continue
		}
		if strings.TrimSpace(displayLine) != "" {
			collector.AppendLine(ctx, "stdout", displayLine)
		}
		if responseText != "" {
			collector.AppendResponseText(responseText)
		}
		if tokensUsed > 0 {
			collector.AddTokens(tokensUsed)
		}
		if eventErr != "" {
			collector.SetEventError(eventErr)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		errCh <- fmt.Errorf("read stdout: %w", err)
		return
	}
	errCh <- nil
}

func parseOpenCodeJSONEvent(line string) (displayLine, responseText string, tokensUsed int, eventErr string, err error) {
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", 0, "", fmt.Errorf("unmarshal event line %q: %w", truncate(line, 200), err)
	}

	eventType := lookupString(event, "type")
	part, _ := event["part"].(map[string]any)
	switch eventType {
	case "text":
		responseText = strings.TrimSpace(lookupString(part, "text"))
		return responseText, responseText, 0, "", nil
	case "tool_use":
		return renderOpenCodeToolUse(part), "", 0, "", nil
	case "step_start":
		return renderOpenCodeStepStart(part), "", 0, "", nil
	case "step_finish":
		tokensUsed = openCodeTokensFromPart(part)
		return renderOpenCodeStepFinish(part, tokensUsed), "", tokensUsed, "", nil
	case "error":
		eventErr = firstNonEmptyText(
			lookupString(event, "error"),
			lookupString(event, "message"),
			lookupString(part, "error"),
			lookupString(part, "message"),
			renderOpenCodeRawPart(part),
		)
		return strings.TrimSpace(firstNonEmptyText("Error: "+strings.TrimSpace(eventErr), eventErr)), "", 0, strings.TrimSpace(eventErr), nil
	default:
		return renderOpenCodeGenericEvent(eventType, part), "", 0, "", nil
	}
}

func renderOpenCodeStepStart(part map[string]any) string {
	title := firstNonEmptyText(
		lookupString(part, "title"),
		lookupString(part, "name"),
		lookupString(part, "tool"),
	)
	if title == "" {
		return "Step started"
	}
	return "Step started: " + title
}

func renderOpenCodeToolUse(part map[string]any) string {
	title := firstNonEmptyText(
		lookupString(part, "title"),
		lookupString(part, "name"),
		lookupString(part, "tool"),
	)
	if title == "" {
		if metadata, ok := part["metadata"].(map[string]any); ok {
			title = firstNonEmptyText(
				lookupString(metadata, "command"),
				lookupString(metadata, "description"),
				lookupString(metadata, "path"),
			)
		}
	}
	if title == "" {
		title = renderOpenCodeRawPart(part)
	}
	if title == "" {
		return "Tool used"
	}
	return "Tool: " + title
}

func renderOpenCodeStepFinish(part map[string]any, tokensUsed int) string {
	var details []string
	if stopReason := firstNonEmptyText(lookupString(part, "stopReason"), lookupString(part, "stop_reason")); stopReason != "" {
		details = append(details, "stop="+stopReason)
	}
	if tokensUsed > 0 {
		details = append(details, fmt.Sprintf("tokens=%d", tokensUsed))
	}
	if len(details) == 0 {
		return "Step finished"
	}
	return "Step finished (" + strings.Join(details, ", ") + ")"
}

func renderOpenCodeGenericEvent(eventType string, part map[string]any) string {
	label := strings.TrimSpace(eventType)
	if label == "" {
		label = "event"
	}
	raw := renderOpenCodeRawPart(part)
	if raw == "" {
		return "OpenCode " + label
	}
	return "OpenCode " + label + ": " + raw
}

func renderOpenCodeRawPart(part map[string]any) string {
	if len(part) == 0 {
		return ""
	}
	payload, err := json.Marshal(part)
	if err != nil {
		return ""
	}
	return truncate(string(payload), 1000)
}

func openCodeTokensFromPart(part map[string]any) int {
	tokens, _ := part["tokens"].(map[string]any)
	if len(tokens) == 0 {
		return 0
	}

	total := lookupInt(tokens, "input") + lookupInt(tokens, "output") + lookupInt(tokens, "reasoning")
	if cache, ok := tokens["cache"].(map[string]any); ok {
		total += lookupInt(cache, "read") + lookupInt(cache, "write")
	}
	return total
}

func lookupString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func lookupInt(values map[string]any, key string) int {
	if len(values) == 0 {
		return 0
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case json.Number:
		value, err := typed.Int64()
		if err == nil {
			return int(value)
		}
	case string:
		value, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return value
		}
	}
	return 0
}

func (e *OpenCodeExecutor) persistEngineerWorkspace(execCtx *ExecutionContext, run *model.AgentRun, artifactWriter *openCodeArtifactWriter) error {
	if !isEngineerStoryRun(execCtx) {
		return nil
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("persisting_changes")
	}

	changed, err := openCodeRunProducedRepoChanges(execCtx)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("opencode completed without modifying the repository")
	}

	if out, err := runGit(execCtx, "add", "-A"); err != nil {
		return fmt.Errorf("stage repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	diff, err := runGit(execCtx, "diff", "--cached")
	if err != nil {
		return fmt.Errorf("inspect staged diff: %s", strings.TrimSpace(firstNonEmptyText(diff, err.Error())))
	}

	filesOutput, err := runGit(execCtx, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("inspect staged files: %s", strings.TrimSpace(firstNonEmptyText(filesOutput, err.Error())))
	}
	changedFiles := strings.Fields(filesOutput)
	if strings.TrimSpace(diff) == "" || len(changedFiles) == 0 {
		return fmt.Errorf("opencode completed without producing a staged repository diff")
	}

	artifactWriter.Save(execCtx.Context, "diff", "patch", diff, false)
	artifactWriter.Save(execCtx.Context, "file_bundle", "json", toJSONString(changedFiles), false)

	commitMessage := buildEngineerCommitMessage(execCtx.Story)
	if out, err := runGit(execCtx, "commit", "-m", commitMessage); err != nil {
		return fmt.Errorf("commit repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	if execCtx.Heartbeat != nil {
		_ = execCtx.Heartbeat("pushing_changes")
	}

	branch, err := resolveWorkingBranch(execCtx)
	if err != nil {
		return err
	}
	if out, err := runGit(execCtx, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push repository changes: %s", strings.TrimSpace(firstNonEmptyText(out, err.Error())))
	}

	shaOutput, err := runGit(execCtx, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve commit SHA: %s", strings.TrimSpace(firstNonEmptyText(shaOutput, err.Error())))
	}
	sha := strings.TrimSpace(shaOutput)
	execCtx.WorkingBranch = branch
	if execCtx.OnGitPush != nil {
		if err := execCtx.OnGitPush(branch, sha); err != nil {
			return fmt.Errorf("record pushed branch: %w", err)
		}
	}

	persistenceResult := map[string]any{
		"branch":         branch,
		"commit_sha":     sha,
		"commit_message": commitMessage,
		"changed_files":  changedFiles,
	}
	artifactWriter.Save(execCtx.Context, "git_persistence_result", "json", toJSONString(persistenceResult), false)
	return nil
}

func cloneExecutionContext(execCtx *ExecutionContext, ctx context.Context) *ExecutionContext {
	if execCtx == nil {
		return nil
	}
	clone := *execCtx
	clone.Context = ctx
	return &clone
}

func normalizeOpenCodePostRunError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("opencode finalization timed out after %s", openCodePostRunTimeout)
	}
	return err
}

func isEngineerStoryRun(execCtx *ExecutionContext) bool {
	return execCtx != nil && execCtx.Agent != nil && execCtx.Agent.AgentClass == model.AgentClassEngineer && execCtx.Story != nil
}

func resolveWorkingBranch(execCtx *ExecutionContext) (string, error) {
	if execCtx != nil && strings.TrimSpace(execCtx.WorkingBranch) != "" {
		return strings.TrimSpace(execCtx.WorkingBranch), nil
	}
	branchOutput, err := runGit(execCtx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve working branch: %s", strings.TrimSpace(firstNonEmptyText(branchOutput, err.Error())))
	}
	branch := strings.TrimSpace(branchOutput)
	if branch == "" {
		return "", fmt.Errorf("resolve working branch: git returned an empty branch name")
	}
	return branch, nil
}

func buildEngineerCommitMessage(story *model.PMStory) string {
	if story == nil {
		return "tp: apply engineer run changes"
	}
	if story.DisplayID > 0 {
		return fmt.Sprintf("tp: story #%d %s", story.DisplayID, story.Name)
	}
	return fmt.Sprintf("tp: story %s", story.Name)
}

func buildOpenCodeUserPrompt(execCtx *ExecutionContext, userPrompt string) string {
	parts := []string{strings.TrimSpace(userPrompt)}
	if execCtx != nil && execCtx.Agent != nil && execCtx.Agent.AgentClass == model.AgentClassEngineer && execCtx.Story != nil {
		parts = append(parts, "This is an implementation run, not an analysis-only pass. Make the code changes in the repository, run relevant validation when practical, and finish with a concise summary of the concrete files changed.")
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func openCodeAgentName(execCtx *ExecutionContext) string {
	if execCtx == nil || execCtx.Agent == nil {
		return "teampulse"
	}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassProductPlanner:
		return "teampulse-planner"
	case model.AgentClassEngineer:
		return "teampulse-engineer"
	case model.AgentClassReviewer:
		return "teampulse-reviewer"
	case model.AgentClassSupport:
		return "teampulse-support"
	default:
		return "teampulse"
	}
}

func stripANSI(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func sanitizeOpenCodeOutput(value string) string {
	return strings.TrimSpace(stripANSI(value))
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

func buildOpenCodeConfigContent(execCtx *ExecutionContext, modelID, systemPrompt string) (string, error) {
	agentName := openCodeAgentName(execCtx)
	agentConfig := map[string]any{
		"description": openCodeAgentDescription(execCtx),
		"mode":        "primary",
		"prompt":      systemPrompt,
	}
	if modelID != "" {
		agentConfig["model"] = modelID
	}
	if permissions := buildOpenCodePermissions(execCtx); len(permissions) > 0 {
		agentConfig["permission"] = permissions
	}

	config := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"agent": map[string]any{
			agentName: agentConfig,
		},
	}
	if modelID != "" {
		config["model"] = modelID
		config["small_model"] = modelID
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func openCodeAgentDescription(execCtx *ExecutionContext) string {
	if execCtx == nil || execCtx.Agent == nil {
		return "Teampulse runtime agent"
	}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassProductPlanner:
		return "Teampulse planner agent for OpenSpec-style product planning."
	case model.AgentClassEngineer:
		return "Teampulse engineer agent for story implementation runs."
	case model.AgentClassReviewer:
		return "Teampulse reviewer agent for code review and validation."
	case model.AgentClassSupport:
		return "Teampulse support agent for structured support triage."
	default:
		return "Teampulse runtime agent"
	}
}

func buildOpenCodePermissions(execCtx *ExecutionContext) map[string]any {
	if execCtx == nil || execCtx.Agent == nil {
		return nil
	}

	permissions := map[string]any{}
	switch execCtx.Agent.AgentClass {
	case model.AgentClassEngineer:
		permissions["edit"] = "allow"
	case model.AgentClassProductPlanner, model.AgentClassReviewer, model.AgentClassSupport:
		permissions["edit"] = "deny"
	}

	switch execCtx.Agent.AgentClass {
	case model.AgentClassSupport:
		permissions["bash"] = "deny"
	default:
		if bashRules := buildOpenCodeBashPermissions(execCtx, runtimeProfileFor(execCtx), execCtx.Config); len(bashRules) > 0 {
			permissions["bash"] = bashRules
		}
	}

	return permissions
}

func runtimeProfileFor(execCtx *ExecutionContext) model.RuntimeProfile {
	if execCtx == nil {
		return GetRuntimeProfile("")
	}
	if execCtx.RuntimeProfile.Name != "" {
		return execCtx.RuntimeProfile
	}
	if execCtx.Agent != nil {
		return GetRuntimeProfile(execCtx.Agent.CapabilityProfile)
	}
	return GetRuntimeProfile("")
}

func buildOpenCodeBashPermissions(execCtx *ExecutionContext, profile model.RuntimeProfile, config *WorkflowConfig) map[string]string {
	allowed := allowedCommandsFor(profile, config)
	rules := map[string]string{"*": "deny"}
	for _, command := range allowed {
		command = strings.TrimSpace(command)
		if command == "" || command == "git" {
			continue
		}
		rules[command] = "allow"
		rules[command+" *"] = "allow"
	}

	if execCtx != nil && execCtx.Agent != nil {
		switch execCtx.Agent.AgentClass {
		case model.AgentClassEngineer, model.AgentClassProductPlanner, model.AgentClassReviewer:
			for _, pattern := range readOnlyGitPermissionPatterns() {
				rules[pattern] = "allow"
			}
		}
	}

	return rules
}

func readOnlyGitPermissionPatterns() []string {
	return []string{
		"git status", "git status *",
		"git diff", "git diff *",
		"git log", "git log *",
		"git show", "git show *",
		"git rev-parse", "git rev-parse *",
		"git branch", "git branch *",
		"git ls-files", "git ls-files *",
		"git grep", "git grep *",
	}
}

func openCodeRunProducedRepoChanges(execCtx *ExecutionContext) (bool, error) {
	if execCtx == nil || strings.TrimSpace(execCtx.WorkDir) == "" {
		return false, nil
	}

	statusOutput, err := runGit(execCtx, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("inspect repository status: %s", strings.TrimSpace(firstNonEmptyText(statusOutput, err.Error())))
	}
	if strings.TrimSpace(statusOutput) != "" {
		return true, nil
	}

	baseBranch := strings.TrimSpace(execCtx.BaseBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}

	for _, ref := range []string{baseBranch, "origin/" + baseBranch} {
		diffOutput, err := runGit(execCtx, "diff", "--name-only", ref+"...HEAD")
		if err != nil {
			continue
		}
		return strings.TrimSpace(diffOutput) != "", nil
	}

	return false, fmt.Errorf("inspect repository diff: unable to compare HEAD against %q", baseBranch)
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

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for idx, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[idx] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func backgroundContextOnCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil && ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func shortRunID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
