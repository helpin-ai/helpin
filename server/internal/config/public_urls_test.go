package config

import "testing"

func TestPublicURL(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		origin bool
		want   string
	}{
		{"http://localhost:8090/", true, "http://localhost:8090"},
		{"https://assets.example.test/sdk/lib.js", false, "https://assets.example.test/sdk/lib.js"},
		{"https://example.test/api", true, ""}, {"https://user:secret@example.test", false, ""},
		{"https://example.test/?secret=value", false, ""}, {"javascript:alert(1)", false, ""},
	} {
		got, err := publicURL(tc.raw, tc.origin)
		if tc.want == "" {
			if err == nil {
				t.Errorf("accepted invalid public address")
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("publicURL(%q)=%q,%v", tc.raw, got, err)
		}
	}
}
