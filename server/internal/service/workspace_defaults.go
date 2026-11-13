package service

import (
	"context"
	"fmt"
	"log/slog"
)

// CompositeDefaultsInitializer runs multiple WorkspaceDefaultsInitializer implementations in sequence.
type CompositeDefaultsInitializer struct {
	initializers []WorkspaceDefaultsInitializer
}

// NewCompositeDefaultsInitializer creates a CompositeDefaultsInitializer from one or more initializers.
func NewCompositeDefaultsInitializer(initializers ...WorkspaceDefaultsInitializer) *CompositeDefaultsInitializer {
	return &CompositeDefaultsInitializer{initializers: initializers}
}

// SeedWorkspaceDefaults calls each initializer in order, returning on the first error.
func (c *CompositeDefaultsInitializer) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	for _, init := range c.initializers {
		if err := init.SeedWorkspaceDefaults(ctx, workspaceID, actorID); err != nil {
			return fmt.Errorf("seed workspace defaults: %w", err)
		}
	}
	return nil
}

// SupportAgentBinder points a workspace's support installation at its built-in
// Support Agent.
type SupportAgentBinder interface {
	BindDefaultSupportAgent(ctx context.Context, workspaceID string) error
}

// SupportAgentDefaults binds the support installation to the built-in Support
// Agent. List it after both the support and the agent initializers: the
// installation is seeded before the agents exist, so the link can only be made
// once both are in place.
type SupportAgentDefaults struct {
	binder SupportAgentBinder
}

// NewSupportAgentDefaults creates the initializer that links support to its
// default agent.
func NewSupportAgentDefaults(binder SupportAgentBinder) *SupportAgentDefaults {
	return &SupportAgentDefaults{binder: binder}
}

// SeedWorkspaceDefaults implements WorkspaceDefaultsInitializer.
func (d *SupportAgentDefaults) SeedWorkspaceDefaults(ctx context.Context, workspaceID, _ string) error {
	if d == nil || d.binder == nil {
		return nil
	}
	// The workspace already exists by now. The link only saves an admin one
	// click, so a failure here is logged and never fails the creation; an
	// error returned after the commit would strand the workspace.
	if err := d.binder.BindDefaultSupportAgent(ctx, workspaceID); err != nil {
		slog.WarnContext(ctx, "bind default support agent", "workspace_id", workspaceID, "error", err)
	}
	return nil
}
