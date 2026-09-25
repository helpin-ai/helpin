// Helpin's operator CLI manages the versioned Community Compose bundle.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var version = "dev"
var tagPattern = regexp.MustCompile(`^community-v0\.[0-9]+\.[0-9]+(-[a-z0-9.]+)?$`)

type options struct {
	dir, release, bundle, checksum, mode, domain, storage, help, proxy string
	// proxyMode is "builtin" (Caddy in the bundle terminates HTTPS) or
	// "external" (the operator runs their own reverse proxy). Server mode only.
	proxyMode, acmeEmail        string
	project                     string
	port, storagePort, helpPort int
	yes, noStart                bool
	backupPath                  string
	// Optional integrations. Secrets come only from files, environment
	// variables or hidden prompts, never from command-line values.
	smtpHost, smtpPort, smtpUser, smtpFrom, smtpTLS, smtpPasswordFile  string
	aiProvider, aiKeyFile                                              string
	meetingProvider, meetingKeyFile, meetingWebhookSecretFile, vexaURL string
	googleClientID, googleClientSecretFile                             string
	smtp                                                               *smtpSettings
	ai                                                                 *aiSettings
	meeting                                                            *meetingSettings
	google                                                             *googleSettings
}

type app struct {
	in          *bufio.Reader
	out         io.Writer
	interactive bool
	client      *http.Client
	readyWait   time.Duration
	run         func(string, ...string) error
	output      func(string, ...string) ([]byte, error)
	stream      func(string, io.Reader, io.Writer, ...string) error
	// secret reads one value without echoing it (interactive sessions only).
	secret func(string) (string, error)
}

func main() {
	a := &app{in: bufio.NewReader(os.Stdin), out: os.Stdout, client: &http.Client{Timeout: 5 * time.Minute}, readyWait: 30 * time.Second}
	if info, err := os.Stdin.Stat(); err == nil {
		a.interactive = info.Mode()&os.ModeCharDevice != 0
	}
	a.secret = terminalSecret(a)
	a.run = func(dir string, args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, os.Stdin, os.Stdout, os.Stderr
		cmd.Env = commandEnv(dir)
		return cmd.Run()
	}
	a.output = func(dir string, args ...string) ([]byte, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir, cmd.Env = dir, commandEnv(dir)
		return cmd.Output()
	}
	a.stream = func(dir string, in io.Reader, out io.Writer, args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir, cmd.Env = dir, commandEnv(dir)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, os.Stderr
		return cmd.Run()
	}
	if err := a.execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Helpin:", err)
		os.Exit(1)
	}
}

// An operator's unrelated shell Compose variables must not redirect commands
// to another project or override the persistent secrets in this installation.
func commandEnv(dir string) []string {
	keys := map[string]bool{"COMPOSE_FILE": true, "COMPOSE_PROFILES": true, "COMPOSE_PROJECT_NAME": true, "COMPOSE_ENV_FILES": true, "AGENT_RUNTIME_EXECUTION_APP_CONFIG": true}
	if data, err := os.ReadFile(filepath.Join(dir, ".env")); err == nil {
		for key := range envValues(string(data)) {
			keys[key] = true
		}
	}
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !keys[key] {
			env = append(env, entry)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "apps.json")); err == nil {
		env = append(env, "AGENT_RUNTIME_EXECUTION_APP_CONFIG="+string(data))
	}
	return env
}

func (a *app) ask(label, fallback string) (string, error) {
	if !a.interactive {
		return "", fmt.Errorf("%s is required; use flags with --yes for unattended setup", label)
	}
	fmt.Fprintf(a.out, "%s [%s]: ", label, fallback)
	line, err := a.in.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("setup cancelled: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = fallback
	}
	return line, nil
}

func (a *app) execute(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	command := ""
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	if command == "version" || command == "--version" {
		fmt.Fprintln(a.out, "Helpin", version)
		return nil
	}
	if command == "help" || command == "--help" || command == "-h" {
		fmt.Fprintln(a.out, "Helpin Community\n\nUsage: helpin <command> [options]\nCommands: install, start, stop, restart, status, logs, configure, doctor, backup, restore, upgrade, version\nRun helpin <command> --help for options. Default installation: ~/helpin.\nBackup pauses services; restore uses new volumes. Upgrade requires a compatible release and creates a recovery backup.\nSecrets for install/configure come from --*-file flags (--smtp-password-file, --ai-key-file, --meeting-key-file, --meeting-webhook-secret-file, --google-client-secret-file), the matching HELPIN_* environment variables or hidden prompts, never from command-line values.")
		return nil
	}
	if command == "" {
		if !a.interactive {
			return a.execute([]string{"help"})
		}
		fmt.Fprintln(a.out, "Helpin Community\n\n1. Install\n2. Status\n3. Start\n4. Stop\n5. Restart\n6. Logs\n7. Configure\n8. Diagnose\n9. Backup\n10. Restore\n11. Upgrade\n12. Exit")
		choice, e := a.ask("Choose an action", "1")
		if e != nil {
			return e
		}
		if choice == "12" {
			return nil
		}
		commands := map[string]string{"1": "install", "2": "status", "3": "start", "4": "stop", "5": "restart", "6": "logs", "7": "configure", "8": "doctor", "9": "backup", "10": "restore", "11": "upgrade"}
		command = commands[choice]
		if command == "" {
			return errors.New("choose an action from 1 to 12")
		}
	}
	switch command {
	case "install", "start", "stop", "restart", "status", "logs", "configure", "doctor", "backup", "restore", "upgrade":
	default:
		return fmt.Errorf("unknown command %q; run helpin --help", command)
	}
	o := options{}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(a.out)
	f.StringVar(&o.dir, "dir", filepath.Join(home, "helpin"), "installation directory")
	if command == "backup" || command == "restore" || command == "upgrade" {
		f.BoolVar(&o.yes, "yes", false, "confirm the operation without prompting")
		f.StringVar(&o.backupPath, "backup", "", "backup directory (source for restore; new destination for backup/upgrade)")
	}
	if command == "install" || command == "configure" {
		f.BoolVar(&o.yes, "yes", false, "use supplied options and defaults without prompts")
		f.StringVar(&o.mode, "mode", "", "local or server")
		f.StringVar(&o.domain, "domain", "", "dashboard hostname (server mode)")
		f.StringVar(&o.storage, "storage-domain", "", "attachment hostname (server mode)")
		f.StringVar(&o.help, "help-domain", "", "help-center hostname (server mode)")
		f.StringVar(&o.proxyMode, "proxy", "", "server mode HTTPS: builtin (bundled Caddy, default) or external (your own proxy)")
		f.StringVar(&o.acmeEmail, "acme-email", "", "builtin proxy: optional email for certificate expiry notices")
		f.StringVar(&o.proxy, "proxy-cidr", "", "external proxy: exact trusted proxy source IP/CIDR as seen by ingress")
		f.IntVar(&o.port, "port", 0, "local dashboard port (default 8085)")
		f.IntVar(&o.storagePort, "storage-port", 0, "local storage port (default 9005)")
		f.IntVar(&o.helpPort, "help-port", 0, "local help-center port (default 8086)")
		f.StringVar(&o.smtpHost, "smtp-host", "", "application mail SMTP host (omit to leave mail unchanged)")
		f.StringVar(&o.smtpPort, "smtp-port", "", "SMTP port (default 587)")
		f.StringVar(&o.smtpUser, "smtp-username", "", "SMTP username")
		f.StringVar(&o.smtpFrom, "smtp-from", "", "sender address for application mail")
		f.StringVar(&o.smtpTLS, "smtp-tls", "", "SMTP TLS mode: starttls, tls or none")
		f.StringVar(&o.smtpPasswordFile, "smtp-password-file", "", "file containing the SMTP password (or set "+smtpPasswordEnv+")")
		f.StringVar(&o.aiProvider, "ai-provider", "", "AI provider: openrouter, openai, anthropic or skip")
		f.StringVar(&o.aiKeyFile, "ai-key-file", "", "file containing the AI provider API key (or set "+aiKeyEnv+")")
		f.StringVar(&o.meetingProvider, "meeting-provider", "", "meeting capture provider: recall, vexa or skip")
		f.StringVar(&o.meetingKeyFile, "meeting-key-file", "", "file containing the meeting capture API key (or set "+meetingKeyEnv+")")
		f.StringVar(&o.meetingWebhookSecretFile, "meeting-webhook-secret-file", "", "file containing the meeting capture webhook secret (or set "+meetingSecretEnv+")")
		f.StringVar(&o.vexaURL, "vexa-url", "", "Vexa API URL (default "+defaultVexaURL+"; your server for a self-hosted Vexa)")
		f.StringVar(&o.googleClientID, "google-client-id", "", "Google OAuth client ID for Gmail and Calendar (omit to leave Google unchanged)")
		f.StringVar(&o.googleClientSecretFile, "google-client-secret-file", "", "file containing the Google OAuth client secret (or set "+googleSecretEnv+")")
	}
	if command == "install" || command == "upgrade" {
		f.StringVar(&o.release, "version", "", "release tag (install: CLI version; upgrade: newest published Community release)")
		f.StringVar(&o.bundle, "bundle", "", "install a previously downloaded release archive")
		f.StringVar(&o.checksum, "checksum", "", "checksum file for --bundle")
	}
	if command == "install" || command == "restore" {
		f.BoolVar(&o.noStart, "no-start", false, "prepare configuration without starting services")
	}
	if err = f.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if command != "logs" && f.NArg() != 0 {
		return errors.New("unexpected arguments; options must follow the command")
	}
	if (command == "install" || command == "restore") && !o.yes {
		dirSet := false
		f.Visit(func(value *flag.Flag) {
			if value.Name == "dir" {
				dirSet = true
			}
		})
		if !dirSet {
			label, fallback := "Installation directory", o.dir
			if command == "restore" {
				label, fallback = "New restore directory", o.dir+"-restored"
			}
			o.dir, err = a.ask(label, fallback)
			if err != nil {
				return err
			}
		}
	}
	if o.dir == "~" {
		o.dir = home
	} else if strings.HasPrefix(o.dir, "~/") {
		o.dir = filepath.Join(home, o.dir[2:])
	}
	o.dir, err = filepath.Abs(o.dir)
	if err != nil {
		return err
	}
	if command == "install" {
		return a.install(o)
	}
	if command == "restore" {
		return a.restore(o)
	}
	dir := filepath.Join(o.dir, "community")
	if _, err = os.Stat(filepath.Join(dir, ".env")); err != nil {
		return fmt.Errorf("no installation at %s; run helpin install or pass --dir", o.dir)
	}
	if command == "doctor" {
		return a.doctor(dir)
	}
	if command == "status" || command == "logs" {
		for _, service := range f.Args() {
			if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(service) {
				return errors.New("logs accepts service names only")
			}
		}
		return a.run(dir, append([]string{"bash", "./setup.sh", command}, f.Args()...)...)
	}
	unlock, err := lock(o.dir)
	if err != nil {
		return err
	}
	defer unlock()
	if command == "backup" {
		if err := a.confirm(o.yes, "Pause services and back up this installation?"); err != nil {
			return err
		}
		_, err := a.backup(o, true)
		return err
	}
	if command == "upgrade" {
		return a.upgrade(o)
	}
	if command == "configure" {
		values, err := readEnv(dir)
		if err != nil {
			return err
		}
		if err = a.configuration(&o, values); err != nil {
			return err
		}
		if err = a.integrations(&o, values); err != nil {
			return err
		}
		if err = writeConfiguration(dir, o); err != nil {
			return err
		}
		if err = writeIntegrations(dir, o); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "Configuration saved. Apply it with: helpin restart --dir %q\n", o.dir)
		return nil
	}
	if command == "restart" {
		err = a.run(dir, composeArgs(dir, "up", "-d", "--force-recreate", "--wait", "--wait-timeout", "300")...)
	} else {
		err = a.run(dir, "bash", "./setup.sh", command)
	}
	if err != nil {
		return fmt.Errorf("%s failed; run helpin doctor --dir %q and helpin logs --dir %q: %w", command, o.dir, o.dir, err)
	}
	if command == "start" || command == "restart" {
		return a.ready(dir)
	}
	return nil
}

func lock(dir string) (func(), error) {
	path := filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".helpin.lock")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot lock installation (%s); another command may be running; remove a stale lock only after verifying it has stopped: %w", path, err)
	}
	fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	f.Close()
	return func() { os.Remove(path) }, nil
}

func (a *app) prerequisites() error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return errors.New("use Linux or macOS with Docker Engine/Desktop and Compose v2")
	}
	for _, cmd := range []string{"bash", "openssl"} {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("install %s and retry", cmd)
		}
	}
	if _, err := a.output("", "docker", "compose", "version"); err != nil {
		return errors.New("Docker Compose v2 is required; install Docker Engine with Compose, or start Docker Desktop")
	}
	info, err := a.output("", "docker", "info", "--format", "{{.MemTotal}}")
	if err != nil {
		return errors.New("cannot reach Docker; start it and check your Docker permissions")
	}
	memory, err := strconv.ParseUint(strings.TrimSpace(string(info)), 10, 64)
	if err != nil {
		return errors.New("cannot determine Docker memory")
	}
	warning, err := checkMemory(memory)
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintln(a.out, "!", warning)
	}
	fmt.Fprintln(a.out, "✓ Docker, Compose, and memory checks passed")
	return nil
}

const gib = 1024 * 1024 * 1024

// A nominal 8 GB server reports roughly 7.6–7.8 GiB to Docker after the kernel
// and firmware reservations, so the requirement accepts 7.5 GiB. Between the
// minimum and that threshold the installation may work but is tight.
const (
	memoryRecommended = 15 * gib / 2 // 7.5 GiB
	memoryMinimum     = 6 * gib
)

var errInsufficientMemory = errors.New("insufficient memory")

type memoryError struct{ total uint64 }

func (e memoryError) Error() string {
	return fmt.Sprintf("Docker reports %.1f GiB RAM; Community evaluation needs a server with at least 8 GB RAM; increase the Docker VM/server memory", float64(e.total)/gib)
}

func (e memoryError) Is(target error) bool { return target == errInsufficientMemory }

// checkMemory returns an error below the minimum and a warning between the
// minimum and the nominal-8-GB threshold.
func checkMemory(total uint64) (string, error) {
	switch {
	case total < memoryMinimum:
		return "", memoryError{total}
	case total < memoryRecommended:
		return fmt.Sprintf("Docker reports %.1f GiB RAM; a server with at least 8 GB RAM is recommended, so services may run slowly or restart under load", float64(total)/gib), nil
	}
	return "", nil
}

func checkPorts(o options) error {
	for _, port := range []int{o.port, o.storagePort, o.helpPort} {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return fmt.Errorf("port %d is unavailable; choose --port, --storage-port or --help-port: %w", port, err)
		}
		listener.Close()
	}
	if o.mode == "server" && o.proxyMode == "builtin" {
		for _, port := range []int{80, 443} {
			if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
				conn.Close()
				return fmt.Errorf("port %d is already in use; stop the existing web server or use --proxy external", port)
			}
		}
	}
	return nil
}

func (a *app) ready(dir string) error {
	values, err := readEnv(dir)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	url := "http://127.0.0.1:" + values["DASHBOARD_PORT"]
	if err = waitForApp(client, url, values["PUBLIC_WIDGET_URL"], a.readyWait); err != nil {
		return fmt.Errorf("services started but dashboard readiness failed; run helpin doctor: %w", err)
	}
	fmt.Fprintln(a.out, "✓ Helpin services are ready")
	if values["HELPIN_PROXY"] == "builtin" {
		fmt.Fprintf(a.out, "Public URL: %s\nHTTPS is handled by the bundled Caddy proxy. Point the dashboard, help-center and attachment DNS names at this server and allow ports 80 and 443; certificates are issued automatically. Verify with: helpin doctor --dir %q\n", values["APP_BASE_URL"], filepath.Dir(dir))
	} else if strings.HasPrefix(values["APP_BASE_URL"], "https://") {
		fmt.Fprintf(a.out, "Public URL: %s\nHTTPS is not configured by the CLI. Install the generated Caddyfile on this host, point DNS here, and run helpin doctor --dir %q to verify public access.\n", values["APP_BASE_URL"], filepath.Dir(dir))
	} else {
		fmt.Fprintf(a.out, "Open %s to create your account and workspace.\n", values["APP_BASE_URL"])
	}
	fmt.Fprintf(a.out, "Next: allow your website origin in workspace settings and copy the widget snippet.\nOptional AI: Settings → AI.\nStatus: helpin status --dir %q\nLogs:   helpin logs --dir %q\n", filepath.Dir(dir), filepath.Dir(dir))
	return nil
}

func (a *app) doctor(dir string) error {
	failures, hints := 0, []string{}
	addHint := func(hint string) {
		for _, h := range hints {
			if h == hint {
				return
			}
		}
		hints = append(hints, hint)
	}
	check := func(label string, err error, hint func(error) string) {
		if err != nil {
			fmt.Fprintf(a.out, "✗ %s: %v\n", label, err)
			failures++
			addHint(hint(err))
		} else {
			fmt.Fprintln(a.out, "✓", label)
		}
	}
	fixed := func(hint string) func(error) string { return func(error) string { return hint } }
	logsHint := fixed("inspect helpin logs")
	check("Docker prerequisites", a.prerequisites(), func(err error) string {
		if errors.Is(err, errInsufficientMemory) {
			return "increase server memory to at least 8 GB RAM"
		}
		return "start Docker and check Docker Compose v2 and your Docker permissions"
	})
	_, err := a.output(dir, composeArgs(dir, "config", "--quiet")...)
	check("Compose configuration", err, fixed("review .env and compose.yaml, or rerun helpin configure"))
	check("Service status", a.run(dir, "bash", "./setup.sh", "status"), logsHint)
	check("All required services", a.checkServices(dir), logsHint)
	values, err := readEnv(dir)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	local := "http://127.0.0.1:" + values["DASHBOARD_PORT"]
	check(local, waitForApp(client, local, values["PUBLIC_WIDGET_URL"], 0), logsHint)
	public := values["APP_BASE_URL"]
	publicHint := logsHint
	if !isLoopbackURL(public) {
		publicHint = fixed("for the public URL, check DNS, your HTTPS proxy and trusted proxy CIDR")
	}
	check(public, waitForApp(client, public, values["PUBLIC_WIDGET_URL"], 0), publicHint)
	info, err := os.Stat(filepath.Join(dir, ".env"))
	if err == nil && info.Mode().Perm()&0077 != 0 {
		err = errors.New("configuration contains secrets; run chmod 600 on .env")
	}
	check("Secret file permissions", err, fixed("run chmod 600 on .env"))
	if capabilityFailures := a.reportCapabilities(client, values); len(capabilityFailures) > 0 {
		failures += len(capabilityFailures)
		addHint("follow the Next steps listed under Capabilities")
	}
	if failures > 0 {
		return fmt.Errorf("%d checks failed; %s", failures, strings.Join(hints, "; "))
	}
	return nil
}

func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Match the public configuration, so a proxy pointing at another application
// cannot report a successful installation merely because it returns HTTP 200.
func waitForApp(client *http.Client, base, wanted string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := client.Get(base + "/api/auth/config")
		if err == nil {
			var config struct {
				Widget string `json:"public_widget_url"`
			}
			if resp.StatusCode != 200 {
				err = fmt.Errorf("HTTP %d", resp.StatusCode)
			} else {
				err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&config)
			}
			resp.Body.Close()
			if err == nil && (wanted == "" || config.Widget != wanted) {
				err = errors.New("API belongs to another installation or has not applied the configuration")
			}
		}
		if err == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (a *app) checkServices(dir string) error {
	expected, err := a.output(dir, composeArgs(dir, "config", "--services")...)
	if err != nil {
		return err
	}
	data, err := a.output(dir, composeArgs(dir, "ps", "--all", "--format", "json")...)
	if err != nil {
		return err
	}
	return checkServiceStates(data, strings.Fields(string(expected)))
}

func checkServiceStates(data []byte, names []string) error {
	var err error
	type service struct {
		Service, State, Health string
		ExitCode               int
	}
	var services []service
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		err = json.Unmarshal(data, &services)
	} else {
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		for {
			var item service
			e := decoder.Decode(&item)
			if e == io.EOF {
				break
			}
			if e != nil {
				err = e
				break
			}
			services = append(services, item)
		}
	}
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	seen := map[string]bool{}
	for _, item := range services {
		if !wanted[item.Service] {
			continue
		}
		job := item.Service == "helpin-migrate" || item.Service == "temporal-schema" || item.Service == "temporal-namespace"
		if job && item.State == "exited" && item.ExitCode == 0 {
			seen[item.Service] = true
			continue
		}
		if item.State != "running" || (item.Health != "" && item.Health != "healthy") {
			return fmt.Errorf("%s is %s (%s)", item.Service, item.State, item.Health)
		}
		seen[item.Service] = true
	}
	if len(names) == 0 {
		return errors.New("no services found in this installation")
	}
	for _, name := range names {
		if !seen[name] {
			return fmt.Errorf("%s is missing; run helpin start", name)
		}
	}
	return nil
}
