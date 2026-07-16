package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func homepageConfigJSON(t *testing.T, icons ...string) json.RawMessage {
	t.Helper()
	cards := make([]model.HomepageFeaturedCard, 0, len(icons))
	for _, icon := range icons {
		cards = append(cards, model.HomepageFeaturedCard{
			Title:     "Card",
			Icon:      icon,
			LinkType:  "url",
			LinkValue: "https://example.com",
		})
	}
	raw, err := json.Marshal(model.HelpcenterHomepageConfig{FeaturedCards: cards})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decodeHomepageConfig(t *testing.T, raw json.RawMessage) model.HelpcenterHomepageConfig {
	t.Helper()
	var result model.HelpcenterHomepageConfig
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNormalizeHomepageConfigIconWrites(t *testing.T) {
	current := homepageConfigJSON(t, "Rocket01Icon", "folder")

	unchanged, err := normalizeHomepageConfigIconWrites(
		homepageConfigJSON(t, "Rocket01Icon", "Gear-Six"),
		current,
	)
	if err != nil {
		t.Fatalf("normalize unchanged historical icon: %v", err)
	}
	got := decodeHomepageConfig(t, unchanged)
	if got.FeaturedCards[0].Icon != "Rocket01Icon" {
		t.Fatalf("historical icon = %q, want unchanged", got.FeaturedCards[0].Icon)
	}
	if got.FeaturedCards[1].Icon != "settings" {
		t.Fatalf("canonicalized icon = %q, want settings", got.FeaturedCards[1].Icon)
	}

	if _, err := normalizeHomepageConfigIconWrites(homepageConfigJSON(t, "rocket icon"), current); err == nil {
		t.Fatal("changed invalid featured-card icon unexpectedly succeeded")
	}
}

func TestNormalizePublicHomepageIcons(t *testing.T) {
	cfg := &model.DocsHelpcenterConfig{
		HomepageConfig: homepageConfigJSON(t, "gear-six", "unknown-icon-id", "🚀", "FAQ"),
	}
	normalizePublicHomepageIcons(cfg)
	got := decodeHomepageConfig(t, cfg.HomepageConfig)
	want := []string{"settings", "folder", "🚀", "FAQ"}
	for index, expected := range want {
		if got.FeaturedCards[index].Icon != expected {
			t.Fatalf("icon %d = %q, want %q", index, got.FeaturedCards[index].Icon, expected)
		}
	}
}
