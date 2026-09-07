package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHandoffCoverageFreezesBeforeLateDelivery(t *testing.T) {
	for _, status := range []string{"pending", "failed", "missing"} {
		t.Run(status, func(t *testing.T) {
			r, ctx := handoffTestRepository(t), context.Background()
			query := "UPDATE agent_run_messages SET delivery_status = ? WHERE id = 'first'"
			if status == "missing" {
				query = "DELETE FROM agent_run_messages WHERE id = 'first' AND ? = 'missing'"
			}
			if err := r.db.Exec(query, status).Error; err != nil {
				t.Fatal(err)
			}
			lease, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if lease.SourceThrough != 0 {
				t.Fatalf("lease skipped %s message: %+v", status, lease)
			}
			if err := r.Publish(ctx, *lease, 2, handoffContent("correction"), "model", "v1"); err == nil {
				t.Fatal("covered unresolved gap")
			}
			// Delivery/insertion races with the LLM call. Publication must use
			// the prefix frozen before generation, not the now-complete history.
			if status == "missing" {
				if err := r.db.Exec("INSERT INTO agent_run_messages VALUES ('first','ws','chat',1,'sent')").Error; err != nil {
					t.Fatal(err)
				}
			} else if err := r.db.Exec("UPDATE agent_run_messages SET delivery_status = 'sent' WHERE id = 'first'").Error; err != nil {
				t.Fatal(err)
			}
			if err := r.Publish(ctx, *lease, 2, handoffContent("correction"), "model", "v1"); err == nil {
				t.Fatal("late delivery expanded old source window")
			}
			forged := *lease
			forged.SourceThrough = 2
			if err := r.Publish(ctx, forged, 2, handoffContent("correction"), "model", "v1"); !errors.Is(err, ErrHandoffConflict) {
				t.Fatalf("source limit not persisted: %v", err)
			}
			state, err := r.Get(ctx, "ws", "chat")
			if err != nil {
				t.Fatal(err)
			}
			if state.CoveredSequence != 0 || state.Revision != 0 {
				t.Fatal("failed publication advanced coverage")
			}
			if err := r.Fail(ctx, *lease, "source_unavailable"); err != nil {
				t.Fatal(err)
			}
			next, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if next.SourceThrough != 2 {
				t.Fatalf("repaired prefix: %+v", next)
			}
			if err := r.Publish(ctx, *next, 2, handoffContent("first"), "model", "v1"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHandoffCoverageStopsAtMiddleGap(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	if err := r.db.Exec("UPDATE agent_run_messages SET delivery_status = 'pending' WHERE id = 'correction'").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("INSERT INTO agent_run_messages VALUES ('later','ws','chat',3,'sent')").Error; err != nil {
		t.Fatal(err)
	}
	lease, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if lease.SourceThrough != 1 {
		t.Fatalf("safe prefix: %+v", lease)
	}
	if err := r.Publish(ctx, *lease, 3, handoffContent("later"), "model", "v1"); err == nil {
		t.Fatal("skipped middle gap")
	}
	if err := r.Publish(ctx, *lease, 1, handoffContent("first"), "model", "v1"); err != nil {
		t.Fatal(err)
	}
	next, err := r.Acquire(ctx, "ws", "chat", "scope", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if next.CoveredSequence != 1 || next.SourceThrough != 1 {
		t.Fatalf("next generation skipped gap: %+v", next)
	}
}

func TestHandoffCoverageRejectsSourceRemovedDuringGeneration(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	lease, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("DELETE FROM agent_run_messages WHERE id = 'first'").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(ctx, *lease, 2, handoffContent("correction"), "model", "v1"); err == nil {
		t.Fatal("published across deleted source")
	}
}
