//go:build ee

package aiconnections

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSaaSBYOKFlagDefaultsOffAndExplicitZeroIsValid(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Tariff{}, &WorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	p := NewPolicy(db)
	c := &model.AIConnection{WorkspaceID: "ws", Funding: "customer", Provider: "openai_chatgpt", Scope: "personal"}
	ctx := context.Background()
	if _, err := p.ResolveConnectionPolicy(ctx, "ws", c); !errors.Is(err, ErrBYOKUnavailable) {
		t.Fatalf("default: %v", err)
	}
	version := "free-preview-v1"
	if err := db.Create(&Tariff{Version: version, Currency: "USD", AccountingVersion: aiusage.FlatTokenAccountingVersion}).Error; err != nil {
		t.Fatal(err)
	}
	settings := &WorkspaceSettings{WorkspaceID: "ws", TariffVersion: &version}
	if err := db.Create(settings).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := p.ResolveConnectionPolicy(ctx, "ws", c); !errors.Is(err, ErrBYOKUnavailable) {
		t.Fatalf("disabled: %v", err)
	}
	settings.BYOKEnabled = true
	if err := db.Save(settings).Error; err != nil {
		t.Fatal(err)
	}
	policy, err := p.ResolveConnectionPolicy(ctx, "ws", c)
	if err != nil {
		t.Fatal(err)
	}
	if policy.FundingMode != aiusage.FundingCustomerFlat || policy.FlatTariff.MicrousdPerMillion == nil || *policy.FlatTariff.MicrousdPerMillion != 0 {
		t.Fatalf("explicit zero missing: %+v", policy)
	}
	settings.TariffVersion = nil
	if err := db.Save(settings).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := p.ResolveConnectionPolicy(ctx, "ws", c); !errors.Is(err, ErrBYOKUnavailable) {
		t.Fatalf("missing tariff: %v", err)
	}
	if _, err := p.ResolveConnectionPolicy(ctx, "other", c); !errors.Is(err, ErrBYOKUnavailable) {
		t.Fatalf("cross-workspace: %v", err)
	}
}

func TestManagedRouteKeepsHostedPricingWithoutBYOKFlag(t *testing.T) {
	p := NewPolicy(nil)
	c := &model.AIConnection{WorkspaceID: "ws", Funding: "managed", Scope: "workspace", Provider: "openai"}
	policy, err := p.ResolveConnectionPolicy(context.Background(), "ws", c)
	if err != nil || policy.FundingMode != aiusage.FundingHelpinHosted || policy.FlatTariff != nil {
		t.Fatalf("managed policy: %+v, %v", policy, err)
	}
	c.Provider = "openai_chatgpt"
	if _, err := p.ResolveConnectionPolicy(context.Background(), "ws", c); !errors.Is(err, ErrBYOKUnavailable) {
		t.Fatalf("managed personal OAuth accepted: %v", err)
	}
}
