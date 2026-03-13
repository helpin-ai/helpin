package temporalapp

import (
	"context"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
)

// SignalDetectionResult contains the detected signals.
type SignalDetectionResult struct {
	SignalsDetected int
	SignalIDs       []string
}

// SignalDetectionWorkflow analyzes source data for buyer signals.
func SignalDetectionWorkflow(ctx workflow.Context, payloads []model.SignalSourcePayload) (*SignalDetectionResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result SignalDetectionResult
	if err := workflow.ExecuteActivity(ctx, "SignalDetectionActivities.ExtractSignalsActivity", payloads).Get(ctx, &result); err != nil {
		return nil, err
	}

	if result.SignalsDetected > 0 {
		_ = workflow.ExecuteActivity(ctx, "SignalDetectionActivities.NotifySignalsActivity", result).Get(ctx, nil)
	}

	return &result, nil
}

// signalDetector is an interface to break the import cycle with the service package.
type signalDetector interface {
	DetectSignals(ctx context.Context, payloads []model.SignalSourcePayload) ([]model.CRMBuyerSignal, error)
}

// SignalDetectionActivities contains signal detection activities.
type SignalDetectionActivities struct {
	detectionSvc signalDetector
	wsPublisher  *ws.Publisher
}

// NewSignalDetectionActivities creates signal detection activities.
// The detectionSvc parameter should be *service.SignalDetectionService.
func NewSignalDetectionActivities(detectionSvc signalDetector, wsPublisher *ws.Publisher) *SignalDetectionActivities {
	return &SignalDetectionActivities{
		detectionSvc: detectionSvc,
		wsPublisher:  wsPublisher,
	}
}

// ExtractSignalsActivity calls the LLM to detect signals.
func (a *SignalDetectionActivities) ExtractSignalsActivity(ctx context.Context, payloads []model.SignalSourcePayload) (*SignalDetectionResult, error) {
	signals, err := a.detectionSvc.DetectSignals(ctx, payloads)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, s := range signals {
		ids = append(ids, s.ID)
	}

	return &SignalDetectionResult{
		SignalsDetected: len(signals),
		SignalIDs:       ids,
	}, nil
}

// NotifySignalsActivity sends WebSocket events for detected signals.
func (a *SignalDetectionActivities) NotifySignalsActivity(_ context.Context, result SignalDetectionResult) error {
	for _, id := range result.SignalIDs {
		a.wsPublisher.Publish(ws.Event{
			Action:   "created",
			Entity:   "crm_buyer_signal",
			EntityID: id,
		})
	}
	slog.Info("signal notifications sent", "count", result.SignalsDetected)
	return nil
}
