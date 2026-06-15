package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHelpinMCPBridgeCommandExpandsEnvironment(t *testing.T) {
	t.Setenv("PWD", "/root/helpin/server")

	got := resolveHelpinMCPBridgeCommand("$PWD/bin/helpin-mcp-bridge")

	if got != "/root/helpin/server/bin/helpin-mcp-bridge" {
		t.Fatalf("expected expanded absolute command, got %q", got)
	}
}

func TestResolveHelpinMCPBridgeCommandMakesRelativePathAbsolute(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	got := resolveHelpinMCPBridgeCommand("bin/helpin-mcp-bridge")
	want := filepath.Join(tmp, "bin", "helpin-mcp-bridge")
	if got != want {
		t.Fatalf("expected absolute relative command %q, got %q", want, got)
	}
}

func TestResolveHelpinMCPBridgeCommandFallsBackFromBadPWDExpansion(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	bridgePath := filepath.Join(tmp, "bin", "helpin-mcp-bridge")
	if err := os.MkdirAll(filepath.Dir(bridgePath), 0o755); err != nil {
		t.Fatalf("mkdir bridge dir: %v", err)
	}
	if err := os.WriteFile(bridgePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write bridge executable: %v", err)
	}

	got := resolveHelpinMCPBridgeCommand("/bin/helpin-mcp-bridge")

	if got != bridgePath {
		t.Fatalf("expected local bridge fallback %q, got %q", bridgePath, got)
	}
}

func TestValidateHelpinMCPBridgeCommandReportsMissingPath(t *testing.T) {
	err := validateHelpinMCPBridgeCommand("/tmp/does-not-exist/helpin-mcp-bridge")
	if err == nil || !strings.Contains(err.Error(), "/tmp/does-not-exist/helpin-mcp-bridge") {
		t.Fatalf("expected missing path in error, got %v", err)
	}
}

func TestValidateHelpinMCPBridgeCommandAcceptsExecutablePath(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "helpin-mcp-bridge")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	if err := validateHelpinMCPBridgeCommand(path); err != nil {
		t.Fatalf("expected executable path to validate: %v", err)
	}
}
