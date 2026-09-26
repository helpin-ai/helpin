package aimodel

import (
	"errors"
	"testing"
)

func TestCustomModelNeedsNoPriceOrTier(t *testing.T) {
	catalog := &Catalog{
		Models: []ModelDefinition{{Provider: "openai", CanonicalModel: "custom-model", Aliases: []string{"custom"}, Enabled: true}},
		Routes: []RouteDefinition{{Provider: "openai", CanonicalModel: "custom-model", Route: "custom", ServiceTier: "standard", Enabled: true}},
	}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.ResolveDefault(" OpenAI ", "CUSTOM", "")
	if err != nil || got.Route != "custom" || got.Tier != "" {
		t.Fatalf("resolve unpriced model: %+v, %v", got, err)
	}
}

func TestResolveDefaultRejectsAmbiguousCanonicalModel(t *testing.T) {
	catalog := &Catalog{
		Models: []ModelDefinition{{Provider: "openrouter", CanonicalModel: "model", Aliases: []string{"model:fast", "model:other"}, Enabled: true}},
		Routes: []RouteDefinition{
			{Provider: "openrouter", CanonicalModel: "model", Route: "model:fast", ServiceTier: "standard", Enabled: true},
			{Provider: "openrouter", CanonicalModel: "model", Route: "model:other", ServiceTier: "standard", Enabled: true},
		},
	}
	if _, err := catalog.ResolveDefault("openrouter", "model", ""); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("ambiguous route error = %v", err)
	}
	if got, err := catalog.ResolveDefault("openrouter", "model:fast", ""); err != nil || got.Route != "model:fast" {
		t.Fatalf("exact route = %+v, %v", got, err)
	}
}

func TestCatalogRejectsInvalidMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Catalog)
	}{
		{"duplicate model", func(c *Catalog) { c.Models = append(c.Models, c.Models[0]) }},
		{"alias collision", func(c *Catalog) {
			c.Models[1].Aliases = append(c.Models[1].Aliases, c.Models[0].CanonicalModel)
			c.Models[1].Provider = c.Models[0].Provider
		}},
		{"duplicate route", func(c *Catalog) { c.Routes = append(c.Routes, c.Routes[0]) }},
		{"unknown model", func(c *Catalog) { c.Routes[0].CanonicalModel = "missing" }},
		{"invalid limits", func(c *Catalog) { c.Routes[0].MaximumOutput = c.Routes[0].ContextWindow + 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog, err := LoadCatalog()
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(catalog)
			if err := catalog.Validate(); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
}

func TestDisabledModelCannotResolve(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	route := catalog.Routes[0]
	for i := range catalog.Models {
		if catalog.Models[i].Provider == route.Provider && catalog.Models[i].CanonicalModel == route.CanonicalModel {
			catalog.Models[i].Enabled = false
		}
	}
	if _, err := catalog.ResolveDefault(route.Provider, route.Route, route.ServiceTier); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("disabled model error = %v", err)
	}
}
