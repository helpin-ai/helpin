package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateAgentTeamScope(t *testing.T) {
	t.Parallel()

	teamA := "team-a"
	teamB := "team-b"

	tests := []struct {
		name         string
		agent        model.Agent
		targetType   string
		targetTeamID *string
		wantErr      bool
	}{
		{
			name:       "workspace scoped agent allowed",
			agent:      model.Agent{},
			targetType: "epic",
			wantErr:    false,
		},
		{
			name:         "matching team allowed",
			agent:        model.Agent{TeamID: &teamA},
			targetType:   "story",
			targetTeamID: &teamA,
			wantErr:      false,
		},
		{
			name:         "mismatched team rejected",
			agent:        model.Agent{TeamID: &teamA},
			targetType:   "story",
			targetTeamID: &teamB,
			wantErr:      true,
		},
		{
			name:       "workspace scoped target rejected for team agent",
			agent:      model.Agent{TeamID: &teamA},
			targetType: "epic",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAgentTeamScope(&tt.agent, tt.targetType, tt.targetTeamID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateAgentTeamScope() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
