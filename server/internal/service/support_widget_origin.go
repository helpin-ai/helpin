package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

// SuggestedAllowedOrigins returns the workspace website's origins to offer
// while an installation's empty allowlist still blocks every visitor. Nothing
// is saved; an admin confirms the suggestion in settings.
func (s *SupportInboxService) SuggestedAllowedOrigins(ctx context.Context, inst *model.SupportWidgetInstallation) []string {
	if inst == nil || len(inst.AllowedOrigins) > 0 {
		return nil
	}
	return s.websiteOriginDefaults(ctx, inst.WorkspaceID)
}

// ApplyWebsiteOriginDefaults fills an empty widget allowlist from the workspace
// website, so a workspace that adds its website during setup can use the widget
// there without another step. Saved origins, including allow-all, never change.
func (s *SupportInboxService) ApplyWebsiteOriginDefaults(ctx context.Context, workspaceID string) error {
	if s.installationRepo == nil {
		return nil
	}
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("get widget installation: %w", err)
	}
	if inst == nil || len(inst.AllowedOrigins) > 0 {
		return nil
	}
	origins := s.websiteOriginDefaults(ctx, workspaceID)
	if len(origins) == 0 {
		return nil
	}
	applied, err := s.installationRepo.SetAllowedOriginsIfEmpty(ctx, inst.ID, origins)
	if err != nil {
		return err
	}
	if applied {
		slog.InfoContext(ctx, "widget origins set from workspace website",
			"workspace_id", workspaceID, "installation_id", inst.ID)
	}
	return nil
}

// websiteOriginDefaults returns the allowlist suggested by the workspace
// website, or nil when the workspace has no website or it cannot be read.
func (s *SupportInboxService) websiteOriginDefaults(ctx context.Context, workspaceID string) []string {
	if s.workspaceRepo == nil {
		return nil
	}
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "read workspace website for widget origins", "error", err, "workspace_id", workspaceID)
		return nil
	}
	if ws == nil || ws.WebsiteURL == nil {
		return nil
	}
	return widgetorigin.WebsiteDefaults(*ws.WebsiteURL)
}

// AuthorizeWidgetOrigin checks installation policy without extending or touching
// a session. All supplied references must agree before a visitor action runs.
func (s *SupportInboxService) AuthorizeWidgetOrigin(ctx context.Context, origin string, ref widgetorigin.Reference) error {
	denied := fmt.Errorf("widget origin or credentials are not allowed")
	var installation *model.SupportWidgetInstallation
	accept := func(candidate *model.SupportWidgetInstallation, err error) error {
		if err != nil {
			return fmt.Errorf("resolve widget installation: %w", err)
		}
		if candidate == nil || !candidate.Active {
			return denied
		}
		if installation != nil && installation.ID != candidate.ID {
			return denied
		}
		installation = candidate
		return nil
	}
	if ref.WidgetKey != "" {
		if err := accept(s.installationRepo.GetByWidgetKey(ctx, ref.WidgetKey)); err != nil {
			return err
		}
	}
	if ref.InstallationID != "" {
		if err := accept(s.installationRepo.GetByID(ctx, ref.InstallationID)); err != nil {
			return err
		}
	}
	if ref.SessionToken != "" {
		session, err := s.sessionRepo.GetByToken(ctx, ref.SessionToken)
		if err != nil {
			return fmt.Errorf("resolve widget session: %w", err)
		}
		if session == nil || session.RevokedAt != nil || !session.ExpiresAt.After(time.Now()) {
			return denied
		}
		// GetInstallation maintains one installation per workspace. Sessions are
		// workspace-bound today; multiple installations would require persisting
		// the installation ID on each session before changing that invariant.
		if err := accept(s.installationRepo.GetByWorkspace(ctx, session.WorkspaceID)); err != nil {
			return err
		}
	}
	if installation == nil {
		return denied
	}
	if !widgetorigin.Allowed(origin, installation.AllowedOrigins) {
		return &widgetorigin.DeniedError{InstallationID: installation.ID}
	}
	return nil
}
