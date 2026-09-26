package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Helpin pages a GitHub App flow (create or install) returns the browser to.
const (
	GitHubReturnSettings     = "settings"
	GitHubReturnSetup        = "setup"
	GitHubReturnSystemStatus = "system_status"
	// GitHubReturnOnboarding returns to the Connect GitHub step of workspace
	// onboarding (/onboarding?step=github&workspace=<slug>).
	GitHubReturnOnboarding = "onboarding"
)

// GitHub flow results reported to the frontend in the github query flag.
const (
	GitHubResultConnected = "connected"
	GitHubResultCreated   = "created"
	GitHubResultError     = "error"
)

const (
	// _gitHubResultQueryKey is the query flag every GitHub App redirect uses.
	_gitHubResultQueryKey = "github"
	// _gitHubMessageQueryKey carries a human-readable result message.
	_gitHubMessageQueryKey = "github_message"
	// _gitHubInstalledPath is the frontend route that finishes installs that
	// arrive without Helpin state and reports flows without a return page.
	_gitHubInstalledPath = "/github/installed"
	_defaultAppBaseURL   = "http://localhost:5173"
)

// ErrGitHubReturnToInvalid reports an unknown return_to value.
var ErrGitHubReturnToInvalid = errors.New("return_to must be settings, setup, system_status or onboarding")

// NormalizeGitHubReturnTo validates return_to. Empty means settings.
func NormalizeGitHubReturnTo(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", GitHubReturnSettings:
		return GitHubReturnSettings, nil
	case GitHubReturnSetup:
		return GitHubReturnSetup, nil
	case GitHubReturnSystemStatus:
		return GitHubReturnSystemStatus, nil
	case GitHubReturnOnboarding:
		return GitHubReturnOnboarding, nil
	default:
		return "", ErrGitHubReturnToInvalid
	}
}

// gitHubReturnTo maps a stored claim to a destination; tokens issued before
// return_to existed, or with an unknown value, return to settings.
func gitHubReturnTo(claim string) string {
	returnTo, err := NormalizeGitHubReturnTo(claim)
	if err != nil {
		return GitHubReturnSettings
	}
	return returnTo
}

// gitHubReturnPageURL is the workspace page for returnTo. The onboarding page
// carries its step and workspace in the query, so callers add result flags
// with withGitHubReturnQuery rather than by appending "?".
func gitHubReturnPageURL(appBaseURL, workspaceSlug, returnTo string) string {
	base := appBaseOrDefault(appBaseURL)
	slug := url.PathEscape(strings.TrimSpace(workspaceSlug))
	switch gitHubReturnTo(returnTo) {
	case GitHubReturnOnboarding:
		query := url.Values{}
		query.Set("step", "github")
		query.Set("workspace", strings.TrimSpace(workspaceSlug))
		return fmt.Sprintf("%s/onboarding?%s", base, query.Encode())
	case GitHubReturnSetup:
		return fmt.Sprintf("%s/w/%s/setup", base, slug)
	case GitHubReturnSystemStatus:
		return fmt.Sprintf("%s/w/%s/settings/system-status", base, slug)
	default:
		return fmt.Sprintf("%s/w/%s/settings/git-connections", base, slug)
	}
}

// gitHubInstalledURL is the frontend route for results without a known
// workspace and for installs that arrive without Helpin state.
func gitHubInstalledURL(appBaseURL string, query url.Values) string {
	target := appBaseOrDefault(appBaseURL) + _gitHubInstalledPath
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return target
}

// withGitHubReturnQuery adds query to target, keeping any query target
// already has (the onboarding page's step and workspace).
func withGitHubReturnQuery(target string, query url.Values) string {
	parsed, err := url.Parse(target)
	if err != nil {
		return target
	}
	merged := parsed.Query()
	for key, values := range query {
		merged[key] = values
	}
	parsed.RawQuery = merged.Encode()
	return parsed.String()
}

// gitHubResultQuery builds the github/github_message result flags.
func gitHubResultQuery(status, message string) url.Values {
	query := url.Values{}
	query.Set(_gitHubResultQueryKey, status)
	if strings.TrimSpace(message) != "" {
		query.Set(_gitHubMessageQueryKey, message)
	}
	return query
}

func appBaseOrDefault(appBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if base == "" {
		return _defaultAppBaseURL
	}
	return base
}

// GitHubAppBaseURLBlockedReason explains why GitHub cannot reach appBaseURL
// (webhooks, the App callback and the setup URL all use it), or returns ""
// when it is an https URL on a public hostname or public IP address.
func GitHubAppBaseURLBlockedReason(appBaseURL string) string {
	return PublicBaseURLBlockedReason("GitHub", appBaseURL)
}
