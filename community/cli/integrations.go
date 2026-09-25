package main

import (
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Secrets are never accepted as command-line values: argv is visible to other
// local users and shell history. Use a file, an environment variable, or the
// hidden interactive prompt.
const (
	smtpPasswordEnv = "HELPIN_SMTP_PASSWORD"
	aiKeyEnv        = "HELPIN_AI_API_KEY"
)

var aiProviderKeys = map[string]string{
	"openrouter": "OPENROUTER_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
}

var (
	smtpHostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	plainEnvPattern = regexp.MustCompile(`^[A-Za-z0-9@._+:/,=-]*$`)
)

// smtpSettings is a validated application mail configuration to write.
// password is written only when passwordSet; otherwise the stored one is kept.
type smtpSettings struct {
	host, port, username, password, from, tls string
	passwordSet                               bool
}

// aiSettings is a validated provider key to write.
type aiSettings struct {
	provider, key string
}

// integrations collects optional application mail, AI provider, meeting
// capture and Google (Gmail and Calendar) settings.
// With --yes nothing is prompted: omitted flags leave the settings unchanged.
func (a *app) integrations(o *options, old map[string]string) error {
	if err := a.smtpIntegration(o, old); err != nil {
		return err
	}
	if err := a.aiIntegration(o, old); err != nil {
		return err
	}
	if err := a.meetingIntegration(o, old); err != nil {
		return err
	}
	return a.googleIntegration(o, old)
}

func (a *app) smtpIntegration(o *options, old map[string]string) error {
	host := strings.TrimSpace(o.smtpHost)
	if host == "" && !o.yes {
		fallback := plainEnv(old["SMTP_HOST"])
		if fallback == "" {
			fallback = "skip"
		}
		fmt.Fprintln(a.out, "\nApplication email sends invitations and password resets. Enter skip to configure it later.")
		answer, err := a.ask("SMTP host", fallback)
		if err != nil {
			return err
		}
		host = strings.TrimSpace(answer)
	}
	if host == "" || strings.EqualFold(host, "skip") {
		return nil
	}
	s := smtpSettings{host: host, port: o.smtpPort, username: o.smtpUser, from: o.smtpFrom, tls: o.smtpTLS}
	defaults := map[*string]string{
		&s.port:     firstValue(plainEnv(old["SMTP_PORT"]), "587"),
		&s.username: plainEnv(old["SMTP_USERNAME"]),
		&s.from:     plainEnv(old["SMTP_FROM"]),
		&s.tls:      firstValue(plainEnv(old["SMTP_TLS_MODE"]), "starttls"),
	}
	prompts := []struct {
		value *string
		label string
	}{
		{&s.port, "SMTP port"}, {&s.tls, "SMTP TLS mode: starttls / tls / none"},
		{&s.username, "SMTP username (blank for an unauthenticated relay)"}, {&s.from, "Sender address (SMTP_FROM)"},
	}
	for _, prompt := range prompts {
		if *prompt.value != "" {
			continue
		}
		if o.yes {
			*prompt.value = defaults[prompt.value]
			continue
		}
		answer, err := a.ask(prompt.label, defaults[prompt.value])
		if err != nil {
			return err
		}
		*prompt.value = strings.TrimSpace(answer)
	}
	password, supplied, err := a.secretValue(o.smtpPasswordFile, smtpPasswordEnv)
	if err != nil {
		return err
	}
	if !supplied && s.username != "" && !o.yes {
		label := "SMTP password (input hidden)"
		if plainEnv(old["SMTP_PASSWORD"]) != "" {
			label = "SMTP password (input hidden; blank keeps the current one)"
		}
		if password, err = a.readSecret(label); err != nil {
			return err
		}
		supplied = password != ""
	}
	s.password, s.passwordSet = password, supplied
	if err := s.validate(plainEnv(old["SMTP_PASSWORD"]) != ""); err != nil {
		return err
	}
	o.smtp = &s
	return nil
}

func (s smtpSettings) validate(hasStoredPassword bool) error {
	if !smtpHostPattern.MatchString(s.host) && net.ParseIP(s.host) == nil {
		return fmt.Errorf("invalid SMTP host %q; enter a hostname or IP without a scheme or port", s.host)
	}
	if port, err := strconv.Atoi(s.port); err != nil || port < 1 || port > 65535 {
		return errors.New("SMTP port must be between 1 and 65535")
	}
	switch s.tls {
	case "starttls", "tls", "none":
	default:
		return errors.New("SMTP TLS mode must be starttls, tls or none")
	}
	if strings.ContainsAny(s.username, " \t\r\n") {
		return errors.New("SMTP username must not contain spaces")
	}
	if s.username != "" && s.tls == "none" {
		return errors.New("SMTP credentials require TLS; use starttls or tls")
	}
	if s.username != "" && !s.passwordSet && !hasStoredPassword {
		return fmt.Errorf("SMTP username needs a password: use --smtp-password-file or %s", smtpPasswordEnv)
	}
	if s.username == "" && s.passwordSet {
		return errors.New("an SMTP password needs an SMTP username")
	}
	address, err := mail.ParseAddress(s.from)
	if err != nil || strings.ContainsAny(s.from, "\r\n") || address.Address == "" {
		return errors.New("sender address (SMTP_FROM) must be a valid email address")
	}
	return nil
}

func (a *app) aiIntegration(o *options, old map[string]string) error {
	provider := strings.ToLower(strings.TrimSpace(o.aiProvider))
	if provider == "" && !o.yes {
		fmt.Fprintln(a.out, "\nAI features need a provider API key. Enter skip to connect one later in Settings → AI.")
		answer, err := a.ask("AI provider: openrouter / openai / anthropic / skip", "skip")
		if err != nil {
			return err
		}
		provider = strings.ToLower(strings.TrimSpace(answer))
	}
	if provider == "" || provider == "skip" || provider == "none" {
		return nil
	}
	envKey, ok := aiProviderKeys[provider]
	if !ok {
		return errors.New("AI provider must be openrouter, openai, anthropic or skip")
	}
	key, supplied, err := a.secretValue(o.aiKeyFile, aiKeyEnv)
	if err != nil {
		return err
	}
	stored := plainEnv(old[envKey]) != ""
	if !supplied && !o.yes {
		label := "API key (input hidden)"
		if stored {
			label = "API key (input hidden; blank keeps the current one)"
		}
		if key, err = a.readSecret(label); err != nil {
			return err
		}
		supplied = key != ""
	}
	if !supplied {
		if stored {
			return nil
		}
		return fmt.Errorf("an API key is required for %s: use --ai-key-file or %s", provider, aiKeyEnv)
	}
	if len(key) < 8 || len(key) > 512 || strings.ContainsAny(key, " \t\r\n'\"") {
		return errors.New("the API key has an unexpected format")
	}
	o.ai = &aiSettings{provider: provider, key: key}
	if provider == "anthropic" {
		fmt.Fprintln(a.out, "Note: Anthropic has no embeddings API; knowledge search needs an OpenAI or OpenRouter key.")
	}
	return nil
}

// secretValue reads a secret from a file (first line) or an environment variable.
func (a *app) secretValue(file, env string) (string, bool, error) {
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", false, fmt.Errorf("cannot read secret file: %w", err)
		}
		value := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
		if value == "" {
			return "", false, fmt.Errorf("secret file %s is empty", filepath.Base(file))
		}
		return value, true, nil
	}
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		return value, true, nil
	}
	return "", false, nil
}

func (a *app) readSecret(label string) (string, error) {
	if a.secret == nil || !a.interactive {
		return "", errors.New("cannot read a hidden value here; use the matching --*-file flag or environment variable")
	}
	value, err := a.secret(label)
	return strings.TrimSpace(value), err
}

// terminalSecret reads one line with terminal echo disabled. Echo is restored
// on return and on interrupt.
func terminalSecret(a *app) func(string) (string, error) {
	return func(label string) (string, error) {
		stty := func(arg string) error {
			cmd := exec.Command("stty", arg)
			cmd.Stdin = os.Stdin
			return cmd.Run()
		}
		if err := stty("-echo"); err != nil {
			return "", errors.New("cannot hide terminal input; use the matching --*-file flag or environment variable")
		}
		interrupted := make(chan os.Signal, 1)
		done := make(chan struct{})
		signal.Notify(interrupted, os.Interrupt)
		go func() {
			select {
			case <-interrupted:
				stty("echo")
				fmt.Fprintln(a.out)
				os.Exit(130)
			case <-done:
			}
		}()
		defer func() {
			signal.Stop(interrupted)
			close(done)
			stty("echo")
			fmt.Fprintln(a.out)
		}()
		fmt.Fprintf(a.out, "%s: ", label)
		line, err := a.in.ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("setup cancelled: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
}

// writeIntegrations stores the collected settings in .env, preserving every
// other line, including generated secrets.
func writeIntegrations(dir string, o options) error {
	values := map[string]string{}
	var order []string
	set := func(key, value string) {
		values[key] = value
		order = append(order, key)
	}
	if o.smtp != nil {
		set("SMTP_HOST", o.smtp.host)
		set("SMTP_PORT", o.smtp.port)
		set("SMTP_USERNAME", o.smtp.username)
		set("SMTP_FROM", o.smtp.from)
		set("SMTP_TLS_MODE", o.smtp.tls)
		if o.smtp.passwordSet || o.smtp.username == "" {
			set("SMTP_PASSWORD", o.smtp.password)
		}
	}
	if o.ai != nil {
		set(aiProviderKeys[o.ai.provider], o.ai.key)
	}
	for _, value := range crmIntegrationValues(o) {
		set(value[0], value[1])
	}
	if len(values) == 0 {
		return nil
	}
	encoded := map[string]string{}
	for key, value := range values {
		value, err := envValue(value)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		encoded[key] = value
	}
	path := filepath.Join(dir, ".env")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		key, _, ok := strings.Cut(line, "=")
		if value, found := encoded[key]; ok && found {
			lines[i] = key + "=" + value
			seen[key] = true
		}
	}
	for _, key := range order {
		if !seen[key] {
			lines = append(lines, key+"="+encoded[key])
			seen[key] = true
		}
	}
	return atomicWrite(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

// envValue encodes a value for Compose's .env parser. Single quotes keep
// characters such as $ and # literal; values that cannot be quoted are rejected.
func envValue(value string) (string, error) {
	if strings.ContainsAny(value, "'\r\n\x00") {
		return "", errors.New("value contains unsupported characters (quote or line break)")
	}
	if plainEnvPattern.MatchString(value) {
		return value, nil
	}
	return "'" + value + "'", nil
}

// plainEnv reverses envValue's quoting for display and comparison.
func plainEnv(value string) string {
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
