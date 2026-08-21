package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// fakeFCMClient is an in-memory FCMClient used to unit test PushSenderService
// without a real Firebase project.
type fakeFCMClient struct {
	sent     []fakeSentPush
	failWith map[string]error // token -> error Send should return for that token
}

type fakeSentPush struct {
	Token string
	Notif PushNotification
}

func (f *fakeFCMClient) Send(ctx context.Context, token string, n PushNotification) error {
	if f.failWith != nil {
		if err, ok := f.failWith[token]; ok {
			return err
		}
	}
	f.sent = append(f.sent, fakeSentPush{Token: token, Notif: n})
	return nil
}

// fakePushSenderRepo is an in-memory PushSenderRepo used to unit test
// PushSenderService without a real database.
type fakePushSenderRepo struct {
	devices []model.PushDevice
	deleted []string
}

func (f *fakePushSenderRepo) ListByUserIDs(ctx context.Context, userIDs []string) ([]model.PushDevice, error) {
	want := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		want[id] = true
	}
	var out []model.PushDevice
	for _, d := range f.devices {
		if want[d.UserID] {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakePushSenderRepo) DeleteByTokenAny(ctx context.Context, token string) error {
	f.deleted = append(f.deleted, token)
	filtered := f.devices[:0]
	for _, d := range f.devices {
		if d.Token != token {
			filtered = append(filtered, d)
		}
	}
	f.devices = filtered
	return nil
}

func TestPushSenderNotifyUsers_SendsOnePerDeviceWithDataIntact(t *testing.T) {
	repo := &fakePushSenderRepo{devices: []model.PushDevice{
		{UserID: "user-1", Token: "tok-1", Platform: "ios"},
		{UserID: "user-1", Token: "tok-2", Platform: "android"},
		{UserID: "user-2", Token: "tok-3", Platform: "ios"},
	}}
	client := &fakeFCMClient{}
	svc := NewPushSenderService(repo, client)

	n := PushNotification{
		Title: "Ada Lovelace",
		Body:  "Thanks for the update!",
		Data: map[string]string{
			"workspace_slug":  "acme",
			"conversation_id": "conv-1",
			"deep_link":       "helpin://w/acme/support/conv-1",
		},
	}

	svc.NotifyUsers(context.Background(), []string{"user-1", "user-2"}, n)

	if len(client.sent) != 3 {
		t.Fatalf("len(sent) = %d, want 3", len(client.sent))
	}
	seen := map[string]bool{}
	for _, s := range client.sent {
		seen[s.Token] = true
		if s.Notif.Title != n.Title || s.Notif.Body != n.Body {
			t.Fatalf("sent notif = %+v, want title/body %q/%q", s.Notif, n.Title, n.Body)
		}
		if s.Notif.Data["deep_link"] != n.Data["deep_link"] || s.Notif.Data["workspace_slug"] != n.Data["workspace_slug"] || s.Notif.Data["conversation_id"] != n.Data["conversation_id"] {
			t.Fatalf("sent data = %+v, want data intact %+v", s.Notif.Data, n.Data)
		}
	}
	for _, tok := range []string{"tok-1", "tok-2", "tok-3"} {
		if !seen[tok] {
			t.Fatalf("expected a send to token %q", tok)
		}
	}
}

func TestPushSenderNotifyUsers_UnregisteredErrorDeletesOnlyThatDevice(t *testing.T) {
	repo := &fakePushSenderRepo{devices: []model.PushDevice{
		{UserID: "user-1", Token: "tok-dead", Platform: "ios"},
		{UserID: "user-1", Token: "tok-alive", Platform: "android"},
	}}
	client := &fakeFCMClient{failWith: map[string]error{
		// Mirrors what firebaseFCMClient does: wrap the provider error so
		// errors.Is(err, ErrUnregisteredDevice) succeeds.
		"tok-dead": fmt.Errorf("fcm send: %w: %w", ErrUnregisteredDevice, errors.New("registration-token-not-registered")),
	}}

	svc := NewPushSenderService(repo, client)
	svc.NotifyUsers(context.Background(), []string{"user-1"}, PushNotification{Title: "t", Body: "b"})

	if len(client.sent) != 1 || client.sent[0].Token != "tok-alive" {
		t.Fatalf("sent = %+v, want exactly one send to tok-alive", client.sent)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "tok-dead" {
		t.Fatalf("deleted = %v, want exactly [tok-dead]", repo.deleted)
	}
	if len(repo.devices) != 1 || repo.devices[0].Token != "tok-alive" {
		t.Fatalf("remaining devices = %+v, want only tok-alive left", repo.devices)
	}
}

func TestPushSenderNotifyUsers_OtherErrorsContinueFanOutWithoutDeleting(t *testing.T) {
	repo := &fakePushSenderRepo{devices: []model.PushDevice{
		{UserID: "user-1", Token: "tok-flaky", Platform: "ios"},
		{UserID: "user-1", Token: "tok-ok", Platform: "android"},
	}}
	client := &fakeFCMClient{failWith: map[string]error{
		"tok-flaky": errors.New("transient network error"),
	}}
	svc := NewPushSenderService(repo, client)

	svc.NotifyUsers(context.Background(), []string{"user-1"}, PushNotification{Title: "t", Body: "b"})

	if len(client.sent) != 1 || client.sent[0].Token != "tok-ok" {
		t.Fatalf("sent = %+v, want exactly one send to tok-ok", client.sent)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("deleted = %v, want no deletions for a non-unregistered error", repo.deleted)
	}
	if len(repo.devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2 (no device removed)", len(repo.devices))
	}
}

func TestPushSenderNotifyUsers_NilClientIsNoopNoPanic(t *testing.T) {
	repo := &fakePushSenderRepo{devices: []model.PushDevice{{UserID: "user-1", Token: "tok-1"}}}
	svc := NewPushSenderService(repo, nil)

	svc.NotifyUsers(context.Background(), []string{"user-1"}, PushNotification{Title: "t", Body: "b"})

	if len(repo.deleted) != 0 {
		t.Fatalf("deleted = %v, want none when client is nil", repo.deleted)
	}
}

func TestPushSenderNotifyUsers_EmptyUserIDsIsNoop(t *testing.T) {
	repo := &fakePushSenderRepo{devices: []model.PushDevice{{UserID: "user-1", Token: "tok-1"}}}
	client := &fakeFCMClient{}
	svc := NewPushSenderService(repo, client)

	svc.NotifyUsers(context.Background(), nil, PushNotification{Title: "t", Body: "b"})

	if len(client.sent) != 0 {
		t.Fatalf("sent = %+v, want none for empty userIDs", client.sent)
	}
}

func TestPushSenderNotifyUsers_NilServiceIsNoopNoPanic(t *testing.T) {
	var svc *PushSenderService
	svc.NotifyUsers(context.Background(), []string{"user-1"}, PushNotification{Title: "t", Body: "b"})
}
