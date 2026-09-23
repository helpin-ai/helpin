package service

import (
	"errors"
	"net/url"
	"testing"
)

func TestNormalizeGitHubReturnTo(t *testing.T) {
	tests := []struct {
		value   string
		want    string
		wantErr bool
	}{
		{value: "", want: GitHubReturnSettings},
		{value: "settings", want: GitHubReturnSettings},
		{value: " Setup ", want: GitHubReturnSetup},
		{value: "system_status", want: GitHubReturnSystemStatus},
		{value: "onboarding", want: GitHubReturnOnboarding},
		{value: "ONBOARDING", want: GitHubReturnOnboarding},
		{value: "elsewhere", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := NormalizeGitHubReturnTo(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrGitHubReturnToInvalid) {
					t.Fatalf("NormalizeGitHubReturnTo(%q) error = %v, want ErrGitHubReturnToInvalid", tt.value, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("NormalizeGitHubReturnTo(%q) = %q, %v; want %q", tt.value, got, err, tt.want)
			}
		})
	}
}

func TestGitHubReturnPageURL(t *testing.T) {
	tests := []struct {
		returnTo string
		want     string
	}{
		{returnTo: "", want: "https://helpin.example.com/w/acme/settings/git-connections"},
		{returnTo: GitHubReturnSetup, want: "https://helpin.example.com/w/acme/setup"},
		{returnTo: GitHubReturnSystemStatus, want: "https://helpin.example.com/w/acme/settings/system-status"},
		{returnTo: GitHubReturnOnboarding, want: "https://helpin.example.com/onboarding?step=github&workspace=acme"},
		{returnTo: "unknown", want: "https://helpin.example.com/w/acme/settings/git-connections"},
	}
	for _, tt := range tests {
		t.Run(tt.returnTo, func(t *testing.T) {
			if got := gitHubReturnPageURL("https://helpin.example.com/", "acme", tt.returnTo); got != tt.want {
				t.Fatalf("gitHubReturnPageURL(%q) = %q, want %q", tt.returnTo, got, tt.want)
			}
		})
	}
}

func TestGitHubReturnPageURLEscapesOnboardingWorkspace(t *testing.T) {
	got, err := url.Parse(gitHubReturnPageURL("https://helpin.example.com", "a&b=c", GitHubReturnOnboarding))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Query().Get("workspace") != "a&b=c" || got.Query().Get("step") != "github" {
		t.Fatalf("unexpected onboarding url %q", got.String())
	}
}

func TestWithGitHubReturnQueryKeepsExistingQuery(t *testing.T) {
	target := gitHubReturnPageURL("https://helpin.example.com", "acme", GitHubReturnOnboarding)
	got, err := url.Parse(withGitHubReturnQuery(target, gitHubResultQuery(GitHubResultCreated, "Install it.")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	query := got.Query()
	if got.Path != "/onboarding" || query.Get("step") != "github" || query.Get("workspace") != "acme" ||
		query.Get("github") != GitHubResultCreated || query.Get("github_message") != "Install it." {
		t.Fatalf("unexpected url %q", got.String())
	}

	plain, _ := url.Parse(withGitHubReturnQuery("https://helpin.example.com/w/acme/setup", gitHubResultQuery(GitHubResultError, "")))
	if plain.Path != "/w/acme/setup" || plain.RawQuery != "github=error" {
		t.Fatalf("unexpected url %q", plain.String())
	}
}
