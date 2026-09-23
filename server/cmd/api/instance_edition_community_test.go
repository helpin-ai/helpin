//go:build !ee

package main

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/config"
)

func TestCommunityEnablesServerAdministration(t *testing.T) {
	if !serverAdminEnabled {
		t.Fatal("Community must enable server administration")
	}
	cfg := &config.Config{ServerAdminEmails: []string{"ops@example.com"}}
	if newInstanceService(nil, cfg, func() bool { return false }) == nil {
		t.Fatal("Community must wire the server admin service")
	}
	if newAppEmailConfig(nil, cfg, nil) == nil {
		t.Fatal("Community must allow application email settings in the app")
	}
}
