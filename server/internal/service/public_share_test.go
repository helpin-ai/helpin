package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakePublicShareStore struct {
	share *model.PublicShare
}

func TestSanitizePublicJSONRedactsCredentialFields(t *testing.T) {
	raw := json.RawMessage(`{"command":"deploy","api_key":"secret-value","nested":{"authorization":"Bearer private-token"}}`)
	got := string(sanitizePublicJSON(raw))
	if got == "" || got == string(raw) || publicShareContainsAny(got, "secret-value", "private-token") {
		t.Fatalf("sanitized json = %s", got)
	}
}

func publicShareContainsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func (s *fakePublicShareStore) CreateActive(_ context.Context, workspaceID, resourceType, resourceID, actorID string) (*model.PublicShare, error) {
	if s.share == nil {
		s.share = &model.PublicShare{WorkspaceID: workspaceID, ResourceType: resourceType, ResourceID: resourceID, Token: "public-token", CreatedBy: actorID}
	}
	return s.share, nil
}

func (s *fakePublicShareStore) GetActiveByToken(_ context.Context, token string) (*model.PublicShare, error) {
	if s.share == nil || s.share.Token != token || s.share.RevokedAt != nil {
		return nil, nil
	}
	return s.share, nil
}
func (s *fakePublicShareStore) GetActiveByResource(_ context.Context, _, _, _ string) (*model.PublicShare, error) {
	return s.share, nil
}

func (s *fakePublicShareStore) RevokeActive(_ context.Context, _, _, _, _ string) error {
	s.share = nil
	return nil
}

type fakePublicShareSource struct {
	allow bool
	chat  model.PublicSharedDockChat
}

func (s fakePublicShareSource) CanAccessDockChat(context.Context, string, string, string) (bool, error) {
	return s.allow, nil
}

func (s fakePublicShareSource) CanAccessAgentRun(context.Context, string, string, string) (bool, error) {
	return s.allow, nil
}

func (s fakePublicShareSource) PublicDockChat(context.Context, string, string) (*model.PublicSharedDockChat, error) {
	return &s.chat, nil
}

func (s fakePublicShareSource) PublicAgentRun(context.Context, string, string) (*model.PublicSharedAgentRun, error) {
	return &model.PublicSharedAgentRun{Title: "Live run"}, nil
}

func TestPublicShareServiceRequiresAccessAndReturnsLiveResource(t *testing.T) {
	store := &fakePublicShareStore{}
	denied := NewPublicShareService(store, fakePublicShareSource{allow: false}, "https://app.helpin.ai")
	if _, err := denied.Create(context.Background(), "ws-1", "user-1", model.PublicShareResourceDockChat, "chat-1"); !errors.Is(err, ErrPublicShareNotFound) {
		t.Fatalf("denied create error = %v", err)
	}

	source := fakePublicShareSource{allow: true, chat: model.PublicSharedDockChat{Title: "First title"}}
	svc := NewPublicShareService(store, source, "https://app.helpin.ai")
	link, err := svc.Create(context.Background(), "ws-1", "user-1", model.PublicShareResourceDockChat, "chat-1")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if link.URL != "https://app.helpin.ai/shared/public-token" {
		t.Fatalf("share url = %q", link.URL)
	}
	current, err := svc.GetLink(context.Background(), "ws-1", "user-2", model.PublicShareResourceDockChat, "chat-1")
	if err != nil || current == nil || current.Token != "public-token" {
		t.Fatalf("current link = %#v, %v", current, err)
	}
	resource, err := svc.GetPublic(context.Background(), "public-token")
	if err != nil || resource.DockChat == nil || resource.DockChat.Title != "First title" {
		t.Fatalf("public resource = %#v, %v", resource, err)
	}
	if err := svc.Revoke(context.Background(), "ws-1", "user-2", model.PublicShareResourceDockChat, "chat-1"); err != nil {
		t.Fatalf("teammate revoke: %v", err)
	}
	if _, err := svc.GetPublic(context.Background(), "public-token"); !errors.Is(err, ErrPublicShareNotFound) {
		t.Fatalf("revoked lookup error = %v", err)
	}
}
