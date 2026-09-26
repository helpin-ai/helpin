package service

import "context"

// EntitlementPolicy is an optional edition policy. Community construction omits
// it; product authorization and operational limits remain separate.
type EntitlementPolicy interface {
	RequireFeature(context.Context, string, EntitlementFeature) error
	RequireLimitUsage(context.Context, string, EntitlementLimit, int64, int64) error
}

// WorkspaceLifecyclePolicy attaches edition behavior to workspace lifecycle events.
type WorkspaceLifecyclePolicy interface {
	WorkspaceCreated(context.Context, string) error
	WorkspaceDeleting(context.Context, string) error
}

// WorkspaceSeatPolicy permits an edition to apply its own membership capacity policy.
type WorkspaceSeatPolicy interface {
	CanReserveWorkspaceSeat(context.Context, string) error
}
