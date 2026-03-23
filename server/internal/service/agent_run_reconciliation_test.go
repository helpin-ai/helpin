package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestShouldFailStuckPostRun(t *testing.T) {
	now := time.Now()
	heartbeat := now.Add(-(stuckPostRunThreshold + time.Second))
	stage := "opencode_finished"

	run := &model.AgentRun{
		RuntimeKind:     "opencode",
		Status:          "running",
		ExecutionStage:  &stage,
		LastHeartbeatAt: &heartbeat,
	}

	if !shouldFailStuckPostRun(run, now) {
		t.Fatalf("expected post-run reconciliation to fail a stale opencode run")
	}
}

func TestShouldNotFailActiveOrNonPostRunStates(t *testing.T) {
	now := time.Now()
	heartbeat := now.Add(-time.Minute)
	stage := "opencode_running"

	run := &model.AgentRun{
		RuntimeKind:     "opencode",
		Status:          "running",
		ExecutionStage:  &stage,
		LastHeartbeatAt: &heartbeat,
	}

	if shouldFailStuckPostRun(run, now) {
		t.Fatalf("did not expect an active execution stage to be reconciled as failed")
	}
}

func TestShouldInspectQueuedRunWhenStale(t *testing.T) {
	now := time.Now()
	run := &model.AgentRun{
		Status:    "queued",
		CreatedAt: now.Add(-(staleQueuedRunThreshold + time.Second)),
	}

	if !shouldInspectQueuedRun(run, now) {
		t.Fatalf("expected a stale queued run to be inspected")
	}
}

func TestShouldNotInspectFreshQueuedRun(t *testing.T) {
	now := time.Now()
	run := &model.AgentRun{
		Status:    "queued",
		CreatedAt: now.Add(-(staleQueuedRunThreshold - time.Second)),
	}

	if shouldInspectQueuedRun(run, now) {
		t.Fatalf("did not expect a fresh queued run to be inspected")
	}
}
