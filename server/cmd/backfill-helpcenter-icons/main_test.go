package main

import "testing"

func iconPointer(value string) *string { return &value }

func TestNormalizePersistedIcon(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		opts    options
		kind    string
		want    *string
		changed bool
	}{
		{name: "canonical alias", value: "gear-six", kind: "canonical", want: iconPointer("settings"), changed: true},
		{name: "case variant", value: "Rocket", kind: "canonical", want: iconPointer("rocket"), changed: true},
		{name: "unicode", value: " 🚀 ", kind: "unicode-display", want: iconPointer("🚀"), changed: true},
		{name: "ASCII review", value: "FAQ", kind: "ascii-display-review", want: iconPointer("FAQ")},
		{name: "ASCII clear", value: "FAQ", opts: options{clearASCIIDisplayText: true}, kind: "ascii-display-cleared", changed: true},
		{name: "unknown review", value: "not-a-real-icon", kind: "unresolved-review", want: iconPointer("not-a-real-icon")},
		{name: "unknown clear", value: "not-a-real-icon", opts: options{clearUnresolved: true}, kind: "unresolved-cleared", changed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizePersistedIcon(iconPointer(test.value), test.opts)
			if got.kind != test.kind || got.changed != test.changed || !equalPointers(got.value, test.want) {
				t.Fatalf("normalizePersistedIcon() = %#v, want kind=%q value=%v changed=%v", got, test.kind, test.want, test.changed)
			}
		})
	}
}

func equalPointers(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
