package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAuthorizeCommandActor(t *testing.T) {
	svc := &InternalCommandService{authz: authorization.NewAuthzService(nil, nil, nil)}

	tests := []struct {
		name     string
		role     string
		module   string
		mutating bool
		wantDeny bool
	}{
		{name: "no role passes through", role: "", module: "pm", mutating: true},
		{name: "viewer can read pm", role: "viewer", module: "pm"},
		{name: "viewer cannot mutate pm", role: "viewer", module: "pm", mutating: true, wantDeny: true},
		{name: "member can mutate pm", role: "member", module: "pm", mutating: true},
		{name: "viewer cannot mutate docs", role: "viewer", module: "docs", mutating: true, wantDeny: true},
		{name: "viewer can read agents", role: "viewer", module: "agents"},
		{name: "viewer cannot launch agents", role: "viewer", module: "agents", mutating: true, wantDeny: true},
		{name: "member can launch agents", role: "member", module: "agents", mutating: true},
		{name: "unknown module is not gated", role: "viewer", module: "somethingelse", mutating: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := model.InternalCommandContext{
				WorkspaceID: "ws",
				ActorID:     "user",
				ActorRole:   tt.role,
			}
			def := InternalCommandDefinition{Name: tt.module + ".cmd", Module: tt.module, Mutating: tt.mutating}
			err := svc.authorizeCommandActor(meta, def)
			if tt.wantDeny {
				if err == nil {
					t.Fatalf("authorizeCommandActor() = nil, want permission error")
				}
				if !strings.Contains(err.Error(), "permission") {
					t.Errorf("authorizeCommandActor() error = %v, want permission error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("authorizeCommandActor() error = %v, want nil", err)
			}
		})
	}
}

func TestAuthorizeCommandActorWithoutAuthz(t *testing.T) {
	svc := &InternalCommandService{}
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "user", ActorRole: "viewer"}
	def := InternalCommandDefinition{Name: "pm.cmd", Module: "pm", Mutating: true}
	if err := svc.authorizeCommandActor(meta, def); err != nil {
		t.Fatalf("authorizeCommandActor() without authz = %v, want nil", err)
	}
}
