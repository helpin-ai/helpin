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
		{"wildcard", "https://customer.example", []string{"*"}, false},
		{"missing", "", []string{"https://customer.example"}, false},
		{"opaque", "null", []string{"null"}, false},
		{"credentials", "https://user@customer.example", []string{"https://customer.example"}, false},
		{"path", "https://customer.example/", []string{"https://customer.example"}, false},
		{"multiple", "https://customer.example https://evil.example", []string{"https://customer.example"}, false},
		{"query", "https://customer.example?", []string{"https://customer.example"}, false},
		{"fragment", "https://customer.example#", []string{"https://customer.example"}, false},
		{"ipv6", "http://[::1]:80", []string{"http://[::1]"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Allowed(tc.origin, tc.list); got != tc.want {
				t.Fatalf("Allowed = %v, want %v", got, tc.want)
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
