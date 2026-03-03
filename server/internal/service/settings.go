package service

import (
	"context"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// SettingsService handles workspace configuration business logic.
type SettingsService struct {
	settingsRepo *repository.SettingsRepository
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(settingsRepo *repository.SettingsRepository) *SettingsService {
	return &SettingsService{settingsRepo: settingsRepo}
}

// GetAll returns the full workspace configuration.
func (s *SettingsService) GetAll(ctx context.Context, workspaceID string) (*model.FullWorkspaceConfig, error) {
	return s.settingsRepo.GetAll(ctx, workspaceID)
}

// Initialize creates default workspace settings.
func (s *SettingsService) Initialize(ctx context.Context, workspaceID string) (*model.WorkspaceSettings, error) {
	return s.settingsRepo.Initialize(ctx, workspaceID)
}

// CreateTeam creates a new team.
func (s *SettingsService) CreateTeam(ctx context.Context, req model.CreateTeamRequest) (*model.WorkspaceTeam, error) {
	return s.settingsRepo.CreateTeam(ctx, req)
}

// UpdateTeam modifies a team.
func (s *SettingsService) UpdateTeam(ctx context.Context, id string, req model.UpdateTeamRequest) (*model.WorkspaceTeam, error) {
	return s.settingsRepo.UpdateTeam(ctx, id, req)
}

// DeleteTeam removes a team.
func (s *SettingsService) DeleteTeam(ctx context.Context, id string) error {
	return s.settingsRepo.DeleteTeam(ctx, id)
}

// CreatePerson creates a new person.
func (s *SettingsService) CreatePerson(ctx context.Context, req model.CreatePersonRequest) (*model.WorkspacePerson, error) {
	return s.settingsRepo.CreatePerson(ctx, req)
}

// UpdatePerson modifies a person.
func (s *SettingsService) UpdatePerson(ctx context.Context, id string, req model.UpdatePersonRequest) (*model.WorkspacePerson, error) {
	return s.settingsRepo.UpdatePerson(ctx, id, req)
}

// DeletePerson removes a person.
func (s *SettingsService) DeletePerson(ctx context.Context, id string) error {
	return s.settingsRepo.DeletePerson(ctx, id)
}

// UpdateBonusTiers replaces bonus tiers for a workspace.
func (s *SettingsService) UpdateBonusTiers(ctx context.Context, workspaceID string, tiers []model.BonusTierItem) ([]model.BonusTier, error) {
	return s.settingsRepo.UpdateBonusTiers(ctx, workspaceID, tiers)
}

// UpdateJobRoleCriteria replaces criteria for a job role.
func (s *SettingsService) UpdateJobRoleCriteria(ctx context.Context, workspaceID, jobRole string, criteria []model.JobRoleCriteriaItem) ([]model.JobRoleCriteria, error) {
	return s.settingsRepo.UpdateJobRoleCriteria(ctx, workspaceID, jobRole, criteria)
}

// DeleteJobRole removes all criteria for a job role.
func (s *SettingsService) DeleteJobRole(ctx context.Context, workspaceID, jobRole string) error {
	return s.settingsRepo.DeleteJobRole(ctx, workspaceID, jobRole)
}

// UpdateSystem updates workspace system settings.
func (s *SettingsService) UpdateSystem(ctx context.Context, workspaceID string, req model.UpdateSystemSettingsRequest) (*model.WorkspaceSettings, error) {
	return s.settingsRepo.UpdateSystem(ctx, workspaceID, req)
}
