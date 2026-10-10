//go:build integration

package repository

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSupportAssignableMembersPostgres(t *testing.T) {
	dsn := os.Getenv("SUPPORT_ASSIGNMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_ASSIGNMENT_TEST_DSN required")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "support_assignees_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		connection, err := base.DB()
		if err != nil {
			t.Error(err)
		} else if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		connection, err := db.DB()
		if err != nil {
			t.Error(err)
		} else if err := connection.Close(); err != nil {
			t.Error(err)
		}
	})
	exec := func(statement string, args ...any) {
		t.Helper()
		if err := db.Exec(statement, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`CREATE TABLE users(id uuid PRIMARY KEY, full_name text, avatar_url text, avatar_style text, avatar_seed text, avatar_background_mode text, avatar_background_color text)`,
		`CREATE TABLE workspace_members(id uuid PRIMARY KEY, workspace_id uuid, user_id uuid, role text, email text, display_name text, status text, invited_by uuid, invited_at timestamptz, accepted_at timestamptz)`,
		`CREATE TABLE team_workspace_memberships(workspace_member_id uuid, team_id uuid)`,
		`CREATE TABLE workspace_module_grants(id uuid PRIMARY KEY, workspace_id uuid, module text, subject_type text, subject_id uuid)`,
		`CREATE TABLE support_mailboxes(id uuid PRIMARY KEY, linked_team_id uuid)`,
		`CREATE TABLE support_mailbox_memberships(mailbox_id uuid, workspace_member_id uuid)`,
	} {
		exec(statement)
	}
	workspace, otherWorkspace := uuid.NewString(), uuid.NewString()
	mailbox, team, secondTeam := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO support_mailboxes VALUES (?, ?)`, mailbox, team)
	memberIDs := make(map[string]string)
	for _, row := range []struct{ key, name, role, status, workspace string }{
		{"admin", "Alpha", "admin", "active", workspace},
		{"direct", "beta", "member", "active", workspace},
		{"team", "CHARLIE", "member", "active", workspace},
		{"outside-inbox", "delta", "member", "active", workspace},
		{"no-support", "Excluded", "member", "active", workspace},
		{"revoked", "Excluded", "admin", "revoked", workspace},
		{"foreign", "Excluded", "admin", "active", otherWorkspace},
	} {
		member, user := uuid.NewString(), uuid.NewString()
		memberIDs[row.key] = member
		exec(`INSERT INTO users(id,full_name) VALUES (?,?)`, user, row.name)
		exec(`INSERT INTO workspace_members(id,workspace_id,user_id,role,email,display_name,status) VALUES (?,?,?,?,?,'',?)`, member, row.workspace, user, row.role, row.key+"@example.invalid", row.status)
	}
	for _, key := range []string{"direct", "outside-inbox"} {
		exec(`INSERT INTO workspace_module_grants VALUES (?,?,'support','workspace_member',?)`, uuid.NewString(), workspace, memberIDs[key])
	}
	// Multiple team memberships and grants must not duplicate a teammate.
	for _, teamID := range []string{team, secondTeam} {
		exec(`INSERT INTO team_workspace_memberships VALUES (?,?)`, memberIDs["direct"], teamID)
		exec(`INSERT INTO workspace_module_grants VALUES (?,?,'support','team',?)`, uuid.NewString(), workspace, teamID)
	}
	exec(`INSERT INTO team_workspace_memberships VALUES (?,?)`, memberIDs["team"], team)
	exec(`INSERT INTO support_mailbox_memberships VALUES (?,?)`, mailbox, memberIDs["direct"])
	for _, tc := range []struct {
		name    string
		mailbox *string
		want    []string
	}{
		{"workspace", nil, []string{memberIDs["admin"], memberIDs["direct"], memberIDs["team"], memberIDs["outside-inbox"]}},
		{"inbox", &mailbox, []string{memberIDs["admin"], memberIDs["direct"], memberIDs["team"]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			members, err := NewWorkspaceRepository(db).ListSupportAssignableMembers(context.Background(), workspace, tc.mailbox)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(members))
			for _, member := range members {
				ids = append(ids, member.ID)
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("ordered eligible members = %v, want %v", ids, tc.want)
			}
		})
	}
}
