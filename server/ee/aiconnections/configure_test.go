//go:build ee

package aiconnections

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConfigurePreviewImmutableTariffsAndDisable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Tariff{}, &WorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE workspaces (id TEXT PRIMARY KEY); INSERT INTO workspaces VALUES ('ws')").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rate := int64(0)
	options := ConfigureOptions{WorkspaceID: "ws", Enabled: true, Tariff: &aiusage.FlatTokenTariff{Version: "v1", Currency: "USD", MicrousdPerMillion: &rate, AccountingVersion: aiusage.FlatTokenAccountingVersion}}
	preview, err := Configure(ctx, db, options)
	if err != nil || preview.Applied || !preview.Enabled || *preview.Tariff.MicrousdPerMillion != 0 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	for _, table := range []string{"ai_byok_tariffs", "workspace_ai_billing_settings"} {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil || n != 0 {
			t.Fatalf("preview persisted %s: %d, %v", table, n, err)
		}
	}
	options.Apply = true
	for i := 0; i < 2; i++ {
		if _, err := Configure(ctx, db, options); err != nil {
			t.Fatal(err)
		}
	}
	rate = 2_000_000
	if _, err := Configure(ctx, db, options); err == nil {
		t.Fatal("rewrote immutable tariff")
	}
	var stored Tariff
	if err := db.First(&stored).Error; err != nil || stored.MicrousdPerMillion != 0 {
		t.Fatalf("tariff changed: %+v, %v", stored, err)
	}
	options.Tariff.Version = "v2"
	if _, err := Configure(ctx, db, options); err != nil {
		t.Fatal(err)
	}
	disabled, err := Configure(ctx, db, ConfigureOptions{WorkspaceID: "ws", Apply: true})
	if err != nil || disabled.Enabled || disabled.Tariff.Version != "v2" {
		t.Fatalf("disable: %+v, %v", disabled, err)
	}
	var settings WorkspaceSettings
	if err := db.First(&settings).Error; err != nil || settings.BYOKEnabled || *settings.TariffVersion != "v2" {
		t.Fatalf("disable persisted: %+v, %v", settings, err)
	}
	options.WorkspaceID = "missing"
	if _, err := Configure(ctx, db, options); err == nil {
		t.Fatal("configured unknown workspace")
	}
	options.WorkspaceID = "ws"
	options.Tariff.MicrousdPerMillion = nil
	if _, err := Configure(ctx, db, options); err == nil {
		t.Fatal("accepted unset rate")
	}
}
