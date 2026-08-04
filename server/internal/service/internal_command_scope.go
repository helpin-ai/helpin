package service

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// commandAgentTeamFilter returns an additional team filter for agent-executed
// commands. A nil result means no agent restriction; it never represents an
// unresolved agent scope.
func commandAgentTeamFilter(meta model.InternalCommandContext) ([]string, error) {
	if strings.TrimSpace(meta.AgentID) == "" {
		return nil, nil
	}
	if !meta.AgentScopeResolved {
		return nil, fmt.Errorf("agent team scope is unresolved")
	}
	teamIDs := normalizeCommandTeamIDs(meta.AgentTeamIDs)
	if len(teamIDs) == 0 {
		return nil, nil
	}
	return teamIDs, nil
}

// requireCommandAgentTeam applies agent scope as an additional restriction.
// Existing human authorization remains responsible for its own independent
// permission and membership checks.
func requireCommandAgentTeam(meta model.InternalCommandContext, teamID *string) error {
	allowedTeamIDs, err := commandAgentTeamFilter(meta)
	if err != nil || len(allowedTeamIDs) == 0 {
		return err
	}
	actualTeamID := strings.TrimSpace(derefString(teamID))
	for _, allowedTeamID := range allowedTeamIDs {
		if allowedTeamID == actualTeamID {
			return nil
		}
	}
	return &model.ErrForbidden{Message: "agent does not have access to this team's resources"}
}

// requireCommandAgentTeams applies the agent's configured team scope to an
// entity with multiple linked teams. Any overlap is sufficient, matching the
// product's objective management rule for human team managers.
func requireCommandAgentTeams(meta model.InternalCommandContext, teamIDs []string) error {
	allowedTeamIDs, err := commandAgentTeamFilter(meta)
	if err != nil || len(allowedTeamIDs) == 0 {
		return err
	}
	actual := make(map[string]struct{}, len(teamIDs))
	for _, teamID := range normalizeCommandTeamIDs(teamIDs) {
		actual[teamID] = struct{}{}
	}
	for _, allowedTeamID := range allowedTeamIDs {
		if _, ok := actual[allowedTeamID]; ok {
			return nil
		}
	}
	return &model.ErrForbidden{Message: "agent does not have access to this objective's teams"}
}

func normalizeCommandTeamIDs(teamIDs []string) []string {
	seen := make(map[string]struct{}, len(teamIDs))
	normalized := make([]string, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		teamID = strings.TrimSpace(teamID)
		if teamID == "" {
			continue
		}
		if _, exists := seen[teamID]; exists {
			continue
		}
		seen[teamID] = struct{}{}
		normalized = append(normalized, teamID)
	}
	return normalized
}
