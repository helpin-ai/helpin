package websocket

import (
	"context"
	"testing"
)

func TestBuildPresenceSnapshotPayloadIncludesIdentity(t *testing.T) {
	hub := NewHub()
	handler := &Handler{hub: hub}
	handler.SetUserLookup(func(_ context.Context, userID string) (string, *string) {
		switch userID {
		case "user-1":
			avatar := "https://example.com/alice.png"
			return "Alice Agent", &avatar
		case "user-2":
			avatar := "https://example.com/bob.png"
			return "Bob Agent", &avatar
		default:
			return "", nil
		}
	})

	ctx := context.Background()
	if _, err := hub.Presence.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1"); err != nil {
		t.Fatalf("SetViewing: %v", err)
	}
	if err := hub.Presence.SetTyping(ctx, "ws-1", "conv-1", "user-2", "conn-2", "drafting"); err != nil {
		t.Fatalf("SetTyping: %v", err)
	}

	payload, err := handler.buildPresenceSnapshotPayload(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("buildPresenceSnapshotPayload: %v", err)
	}

	if payload.ConversationID != "conv-1" {
		t.Fatalf("conversation_id = %q, want conv-1", payload.ConversationID)
	}
	if len(payload.Viewers) != 1 {
		t.Fatalf("expected 1 viewer, got %d", len(payload.Viewers))
	}
	if payload.Viewers[0].UserID != "user-1" {
		t.Fatalf("viewer user_id = %q, want user-1", payload.Viewers[0].UserID)
	}
	if payload.Viewers[0].Name == nil || *payload.Viewers[0].Name != "Alice Agent" {
		t.Fatalf("viewer name = %v, want Alice Agent", payload.Viewers[0].Name)
	}
	if payload.Viewers[0].Avatar == nil || *payload.Viewers[0].Avatar != "https://example.com/alice.png" {
		t.Fatalf("viewer avatar = %v, want Alice avatar", payload.Viewers[0].Avatar)
	}

	typer, ok := payload.Typers["user-2"]
	if !ok {
		t.Fatal("expected typer for user-2")
	}
	if typer.Content != "drafting" {
		t.Fatalf("typer content = %q, want drafting", typer.Content)
	}
	if typer.Name == nil || *typer.Name != "Bob Agent" {
		t.Fatalf("typer name = %v, want Bob Agent", typer.Name)
	}
	if typer.Avatar == nil || *typer.Avatar != "https://example.com/bob.png" {
		t.Fatalf("typer avatar = %v, want Bob avatar", typer.Avatar)
	}
}

func TestBuildPresenceSnapshotPayloadWithoutUserLookup(t *testing.T) {
	hub := NewHub()
	handler := &Handler{hub: hub}

	ctx := context.Background()
	if _, err := hub.Presence.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1"); err != nil {
		t.Fatalf("SetViewing: %v", err)
	}

	payload, err := handler.buildPresenceSnapshotPayload(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("buildPresenceSnapshotPayload: %v", err)
	}

	if len(payload.Viewers) != 1 {
		t.Fatalf("expected 1 viewer, got %d", len(payload.Viewers))
	}
	if payload.Viewers[0].Name != nil {
		t.Fatalf("expected nil name without lookup, got %v", payload.Viewers[0].Name)
	}
	if payload.Viewers[0].Avatar != nil {
		t.Fatalf("expected nil avatar without lookup, got %v", payload.Viewers[0].Avatar)
	}
}
