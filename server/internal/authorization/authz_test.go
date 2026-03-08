package authorization

import (
	"context"
	"fmt"
	"testing"
)

// mockMemberRepo is a test double for MemberRepository.
type mockMemberRepo struct {
	memberships    map[string]*MemberInfo // key: "workspaceID:userID"
	teamMemberships map[string][]TeamRole  // key: workspaceMemberID
}

func newMockMemberRepo() *mockMemberRepo {
	return &mockMemberRepo{
		memberships:     make(map[string]*MemberInfo),
		teamMemberships: make(map[string][]TeamRole),
	}
}

func (m *mockMemberRepo) GetMembership(_ context.Context, workspaceID, userID string) (*MemberInfo, error) {
	key := workspaceID + ":" + userID
	info, ok := m.memberships[key]
	if !ok {
		return nil, nil
	}
	return info, nil
}

func (m *mockMemberRepo) GetTeamMemberships(_ context.Context, workspaceMemberID string) ([]TeamRole, error) {
	return m.teamMemberships[workspaceMemberID], nil
}

func (m *mockMemberRepo) addMember(workspaceID, userID, memberID, role, status string) {
	key := workspaceID + ":" + userID
	m.memberships[key] = &MemberInfo{ID: memberID, Role: role, Status: status}
}

func (m *mockMemberRepo) addTeamMembership(memberID, teamID, role string) {
	m.teamMemberships[memberID] = append(m.teamMemberships[memberID], TeamRole{TeamID: teamID, Role: role})
}

func TestAuthzService_ResolveActor_ActiveMember(t *testing.T) {
	repo := newMockMemberRepo()
	repo.addMember("ws-1", "user-1", "wm-1", "admin", "active")
	repo.addTeamMembership("wm-1", "team-1", "owner")
	repo.addTeamMembership("wm-1", "team-2", "member")

	svc := &AuthzService{
		rbac:       NewRBACEngine(),
		memberRepo: repo,
	}

	actor, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.UserID != "user-1" {
		t.Errorf("expected UserID user-1, got %s", actor.UserID)
	}
	if actor.WorkspaceMemberID != "wm-1" {
		t.Errorf("expected WorkspaceMemberID wm-1, got %s", actor.WorkspaceMemberID)
	}
	if actor.Role != "admin" {
		t.Errorf("expected role admin, got %s", actor.Role)
	}
	if len(actor.TeamMemberships) != 2 {
		t.Errorf("expected 2 team memberships, got %d", len(actor.TeamMemberships))
	}
}

func TestAuthzService_ResolveActor_NotAMember(t *testing.T) {
	repo := newMockMemberRepo()
	svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

	_, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != ErrNotAMember {
		t.Errorf("expected ErrNotAMember, got %v", err)
	}
}

func TestAuthzService_ResolveActor_PendingMember(t *testing.T) {
	repo := newMockMemberRepo()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "pending")
	svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

	_, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != ErrMembershipPending {
		t.Errorf("expected ErrMembershipPending, got %v", err)
	}
}

func TestAuthzService_ResolveActor_RevokedMember(t *testing.T) {
	repo := newMockMemberRepo()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "revoked")
	svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

	_, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != ErrMembershipRevoked {
		t.Errorf("expected ErrMembershipRevoked, got %v", err)
	}
}

func TestAuthzService_ResolveActor_InactiveMember(t *testing.T) {
	repo := newMockMemberRepo()
	repo.addMember("ws-1", "user-1", "wm-1", "member", "inactive")
	svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

	_, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != ErrMembershipInactive {
		t.Errorf("expected ErrMembershipInactive, got %v", err)
	}
}

func TestAuthzService_Can(t *testing.T) {
	svc := &AuthzService{rbac: NewRBACEngine()}

	actor := &Actor{Role: "admin"}
	if !svc.Can(actor, PermPMEdit) {
		t.Error("admin should have pm.edit")
	}
	if svc.Can(actor, PermWorkspaceDelete) {
		t.Error("admin should NOT have workspace.delete")
	}
}

func TestAuthzService_CanAny(t *testing.T) {
	svc := &AuthzService{rbac: NewRBACEngine()}

	actor := &Actor{Role: "viewer"}
	if !svc.CanAny(actor, PermPMRead, PermPMEdit) {
		t.Error("viewer should have pm.read via CanAny")
	}
	if svc.CanAny(actor, PermPMEdit, PermRewardsManage) {
		t.Error("viewer should NOT have pm.edit or rewards.manage")
	}
}

func TestAuthzService_CanManageTeam(t *testing.T) {
	svc := &AuthzService{rbac: NewRBACEngine()}

	tests := []struct {
		name   string
		actor  *Actor
		teamID string
		want   bool
	}{
		{
			name:   "workspace owner can manage any team",
			actor:  &Actor{Role: "owner"},
			teamID: "any-team",
			want:   true,
		},
		{
			name:   "workspace admin can manage any team",
			actor:  &Actor{Role: "admin"},
			teamID: "any-team",
			want:   true,
		},
		{
			name: "team owner can manage their team",
			actor: &Actor{
				Role:            "member",
				TeamMemberships: []TeamRole{{TeamID: "team-1", Role: "owner"}},
			},
			teamID: "team-1",
			want:   true,
		},
		{
			name: "team member cannot manage their team",
			actor: &Actor{
				Role:            "member",
				TeamMemberships: []TeamRole{{TeamID: "team-1", Role: "member"}},
			},
			teamID: "team-1",
			want:   false,
		},
		{
			name:   "regular member cannot manage any team",
			actor:  &Actor{Role: "member"},
			teamID: "any-team",
			want:   false,
		},
		{
			name: "team owner cannot manage other teams",
			actor: &Actor{
				Role:            "member",
				TeamMemberships: []TeamRole{{TeamID: "team-1", Role: "owner"}},
			},
			teamID: "team-2",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.CanManageTeam(tt.actor, tt.teamID)
			if got != tt.want {
				t.Errorf("CanManageTeam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthzService_IsOwnerOnly(t *testing.T) {
	svc := &AuthzService{rbac: NewRBACEngine()}

	if !svc.IsOwnerOnly(&Actor{Role: "owner"}) {
		t.Error("owner should pass IsOwnerOnly")
	}
	if svc.IsOwnerOnly(&Actor{Role: "admin"}) {
		t.Error("admin should NOT pass IsOwnerOnly")
	}
}

func TestAuthzService_PermissionsForActor(t *testing.T) {
	svc := &AuthzService{rbac: NewRBACEngine()}

	perms := svc.PermissionsForActor(&Actor{Role: "owner"})
	if len(perms) != len(AllPermissions()) {
		t.Errorf("owner should have %d permissions, got %d", len(AllPermissions()), len(perms))
	}
}

func TestAuthzService_CrossWorkspaceIsolation(t *testing.T) {
	repo := newMockMemberRepo()
	repo.addMember("ws-1", "user-1", "wm-1", "admin", "active")
	// user-1 is NOT a member of ws-2

	svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

	// Should succeed for ws-1
	actor, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.WorkspaceID != "ws-1" {
		t.Errorf("expected workspace ws-1, got %s", actor.WorkspaceID)
	}

	// Should fail for ws-2
	_, err = svc.ResolveActor(context.Background(), "ws-2", "user-1")
	if err != ErrNotAMember {
		t.Errorf("expected ErrNotAMember for cross-workspace access, got %v", err)
	}
}

func TestAuthzService_AllStatusTransitions(t *testing.T) {
	statuses := []struct {
		status string
		err    error
	}{
		{"active", nil},
		{"pending", ErrMembershipPending},
		{"revoked", ErrMembershipRevoked},
		{"inactive", ErrMembershipInactive},
	}

	for _, s := range statuses {
		t.Run(fmt.Sprintf("status_%s", s.status), func(t *testing.T) {
			repo := newMockMemberRepo()
			repo.addMember("ws-1", "user-1", "wm-1", "member", s.status)
			svc := &AuthzService{rbac: NewRBACEngine(), memberRepo: repo}

			actor, err := svc.ResolveActor(context.Background(), "ws-1", "user-1")
			if s.err == nil {
				if err != nil {
					t.Fatalf("expected no error for %s, got %v", s.status, err)
				}
				if actor.Status != "active" {
					t.Errorf("expected active status, got %s", actor.Status)
				}
			} else {
				if err != s.err {
					t.Errorf("expected %v for status %s, got %v", s.err, s.status, err)
				}
			}
		})
	}
}
