package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestServiceAccountWriteGrantIsRefusedInsteadOfDowngraded(t *testing.T) {
	writable := &model.MCPWorkspacePolicy{
		AllowedToolsets: mustMCPJSON([]string{MCPToolsetContext, MCPToolsetCRM}),
		AllowedScopes:   mustMCPJSON([]string{MCPScopeContextRead, MCPScopeCRMRead, MCPScopeCRMWrite}),
	}
	enforced := *writable
	enforced.EnforceReadOnly = true
	noWriteScope := &model.MCPWorkspacePolicy{
		AllowedToolsets: mustMCPJSON([]string{MCPToolsetContext, MCPToolsetCRM}),
		AllowedScopes:   mustMCPJSON([]string{MCPScopeContextRead, MCPScopeCRMRead}),
	}
	writeRequest := []string{MCPScopeContextRead, MCPScopeCRMRead, MCPScopeCRMWrite}
	toolsets := []string{MCPToolsetContext, MCPToolsetCRM}

	tests := []struct {
		name         string
		policy       *model.MCPWorkspacePolicy
		readOnly     bool
		wantErr      string
		wantReadOnly bool
	}{
		{name: "write request on a writable policy keeps write access", policy: writable},
		{name: "write request under enforced read-only is refused", policy: &enforced, wantErr: "turn on bounded writes"},
		{name: "write request without an allowed write scope is refused", policy: noWriteScope, wantErr: "write scope"},
		{name: "read-only request under enforced read-only is fine", policy: &enforced, readOnly: true, wantReadOnly: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &MCPService{}
			_, scopes, granted, err := service.constrainGrant(tt.policy, toolsets, writeRequest, tt.readOnly)
			if err != nil {
				t.Fatalf("constrainGrant: %v", err)
			}
			err = serviceAccountWriteGrantError(tt.readOnly, granted, tt.policy)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if granted != tt.wantReadOnly {
					t.Fatalf("read-only = %v, want %v (scopes %v)", granted, tt.wantReadOnly, scopes)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want one mentioning %q", err, tt.wantErr)
			}
		})
	}
}
