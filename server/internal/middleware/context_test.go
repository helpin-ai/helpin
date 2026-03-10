package middleware

import (
	"context"
	"testing"
)

func TestWithUserID_GetUserID_Roundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "user-123")

	got := GetUserID(ctx)
	if got != "user-123" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-123")
	}
}

func TestGetUserID_EmptyContext(t *testing.T) {
	ctx := context.Background()

	got := GetUserID(ctx)
	if got != "" {
		t.Errorf("GetUserID() on empty context = %q, want %q", got, "")
	}
}

func TestWithUserEmail_GetUserEmail_Roundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserEmail(ctx, "alice@example.com")

	got := GetUserEmail(ctx)
	if got != "alice@example.com" {
		t.Errorf("GetUserEmail() = %q, want %q", got, "alice@example.com")
	}
}

func TestGetUserEmail_EmptyContext(t *testing.T) {
	ctx := context.Background()

	got := GetUserEmail(ctx)
	if got != "" {
		t.Errorf("GetUserEmail() on empty context = %q, want %q", got, "")
	}
}

func TestWithWorkspaceID_GetWorkspaceID_Roundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithWorkspaceID(ctx, "ws-456")

	got := GetWorkspaceID(ctx)
	if got != "ws-456" {
		t.Errorf("GetWorkspaceID() = %q, want %q", got, "ws-456")
	}
}

func TestGetWorkspaceID_EmptyContext(t *testing.T) {
	ctx := context.Background()

	got := GetWorkspaceID(ctx)
	if got != "" {
		t.Errorf("GetWorkspaceID() on empty context = %q, want %q", got, "")
	}
}

func TestMultipleValuesInSameContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "user-789")
	ctx = WithUserEmail(ctx, "bob@example.com")
	ctx = WithWorkspaceID(ctx, "ws-012")

	if got := GetUserID(ctx); got != "user-789" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-789")
	}
	if got := GetUserEmail(ctx); got != "bob@example.com" {
		t.Errorf("GetUserEmail() = %q, want %q", got, "bob@example.com")
	}
	if got := GetWorkspaceID(ctx); got != "ws-012" {
		t.Errorf("GetWorkspaceID() = %q, want %q", got, "ws-012")
	}
}
