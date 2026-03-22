package crawler

import (
	"encoding/json"
	"testing"
)

func TestCfRecordText(t *testing.T) {
	tests := []struct {
		name   string
		record CloudflareCrawlRecord
		want   string
	}{
		{
			name: "prefers markdown over HTML",
			record: CloudflareCrawlRecord{
				Markdown: "# Hello World",
				HTML:     "<h1>Hello World</h1>",
			},
			want: "# Hello World",
		},
		{
			name: "falls back to HTML when no markdown",
			record: CloudflareCrawlRecord{
				Markdown: "",
				HTML:     "<h1>Hello World</h1>",
			},
			want: "<h1>Hello World</h1>",
		},
		{
			name: "returns empty for empty content",
			record: CloudflareCrawlRecord{
				Markdown: "",
				HTML:     "",
			},
			want: "",
		},
		{
			name: "whitespace-only markdown falls through to HTML",
			record: CloudflareCrawlRecord{
				Markdown: "   \n  ",
				HTML:     "<p>content</p>",
			},
			want: "<p>content</p>",
		},
		{
			name: "JSON content used when no markdown",
			record: CloudflareCrawlRecord{
				Markdown: "",
				HTML:     "",
				JSON:     json.RawMessage(`{"key":"value"}`),
			},
			want: "{\n  \"key\": \"value\"\n}",
		},
		{
			name: "null JSON is skipped",
			record: CloudflareCrawlRecord{
				Markdown: "",
				HTML:     "<p>fallback</p>",
				JSON:     json.RawMessage(`null`),
			},
			want: "<p>fallback</p>",
		},
		{
			name: "markdown trimmed of whitespace",
			record: CloudflareCrawlRecord{
				Markdown: "  # Title  \n",
			},
			want: "# Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfRecordText(tt.record)
			if got != tt.want {
				t.Errorf("cfRecordText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCfRecordTitle(t *testing.T) {
	tests := []struct {
		name   string
		record CloudflareCrawlRecord
		want   string
	}{
		{
			name: "title from metadata",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{"title": "My Page Title"},
			},
			want: "My Page Title",
		},
		{
			name: "fallback to URL when no metadata title",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{},
			},
			want: "https://example.com/page",
		},
		{
			name: "fallback to URL when metadata is nil",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: nil,
			},
			want: "https://example.com/page",
		},
		{
			name: "fallback to URL when title is empty string",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{"title": ""},
			},
			want: "https://example.com/page",
		},
		{
			name: "fallback to URL when title is whitespace only",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{"title": "   "},
			},
			want: "https://example.com/page",
		},
		{
			name: "title trimmed of whitespace",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{"title": "  My Title  "},
			},
			want: "My Title",
		},
		{
			name: "non-string title falls back to URL",
			record: CloudflareCrawlRecord{
				URL:      "https://example.com/page",
				Metadata: map[string]any{"title": 123},
			},
			want: "https://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfRecordTitle(tt.record)
			if got != tt.want {
				t.Errorf("cfRecordTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCfRecordHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		record CloudflareCrawlRecord
		want   int
	}{
		{
			name: "float64 status",
			record: CloudflareCrawlRecord{
				Metadata: map[string]any{"status": float64(200)},
			},
			want: 200,
		},
		{
			name: "int status",
			record: CloudflareCrawlRecord{
				Metadata: map[string]any{"status": 404},
			},
			want: 404,
		},
		{
			name: "missing status returns zero",
			record: CloudflareCrawlRecord{
				Metadata: map[string]any{},
			},
			want: 0,
		},
		{
			name: "nil metadata returns zero",
			record: CloudflareCrawlRecord{
				Metadata: nil,
			},
			want: 0,
		},
		{
			name: "string status returns zero",
			record: CloudflareCrawlRecord{
				Metadata: map[string]any{"status": "200"},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfRecordHTTPStatus(tt.record)
			if got != tt.want {
				t.Errorf("cfRecordHTTPStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewCloudflareCrawlClient(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		apiToken  string
		baseURL   string
		wantNil   bool
	}{
		{
			name:      "returns nil when accountID is empty",
			accountID: "",
			apiToken:  "token123",
			baseURL:   "",
			wantNil:   true,
		},
		{
			name:      "returns nil when apiToken is empty",
			accountID: "account123",
			apiToken:  "",
			baseURL:   "",
			wantNil:   true,
		},
		{
			name:      "returns nil when both empty",
			accountID: "",
			apiToken:  "",
			baseURL:   "",
			wantNil:   true,
		},
		{
			name:      "returns client when both set",
			accountID: "account123",
			apiToken:  "token123",
			baseURL:   "",
			wantNil:   false,
		},
		{
			name:      "returns nil when accountID is whitespace only",
			accountID: "   ",
			apiToken:  "token123",
			baseURL:   "",
			wantNil:   true,
		},
		{
			name:      "returns nil when apiToken is whitespace only",
			accountID: "account123",
			apiToken:  "   ",
			baseURL:   "",
			wantNil:   true,
		},
		{
			name:      "custom baseURL is used",
			accountID: "account123",
			apiToken:  "token123",
			baseURL:   "https://custom-api.example.com/v1",
			wantNil:   false,
		},
		{
			name:      "default baseURL when empty",
			accountID: "account123",
			apiToken:  "token123",
			baseURL:   "",
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCloudflareCrawlClient(tt.accountID, tt.apiToken, tt.baseURL)
			if tt.wantNil && client != nil {
				t.Errorf("NewCloudflareCrawlClient() = %v, want nil", client)
			}
			if !tt.wantNil && client == nil {
				t.Error("NewCloudflareCrawlClient() = nil, want non-nil client")
			}
			if !tt.wantNil && client != nil {
				if tt.baseURL != "" && client.baseURL != tt.baseURL {
					t.Errorf("client.baseURL = %q, want %q", client.baseURL, tt.baseURL)
				}
				if tt.baseURL == "" && client.baseURL != "https://api.cloudflare.com/client/v4" {
					t.Errorf("client.baseURL = %q, want default", client.baseURL)
				}
			}
		})
	}
}

func TestFlexString_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string value",
			input: `"hello"`,
			want:  "hello",
		},
		{
			name:  "integer number value",
			input: `42`,
			want:  "42",
		},
		{
			name:  "float number value",
			input: `3.14`,
			want:  "3.14",
		},
		{
			name:  "null value returns empty string",
			input: `null`,
			want:  "",
		},
		{
			name:  "empty string",
			input: `""`,
			want:  "",
		},
		{
			name:  "boolean value returns empty string",
			input: `true`,
			want:  "",
		},
		{
			name:  "array value returns empty string",
			input: `[1,2,3]`,
			want:  "",
		},
		{
			name:  "object value returns empty string",
			input: `{"key":"val"}`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fs flexString
			err := fs.UnmarshalJSON([]byte(tt.input))
			if err != nil {
				t.Fatalf("UnmarshalJSON(%s) returned error: %v", tt.input, err)
			}
			if string(fs) != tt.want {
				t.Errorf("UnmarshalJSON(%s) = %q, want %q", tt.input, string(fs), tt.want)
			}
		})
	}
}

func TestFlexString_InStruct(t *testing.T) {
	// Test that flexString works correctly when embedded in a struct via json.Unmarshal.
	type wrapper struct {
		Cursor flexString `json:"cursor"`
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string cursor in struct",
			input: `{"cursor":"abc123"}`,
			want:  "abc123",
		},
		{
			name:  "numeric cursor in struct",
			input: `{"cursor":99}`,
			want:  "99",
		},
		{
			name:  "null cursor in struct",
			input: `{"cursor":null}`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w wrapper
			if err := json.Unmarshal([]byte(tt.input), &w); err != nil {
				t.Fatalf("json.Unmarshal(%s) error: %v", tt.input, err)
			}
			if string(w.Cursor) != tt.want {
				t.Errorf("Cursor = %q, want %q", string(w.Cursor), tt.want)
			}
		})
	}
}

func TestNormalizePurposes(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty input returns defaults",
			input: nil,
			want:  []string{"search", "ai-input"},
		},
		{
			name:  "valid purposes preserved",
			input: []string{"search", "ai-train"},
			want:  []string{"search", "ai-train"},
		},
		{
			name:  "invalid purposes filtered out",
			input: []string{"search", "invalid", "ai-input"},
			want:  []string{"search", "ai-input"},
		},
		{
			name:  "all invalid returns defaults",
			input: []string{"invalid1", "invalid2"},
			want:  []string{"search", "ai-input"},
		},
		{
			name:  "whitespace trimmed",
			input: []string{"  search  ", " ai-input "},
			want:  []string{"search", "ai-input"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePurposes(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("normalizePurposes() returned %d elements %v, want %d elements %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("normalizePurposes()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeFormats(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty input returns default markdown",
			input: nil,
			want:  []string{"markdown"},
		},
		{
			name:  "valid formats preserved",
			input: []string{"html", "json"},
			want:  []string{"html", "json"},
		},
		{
			name:  "invalid formats filtered out",
			input: []string{"html", "xml", "markdown"},
			want:  []string{"html", "markdown"},
		},
		{
			name:  "all invalid returns default",
			input: []string{"pdf", "xml"},
			want:  []string{"markdown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFormats(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeFormats() returned %d elements %v, want %d elements %v", len(got), got, len(tt.want), tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeFormats()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSliceContains(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
		want     bool
	}{
		{
			name:     "element present",
			values:   []string{"a", "b", "c"},
			expected: "b",
			want:     true,
		},
		{
			name:     "element not present",
			values:   []string{"a", "b", "c"},
			expected: "d",
			want:     false,
		},
		{
			name:     "empty slice",
			values:   nil,
			expected: "a",
			want:     false,
		},
		{
			name:     "empty expected in slice with empty string",
			values:   []string{"", "a"},
			expected: "",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceContains(tt.values, tt.expected)
			if got != tt.want {
				t.Errorf("sliceContains(%v, %q) = %v, want %v", tt.values, tt.expected, got, tt.want)
			}
		})
	}
}

func TestJoinCloudflareErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []struct{ Message string `json:"message"` }
		fallback string
		want     string
	}{
		{
			name:     "no errors returns fallback",
			errors:   nil,
			fallback: "500 Internal Server Error",
			want:     "500 Internal Server Error",
		},
		{
			name: "single error message",
			errors: []struct{ Message string `json:"message"` }{
				{Message: "rate limited"},
			},
			fallback: "fallback",
			want:     "rate limited",
		},
		{
			name: "multiple error messages joined with semicolon",
			errors: []struct{ Message string `json:"message"` }{
				{Message: "first error"},
				{Message: "second error"},
			},
			fallback: "fallback",
			want:     "first error; second error",
		},
		{
			name: "empty message errors use fallback",
			errors: []struct{ Message string `json:"message"` }{
				{Message: ""},
				{Message: "   "},
			},
			fallback: "fallback status",
			want:     "fallback status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinCloudflareErrors(tt.errors, tt.fallback)
			if got != tt.want {
				t.Errorf("joinCloudflareErrors() = %q, want %q", got, tt.want)
			}
		})
	}
}
