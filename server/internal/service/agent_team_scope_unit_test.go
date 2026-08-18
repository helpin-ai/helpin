package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateAgentTeamScope(t *testing.T) {
	teamA, teamB := "team-a", "team-b"
	tests := []struct {
		name    string
		agent   model.Agent
		target  *string
		wantErr bool
	}{
		{"workspace scoped", model.Agent{}, &teamB, false},
		{"matching team", model.Agent{TeamID: &teamA}, &teamA, false},
		{"mismatched team", model.Agent{TeamID: &teamA}, &teamB, true},
		{"unresolved nil target", model.Agent{TeamID: &teamA}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentTeamScope(&tt.agent, "task", tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "cannot run") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
