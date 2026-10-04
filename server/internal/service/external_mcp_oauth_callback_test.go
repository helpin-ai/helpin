package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	mcpauth "github.com/helpin-ai/agent-runtime-go/mcpauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExternalMCPOAuthCallbackPreservesInitiatorBinding(t *testing.T) {
	for _, tc := range []struct {
		name, user                   string
		expired, consumed, authorize bool
	}{
		{name: "valid state rechecks initiator permission", user: "initiator", authorize: true},
		{name: "another signed-in user", user: "other"},
		{name: "missing user", user: ""},
		{name: "expired state", user: "initiator", expired: true},
		{name: "replayed state", user: "initiator", consumed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:mcp_callback_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`CREATE TABLE external_mcp_oauth_states (id TEXT PRIMARY KEY, state_hash TEXT, workspace_id TEXT, server_id TEXT, user_id TEXT, return_path TEXT, expires_at DATETIME, consumed_at DATETIME)`).Error; err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			expires := now.Add(time.Minute)
			if tc.expired {
				expires = now.Add(-time.Minute)
			}
			var consumed *time.Time
			if tc.consumed {
				consumed = &now
			}
			if err := db.Exec(`INSERT INTO external_mcp_oauth_states VALUES (?,?,?,?,?,?,?,?)`, "state-id", mcpauth.HashState("secret-state"), "workspace", "server", "initiator", "/w/acme/settings/external-tools", expires, consumed).Error; err != nil {
				t.Fatal(err)
			}
			svc := &ExternalMCPService{repo: repository.NewExternalMCPRepository(db), cfg: ExternalMCPServiceConfig{Enabled: true}, now: func() time.Time { return now }}
			calls := 0
			denied := errors.New("permission revoked")
			result, err := svc.CompleteOAuth(context.Background(), tc.user, "secret-state", "provider-code", "", func(_ context.Context, workspace, user string) error {
				calls++
				if workspace != "workspace" || user != "initiator" {
					t.Fatalf("authorization used unbound identity: %s %s", workspace, user)
				}
				return denied
			})
			if err == nil {
				t.Fatal("unsafe callback was accepted")
			}
			if tc.authorize {
				if calls != 1 || !errors.Is(err, denied) || result == nil || result.WorkspaceID != "workspace" {
					t.Fatalf("permission check bypassed: %d %v %+v", calls, err, result)
				}
				_, err = svc.CompleteOAuth(context.Background(), tc.user, "secret-state", "provider-code", "", func(context.Context, string, string) error { t.Fatal("state reused"); return nil })
				if err == nil {
					t.Fatal("state replay accepted")
				}
			} else if calls != 0 || result != nil {
				t.Fatal("unbound callback reached authorization")
			}
		})
	}
}
