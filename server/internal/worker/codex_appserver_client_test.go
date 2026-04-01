package worker

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveCodexLaunchPrefersRepoBinary(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex-rs", "target", "debug", codexNativeBinaryName())
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatalf("mkdir binary dir: %v", err)
	}
	if err := os.WriteFile(binary, []byte(""), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	bin, prefixArgs, workDir, err := resolveCodexLaunch(root, "/tmp/workspace")
	if err != nil {
		t.Fatalf("resolve codex launch: %v", err)
	}
	if bin != binary {
		t.Fatalf("expected repo binary %q, got %q", binary, bin)
	}
	if len(prefixArgs) != 0 {
		t.Fatalf("expected no launcher args, got %#v", prefixArgs)
	}
	if workDir != "/tmp/workspace" {
		t.Fatalf("expected fallback workdir to be preserved, got %q", workDir)
	}
}

func TestResolveCodexLaunchUsesPackagedNodeLauncherWhenVendorPresent(t *testing.T) {
	root := t.TempDir()
	launcher := filepath.Join(root, "codex-cli", "bin", "codex.js")
	vendorBinary := codexRepoVendorBinary(root)
	if vendorBinary == "" {
		t.Skipf("no vendor target triple for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatalf("mkdir launcher dir: %v", err)
	}
	if err := os.WriteFile(launcher, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatalf("write launcher: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(vendorBinary), 0o755); err != nil {
		t.Fatalf("mkdir vendor dir: %v", err)
	}
	if err := os.WriteFile(vendorBinary, []byte(""), 0o755); err != nil {
		t.Fatalf("write vendor binary: %v", err)
	}

	bin, prefixArgs, workDir, err := resolveCodexLaunch(root, "/tmp/workspace")
	if err != nil {
		t.Fatalf("resolve codex launch: %v", err)
	}
	if bin != "node" {
		t.Fatalf("expected node launcher, got %q", bin)
	}
	if len(prefixArgs) != 1 || prefixArgs[0] != launcher {
		t.Fatalf("expected node to launch %q, got %#v", launcher, prefixArgs)
	}
	if workDir != "/tmp/workspace" {
		t.Fatalf("expected fallback workdir to be preserved, got %q", workDir)
	}
}

func TestResolveCodexLaunchRejectsRepoWithoutRunnableBinary(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "codex-rs", "Cargo.toml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(manifest, []byte("[workspace]\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, _, _, err := resolveCodexLaunch(root, "/tmp/workspace")
	if err == nil {
		t.Fatal("expected resolveCodexLaunch to reject repo without runnable binary")
	}
	if !strings.Contains(err.Error(), "does not contain a runnable Codex binary") {
		t.Fatalf("expected helpful prereq error, got %v", err)
	}
}

func TestRequestHandlesNotificationBeforeResponse(t *testing.T) {
	client := newCodexAppServerClient("codex", "", nil)
	client.stdin = nopWriteCloser{Writer: io.Discard}

	go func() {
		client.lines <- codexRPCMessage{
			Method: "thread/started",
			Params: json.RawMessage(`{"thread":{"id":"thr_123"}}`),
		}
		client.lines <- codexRPCMessage{
			ID:     json.RawMessage(`1`),
			Result: json.RawMessage(`{"ok":true}`),
		}
	}()

	result, err := client.Request(context.Background(), "thread/start", map[string]any{"cwd": "/tmp"})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if strings.TrimSpace(string(result)) != `{"ok":true}` {
		t.Fatalf("unexpected response payload: %s", string(result))
	}

	queued, err := client.Next(context.Background())
	if err != nil {
		t.Fatalf("expected queued notification after request, got error: %v", err)
	}
	if queued.Method != "thread/started" {
		t.Fatalf("expected queued thread/started notification, got method=%q", queued.Method)
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }
