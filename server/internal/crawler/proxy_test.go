package crawler

import (
	"testing"
)

func TestParseProxyURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only returns nil",
			input: "   ",
			want:  nil,
		},
		{
			name:  "single URL",
			input: "http://user:pass@proxy.example.com:8080",
			want:  []string{"http://user:pass@proxy.example.com:8080"},
		},
		{
			name:  "multiple comma-separated URLs",
			input: "http://proxy1.example.com:8080,http://proxy2.example.com:8081,socks5://proxy3.example.com:1080",
			want: []string{
				"http://proxy1.example.com:8080",
				"http://proxy2.example.com:8081",
				"socks5://proxy3.example.com:1080",
			},
		},
		{
			name:  "whitespace around URLs is trimmed",
			input: "  http://proxy1.example.com:8080 , http://proxy2.example.com:8081  ",
			want: []string{
				"http://proxy1.example.com:8080",
				"http://proxy2.example.com:8081",
			},
		},
		{
			name:  "empty entries between commas are ignored",
			input: "http://proxy1.example.com:8080,,http://proxy2.example.com:8081",
			want: []string{
				"http://proxy1.example.com:8080",
				"http://proxy2.example.com:8081",
			},
		},
		{
			name:  "trailing comma ignored",
			input: "http://proxy1.example.com:8080,",
			want:  []string{"http://proxy1.example.com:8080"},
		},
		{
			name:  "leading comma ignored",
			input: ",http://proxy1.example.com:8080",
			want:  []string{"http://proxy1.example.com:8080"},
		},
		{
			name:  "only commas returns empty slice",
			input: ",,,",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseProxyURLs(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseProxyURLs(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseProxyURLs(%q) returned %d elements, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("ParseProxyURLs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
