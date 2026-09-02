package service

import "testing"

func TestMeaningfulCoverageSearchRequiresThreeNonStopwordTokens(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"reset", false},
		{"reset password", false},
		{"how do I reset", false},
		{"reset account password", true},
		{"how do I reset my account password", true},
	}
	for _, test := range tests {
		if got := IsMeaningfulCoverageSearchQuery(test.query); got != test.want {
			t.Errorf("IsMeaningfulCoverageSearchQuery(%q) = %v, want %v", test.query, got, test.want)
		}
	}
}
