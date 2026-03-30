package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

type codexSessionHost struct {
	executor              *CodexExecutor
	threadStore           *codexThreadStore
	writer                *codexArtifactWriter
	execCtx               *ExecutionContext
	run                   *appmodel.AgentRun
	developerInstructions string
}

type codexPreparedSession struct {
	state         *codexSessionState
	env           []string
	profile       codexResolvedRuntimeProfile
	configContent string
}

func newCodexSessionHost(
	executor *CodexExecutor,
	threadStore *codexThreadStore,
	writer *codexArtifactWriter,
	execCtx *ExecutionContext,
	run *appmodel.AgentRun,
	developerInstructions string,
) *codexSessionHost {
	return &codexSessionHost{
		executor:              executor,
		threadStore:           threadStore,
		writer:                writer,
		execCtx:               execCtx,
		run:                   run,
		developerInstructions: strings.TrimSpace(developerInstructions),
	}
}

func (h *codexSessionHost) Execute() (*ExecutionResult, error) {
	if h == nil || h.executor == nil || h.execCtx == nil || h.run == nil {
		return nil, fmt.Errorf("codex session host is not configured")
	}

	ctx := h.execCtx.Context
	state, err := h.threadStore.Load(ctx, h.run)
	if err != nil {
		return nil, err
	}
	session, err := h.prepareSession(state)
	if err != nil {
		return nil, err
	}
	h.writer.Save(ctx, "codex_config", "toml", session.configContent, true)

	client := newCodexAppServerClient(h.executor.commandPath, h.execCtx.WorkDir, session.env)
	if err := client.Start(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			slog.WarnContext(ctx, "failed to close codex app-server",
				"error", closeErr,
				"run_id", h.run.ID)
		}
	}()

	if err := client.Initialize(ctx); err != nil {
		return nil, err
	}

	if err := h.executor.loginSession(ctx, client, session.profile); err != nil {
		return nil, err
	}

	if err := h.startOrResumeThread(ctx, client, session.state, session.profile); err != nil {
		return nil, err
	}

	var mapper *codexEventMapper
	if session.state.PendingRequest != nil {
		mapper, err = h.resumePendingRequest(ctx, client, session.state)
	} else {
		mapper, err = h.startFreshTurn(ctx, client, session.state)
	}
	if mapper == nil && err == nil {
		return nil, fmt.Errorf("codex returned no execution result")
	}

	h.persistRuntimeArtifacts(ctx, client, mapper)
	if mapper != nil && strings.TrimSpace(mapper.LatestDiff()) != "" {
		h.writer.Save(ctx, "codex_diff", "patch", mapper.LatestDiff(), false)
	}

	if err != nil {
		_ = h.clearSessionState(ctx, session.state)
		return nil, err
	}

	completedTurn := mapper.CompletedTurn()
	if mapper.PendingRequest() != nil {
		session.state.PendingRequest = mapper.PendingRequest()
		if saveErr := h.threadStore.Save(ctx, h.run, session.state); saveErr != nil {
			return nil, saveErr
		}
		return mapper.Result(), nil
	}

	if completedTurn == nil {
		return nil, fmt.Errorf("codex turn ended without completion or pending request")
	}

	switch strings.ToLower(strings.TrimSpace(completedTurn.Status)) {
	case "completed":
		if err := h.clearSessionState(ctx, session.state); err != nil {
			return nil, err
		}
		return mapper.Result(), nil
	case "interrupted":
		if err := h.clearSessionState(ctx, session.state); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("codex turn was interrupted")
	default:
		message := "codex turn failed"
		if completedTurn.Error != nil && strings.TrimSpace(completedTurn.Error.Message) != "" {
			message = strings.TrimSpace(completedTurn.Error.Message)
		}
		_ = h.clearSessionState(ctx, session.state)
		return nil, fmt.Errorf("%s", message)
	}
}

func (h *codexSessionHost) prepareSession(existing *codexSessionState) (*codexPreparedSession, error) {
	state := &codexSessionState{}
	if existing != nil {
		*state = *existing
	}

	runRoot := strings.TrimSpace(state.HomeRoot)
	if runRoot == "" {
		runRoot = filepath.Join(os.TempDir(), "helpin-codex", sanitizeCodexPathComponent(h.run.ID), "home")
	}
	codexHome := strings.TrimSpace(state.CodexHome)
	if codexHome == "" {
		codexHome = filepath.Join(runRoot, ".codex")
	}
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		return nil, fmt.Errorf("create codex session home: %w", err)
	}

	profile, err := h.executor.resolveRuntimeProfile(h.execCtx.Agent)
	if err != nil {
		return nil, err
	}

	state.HomeRoot = runRoot
	state.CodexHome = codexHome
	state.Provider = profile.Provider
	state.Model = profile.Model
	state.AuthMode = profile.AuthMode
	state.Sandbox = codexSandboxMode(h.execCtx)
	state.InvocationMode = strings.TrimSpace(h.run.InvocationMode)

	configContent, err := h.executor.buildConfigArtifact(h.execCtx, profile, codexApprovalPolicyForRun(h.run))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(configContent), 0o600); err != nil {
		return nil, fmt.Errorf("write codex config.toml: %w", err)
	}

	env := h.executor.buildBaseEnv()
	env = upsertEnv(env, "HOME", runRoot)
	env = upsertEnv(env, "CODEX_HOME", codexHome)
	env = h.executor.upsertProviderEnv(env, profile.Provider)

	return &codexPreparedSession{
		state:         state,
		env:           env,
		profile:       profile,
		configContent: configContent,
	}, nil
}

func (h *codexSessionHost) startOrResumeThread(ctx context.Context, client *codexAppServerClient, state *codexSessionState, profile codexResolvedRuntimeProfile) error {
	approvalPolicy := codexApprovalPolicyForRun(h.run)
	params := map[string]any{
		"cwd":                   h.execCtx.WorkDir,
		"modelProvider":         profile.Provider,
		"approvalPolicy":        approvalPolicy,
		"approvalsReviewer":     "user",
		"sandbox":               codexSandboxMode(h.execCtx),
		"serviceName":           "Helpin",
		"developerInstructions": h.developerInstructions,
	}
	if strings.TrimSpace(profile.Model) != "" {
		params["model"] = profile.Model
	}

	var raw json.RawMessage
	var err error
	if strings.TrimSpace(state.ThreadID) == "" {
		raw, err = client.Request(ctx, "thread/start", params)
	} else {
		params["threadId"] = state.ThreadID
		raw, err = client.Request(ctx, "thread/resume", params)
	}
	if err != nil {
		return err
	}

	var response codexThreadLifecycleResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decode codex thread response: %w", err)
	}
	state.ThreadID = strings.TrimSpace(response.Thread.ID)
	if response.Thread.Path != nil {
		state.ThreadPath = strings.TrimSpace(*response.Thread.Path)
	}
	if strings.TrimSpace(response.ModelProvider) != "" {
		state.Provider = strings.TrimSpace(response.ModelProvider)
	}
	if strings.TrimSpace(response.Model) != "" {
		state.Model = strings.TrimSpace(response.Model)
	}
	return h.threadStore.Save(ctx, h.run, state)
}

func (h *codexSessionHost) startFreshTurn(ctx context.Context, client *codexAppServerClient, state *codexSessionState) (*codexEventMapper, error) {
	sequenceNo, input := latestPendingUserMessage(h.execCtx.ConversationHistory, state.LastSubmittedMessageSeqNo)
	if sequenceNo <= 0 || strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("no pending user turn found for codex run")
	}
	h.writer.Save(ctx, "codex_prompt", "markdown", buildCodexPromptArtifact(h.developerInstructions, input, nil), false)

	raw, err := client.Request(ctx, "turn/start", map[string]any{
		"threadId": state.ThreadID,
		"input": []map[string]any{{
			"type":          "text",
			"text":          input,
			"text_elements": []any{},
		}},
	})
	if err != nil {
		return nil, err
	}

	var response codexTurnResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode codex turn response: %w", err)
	}
	if strings.TrimSpace(response.Turn.ID) == "" {
		return nil, fmt.Errorf("codex turn/start returned an empty turn id")
	}
	state.LastSubmittedMessageSeqNo = sequenceNo
	if err := h.threadStore.Save(ctx, h.run, state); err != nil {
		return nil, err
	}
	return h.processTurn(ctx, client)
}

func (h *codexSessionHost) resumePendingRequest(ctx context.Context, client *codexAppServerClient, state *codexSessionState) (*codexEventMapper, error) {
	if state.PendingRequest == nil {
		return nil, fmt.Errorf("missing pending request state")
	}

	promptText := ""
	if seq, input := latestPendingUserMessage(h.execCtx.ConversationHistory, state.LastSubmittedMessageSeqNo); seq > 0 {
		promptText = input
	}
	h.writer.Save(ctx, "codex_prompt", "markdown", buildCodexPromptArtifact(h.developerInstructions, promptText, state.PendingRequest), false)

	replayedRequest, err := h.awaitPendingRequestReplay(ctx, client)
	if err != nil {
		return nil, err
	}

	responsePayload, followupSequenceNo, followupInput, consumedSequenceNo, err := h.pendingResponsePayload(state.PendingRequest, state.LastSubmittedMessageSeqNo)
	if err != nil {
		return nil, err
	}
	if err := client.Respond(ctx, replayedRequest.ID, responsePayload); err != nil {
		return nil, err
	}

	if consumedSequenceNo > 0 {
		state.LastSubmittedMessageSeqNo = consumedSequenceNo
	}
	state.PendingRequest = nil
	if err := h.threadStore.Save(ctx, h.run, state); err != nil {
		return nil, err
	}

	mapper, err := h.processTurn(ctx, client)
	if err != nil {
		return mapper, err
	}
	if followupSequenceNo <= 0 || strings.TrimSpace(followupInput) == "" {
		return mapper, nil
	}
	completedTurn := mapper.CompletedTurn()
	if completedTurn == nil || strings.ToLower(strings.TrimSpace(completedTurn.Status)) != "interrupted" {
		return mapper, nil
	}

	raw, err := client.Request(ctx, "turn/start", map[string]any{
		"threadId": state.ThreadID,
		"input": []map[string]any{{
			"type":          "text",
			"text":          followupInput,
			"text_elements": []any{},
		}},
	})
	if err != nil {
		return mapper, err
	}
	var response codexTurnResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return mapper, fmt.Errorf("decode codex follow-up turn response: %w", err)
	}
	if strings.TrimSpace(response.Turn.ID) == "" {
		return mapper, fmt.Errorf("codex follow-up turn/start returned an empty turn id")
	}
	state.LastSubmittedMessageSeqNo = followupSequenceNo
	if err := h.threadStore.Save(ctx, h.run, state); err != nil {
		return mapper, err
	}
	return h.processTurn(ctx, client)
}

func (h *codexSessionHost) processTurn(ctx context.Context, client *codexAppServerClient) (*codexEventMapper, error) {
	mapper := newCodexEventMapper(h.execCtx, h.run, h.writer)
	for {
		msg, err := client.Next(ctx)
		if err != nil {
			if err == io.EOF && mapper.CompletedTurn() != nil {
				return mapper, nil
			}
			if ctx.Err() != nil {
				return mapper, ErrRunCancelled
			}
			return mapper, fmt.Errorf("read codex app-server event: %w", err)
		}
		if strings.TrimSpace(msg.Method) == "" {
			continue
		}
		if handled, err := h.handleManagedRequest(ctx, client, msg); handled {
			if err != nil {
				return mapper, err
			}
			continue
		}
		if isCodexPauseRequestMethod(msg.Method) {
			if err := mapper.HandleRequest(msg.Method, msg.ID, msg.Params); err != nil {
				return mapper, err
			}
			return mapper, nil
		}
		if err := mapper.HandleNotification(ctx, msg.Method, msg.Params); err != nil {
			return mapper, err
		}
		if mapper.CompletedTurn() != nil {
			return mapper, nil
		}
	}
}

func (h *codexSessionHost) awaitPendingRequestReplay(ctx context.Context, client *codexAppServerClient) (codexRPCMessage, error) {
	mapper := newCodexEventMapper(h.execCtx, h.run, h.writer)
	for {
		msg, err := client.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return codexRPCMessage{}, ErrRunCancelled
			}
			return codexRPCMessage{}, fmt.Errorf("await codex pending request replay: %w", err)
		}
		if strings.TrimSpace(msg.Method) == "" {
			continue
		}
		if handled, err := h.handleManagedRequest(ctx, client, msg); handled {
			if err != nil {
				return codexRPCMessage{}, err
			}
			continue
		}
		if isCodexPauseRequestMethod(msg.Method) {
			return msg, nil
		}
		if err := mapper.HandleNotification(ctx, msg.Method, msg.Params); err != nil {
			return codexRPCMessage{}, err
		}
		if mapper.CompletedTurn() != nil {
			return codexRPCMessage{}, fmt.Errorf("codex completed the turn before replaying pending request")
		}
	}
}

func (h *codexSessionHost) handleManagedRequest(ctx context.Context, client *codexAppServerClient, msg codexRPCMessage) (bool, error) {
	switch strings.TrimSpace(msg.Method) {
	case "account/chatgptAuthTokens/refresh":
		var params codexChatGPTAuthTokensRefreshParams
		if len(msg.Params) > 0 {
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return true, fmt.Errorf("decode chatgpt auth refresh params: %w", err)
			}
		}
		_ = params
		payload, err := h.executor.chatGPTAuthPayload()
		if err != nil {
			return true, err
		}
		if err := client.Respond(ctx, msg.ID, payload); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func (h *codexSessionHost) pendingResponsePayload(pending *codexPendingRequest, afterSequenceNo int) (response any, followupSequenceNo int, followupInput string, consumedSequenceNo int, err error) {
	sequenceNo, input := latestPendingUserMessage(h.execCtx.ConversationHistory, afterSequenceNo)
	switch pending.Kind {
	case codexPendingRequestKindHumanInput:
		if sequenceNo <= 0 || strings.TrimSpace(input) == "" {
			return nil, 0, "", 0, fmt.Errorf("human input request is missing a reply")
		}
		response, err := codexParseUserInputResponse(pending, input)
		if err != nil {
			return nil, 0, "", 0, err
		}
		return response, 0, "", sequenceNo, nil
	case codexPendingRequestKindCommandApproval, codexPendingRequestKindFileApproval, codexPendingRequestKindPermissions:
		requestChanges := h.run.ApprovalState == "rejected" && strings.TrimSpace(input) != ""
		approved := h.run.ApprovalState == "approved"
		response, err := codexApprovalResponse(pending, approved, requestChanges)
		if err != nil {
			return nil, 0, "", 0, err
		}
		if requestChanges {
			return response, sequenceNo, input, 0, nil
		}
		if sequenceNo > 0 {
			return response, 0, "", sequenceNo, nil
		}
		return response, 0, "", 0, nil
	default:
		return nil, 0, "", 0, fmt.Errorf("unsupported pending request kind %q", pending.Kind)
	}
}

func (h *codexSessionHost) persistRuntimeArtifacts(ctx context.Context, client *codexAppServerClient, mapper *codexEventMapper) {
	if mapper == nil {
		return
	}
	stdoutText := strings.TrimSpace(mapper.Stdout())
	stderrText := strings.TrimSpace(joinNonEmptyLines(mapper.Stderr(), client.Stderr()))
	if stdoutText != "" {
		h.writer.Save(ctx, "codex_stdout", "text", stdoutText, false)
	}
	if stderrText != "" {
		h.writer.Save(ctx, "codex_stderr", "text", stderrText, false)
	}
}

func (h *codexSessionHost) clearSessionState(ctx context.Context, state *codexSessionState) error {
	if h.threadStore == nil {
		return nil
	}
	if state != nil && strings.TrimSpace(state.HomeRoot) != "" {
		_ = os.RemoveAll(strings.TrimSpace(state.HomeRoot))
	}
	return h.threadStore.Clear(ctx, h.run)
}

func buildCodexPromptArtifact(developerInstructions, input string, pending *codexPendingRequest) string {
	sections := make([]string, 0, 3)
	if strings.TrimSpace(developerInstructions) != "" {
		sections = append(sections, "Developer instructions:\n"+strings.TrimSpace(developerInstructions))
	}
	if strings.TrimSpace(input) != "" {
		sections = append(sections, "Turn input:\n"+strings.TrimSpace(input))
	}
	if pending != nil {
		sections = append(sections, fmt.Sprintf(
			"Pending request replay:\n- kind: %s\n- turn_id: %s\n- item_id: %s",
			strings.TrimSpace(pending.Kind),
			strings.TrimSpace(pending.TurnID),
			strings.TrimSpace(pending.ItemID),
		))
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func latestPendingUserMessage(history []ExecutionMessage, afterSequenceNo int) (int, string) {
	latestSequence := 0
	latestContent := ""
	for _, message := range history {
		if strings.TrimSpace(message.Role) != "user" {
			continue
		}
		if message.SequenceNo <= afterSequenceNo || message.SequenceNo <= 0 {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		if message.SequenceNo >= latestSequence {
			latestSequence = message.SequenceNo
			latestContent = strings.TrimSpace(message.Content)
		}
	}
	return latestSequence, latestContent
}

func isCodexPauseRequestMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/tool/requestUserInput", "item/permissions/requestApproval":
		return true
	default:
		return false
	}
}

func codexApprovalPolicyForRun(run *appmodel.AgentRun) string {
	if run != nil && strings.TrimSpace(run.InvocationMode) == appmodel.InvocationModeInteractive {
		return "untrusted"
	}
	return "on-request"
}

func sanitizeCodexPathComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "session"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return "session"
	}
	return sanitized
}

func joinNonEmptyLines(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n")
}

func (e *CodexExecutor) selectedModelID(agent *appmodel.Agent) string {
	if requested := e.requestedModelID(agent); strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	defaultModel := strings.TrimSpace(e.defaultModel)
	if defaultModel == "" {
		return ""
	}
	return normalizeOpenCodeConfiguredModelName(e.resolveProvider(agent), defaultModel)
}
