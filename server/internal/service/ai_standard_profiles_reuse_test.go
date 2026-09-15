package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestStandardProfilesReuseExistingManagedConnections(t *testing.T) {
	for _, funding := range []string{"managed", "customer"} {
		t.Run(funding, func(t *testing.T) {
			s, connections, db := standardProfilesFixture(t)
			s.funding = funding // Agent creation uses the customer-mode initializer even in EE.
			ctx := context.Background()
			for _, provider := range []string{"openai", "anthropic", "openrouter"} {
				c := &model.AIConnection{ID: "existing-" + provider, WorkspaceID: "workspace", Scope: "workspace", Funding: "managed", Provider: provider, Name: provider + " (managed)", Status: "connected"}
				if err := connections.seal(c, aiConnectionSecret{APIKey: "existing-key"}); err != nil {
					t.Fatal(err)
				}
				if err := connections.repo.Create(ctx, c); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
					t.Fatal(err)
				}
			}
			var count int64
			if err := db.Model(&model.AIConnection{}).Count(&count).Error; err != nil || count != 3 {
				t.Fatalf("provisioning created duplicate connections: %d, %v", count, err)
			}
			for tier, provider := range map[string]string{"small": "openrouter", "medium": "openrouter", "large": "openai", "flagship": "anthropic"} {
				var p model.AIProfile
				if err := db.First(&p, "id = ?", model.StandardAIProfileID("workspace", tier)).Error; err != nil {
					t.Fatal(err)
				}
				if p.Primary.ConnectionID != "existing-"+provider || p.Primary.Model.Provider != provider {
					t.Fatalf("%s did not reuse its existing managed connection", tier)
				}
			}
		})
	}
}

func TestSupersededManagedConnectionRemainsAvailableToFrozenRuns(t *testing.T) {
	s, connections, db := standardProfilesFixture(t)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	canonical := &model.AIConnection{ID: "existing-openrouter", WorkspaceID: "workspace", Scope: "workspace", Funding: "managed", Provider: "openrouter", Name: "openrouter (managed)", Status: "connected"}
	if err := connections.seal(canonical, aiConnectionSecret{APIKey: "existing-key"}); err != nil {
		t.Fatal(err)
	}
	if err := connections.repo.Create(ctx, canonical); err != nil {
		t.Fatal(err)
	}
	oldID := model.StandardAIConnectionID("workspace", "openrouter")
	s.credentials["openrouter"] = "rotated-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AIConnection{}).Where("id = ?", oldID).Update("superseded_by", canonical.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	visible, err := connections.List(ctx, "workspace", "owner")
	if err != nil || len(visible) != 3 {
		t.Fatalf("duplicate still listed: %d, %v", len(visible), err)
	}
	for _, c := range visible {
		if c.ID == oldID {
			t.Fatal("superseded connection offered for new profiles")
		}
	}
	current, err := connections.repo.Get(ctx, canonical.ID)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(current)
	if err != nil || secret.APIKey != "rotated-key" {
		t.Fatal("managed rotation did not update the canonical connection")
	}
	route, err := standardModelForTier("small")
	if err != nil {
		t.Fatal(err)
	}
	selection := &model.AIExecutionSelection{ConnectionScope: "workspace", Route: model.AIProfileRoute{ConnectionID: oldID, Model: route}}
	profiles := NewAIProfileService(repository.NewAIProfileRepository(db), connections)
	if _, err := profiles.Restore(ctx, "workspace", "owner", selection); err != nil {
		t.Fatalf("frozen run lost its credential: %v", err)
	}
	old, err := connections.repo.Get(ctx, oldID)
	if err != nil {
		t.Fatal(err)
	}
	secret, err = connections.open(old)
	if err != nil || secret.APIKey != "first-fixture-key" {
		t.Fatal("frozen run credential changed")
	}
	if err := profiles.validateRoute(ctx, "workspace", "owner", "workspace", &selection.Route); err == nil {
		t.Fatal("superseded connection accepted for a new profile")
	}
}

func TestStandardProfilesPreserveDisconnectedManagedDefault(t *testing.T) {
	s, connections, db := standardProfilesFixture(t)
	ctx := context.Background()
	c := &model.AIConnection{ID: "existing-openai", WorkspaceID: "workspace", Scope: "workspace", Funding: "managed", Provider: "openai", Name: "openai (managed)", Status: "disconnected"}
	if err := connections.repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	s.credentials["openai"] = "configured-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	var p model.AIProfile
	if err := db.First(&p, "id = ?", model.StandardAIProfileID("workspace", "large")).Error; err != nil || p.Primary.ConnectionID != c.ID {
		t.Fatal("disconnected default bypassed")
	}
	stored, err := connections.repo.Get(ctx, c.ID)
	if err != nil || stored.Status != "disconnected" || len(stored.EncryptedSecret) != 0 {
		t.Fatal("disconnected default reconnected")
	}
}
