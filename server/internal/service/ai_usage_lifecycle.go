package service

import "context"

// AIUsageLifecycle preserves admission, durable usage accounting, and recovery
// independently of the edition's financial policy. A community implementation
// records telemetry without financial reservations or settlement.
type AIUsageLifecycle interface {
	ResolveMeteringContext(MeteringRequest) (MeteringContext, error)
	Preflight(context.Context, PreflightRequest) (*MeteringContext, error)
	Checkpoint(context.Context, CompletionUsage) (*UsageResult, error)
	Reconcile(context.Context, CompletionUsage) (*UsageResult, error)
	Heartbeat(context.Context, MeteringContext) error
	SuspendReservation(context.Context, MeteringContext) error
	Fail(context.Context, string) error
	Release(context.Context, string, string) error
}

var _ AIUsageLifecycle = (*AIUsageService)(nil)
