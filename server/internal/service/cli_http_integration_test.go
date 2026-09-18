package service_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// TestCLIHTTPBinary runs the unmodified CLI against real Helpin handlers and
// service/repository layers. Only the browser session and workspace are fixtures.
func TestCLIHTTPBinary(t *testing.T)          { testCLIHTTPBinary(t, false) }
func TestCLIConnectedHTTPBinary(t *testing.T) { testCLIHTTPBinary(t, true) }
func testCLIHTTPBinary(t *testing.T, connected bool) {
	t.Helper()
	binary := os.Getenv("AGENT_RUNTIME_CLI_TEST_BINARY")
	if binary == "" {
		t.Skip("set AGENT_RUNTIME_CLI_TEST_BINARY to exercise the compiled CLI")
	}
	router := chi.NewRouter()
	server := httptest.NewUnstartedServer(router)
	baseURL := "http://" + server.Listener.Addr().String()
	defer server.Close()
	svc, db, dispatches := service.NewCLIIntegrationFixture(t, baseURL)
	var observedUsage func() int64
	if connected {
		observedUsage = service.EnableCLIConnectedFixture(t, svc, db)
	}
	h := handler.NewCLIHandler(svc)
	router.Get("/agent-runtime/cli.json", h.Discovery)
	router.Get("/.well-known/oauth-authorization-server/api/cli/oauth", h.Metadata)
	router.Get("/api/cli/oauth/authorize", h.AuthorizeRedirect)
	router.Post("/api/cli/oauth/token", h.Token)
	router.Post("/api/cli/oauth/revoke", h.RevokeToken)
	router.Post("/api/cli/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		h.Authorize(w, r.WithContext(middleware.WithUserID(r.Context(), "user-1")))
	})
	router.Get("/api/cli/v1/me", h.Me)
	router.Get("/api/cli/v1/agents", h.Agents)
	router.Post("/api/cli/v1/runs", h.Admit)
	router.Post("/api/cli/v1/runs/{run_id}/model", h.Model)
	router.Post("/api/cli/v1/runs/{run_id}/events", h.Report)
	router.Post("/api/cli/v1/runs/{run_id}/artifacts", h.Artifact)
	router.Get("/api/cli/v1/runs/{run_id}/execution", h.Execution)
	router.Post("/api/cli/v1/runs/{run_id}/{action:bind|renew|revoke}", h.Execution)
	server.Start()
	home := t.TempDir()
	env := append(os.Environ(), "AGENT_RUNTIME_CLI_HOME="+home, "PATH="+t.TempDir())
	command := func(args ...string) *exec.Cmd {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		t.Cleanup(cancel)
		c := exec.CommandContext(ctx, binary, args...)
		c.Env = env
		return c
	}
	run := func(args ...string) []byte {
		t.Helper()
		out, err := command(args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return out
	}
	run("connect", server.URL, "--name", "helpin", "--credential-store", "file")
	login := command("login", "helpin")
	pipe, err := login.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	login.Stderr = &stderr
	if err = login.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(pipe)
	var authURL string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), server.URL) {
			authURL = scanner.Text()
			break
		}
	}
	if authURL == "" {
		t.Fatalf("login URL missing: %s", stderr.String())
	}
	// Browser follows the actual validation redirect, then explicitly posts consent.
	browser := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }, Timeout: 5 * time.Second}
	response, err := browser.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 302 {
		t.Fatalf("authorize status %d", response.StatusCode)
	}
	consentURL, err := url.Parse(response.Header.Get("Location"))
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	q := consentURL.Query()
	decision := model.CLIConsentDecision{WorkspaceID: "ws-1", Query: model.CLIAuthorizationQuery{ClientID: q.Get("client_id"), Resource: q.Get("resource"), RedirectURI: q.Get("redirect_uri"), ResponseType: q.Get("response_type"), Scope: q.Get("scope"), State: q.Get("state"), CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method")}}
	body, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	response, err = browser.Post(server.URL+"/api/cli/oauth/authorize", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var authorized struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err = json.NewDecoder(response.Body).Decode(&authorized); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if authorized.RedirectURL == "" {
		t.Fatal("no authorized callback")
	}
	response, err = browser.Get(authorized.RedirectURL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if _, err = io.Copy(io.Discard, pipe); err != nil {
		t.Fatal(err)
	}
	if err = login.Wait(); err != nil {
		t.Fatalf("login: %v %s", err, stderr.String())
	}
	run("whoami", "helpin")
	run("agents", "helpin")
	args := []string{"admit", "--connection", "helpin", "--agent", "agent-1", "--target", "task:task-1", "--request-id", "http-test-123", "Fix login"}
	// Separate stderr because request recovery information accompanies JSON output.
	admit := command(args...)
	var output bytes.Buffer
	admit.Stdout = &output
	admit.Stderr = &stderr
	if err = admit.Run(); err != nil {
		t.Fatalf("admit: %v %s", err, stderr.String())
	}
	var admission model.CLIAdmission
	if err = json.Unmarshal(output.Bytes(), &admission); err != nil {
		t.Fatal(err)
	}
	if admission.Execution == nil || admission.Execution.Epoch != 1 || !strings.Contains(admission.Context, "login") {
		t.Fatalf("incomplete admission: %+v", admission)
	}
	run(args...)
	var count int64
	if err = db.Model(&model.AgentRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || dispatches() != 0 {
		t.Fatalf("runs=%d cloud dispatches=%d", count, dispatches())
	}
	run("executions", "show", "helpin", admission.RunID)
	run("executions", "bind", "helpin", admission.RunID, "--epoch", "1", "--local-run-id", "local-http-1")
	run("executions", "renew", "helpin", admission.RunID, "--epoch", "1")
	run("executions", "revoke", "helpin", admission.RunID)
	run("executions", "revoke", "helpin", admission.RunID)
	if out, err := command("executions", "renew", "helpin", admission.RunID, "--epoch", "1").CombinedOutput(); err == nil {
		t.Fatalf("revoked grant accepted: %s", out)
	}
	if connected {
		env = append(env, "PATH="+os.Getenv("PATH"))
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "calc.py"), []byte("def add(a, b): return a - b\n"), 0600); err != nil {
			t.Fatal(err)
		}
		git := exec.Command("git", "init", dir)
		if out, err := git.CombinedOutput(); err != nil {
			t.Fatalf("git init %v %s", err, out)
		}
		git = exec.Command("git", "-C", dir, "add", "calc.py")
		if out, err := git.CombinedOutput(); err != nil {
			t.Fatalf("git add %v %s", err, out)
		}
		run("run", "--connection", "helpin", "--agent", "agent-1", "--target", "task:task-1", "--dir", dir, "--yes", "--json", "Fix addition and run tests")
		for attempt := 0; attempt < 4; attempt++ {
			listing := strings.Fields(string(run("runs", "list")))
			if len(listing) < 2 {
				t.Fatal("no local run")
			}
			if listing[1] == "completed" {
				break
			}
			if listing[1] != "paused" {
				t.Fatalf("unexpected local state: %v", listing)
			}
			run("runs", "resume", listing[0], "--intent", "approve", "--json")
		}
		data, err := os.ReadFile(filepath.Join(dir, "calc.py"))
		if err != nil || !strings.Contains(string(data), "a + b") {
			t.Fatalf("local edit missing: %s %v", data, err)
		}
		var finished model.AgentRun
		if err = db.Where("status = ?", "completed").First(&finished).Error; err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(finished.OutputSummary), "local_report") {
			t.Fatal("missing local provenance")
		}
		var artifacts int64
		if err = db.Model(&model.AgentRunArtifact{}).Where("run_id = ?", finished.ID).Count(&artifacts).Error; err != nil {
			t.Fatal(err)
		}
		if artifacts < 2 || observedUsage() < 400 || dispatches() != 0 {
			t.Fatalf("artifacts=%d observed tokens=%d cloud=%d", artifacts, observedUsage(), dispatches())
		}
		listing := strings.Fields(string(run("runs", "list")))
		before := observedUsage()
		run("runs", "sync", listing[0])
		if observedUsage() != before {
			t.Fatal("result replay rebilled usage")
		}
	}
	run("logout", "helpin")
	var revoked int64
	if err = db.Model(&model.CLIConnection{}).Where("revoked_at IS NOT NULL").Count(&revoked).Error; err != nil {
		t.Fatal(err)
	}
	if revoked != 1 {
		t.Fatal("logout did not revoke remote connection")
	}
}
