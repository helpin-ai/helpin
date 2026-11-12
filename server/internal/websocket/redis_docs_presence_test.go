package websocket

import (
	"context"
	"testing"
	"time"
)

func TestRedisPresence_DocSnapshotOmitsExpiredViewers(t *testing.T) {
	p, mr := setupRedisPresence(t)
	ctx := context.Background()
	if _, err := p.SetDocViewing(ctx, "ws-1", "doc-1", "user-1", "conn-1"); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(docViewingTTL + time.Second)

	snapshot, err := p.GetDocSnapshot(ctx, "ws-1", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Viewers) != 0 {
		t.Fatalf("expired connections must not appear as viewers: %v", snapshot.Viewers)
	}
	members, err := p.rdb.SMembers(ctx, docViewingSetKey("ws-1", "doc-1")).Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("expired viewers must be removed from the index: %v", members)
	}
}

func TestRedisPresence_DocViewersKeepAnotherLiveTab(t *testing.T) {
	p, mr := setupRedisPresence(t)
	ctx := context.Background()
	for _, connID := range []string{"tab-1", "tab-2"} {
		if _, err := p.SetDocViewing(ctx, "ws-1", "doc-1", "user-1", connID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.SetDocViewing(ctx, "ws-1", "doc-1", "user-2", "tab-3"); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(docViewingTTL / 2)
	if err := p.RefreshDocViewing(ctx, "ws-1", "doc-1", "user-1", "tab-2"); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(docViewingTTL/2 + time.Second)

	viewers, err := p.GetDocViewers(ctx, "ws-1", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(viewers) != 1 || viewers[0] != "user-1" {
		t.Fatalf("only the user with a live tab should remain: %v", viewers)
	}
	cleared, err := p.ClearDocViewing(ctx, "ws-1", "doc-1", "user-1", "tab-2")
	if err != nil || !cleared {
		t.Fatalf("closing the final tab should remove the viewer: cleared=%v, err=%v", cleared, err)
	}
}

func TestRedisPresence_DocViewingRenewalDoesNotAnnounceAnotherViewer(t *testing.T) {
	p, mr := setupRedisPresence(t)
	ctx := context.Background()
	for i, wantChanged := range []bool{true, false, false} {
		changed, err := p.SetDocViewing(ctx, "ws-1", "doc-1", "user-1", "conn-1")
		if err != nil {
			t.Fatal(err)
		}
		if changed != wantChanged {
			t.Errorf("renewal %d: changed=%v, want %v", i, changed, wantChanged)
		}
		mr.FastForward(docViewingTTL / 2)
	}
	viewers, err := p.GetDocViewers(ctx, "ws-1", "doc-1")
	if err != nil || len(viewers) != 1 {
		t.Fatalf("renewed viewer should stay present: viewers=%v, err=%v", viewers, err)
	}
}
