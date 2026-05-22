package repository

import "testing"

func TestNormalizeSearchTextTreatsSpecialCharactersAsSeparators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "pipe", in: "Billing | Invoices", want: "billing invoices"},
		{name: "mixed punctuation", in: "API-v2 / OAuth (SSO)", want: "api v2 oauth sso"},
		{name: "extra spaces", in: "  Alpha    Beta  ", want: "alpha beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeSearchText(tt.in); got != tt.want {
				t.Fatalf("normalizeSearchText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
