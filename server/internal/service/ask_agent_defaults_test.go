package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAskAgentDefaultsUsesBuiltInAgentProfile(t *testing.T) {
	db := newAgentServiceTestDB(t)
	s := &AgentService{agentRepo: repository.NewAgentRepository(db)}
	profile := "shared-profile"
	agent := &model.Agent{ID: "ask-agent", WorkspaceID: "workspace", Name: "Ask Agent", PresetKey: model.AgentPresetAskAgent, IsSystem: true, AIProfileID: &profile, AllowedTools: json.RawMessage(`[]`), AllowedCommands: json.RawMessage(`[]`), AllowedTargets: json.RawMessage(`[]`)}
	if err := s.agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	defaults, err := s.AskAgentDefaults(context.Background(), "workspace")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.AIProfileID == nil || *defaults.AIProfileID != profile {
		t.Fatalf("wrong default: %+v", defaults)
	}
	other, err := s.AskAgentDefaults(context.Background(), "other-workspace")
	if err != nil {
		t.Fatal(err)
	}
	if other.AIProfileID != nil {
		t.Fatal("leaked another workspace's default")
	}
}
