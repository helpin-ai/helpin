package authorization

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

type pickerModuleRepo struct{ fail bool }

func (r pickerModuleRepo) ListAccessibleModules(_ context.Context, workspace, member string, teams []string) ([]model.ModuleID, error) {
	if r.fail {
		return nil, fmt.Errorf("unavailable")
	}
	if workspace == "ws" && (member == "direct" || len(teams) == 1 && teams[0] == "crm-team") {
		return []model.ModuleID{model.ModuleCRM}, nil
	}
	return nil, nil
}
func TestFilterMembersByModuleAccess(t *testing.T) {
	members := newMockMemberRepo()
	members.addTeamMembership("team", "crm-team", "member")
	svc := &AuthzService{memberRepo: members, moduleRepo: pickerModuleRepo{}}
	candidates := []model.AssignableMember{}
	for _, id := range []string{"owner", "admin", "direct", "team", "denied", "pending", "inactive"} {
		role := "member"
		if id == "owner" || id == "admin" {
			role = id
		}
		status := "active"
		if id == "pending" || id == "inactive" {
			status = id
		}
		candidates = append(candidates, model.AssignableMember{ID: id, Role: role, Status: status})
	}
	got, err := svc.FilterMembersByModuleAccess(context.Background(), "ws", candidates, model.ModuleCRM)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("expected owner, admin, direct and team, got %+v", got)
	}
	for i, id := range []string{"owner", "admin", "direct", "team"} {
		if got[i].ID != id {
			t.Fatalf("member %d = %s, want %s", i, got[i].ID, id)
		}
	}
	svc.moduleRepo = pickerModuleRepo{fail: true}
	if _, err := svc.FilterMembersByModuleAccess(context.Background(), "ws", candidates, model.ModuleCRM); err == nil {
		t.Fatal("access lookup failure must not return unfiltered members")
	}
}
