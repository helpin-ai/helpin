package service

import "testing"

// TestFreezeOrDeriveTranslationSlug covers the Option C "set once,
// then frozen" rule for help-center translation slugs.
func TestFreezeOrDeriveTranslationSlug(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	deref := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}

	tests := []struct {
		name     string
		existing *string
		request  *string
		reqName  string
		want     string
	}{
		{
			name:     "existing slug is returned as-is and ignores the request",
			existing: strPtr("getting-started"),
			request:  strPtr("new-slug"),
			reqName:  "New Name",
			want:     "getting-started",
		},
		{
			name:     "existing slug stays even when request and name are empty",
			existing: strPtr("frozen"),
			request:  nil,
			reqName:  "",
			want:     "frozen",
		},
		{
			name:     "no existing, explicit request slug is normalized and used",
			existing: nil,
			request:  strPtr("First Article"),
			reqName:  "Different Name",
			want:     "first-article",
		},
		{
			name:     "no existing, no request slug falls back to name",
			existing: nil,
			request:  nil,
			reqName:  "Getting Started",
			want:     "getting-started",
		},
		{
			name:     "no existing, empty request slug still falls back to name",
			existing: nil,
			request:  strPtr("   "),
			reqName:  "Billing & Invoicing",
			want:     "billing-invoicing",
		},
		{
			name:     "no existing, empty request and empty name returns nil",
			existing: nil,
			request:  nil,
			reqName:  "",
			want:     "<nil>",
		},
		{
			name:     "existing nil slug on row falls through to derivation — legacy rows can set slug",
			existing: nil,
			request:  nil,
			reqName:  "Help Center",
			want:     "help-center",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freezeOrDeriveTranslationSlug(tt.existing, tt.request, tt.reqName)
			if deref(got) != tt.want {
				t.Errorf("freezeOrDeriveTranslationSlug() = %q, want %q", deref(got), tt.want)
			}
		})
	}
}
