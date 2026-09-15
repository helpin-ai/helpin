//go:build ee

package bootstrap

import (
	"context"
	"fmt"

	eeservice "github.com/helpin-ai/helpin/server/ee/service"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type workspaceLifecycle struct {
	billing  *eeservice.BillingService
	profiles *service.AIStandardProfiles
}

func (p *workspaceLifecycle) WorkspaceCreated(ctx context.Context, workspace string) error {
	if err := p.billing.WorkspaceCreated(ctx, workspace); err != nil {
		return err
	}
	// Provision the shared defaults without changing saved agent selections.
	if err := p.profiles.EnsureWorkspace(ctx, workspace); err != nil {
		return fmt.Errorf("initialize managed AI profiles: %w", err)
	}
	return nil
}
func (p *workspaceLifecycle) WorkspaceDeleting(ctx context.Context, workspace string) error {
	return p.billing.WorkspaceDeleting(ctx, workspace)
}
