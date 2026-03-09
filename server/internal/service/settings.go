package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SettingsService handles workspace configuration business logic.
type SettingsService struct {
	settingsRepo      *repository.SettingsRepository
	braveSearchAPIKey string
}

var teamHandlePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// NewSettingsService creates a new SettingsService.
func NewSettingsService(settingsRepo *repository.SettingsRepository, braveSearchAPIKey string) *SettingsService {
	return &SettingsService{
		settingsRepo:      settingsRepo,
		braveSearchAPIKey: strings.TrimSpace(braveSearchAPIKey),
	}
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
func (s *SettingsService) CreateTeam(ctx context.Context, req model.CreateTeamRequest, actorUserID string) (*model.WorkspaceTeam, error) {
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	handle, err := s.prepareTeamHandle(ctx, req.WorkspaceID, req.Handle, req.Name, nil)
	if err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Handle = &handle

	team, err := s.settingsRepo.CreateTeam(ctx, req)
	if err != nil {
		return nil, err
	}
	if actorUserID != "" {
		_, err = s.settingsRepo.AddTeamUserMembership(ctx, team.ID, actorUserID, "owner")
		if err != nil {
			return nil, err
		}
	}
	return team, nil
}

// UpdateTeam modifies a team.
func (s *SettingsService) UpdateTeam(ctx context.Context, id string, req model.UpdateTeamRequest) (*model.WorkspaceTeam, error) {
	current, err := s.settingsRepo.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("team not found")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		req.Name = &name
	}
	if req.Handle != nil {
		handle, err := s.prepareTeamHandle(ctx, current.WorkspaceID, req.Handle, current.Name, &current.ID)
		if err != nil {
			return nil, err
		}
		req.Handle = &handle
	}
	return s.settingsRepo.UpdateTeam(ctx, id, req)
}

// DeleteTeam removes a team.
func (s *SettingsService) DeleteTeam(ctx context.Context, id string) error {
	return s.settingsRepo.DeleteTeam(ctx, id)
}

// AddTeamMember adds a workspace member to a team.
func (s *SettingsService) AddTeamMember(ctx context.Context, teamID string, req model.AddTeamMemberRequest) (*model.TeamUserMembership, error) {
	if strings.TrimSpace(req.UserID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "member"
	}
	if !isValidTeamMemberRole(role) {
		return nil, fmt.Errorf("invalid role")
	}
	return s.settingsRepo.AddTeamUserMembership(ctx, teamID, req.UserID, role)
}

// UpdateTeamMember updates a user's role within a team.
func (s *SettingsService) UpdateTeamMember(ctx context.Context, teamID, userID string, req model.UpdateTeamMemberRequest) (*model.TeamUserMembership, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if !isValidTeamMemberRole(role) {
			return nil, fmt.Errorf("invalid role")
		}
		req.Role = &role
	}
	return s.settingsRepo.UpdateTeamUserMembership(ctx, teamID, userID, req)
}

// RemoveTeamMember removes a user from a team.
func (s *SettingsService) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	return s.settingsRepo.RemoveTeamUserMembership(ctx, teamID, userID)
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
	if req.PlanningMethodology != nil {
		methodology := normalizePlanningMethodology(*req.PlanningMethodology)
		req.PlanningMethodology = &methodology
	}
	if req.PlanningWebSearchProvider != nil {
		provider := model.NormalizePlanningWebSearchProvider(*req.PlanningWebSearchProvider)
		req.PlanningWebSearchProvider = &provider
	}
	if err := s.validatePlanningWebSearchSettings(ctx, workspaceID, req); err != nil {
		return nil, err
	}
	return s.settingsRepo.UpdateSystem(ctx, workspaceID, req)
}

func (s *SettingsService) validatePlanningWebSearchSettings(ctx context.Context, workspaceID string, req model.UpdateSystemSettingsRequest) error {
	var current *model.WorkspaceSettings
	if s.settingsRepo != nil {
		settings, err := s.settingsRepo.GetWorkspaceSettings(ctx, workspaceID)
		if err != nil {
			return err
		}
		current = settings
	}
	enabled, provider := resolvePlanningWebSearchConfig(current, req)

	if enabled && provider == model.PlanningWebSearchProviderBrave && s.braveSearchAPIKey == "" {
		return fmt.Errorf("brave web search is not available because BRAVE_SEARCH_API_KEY is not configured")
	}

	return nil
}

func resolvePlanningWebSearchConfig(current *model.WorkspaceSettings, req model.UpdateSystemSettingsRequest) (bool, string) {
	if current == nil {
		current = &model.WorkspaceSettings{
			PlanningWebSearchProvider: model.PlanningWebSearchProviderBrave,
		}
	}

	enabled := current.PlanningWebSearchEnabled
	if req.PlanningWebSearchEnabled != nil {
		enabled = *req.PlanningWebSearchEnabled
	}
	provider := model.NormalizePlanningWebSearchProvider(current.PlanningWebSearchProvider)
	if req.PlanningWebSearchProvider != nil {
		provider = model.NormalizePlanningWebSearchProvider(*req.PlanningWebSearchProvider)
	}
	if provider == "" {
		provider = model.PlanningWebSearchProviderBrave
	}
	return enabled, provider
}

func (s *SettingsService) prepareTeamHandle(ctx context.Context, workspaceID string, requested *string, name string, excludeID *string) (string, error) {
	source := name
	if requested != nil {
		source = *requested
	}
	handle := slugifyHandle(source)
	if handle == "" {
		return "", fmt.Errorf("team handle is required")
	}
	if !teamHandlePattern.MatchString(handle) {
		return "", fmt.Errorf("team handle must contain only lowercase letters, numbers, and single hyphens")
	}
	existing, err := s.settingsRepo.GetTeamByHandle(ctx, workspaceID, handle)
	if err != nil {
		return "", err
	}
	if existing != nil && (excludeID == nil || existing.ID != *excludeID) {
		return "", fmt.Errorf("team handle already exists")
	}
	return handle, nil
}

func slugifyHandle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}

func isValidTeamMemberRole(role string) bool {
	return role == "owner" || role == "member"
}

// validEstimateScales enumerates the allowed estimate scale values.
var validEstimateScales = map[string]bool{
	"exponential": true,
	"fibonacci":   true,
	"linear":      true,
	"tshirt":      true,
	"hours":       true,
}

// GetTeamEstimateSettings returns estimate settings for a team.
func (s *SettingsService) GetTeamEstimateSettings(ctx context.Context, teamID string) (*model.PMTeamEstimateSettings, error) {
	return s.settingsRepo.GetTeamEstimateSettings(ctx, teamID)
}

// UpdateTeamEstimateSettings creates or updates estimate settings for a team.
func (s *SettingsService) UpdateTeamEstimateSettings(ctx context.Context, teamID string, req model.UpdateTeamEstimateSettingsRequest) (*model.PMTeamEstimateSettings, error) {
	if req.Scale != nil && !validEstimateScales[*req.Scale] {
		return nil, fmt.Errorf("invalid estimate scale: %s", *req.Scale)
	}
	return s.settingsRepo.UpsertTeamEstimateSettings(ctx, teamID, req)
}

// GetTeamFieldVisibility returns field visibility settings for a team.
func (s *SettingsService) GetTeamFieldVisibility(ctx context.Context, teamID string) (*model.PMTeamFieldVisibility, error) {
	return s.settingsRepo.GetTeamFieldVisibility(ctx, teamID)
}

// UpdateTeamFieldVisibility creates or updates field visibility settings for a team.
func (s *SettingsService) UpdateTeamFieldVisibility(ctx context.Context, teamID string, req model.UpdateTeamFieldVisibilityRequest) (*model.PMTeamFieldVisibility, error) {
	return s.settingsRepo.UpsertTeamFieldVisibility(ctx, teamID, req)
}

// GetTeamRepoDefault returns the delivery repo default for a team.
func (s *SettingsService) GetTeamRepoDefault(ctx context.Context, teamID string) (*model.PMTeamRepoDefault, error) {
	return s.settingsRepo.GetTeamRepoDefault(ctx, teamID)
}

// UpdateTeamRepoDefault creates or updates the delivery repo default for a team.
func (s *SettingsService) UpdateTeamRepoDefault(ctx context.Context, teamID string, req model.UpdateTeamRepoDefaultRequest) (*model.PMTeamRepoDefault, error) {
	if strings.TrimSpace(req.RepositoryID) == "" {
		return nil, fmt.Errorf("repository_id is required")
	}
	if req.BaseBranch != nil {
		trimmed := strings.TrimSpace(*req.BaseBranch)
		req.BaseBranch = &trimmed
	}
	if req.BranchTemplate != nil {
		trimmed := strings.TrimSpace(*req.BranchTemplate)
		req.BranchTemplate = &trimmed
	}
	return s.settingsRepo.UpsertTeamRepoDefault(ctx, teamID, req)
}

// AddInvitationTeamPreassignment pre-assigns a pending invitation to a team.
func (s *SettingsService) AddInvitationTeamPreassignment(ctx context.Context, teamID, invitationID string) (*model.InvitationTeamPreassignment, error) {
	return s.settingsRepo.AddInvitationTeamPreassignment(ctx, invitationID, teamID)
}

// RemoveInvitationTeamPreassignment removes a preassignment.
func (s *SettingsService) RemoveInvitationTeamPreassignment(ctx context.Context, teamID, invitationID string) error {
	return s.settingsRepo.RemoveInvitationTeamPreassignment(ctx, invitationID, teamID)
}
