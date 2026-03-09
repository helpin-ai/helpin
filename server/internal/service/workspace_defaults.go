package service

import (
	"context"
	"fmt"
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
