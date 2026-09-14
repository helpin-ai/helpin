//go:build ee

package bootstrap

import (
	"context"
	"fmt"

	eeservice "github.com/helpin-ai/helpin/server/ee/service"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type workspaceLifecycle struct {
	billing     *eeservice.BillingService
	profiles    *service.AIProfileBootstrapService
	credentials map[string]string
}

func (p *workspaceLifecycle) WorkspaceCreated(ctx context.Context, workspace string) error {
	if err := p.billing.WorkspaceCreated(ctx, workspace); err != nil {
		return err
	}
	// Defaults have already been seeded. Both the imported managed credentials and
	// the agent/profile defaults are initialized transactionally by the bootstrap.
	_, err := p.profiles.Apply(ctx, service.AIProfileBootstrapOptions{WorkspaceID: workspace, Funding: "managed", Credentials: p.credentials})
	if err != nil {
		return fmt.Errorf("initialize managed AI profiles: %w", err)
	}
	return nil
}
func (p *workspaceLifecycle) WorkspaceDeleting(ctx context.Context, workspace string) error {
	return p.billing.WorkspaceDeleting(ctx, workspace)
}
