package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const capabilityProbeTimeout = 3 * time.Second

// CapabilityConfig is the non-secret server configuration behind capability
// statuses. Probes perform bounded live checks; nil probes report
// unable_to_verify rather than assuming success.
type CapabilityConfig struct {
	Edition string
	Modules []model.ModuleID
	// AppEmailConfigured reports an application mail sender. AppEmailFingerprint
	// identifies its configuration (never including secrets).
	AppEmailConfigured  bool
	AppEmailFingerprint string
	// AIConnectionsEnabled reports whether workspace AI connections can be stored and used.
	AIConnectionsEnabled bool
	// ServerChatProviders lists providers configured through server environment keys.
	ServerChatProviders []string
	// EmbeddingModel is the knowledge embedding model, or "" when none is configured.
	EmbeddingModel string
	// SupportEmailConfigured reports that support email domains, a sending
	// provider and an inbound webhook secret are all configured.
	SupportEmailConfigured  bool
	ObjectStorageConfigured bool
	// GitHubAppConfigured reports an instance GitHub App; nil means not configured.
	GitHubAppConfigured func(context.Context) bool
	// StorageProbe checks that the object storage bucket answers.
	StorageProbe func(context.Context) error
	// WorkerProbe reports whether a background worker polls for jobs.
	WorkerProbe func(context.Context) (bool, error)
}

// capabilityEvidence is the narrow read contract the capability service needs.
type capabilityEvidence interface {
	WorkspaceOrganizationID(ctx context.Context, workspaceID string) (string, error)
	ActiveWidgetInstallationCount(ctx context.Context, workspaceID string) (int64, error)
	HasEmbeddedChunks(ctx context.Context, workspaceID string) (bool, error)
	HasInboundSupportEmail(ctx context.Context, workspaceID string) (bool, error)
	HasGitHubInstallation(ctx context.Context, organizationID string) (bool, error)
	ConnectedRepositoryCount(ctx context.Context, workspaceID string) (int64, error)
	SharedAIConnectionSummary(ctx context.Context) (repository.AIConnectionSummary, error)
	GetCheck(ctx context.Context, key string) (*model.InstanceCapabilityCheck, error)
	RecordCheck(ctx context.Context, check *model.InstanceCapabilityCheck) error
}

// capabilitySetupEvidence reuses the Setup guide's verified evidence.
type capabilitySetupEvidence interface {
	VerifiedWidgetInstallationCount(ctx context.Context, workspaceID string) (int64, error)
	ActiveEmailRouteCount(ctx context.Context, workspaceID string) (int64, error)
}

// capabilityAIStore resolves the workspace default AI profile and its connection.
type capabilityAIStore interface {
	DefaultProfile(ctx context.Context, workspaceID string) (*model.AIProfile, error)
	Get(ctx context.Context, id string) (*model.AIConnection, error)
}

// CapabilityService reports which capabilities are ready, need setup, or cannot
// be verified, for a workspace or for the whole instance.
type CapabilityService struct {
	cfg      CapabilityConfig
	evidence capabilityEvidence
	setup    capabilitySetupEvidence
	ai       capabilityAIStore
	email    *testEmailSender
}

// NewCapabilityService creates a CapabilityService.
func NewCapabilityService(cfg CapabilityConfig, evidence *repository.CapabilityRepository, setup *repository.SetupRepository, ai *repository.AIConnectionRepository) *CapabilityService {
	return newCapabilityService(cfg, evidence, setup, ai)
}

func newCapabilityService(cfg CapabilityConfig, evidence capabilityEvidence, setup capabilitySetupEvidence, ai capabilityAIStore) *CapabilityService {
	return &CapabilityService{cfg: cfg, evidence: evidence, setup: setup, ai: ai}
}

// Workspace reports capabilities as seen by one workspace.
func (s *CapabilityService) Workspace(ctx context.Context, workspaceID string) (model.CapabilitiesResponse, error) {
	var out []model.Capability
	steps := []func(context.Context, string) (model.Capability, error){
		s.workspaceAIChat,
		s.aiEmbeddings,
		func(ctx context.Context, _ string) (model.Capability, error) { return s.emailOutbound(ctx, true) },
		s.supportWidget,
		s.supportEmailInbound,
		s.workspaceGitHub,
		func(ctx context.Context, _ string) (model.Capability, error) { return s.objectStorage(ctx), nil },
		func(ctx context.Context, _ string) (model.Capability, error) { return s.workers(ctx), nil },
	}
	for _, step := range steps {
		capability, err := step(ctx, workspaceID)
		if err != nil {
			return model.CapabilitiesResponse{}, err
		}
		out = append(out, capability)
	}
	return model.CapabilitiesResponse{Edition: s.cfg.Edition, Capabilities: out}, nil
}

// Instance reports coarse instance-wide capabilities for operator tooling.
// Details and actions reference server configuration, never secret values.
func (s *CapabilityService) Instance(ctx context.Context) (model.CapabilitiesResponse, error) {
	var out []model.Capability
	steps := []func(context.Context) (model.Capability, error){
		s.instanceAIChat,
		func(ctx context.Context) (model.Capability, error) { return s.aiEmbeddings(ctx, "") },
		func(ctx context.Context) (model.Capability, error) { return s.emailOutbound(ctx, false) },
		func(context.Context) (model.Capability, error) { return s.instanceSupportEmailInbound(), nil },
		func(ctx context.Context) (model.Capability, error) { return s.instanceGitHub(ctx), nil },
		func(ctx context.Context) (model.Capability, error) { return s.objectStorage(ctx), nil },
		func(ctx context.Context) (model.Capability, error) { return s.workers(ctx), nil },
	}
	for _, step := range steps {
		capability, err := step(ctx)
		if err != nil {
			return model.CapabilitiesResponse{}, err
		}
		out = append(out, capability)
	}
	return model.CapabilitiesResponse{Edition: s.cfg.Edition, Capabilities: out}, nil
}

func (s *CapabilityService) moduleEnabled(modules ...model.ModuleID) bool {
	for _, enabled := range s.cfg.Modules {
		for _, module := range modules {
			if enabled == module {
				return true
			}
		}
	}
	return false
}

func capability(key, status, detail string, action *model.CapabilityAction) model.Capability {
	return model.Capability{Key: key, Status: status, Detail: detail, Action: action}
}

func settingsAction(label, path string) *model.CapabilityAction {
	return &model.CapabilityAction{Kind: model.CapabilityActionOpenSettings, Label: label, Path: path}
}

func serverAction(label string) *model.CapabilityAction {
	return &model.CapabilityAction{Kind: model.CapabilityActionServerConfig, Label: label}
}

func (s *CapabilityService) workspaceAIChat(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeyAIChat
	if !s.cfg.AIConnectionsEnabled {
		return capability(key, model.CapabilityUnavailable, "AI connections are not enabled on this server.", nil), nil
	}
	connect := settingsAction("Connect an AI provider", "settings/ai")
	profile, err := s.ai.DefaultProfile(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if profile == nil {
		return capability(key, model.CapabilityNeedsSetup, "No workspace AI default is set.", connect), nil
	}
	connection, err := s.ai.Get(ctx, profile.Primary.ConnectionID)
	if err != nil {
		return model.Capability{}, err
	}
	if connection == nil || connection.WorkspaceID != workspaceID {
		return capability(key, model.CapabilityNeedsSetup, "The workspace AI default has no connection.", connect), nil
	}
	switch connection.Status {
	case "connected":
	case "reauthorization_required":
		return capability(key, model.CapabilityNeedsSetup, "The default AI connection needs to be reconnected.", connect), nil
	default:
		return capability(key, model.CapabilityNeedsSetup, "The default AI connection is not connected.", connect), nil
	}
	if connection.Funding == "managed" {
		return capability(key, model.CapabilityReady, "AI is provided by the platform.", nil), nil
	}
	test := &model.CapabilityAction{Kind: model.CapabilityActionTestAIConnection, Label: "Test the connection", Path: "settings/ai", ConnectionID: connection.ID}
	if connection.LastVerifiedAt == nil {
		return capability(key, model.CapabilityUnableToVerify, "The default AI connection has not been tested.", test), nil
	}
	if connection.LastVerificationError != nil {
		result := capability(key, model.CapabilityNeedsSetup, "The last connection test failed: "+*connection.LastVerificationError, test)
		result.CheckedAt = connection.LastVerifiedAt
		return result, nil
	}
	result := capability(key, model.CapabilityReady, "The last connection test succeeded.", nil)
	result.CheckedAt = connection.LastVerifiedAt
	return result, nil
}

func (s *CapabilityService) instanceAIChat(ctx context.Context) (model.Capability, error) {
	const key = model.CapabilityKeyAIChat
	summary, err := s.evidence.SharedAIConnectionSummary(ctx)
	if err != nil {
		return model.Capability{}, err
	}
	switch {
	case summary.Verified > 0:
		return capability(key, model.CapabilityReady, strconv.FormatInt(summary.Verified, 10)+" shared AI connection(s) passed their last test.", nil), nil
	case summary.Connected > 0 || len(s.cfg.ServerChatProviders) > 0:
		return capability(key, model.CapabilityUnableToVerify, "An AI provider is configured but no connection test has succeeded.",
			serverAction("Test the connection in Settings → AI connections")), nil
	case !s.cfg.AIConnectionsEnabled:
		return capability(key, model.CapabilityUnavailable, "AI connections are not enabled on this server.", nil), nil
	default:
		return capability(key, model.CapabilityNeedsSetup, "No AI provider is configured.",
			serverAction("Set OPENROUTER_API_KEY, OPENAI_API_KEY or ANTHROPIC_API_KEY, or connect a provider in Settings → AI connections")), nil
	}
}

func (s *CapabilityService) aiEmbeddings(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeyAIEmbeddings
	if s.cfg.EmbeddingModel == "" {
		return capability(key, model.CapabilityNeedsSetup, "No embedding provider is configured; knowledge search uses keywords only.",
			serverAction("Set OPENAI_API_KEY or OPENROUTER_API_KEY on the server")), nil
	}
	embedded, err := s.evidence.HasEmbeddedChunks(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if embedded {
		return capability(key, model.CapabilityReady, "Knowledge has been indexed with "+s.cfg.EmbeddingModel+".", nil), nil
	}
	return capability(key, model.CapabilityUnableToVerify, "Embeddings are configured with "+s.cfg.EmbeddingModel+"; they are confirmed once knowledge is indexed.", nil), nil
}

func (s *CapabilityService) emailOutbound(ctx context.Context, workspace bool) (model.Capability, error) {
	const key = model.CapabilityKeyEmailOutbound
	if !s.cfg.AppEmailConfigured {
		return capability(key, model.CapabilityNeedsSetup, "Application email is not configured; invitations and password resets cannot be sent.",
			serverAction("Set SMTP_HOST and SMTP_FROM on the server")), nil
	}
	test := serverAction("Send a test email with Settings or the API")
	if workspace {
		test = &model.CapabilityAction{Kind: model.CapabilityActionSendTestEmail, Label: "Send a test email"}
	}
	check, err := s.evidence.GetCheck(ctx, key)
	if err != nil {
		return model.Capability{}, err
	}
	if check == nil || check.ConfigFingerprint != s.cfg.AppEmailFingerprint {
		return capability(key, model.CapabilityUnableToVerify, "Application email is configured but no test email has been sent.", test), nil
	}
	checkedAt := check.CheckedAt
	if !check.OK {
		detail := "The last test email failed."
		if check.Error != nil && *check.Error != "" {
			detail = "The last test email failed: " + *check.Error
		}
		result := capability(key, model.CapabilityNeedsSetup, detail, test)
		result.CheckedAt = &checkedAt
		return result, nil
	}
	result := capability(key, model.CapabilityReady, "The last test email was accepted for delivery.", nil)
	result.CheckedAt = &checkedAt
	return result, nil
}

func (s *CapabilityService) supportWidget(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeySupportWidget
	if !s.moduleEnabled(model.ModuleSupport) {
		return capability(key, model.CapabilityUnavailable, "Support is not enabled on this server.", nil), nil
	}
	install := settingsAction("Install the chat widget", "settings/chat-general")
	verified, err := s.setup.VerifiedWidgetInstallationCount(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if verified > 0 {
		return capability(key, model.CapabilityReady, "The chat widget has loaded for visitors.", nil), nil
	}
	installs, err := s.evidence.ActiveWidgetInstallationCount(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if installs > 0 {
		return capability(key, model.CapabilityNeedsSetup, "The chat widget is set up but has not loaded on a website yet.", install), nil
	}
	return capability(key, model.CapabilityNeedsSetup, "The chat widget is not installed.", install), nil
}

func (s *CapabilityService) supportEmailInbound(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeySupportEmailInbound
	if !s.moduleEnabled(model.ModuleSupport) {
		return capability(key, model.CapabilityUnavailable, "Support is not enabled on this server.", nil), nil
	}
	if !s.cfg.SupportEmailConfigured {
		return s.instanceSupportEmailInbound(), nil
	}
	received, err := s.evidence.HasInboundSupportEmail(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if received {
		return capability(key, model.CapabilityReady, "Support email has been received.", nil), nil
	}
	routes, err := s.setup.ActiveEmailRouteCount(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if routes > 0 {
		return capability(key, model.CapabilityUnableToVerify, "A support address is set up; no email has been received yet.",
			settingsAction("Send an email to your support address", "settings/inboxes-routing")), nil
	}
	return capability(key, model.CapabilityNeedsSetup, "No support email address is set up.",
		settingsAction("Add a support email address", "settings/inboxes-routing")), nil
}

func (s *CapabilityService) instanceSupportEmailInbound() model.Capability {
	const key = model.CapabilityKeySupportEmailInbound
	if !s.moduleEnabled(model.ModuleSupport) {
		return capability(key, model.CapabilityUnavailable, "Support is not enabled on this server.", nil)
	}
	if !s.cfg.SupportEmailConfigured {
		return capability(key, model.CapabilityNeedsSetup, "Inbound support email is not configured on this server.",
			serverAction("Configure support email domains and an inbound mail provider"))
	}
	return capability(key, model.CapabilityUnableToVerify, "Inbound support email is configured; delivery is confirmed when email arrives.", nil)
}

func (s *CapabilityService) gitHubAppConfigured(ctx context.Context) bool {
	return s.cfg.GitHubAppConfigured != nil && s.cfg.GitHubAppConfigured(ctx)
}

func (s *CapabilityService) workspaceGitHub(ctx context.Context, workspaceID string) (model.Capability, error) {
	const key = model.CapabilityKeyGitHub
	if !s.moduleEnabled(model.ModulePM, model.ModuleAgents) {
		return capability(key, model.CapabilityUnavailable, "Projects and Agents are not enabled on this server.", nil), nil
	}
	if !s.gitHubAppConfigured(ctx) {
		return capability(key, model.CapabilityNeedsSetup, "The GitHub App is not configured on this server.",
			settingsAction("Set up GitHub", "settings/git-connections")), nil
	}
	orgID, err := s.evidence.WorkspaceOrganizationID(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	installed, err := s.evidence.HasGitHubInstallation(ctx, orgID)
	if err != nil {
		return model.Capability{}, err
	}
	if !installed {
		return capability(key, model.CapabilityNeedsSetup, "The GitHub App is not installed for this organization.",
			settingsAction("Install the GitHub App", "settings/git-connections")), nil
	}
	repositories, err := s.evidence.ConnectedRepositoryCount(ctx, workspaceID)
	if err != nil {
		return model.Capability{}, err
	}
	if repositories == 0 {
		return capability(key, model.CapabilityNeedsSetup, "GitHub is installed; no repositories are selected for this workspace.",
			settingsAction("Select repositories", "settings/repositories")), nil
	}
	return capability(key, model.CapabilityReady, strconv.FormatInt(repositories, 10)+" repository(ies) connected.", nil), nil
}

func (s *CapabilityService) instanceGitHub(ctx context.Context) model.Capability {
	const key = model.CapabilityKeyGitHub
	if !s.moduleEnabled(model.ModulePM, model.ModuleAgents) {
		return capability(key, model.CapabilityUnavailable, "Projects and Agents are not enabled on this server.", nil)
	}
	if !s.gitHubAppConfigured(ctx) {
		return capability(key, model.CapabilityNeedsSetup, "The GitHub App is not configured.",
			serverAction("Set up a GitHub App in Settings → Git connections"))
	}
	return capability(key, model.CapabilityReady, "The GitHub App is configured.", nil)
}

func (s *CapabilityService) objectStorage(ctx context.Context) model.Capability {
	result := s.probeObjectStorage(ctx)
	result.Required = true
	return result
}

func (s *CapabilityService) probeObjectStorage(ctx context.Context) model.Capability {
	const key = model.CapabilityKeyObjectStorage
	if !s.cfg.ObjectStorageConfigured {
		return capability(key, model.CapabilityNeedsSetup, "Object storage is not configured; attachments are disabled.",
			serverAction("Configure S3-compatible storage (AWS_S3_BUCKET_NAME and credentials)"))
	}
	if s.cfg.StorageProbe == nil {
		return capability(key, model.CapabilityUnableToVerify, "Object storage is configured but was not checked.", nil)
	}
	probeCtx, cancel := context.WithTimeout(ctx, capabilityProbeTimeout)
	defer cancel()
	if err := s.cfg.StorageProbe(probeCtx); err != nil {
		slog.WarnContext(ctx, "object storage capability probe failed", "error", err)
		return capability(key, model.CapabilityNeedsSetup, "Object storage did not respond.",
			serverAction("Check the storage service and its credentials"))
	}
	now := time.Now().UTC()
	result := capability(key, model.CapabilityReady, "Object storage responded.", nil)
	result.CheckedAt = &now
	return result
}

func (s *CapabilityService) workers(ctx context.Context) model.Capability {
	const key = model.CapabilityKeyWorkers
	result := func() model.Capability {
		if s.cfg.WorkerProbe == nil {
			return capability(key, model.CapabilityUnableToVerify, "Background workers were not checked.", nil)
		}
		probeCtx, cancel := context.WithTimeout(ctx, capabilityProbeTimeout)
		defer cancel()
		polling, err := s.cfg.WorkerProbe(probeCtx)
		if err != nil {
			slog.WarnContext(ctx, "worker capability probe failed", "error", err)
			return capability(key, model.CapabilityUnableToVerify, "The workflow service could not be reached.",
				serverAction("Check the Temporal and worker services"))
		}
		if !polling {
			return capability(key, model.CapabilityNeedsSetup, "No background worker is polling for jobs.",
				serverAction("Start the worker service"))
		}
		now := time.Now().UTC()
		ready := capability(key, model.CapabilityReady, "A background worker is polling for jobs.", nil)
		ready.CheckedAt = &now
		return ready
	}()
	result.Required = true
	return result
}

// AppEmailFingerprint identifies an application mail configuration without
// secrets, so a stored test result stops applying when the configuration changes.
// Callers pass non-secret settings only (provider, host, port, username, sender).
func AppEmailFingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
