package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const codexAppServerRequestTimeout = 30 * time.Second
const codexTurnEventIdleTimeout = 5 * time.Minute

var codexPendingReplayGraceTimeout = 2 * time.Second

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
	h.appendRuntimeProgress(ctx, "Starting Codex app-server.\n")

	client := newCodexAppServerClient(h.executor.commandPath, h.execCtx.WorkDir, session.env)
	if err := client.Start(ctx); err != nil {
		if result := h.recoverWithCodexAuthRetry(ctx, err, session.state, session.profile); result != nil {
			return result, nil
		}
		return nil, err
	}
	if h.execCtx.Heartbeat != nil {
		_ = h.execCtx.Heartbeat("codex_appserver_started")
	}
	var mapper *codexEventMapper
	defer func() {
		h.persistRuntimeArtifacts(ctx, client, mapper)
		if closeErr := client.Close(); closeErr != nil {
			slog.WarnContext(ctx, "failed to close codex app-server",
				"error", closeErr,
				"run_id", h.run.ID)
		}
	}()

	h.appendRuntimeProgress(ctx, "Initializing Codex session.\n")
	initCtx, cancelInitialize := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancelInitialize()
	if err := client.Initialize(initCtx); err != nil {
		if result := h.recoverWithCodexAuthRetry(ctx, err, session.state, session.profile); result != nil {
			return result, nil
		}
		return nil, err
	}

	authResult, err := h.ensureAuthenticated(ctx, client, session.state, session.profile)
	if err != nil {
		if result := h.recoverWithCodexAuthRetry(ctx, err, session.state, session.profile); result != nil {
			return result, nil
		}
		return nil, err
	}
	if authResult != nil {
		if err := h.threadStore.Save(ctx, h.run, session.state); err != nil {
			return nil, err
		}
		return authResult, nil
	}

	if err := h.startOrResumeThread(ctx, client, session.state, session.profile); err != nil {
		if result := h.recoverWithCodexAuthRetry(ctx, err, session.state, session.profile); result != nil {
			return result, nil
		}
		return nil, err
	}

	if session.state.PendingRequest != nil {
		mapper, err = h.resumePendingRequest(ctx, client, session.state)
	} else {
		mapper, err = h.startFreshTurn(ctx, client, session.state)
	}
	if mapper == nil && err == nil {
		return nil, fmt.Errorf("codex returned no execution result")
	}

	if mapper != nil && strings.TrimSpace(mapper.LatestDiff()) != "" {
		h.writer.Save(ctx, "codex_diff", "patch", mapper.LatestDiff(), false)
	}

	if err != nil {
		if result := h.recoverWithCodexAuthRetry(ctx, err, session.state, session.profile); result != nil {
			return result, nil
		}
		_ = h.clearSessionState(ctx, session.state)
		return nil, err
	}

	completedTurn := mapper.CompletedTurn()
	if mapper.PendingRequest() != nil {
		h.persistWorkspaceAuth(ctx, session.state)
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
		if codexShouldReauthForMessage(session.profile, message) {
			result := h.codexAuthRequiredState(ctx, session.state, session.profile, "ChatGPT authentication needs to be refreshed. Sign in with ChatGPT again.")
			return result, nil
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
	state.Sandbox = h.executor.sandboxModeFor(h.execCtx)
	state.InvocationMode = strings.TrimSpace(h.run.InvocationMode)

	configContent, err := h.executor.buildConfigArtifact(h.execCtx, profile, codexApprovalPolicyForRun(h.run))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(configContent), 0o600); err != nil {
		return nil, fmt.Errorf("write codex config.toml: %w", err)
	}
	if stagedRoot := strings.TrimSpace(h.execCtx.StagedRuntimeSkillRoot); stagedRoot != "" {
		if err := SyncRuntimeSkillRoot(stagedRoot, filepath.Join(codexHome, "skills", "helpin")); err != nil {
			return nil, fmt.Errorf("sync staged codex skills: %w", err)
		}
	}
	if err := h.executor.restoreWorkspaceAuth(h.execCtx.Context, h.run.WorkspaceID, profile.Provider, profile.AuthMode, codexHome); err != nil {
		return nil, err
	}

	env := h.executor.buildBaseEnv()
	env = upsertEnv(env, "HOME", runRoot)
	env = upsertEnv(env, "CODEX_HOME", codexHome)
	env = h.executor.upsertProviderEnv(env, profile.Provider)
	guardBinDir, err := installCodexCommandGuards(runRoot)
	if err != nil {
		return nil, err
	}
	env = prependPathEnv(env, guardBinDir)

	return &codexPreparedSession{
		state:         state,
		env:           env,
		profile:       profile,
		configContent: configContent,
	}, nil
}

func (h *codexSessionHost) ensureAuthenticated(ctx context.Context, client *codexAppServerClient, state *codexSessionState, profile codexResolvedRuntimeProfile) (*ExecutionResult, error) {
	switch strings.TrimSpace(profile.Provider) {
	case appmodel.AgentModelProviderOpenAI:
		switch strings.TrimSpace(profile.AuthMode) {
		case codexOpenAIAuthModeDevice:
			authState, authenticated, err := h.readAccountAuthState(ctx, client, state, profile)
			if err != nil {
				return nil, err
			}
			if authenticated {
				return nil, nil
			}
			if authState == nil {
				authState = &appmodel.CodexAuthState{
					Provider:  strings.TrimSpace(profile.Provider),
					AuthMode:  strings.TrimSpace(profile.AuthMode),
					State:     appmodel.CodexAuthStateRequired,
					UpdatedAt: time.Now().UTC(),
				}
			}
			return &ExecutionResult{
				AssistantText:  "Sign in with ChatGPT to continue this Codex run.",
				CodexAuthState: authState,
			}, nil
		default:
			authCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
			defer cancel()
			return nil, h.executor.loginSession(authCtx, client, profile)
		}
	default:
		return nil, nil
	}
}

func (h *codexSessionHost) readAccountAuthState(ctx context.Context, client *codexAppServerClient, state *codexSessionState, profile codexResolvedRuntimeProfile) (*appmodel.CodexAuthState, bool, error) {
	requestCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancel()
	raw, err := client.Request(requestCtx, "account/read", map[string]any{
		"refreshToken": false,
	})
	if err != nil {
		if codexShouldReauthForError(profile, err) {
			codexClearRecoveredAuthState(ctx, h.executor, h.run.WorkspaceID, state, profile)
			return codexBuildReauthRequiredState(profile, "ChatGPT authentication needs to be refreshed."), false, nil
		}
		return nil, false, err
	}

	var response codexAccountReadResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, false, fmt.Errorf("decode codex account/read response: %w", err)
	}

	if !response.RequiresOpenAIAuth {
		return nil, true, nil
	}

	if response.Account != nil && strings.TrimSpace(response.Account.Type) != "" {
		state := &appmodel.CodexAuthState{
			Provider:  strings.TrimSpace(profile.Provider),
			AuthMode:  strings.TrimSpace(profile.AuthMode),
			State:     appmodel.CodexAuthStateConnected,
			UpdatedAt: time.Now().UTC(),
		}
		if response.Account.PlanType != nil && strings.TrimSpace(*response.Account.PlanType) != "" {
			planType := strings.TrimSpace(*response.Account.PlanType)
			state.PlanType = &planType
		}
		return state, true, nil
	}

	return &appmodel.CodexAuthState{
		Provider:  strings.TrimSpace(profile.Provider),
		AuthMode:  strings.TrimSpace(profile.AuthMode),
		State:     appmodel.CodexAuthStateRequired,
		UpdatedAt: time.Now().UTC(),
	}, false, nil
}

func (h *codexSessionHost) recoverWithCodexAuthRetry(ctx context.Context, err error, state *codexSessionState, profile codexResolvedRuntimeProfile) *ExecutionResult {
	if !codexShouldReauthForError(profile, err) {
		return nil
	}
	return h.codexAuthRequiredState(ctx, state, profile, "ChatGPT authentication needs to be refreshed. Sign in with ChatGPT again.")
}

func (h *codexSessionHost) codexAuthRequiredState(ctx context.Context, state *codexSessionState, profile codexResolvedRuntimeProfile, message string) *ExecutionResult {
	codexClearRecoveredAuthState(ctx, h.executor, h.run.WorkspaceID, state, profile)
	return &ExecutionResult{
		CodexAuthState: codexBuildReauthRequiredState(profile, message),
		AssistantText:  "Sign in with ChatGPT to continue this Codex run.",
	}
}

func (h *codexSessionHost) startOrResumeThread(ctx context.Context, client *codexAppServerClient, state *codexSessionState, profile codexResolvedRuntimeProfile) error {
	approvalPolicy := codexApprovalPolicyForRun(h.run)
	params := map[string]any{
		"cwd":                   h.execCtx.WorkDir,
		"modelProvider":         profile.Provider,
		"approvalPolicy":        approvalPolicy,
		"approvalsReviewer":     "user",
		"sandbox":               h.executor.sandboxModeFor(h.execCtx),
		"serviceName":           "Helpin",
		"developerInstructions": h.developerInstructions,
	}
	if strings.TrimSpace(profile.Model) != "" {
		params["model"] = profile.Model
	}

	var raw json.RawMessage
	var err error
	requestCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancel()
	if strings.TrimSpace(state.ThreadID) == "" {
		h.appendRuntimeProgress(ctx, "Starting Codex thread.\n")
		raw, err = client.Request(requestCtx, "thread/start", params)
	} else {
		params["threadId"] = state.ThreadID
		h.appendRuntimeProgress(ctx, "Resuming Codex thread.\n")
		raw, err = client.Request(requestCtx, "thread/resume", params)
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

	h.appendRuntimeProgress(ctx, "Starting Codex turn.\n")
	requestCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancel()
	raw, err := client.Request(requestCtx, "turn/start", map[string]any{
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
	return h.processTurn(ctx, client, state)
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

	responsePayload, followupSequenceNo, followupInput, consumedSequenceNo, err := h.pendingResponsePayload(state.PendingRequest, state.LastSubmittedMessageSeqNo)
	if err != nil {
		return nil, err
	}
	responseID, err := h.pendingResponseRequestID(ctx, client, state.PendingRequest)
	if err != nil {
		return nil, err
	}
	respondCtx, cancelRespond := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancelRespond()
	if err := client.Respond(respondCtx, responseID, responsePayload); err != nil {
		return nil, err
	}

	if consumedSequenceNo > 0 {
		state.LastSubmittedMessageSeqNo = consumedSequenceNo
	}
	state.PendingRequest = nil
	if err := h.threadStore.Save(ctx, h.run, state); err != nil {
		return nil, err
	}

	mapper, err := h.processTurn(ctx, client, state)
	if err != nil {
		return mapper, err
	}
	return h.startFollowupTurn(ctx, client, state, mapper, followupInput, followupSequenceNo)
}

func (h *codexSessionHost) pendingResponseRequestID(ctx context.Context, client *codexAppServerClient, pending *codexPendingRequest) (json.RawMessage, error) {
	if pending == nil {
		return nil, fmt.Errorf("missing pending request state")
	}
	storedID := codexPendingRequestResponseID(pending)
	if len(storedID) == 0 {
		replayedRequest, err := h.awaitPendingRequestReplay(ctx, client)
		if err != nil {
			return nil, err
		}
		return replayedRequest.ID, nil
	}

	replayCtx, cancel := context.WithTimeout(ctx, codexPendingReplayGraceTimeout)
	defer cancel()
	replayedRequest, err := h.awaitPendingRequestReplay(replayCtx, client)
	if err == nil {
		if len(storedID) > 0 && !jsonRawEqual(replayedRequest.ID, storedID) {
			slog.WarnContext(ctx, "codex pending request replayed with a different request id",
				"run_id", h.run.ID,
				"request_kind", strings.TrimSpace(pending.Kind),
				"stored_request_id", strings.TrimSpace(string(storedID)),
				"replayed_request_id", strings.TrimSpace(string(replayedRequest.ID)))
		}
		return replayedRequest.ID, nil
	}
	if ctx.Err() != nil {
		return nil, ErrRunCancelled
	}
	if isCodexPendingReplayTimeout(err) {
		return nil, codexPendingReplayProtocolError(pending)
	}
	return nil, err
}

func (h *codexSessionHost) processTurn(ctx context.Context, client *codexAppServerClient, state *codexSessionState) (*codexEventMapper, error) {
	mapper := newCodexEventMapper(h.execCtx, h.run, h.writer)
	for {
		msg, err := h.nextTurnMessage(ctx, client, "turn execution")
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
			if h.execCtx != nil && h.execCtx.HandleInteractivePause != nil {
				pending := mapper.PendingRequest()
				if pending == nil {
					return mapper, fmt.Errorf("codex pause request did not produce pending request state")
				}
				if state != nil && h.threadStore != nil {
					h.persistWorkspaceAuth(ctx, state)
					state.PendingRequest = pending
					if err := h.threadStore.Save(ctx, h.run, state); err != nil {
						return mapper, err
					}
				}
				signal, err := h.execCtx.HandleInteractivePause(mapper.Result())
				if err != nil {
					return mapper, err
				}
				responsePayload, followupInput, err := h.pendingResponsePayloadForSignal(pending, signal)
				if err != nil {
					return mapper, err
				}
				if state != nil && h.threadStore != nil {
					state.PendingRequest = nil
					if err := h.threadStore.Save(ctx, h.run, state); err != nil {
						return mapper, err
					}
				}
				respondCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
				err = client.Respond(respondCtx, msg.ID, responsePayload)
				cancel()
				if err != nil {
					return mapper, err
				}
				if signal.Acknowledge != nil {
					if err := signal.Acknowledge(); err != nil {
						return mapper, err
					}
				}
				resumedMapper, err := h.processTurn(ctx, client, state)
				if resumedMapper == nil {
					return mapper, err
				}
				if resumedMapper.result.Usage == (ExecutionUsage{}) {
					resumedMapper.result.Usage = mapper.result.Usage
				}
				if strings.TrimSpace(resumedMapper.latestDiff) == "" {
					resumedMapper.latestDiff = mapper.latestDiff
				}
				return h.startFollowupTurn(ctx, client, state, resumedMapper, followupInput, 0)
			}
			return mapper, nil
		}
		if isUnhandledCodexServerRequest(msg) {
			return mapper, unsupportedCodexServerRequestError(msg)
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
		msg, err := h.nextTurnMessage(ctx, client, "pending request replay")
		if err != nil {
			if h.execCtx != nil && h.execCtx.Context != nil && h.execCtx.Context.Err() != nil {
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
		if isUnhandledCodexServerRequest(msg) {
			return codexRPCMessage{}, unsupportedCodexServerRequestError(msg)
		}
		if err := mapper.HandleNotification(ctx, msg.Method, msg.Params); err != nil {
			return codexRPCMessage{}, err
		}
		if mapper.CompletedTurn() != nil {
			return codexRPCMessage{}, fmt.Errorf("codex completed the turn before replaying pending request")
		}
	}
}

func (h *codexSessionHost) nextTurnMessage(ctx context.Context, client *codexAppServerClient, phase string) (codexRPCMessage, error) {
	if client == nil {
		return codexRPCMessage{}, fmt.Errorf("codex app-server client is not configured")
	}
	waitCtx, cancel := context.WithTimeout(ctx, codexTurnEventIdleTimeout)
	defer cancel()
	msg, err := client.Next(waitCtx)
	if err == nil {
		return msg, nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
		return codexRPCMessage{}, fmt.Errorf("timed out waiting for codex app-server events during %s after %s", strings.TrimSpace(phase), codexTurnEventIdleTimeout)
	}
	return codexRPCMessage{}, err
}

func isCodexPendingReplayTimeout(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timed out waiting for codex app-server events during pending request replay")
}

func codexPendingReplayProtocolError(pending *codexPendingRequest) error {
	kind := "server request"
	if pending != nil && strings.TrimSpace(pending.Kind) != "" {
		kind = strings.TrimSpace(pending.Kind)
	}
	return fmt.Errorf("codex did not replay the pending %s after thread/resume; Helpin starts a fresh app-server process for resumed runs, but Codex only replays pending server requests when reattaching to a still-running thread", kind)
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
		respondCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
		defer cancel()
		if err := client.Respond(respondCtx, msg.ID, payload); err != nil {
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

func (h *codexSessionHost) pendingResponsePayloadForSignal(pending *codexPendingRequest, signal *LiveExecutionResumeSignal) (response any, followupInput string, err error) {
	if pending == nil {
		return nil, "", fmt.Errorf("missing pending request state")
	}
	if signal == nil {
		return nil, "", fmt.Errorf("missing live resume signal")
	}
	if len(signal.ResponsePayload) > 0 {
		var responsePayload any
		if err := json.Unmarshal(signal.ResponsePayload, &responsePayload); err != nil {
			return nil, "", fmt.Errorf("parse live resume response payload: %w", err)
		}
		if strings.TrimSpace(signal.Intent) == appmodel.AgentRunResumeIntentRequestChanges && strings.TrimSpace(signal.Content) != "" {
			return responsePayload, strings.TrimSpace(signal.Content), nil
		}
		return responsePayload, "", nil
	}

	switch pending.Kind {
	case codexPendingRequestKindHumanInput:
		if strings.TrimSpace(signal.Intent) != appmodel.AgentRunResumeIntentReply {
			return nil, "", fmt.Errorf("human input request requires reply intent, got %q", strings.TrimSpace(signal.Intent))
		}
		response, err := codexParseUserInputResponse(pending, signal.Content)
		if err != nil {
			return nil, "", err
		}
		return response, "", nil
	case codexPendingRequestKindCommandApproval, codexPendingRequestKindFileApproval, codexPendingRequestKindPermissions:
		intent := strings.TrimSpace(signal.Intent)
		approved := intent == appmodel.AgentRunResumeIntentApprove
		requestChanges := intent == appmodel.AgentRunResumeIntentRequestChanges && strings.TrimSpace(signal.Content) != ""
		response, err := codexApprovalResponse(pending, approved, requestChanges)
		if err != nil {
			return nil, "", err
		}
		if requestChanges {
			return response, strings.TrimSpace(signal.Content), nil
		}
		return response, "", nil
	default:
		return nil, "", fmt.Errorf("unsupported pending request kind %q", pending.Kind)
	}
}

func (h *codexSessionHost) startFollowupTurn(ctx context.Context, client *codexAppServerClient, state *codexSessionState, mapper *codexEventMapper, followupInput string, followupSequenceNo int) (*codexEventMapper, error) {
	if mapper == nil || !shouldStartCodexFollowupTurn(mapper.CompletedTurn(), followupInput) {
		return mapper, nil
	}

	h.appendRuntimeProgress(ctx, "Starting follow-up Codex turn.\n")
	requestCtx, cancel := context.WithTimeout(ctx, codexAppServerRequestTimeout)
	defer cancel()
	raw, err := client.Request(requestCtx, "turn/start", map[string]any{
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
	if followupSequenceNo > 0 {
		state.LastSubmittedMessageSeqNo = followupSequenceNo
	}
	if err := h.threadStore.Save(ctx, h.run, state); err != nil {
		return mapper, err
	}
	return h.processTurn(ctx, client, state)
}

func shouldStartCodexFollowupTurn(completedTurn *codexTurn, followupInput string) bool {
	if completedTurn == nil || strings.TrimSpace(followupInput) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(completedTurn.Status)) {
	case "completed", "interrupted":
		return true
	default:
		return false
	}
}

func (h *codexSessionHost) persistRuntimeArtifacts(ctx context.Context, client *codexAppServerClient, mapper *codexEventMapper) {
	if client == nil {
		return
	}
	stdoutText := ""
	stderrText := strings.TrimSpace(client.Stderr())
	if mapper != nil {
		stdoutText = strings.TrimSpace(mapper.Stdout())
		stderrText = strings.TrimSpace(joinNonEmptyLines(mapper.Stderr(), stderrText))
	}
	if stdoutText != "" {
		h.writer.Save(ctx, "codex_stdout", "text", stdoutText, false)
	}
	if stderrText != "" {
		h.writer.Save(ctx, "codex_stderr", "text", stderrText, false)
	}
}

func (h *codexSessionHost) appendRuntimeProgress(ctx context.Context, text string) {
	if h == nil || h.writer == nil || strings.TrimSpace(text) == "" {
		return
	}
	h.writer.Save(ctx, "codex_stdout_chunk", "text", text, true)
}

func (h *codexSessionHost) clearSessionState(ctx context.Context, state *codexSessionState) error {
	if h.threadStore == nil {
		return nil
	}
	h.persistWorkspaceAuth(ctx, state)
	if state != nil && strings.TrimSpace(state.HomeRoot) != "" {
		_ = os.RemoveAll(strings.TrimSpace(state.HomeRoot))
	}
	return h.threadStore.Clear(ctx, h.run)
}

func (h *codexSessionHost) persistWorkspaceAuth(ctx context.Context, state *codexSessionState) {
	if h == nil || h.run == nil || state == nil || h.executor == nil {
		return
	}
	if err := h.executor.persistWorkspaceAuth(ctx, h.run.WorkspaceID, state.Provider, state.AuthMode, state.CodexHome); err != nil {
		slog.WarnContext(ctx, "failed to persist workspace codex auth",
			"error", err,
			"workspace_id", h.run.WorkspaceID,
			"run_id", h.run.ID)
	}
}

func buildCodexPromptArtifact(developerInstructions, input string, pending *codexPendingRequest) string {
	sections := make([]string, 0, 3)
	if strings.TrimSpace(developerInstructions) != "" {
		sections = append(sections, "Developer prompt:\n"+strings.TrimSpace(developerInstructions))
	}
	if strings.TrimSpace(input) != "" {
		sections = append(sections, "User prompt:\n"+strings.TrimSpace(input))
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

func isCodexManagedRequestMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case "account/chatgptAuthTokens/refresh":
		return true
	default:
		return false
	}
}

func isUnhandledCodexServerRequest(msg codexRPCMessage) bool {
	method := strings.TrimSpace(msg.Method)
	if method == "" || len(msg.ID) == 0 {
		return false
	}
	if isCodexPauseRequestMethod(method) || isCodexManagedRequestMethod(method) {
		return false
	}
	return true
}

func unsupportedCodexServerRequestError(msg codexRPCMessage) error {
	method := strings.TrimSpace(msg.Method)
	if method == "" {
		method = "unknown"
	}
	return fmt.Errorf("unsupported codex server request %q; Codex is waiting for a client response", method)
}

func codexApprovalPolicyForRun(run *appmodel.AgentRun) string {
	if run != nil && strings.TrimSpace(run.InvocationMode) == appmodel.InvocationModeAutonomous {
		return "never"
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
