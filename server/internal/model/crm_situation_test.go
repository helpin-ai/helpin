package model

import "testing"

func TestCRMSituationCategoryMapping(t *testing.T) {
	tests := []struct{ motion, category string }{
		{"prospecting", "sales"}, {"conversion", "sales"},
		{"onboarding", "onboarding_adoption"}, {"adoption", "onboarding_adoption"},
		{"expansion", "expansion"}, {"renewal", "retention"}, {"retention", "retention"},
		{"sales", ""}, {"unknown", ""}, {"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.motion, func(t *testing.T) {
			if got := CRMSituationCategoryForMotion(tt.motion); got != tt.category {
				t.Fatalf("category = %q, want %q", got, tt.category)
			}
		})
	}
}

func TestCRMSituationCategoriesAreIndependentCopies(t *testing.T) {
	first := CRMSituationCategories()
	first[0].Motions[0] = "changed"
	if got := CRMSituationCategories()[0]; got.Label != "Sales" || got.Motions[0] != "prospecting" {
		t.Fatalf("shared category state was modified: %#v", got)
	}
}
