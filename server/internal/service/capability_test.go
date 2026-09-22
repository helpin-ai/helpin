package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeCapabilityEvidence struct {
	orgID        string
	installs     int64
	embedded     bool
	inbound      bool
	github       bool
	repositories int64
	summary      repository.AIConnectionSummary
	check        *model.InstanceCapabilityCheck
	recorded     []*model.InstanceCapabilityCheck
}

func (f *fakeCapabilityEvidence) WorkspaceOrganizationID(context.Context, string) (string, error) {
	return f.orgID, nil
}
func (f *fakeCapabilityEvidence) ActiveWidgetInstallationCount(context.Context, string) (int64, error) {
	return f.installs, nil
}
func (f *fakeCapabilityEvidence) HasEmbeddedChunks(context.Context, string) (bool, error) {
	return f.embedded, nil
}
func (f *fakeCapabilityEvidence) HasInboundSupportEmail(context.Context, string) (bool, error) {
	return f.inbound, nil
}
func (f *fakeCapabilityEvidence) HasGitHubInstallation(context.Context, string) (bool, error) {
	return f.github, nil
}
func (f *fakeCapabilityEvidence) ConnectedRepositoryCount(context.Context, string) (int64, error) {
	return f.repositories, nil
}
func (f *fakeCapabilityEvidence) SharedAIConnectionSummary(context.Context) (repository.AIConnectionSummary, error) {
	return f.summary, nil
}
func (f *fakeCapabilityEvidence) GetCheck(context.Context, string) (*model.InstanceCapabilityCheck, error) {
	return f.check, nil
}
func (f *fakeCapabilityEvidence) RecordCheck(_ context.Context, check *model.InstanceCapabilityCheck) error {
	f.recorded = append(f.recorded, check)
	f.check = check
	return nil
}

type fakeCapabilitySetup struct{ verifiedWidgets, routes int64 }

func (f fakeCapabilitySetup) VerifiedWidgetInstallationCount(context.Context, string) (int64, error) {
	return f.verifiedWidgets, nil
}
func (f fakeCapabilitySetup) ActiveEmailRouteCount(context.Context, string) (int64, error) {
	return f.routes, nil
}

type fakeCapabilityAI struct {
	profile    *model.AIProfile
	connection *model.AIConnection
}

func (f fakeCapabilityAI) DefaultProfile(context.Context, string) (*model.AIProfile, error) {
	return f.profile, nil
}
func (f fakeCapabilityAI) Get(context.Context, string) (*model.AIConnection, error) {
	return f.connection, nil
}

func allModules() []model.ModuleID {
	return []model.ModuleID{model.ModuleSupport, model.ModuleDocs, model.ModuleAgents, model.ModulePM, model.ModuleCRM, model.ModuleAutomation}
}

func capabilityByKey(t *testing.T, response model.CapabilitiesResponse, key string) model.Capability {
	t.Helper()
	for _, c := range response.Capabilities {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("capability %q missing", key)
	return model.Capability{}
}

func TestWorkspaceCapabilitiesAIChat(t *testing.T) {
	verifiedAt := time.Now().UTC()
	failure := "The provider rejected the API key."
	profile := &model.AIProfile{Primary: model.AIProfileRoute{ConnectionID: "c1"}}
	connection := func(mutate func(*model.AIConnection)) *model.AIConnection {
		c := &model.AIConnection{ID: "c1", WorkspaceID: "ws", Status: "connected", Funding: "customer", Scope: "workspace"}
		if mutate != nil {
			mutate(c)
		}
		return c
	}
	tests := []struct {
		name    string
		enabled bool
		ai      fakeCapabilityAI
		want    string
		action  string
	}{
		{"connections disabled", false, fakeCapabilityAI{}, model.CapabilityUnavailable, ""},
		{"no default profile", true, fakeCapabilityAI{}, model.CapabilityNeedsSetup, model.CapabilityActionOpenSettings},
		{"missing connection", true, fakeCapabilityAI{profile: profile}, model.CapabilityNeedsSetup, model.CapabilityActionOpenSettings},
		{"other workspace connection", true, fakeCapabilityAI{profile: profile, connection: connection(func(c *model.AIConnection) { c.WorkspaceID = "other" })}, model.CapabilityNeedsSetup, model.CapabilityActionOpenSettings},
		{"needs reauthorization", true, fakeCapabilityAI{profile: profile, connection: connection(func(c *model.AIConnection) { c.Status = "reauthorization_required" })}, model.CapabilityNeedsSetup, model.CapabilityActionOpenSettings},
		{"never tested", true, fakeCapabilityAI{profile: profile, connection: connection(nil)}, model.CapabilityUnableToVerify, model.CapabilityActionTestAIConnection},
		{"last test failed", true, fakeCapabilityAI{profile: profile, connection: connection(func(c *model.AIConnection) {
			c.LastVerifiedAt, c.LastVerificationError = &verifiedAt, &failure
		})}, model.CapabilityNeedsSetup, model.CapabilityActionTestAIConnection},
		{"last test succeeded", true, fakeCapabilityAI{profile: profile, connection: connection(func(c *model.AIConnection) { c.LastVerifiedAt = &verifiedAt })}, model.CapabilityReady, ""},
		{"managed connection", true, fakeCapabilityAI{profile: profile, connection: connection(func(c *model.AIConnection) { c.Funding = "managed" })}, model.CapabilityReady, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newCapabilityService(CapabilityConfig{Edition: "community", Modules: allModules(), AIConnectionsEnabled: tt.enabled},
				&fakeCapabilityEvidence{}, fakeCapabilitySetup{}, tt.ai)
			response, err := svc.Workspace(context.Background(), "ws")
			if err != nil {
				t.Fatal(err)
			}
			got := capabilityByKey(t, response, model.CapabilityKeyAIChat)
			if got.Status != tt.want {
				t.Fatalf("status=%s detail=%q, want %s", got.Status, got.Detail, tt.want)
			}
			if (got.Action == nil) != (tt.action == "") || (got.Action != nil && got.Action.Kind != tt.action) {
				t.Fatalf("action=%+v, want %q", got.Action, tt.action)
			}
			if tt.name == "last test failed" && got.Detail != "The last connection test failed: "+failure {
				t.Fatal(got.Detail)
			}
		})
	}
}

func TestWorkspaceCapabilitiesUseEvidenceNotConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		cfg      CapabilityConfig
		evidence *fakeCapabilityEvidence
		setup    fakeCapabilitySetup
		key      string
		want     string
	}{
		{"embeddings not configured", CapabilityConfig{}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyAIEmbeddings, model.CapabilityNeedsSetup},
		{"embeddings configured, nothing indexed", CapabilityConfig{EmbeddingModel: "m"}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyAIEmbeddings, model.CapabilityUnableToVerify},
		{"embeddings indexed", CapabilityConfig{EmbeddingModel: "m"}, &fakeCapabilityEvidence{embedded: true}, fakeCapabilitySetup{}, model.CapabilityKeyAIEmbeddings, model.CapabilityReady},
		{"widget not installed", CapabilityConfig{}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeySupportWidget, model.CapabilityNeedsSetup},
		{"widget installed, never loaded", CapabilityConfig{}, &fakeCapabilityEvidence{installs: 1}, fakeCapabilitySetup{}, model.CapabilityKeySupportWidget, model.CapabilityNeedsSetup},
		{"widget seen", CapabilityConfig{}, &fakeCapabilityEvidence{installs: 1}, fakeCapabilitySetup{verifiedWidgets: 1}, model.CapabilityKeySupportWidget, model.CapabilityReady},
		{"widget without support module", CapabilityConfig{Modules: []model.ModuleID{model.ModulePM}}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{verifiedWidgets: 1}, model.CapabilityKeySupportWidget, model.CapabilityUnavailable},
		{"inbound email not configured", CapabilityConfig{}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{routes: 1}, model.CapabilityKeySupportEmailInbound, model.CapabilityNeedsSetup},
		{"inbound email address without mail", CapabilityConfig{SupportEmailConfigured: true}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{routes: 1}, model.CapabilityKeySupportEmailInbound, model.CapabilityUnableToVerify},
		{"inbound email received", CapabilityConfig{SupportEmailConfigured: true}, &fakeCapabilityEvidence{inbound: true}, fakeCapabilitySetup{}, model.CapabilityKeySupportEmailInbound, model.CapabilityReady},
		{"github app missing", CapabilityConfig{}, &fakeCapabilityEvidence{github: true, repositories: 2}, fakeCapabilitySetup{}, model.CapabilityKeyGitHub, model.CapabilityNeedsSetup},
		{"github not installed", CapabilityConfig{GitHubAppConfigured: func(context.Context) bool { return true }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyGitHub, model.CapabilityNeedsSetup},
		{"github without repositories", CapabilityConfig{GitHubAppConfigured: func(context.Context) bool { return true }}, &fakeCapabilityEvidence{github: true}, fakeCapabilitySetup{}, model.CapabilityKeyGitHub, model.CapabilityNeedsSetup},
		{"github ready", CapabilityConfig{GitHubAppConfigured: func(context.Context) bool { return true }}, &fakeCapabilityEvidence{github: true, repositories: 2}, fakeCapabilitySetup{}, model.CapabilityKeyGitHub, model.CapabilityReady},
		{"storage not configured", CapabilityConfig{}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyObjectStorage, model.CapabilityNeedsSetup},
		{"storage configured without probe", CapabilityConfig{ObjectStorageConfigured: true}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyObjectStorage, model.CapabilityUnableToVerify},
		{"storage probe fails", CapabilityConfig{ObjectStorageConfigured: true, StorageProbe: func(context.Context) error { return errors.New("dial tcp 10.0.0.1") }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyObjectStorage, model.CapabilityNeedsSetup},
		{"storage probe succeeds", CapabilityConfig{ObjectStorageConfigured: true, StorageProbe: func(context.Context) error { return nil }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyObjectStorage, model.CapabilityReady},
		{"worker unreachable", CapabilityConfig{WorkerProbe: func(context.Context) (bool, error) { return false, errors.New("down") }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyWorkers, model.CapabilityUnableToVerify},
		{"worker not polling", CapabilityConfig{WorkerProbe: func(context.Context) (bool, error) { return false, nil }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyWorkers, model.CapabilityNeedsSetup},
		{"worker polling", CapabilityConfig{WorkerProbe: func(context.Context) (bool, error) { return true, nil }}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, model.CapabilityKeyWorkers, model.CapabilityReady},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.Modules == nil {
				tt.cfg.Modules = allModules()
			}
			svc := newCapabilityService(tt.cfg, tt.evidence, tt.setup, fakeCapabilityAI{})
			response, err := svc.Workspace(context.Background(), "ws")
			if err != nil {
				t.Fatal(err)
			}
			got := capabilityByKey(t, response, tt.key)
			if got.Status != tt.want {
				t.Fatalf("status=%s detail=%q, want %s", got.Status, got.Detail, tt.want)
			}
			if got.Status != model.CapabilityReady && got.Status != model.CapabilityUnavailable && got.Action == nil && tt.key != model.CapabilityKeyAIEmbeddings && tt.key != model.CapabilityKeyObjectStorage {
				t.Fatalf("%s without a next action", got.Status)
			}
		})
	}
}

func TestEmailOutboundCapabilityFollowsLastTestForCurrentConfiguration(t *testing.T) {
	failure := testEmailSendFailed
	tests := []struct {
		name  string
		cfg   CapabilityConfig
		check *model.InstanceCapabilityCheck
		want  string
	}{
		{"not configured", CapabilityConfig{}, nil, model.CapabilityNeedsSetup},
		{"never tested", CapabilityConfig{AppEmailConfigured: true, AppEmailFingerprint: "a"}, nil, model.CapabilityUnableToVerify},
		{"tested another configuration", CapabilityConfig{AppEmailConfigured: true, AppEmailFingerprint: "b"}, &model.InstanceCapabilityCheck{OK: true, ConfigFingerprint: "a"}, model.CapabilityUnableToVerify},
		{"last test failed", CapabilityConfig{AppEmailConfigured: true, AppEmailFingerprint: "a"}, &model.InstanceCapabilityCheck{OK: false, Error: &failure, ConfigFingerprint: "a"}, model.CapabilityNeedsSetup},
		{"last test succeeded", CapabilityConfig{AppEmailConfigured: true, AppEmailFingerprint: "a"}, &model.InstanceCapabilityCheck{OK: true, ConfigFingerprint: "a", CheckedAt: time.Now()}, model.CapabilityReady},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newCapabilityService(tt.cfg, &fakeCapabilityEvidence{check: tt.check}, fakeCapabilitySetup{}, fakeCapabilityAI{})
			for _, load := range []func() (model.CapabilitiesResponse, error){
				func() (model.CapabilitiesResponse, error) { return svc.Workspace(context.Background(), "ws") },
				func() (model.CapabilitiesResponse, error) { return svc.Instance(context.Background()) },
			} {
				response, err := load()
				if err != nil {
					t.Fatal(err)
				}
				if got := capabilityByKey(t, response, model.CapabilityKeyEmailOutbound); got.Status != tt.want {
					t.Fatalf("status=%s, want %s", got.Status, tt.want)
				}
			}
		})
	}
}

func TestInstanceCapabilitiesAreCoarseAndMarkRequired(t *testing.T) {
	svc := newCapabilityService(CapabilityConfig{Edition: "community", Modules: allModules(), AIConnectionsEnabled: true, ServerChatProviders: []string{"openrouter"}},
		&fakeCapabilityEvidence{}, fakeCapabilitySetup{}, fakeCapabilityAI{})
	response, err := svc.Instance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if response.Edition != "community" {
		t.Fatal(response.Edition)
	}
	if got := capabilityByKey(t, response, model.CapabilityKeyAIChat); got.Status != model.CapabilityUnableToVerify {
		t.Fatalf("configured but untested AI reported %s", got.Status)
	}
	for _, c := range response.Capabilities {
		required := c.Key == model.CapabilityKeyObjectStorage || c.Key == model.CapabilityKeyWorkers
		if c.Required != required {
			t.Fatalf("%s required=%v", c.Key, c.Required)
		}
		if c.Key == model.CapabilityKeySupportWidget {
			t.Fatal("instance response must not include workspace-only capabilities")
		}
	}
	verified := newCapabilityService(CapabilityConfig{AIConnectionsEnabled: true}, &fakeCapabilityEvidence{summary: repository.AIConnectionSummary{Connected: 2, Verified: 1}}, fakeCapabilitySetup{}, fakeCapabilityAI{})
	response, err = verified.Instance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := capabilityByKey(t, response, model.CapabilityKeyAIChat); got.Status != model.CapabilityReady {
		t.Fatal(got.Status)
	}
}

type fakeTestMailer struct {
	to  []string
	err error
}

func (f *fakeTestMailer) SendEmail(to, _, _, _ string) error {
	f.to = append(f.to, to)
	return f.err
}

type fakeCapabilityUsers map[string]*model.User

func (f fakeCapabilityUsers) GetByID(_ context.Context, id string) (*model.User, error) {
	return f[id], nil
}

func TestSendTestEmailGoesToRequesterAndRecordsRedactedResult(t *testing.T) {
	evidence := &fakeCapabilityEvidence{}
	mailer := &fakeTestMailer{err: errors.New("535 auth failed for user@smtp.internal.example")}
	svc := newCapabilityService(CapabilityConfig{AppEmailConfigured: true, AppEmailFingerprint: "fp"}, evidence, fakeCapabilitySetup{}, fakeCapabilityAI{}).
		SetTestEmail(mailer, fakeCapabilityUsers{"u1": {ID: "u1", Email: "owner@example.com"}})
	result, err := svc.SendTestEmail(context.Background(), "ws", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error != testEmailSendFailed || len(mailer.to) != 1 || mailer.to[0] != "owner@example.com" {
		t.Fatalf("%+v %v", result, mailer.to)
	}
	if len(evidence.recorded) != 1 || evidence.recorded[0].OK || *evidence.recorded[0].Error != testEmailSendFailed || evidence.recorded[0].ConfigFingerprint != "fp" {
		t.Fatalf("%+v", evidence.recorded)
	}
	if _, err := svc.SendTestEmail(context.Background(), "ws", "u1"); !errors.Is(err, ErrTestEmailRateLimited) {
		t.Fatalf("second immediate send err=%v", err)
	}
}

func TestSendTestEmailWithoutMailOrRecipient(t *testing.T) {
	svc := newCapabilityService(CapabilityConfig{}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, fakeCapabilityAI{}).
		SetTestEmail(nil, fakeCapabilityUsers{})
	result, err := svc.SendTestEmail(context.Background(), "ws", "u1")
	if err != nil || result.OK || result.Error == "" {
		t.Fatalf("%+v %v", result, err)
	}
	configured := newCapabilityService(CapabilityConfig{AppEmailConfigured: true}, &fakeCapabilityEvidence{}, fakeCapabilitySetup{}, fakeCapabilityAI{}).
		SetTestEmail(&fakeTestMailer{}, fakeCapabilityUsers{"u1": {ID: "u1"}})
	if _, err := configured.SendTestEmail(context.Background(), "ws", "u1"); !errors.Is(err, ErrTestEmailNoRecipient) {
		t.Fatal(err)
	}
}

func TestTestEmailLimiterWindow(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	limiter := newTestEmailLimiter(func() time.Time { return now })
	for i := 0; i < testEmailPerWindow; i++ {
		if !limiter.allow("u") {
			t.Fatalf("send %d rejected", i)
		}
		if limiter.allow("u") {
			t.Fatal("allowed a send inside the minimum interval")
		}
		if !limiter.allow("other") && i == 0 {
			t.Fatal("users share a limit")
		}
		now = now.Add(testEmailMinInterval)
	}
	if limiter.allow("u") {
		t.Fatal("allowed more than the hourly limit")
	}
	now = now.Add(testEmailWindow)
	if !limiter.allow("u") {
		t.Fatal("limit did not reset")
	}
}
