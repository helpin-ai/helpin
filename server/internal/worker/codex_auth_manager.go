package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	codexAuthRequestTimeout         = 30 * time.Second
	codexLoginTypeChatGPT           = "chatgpt"
	codexLoginTypeChatGPTDeviceCode = "chatgptDeviceCode"
)

type CodexAuthUpdateHandler func(context.Context, *appmodel.CodexAuthState)

type CodexAuthManager struct {
	executor    *CodexExecutor
	threadStore *codexThreadStore

	mu       sync.Mutex
	sessions map[string]*codexManagedAuthSession
}

type codexManagedAuthSession struct {
	runID       string
	workspaceID string
	loginID     string
	codexHome   string
	client      *codexAppServerClient
	cancel      context.CancelFunc
	onUpdate    CodexAuthUpdateHandler

	mu    sync.Mutex
	state appmodel.CodexAuthState
}

func NewCodexAuthManager(config CodexRuntimeConfig, artifactRepo *repository.AgentRunArtifactRepository, workspaceAuth *CodexWorkspaceAuthStore) *CodexAuthManager {
	return &CodexAuthManager{
		executor:    NewCodexExecutor("codex", config, nil, artifactRepo, workspaceAuth),
		threadStore: newCodexThreadStore(artifactRepo),
		sessions:    map[string]*codexManagedAuthSession{},
	}
}

func (m *CodexAuthManager) StartDeviceCode(ctx context.Context, run *appmodel.AgentRun, agent *appmodel.Agent, onUpdate CodexAuthUpdateHandler) (*appmodel.CodexAuthState, error) {
	if m == nil || m.executor == nil || m.threadStore == nil {
		return nil, fmt.Errorf("codex auth manager is not configured")
	}
	if run == nil {
		return nil, fmt.Errorf("run is required")
	}

	m.mu.Lock()
	if session := m.sessions[run.ID]; session != nil {
		current := session.snapshot()
		m.mu.Unlock()
		return &current, nil
	}
	m.mu.Unlock()

	state, profile, env, err := m.prepareSession(ctx, run, agent)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.Provider) != appmodel.AgentModelProviderOpenAI || strings.TrimSpace(profile.AuthMode) != codexOpenAIAuthModeDevice {
		return nil, fmt.Errorf("codex device-code auth requires provider openai with CODEX_OPENAI_AUTH_MODE=%q", codexOpenAIAuthModeDevice)
	}

	sessionCtx, cancel := context.WithCancel(context.Background())
	client := newCodexAppServerClient(m.executor.commandPath, "", env)
	requestCtx, cancelRequest := context.WithTimeout(sessionCtx, codexAuthRequestTimeout)
	defer cancelRequest()

	if err := client.Start(sessionCtx); err != nil {
		cancel()
		return nil, err
	}
	if err := client.Initialize(requestCtx); err != nil {
		_ = client.Close()
		cancel()
		return nil, err
	}

	authState, authenticated, err := codexReadManagedAuthState(requestCtx, client, profile)
	if err != nil {
		_ = client.Close()
		cancel()
		return nil, err
	}
	if authenticated {
		_ = client.Close()
		cancel()
		if authState == nil {
			authState = &appmodel.CodexAuthState{
				Provider:  strings.TrimSpace(profile.Provider),
				AuthMode:  strings.TrimSpace(profile.AuthMode),
				State:     appmodel.CodexAuthStateConnected,
				UpdatedAt: time.Now().UTC(),
			}
		}
		return authState, nil
	}

	raw, err := m.startManagedChatGPTLogin(requestCtx, client)
	if err != nil {
		_ = client.Close()
		cancel()
		return nil, err
	}

	var response codexLoginAccountResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		_ = client.Close()
		cancel()
		return nil, fmt.Errorf("decode codex device-code login response: %w", err)
	}
	if responseType := strings.TrimSpace(response.Type); responseType != codexLoginTypeChatGPTDeviceCode && responseType != codexLoginTypeChatGPT {
		_ = client.Close()
		cancel()
		return nil, fmt.Errorf("unexpected codex managed login response type %q", strings.TrimSpace(response.Type))
	}
	loginID := ""
	if response.LoginID != nil {
		loginID = strings.TrimSpace(*response.LoginID)
	}
	if loginID == "" {
		_ = client.Close()
		cancel()
		return nil, fmt.Errorf("codex device-code login response did not include loginId")
	}

	current := appmodel.CodexAuthState{
		Provider:  strings.TrimSpace(profile.Provider),
		AuthMode:  strings.TrimSpace(profile.AuthMode),
		State:     appmodel.CodexAuthStatePending,
		UpdatedAt: time.Now().UTC(),
	}
	if response.LoginID != nil {
		value := strings.TrimSpace(*response.LoginID)
		current.LoginID = &value
	}
	if response.AuthURL != nil && strings.TrimSpace(*response.AuthURL) != "" {
		value := strings.TrimSpace(*response.AuthURL)
		current.AuthURL = &value
	}
	if response.VerificationURL != nil && strings.TrimSpace(*response.VerificationURL) != "" {
		value := strings.TrimSpace(*response.VerificationURL)
		current.VerificationURL = &value
	}
	if response.UserCode != nil && strings.TrimSpace(*response.UserCode) != "" {
		value := strings.TrimSpace(*response.UserCode)
		current.UserCode = &value
	}

	session := &codexManagedAuthSession{
		runID:       run.ID,
		workspaceID: run.WorkspaceID,
		loginID:     loginID,
		codexHome:   state.CodexHome,
		client:      client,
		cancel:      cancel,
		onUpdate:    onUpdate,
		state:       current,
	}

	m.mu.Lock()
	m.sessions[run.ID] = session
	m.mu.Unlock()

	go m.watchSession(sessionCtx, session, profile)

	snapshot := session.snapshot()
	return &snapshot, nil
}

func (m *CodexAuthManager) CancelDeviceCode(ctx context.Context, runID string) (*appmodel.CodexAuthState, error) {
	if m == nil {
		return nil, fmt.Errorf("codex auth manager is not configured")
	}
	m.mu.Lock()
	session := m.sessions[runID]
	m.mu.Unlock()
	if session == nil {
		return nil, fmt.Errorf("no pending codex device-code login for this run")
	}

	requestCtx, cancel := context.WithTimeout(context.Background(), codexAuthRequestTimeout)
	defer cancel()

	raw, err := session.client.Request(requestCtx, "account/login/cancel", map[string]any{
		"loginId": session.loginID,
	})
	if err != nil {
		return nil, err
	}
	var response codexCancelLoginAccountResponse
	if err := json.Unmarshal(raw, &response); err == nil && strings.EqualFold(strings.TrimSpace(response.Status), "notFound") {
		return nil, fmt.Errorf("pending codex device-code login was not found")
	}

	current := session.snapshot()
	return &current, nil
}

func (m *CodexAuthManager) startManagedChatGPTLogin(ctx context.Context, client *codexAppServerClient) (json.RawMessage, error) {
	raw, err := client.Request(ctx, "account/login/start", map[string]any{
		"type": codexLoginTypeChatGPTDeviceCode,
	})
	if err != nil && codexNeedsManagedChatGPTFallback(err) {
		return nil, fmt.Errorf("the configured Codex binary does not support ChatGPT device-code login; point CODEX_PATH at a newer Codex build with chatgptDeviceCode support")
	}
	return raw, err
}

func codexNeedsManagedChatGPTFallback(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "unknown variant") &&
		strings.Contains(message, "chatgpt") &&
		strings.Contains(message, "chatgptauthtokens")
}

func (m *CodexAuthManager) watchSession(ctx context.Context, session *codexManagedAuthSession, profile codexResolvedRuntimeProfile) {
	defer func() {
		session.cancel()
		_ = session.client.Close()
		m.mu.Lock()
		delete(m.sessions, session.runID)
		m.mu.Unlock()
	}()

	loginSucceeded := false
	for {
		msg, err := session.client.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if loginSucceeded {
				connected := session.snapshot()
				if strings.TrimSpace(connected.State) == appmodel.CodexAuthStateConnected {
					return
				}
			}
			errText := strings.TrimSpace(err.Error())
			failed := session.apply(appmodel.CodexAuthState{
				Provider:  strings.TrimSpace(profile.Provider),
				AuthMode:  strings.TrimSpace(profile.AuthMode),
				State:     appmodel.CodexAuthStateFailed,
				Error:     optionalStringPtr(errText),
				UpdatedAt: time.Now().UTC(),
			})
			m.emitUpdate(failed, session.onUpdate)
			return
		}

		switch strings.TrimSpace(msg.Method) {
		case "account/login/completed":
			var payload codexAccountLoginCompletedNotification
			if len(msg.Params) > 0 {
				if err := json.Unmarshal(msg.Params, &payload); err != nil {
					errText := fmt.Sprintf("decode account/login/completed: %v", err)
					failed := session.apply(appmodel.CodexAuthState{
						Provider:  strings.TrimSpace(profile.Provider),
						AuthMode:  strings.TrimSpace(profile.AuthMode),
						State:     appmodel.CodexAuthStateFailed,
						Error:     optionalStringPtr(errText),
						UpdatedAt: time.Now().UTC(),
					})
					m.emitUpdate(failed, session.onUpdate)
					return
				}
			}
			if !payload.Success {
				stateKind := appmodel.CodexAuthStateFailed
				if payload.Error != nil && strings.Contains(strings.ToLower(strings.TrimSpace(*payload.Error)), "cancel") {
					stateKind = appmodel.CodexAuthStateCancelled
				}
				next := appmodel.CodexAuthState{
					Provider:  strings.TrimSpace(profile.Provider),
					AuthMode:  strings.TrimSpace(profile.AuthMode),
					State:     stateKind,
					UpdatedAt: time.Now().UTC(),
				}
				if payload.LoginID != nil && strings.TrimSpace(*payload.LoginID) != "" {
					value := strings.TrimSpace(*payload.LoginID)
					next.LoginID = &value
				}
				if payload.Error != nil && strings.TrimSpace(*payload.Error) != "" {
					value := strings.TrimSpace(*payload.Error)
					next.Error = &value
				}
				cancelled := session.apply(next)
				m.emitUpdate(cancelled, session.onUpdate)
				return
			}
			loginSucceeded = true
		case "account/updated":
			var payload codexAccountUpdatedNotification
			if len(msg.Params) > 0 {
				if err := json.Unmarshal(msg.Params, &payload); err != nil {
					errText := fmt.Sprintf("decode account/updated: %v", err)
					failed := session.apply(appmodel.CodexAuthState{
						Provider:  strings.TrimSpace(profile.Provider),
						AuthMode:  strings.TrimSpace(profile.AuthMode),
						State:     appmodel.CodexAuthStateFailed,
						Error:     optionalStringPtr(errText),
						UpdatedAt: time.Now().UTC(),
					})
					m.emitUpdate(failed, session.onUpdate)
					return
				}
			}
			if payload.AuthMode != nil && strings.TrimSpace(*payload.AuthMode) == "chatgpt" {
				connected := appmodel.CodexAuthState{
					Provider:  strings.TrimSpace(profile.Provider),
					AuthMode:  strings.TrimSpace(profile.AuthMode),
					State:     appmodel.CodexAuthStateConnected,
					UpdatedAt: time.Now().UTC(),
				}
				if payload.PlanType != nil && strings.TrimSpace(*payload.PlanType) != "" {
					value := strings.TrimSpace(*payload.PlanType)
					connected.PlanType = &value
				}
				if current := session.snapshot(); current.LoginID != nil && strings.TrimSpace(*current.LoginID) != "" {
					value := strings.TrimSpace(*current.LoginID)
					connected.LoginID = &value
				}
				if current := session.snapshot(); current.AuthURL != nil && strings.TrimSpace(*current.AuthURL) != "" {
					value := strings.TrimSpace(*current.AuthURL)
					connected.AuthURL = &value
				}
				if current := session.snapshot(); current.VerificationURL != nil && strings.TrimSpace(*current.VerificationURL) != "" {
					value := strings.TrimSpace(*current.VerificationURL)
					connected.VerificationURL = &value
				}
				if current := session.snapshot(); current.UserCode != nil && strings.TrimSpace(*current.UserCode) != "" {
					value := strings.TrimSpace(*current.UserCode)
					connected.UserCode = &value
				}
				final := session.apply(connected)
				if err := m.executor.persistWorkspaceAuth(ctx, session.workspaceID, profile.Provider, profile.AuthMode, session.codexHome); err != nil {
					errText := fmt.Sprintf("persist workspace codex auth: %v", err)
					final = session.apply(appmodel.CodexAuthState{
						Provider:  strings.TrimSpace(profile.Provider),
						AuthMode:  strings.TrimSpace(profile.AuthMode),
						State:     appmodel.CodexAuthStateFailed,
						Error:     optionalStringPtr(errText),
						UpdatedAt: time.Now().UTC(),
					})
				}
				m.emitUpdate(final, session.onUpdate)
				return
			}
		}
	}
}

func (m *CodexAuthManager) prepareSession(ctx context.Context, run *appmodel.AgentRun, agent *appmodel.Agent) (*codexSessionState, codexResolvedRuntimeProfile, []string, error) {
	state, err := m.threadStore.Load(ctx, run)
	if err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, err
	}
	if state == nil {
		state = &codexSessionState{}
	}

	runRoot := strings.TrimSpace(state.HomeRoot)
	if runRoot == "" {
		runRoot = filepath.Join(os.TempDir(), "helpin-codex", sanitizeCodexPathComponent(run.ID), "home")
	}
	codexHome := strings.TrimSpace(state.CodexHome)
	if codexHome == "" {
		codexHome = filepath.Join(runRoot, ".codex")
	}
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, fmt.Errorf("create codex auth session home: %w", err)
	}

	profile, err := m.executor.resolveRuntimeProfile(agent)
	if err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, err
	}

	state.HomeRoot = runRoot
	state.CodexHome = codexHome
	state.Provider = profile.Provider
	state.Model = profile.Model
	state.AuthMode = profile.AuthMode
	state.InvocationMode = strings.TrimSpace(run.InvocationMode)

	configContent, err := m.executor.buildConfigArtifact(nil, profile, codexApprovalPolicyForRun(run))
	if err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, err
	}
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(configContent), 0o600); err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, fmt.Errorf("write codex auth config.toml: %w", err)
	}
	if err := m.executor.restoreWorkspaceAuth(ctx, run.WorkspaceID, profile.Provider, profile.AuthMode, codexHome); err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, err
	}
	if err := m.threadStore.Save(ctx, run, state); err != nil {
		return nil, codexResolvedRuntimeProfile{}, nil, err
	}

	env := m.executor.buildBaseEnv()
	env = upsertEnv(env, "HOME", runRoot)
	env = upsertEnv(env, "CODEX_HOME", codexHome)
	env = m.executor.upsertProviderEnv(env, profile.Provider)

	return state, profile, env, nil
}

func (m *CodexAuthManager) emitUpdate(state appmodel.CodexAuthState, callback CodexAuthUpdateHandler) {
	if callback == nil {
		return
	}
	callback(context.Background(), &state)
}

func (s *codexManagedAuthSession) snapshot() appmodel.CodexAuthState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *codexManagedAuthSession) apply(next appmodel.CodexAuthState) appmodel.CodexAuthState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if next.LoginID == nil && s.state.LoginID != nil {
		value := strings.TrimSpace(*s.state.LoginID)
		if value != "" {
			next.LoginID = &value
		}
	}
	if next.AuthURL == nil && s.state.AuthURL != nil {
		value := strings.TrimSpace(*s.state.AuthURL)
		if value != "" {
			next.AuthURL = &value
		}
	}
	if next.VerificationURL == nil && s.state.VerificationURL != nil {
		value := strings.TrimSpace(*s.state.VerificationURL)
		if value != "" {
			next.VerificationURL = &value
		}
	}
	if next.UserCode == nil && s.state.UserCode != nil {
		value := strings.TrimSpace(*s.state.UserCode)
		if value != "" {
			next.UserCode = &value
		}
	}
	if next.PlanType == nil && s.state.PlanType != nil {
		value := strings.TrimSpace(*s.state.PlanType)
		if value != "" {
			next.PlanType = &value
		}
	}
	if next.Error == nil && s.state.Error != nil && strings.TrimSpace(next.State) == appmodel.CodexAuthStatePending {
		value := strings.TrimSpace(*s.state.Error)
		if value != "" {
			next.Error = &value
		}
	}
	s.state = next
	return s.state
}

func codexReadManagedAuthState(ctx context.Context, client *codexAppServerClient, profile codexResolvedRuntimeProfile) (*appmodel.CodexAuthState, bool, error) {
	raw, err := client.Request(ctx, "account/read", map[string]any{
		"refreshToken": false,
	})
	if err != nil {
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
			value := strings.TrimSpace(*response.Account.PlanType)
			state.PlanType = &value
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

func optionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
