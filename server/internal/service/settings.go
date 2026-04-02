package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SettingsService handles workspace configuration business logic.
type SettingsService struct {
	settingsRepo      *repository.SettingsRepository
	pmWorkflowService *PMWorkflowService
	wsPublisher       *websocket.Publisher
	logger            *slog.Logger
}

var teamHandlePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var validTeamTypes = map[string]struct{}{
	"engineering": {},
	"product":     {},
	"design":      {},
	"support":     {},
	"marketing":   {},
	"sales":       {},
	"hr":          {},
	"operations":  {},
	"custom":      {},
}

func isValidTeamType(value string) bool {
	_, ok := validTeamTypes[value]
	return ok
}

func isValidDefaultStoryType(value string) bool {
	switch value {
	case model.PMStoryTypeFeature, model.PMStoryTypeBug, model.PMStoryTypeChore:
		return true
	default:
		return false
	}
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(settingsRepo *repository.SettingsRepository, pmWorkflowService *PMWorkflowService, wsPublisher *websocket.Publisher) *SettingsService {
	return &SettingsService{
		settingsRepo:      settingsRepo,
		pmWorkflowService: pmWorkflowService,
		wsPublisher:       wsPublisher,
		logger:            slog.Default().With("service", "settings"),
	}
}

// GetAll returns the full workspace configuration.
func (s *SettingsService) GetAll(ctx context.Context, workspaceID string) (*model.FullWorkspaceConfig, error) {
	config, err := s.settingsRepo.GetAll(ctx, workspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get workspace config", "error", err, "workspace_id", workspaceID)
		return nil, err
	}
	s.logger.DebugContext(ctx, "fetched workspace config", "workspace_id", workspaceID)
	return config, nil
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
	if strings.TrimSpace(req.TeamType) == "" {
		req.TeamType = "engineering"
	}
	if !isValidTeamType(req.TeamType) {
		return nil, fmt.Errorf("invalid team_type")
	}
	if strings.TrimSpace(req.DefaultStoryType) == "" {
		req.DefaultStoryType = model.PMStoryTypeFeature
	}
	if !isValidDefaultStoryType(req.DefaultStoryType) {
		return nil, fmt.Errorf("invalid default_task_type")
	}

	team, err := s.settingsRepo.CreateTeam(ctx, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create team", "error", err, "workspace_id", req.WorkspaceID, "team_name", req.Name)
		return nil, err
	}
	if actorUserID != "" {
		_, err = s.settingsRepo.AddTeamUserMembership(ctx, team.ID, actorUserID, "owner")
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to add team owner membership", "error", err, "team_id", team.ID, "user_id", actorUserID)
			return nil, err
		}
	}
	// Auto-add workspace admins and owners to the new team.
	adminIDs, err := s.settingsRepo.ListAdminOwnerUserIDs(ctx, req.WorkspaceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list admins for team auto-add", "error", err, "team_id", team.ID)
		// Non-fatal: team was created, admins can be added manually.
	} else {
		for _, uid := range adminIDs {
			if uid == actorUserID {
				continue // already added as owner
			}
			if _, err := s.settingsRepo.AddTeamUserMembership(ctx, team.ID, uid, "member"); err != nil {
				s.logger.ErrorContext(ctx, "failed to auto-add admin to team", "error", err, "team_id", team.ID, "user_id", uid)
				// Non-fatal: continue with remaining admins.
			}
		}
	}
	// Seed a default workflow for the new team.
	if s.pmWorkflowService != nil {
		if err := s.pmWorkflowService.SeedTeamWorkflow(ctx, req.WorkspaceID, team.ID, req.Name); err != nil {
			s.logger.ErrorContext(ctx, "failed to seed team workflow", "error", err, "team_id", team.ID, "workspace_id", req.WorkspaceID)
			// Non-fatal: team was created successfully, workflow can be added later.
		}
	}

	s.logger.InfoContext(ctx, "team created", "team_id", team.ID, "workspace_id", req.WorkspaceID, "team_name", req.Name)
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
	if req.TeamType != nil {
		teamType := strings.TrimSpace(*req.TeamType)
		if teamType == "" {
			teamType = "engineering"
		}
		if !isValidTeamType(teamType) {
			return nil, fmt.Errorf("invalid team_type")
		}
		req.TeamType = &teamType
	}
	if req.DefaultStoryType != nil {
		defaultStoryType := strings.TrimSpace(*req.DefaultStoryType)
		if defaultStoryType == "" {
			defaultStoryType = model.PMStoryTypeFeature
		}
		if !isValidDefaultStoryType(defaultStoryType) {
			return nil, fmt.Errorf("invalid default_task_type")
		}
		req.DefaultStoryType = &defaultStoryType
	}
	return s.settingsRepo.UpdateTeam(ctx, id, req)
}

// DeleteTeam removes a team.
func (s *SettingsService) DeleteTeam(ctx context.Context, id string) error {
	if err := s.settingsRepo.DeleteTeam(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete team", "error", err, "team_id", id)
		return err
	}
	s.logger.InfoContext(ctx, "team deleted", "team_id", id)
	return nil
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
func (s *SettingsService) UpdateTeamMember(ctx context.Context, teamID, userID string, actor *authorization.Actor, req model.UpdateTeamMemberRequest) (*model.TeamUserMembership, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if actor != nil && actor.UserID == userID {
		return nil, fmt.Errorf("cannot change your own team role")
	}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if !isValidTeamMemberRole(role) {
			return nil, fmt.Errorf("invalid role")
		}
		req.Role = &role
	}
	// Check if target is a team owner — only workspace admin/owner can change team owners
	if actor != nil {
		isWsAdmin := actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin
		target, err := s.settingsRepo.GetTeamUserMembership(ctx, teamID, userID)
		if err != nil {
			return nil, err
		}
		if target != nil && target.Role == "owner" && !isWsAdmin {
			return nil, fmt.Errorf("only workspace admins can change a team owner's role")
		}
		// Prevent demoting the last team owner
		if target != nil && target.Role == "owner" && req.Role != nil && *req.Role != "owner" {
			count, err := s.settingsRepo.CountTeamMembersByRole(ctx, teamID, "owner")
			if err != nil {
				return nil, err
			}
			if count <= 1 {
				return nil, fmt.Errorf("cannot demote the last team owner")
			}
		}
	}
	return s.settingsRepo.UpdateTeamUserMembership(ctx, teamID, userID, req)
}

// RemoveTeamMember removes a user from a team.
func (s *SettingsService) RemoveTeamMember(ctx context.Context, teamID, userID string, actor *authorization.Actor) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if actor != nil && actor.UserID == userID {
		return fmt.Errorf("cannot remove yourself from the team")
	}
	// Check if target is a team owner — only workspace admin/owner can remove team owners
	if actor != nil {
		isWsAdmin := actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin
		target, err := s.settingsRepo.GetTeamUserMembership(ctx, teamID, userID)
		if err != nil {
			return err
		}
		if target != nil && target.Role == "owner" && !isWsAdmin {
			return fmt.Errorf("only workspace admins can remove a team owner")
		}
		// Prevent removing the last team owner
		if target != nil && target.Role == "owner" {
			count, err := s.settingsRepo.CountTeamMembersByRole(ctx, teamID, "owner")
			if err != nil {
				return err
			}
			if count <= 1 {
				return fmt.Errorf("cannot remove the last team owner")
			}
		}
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
	result, err := s.settingsRepo.UpdateSystem(ctx, workspaceID, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update system settings", "error", err, "workspace_id", workspaceID)
		return nil, err
	}
	s.logger.InfoContext(ctx, "system settings updated", "workspace_id", workspaceID)
	return result, nil
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
	settings, err := s.settingsRepo.UpsertTeamEstimateSettings(ctx, teamID, req)
	if err != nil {
		return nil, err
	}
	team, err := s.settingsRepo.GetTeamByID(ctx, teamID)
	if err == nil && team != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "updated", "team_estimate_settings", teamID, team.WorkspaceID, "", "team", teamID, nil)
	}
	return settings, nil
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
