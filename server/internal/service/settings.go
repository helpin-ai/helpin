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
	moduleGrantRepo   *repository.WorkspaceModuleGrantRepository
	gitRepo           *repository.GitRepositoryRepository
	pmWorkflowService *PMWorkflowService
	wsPublisher       *websocket.Publisher
	logger            *slog.Logger
}

var teamHandlePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var validTeamTypes = map[string]struct{}{
	model.TeamTypeEngineering: {},
	model.TeamTypeProduct:     {},
	model.TeamTypeDesign:      {},
	model.TeamTypeSupport:     {},
	model.TeamTypeMarketing:   {},
	model.TeamTypeSales:       {},
	model.TeamTypeHR:          {},
	model.TeamTypeOperations:  {},
	model.TeamTypeCustom:      {},
}

func isValidTeamType(value string) bool {
	_, ok := validTeamTypes[value]
	return ok
}

func isValidDefaultStoryType(value string) bool {
	switch value {
	case model.PMTaskTypeFeature, model.PMTaskTypeBug, model.PMTaskTypeChore:
		return true
	default:
		return false
	}
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(settingsRepo *repository.SettingsRepository, moduleGrantRepo *repository.WorkspaceModuleGrantRepository, pmWorkflowService *PMWorkflowService, wsPublisher *websocket.Publisher) *SettingsService {
	return &SettingsService{
		settingsRepo:      settingsRepo,
		moduleGrantRepo:   moduleGrantRepo,
		pmWorkflowService: pmWorkflowService,
		wsPublisher:       wsPublisher,
		logger:            slog.Default().With("service", "settings"),
	}
}

// SetGitRepositoryRepository wires repository validation for PM delivery settings.
func (s *SettingsService) SetGitRepositoryRepository(gitRepo *repository.GitRepositoryRepository) *SettingsService {
	s.gitRepo = gitRepo
	return s
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
		req.TeamType = model.TeamTypeEngineering
	}
	if !isValidTeamType(req.TeamType) {
		return nil, fmt.Errorf("invalid team_type")
	}
	if strings.TrimSpace(req.DefaultStoryType) == "" {
		req.DefaultStoryType = model.PMTaskTypeFeature
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
		if err := s.pmWorkflowService.SeedTeamWorkflow(ctx, req.WorkspaceID, team.ID, req.Name, req.TeamType); err != nil {
			s.logger.ErrorContext(ctx, "failed to seed team workflow", "error", err, "team_id", team.ID, "workspace_id", req.WorkspaceID)
			// Non-fatal: team was created successfully, workflow can be added later.
		}
	}

	s.logger.InfoContext(ctx, "team created", "team_id", team.ID, "workspace_id", req.WorkspaceID, "team_name", req.Name)
	return team, nil
}

// defaultTeamDisplayName maps a team type to the human-readable name used when
// EnsureDefaultTeam auto-creates a team for that type.
func defaultTeamDisplayName(teamType string) string {
	switch teamType {
	case model.TeamTypeEngineering:
		return "Engineering"
	case model.TeamTypeProduct:
		return "Product"
	case model.TeamTypeDesign:
		return "Design"
	case model.TeamTypeSupport:
		return "Support"
	case model.TeamTypeMarketing:
		return "Marketing"
	case model.TeamTypeSales:
		return "Sales"
	case model.TeamTypeHR:
		return "HR"
	case model.TeamTypeOperations:
		return "Operations"
	default:
		return "Team"
	}
}

// defaultTaskTypeForTeamType picks the default_task_type seeded on a team that
// EnsureDefaultTeam creates. Engineering gets feature-typed tasks; all other
// team types default to chore (their tasks rarely map to eng features/bugs).
func defaultTaskTypeForTeamType(teamType string) string {
	if teamType == model.TeamTypeEngineering {
		return model.PMTaskTypeFeature
	}
	return model.PMTaskTypeChore
}

// EnsureDefaultTeam returns the workspace's canonical team for the given type,
// creating it if it does not exist. This is the entry point for CRM surfaces
// (and any other caller) that need a "the sales team for this workspace" handle
// without requiring onboarding to have pre-created one.
//
// If multiple teams share the team_type (e.g. two sales teams), the
// earliest-created one wins. Creation races are resolved by re-querying after
// a unique-index violation: if two callers land on the same handle, the loser
// finds the winner's row and returns it.
//
// The caller's actorUserID, when non-empty, is attached as team owner so the
// user who triggered the lazy creation has immediate edit access.
func (s *SettingsService) EnsureDefaultTeam(ctx context.Context, workspaceID, teamType, actorUserID string) (*model.WorkspaceTeam, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !isValidTeamType(teamType) {
		return nil, fmt.Errorf("invalid team_type")
	}

	// Fast path: team of this type already exists.
	existing, err := s.settingsRepo.FindFirstTeamByType(ctx, workspaceID, teamType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// Pick a non-colliding handle. The base handle matches the team type
	// (e.g. "sales"); if that is taken by an unrelated team, suffix -1, -2,
	// etc. up to a small cap.
	handle, err := s.firstAvailableTeamHandle(ctx, workspaceID, teamType)
	if err != nil {
		return nil, err
	}

	req := model.CreateTeamRequest{
		WorkspaceID:      workspaceID,
		Name:             defaultTeamDisplayName(teamType),
		Handle:           &handle,
		TeamType:         teamType,
		DefaultStoryType: defaultTaskTypeForTeamType(teamType),
	}
	team, err := s.CreateTeam(ctx, req, actorUserID)
	if err == nil {
		s.logger.InfoContext(ctx, "default team auto-created", "workspace_id", workspaceID, "team_type", teamType, "team_id", team.ID)
		return team, nil
	}

	// Creation raced with a concurrent EnsureDefaultTeam or a direct CreateTeam
	// call that claimed our chosen handle. Re-query by type; if a row now
	// exists, the race is benign and we return it.
	if racer, qErr := s.settingsRepo.FindFirstTeamByType(ctx, workspaceID, teamType); qErr == nil && racer != nil {
		s.logger.InfoContext(ctx, "default team create raced, returning concurrent winner", "workspace_id", workspaceID, "team_type", teamType, "team_id", racer.ID)
		return racer, nil
	}
	return nil, err
}

// firstAvailableTeamHandle finds the first handle in the sequence
// {teamType, teamType-1, teamType-2, ...} that no team in the workspace
// currently uses. The search is bounded; if nothing is free after the cap,
// the caller gets an error rather than a silently-wrong handle.
func (s *SettingsService) firstAvailableTeamHandle(ctx context.Context, workspaceID, teamType string) (string, error) {
	base := slugifyHandle(teamType)
	if base == "" {
		return "", fmt.Errorf("cannot derive handle from team_type")
	}
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		existing, err := s.settingsRepo.GetTeamByHandle(ctx, workspaceID, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free team handle for type %q after %d attempts", teamType, maxAttempts)
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
			defaultStoryType = model.PMTaskTypeFeature
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
	team, err := s.settingsRepo.GetTeamByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load team before delete", "error", err, "team_id", id)
		return err
	}
	if team == nil {
		return fmt.Errorf("team not found")
	}
	if s.moduleGrantRepo != nil {
		if err := s.moduleGrantRepo.DeleteBySubject(ctx, team.WorkspaceID, model.ModuleGrantSubjectTeam, id); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete team module grants", "error", err, "team_id", id, "workspace_id", team.WorkspaceID)
			return err
		}
	}
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
	person, err := s.settingsRepo.GetWorkspaceMemberByID(ctx, id)
	if err != nil {
		return err
	}
	if person == nil {
		return fmt.Errorf("person not found")
	}
	if s.moduleGrantRepo != nil {
		if err := s.moduleGrantRepo.DeleteBySubject(ctx, person.WorkspaceID, model.ModuleGrantSubjectWorkspaceMember, id); err != nil {
			return err
		}
	}
	if err := s.settingsRepo.DeletePerson(ctx, id); err != nil {
		return err
	}
	return nil
}

func (s *SettingsService) ListModuleGrants(ctx context.Context, workspaceID string) ([]model.WorkspaceModuleGrant, error) {
	if s.moduleGrantRepo == nil {
		return []model.WorkspaceModuleGrant{}, nil
	}
	return s.moduleGrantRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SettingsService) UpsertModuleGrant(ctx context.Context, req model.CreateWorkspaceModuleGrantRequest, actorUserID string) (*model.WorkspaceModuleGrant, error) {
	if s.moduleGrantRepo == nil {
		return nil, fmt.Errorf("module grant repository unavailable")
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !model.IsManagedWorkspaceModule(req.Module) {
		return nil, fmt.Errorf("invalid module")
	}
	if !model.IsValidModuleGrantSubjectType(req.SubjectType) {
		return nil, fmt.Errorf("invalid subject_type")
	}
	if strings.TrimSpace(req.SubjectID) == "" {
		return nil, fmt.Errorf("subject_id is required")
	}

	switch req.SubjectType {
	case model.ModuleGrantSubjectTeam:
		ok, err := s.moduleGrantRepo.TeamBelongsToWorkspace(ctx, req.WorkspaceID, req.SubjectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("team not found in workspace")
		}
	case model.ModuleGrantSubjectWorkspaceMember:
		ok, err := s.moduleGrantRepo.WorkspaceMemberBelongsToWorkspace(ctx, req.WorkspaceID, req.SubjectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("workspace member not found in workspace")
		}
	default:
		return nil, fmt.Errorf("invalid subject_type")
	}

	var createdByID *string
	if strings.TrimSpace(actorUserID) != "" {
		createdByID = &actorUserID
	}

	grant, err := s.moduleGrantRepo.Upsert(ctx, model.WorkspaceModuleGrant{
		WorkspaceID: req.WorkspaceID,
		Module:      req.Module,
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		AccessLevel: model.ModuleGrantAccessLevelMember,
		CreatedByID: createdByID,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert module grant", "error", err, "workspace_id", req.WorkspaceID, "module", req.Module, "subject_type", req.SubjectType, "subject_id", req.SubjectID)
		return nil, err
	}
	return grant, nil
}

func (s *SettingsService) DeleteModuleGrant(ctx context.Context, workspaceID, grantID string) error {
	if s.moduleGrantRepo == nil {
		return fmt.Errorf("module grant repository unavailable")
	}
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(grantID) == "" {
		return fmt.Errorf("workspace_id and grant id are required")
	}
	if err := s.moduleGrantRepo.Delete(ctx, workspaceID, grantID); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete module grant", "error", err, "workspace_id", workspaceID, "grant_id", grantID)
		return err
	}
	return nil
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
	team, err := s.settingsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("team not found")
	}
	if s.gitRepo == nil {
		return nil, fmt.Errorf("git repository repository is not configured")
	}
	repo, err := s.gitRepo.GetEnabledByID(ctx, team.WorkspaceID, strings.TrimSpace(req.RepositoryID))
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("repository is not available for PM delivery")
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
