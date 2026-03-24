package temporalapp

import "testing"

func TestHealthIncludesSharedQueues(t *testing.T) {
	engine := &RunEngine{namespace: "test-namespace"}

	health := engine.Health()

	if health.Namespace != "test-namespace" {
		t.Fatalf("expected namespace test-namespace, got %q", health.Namespace)
	}
	if health.TemporalConfigured {
		t.Fatal("expected TemporalConfigured to be false when client is nil")
	}
	if len(health.Queues) != len(SharedQueues()) {
		t.Fatalf("expected %d queues, got %d", len(SharedQueues()), len(health.Queues))
	}
	if health.Queues[0].Name != QueueAgentNativeInteractive {
		t.Fatalf("expected first queue %q, got %q", QueueAgentNativeInteractive, health.Queues[0].Name)
	}
	if health.Queues[0].Concurrency != 8 {
		t.Fatalf("expected native interactive concurrency 8, got %d", health.Queues[0].Concurrency)
	}
}
