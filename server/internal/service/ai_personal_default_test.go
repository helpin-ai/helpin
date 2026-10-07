package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIPersonalDefaultIsolationAndResolution(t *testing.T) {
	s, primary, _, _ := setupAIProfileTestDB(t)
	ctx := context.Background()
	shared, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Shared", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := s.connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Scope: "personal", Provider: "openai", APIKey: "personal-key"})
	if err != nil {
		t.Fatal(err)
	}
	personalRoute := primary
	personalRoute.ConnectionID = connection.Connection.ID
	personal, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Mine", Scope: "personal", Primary: personalRoute})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefault(ctx, "workspace", "owner", shared.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "owner", personal.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "teammate", personal.ID); err == nil {
		t.Fatal("another user's model accepted")
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "teammate", shared.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonalDefault(ctx, "other-workspace", "owner", shared.ID); err == nil {
		t.Fatal("cross-workspace default accepted")
	}
	settings, err := s.Settings(ctx, "workspace", "owner")
	if err != nil || derefString(settings.PersonalDefaultProfileID) != personal.ID {
		t.Fatalf("personal settings: %+v %v", settings, err)
	}
	other, err := s.Settings(ctx, "workspace", "teammate")
	if err != nil || derefString(other.PersonalDefaultProfileID) != shared.ID {
		t.Fatalf("settings leaked: %+v %v", other, err)
	}
	for _, tc := range []struct {
		name string
		req  AIProfileSelectionRequest
		want string
	}{
		{"ask", AIProfileSelectionRequest{UsePersonalDefault: true}, personal.ID},
		{"explicit", AIProfileSelectionRequest{UsePersonalDefault: true, ProfileID: shared.ID}, shared.ID},
		{"agent", AIProfileSelectionRequest{UsePersonalDefault: true, AgentProfileID: shared.ID}, shared.ID},
		{"other surface", AIProfileSelectionRequest{}, shared.ID},
		{"automation", AIProfileSelectionRequest{UsePersonalDefault: true, Unattended: true}, shared.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, _, err := s.Resolve(ctx, "workspace", "owner", tc.req)
			if err != nil || selection.ProfileID != tc.want {
				t.Fatalf("wrong route: %+v %v", selection, err)
			}
		})
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "owner", ""); err != nil {
		t.Fatal(err)
	}
	selection, _, err := s.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{UsePersonalDefault: true})
	if err != nil || selection.ProfileID != shared.ID {
		t.Fatalf("clear did not inherit workspace: %v", err)
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "owner", personal.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "workspace", "owner", personal.ID, personal.Revision); err != nil {
		t.Fatal(err)
	}
	settings, err = s.Settings(ctx, "workspace", "owner")
	if err != nil || settings.PersonalDefaultProfileID != nil {
		t.Fatalf("deleted default retained: %+v %v", settings, err)
	}
}

func TestAIPersonalDefaultUsedOnlyForNewDockChats(t *testing.T) {
	profiles, primary, fallback, _ := setupAIProfileTestDB(t)
	ctx := context.Background()
	shared, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Shared", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	chosen, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "My preferred shared model", Scope: "workspace", Primary: fallback})
	if err != nil {
		t.Fatal(err)
	}
	if err := profiles.SetDefault(ctx, "workspace", "owner", shared.ID); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SetPersonalDefault(ctx, "workspace", "owner", chosen.ID); err != nil {
		t.Fatal(err)
	}
	svc := (&AgentService{}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
	for _, tc := range []struct {
		name     string
		chat     *string
		preset   string
		explicit string
		want     string
	}{
		{"new ask chat", strPtr("chat"), model.AgentPresetAskAgent, "", fallback.ConnectionID},
		{"manual agent", nil, model.AgentPresetAskAgent, "", primary.ConnectionID},
		{"other agent", strPtr("chat"), "custom", "", primary.ConnectionID},
		{"explicit choice", strPtr("chat"), model.AgentPresetAskAgent, shared.ID, primary.ConnectionID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := createRunParams{workspaceID: "workspace", actorID: strPtr("owner"), input: []byte(`{}`), agent: &model.Agent{IsSystem: true, PresetKey: tc.preset}, dockChatID: tc.chat, aiProfileID: tc.explicit}
			if _, _, _, err := svc.prepareAIProfileRun(ctx, &params); err != nil {
				t.Fatal(err)
			}
			if params.modelConnectionID != tc.want {
				t.Fatalf("connection = %s, want %s", params.modelConnectionID, tc.want)
			}
		})
	}
}

func TestAIPersonalDefaultRejectsUnavailableModels(t *testing.T) {
	s, primary, _ := setupAIProfileTest(t)
	ctx := context.Background()
	p, err := s.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Shared", Scope: "workspace", Primary: primary})
	if err != nil {
		t.Fatal(err)
	}
	s.SetAdmissionPolicy(connectionPolicyFunc(func(context.Context, string, *model.AIConnection) (*model.AIExecutionPolicySnapshot, error) {
		return nil, errors.New("connection unavailable by policy")
	}))
	if err := s.SetPersonalDefault(ctx, "workspace", "teammate", p.ID); err == nil {
		t.Fatal("policy-blocked model accepted as default")
	}
	s.SetAdmissionPolicy(nil)
	if _, err := s.SetVisibility(ctx, "workspace", "owner", p.ID, p.Revision, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonalDefault(ctx, "workspace", "owner", p.ID); err == nil {
		t.Fatal("hidden model accepted as default")
	}
}
