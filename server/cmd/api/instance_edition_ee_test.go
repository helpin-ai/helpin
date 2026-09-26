//go:build ee

package main

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/config"
)

func TestEnterpriseDisablesServerAdministration(t *testing.T) {
	if serverAdminEnabled {
		t.Fatal("Enterprise must not enable server administration")
	}
	cfg := &config.Config{ServerAdminEmails: []string{"ops@example.com"}}
	if newInstanceService(nil, cfg, func() bool { return true }) != nil {
		t.Fatal("Enterprise must not apply a signup policy or grant server admins")
	}
	if newAppEmailConfig(nil, cfg, nil) != nil {
		t.Fatal("Enterprise application email must come only from the environment")
	}
}
