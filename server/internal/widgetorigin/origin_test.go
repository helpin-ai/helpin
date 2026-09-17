package widgetorigin

import "testing"

func TestAllowed(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		list         []string
		want         bool
	}{
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
