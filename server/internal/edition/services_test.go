package edition

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestCommunityAcceptsUnpricedModelsWithoutCommercialDependencies(t *testing.T) {
	services := Community(nil)
	if services.Entitlements != nil || services.WorkspaceLifecycle != nil || services.Seats != nil || services.Routes != nil || services.ConnectionPolicy != nil || services.RequireConfiguredProviders || services.BillingInsights != nil || services.BillingMetadata != nil || services.RunWorkers != nil {
		t.Fatal("community installed a commercial dependency")
	}
	ctx, err := services.Usage.Preflight(context.Background(), service.PreflightRequest{Metering: service.MeteringRequest{
		WorkspaceID: "workspace", IdempotencyKey: "execution", Provider: "openai", Model: "customer-private-model",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PolicyMode != "community" || ctx.PricingVersion != "" {
		t.Fatalf("unexpected policy: %+v", ctx)
	}
	select {
	case <-services.StartWorkers(context.Background()):
	default:
		t.Fatal("community started background workers")
	}
}

func TestEditionShutdownJoinsWorkers(t *testing.T) {
	started := make(chan struct{})
	exited := make(chan struct{})
	services := &Services{RunWorkers: func(ctx context.Context) { close(started); <-ctx.Done(); close(exited) }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := services.StartWorkers(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	select {
	case <-exited:
	default:
		t.Fatal("completion preceded worker exit")
	}
}
