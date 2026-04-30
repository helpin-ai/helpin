package model

import "testing"

func TestResolveGitHubAPIBaseURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input *string
		want  string
	}{
		{name: "default github cloud", input: nil, want: "https://api.github.com"},
		{name: "github web host", input: gitStrPtr("https://github.com"), want: "https://api.github.com"},
		{name: "github api host", input: gitStrPtr("https://api.github.com"), want: "https://api.github.com"},
		{name: "enterprise web host", input: gitStrPtr("https://github.acme.com"), want: "https://github.acme.com/api/v3"},
		{name: "enterprise api host preserved", input: gitStrPtr("https://github.acme.com/api/v3"), want: "https://github.acme.com/api/v3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveGitHubAPIBaseURL(tc.input); got != tc.want {
				t.Fatalf("ResolveGitHubAPIBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveGitHubWebBaseURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input *string
		want  string
	}{
		{name: "default github cloud", input: nil, want: "https://github.com"},
		{name: "github web host", input: gitStrPtr("https://github.com"), want: "https://github.com"},
		{name: "github api host maps to web", input: gitStrPtr("https://api.github.com"), want: "https://github.com"},
		{name: "enterprise web host", input: gitStrPtr("https://github.acme.com"), want: "https://github.acme.com"},
		{name: "enterprise api host strips api suffix", input: gitStrPtr("https://github.acme.com/api/v3"), want: "https://github.acme.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveGitHubWebBaseURL(tc.input); got != tc.want {
				t.Fatalf("ResolveGitHubWebBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func gitStrPtr(value string) *string {
	return &value
}
