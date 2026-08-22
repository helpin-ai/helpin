package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// fakePushDeviceRepo is an in-memory PushDeviceRepo used to unit test
// PushDeviceService without a real database.
type fakePushDeviceRepo struct {
	devices []model.PushDevice
}

func (f *fakePushDeviceRepo) UpsertByToken(ctx context.Context, device *model.PushDevice) error {
	for i := range f.devices {
		if f.devices[i].Token == device.Token {
			device.ID = f.devices[i].ID
			f.devices[i] = *device
			return nil
		}
	}
	device.ID = "generated-id"
	f.devices = append(f.devices, *device)
	return nil
}

func (f *fakePushDeviceRepo) DeleteByToken(ctx context.Context, userID, token string) error {
	filtered := f.devices[:0]
	for _, d := range f.devices {
		if d.UserID == userID && d.Token == token {
			continue
		}
		filtered = append(filtered, d)
	}
	f.devices = filtered
	return nil
}

func TestRegisterDevice_UpsertsByToken(t *testing.T) {
	repo := &fakePushDeviceRepo{}
	svc := NewPushDeviceService(repo)

	first, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-a", AppVersion: "0.1.0"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if first.ID == "" {
		t.Fatal("expected generated ID on first registration, got empty string")
	}

	// same token, different user → device moves to user-2, same row (ID preserved)
	dev, err := svc.Register(context.Background(), "user-2", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-a", AppVersion: "0.2.0"})
	if err != nil {
		t.Fatalf("Register (re-register): %v", err)
	}
	if dev.UserID != "user-2" {
		t.Fatalf("UserID = %q, want user-2", dev.UserID)
	}
	if dev.ID != first.ID {
		t.Fatalf("ID = %q, want preserved ID %q (re-registration should update the existing row, not create a new one)", dev.ID, first.ID)
	}
	if len(repo.devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1", len(repo.devices))
	}
}

func TestRegisterDevice_LowercasesPlatform(t *testing.T) {
	repo := &fakePushDeviceRepo{}
	svc := NewPushDeviceService(repo)

	dev, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "iOS", Token: "tok-e"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if dev.Platform != "ios" {
		t.Fatalf("Platform = %q, want lowercased ios", dev.Platform)
	}
}

func TestRegisterDevice_RejectsBadPlatform(t *testing.T) {
	svc := NewPushDeviceService(&fakePushDeviceRepo{})
	_, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "web", Token: "t"})
	if err == nil {
		t.Fatal("expected error for invalid platform, got nil")
	}
}

func TestRegisterDevice_RejectsEmptyToken(t *testing.T) {
	svc := NewPushDeviceService(&fakePushDeviceRepo{})
	_, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "ios", Token: "   "})
	if err == nil {
		t.Fatal("expected error for empty/whitespace token, got nil")
	}
}

func TestRegisterDevice_TrimsToken(t *testing.T) {
	repo := &fakePushDeviceRepo{}
	svc := NewPushDeviceService(repo)

	dev, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "android", Token: "  tok-b  "})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if dev.Token != "tok-b" {
		t.Fatalf("Token = %q, want trimmed tok-b", dev.Token)
	}
}

func TestRegisterDevice_StampsLastSeenAt(t *testing.T) {
	svc := NewPushDeviceService(&fakePushDeviceRepo{})
	dev, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-c"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if dev.LastSeenAt.IsZero() {
		t.Fatal("expected LastSeenAt to be stamped, got zero value")
	}
}

func TestUnregisterDevice_ScopesToOwner(t *testing.T) {
	repo := &fakePushDeviceRepo{}
	svc := NewPushDeviceService(repo)

	if _, err := svc.Register(context.Background(), "user-1", model.RegisterPushDeviceRequest{Platform: "ios", Token: "tok-d"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// A different user attempting to unregister the device should not remove it.
	if err := svc.Unregister(context.Background(), "user-2", "tok-d"); err != nil {
		t.Fatalf("Unregister (non-owner): %v", err)
	}
	if len(repo.devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1 (non-owner unregister should be a no-op)", len(repo.devices))
	}

	// The actual owner can unregister it.
	if err := svc.Unregister(context.Background(), "user-1", "tok-d"); err != nil {
		t.Fatalf("Unregister (owner): %v", err)
	}
	if len(repo.devices) != 0 {
		t.Fatalf("len(devices) = %d, want 0 after owner unregisters", len(repo.devices))
	}
}

func TestUnregisterDevice_RejectsEmptyToken(t *testing.T) {
	svc := NewPushDeviceService(&fakePushDeviceRepo{})
	if err := svc.Unregister(context.Background(), "user-1", "  "); err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}
