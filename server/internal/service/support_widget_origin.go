package service

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/widgetorigin"
)

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
		if err := accept(s.installationRepo.GetByWorkspace(ctx, session.WorkspaceID)); err != nil {
			return err
		}
	}
	if installation == nil || !widgetorigin.Allowed(origin, installation.AllowedOrigins) {
		return denied
	}
	return nil
}
