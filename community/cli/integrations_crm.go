package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Secrets for meeting capture and Google come from files, environment
// variables or hidden prompts, never from command-line values.
const (
	meetingKeyEnv    = "HELPIN_MEETING_API_KEY"
	meetingSecretEnv = "HELPIN_MEETING_WEBHOOK_SECRET"
	googleSecretEnv  = "HELPIN_GOOGLE_CLIENT_SECRET"
	defaultVexaURL   = "https://api.cloud.vexa.ai"
	// googleCallbackPath must match the API's Gmail and Calendar OAuth callback.
	googleCallbackPath = "/api/crm/email/oauth/callback"
)

var googleClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,200}\.apps\.googleusercontent\.com$`)

// meetingProviders maps each capture provider to its display name and the
// .env keys for its API key and webhook secret.
var meetingProviders = map[string]struct{ title, keyEnv, secretEnv string }{
	"recall": {"Recall", "RECALL_API_KEY", "RECALL_WEBHOOK_SECRET"},
	"vexa":   {"Vexa", "VEXA_API_KEY", "VEXA_WEBHOOK_SECRET"},
}

// meetingSettings is a validated capture provider configuration to write. The
// key and secret are written only when set; otherwise the stored ones are kept.
type meetingSettings struct {
	provider, key, secret, vexaURL string
	keySet, secretSet              bool
}

// googleSettings is a validated Google OAuth client to write.
type googleSettings struct {
	clientID, secret string
	secretSet        bool
}

// publicURL is the dashboard address these settings will run under.
func publicURL(o *options, old map[string]string) string {
	switch {
	case o.mode == "server" && o.domain != "":
		return "https://" + o.domain
	case o.mode == "local" && o.port != 0:
		return fmt.Sprintf("http://localhost:%d", o.port)
	default:
		return firstValue(plainEnv(old["APP_BASE_URL"]), "http://localhost:8085")
	}
}

func (a *app) meetingIntegration(o *options, old map[string]string) error {
	provider := strings.ToLower(strings.TrimSpace(o.meetingProvider))
	if provider == "" && !o.yes {
		fallback := "skip"
		if stored := firstValue(plainEnv(old["CRM_MEETING_CAPTURE_PROVIDER"]), "recall"); plainEnv(old[meetingProviders[stored].keyEnv]) != "" {
			fallback = stored
		}
		fmt.Fprintln(a.out, "\nMeeting capture sends a notetaker to calls through Recall (hosted) or Vexa (open source, hosted or self-hosted). Enter skip to set it up later.")
		answer, err := a.ask("Meeting capture: skip / recall / vexa", fallback)
		if err != nil {
			return err
		}
		provider = strings.ToLower(strings.TrimSpace(answer))
	}
	if provider == "" || provider == "skip" || provider == "none" {
		return nil
	}
	spec, ok := meetingProviders[provider]
	if !ok {
		return errors.New("meeting capture provider must be recall, vexa or skip")
	}
	m := meetingSettings{provider: provider}
	var err error
	if m.key, m.keySet, err = a.providerSecret(o, old, secretPrompt{
		file: o.meetingKeyFile, env: meetingKeyEnv, envKey: spec.keyEnv, flag: "--meeting-key-file",
		label: spec.title + " API key", what: "an API key for " + spec.title,
	}); err != nil {
		return err
	}
	if m.secret, m.secretSet, err = a.providerSecret(o, old, secretPrompt{
		file: o.meetingWebhookSecretFile, env: meetingSecretEnv, envKey: spec.secretEnv, flag: "--meeting-webhook-secret-file",
		label: spec.title + " webhook secret", what: "a webhook secret for " + spec.title + " (Helpin rejects unsigned events)",
	}); err != nil {
		return err
	}
	if m.keySet {
		if err := validateToken(m.key, "the meeting API key"); err != nil {
			return err
		}
	}
	if m.secretSet {
		if err := validateWebhookSecret(provider, m.secret); err != nil {
			return err
		}
	}
	if provider == "vexa" {
		if m.vexaURL, err = a.vexaURL(o, old); err != nil {
			return err
		}
	}
	o.meeting = &m
	base := publicURL(o, old)
	fmt.Fprintf(a.out, "Register %s/api/webhooks/meeting-capture/%s as the %s webhook.\n", base, provider, spec.title)
	if !strings.HasPrefix(base, "https://") {
		fmt.Fprintf(a.out, "Note: %s delivers events only to a public https address; captures cannot finish until this server uses one (helpin configure --mode server).\n", spec.title)
	}
	return nil
}

func (a *app) vexaURL(o *options, old map[string]string) (string, error) {
	value := strings.TrimSpace(o.vexaURL)
	fallback := firstValue(plainEnv(old["VEXA_BASE_URL"]), defaultVexaURL)
	if value == "" && !o.yes {
		answer, err := a.ask("Vexa API URL (your server's address for a self-hosted Vexa)", fallback)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(answer)
	}
	value = strings.TrimRight(firstValue(value, fallback), "/")
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid Vexa URL %q; enter an http or https address such as %s", value, defaultVexaURL)
	}
	return value, nil
}

func (a *app) googleIntegration(o *options, old map[string]string) error {
	clientID := strings.TrimSpace(o.googleClientID)
	redirect := publicURL(o, old) + googleCallbackPath
	if clientID == "" && !o.yes {
		fmt.Fprintf(a.out, "\nConnecting Gmail and Google Calendar needs a Google OAuth client (web application) with redirect URI %s. Enter skip to set it up later.\n", redirect)
		answer, err := a.ask("Google OAuth client ID", firstValue(plainEnv(old["GMAIL_CLIENT_ID"]), "skip"))
		if err != nil {
			return err
		}
		clientID = strings.TrimSpace(answer)
	}
	if clientID == "" || strings.EqualFold(clientID, "skip") {
		return nil
	}
	if !googleClientIDPattern.MatchString(clientID) {
		return errors.New("the Google OAuth client ID must end with .apps.googleusercontent.com")
	}
	secret, secretSet, err := a.providerSecret(o, old, secretPrompt{
		file: o.googleClientSecretFile, env: googleSecretEnv, envKey: "GMAIL_CLIENT_SECRET", flag: "--google-client-secret-file",
		label: "Google OAuth client secret", what: "the Google OAuth client secret",
	})
	if err != nil {
		return err
	}
	if secretSet {
		if err := validateToken(secret, "the Google OAuth client secret"); err != nil {
			return err
		}
	}
	o.google = &googleSettings{clientID: clientID, secret: secret, secretSet: secretSet}
	fmt.Fprintf(a.out, "Add %s to the OAuth client's authorized redirect URIs.\n", redirect)
	return nil
}

// secretPrompt describes one required provider secret and where it can come from.
type secretPrompt struct {
	file, env, envKey, flag, label, what string
}

// providerSecret reads a required secret from its file, environment variable or
// a hidden prompt. It reports false when the stored value is kept.
func (a *app) providerSecret(o *options, old map[string]string, p secretPrompt) (string, bool, error) {
	value, supplied, err := a.secretValue(p.file, p.env)
	if err != nil {
		return "", false, err
	}
	stored := plainEnv(old[p.envKey]) != ""
	if !supplied && !o.yes {
		label := p.label + " (input hidden)"
		if stored {
			label = p.label + " (input hidden; blank keeps the current one)"
		}
		if value, err = a.readSecret(label); err != nil {
			return "", false, err
		}
		supplied = value != ""
	}
	if !supplied && !stored {
		return "", false, fmt.Errorf("%s is required: use %s or %s", p.what, p.flag, p.env)
	}
	return value, supplied, nil
}

func validateToken(value, name string) error {
	if len(value) < 8 || len(value) > 512 || strings.ContainsAny(value, " \t\r\n'\"") {
		return fmt.Errorf("%s has an unexpected format", name)
	}
	return nil
}

// validateWebhookSecret applies the provider's signing rules: Recall (Svix)
// secrets are base64 keys, usually prefixed whsec_; Vexa uses the secret you
// registered with its webhook.
func validateWebhookSecret(provider, secret string) error {
	if err := validateToken(secret, "the webhook secret"); err != nil {
		return err
	}
	if provider == "recall" {
		if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_")); err != nil {
			return errors.New("the Recall webhook secret must be the whsec_ signing secret from the Recall dashboard")
		}
	}
	if provider == "vexa" && len(secret) < 16 {
		return errors.New("use a Vexa webhook secret of at least 16 characters (for example: openssl rand -hex 32)")
	}
	return nil
}

// crmIntegrationValues lists the .env values for collected meeting and Google
// settings, in a stable order.
func crmIntegrationValues(o options) [][2]string {
	var values [][2]string
	if m := o.meeting; m != nil {
		spec := meetingProviders[m.provider]
		values = append(values, [2]string{"CRM_MEETING_CAPTURE_PROVIDER", m.provider})
		if m.keySet {
			values = append(values, [2]string{spec.keyEnv, m.key})
		}
		if m.secretSet {
			values = append(values, [2]string{spec.secretEnv, m.secret})
		}
		if m.provider == "vexa" {
			values = append(values, [2]string{"VEXA_BASE_URL", m.vexaURL})
		}
	}
	if g := o.google; g != nil {
		values = append(values, [2]string{"GMAIL_CLIENT_ID", g.clientID})
		if g.secretSet {
			values = append(values, [2]string{"GMAIL_CLIENT_SECRET", g.secret})
		}
		// Empty lets the server derive the redirect from APP_BASE_URL, so it
		// stays correct when the domain changes.
		values = append(values, [2]string{"GMAIL_OAUTH_REDIRECT_URL", ""})
	}
	return values
}
