package service

import (
	"context"
	"errors"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func setupAIProfileTest(t *testing.T) (*AIProfileService, model.AIProfileRoute, model.AIProfileRoute) {
	t.Helper()
	s, primary, fallback, _ := setupAIProfileTestDB(t)
	return s, primary, fallback
}

func setupAIProfileTestDB(t *testing.T) (*AIProfileService, model.AIProfileRoute, model.AIProfileRoute, *gorm.DB) {
	t.Helper()
	connections, db := setupAIConnectionTest(t)
	connections.SetAuthorizationService(authorization.NewAuthzService(db, connectionMembers{}, nil))
	if err := db.AutoMigrate(&model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspaces (id TEXT, status TEXT); INSERT INTO workspaces VALUES ('workspace','active')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE agents (id TEXT PRIMARY KEY, workspace_id TEXT, ai_profile_id TEXT, deleted_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	routes := make([]model.AIProfileRoute, 2)
	for i, name := range []string{"Primary", "Fallback"} {
		c, err := connections.Create(context.Background(), "workspace", "owner", model.CreateAIConnectionRequest{Name: name, Scope: "workspace", Provider: "openai", APIKey: name + "-key"})
		if err != nil {
			t.Fatal(err)
		}
		routes[i] = model.AIProfileRoute{ConnectionID: c.Connection.ID, Model: sdk.RunModel{Provider: "openai", Model: "custom-unpriced-model", Controls: &sdk.ModelControls{ReasoningEffort: strPtr("low")}}}
	}
	return NewAIProfileService(repository.NewAIProfileRepository(db), connections), routes[0], routes[1], db
}

func TestAIProfileResolutionFreezesAcceptedRoute(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	req := model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback}
	p, err := s.Save(ctx, "workspace", "owner", "", req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefault(ctx, "workspace", "owner", p.ID); err != nil {
		t.Fatal(err)
	}
	selection, credential, err := s.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{Unattended: true})
	if err != nil {
		t.Fatal(err)
	}
	if selection.ProfileRevision != 1 || selection.Source != "workspace" || credential.APIKey != "Primary-key" || selection.Route.Model.Controls == nil {
		t.Fatalf("wrong selection: %+v", selection)
	}
	req.Revision, req.Primary, req.Fallback = p.Revision, fallback, nil
	p, err = s.Save(ctx, "workspace", "owner", p.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(ctx, "workspace", "owner", p.ID, req); !errors.Is(err, repository.ErrAIProfileChanged) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	updated, err := s.repo.Get(ctx, "workspace", p.ID)
	if err != nil || updated.Fallback != nil || updated.Primary.ConnectionID != fallback.ConnectionID {
		t.Fatalf("route update or fallback removal not persisted: %+v %v", updated, err)
	}
	credential, err = s.Restore(ctx, "workspace", "", selection)
	if err != nil || credential.APIKey != "Primary-key" {
		t.Fatalf("accepted run changed with profile: %v", err)
	}
	if err := s.Delete(ctx, "workspace", "owner", p.ID, p.Revision); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, "workspace", "", AIProfileSelectionRequest{Unattended: true}); err == nil {
		t.Fatal("deleted default still resolved")
	}
	if _, err := s.Restore(ctx, "workspace", "", selection); err != nil {
		t.Fatalf("profile deletion changed accepted run: %v", err)
	}
}

func TestAIProfileFallbackOnlyForAuthorizedUnavailableConnection(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	disconnect := func(c *model.AIConnection) error { c.Status = "disconnected"; return nil }
	if err := s.connections.repo.WithLocked(ctx, primary.ConnectionID, disconnect); err != nil {
		t.Fatal(err)
	}
	selection, cred, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: p.ID})
	if err != nil || cred.APIKey != "Fallback-key" || selection.FallbackReason != "primary_connection_unavailable" {
		t.Fatalf("fallback failed: %v", err)
	}
	if err := s.connections.repo.WithLocked(ctx, fallback.ConnectionID, disconnect); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(ctx, "workspace", "teammate", selection); !errors.Is(err, ErrAIConnectionUnavailable) {
		t.Fatalf("resume switched instead of failing: %v", err)
	}
	if _, _, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: p.ID}); !errors.Is(err, ErrAIConnectionUnavailable) {
		t.Fatalf("both unavailable: %v", err)
	}
	// A corrupt secret is an infrastructure error, not permission to change routes.
	if err := s.connections.repo.WithLocked(ctx, primary.ConnectionID, func(c *model.AIConnection) error {
		c.Status = "connected"
		c.EncryptedSecret = []byte("broken")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: p.ID}); err == nil || errors.Is(err, ErrAIConnectionUnavailable) {
		t.Fatalf("corrupt primary fell back: %v", err)
	}
}

func TestAIProfilePersonalAndSharedAuthorization(t *testing.T) {
	s, primary, fallback := setupAIProfileTest(t)
	ctx := context.Background()
	c, err := s.connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "personal-key"})
	if err != nil {
		t.Fatal(err)
	}
	primary.ConnectionID = c.Connection.ID
	req := model.SaveAIProfileRequest{Name: "Personal", Scope: "personal", Primary: primary, Fallback: &fallback}
	p, err := s.Save(ctx, "workspace", "owner", "", req)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: p.ID}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []AIProfileSelectionRequest{{ProfileID: p.ID, Unattended: true}, {AgentProfileID: p.ID}} {
		if _, _, err := s.Resolve(ctx, "workspace", "owner", r); err == nil {
			t.Fatal("personal default/unattended launch accepted")
		}
	}
	if _, _, err := s.Resolve(ctx, "workspace", "teammate", AIProfileSelectionRequest{ProfileID: p.ID}); err == nil {
		t.Fatal("other owner used personal profile")
	}
	if err := s.SetDefault(ctx, "workspace", "owner", p.ID); err == nil {
		t.Fatal("personal workspace default accepted")
	}
	if err := s.Delete(ctx, "workspace", "teammate", p.ID, p.Revision); err == nil {
		t.Fatal("other member deleted personal profile")
	}
	req.Scope = "workspace"
	if _, err := s.Save(ctx, "workspace", "owner", "", req); err == nil {
		t.Fatal("shared profile referenced personal secret")
	}
	req.Primary = fallback
	if _, err := s.Save(ctx, "workspace", "teammate", "", req); err == nil {
		t.Fatal("member created shared profile")
	}
}

func TestAIProfileDeletionRequiresAgentReassignment(t *testing.T) {
	s, primary, _, db := setupAIProfileTestDB(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Team", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO agents (id,workspace_id,ai_profile_id) VALUES ('agent','workspace',?)", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "workspace", "owner", p.ID, p.Revision); !errors.Is(err, repository.ErrAIProfileInUse) {
		t.Fatalf("referenced profile deleted: %v", err)
	}
	stored, err := s.repo.Get(ctx, "workspace", p.ID)
	if err != nil || stored == nil || stored.Revision != p.Revision {
		t.Fatal("rejected deletion changed profile")
	}
	if err := db.Exec("UPDATE agents SET ai_profile_id=NULL WHERE id='agent'").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "workspace", "owner", p.ID, p.Revision); err != nil {
		t.Fatal(err)
	}
}
