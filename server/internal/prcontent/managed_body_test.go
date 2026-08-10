package prcontent

import "testing"

func TestMergePreservesHumanContentAndRefreshesManagedSection(t *testing.T) {
	existing := "Human introduction.\n\n" + Wrap("old") + "\n\nHuman checklist."
	got := Merge(existing, Wrap("new"))
	want := "Human introduction.\n\n" + Wrap("new") + "\n\nHuman checklist."
	if got != want {
		t.Fatalf("Merge() = %q, want %q", got, want)
	}
}

func TestMergeAppendsManagedSectionToLegacyBody(t *testing.T) {
	got := Merge("Human-authored body.", Wrap("Helpin context"))
	want := "Human-authored body.\n\n" + Wrap("Helpin context")
	if got != want {
		t.Fatalf("Merge() = %q, want %q", got, want)
	}
}
