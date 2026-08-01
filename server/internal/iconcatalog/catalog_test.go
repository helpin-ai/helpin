package iconcatalog

import "testing"

func stringPointer(value string) *string { return &value }

func TestResolveStoredPreservesCompatibility(t *testing.T) {
	tests := []struct {
		name  string
		value string
		kind  ResolutionKind
		want  string
	}{
		{name: "canonical", value: "rocket01", kind: ResolutionAsset, want: "rocket01"},
		{name: "case variant", value: "Rocket", kind: ResolutionAsset, want: "rocket"},
		{name: "legacy alias", value: "gear-six", kind: ResolutionAsset, want: "settings"},
		{name: "legacy export", value: "QuillWriteIcon", kind: ResolutionAsset, want: "quill-write"},
		{name: "legacy ASCII text", value: "FAQ", kind: ResolutionDisplayText, want: "FAQ"},
		{name: "unknown raw export", value: "DefinitelyNotAnIcon", kind: ResolutionNone},
		{name: "unknown identifier", value: "definitely-not-an-icon", kind: ResolutionNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveStored(stringPointer(test.value))
			if got.Kind != test.kind || got.Value != test.want {
				t.Fatalf("ResolveStored(%q) = %#v, want kind %d value %q", test.value, got, test.kind, test.want)
			}
		})
	}
}

func TestResolvePublicValueUsesFallbackWithoutExposingRawID(t *testing.T) {
	got := ResolvePublicValue(stringPointer("definitely-not-an-icon"), "folder")
	if got == nil || *got != "folder" {
		t.Fatalf("ResolvePublicValue() = %v, want folder", got)
	}
	if got := ResolvePublicValue(nil, "folder"); got != nil {
		t.Fatalf("ResolvePublicValue(nil) = %v, want nil", got)
	}
}

func TestNormalizeNewRejectsASCIINamesAndAllowsDisplayText(t *testing.T) {
	for _, value := range []string{"Rocket01Icon", "rocket icon", "FAQ", "not-a-real-icon"} {
		if _, err := NormalizeNew(stringPointer(value)); err == nil {
			t.Fatalf("NormalizeNew(%q) unexpectedly succeeded", value)
		}
	}
	for _, value := range []string{"Rocket", "🚀", "文档"} {
		if _, err := NormalizeNew(stringPointer(value)); err != nil {
			t.Fatalf("NormalizeNew(%q) returned %v", value, err)
		}
	}
	if normalized, err := NormalizeNew(stringPointer("  ")); err != nil || normalized != nil {
		t.Fatalf("NormalizeNew(empty) = (%v, %v), want (nil, nil)", normalized, err)
	}
}

func TestNormalizeUpdateToleratesUnchangedHistoricalValue(t *testing.T) {
	invalid := stringPointer("Rocket01Icon")
	if normalized, changed, err := NormalizeUpdate(invalid, invalid); err != nil || changed || normalized != nil {
		t.Fatalf("unchanged update = (%v, %v, %v), want (nil, false, nil)", normalized, changed, err)
	}
	if _, _, err := NormalizeUpdate(stringPointer("rocket icon"), invalid); err == nil {
		t.Fatal("changed invalid update unexpectedly succeeded")
	}
	normalized, changed, err := NormalizeUpdate(stringPointer("Rocket"), invalid)
	if err != nil || !changed || normalized == nil || *normalized != "rocket" {
		t.Fatalf("valid update = (%v, %v, %v), want (rocket, true, nil)", normalized, changed, err)
	}
	normalized, changed, err = NormalizeUpdate(stringPointer(""), stringPointer("rocket"))
	if err != nil || !changed || normalized != nil {
		t.Fatalf("clear update = (%v, %v, %v), want (nil, true, nil)", normalized, changed, err)
	}
}

func TestSearchReturnsCanonicalBoundedResults(t *testing.T) {
	results := Search("rocket", 3)
	if len(results) == 0 || len(results) > 3 {
		t.Fatalf("Search() returned %d results", len(results))
	}
	if results[0].ID != "rocket" {
		t.Fatalf("Search()[0] = %#v, want rocket", results[0])
	}
	if results[0].Label == "" {
		t.Fatal("Search() returned an empty label")
	}
}
