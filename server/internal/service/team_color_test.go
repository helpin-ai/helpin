package service

import (
	"context"
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCreateTeam_Color(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "preset", input: "#5e6ad2", want: "#5e6ad2"},
		{name: "short hex", input: " #AbC ", want: "#aabbcc"},
		{name: "default", input: ""},
		{name: "named CSS", input: "red", invalid: true},
		{name: "alpha", input: "#11223344", invalid: true},
		{name: "invalid hex", input: "#zzzzzz", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := newSettingsService(t)
			seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
			seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
			team, err := svc.CreateTeam(context.Background(), model.CreateTeamRequest{
				WorkspaceID: "ws1", Name: "Design", Color: &tc.input,
			}, "")
			if tc.invalid {
				if err == nil {
					t.Fatal("expected invalid color to be rejected")
				}
				var count int64
				if err := db.Model(&model.WorkspaceTeam{}).Count(&count).Error; err != nil {
					t.Fatalf("count teams: %v", err)
				}
				if count != 0 {
					t.Fatal("invalid color created a team")
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateTeam: %v", err)
			}
			stored, err := svc.settingsRepo.GetTeamByID(context.Background(), team.ID)
			if err != nil {
				t.Fatalf("GetTeamByID: %v", err)
			}
			if tc.want == "" {
				if stored.Color == nil || !slices.Contains([]string{"#5e6ad2", "#4e8fea", "#3daed4", "#2da88e", "#45a557", "#7da642", "#c7a53d", "#e58c3a", "#e2564a", "#e54e78", "#d44ca0", "#b44ec9", "#8b5cf6", "#4a9ed6", "#a08060", "#788596"}, *stored.Color) {
					t.Fatalf("expected an automatic palette color, got %v", stored.Color)
				}
			} else if stored.Color == nil || *stored.Color != tc.want {
				t.Fatalf("expected persisted color %s, got %v", tc.want, stored.Color)
			}
		})
	}
}

func TestUpdateTeam_ColorPreservesAndResets(t *testing.T) {
	svc, db := newSettingsService(t)
	ctx := context.Background()
	seedUser(t, db, "u1", "owner@test.com", "Owner", "hash")
	seedWorkspace(t, db, "ws1", "Test WS", "test-ws", "u1")
	team, err := svc.CreateTeam(ctx, model.CreateTeamRequest{WorkspaceID: "ws1", Name: "Design"}, "")
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if team.Color == nil {
		t.Fatal("expected an omitted color to receive a palette color")
	}
	color := " #ABCDEF "
	if _, err := svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{Color: &color}); err != nil {
		t.Fatalf("save color: %v", err)
	}
	name := "Product Design"
	if _, err := svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{Name: &name}); err != nil {
		t.Fatalf("rename team: %v", err)
	}
	invalid := "var(--primary)"
	if _, err := svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{Color: &invalid}); err == nil {
		t.Fatal("expected invalid color to be rejected")
	}
	teams, err := svc.settingsRepo.ListTeams(ctx, "ws1")
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 1 || teams[0].Color == nil || *teams[0].Color != "#abcdef" || teams[0].Name != name {
		t.Fatalf("expected renamed team to keep saved color, got %+v", teams)
	}
	reset := ""
	if _, err := svc.UpdateTeam(ctx, team.ID, model.UpdateTeamRequest{Color: &reset}); err != nil {
		t.Fatalf("reset color: %v", err)
	}
	stored, err := svc.settingsRepo.GetTeamByID(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetTeamByID: %v", err)
	}
	if stored.Color != nil {
		t.Fatalf("expected reset to persist NULL, got %q", *stored.Color)
	}
}
