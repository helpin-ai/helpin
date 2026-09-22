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
	project                                                            string
	port, storagePort, helpPort                                        int
	yes, noStart                                                       bool
	backupPath                                                         string
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
}

func main() {
	a := &app{in: bufio.NewReader(os.Stdin), out: os.Stdout, client: &http.Client{Timeout: 5 * time.Minute}, readyWait: 30 * time.Second}
	if info, err := os.Stdin.Stat(); err == nil {
		a.interactive = info.Mode()&os.ModeCharDevice != 0
	}
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
		fmt.Fprintln(a.out, "Helpin Community\n\nUsage: helpin <command> [options]\nCommands: install, start, stop, restart, status, logs, configure, doctor, backup, restore, upgrade, version\nRun helpin <command> --help for options. Default installation: ~/helpin.\nBackup pauses services; restore uses new volumes. Upgrade requires a compatible release and creates a recovery backup.")
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
		f.StringVar(&o.proxy, "proxy-cidr", "", "exact trusted proxy source IP/CIDR as seen by ingress")
		f.IntVar(&o.port, "port", 0, "local dashboard port (default 8085)")
		f.IntVar(&o.storagePort, "storage-port", 0, "local storage port (default 9005)")
		f.IntVar(&o.helpPort, "help-port", 0, "local help-center port (default 8086)")
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
		if err = writeConfiguration(dir, o); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "Configuration saved. Apply it with: helpin restart --dir %q\n", o.dir)
		return nil
	}
	if command == "restart" {
		err = a.run(dir, "docker", "compose", "--env-file", ".env", "-f", "compose.yaml", "up", "-d", "--force-recreate", "--wait", "--wait-timeout", "300")
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
	if memory < 8*1024*1024*1024 {
		return errors.New("Docker needs at least 8 GiB RAM for Community evaluation; increase the Docker VM/server memory")
	}
	fmt.Fprintln(a.out, "✓ Docker, Compose, and memory checks passed")
	return nil
}

func checkPorts(o options) error {
	for _, port := range []int{o.port, o.storagePort, o.helpPort} {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return fmt.Errorf("port %d is unavailable; choose --port, --storage-port or --help-port: %w", port, err)
		}
		listener.Close()
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
	if strings.HasPrefix(values["APP_BASE_URL"], "https://") {
		fmt.Fprintf(a.out, "Public URL: %s\nHTTPS is not configured by the CLI. Install the generated Caddyfile on this host, point DNS here, and run helpin doctor --dir %q to verify public access.\n", values["APP_BASE_URL"], filepath.Dir(dir))
	} else {
		fmt.Fprintf(a.out, "Open %s to create your account and workspace.\n", values["APP_BASE_URL"])
	}
	fmt.Fprintf(a.out, "Next: allow your website origin in workspace settings and copy the widget snippet.\nOptional AI: Settings → AI.\nStatus: helpin status --dir %q\nLogs:   helpin logs --dir %q\n", filepath.Dir(dir), filepath.Dir(dir))
	return nil
}

func (a *app) doctor(dir string) error {
	var failures []string
	check := func(label string, err error) {
		if err != nil {
			fmt.Fprintf(a.out, "✗ %s: %v\n", label, err)
			failures = append(failures, label)
		} else {
			fmt.Fprintln(a.out, "✓", label)
		}
	}
	check("Docker prerequisites", a.prerequisites())
	_, err := a.output(dir, "docker", "compose", "--env-file", ".env", "-f", "compose.yaml", "config", "--quiet")
	check("Compose configuration", err)
	check("Service status", a.run(dir, "bash", "./setup.sh", "status"))
	check("All required services", a.checkServices(dir))
	values, err := readEnv(dir)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, endpoint := range []string{"http://127.0.0.1:" + values["DASHBOARD_PORT"], values["APP_BASE_URL"]} {
		check(endpoint, waitForApp(client, endpoint, values["PUBLIC_WIDGET_URL"], 0))
	}
	info, err := os.Stat(filepath.Join(dir, ".env"))
	if err == nil && info.Mode().Perm()&0077 != 0 {
		err = errors.New("configuration contains secrets; run chmod 600 on .env")
	}
	check("Secret file permissions", err)
	if len(failures) > 0 {
		return fmt.Errorf("%d checks failed; inspect helpin logs; for public URLs check DNS, your HTTPS proxy and trusted proxy CIDR", len(failures))
	}
	return nil
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
	expected, err := a.output(dir, "docker", "compose", "--env-file", ".env", "-f", "compose.yaml", "config", "--services")
	if err != nil {
		return err
	}
	data, err := a.output(dir, "docker", "compose", "--env-file", ".env", "-f", "compose.yaml", "ps", "--all", "--format", "json")
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
