package widgetorigin

import "testing"

func TestAllowed(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		list         []string
		want         bool
	}{
		{"tauri", "tauri://localhost", []string{"tauri://localhost"}, true},
		{"tauri canonical", "TAURI://LOCALHOST", []string{"tauri://localhost"}, true},
		{"tauri not configured", "tauri://localhost", []string{"http://tauri.localhost"}, false},
		{"tauri empty policy", "tauri://localhost", nil, false},
		{"tauri http", "http://tauri.localhost", []string{"http://tauri.localhost"}, true},
		{"tauri https", "https://tauri.localhost", []string{"https://tauri.localhost"}, true},
		{"tauri http mismatch", "http://tauri.localhost", []string{"tauri://localhost"}, false},
		{"exact", "https://customer.example", []string{"https://customer.example"}, true},
		{"canonical", "https://CUSTOMER.example:443", []string{"https://customer.example"}, true},
		{"third party", "https://attacker.example", []string{"https://customer.example"}, false},
		{"subdomain", "https://sub.customer.example", []string{"https://customer.example"}, false},
		{"different port", "https://customer.example:444", []string{"https://customer.example"}, false},
		{"http downgrade", "http://customer.example", []string{"https://customer.example"}, false},
		{"empty policy", "https://customer.example", nil, false},
		{"wildcard", "https://customer.example", []string{"*"}, true},
		{"wildcard missing", "", []string{"*"}, true},
		{"wildcard opaque", "null", []string{"*"}, true},
		{"missing", "", []string{"https://customer.example"}, false},
		{"opaque", "null", []string{"null"}, false},
		{"credentials", "https://user@customer.example", []string{"https://customer.example"}, false},
		{"path", "https://customer.example/", []string{"https://customer.example"}, false},
		{"multiple", "https://customer.example https://evil.example", []string{"https://customer.example"}, false},
		{"query", "https://customer.example?", []string{"https://customer.example"}, false},
		{"fragment", "https://customer.example#", []string{"https://customer.example"}, false},
		{"ipv6", "http://[::1]:80", []string{"http://[::1]"}, true},
		{"subdomain pattern", "https://app.customer.example", []string{"https://*.customer.example"}, true},
		{"subdomain pattern nested", "https://a.b.customer.example", []string{"https://*.customer.example"}, true},
		{"subdomain pattern canonical", "https://APP.customer.example:443", []string{"https://*.customer.example"}, true},
		{"subdomain pattern excludes root", "https://customer.example", []string{"https://*.customer.example"}, false},
		{"subdomain pattern lookalike", "https://evilcustomer.example", []string{"https://*.customer.example"}, false},
		{"subdomain pattern suffix only", "https://customer.example.evil", []string{"https://*.customer.example"}, false},
		{"subdomain pattern http", "http://app.customer.example", []string{"https://*.customer.example"}, false},
		{"subdomain pattern port", "https://app.customer.example:8443", []string{"https://*.customer.example"}, false},
		{"subdomain pattern same port", "https://app.customer.example:8443", []string{"https://*.customer.example:8443"}, true},
		{"subdomain pattern missing", "", []string{"https://*.customer.example"}, false},
		{"root and pattern", "https://customer.example", []string{"https://*.customer.example", "https://customer.example"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Allowed(tc.origin, tc.list); got != tc.want {
				t.Fatalf("Allowed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeEntry(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"*", "*"},
		{"https://customer.example", "https://customer.example"},
		{"https://*.customer.example", "https://*.customer.example"},
		{"HTTPS://*.Customer.Example:443", "https://*.customer.example"},
		{"https://*.customer.example:8443", "https://*.customer.example:8443"},
		{"https://*.acme.github.io", "https://*.acme.github.io"},
		{"https://*.example.co.uk", "https://*.example.co.uk"},
		{"tauri://localhost", "tauri://localhost"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := NormalizeEntry(tc.raw)
			if err != nil || got != tc.want {
				t.Fatalf("NormalizeEntry(%q) = %q, %v; want %q", tc.raw, got, err, tc.want)
			}
		})
	}
	for _, raw := range []string{
		"https://*.", "https://*", "http://*.customer.example", "*.customer.example",
		"https://*.*.customer.example", "https://app.*.customer.example", "https://*customer.example",
		"https://*.com", "https://*.co.uk", "https://*.github.io", "https://*.localhost",
		"https://*.127.0.0.1", "https://*.[::1]", "https://*.customer.example/path",
		"https://*.customer.example?", "https://user@*.customer.example", "**",
	} {
		t.Run(raw, func(t *testing.T) {
			if got, err := NormalizeEntry(raw); err == nil {
				t.Fatalf("accepted %q as %q", raw, got)
			}
		})
	}
}

func TestWebsiteDefaults(t *testing.T) {
	for _, tc := range []struct {
		website string
		want    []string
	}{
		{"https://www.customer.example/pricing", []string{"https://customer.example", "https://*.customer.example"}},
		{"customer.example", []string{"https://customer.example", "https://*.customer.example"}},
		{"HTTPS://WWW.Customer.Example", []string{"https://customer.example", "https://*.customer.example"}},
		{"http://customer.example", []string{"http://customer.example", "https://*.customer.example"}},
		{"https://acme.github.io", []string{"https://acme.github.io", "https://*.acme.github.io"}},
		{"https://app.customer.example", []string{"https://app.customer.example", "https://*.app.customer.example"}},
		{"https://github.io", []string{"https://github.io"}},
		{"http://localhost:3000", []string{"http://localhost:3000"}},
		{"https://customer.example:8443", []string{"https://customer.example:8443"}},
		{"", nil},
		{"   ", nil},
		{"ftp://customer.example", nil},
		{"https://", nil},
	} {
		t.Run(tc.website, func(t *testing.T) {
			got := WebsiteDefaults(tc.website)
			if len(got) != len(tc.want) {
				t.Fatalf("WebsiteDefaults(%q) = %q, want %q", tc.website, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("WebsiteDefaults(%q) = %q, want %q", tc.website, got, tc.want)
				}
			}
		})
	}
}

func TestNormalizeRejectsInvalidTauriOrigins(t *testing.T) {
	for _, origin := range []string{"tauri://", "tauri://other", "tauri://localhoſt", "tauri://localhost.evil", "tauri://localhost:1420", "tauri://user@localhost", "tauri://localhost/path", "tauri://localhost/", "tauri://localhost?", "tauri://localhost#", "tauri://*", "tauri://localhost tauri://other", " tauri://localhost", "app://localhost", "file:///app", "null"} {
		t.Run(origin, func(t *testing.T) {
			if _, err := Normalize(origin); err == nil {
				t.Fatalf("accepted %q", origin)
			}
		})
	}
}
