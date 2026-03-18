package temporalapp

import (
	"encoding/json"
	"time"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	QueueFlowOrchestrator = "flow-orchestrator"
	WorkflowSignalFlowRun = "FlowRunSignal"

	FlowSignalTypeNodeAction = "node_action"
	FlowSignalTypeChildState = "child_state"
	FlowSignalTypeRetry      = "retry"
	FlowSignalTypeCancel     = "cancel"

	FlowChildTypeAgentRun        = "agent_run"
	FlowChildTypePlanningSession = "planning_session"

	flowChangeIDSignalWaitDeadline = "flow-signal-wait-deadline-v1"
	flowChangeIDReconcileInterval  = "flow-reconcile-interval-v1"
	flowChangeIDGenericPlatform    = "flow-generic-platform-v1"
	legacyFlowReconcileInterval    = 5 * time.Second

	FlowReconcileInterval = 30 * time.Second
	FlowMaxLifetime       = 30 * 24 * time.Hour
)

type FlowRunWorkflowInput struct {
	FlowRunID string
	ActorID   string
}

type FlowRunSignal struct {
	Type        string          `json:"type"`
	NodeRunID   string          `json:"node_run_id,omitempty"`
	Action      string          `json:"action,omitempty"`
	ActorID     string          `json:"actor_id,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	ChildType   string          `json:"child_type,omitempty"`
	ChildID     string          `json:"child_id,omitempty"`
	ChildStatus string          `json:"child_status,omitempty"`
}

type FlowRuntimeState struct {
	RunID            string `json:"run_id"`
	Status           string `json:"status"`
	CurrentNodeID    string `json:"current_node_id,omitempty"`
	CurrentNodeRunID string `json:"current_node_run_id,omitempty"`
}

type FlowProgressResult struct {
	State        FlowRuntimeState `json:"state"`
	WaitingChild bool             `json:"waiting_child"`
}

func WorkflowIDForFlowRun(flowRunID string) string {
	return "flow-run-" + flowRunID
}

func FlowRunWorkflow(ctx workflow.Context, input FlowRunWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	state := FlowRuntimeState{RunID: input.FlowRunID}

	// Version the timeout/timer behavior so in-flight workflows created before
	// these changes can continue replaying deterministically.
	signalWaitDeadlineVersion := workflow.GetVersion(ctx, flowChangeIDSignalWaitDeadline, workflow.DefaultVersion, 1)
	reconcileIntervalVersion := workflow.GetVersion(ctx, flowChangeIDReconcileInterval, workflow.DefaultVersion, 1)
	genericPlatformVersion := workflow.GetVersion(ctx, flowChangeIDGenericPlatform, workflow.DefaultVersion, 1)

	deadlineEnabled := signalWaitDeadlineVersion != workflow.DefaultVersion
	var deadline time.Time
	if deadlineEnabled {
		deadline = workflow.Now(ctx).Add(FlowMaxLifetime)
	}

	_ = workflow.SetQueryHandler(ctx, "current_state", func() (FlowRuntimeState, error) {
		return state, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	bootstrapActivity := "FlowRuntimeActivities.BootstrapEpicPlanningRunActivity"
	if genericPlatformVersion != workflow.DefaultVersion {
		bootstrapActivity = "FlowRuntimeActivities.BootstrapRunActivity"
	}
	if err := workflow.ExecuteActivity(ctx, bootstrapActivity, input.FlowRunID, input.ActorID).Get(ctx, nil); err != nil {
		logger.Error("bootstrap flow run failed", "flow_run_id", input.FlowRunID, "error", err)
		if loadErr := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); loadErr != nil {
			return err
		}
		if state.Status == model.FlowStatusFailed || state.Status == model.FlowStatusCancelled || state.Status == model.FlowStatusCompleted {
			return nil
		}
	}
	if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
		return err
	}

	signalCh := workflow.GetSignalChannel(ctx, WorkflowSignalFlowRun)

	for {
		switch state.Status {
		case model.FlowStatusCompleted, model.FlowStatusCancelled:
			return nil
		case model.FlowStatusAwaitingInput:
			sig, gotSignal, timedOut := waitForSignalState(ctx, signalCh, deadlineEnabled, deadline)
			if timedOut {
				if !cancelFlowForReason(ctx, input.FlowRunID, "system:stale-timeout", "stale_timeout", logger) {
					continue
				}
				return nil
			}
			if !gotSignal {
				return nil
			}
			switch sig.Type {
			case FlowSignalTypeCancel:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.CancelRunActivity", input.FlowRunID, sig.ActorID, fallbackString(sig.Reason, "cancelled_by_user")).Get(ctx, nil); err != nil {
					logger.Error("cancel flow activity failed", "flow_run_id", input.FlowRunID, "error", err)
					continue
				}
			case FlowSignalTypeNodeAction:
				if sig.Action != model.FlowActionFinalize {
					continue
				}
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.FinalizeInteractiveNodeActivity", input.FlowRunID, sig.ActorID).Get(ctx, nil); err != nil {
					logger.Error("finalize interactive node failed", "flow_run_id", input.FlowRunID, "error", err)
					continue
				}
			case FlowSignalTypeChildState:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleChildStateActivity", input.FlowRunID, sig.NodeRunID, sig.ChildType, sig.ChildID, sig.ChildStatus).Get(ctx, nil); err != nil {
					logger.Error("handle child state failed", "flow_run_id", input.FlowRunID, "child_id", sig.ChildID, "error", err)
					continue
				}
			}
			if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
				logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
			}
		case model.FlowStatusAwaitingApproval:
			sig, gotSignal, timedOut := waitForSignalState(ctx, signalCh, deadlineEnabled, deadline)
			if timedOut {
				if !cancelFlowForReason(ctx, input.FlowRunID, "system:stale-timeout", "stale_timeout", logger) {
					continue
				}
				return nil
			}
			if !gotSignal {
				return nil
			}
			switch sig.Type {
			case FlowSignalTypeCancel:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.CancelRunActivity", input.FlowRunID, sig.ActorID, fallbackString(sig.Reason, "cancelled_by_user")).Get(ctx, nil); err != nil {
					logger.Error("cancel flow activity failed", "flow_run_id", input.FlowRunID, "error", err)
					continue
				}
			case FlowSignalTypeNodeAction:
				switch sig.Action {
				case model.FlowActionApprove:
					if genericPlatformVersion != workflow.DefaultVersion {
						if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleApprovalActionActivity", input.FlowRunID, sig.ActorID, sig.Action, sig.Payload).Get(ctx, nil); err != nil {
							logger.Error("generic approval activity failed", "flow_run_id", input.FlowRunID, "error", err)
							continue
						}
					} else if state.CurrentNodeID == model.FlowNodeSpecApproval {
						if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.ApproveSpecNodeActivity", input.FlowRunID, sig.ActorID, sig.Payload).Get(ctx, nil); err != nil {
							logger.Error("approve spec activity failed", "flow_run_id", input.FlowRunID, "error", err)
							continue
						}
					} else if state.CurrentNodeID == model.FlowNodePlanApproval {
						if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.ApprovePlanNodeActivity", input.FlowRunID, sig.ActorID, sig.Payload).Get(ctx, nil); err != nil {
							logger.Error("approve plan activity failed", "flow_run_id", input.FlowRunID, "error", err)
							continue
						}
					}
				case model.FlowActionRequestChanges:
					if genericPlatformVersion == workflow.DefaultVersion {
						continue
					}
					if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleApprovalActionActivity", input.FlowRunID, sig.ActorID, sig.Action, sig.Payload).Get(ctx, nil); err != nil {
						logger.Error("request changes activity failed", "flow_run_id", input.FlowRunID, "error", err)
						continue
					}
				case model.FlowActionReject:
					activityName := "FlowRuntimeActivities.RejectApprovalNodeActivity"
					args := []any{input.FlowRunID, sig.ActorID}
					if genericPlatformVersion != workflow.DefaultVersion {
						activityName = "FlowRuntimeActivities.HandleApprovalActionActivity"
						args = []any{input.FlowRunID, sig.ActorID, sig.Action, sig.Payload}
					}
					if err := workflow.ExecuteActivity(ctx, activityName, args...).Get(ctx, nil); err != nil {
						logger.Error("reject approval activity failed", "flow_run_id", input.FlowRunID, "error", err)
						continue
					}
				}
			case FlowSignalTypeRetry:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.RetryNodeActivity", input.FlowRunID, sig.NodeRunID, sig.ActorID).Get(ctx, nil); err != nil {
					logger.Error("retry node activity failed", "flow_run_id", input.FlowRunID, "node_run_id", sig.NodeRunID, "error", err)
					continue
				}
			case FlowSignalTypeChildState:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleChildStateActivity", input.FlowRunID, sig.NodeRunID, sig.ChildType, sig.ChildID, sig.ChildStatus).Get(ctx, nil); err != nil {
					logger.Error("handle child state failed", "flow_run_id", input.FlowRunID, "child_id", sig.ChildID, "error", err)
					continue
				}
			}
			if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
				logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
			}
		case model.FlowStatusRunning:
			if genericPlatformVersion == workflow.DefaultVersion && state.CurrentNodeID != model.FlowNodeStoryPlanning {
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
					logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
					workflow.Sleep(ctx, 10*time.Second)
				}
				continue
			}

			// Child executions signal the parent flow on completion. This timer is only
			// a reconciliation fallback in case a child signal is missed.
			reconcileAfter := nextReconcileDelay(ctx, deadlineEnabled, deadline, reconcileIntervalVersion)
			timer := workflow.NewTimer(ctx, reconcileAfter)
			var sig FlowRunSignal
			gotSignal := false
			selector := workflow.NewSelector(ctx)
			selector.AddFuture(timer, func(workflow.Future) {})
			selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
				gotSignal = more
				if more {
					c.Receive(ctx, &sig)
				}
			})
			selector.Select(ctx)

			if gotSignal {
				switch sig.Type {
				case FlowSignalTypeCancel:
					if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.CancelRunActivity", input.FlowRunID, sig.ActorID, fallbackString(sig.Reason, "cancelled_by_user")).Get(ctx, nil); err != nil {
						logger.Error("cancel flow activity failed", "flow_run_id", input.FlowRunID, "error", err)
						continue
					}
				case FlowSignalTypeRetry:
					if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.RetryNodeActivity", input.FlowRunID, sig.NodeRunID, sig.ActorID).Get(ctx, nil); err != nil {
						logger.Error("retry node activity failed", "flow_run_id", input.FlowRunID, "node_run_id", sig.NodeRunID, "error", err)
						continue
					}
				case FlowSignalTypeChildState:
					if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleChildStateActivity", input.FlowRunID, sig.NodeRunID, sig.ChildType, sig.ChildID, sig.ChildStatus).Get(ctx, nil); err != nil {
						logger.Error("handle child state failed", "flow_run_id", input.FlowRunID, "child_id", sig.ChildID, "error", err)
						continue
					}
				}
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
					logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
				}
				continue
			}
			if deadlineEnabled && (workflow.Now(ctx).Add(reconcileAfter).After(deadline) || workflow.Now(ctx).Add(reconcileAfter).Equal(deadline)) {
				if !cancelFlowForReason(ctx, input.FlowRunID, "system:stale-timeout", "stale_timeout", logger) {
					continue
				}
				return nil
			}

			var progress FlowProgressResult
			if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.ProgressRunStateActivity", input.FlowRunID).Get(ctx, &progress); err != nil {
				logger.Error("progress flow state failed", "flow_run_id", input.FlowRunID, "error", err)
				continue
			}
			state = progress.State
		case model.FlowStatusFailed:
			sig, gotSignal, timedOut := waitForSignalState(ctx, signalCh, deadlineEnabled, deadline)
			if timedOut {
				if !cancelFlowForReason(ctx, input.FlowRunID, "system:stale-timeout", "stale_timeout", logger) {
					continue
				}
				return nil
			}
			if !gotSignal {
				return nil
			}
			switch sig.Type {
			case FlowSignalTypeCancel:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.CancelRunActivity", input.FlowRunID, sig.ActorID, fallbackString(sig.Reason, "cancelled_by_user")).Get(ctx, nil); err != nil {
					logger.Error("cancel flow activity failed", "flow_run_id", input.FlowRunID, "error", err)
					continue
				}
			case FlowSignalTypeRetry:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.RetryNodeActivity", input.FlowRunID, sig.NodeRunID, sig.ActorID).Get(ctx, nil); err != nil {
					logger.Error("retry node failed", "flow_run_id", input.FlowRunID, "node_run_id", sig.NodeRunID, "error", err)
				}
			case FlowSignalTypeChildState:
				if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.HandleChildStateActivity", input.FlowRunID, sig.NodeRunID, sig.ChildType, sig.ChildID, sig.ChildStatus).Get(ctx, nil); err != nil {
					logger.Error("handle child state failed", "flow_run_id", input.FlowRunID, "child_id", sig.ChildID, "error", err)
				}
			}
			if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
				logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
			}
		default:
			if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.LoadRunStateActivity", input.FlowRunID).Get(ctx, &state); err != nil {
				logger.Error("load flow state failed", "flow_run_id", input.FlowRunID, "error", err)
				workflow.Sleep(ctx, 10*time.Second)
			}
		}
	}
}

func waitForSignalState(ctx workflow.Context, ch workflow.ReceiveChannel, deadlineEnabled bool, deadline time.Time) (FlowRunSignal, bool, bool) {
	if !deadlineEnabled {
		return waitForSignal(ctx, ch)
	}
	return waitForSignalOrDeadline(ctx, ch, deadline)
}

func waitForSignal(ctx workflow.Context, ch workflow.ReceiveChannel) (FlowRunSignal, bool, bool) {
	var sig FlowRunSignal
	more := false
	ch.Receive(ctx, &sig)
	more = true
	return sig, more, false
}

func waitForSignalOrDeadline(ctx workflow.Context, ch workflow.ReceiveChannel, deadline time.Time) (FlowRunSignal, bool, bool) {
	var sig FlowRunSignal
	more := false
	timedOut := false
	remaining := deadline.Sub(workflow.Now(ctx))
	if remaining <= 0 {
		return sig, false, true
	}
	timer := workflow.NewTimer(ctx, remaining)
	selector := workflow.NewSelector(ctx)
	selector.AddFuture(timer, func(workflow.Future) {
		timedOut = true
	})
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, ok bool) {
		more = ok
		if ok {
			c.Receive(ctx, &sig)
		}
	})
	selector.Select(ctx)
	return sig, more, timedOut
}

func nextReconcileDelay(ctx workflow.Context, deadlineEnabled bool, deadline time.Time, reconcileIntervalVersion workflow.Version) time.Duration {
	interval := legacyFlowReconcileInterval
	if reconcileIntervalVersion != workflow.DefaultVersion {
		interval = FlowReconcileInterval
	}
	if !deadlineEnabled {
		return interval
	}
	remaining := deadline.Sub(workflow.Now(ctx))
	if remaining <= 0 {
		return 0
	}
	if remaining < interval {
		return remaining
	}
	return interval
}

func cancelFlowForReason(ctx workflow.Context, flowRunID, actorID, reason string, logger log.Logger) bool {
	if err := workflow.ExecuteActivity(ctx, "FlowRuntimeActivities.CancelRunActivity", flowRunID, actorID, reason).Get(ctx, nil); err != nil {
		logger.Error("cancel flow for reason failed", "flow_run_id", flowRunID, "reason", reason, "error", err)
		return false
	}
	return true
}

func fallbackString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
