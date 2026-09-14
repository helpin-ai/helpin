package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type connectionMembers struct{}

func (connectionMembers) GetMembership(_ context.Context, workspace, user string) (*authorization.MemberInfo, error) {
	if workspace != "workspace" {
		return nil, nil
	}
	role := "member"
	if user == "owner" {
		role = "owner"
	}
	return &authorization.MemberInfo{ID: user, Status: "active", Role: role}, nil
}
func (connectionMembers) GetTeamMemberships(context.Context, string) ([]authorization.TeamRole, error) {
	return nil, nil
}

func TestSharedAIConnectionAccessAndOwnership(t *testing.T) {
	s, db := setupAIConnectionTest(t)
	s.SetAuthorizationService(authorization.NewAuthzService(db, connectionMembers{}, nil))
	ctx := context.Background()
	if err := db.Exec(`CREATE TABLE workspaces (id TEXT, status TEXT); INSERT INTO workspaces VALUES ('workspace','active')`).Error; err != nil {
		t.Fatal(err)
	}
	req := model.CreateAIConnectionRequest{Name: "Team", Scope: "workspace", Provider: "openai", APIKey: "shared-key"}
	if _, err := s.Create(ctx, "workspace", "teammate", req); err == nil {
		t.Fatal("member created shared connection without settings permission")
	}
	login, err := s.Create(ctx, "workspace", "owner", req)
	if err != nil {
		t.Fatal(err)
	}
	id := login.Connection.ID
	if login.Connection.UserID != nil || login.Connection.Scope != "workspace" {
		t.Fatal("shared connection still owned by its creator")
	}
	list, err := s.List(ctx, "workspace", "teammate")
	if err != nil || len(list) != 1 {
		t.Fatalf("member cannot list shared connection: %v", err)
	}
	_, cred, err := s.Credential(ctx, "workspace", "teammate", id, false)
	if err != nil || cred.APIKey != "shared-key" {
		t.Fatalf("member cannot use shared connection: %v", err)
	}
	for _, scope := range [][2]string{{"workspace", "outsider"}, {"other", "owner"}} {
		if _, _, err := s.Credential(ctx, scope[0], scope[1], id, false); err == nil {
			t.Fatal("unauthorized shared credential access")
		}
	}
	if _, err := s.Reconnect(ctx, "workspace", "teammate", id, "replacement"); err == nil {
		t.Fatal("member rotated shared key")
	}
	if err := s.Disconnect(ctx, "workspace", "teammate", id); err == nil {
		t.Fatal("member disconnected shared key")
	}
	if _, err := s.Reconnect(ctx, "workspace", "owner", id, "rotated"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM workspace_members WHERE user_id='owner'`).Error; err != nil {
		t.Fatal(err)
	}
	_, cred, err = s.sharedCredential(ctx, "workspace", id)
	if err != nil || cred.APIKey != "rotated" {
		t.Fatalf("unattended route depends on former creator: %v", err)
	}
	if err := db.Exec(`UPDATE workspaces SET status='inactive'`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.sharedCredential(ctx, "workspace", id); err == nil {
		t.Fatal("inactive workspace executed")
	}
	req.Provider, req.APIKey = "openai_chatgpt", ""
	if _, err := s.Create(ctx, "workspace", "owner", req); err == nil {
		t.Fatal("shared ChatGPT accepted")
	}
}

func TestPersonalAIConnectionPreservesAADAndRejectsUnattendedAccess(t *testing.T) {
	s, db := setupAIConnectionTest(t)
	ctx := context.Background()
	if err := db.Exec(`CREATE TABLE workspaces (id TEXT, status TEXT); INSERT INTO workspaces VALUES ('workspace','active')`).Error; err != nil {
		t.Fatal(err)
	}
	login, err := s.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Personal", Provider: "openai", APIKey: "private-key"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.repo.Get(ctx, login.Connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldAAD := []byte("ai-connection|v1|workspace|owner|" + c.ID + "|openai")
	if _, err := crypto.DecryptWithAAD(c.EncryptedSecret, s.key, oldAAD); err != nil {
		t.Fatalf("personal encryption binding changed: %v", err)
	}
	if _, _, err := s.sharedCredential(ctx, "workspace", c.ID); err == nil {
		t.Fatal("personal connection used by unattended resolver")
	}
}
