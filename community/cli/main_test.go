package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testApp(t *testing.T) (*app, *bytes.Buffer) {
	t.Helper()
	out := new(bytes.Buffer)
	a := &app{out: out, in: bufio.NewReader(strings.NewReader("")), client: http.DefaultClient}
	a.output = func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "df":
			return []byte("Filesystem 1024-blocks Used Available Capacity Mounted\nfixture 99999999 1 99999998 1% /\n"), nil
		case "docker":
			if len(args) > 1 && args[1] == "info" {
				return []byte("17179869184"), nil
			}
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command %v", args)
	}
	a.run = func(_ string, args ...string) error { return fmt.Errorf("unexpected mutation %v", args) }
	return a, out
}

func put(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
func get(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func template(t *testing.T) string { return get(t, "../.env.example") }
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func bundleFixture(t *testing.T, entries map[string]string) (string, string) {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, data := range entries {
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0700, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err = tw.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err = tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	checksum := archive + ".sha256"
	put(t, checksum, fmt.Sprintf("%x  bundle.tar.gz\n", sha256.Sum256(data)))
	return archive, checksum
}

func operatorBundle(t *testing.T) (string, string) {
	entries := map[string]string{"helpin-community/release-evidence/release.json": `{"tag":"community-v0.1.0-rc.1"}`}
	for _, file := range []string{"setup.sh", ".env.example", "apps.example.json", "compose.yaml"} {
		entries["helpin-community/community/"+file] = get(t, "../"+file)
	}
	return bundleFixture(t, entries)
}

func realSetup(t *testing.T, a *app) {
	bin := t.TempDir()
	put(t, filepath.Join(bin, "docker"), "#!/bin/sh\nexit 0\n")
	os.Chmod(filepath.Join(bin, "docker"), 0700)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	a.run = func(dir string, args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = commandEnv(dir)
		data, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, data)
		}
		return nil
	}
}

func TestInstallGeneratesSecretsAndRepeatPreservesEverything(t *testing.T) {
	a, out := testApp(t)
	realSetup(t, a)
	archive, sum := operatorBundle(t)
	dir := filepath.Join(t.TempDir(), "installation with spaces")
	o := options{dir: dir, bundle: archive, checksum: sum, yes: true, noStart: true, port: freePort(t), storagePort: freePort(t), helpPort: freePort(t)}
	if err := a.install(o); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(dir, "community/.env")
	before := get(t, envPath)
	if strings.Contains(before, "=GENERATE") {
		t.Fatal("secret placeholders remain")
	}
	values := envValues(before)
	if values["JWT_SECRET"] == values["AI_CONNECTION_ENCRYPTION_KEY"] {
		t.Fatal("secrets reused")
	}
	// Modules enabled by default need their own stable 32-byte hex keys.
	for _, key := range []string{"CRM_ENCRYPTION_KEY", "GIT_OAUTH_ENCRYPTION_KEY"} {
		if decoded, err := hex.DecodeString(values[key]); err != nil || len(decoded) != 32 {
			t.Fatalf("%s is not a generated 32-byte hex key", key)
		}
	}
	if values["CRM_ENCRYPTION_KEY"] == values["GIT_OAUTH_ENCRYPTION_KEY"] || values["CRM_ENCRYPTION_KEY"] == values["AI_CONNECTION_ENCRYPTION_KEY"] {
		t.Fatal("encryption keys reused")
	}
	if values["HELPIN_ENABLED_MODULES"] != "support,docs,agents,pm,crm,automation" {
		t.Fatal("new installations must enable every module:", values["HELPIN_ENABLED_MODULES"])
	}
	info, _ := os.Stat(envPath)
	if info.Mode().Perm() != 0600 {
		t.Fatal("secrets are not private")
	}
	a.run = func(string, ...string) error { t.Fatal("repeat install must not run commands"); return nil }
	if err := a.install(o); err != nil {
		t.Fatal(err)
	}
	if before != get(t, envPath) {
		t.Fatal("repeat install changed configuration")
	}
	if !strings.Contains(out.String(), "Existing installation preserved") {
		t.Fatal(out.String())
	}
}

func TestFailedStartupLeavesRecoverableInstallation(t *testing.T) {
	a, _ := testApp(t)
	realSetup(t, a)
	setup := a.run
	a.run = func(dir string, args ...string) error {
		if args[len(args)-1] == "start" {
			return errors.New("image unavailable")
		}
		return setup(dir, args...)
	}
	archive, sum := operatorBundle(t)
	dir := filepath.Join(t.TempDir(), "helpin")
	err := a.install(options{dir: dir, bundle: archive, checksum: sum, yes: true, port: freePort(t), storagePort: freePort(t), helpPort: freePort(t)})
	if err == nil || !strings.Contains(err.Error(), "preserved") {
		t.Fatal(err)
	}
	get(t, filepath.Join(dir, "community/.env"))
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestRejectsCorruptBundleBeforeSetup(t *testing.T) {
	a, _ := testApp(t)
	archive, sum := operatorBundle(t)
	put(t, archive, "tampered")
	dir := filepath.Join(t.TempDir(), "helpin")
	err := a.install(options{dir: dir, bundle: archive, checksum: sum, yes: true, noStart: true, port: freePort(t), storagePort: freePort(t), helpPort: freePort(t)})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatal(err)
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("failed install left destination")
	}
}

func TestExtractionRejectsTraversalAndLinks(t *testing.T) {
	for _, name := range []string{"../outside", "helpin-community/../../outside", "/tmp/outside", "helpin-community/../outside", "helpin-community\\outside"} {
		archive, _ := bundleFixture(t, map[string]string{name: "bad"})
		if err := extract(archive, t.TempDir()); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	archive := filepath.Join(t.TempDir(), "links.tar.gz")
	f, _ := os.Create(archive)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "helpin-community/link", Typeflag: tar.TypeSymlink, Linkname: "/tmp"})
	tw.Close()
	gz.Close()
	f.Close()
	if err := extract(archive, t.TempDir()); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestConfigurePublicPreservesSecretsAndNeverSourcesEnv(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	original := strings.ReplaceAll(template(t), "=GENERATE\n", "=existing-secret\n") + "\nUNUSED=$(touch " + marker + ")\n"
	put(t, filepath.Join(dir, ".env"), original)
	o := options{yes: true, mode: "server", domain: "inbox.example.com", storage: "files.example.com", help: "help.example.com", proxy: "172.18.0.1"}
	if err := a.configuration(&o, envValues(original)); err != nil {
		t.Fatal(err)
	}
	if err := writeConfiguration(dir, o); err != nil {
		t.Fatal(err)
	}
	values, _ := readEnv(dir)
	if values["JWT_SECRET"] != "existing-secret" || values["APP_BASE_URL"] != "https://inbox.example.com" || values["COMMUNITY_TRUSTED_PROXY_CIDR"] != "172.18.0.1/32" {
		t.Fatal(values["APP_BASE_URL"])
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("dotenv executed")
	}
	caddy := get(t, filepath.Join(dir, "Caddyfile"))
	if !strings.Contains(caddy, "files.example.com") || !strings.Contains(caddy, "127.0.0.1:9005") {
		t.Fatal(caddy)
	}
	for _, proxy := range []string{"0.0.0.0/0", "::/0", "bad"} {
		o.proxy = proxy
		if err := a.configuration(&o, nil); err == nil {
			t.Fatal("accepted unsafe proxy")
		}
	}
	for _, domain := range []string{"https://example.com", "example.com\nother", "example.com:443", "localhost", "$(id).example.com"} {
		o.proxy = "172.18.0.1"
		o.domain = domain
		if err := a.configuration(&o, nil); err == nil {
			t.Fatal("accepted invalid domain")
		}
	}
}

func TestPortConflictAndLock(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := checkPorts(options{port: l.Addr().(*net.TCPAddr).Port}); err == nil {
		t.Fatal("ignored port conflict")
	}
	dir := filepath.Join(t.TempDir(), "helpin")
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := lock(dir); err == nil {
		t.Fatal("concurrent command acquired lock")
	}
}

func TestHeadlessFlagsAndCancelledWizard(t *testing.T) {
	a, _ := testApp(t)
	for _, args := range [][]string{{"install", "--unknown"}, {"install", "unexpected"}, {"install", "--mode", "server", "--yes"}, {"upgrade"}} {
		if err := a.execute(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	o := options{}
	if err := a.configuration(&o, nil); err == nil {
		t.Fatal("headless install should require --yes")
	}
	a.interactive = true
	if err := a.configuration(&o, nil); err == nil {
		t.Fatal("EOF should cancel wizard")
	}
}

func TestReadinessDoesNotAnnounceSuccessOnFailure(t *testing.T) {
	a, out := testApp(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	dir := t.TempDir()
	put(t, filepath.Join(dir, ".env"), "DASHBOARD_PORT="+port+"\nAPP_BASE_URL="+server.URL+"\nPUBLIC_WIDGET_URL=http://expected.test\n")
	if err := a.ready(dir); err == nil {
		t.Fatal("announced unready application")
	}
	if strings.Contains(out.String(), "services are ready") {
		t.Fatal(out.String())
	}
}

func TestLifecycleUsesSelectedDirectoryAndPreservesVolumes(t *testing.T) {
	a, _ := testApp(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"public_widget_url":"http://expected.test"}`)
	}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	root := t.TempDir()
	dir := filepath.Join(root, "community")
	put(t, filepath.Join(dir, ".env"), "DASHBOARD_PORT="+port+"\nAPP_BASE_URL="+server.URL+"\nPUBLIC_WIDGET_URL=http://expected.test\n")
	var calls [][]string
	a.run = func(cwd string, args ...string) error {
		if cwd != dir {
			t.Fatal(cwd)
		}
		calls = append(calls, args)
		return nil
	}
	for _, command := range []string{"start", "stop", "restart", "status", "logs"} {
		if err := a.execute([]string{command, "--dir", root}); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(strings.Join(calls[2], " "), "--force-recreate") {
		t.Fatal("restart did not recreate services")
	}
	for _, call := range calls {
		for _, arg := range call {
			if arg == "-v" || arg == "--volumes" {
				t.Fatal("deleted data")
			}
		}
	}
	if err := a.execute([]string{"logs", "--dir", root, "--", "--help"}); err == nil {
		t.Fatal("allowed option injection")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReleaseDiscoveryAndDownloadFailures(t *testing.T) {
	a, _ := testApp(t)
	a.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`[{"tag_name":"cloud-v9"},{"tag_name":"community-v0.3.0","draft":true},{"tag_name":"community-v0.1.0-rc.1"}]`)), Header: make(http.Header)}, nil
	})}
	tag, err := a.releaseTag("")
	if err != nil || tag != "community-v0.1.0-rc.1" {
		t.Fatal(tag, err)
	}
	if _, err = a.releaseTag("../../bad"); err == nil {
		t.Fatal("accepted invalid release")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer server.Close()
	if err := download(server.Client(), server.URL, filepath.Join(t.TempDir(), "bundle"), 1024); err == nil {
		t.Fatal("accepted failed download")
	}
}

func TestEnvironmentCannotRedirectCompose(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, ".env"), "JWT_SECRET=stored\nCOMPOSE_PROJECT_NAME=helpin-community\n")
	t.Setenv("COMPOSE_PROJECT_NAME", "other-project")
	t.Setenv("COMPOSE_FILE", "other.yaml")
	t.Setenv("JWT_SECRET", "wrong")
	env := strings.Join(commandEnv(dir), "\n")
	for _, key := range []string{"JWT_SECRET=", "COMPOSE_FILE=", "COMPOSE_PROJECT_NAME="} {
		if strings.Contains(env, key) {
			t.Fatal("leaked override", key)
		}
	}
}

func TestDoctorFailsForUnhealthyServices(t *testing.T) {
	a, _ := testApp(t)
	a.output = func(_ string, args ...string) ([]byte, error) { return nil, errors.New("Docker unavailable") }
	a.run = func(string, ...string) error { return nil }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	dir := t.TempDir()
	put(t, filepath.Join(dir, ".env"), "DASHBOARD_PORT="+port+"\nAPP_BASE_URL="+server.URL+"\nPUBLIC_WIDGET_URL=http://expected.test\n")
	if err := a.doctor(dir); err == nil {
		t.Fatal("doctor passed broken installation")
	}
}

func TestCheckMemoryAcceptsNominalEightGigabyteServers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		total         uint64
		warn, failure bool
	}{
		{"16 GiB", 16 * gib, false, false},
		{"nominal 8 GB server", 77 * gib / 10, false, false},
		{"threshold", 7.5 * gib, false, false},
		{"tight", 7 * gib, true, false},
		{"minimum", 6 * gib, true, false},
		{"too small", 4 * gib, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warning, err := checkMemory(tc.total)
			if (err != nil) != tc.failure || (warning != "") != tc.warn {
				t.Fatalf("warning=%q err=%v", warning, err)
			}
			if err != nil && (!errors.Is(err, errInsufficientMemory) || !strings.Contains(err.Error(), "at least 8 GB RAM")) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestPrerequisitesWarnsOnTightMemory(t *testing.T) {
	a, out := testApp(t)
	a.output = func(_ string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "info" {
			return []byte(fmt.Sprint(uint64(6.5 * gib))), nil
		}
		return nil, nil
	}
	if err := a.prerequisites(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "! Docker reports 6.5 GiB RAM") {
		t.Fatal(out.String())
	}
}

func TestDoctorHintReflectsFailedChecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	for _, tc := range []struct {
		name, memory, publicURL string
		want, reject            []string
	}{
		{"memory with local URL", "4294967296", server.URL, []string{"increase server memory to at least 8 GB RAM", "inspect helpin logs"}, []string{"DNS"}},
		{"public URL", "17179869184", "http://0.0.0.0:" + port, []string{"check DNS, your HTTPS proxy"}, []string{"memory"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testApp(t)
			a.output = func(_ string, args ...string) ([]byte, error) {
				if len(args) > 1 && args[1] == "info" {
					return []byte(tc.memory), nil
				}
				return nil, nil
			}
			a.run = func(string, ...string) error { return nil }
			dir := t.TempDir()
			put(t, filepath.Join(dir, ".env"), "DASHBOARD_PORT="+port+"\nAPP_BASE_URL="+tc.publicURL+"\nPUBLIC_WIDGET_URL=http://expected.test\n")
			err := a.doctor(dir)
			if err == nil {
				t.Fatal("doctor passed broken installation")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("missing %q in %v", want, err)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(err.Error(), reject) {
					t.Errorf("unexpected %q in %v", reject, err)
				}
			}
		})
	}
}

func TestIsLoopbackURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"http://127.0.0.1:8080": true, "http://localhost:3000": true, "http://[::1]:80": true,
		"https://helpin.example.com": false, "http://0.0.0.0:80": false, "": false,
	} {
		if got := isLoopbackURL(raw); got != want {
			t.Errorf("%q: got %v", raw, got)
		}
	}
}

func TestReadinessRejectsWrongInstallation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"public_widget_url":"https://other.example.com"}`)
	}))
	defer server.Close()
	if err := waitForApp(server.Client(), server.URL, "https://ours.example.com", 0); err == nil {
		t.Fatal("accepted another installation")
	}
}

func TestServiceDiagnosisChecksWorkersAndCompletedJobs(t *testing.T) {
	for _, tc := range []struct {
		name, states string
		wantError    bool
	}{
		{"healthy", `[{"Service":"helpin-api","State":"running","Health":"healthy"},{"Service":"helpin-migrate","State":"exited","ExitCode":0}]`, false},
		{"missing", `[{"Service":"helpin-api","State":"running","Health":"healthy"}]`, true},
		{"unhealthy", `[{"Service":"helpin-api","State":"running","Health":"unhealthy"},{"Service":"helpin-migrate","State":"exited","ExitCode":0}]`, true},
		{"failed migration", `[{"Service":"helpin-api","State":"running","Health":"healthy"},{"Service":"helpin-migrate","State":"exited","ExitCode":1}]`, true},
		{"stopped", `[]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testApp(t)
			a.output = func(_ string, args ...string) ([]byte, error) {
				if args[len(args)-1] == "--services" {
					return []byte("helpin-api\nhelpin-migrate"), nil
				}
				return []byte(tc.states), nil
			}
			err := a.checkServices(t.TempDir())
			if (err != nil) != tc.wantError {
				t.Fatal(err)
			}
		})
	}
}

func TestResourceFailuresDoNotInstall(t *testing.T) {
	for _, kind := range []string{"memory", "disk", "docker"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := testApp(t)
			normal := a.output
			a.output = func(dir string, args ...string) ([]byte, error) {
				if kind == "memory" && args[0] == "docker" && len(args) > 1 && args[1] == "info" {
					return []byte("1000"), nil
				}
				if kind == "docker" && args[0] == "docker" {
					return nil, errors.New("unavailable")
				}
				if kind == "disk" && args[0] == "df" {
					return []byte("Filesystem 1024-blocks Used Available Capacity Mounted\nfixture 100 1 99 1% /"), nil
				}
				return normal(dir, args...)
			}
			dir := filepath.Join(t.TempDir(), "helpin")
			err := a.install(options{dir: dir, yes: true, noStart: true, port: freePort(t), helpPort: freePort(t), storagePort: freePort(t)})
			if err == nil {
				t.Fatal("ignored resource failure")
			}
			if _, err = os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("created installation despite failed prerequisites")
			}
		})
	}
}

func TestSeparateInstallationsAndConfigureKeepProjectIdentity(t *testing.T) {
	a, _ := testApp(t)
	realSetup(t, a)
	archive, sum := operatorBundle(t)
	var previous string
	for i := 0; i < 2; i++ {
		dir := filepath.Join(t.TempDir(), "helpin")
		o := options{dir: dir, bundle: archive, checksum: sum, yes: true, noStart: true, port: freePort(t), helpPort: freePort(t), storagePort: freePort(t)}
		if err := a.install(o); err != nil {
			t.Fatal(err)
		}
		values, err := readEnv(filepath.Join(dir, "community"))
		if err != nil {
			t.Fatal(err)
		}
		project := values["COMPOSE_PROJECT_NAME"]
		if project == "helpin-community" || project == previous || project == "" {
			t.Fatal("installation project collision")
		}
		previous = project
		if err := a.execute([]string{"configure", "--dir", dir, "--yes", "--mode", "local"}); err != nil {
			t.Fatal(err)
		}
		values, _ = readEnv(filepath.Join(dir, "community"))
		if values["COMPOSE_PROJECT_NAME"] != project {
			t.Fatal("configure changed data volume identity")
		}
	}
}

func TestOnlineInstallDownloadsMatchingRelease(t *testing.T) {
	a, _ := testApp(t)
	realSetup(t, a)
	archive, checksum := operatorBundle(t)
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.ReplaceAll(get(t, checksum), "bundle.tar.gz", "helpin-community-v0.1.0-rc.1.tar.gz")
	requests := 0
	a.client = &http.Client{Transport: roundTrip(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Scheme != "https" || request.URL.Host != "github.com" {
			t.Fatal(request.URL)
		}
		base := "/helpin-ai/helpin/releases/download/community-v0.1.0-rc.1/helpin-community-v0.1.0-rc.1.tar.gz"
		var body io.Reader
		switch request.URL.Path {
		case base:
			body = bytes.NewReader(data)
		case base + ".sha256":
			body = strings.NewReader(sum)
		default:
			t.Fatal(request.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(body), Header: make(http.Header)}, nil
	})}
	dir := filepath.Join(t.TempDir(), "helpin")
	if err := a.install(options{dir: dir, release: "community-v0.1.0-rc.1", yes: true, noStart: true, port: freePort(t), helpPort: freePort(t), storagePort: freePort(t)}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatal("bundle and checksum must come from the same release")
	}
	get(t, filepath.Join(dir, "community/.env"))
}

func TestConfigureUpdatesDuplicateURLKeys(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	put(t, filepath.Join(dir, ".env"), template(t)+"APP_BASE_URL=http://old.invalid\n")
	o := options{yes: true, mode: "local", port: 12085}
	if err := a.configuration(&o, nil); err != nil {
		t.Fatal(err)
	}
	if err := writeConfiguration(dir, o); err != nil {
		t.Fatal(err)
	}
	values, _ := readEnv(dir)
	if values["APP_BASE_URL"] != "http://localhost:12085" {
		t.Fatal("duplicate key overrode requested configuration")
	}
}
