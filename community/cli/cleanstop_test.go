package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCheckCleanStopOnlyBlocksForcedKillsOfVolumeServices(t *testing.T) {
	tests := []struct {
		name    string
		states  string
		wantErr bool
	}{
		{"all exited cleanly", `[{"Service":"postgres","State":"exited","ExitCode":0},{"Service":"helpin-helpcenter","State":"exited","ExitCode":0}]`, false},
		// Older help-center images ignore SIGTERM and are killed after the stop
		// timeout; they hold no volume, so the backup stays consistent.
		{"stateless service killed", `[{"Service":"postgres","State":"exited","ExitCode":0},{"Service":"helpin-helpcenter","State":"exited","ExitCode":137}]`, false},
		{"database killed", `[{"Service":"postgres","State":"exited","ExitCode":137}]`, true},
		{"object storage killed", `[{"Service":"garage","State":"exited","ExitCode":137}]`, true},
		{"stateless service still running", `[{"Service":"helpin-helpcenter","State":"running","ExitCode":0}]`, true},
		{"restarting", `[{"Service":"helpin-api","State":"restarting","ExitCode":0}]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &app{output: func(string, ...string) ([]byte, error) { return []byte(tt.states), nil }}
			if err := a.checkCleanStop("dir"); (err != nil) != tt.wantErr {
				t.Fatalf("checkCleanStop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Every Compose service that mounts a named volume must be listed, or a forced
// kill of a new stateful service would be accepted during backups.
func TestVolumeServicesMatchCompose(t *testing.T) {
	data, err := os.ReadFile("../compose.yaml")
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	named := map[string]bool{}
	inVolumes := false
	for _, line := range lines {
		switch {
		case line == "volumes:":
			inVolumes = true
		case inVolumes && regexp.MustCompile(`^\S`).MatchString(line):
			inVolumes = false
		case inVolumes:
			if m := regexp.MustCompile(`^  ([a-z0-9_]+):`).FindStringSubmatch(line); m != nil {
				named[m[1]] = true
			}
		}
	}
	if len(named) == 0 {
		t.Fatal("no named volumes found in compose.yaml")
	}
	withVolumes := map[string]bool{}
	service := ""
	inServices := false
	for _, line := range lines {
		if line == "services:" {
			inServices = true
			continue
		}
		if inServices && regexp.MustCompile(`^\S`).MatchString(line) {
			inServices = false
		}
		if !inServices {
			continue
		}
		if m := regexp.MustCompile(`^  ([a-z0-9-]+):\s*$`).FindStringSubmatch(line); m != nil {
			service = m[1]
			continue
		}
		for volume := range named {
			if service != "" && regexp.MustCompile(`[\s\[',]`+volume+`:/`).MatchString(line) {
				withVolumes[service] = true
			}
		}
	}
	for service := range withVolumes {
		if !volumeServices[service] {
			t.Errorf("service %q mounts a named volume but is missing from volumeServices", service)
		}
	}
	for service := range volumeServices {
		if !withVolumes[service] {
			t.Errorf("volumeServices lists %q, which mounts no named volume in compose.yaml", service)
		}
	}
}
