package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// parseSettings unmarshals the JSONB settings string, applying defaults for missing fields.
func parseSettings(raw string) model.SupportInboxSettings {
	defaults := model.DefaultSupportInboxSettings()
	if raw == "" || raw == "{}" {
		return defaults
	}
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return model.DefaultSupportInboxSettings()
	}
	return defaults
}

// mergeSettingsUpdate applies non-nil patch fields onto current settings.
func mergeSettingsUpdate(current model.SupportInboxSettings, patch model.UpdateInstallationSettingsRequest) model.SupportInboxSettings {
	if patch.RequireEmailBeforeChat != nil {
		current.RequireEmailBeforeChat = *patch.RequireEmailBeforeChat
	}
	if patch.RequirePhoneAfterEmail != nil {
		current.RequirePhoneAfterEmail = *patch.RequirePhoneAfterEmail
	}
	if patch.WelcomeMessage != nil {
		current.WelcomeMessage = *patch.WelcomeMessage
	}
	if patch.AIEnabled != nil {
		current.AIEnabled = *patch.AIEnabled
	}
	if patch.AIAgentID != nil {
		trimmed := strings.TrimSpace(*patch.AIAgentID)
		if trimmed == "" {
			current.AIAgentID = nil
		} else {
			current.AIAgentID = &trimmed
		}
	}
	if patch.AIConfidenceThreshold != nil {
		current.AIConfidenceThreshold = *patch.AIConfidenceThreshold
	}
	if patch.AIResponseMode != nil {
		current.AIResponseMode = *patch.AIResponseMode
	}
	if patch.AIMaxFollowups != nil {
		current.AIMaxFollowups = *patch.AIMaxFollowups
	}
	if patch.AIAutoResolveTimeout != nil {
		current.AIAutoResolveTimeout = *patch.AIAutoResolveTimeout
	}
	if patch.ShowTalkToHuman != nil {
		current.ShowTalkToHuman = *patch.ShowTalkToHuman
	}
	if patch.HandoffBehavior != nil {
		current.HandoffBehavior = *patch.HandoffBehavior
	}
	if patch.HandoffTeamID != nil {
		current.HandoffTeamID = patch.HandoffTeamID
	}
	if patch.BusinessHoursEnabled != nil {
		current.BusinessHoursEnabled = *patch.BusinessHoursEnabled
	}
	if patch.BusinessHoursTimezone != nil {
		current.BusinessHoursTimezone = *patch.BusinessHoursTimezone
	}
	if patch.BusinessHoursSchedule != nil {
		current.BusinessHoursSchedule = patch.BusinessHoursSchedule
	}
	if patch.OutsideHoursMessage != nil {
		current.OutsideHoursMessage = *patch.OutsideHoursMessage
	}
	if patch.EmailFallbackEnabled != nil {
		current.EmailFallbackEnabled = *patch.EmailFallbackEnabled
	}
	if patch.EmailFallbackDelaySecs != nil {
		current.EmailFallbackDelaySecs = *patch.EmailFallbackDelaySecs
	}
	if patch.EmailFallbackFromName != nil {
		current.EmailFallbackFromName = *patch.EmailFallbackFromName
	}
	if patch.WidgetName != nil {
		current.WidgetName = *patch.WidgetName
	}
	if patch.WidgetAvatarURL != nil {
		current.WidgetAvatarURL = *patch.WidgetAvatarURL
	}
	if patch.WidgetHelpSpaceIDs != nil {
		current.WidgetHelpSpaceIDs = append([]string(nil), patch.WidgetHelpSpaceIDs...)
	}
	if patch.BrandColor != nil {
		current.BrandColor = *patch.BrandColor
	}
	if patch.ShowBranding != nil {
		current.ShowBranding = *patch.ShowBranding
	}
	if patch.ColorScheme != nil {
		current.ColorScheme = *patch.ColorScheme
	}
	if patch.ButtonColor != nil {
		current.ButtonColor = *patch.ButtonColor
	}
	if patch.ButtonIconColor != nil {
		current.ButtonIconColor = *patch.ButtonIconColor
	}
	if patch.LogoURL != nil {
		current.LogoURL = *patch.LogoURL
	}
	if patch.LauncherPosition != nil {
		current.LauncherPosition = *patch.LauncherPosition
	}
	if patch.LauncherIcon != nil {
		current.LauncherIcon = *patch.LauncherIcon
	}
	if patch.CSATEnabled != nil {
		current.CSATEnabled = *patch.CSATEnabled
	}
	if patch.FileUploadsEnabled != nil {
		current.FileUploadsEnabled = *patch.FileUploadsEnabled
	}
	if patch.ForceVisitorIdentity != nil {
		current.ForceVisitorIdentity = *patch.ForceVisitorIdentity
	}
	return current
}

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateSettings checks settings field constraints.
func (s *SupportInboxService) validateSettings(ctx context.Context, workspaceID string, settings model.SupportInboxSettings) error {
	if settings.AIConfidenceThreshold < 0 || settings.AIConfidenceThreshold > 1 {
		return fmt.Errorf("ai_confidence_threshold must be between 0.0 and 1.0")
	}
	if settings.BrandColor != "" && !hexColorRegex.MatchString(settings.BrandColor) {
		return fmt.Errorf("brand_color must be a valid hex color (e.g. #6366F1)")
	}
	if settings.ButtonColor != "" && !hexColorRegex.MatchString(settings.ButtonColor) {
		return fmt.Errorf("button_color must be a valid hex color (e.g. #000000)")
	}
	if settings.ButtonIconColor != "" && !hexColorRegex.MatchString(settings.ButtonIconColor) {
		return fmt.Errorf("button_icon_color must be a valid hex color (e.g. #FFFFFF)")
	}
	validColorScheme := map[string]bool{"system": true, "light": true, "dark": true}
	if settings.ColorScheme != "" && !validColorScheme[settings.ColorScheme] {
		return fmt.Errorf("color_scheme must be system, light, or dark")
	}
	validHandoff := map[string]bool{"unassigned": true, "assign_to_team": true, "round_robin": true}
	if !validHandoff[settings.HandoffBehavior] {
		return fmt.Errorf("handoff_behavior must be unassigned, assign_to_team, or round_robin")
	}
	if settings.HandoffBehavior == "assign_to_team" && (settings.HandoffTeamID == nil || *settings.HandoffTeamID == "") {
		return fmt.Errorf("handoff_team_id is required when handoff_behavior is assign_to_team")
	}
	validPosition := map[string]bool{"bottom_right": true, "bottom_left": true}
	if !validPosition[settings.LauncherPosition] {
		return fmt.Errorf("launcher_position must be bottom_right or bottom_left")
	}
	validIcon := map[string]bool{"chat_bubble": true, "question_mark": true, "help": true}
	if !validIcon[settings.LauncherIcon] {
		return fmt.Errorf("launcher_icon must be chat_bubble, question_mark, or help")
	}
	validResponseMode := map[string]bool{"ai_first": true, "off": true}
	if settings.AIResponseMode != "" && !validResponseMode[settings.AIResponseMode] {
		return fmt.Errorf("ai_response_mode must be ai_first or off")
	}
	if settings.AIMaxFollowups < 0 || settings.AIMaxFollowups > 50 {
		return fmt.Errorf("ai_max_followups must be between 0 and 50")
	}
	if settings.AIAutoResolveTimeout < 0 {
		return fmt.Errorf("ai_auto_resolve_timeout must be >= 0")
	}
	if settings.EmailFallbackDelaySecs < 30 || settings.EmailFallbackDelaySecs > 600 {
		return fmt.Errorf("email_fallback_delay_secs must be between 30 and 600")
	}
	if settings.AIEnabled {
		if settings.AIAgentID == nil || strings.TrimSpace(*settings.AIAgentID) == "" {
			return fmt.Errorf("ai_agent_id is required when ai_enabled is true")
		}
		if s == nil || s.agentRepo == nil {
			return fmt.Errorf("support agent validation is unavailable")
		}
		agent, err := s.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*settings.AIAgentID))
		if err != nil {
			return fmt.Errorf("get support ai agent: %w", err)
		}
		if agent == nil {
			return fmt.Errorf("selected support ai agent was not found")
		}
		if agent.AgentClass != model.AgentClassSupport {
			return fmt.Errorf("selected ai agent must be a support agent")
		}
		if err := validateAgentTarget(agent, "support_conversation"); err != nil {
			return err
		}
	}
	return nil
}

// isOnline computes whether the widget is currently within business hours.
func isOnline(s model.SupportInboxSettings) bool {
	if !s.BusinessHoursEnabled {
		return true // always online when business hours not configured
	}

	loc, err := time.LoadLocation(s.BusinessHoursTimezone)
	if err != nil {
		return true // fallback to online if timezone invalid
	}

	now := time.Now().In(loc)
	dayNames := map[time.Weekday]string{
		time.Monday: "mon", time.Tuesday: "tue", time.Wednesday: "wed",
		time.Thursday: "thu", time.Friday: "fri", time.Saturday: "sat", time.Sunday: "sun",
	}
	dayKey := dayNames[now.Weekday()]
	day, ok := s.BusinessHoursSchedule[dayKey]
	if !ok || !day.Enabled {
		return false
	}

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := parseTimeToMinutes(day.Start)
	endMinutes := parseTimeToMinutes(day.End)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

func parseTimeToMinutes(t string) int {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h*60 + m
}

// GetInstallation returns the installation and its parsed settings for a workspace.
// If no installation exists yet, one is auto-created with default settings and a new widget key.
func (s *SupportInboxService) GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		widgetKey, err := generateSecureToken(16)
		if err != nil {
			return nil, nil, fmt.Errorf("generate widget key: %w", err)
		}
		secretKey, err := generateSecureToken(32)
		if err != nil {
			return nil, nil, fmt.Errorf("generate secret key: %w", err)
		}

		defaults := model.DefaultSupportInboxSettings()
		raw, err := json.Marshal(defaults)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal default settings: %w", err)
		}

		inst = &model.SupportWidgetInstallation{
			WorkspaceID: workspaceID,
			WidgetKey:   widgetKey,
			SecretKey:   secretKey,
			Settings:    string(raw),
			Active:      true,
		}
		if err := s.installationRepo.Create(ctx, inst); err != nil {
			return nil, nil, fmt.Errorf("create widget installation: %w", err)
		}

		slog.InfoContext(ctx, "auto-created support widget installation", "workspace_id", workspaceID, "installation_id", inst.ID)
		return inst, &defaults, nil
	}
	settings := parseSettings(inst.Settings)
	return inst, &settings, nil
}

// UpdateInstallationSettings merges, validates, and saves settings.
func (s *SupportInboxService) UpdateInstallationSettings(ctx context.Context, workspaceID string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("no widget installation found for this workspace")
	}

	current := parseSettings(inst.Settings)
	merged := mergeSettingsUpdate(current, req)
	if err := s.validateSettings(ctx, workspaceID, merged); err != nil {
		return nil, nil, err
	}

	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal settings: %w", err)
	}
	inst.Settings = string(raw)

	if err := s.installationRepo.Update(ctx, inst); err != nil {
		return nil, nil, err
	}

	// Broadcast config update to all connected widget clients in this workspace
	if configResp, err := s.buildWidgetConfigResponse(ctx, inst); err == nil {
		configData, _ := json.Marshal(configResp)
		s.wsPublisher.Publish(websocket.Event{
			Action:      "config_updated",
			Entity:      "support_widget",
			EntityID:    inst.ID,
			WorkspaceID: workspaceID,
			Data:        configData,
		})
	} else {
		slog.ErrorContext(ctx, "failed to build updated support widget config", "error", err, "workspace_id", workspaceID)
	}

	slog.InfoContext(ctx, "updated support installation settings", "workspace_id", workspaceID)
	return inst, &merged, nil
}

// RegenerateWidgetKey generates a new widget key + secret key.
// If no installation exists yet, one is created with default settings.
func (s *SupportInboxService) RegenerateWidgetKey(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	// No installation yet — create one (handles edge cases where seeding was missed).
	if inst == nil {
		inst, settings, err := s.GetInstallation(ctx, workspaceID)
		if err != nil {
			return nil, nil, fmt.Errorf("create widget installation: %w", err)
		}
		return inst, settings, nil
	}

	newWidgetKey, err := generateSecureToken(16)
	if err != nil {
		return nil, nil, fmt.Errorf("generate widget key: %w", err)
	}
	newSecretKey, err := generateSecureToken(32)
	if err != nil {
		return nil, nil, fmt.Errorf("generate secret key: %w", err)
	}

	if err := s.installationRepo.RegenerateKeys(ctx, inst.ID, newWidgetKey, newSecretKey); err != nil {
		return nil, nil, err
	}

	inst.WidgetKey = newWidgetKey
	inst.SecretKey = newSecretKey

	settings := parseSettings(inst.Settings)
	slog.InfoContext(ctx, "regenerated support widget keys", "workspace_id", workspaceID, "installation_id", inst.ID)
	return inst, &settings, nil
}

// SeedWorkspaceDefaults creates a default widget installation for a new workspace.
func (s *SupportInboxService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	existing, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("check existing installation: %w", err)
	}
	if existing != nil {
		return nil // already seeded
	}

	widgetKey, err := generateSecureToken(16)
	if err != nil {
		return fmt.Errorf("generate widget key: %w", err)
	}
	secretKey, err := generateSecureToken(32)
	if err != nil {
		return fmt.Errorf("generate secret key: %w", err)
	}

	defaults := model.DefaultSupportInboxSettings()
	raw, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("marshal default settings: %w", err)
	}

	inst := &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   secretKey,
		Settings:    string(raw),
		Active:      true,
	}
	if err := s.installationRepo.Create(ctx, inst); err != nil {
		return fmt.Errorf("create widget installation: %w", err)
	}

	slog.InfoContext(ctx, "seeded support widget installation", "workspace_id", workspaceID, "installation_id", inst.ID)
	return nil
}

// ListCannedResponses returns all canned responses for a workspace.
func (s *SupportInboxService) ListCannedResponses(ctx context.Context, workspaceID string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.List(ctx, workspaceID)
}

// SearchCannedResponses returns canned responses matching a query.
func (s *SupportInboxService) SearchCannedResponses(ctx context.Context, workspaceID, query string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.Search(ctx, workspaceID, query)
}

// CreateCannedResponse creates a new canned response.
func (s *SupportInboxService) CreateCannedResponse(ctx context.Context, workspaceID string, req model.CannedResponseRequest, createdByID string) (*model.SupportCannedResponse, error) {
	if strings.TrimSpace(req.ShortCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("short_code, title, and content are required")
	}

	response := &model.SupportCannedResponse{
		WorkspaceID: workspaceID,
		ShortCode:   req.ShortCode,
		Title:       req.Title,
		Content:     req.Content,
		CreatedByID: createdByID,
	}
	if err := s.cannedResponseRepo.Create(ctx, response); err != nil {
		return nil, fmt.Errorf("create canned response: %w", err)
	}
	return response, nil
}

// UpdateCannedResponse updates an existing canned response.
func (s *SupportInboxService) UpdateCannedResponse(ctx context.Context, workspaceID, id string, req model.CannedResponseRequest) (*model.SupportCannedResponse, error) {
	response, err := s.cannedResponseRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("canned response not found")
	}

	response.ShortCode = req.ShortCode
	response.Title = req.Title
	response.Content = req.Content

	if err := s.cannedResponseRepo.Update(ctx, response); err != nil {
		return nil, fmt.Errorf("update canned response: %w", err)
	}
	return response, nil
}

// DeleteCannedResponse deletes a canned response.
func (s *SupportInboxService) DeleteCannedResponse(ctx context.Context, workspaceID, id string) error {
	return s.cannedResponseRepo.Delete(ctx, workspaceID, id)
}
