package config

import "testing"

func TestGmailOAuthRedirectURLDefaultsUnderAppBaseURL(t *testing.T) {
	t.Setenv("GMAIL_OAUTH_REDIRECT_URL", "")
	if got := gmailOAuthRedirectURL("https://helpin.example.com/"); got != "https://helpin.example.com/api/crm/email/oauth/callback" {
		t.Fatalf("default redirect = %q", got)
	}
	t.Setenv("GMAIL_OAUTH_REDIRECT_URL", " https://api.example.com/api/crm/email/oauth/callback ")
	if got := gmailOAuthRedirectURL("https://helpin.example.com"); got != "https://api.example.com/api/crm/email/oauth/callback" {
		t.Fatalf("explicit redirect = %q", got)
	}
}
