package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVolumeTar(t *testing.T, headers []*tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := tw.Write(bytes.Repeat([]byte("x"), int(h.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
func TestVolumeArchiveRejectsUnsafeEntries(t *testing.T) {
	for name, headers := range map[string][]*tar.Header{
		"traversal":         {{Name: "../outside", Typeflag: tar.TypeReg}},
		"absolute":          {{Name: "/outside", Typeflag: tar.TypeReg}},
		"escaping link":     {{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside"}},
		"link traversal":    {{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "data"}, {Name: "link/child", Typeflag: tar.TypeReg}},
		"replace directory": {{Name: "link/child", Typeflag: tar.TypeReg}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "data"}},
		"duplicate":         {{Name: "file", Typeflag: tar.TypeReg}, {Name: "file", Typeflag: tar.TypeReg}},
		"device":            {{Name: "device", Typeflag: tar.TypeChar}},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "volume.tar.gz")
			if err := os.WriteFile(p, writeVolumeTar(t, headers), 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateVolumeArchive(p); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	p := filepath.Join(t.TempDir(), "volume.tar.gz")
	os.WriteFile(p, writeVolumeTar(t, []*tar.Header{{Name: "./", Typeflag: tar.TypeDir}, {Name: "./data", Typeflag: tar.TypeReg, Size: 4}, {Name: "./link", Typeflag: tar.TypeSymlink, Linkname: "data"}}), 0600)
	if err := validateVolumeArchive(p); err != nil {
		t.Fatal(err)
	}
}

func lifecycleFixture(t *testing.T) (*app, string, *bytes.Buffer, *[]string) {
	t.Helper()
	a, out := testApp(t)
	root := filepath.Join(t.TempDir(), "installed")
	put(t, filepath.Join(root, "community/.env"), "COMPOSE_PROJECT_NAME=helpin-fixture\nJWT_SECRET=keep-this-secret\n")
	put(t, filepath.Join(root, "community/apps.json"), "{}")
	put(t, filepath.Join(root, "community/compose.yaml"), "fixture")
	put(t, filepath.Join(root, "release-evidence/release.json"), `{"tag":"community-v0.1.0"}`)
	var events []string
	a.output = func(dir string, args ...string) ([]byte, error) {
		text := strings.Join(args, " ")
		switch {
		case strings.Contains(text, "docker info"):
			return []byte("x86_64"), nil
		case strings.Contains(text, "config --format json"):
			env, _ := readEnv(dir)
			project := env["COMPOSE_PROJECT_NAME"]
			return []byte(fmt.Sprintf(`{"name":%q,"services":{"helpin-api":{"image":%q}},"volumes":{"data":{"name":%q}}}`, project, archiveImage, project+"_data")), nil
		case strings.Contains(text, "volume inspect"):
			name := args[len(args)-1]
			project := strings.TrimSuffix(name, "_data")
			return []byte(fmt.Sprintf(`[{"Name":%q,"Driver":"local","Labels":{"com.docker.compose.project":%q,"com.docker.compose.volume":"data"}}]`, name, project)), nil
		case strings.Contains(text, "--status running"):
			return []byte("helpin-api\n"), nil
		case strings.Contains(text, "ps --all --format json"):
			return []byte(`[{"Service":"helpin-api","State":"exited","ExitCode":0}]`), nil
		default:
			return nil, nil
		}
	}
	a.run = func(_ string, args ...string) error { events = append(events, strings.Join(args, " ")); return nil }
	archive := writeVolumeTar(t, []*tar.Header{{Name: "./", Typeflag: tar.TypeDir}, {Name: "./data", Typeflag: tar.TypeReg, Size: 6}})
	a.stream = func(_ string, in io.Reader, out io.Writer, args ...string) error {
		events = append(events, strings.Join(args, " "))
		if in != nil {
			_, err := io.Copy(io.Discard, in)
			return err
		}
		_, err := out.Write(archive)
		return err
	}
	return a, root, out, &events
}
func TestBackupRestorePreservesKeysAndSeparatesVolumes(t *testing.T) {
	a, root, _, events := lifecycleFixture(t)
	backup, err := a.backup(options{dir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := validateBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Project != "helpin-fixture" || len(manifest.Volumes) != 1 {
		t.Fatal(manifest)
	}
	all := strings.Join(*events, "\n")
	if strings.Index(all, "stop --timeout") > strings.Index(all, "--entrypoint tar") || !strings.Contains(all, "start --wait --wait-timeout 300 helpin-api") {
		t.Fatal(all)
	}
	target := root + "-restored"
	if err = a.restore(options{dir: target, backupPath: backup, yes: true, noStart: true}); err != nil {
		t.Fatal(err)
	}
	values, err := readEnv(filepath.Join(target, "community"))
	if err != nil {
		t.Fatal(err)
	}
	if values["JWT_SECRET"] != "keep-this-secret" || values["COMPOSE_PROJECT_NAME"] == "helpin-fixture" {
		t.Fatal(values)
	}
	for _, event := range *events {
		if strings.Contains(event, "volume rm helpin-fixture") {
			t.Fatal("source volume removed")
		}
	}
	if err = a.restore(options{dir: target, backupPath: backup, yes: true}); err == nil {
		t.Fatal("overwrote installation")
	}
	info, _ := os.Stat(filepath.Join(target, "community/.env"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("insecure restored keys")
	}
}
func TestBackupFailureResumesServicesAndNeverCompletesManifest(t *testing.T) {
	a, root, _, events := lifecycleFixture(t)
	a.stream = func(string, io.Reader, io.Writer, ...string) error { return errors.New("disk full") }
	destination := root + "-backup"
	if _, err := a.backup(options{dir: root, backupPath: destination}, true); err == nil {
		t.Fatal("failure swallowed")
	}
	if _, err := os.Stat(filepath.Join(destination, "backup.json")); !os.IsNotExist(err) {
		t.Fatal("incomplete backup marked complete")
	}
	if !strings.Contains(strings.Join(*events, "\n"), "start --wait --wait-timeout 300 helpin-api") {
		t.Fatal("services were not resumed")
	}
}
func TestBackupRefusesExternalWritersAndUncleanStops(t *testing.T) {
	for _, scenario := range []string{"writer", "unclean"} {
		t.Run(scenario, func(t *testing.T) {
			a, root, _, _ := lifecycleFixture(t)
			output := a.output
			a.output = func(dir string, args ...string) ([]byte, error) {
				text := strings.Join(args, " ")
				if scenario == "writer" && strings.Contains(text, "--filter volume=") {
					return []byte("another-container"), nil
				}
				if scenario == "unclean" && strings.Contains(text, "ps --all --format json") {
					return []byte(`[{"Service":"postgres","State":"exited","ExitCode":137}]`), nil
				}
				return output(dir, args...)
			}
			a.stream = func(string, io.Reader, io.Writer, ...string) error { t.Fatal("snapshot started"); return nil }
			if _, err := a.backup(options{dir: root}, true); err == nil {
				t.Fatal("unsafe snapshot accepted")
			}
		})
	}
}
func TestCorruptBackupAndOccupiedRestoreDoNotMutateDocker(t *testing.T) {
	a, root, _, events := lifecycleFixture(t)
	backup, err := a.backup(options{dir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	*events = nil
	output := a.output
	a.output = func(dir string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "volume ls") {
			// Read the destination project from the same deterministic identity used by restore.
			return []byte("unrelated\n"), nil
		}
		return output(dir, args...)
	}
	put(t, filepath.Join(backup, "volume-data.tar.gz"), "corrupt")
	if err = a.restore(options{dir: root + "-restore", backupPath: backup, yes: true, noStart: true}); err == nil {
		t.Fatal("corruption accepted")
	}
	if len(*events) != 0 {
		t.Fatal("mutated Docker before validation")
	}
}
func TestUpgradeCompatibilityAndEnvironment(t *testing.T) {
	current := releaseMetadata{Tag: "community-v0.1.0"}
	next := releaseMetadata{Tag: "community-v0.2.0"}
	if checkUpgradeCompatibility(current, next) == nil {
		t.Fatal("unproven upgrade accepted")
	}
	next.UpgradeFrom = []string{current.Tag}
	next.UpgradeEvidence = "https://example.test/evidence"
	if err := checkUpgradeCompatibility(current, next); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	old := filepath.Join(root, "old")
	target := filepath.Join(root, "next")
	put(t, old, "JWT_SECRET=preserve\nOPTIONAL=value\n")
	put(t, target, "JWT_SECRET=rotate\nNEW_SECRET=new\nOPTIONAL=default\n")
	if err := mergeUpgradeEnvironment(old, target); err != nil {
		t.Fatal(err)
	}
	values := envValues(get(t, target))
	if values["JWT_SECRET"] != "preserve" || values["NEW_SECRET"] != "new" || values["OPTIONAL"] != "value" {
		t.Fatal(values)
	}
	config := composeConfig{Name: "project", Volumes: map[string]composeVolume{"data": {Name: "project_data"}}, Services: map[string]composeService{"postgres": {Image: "old"}}}
	nextConfig := config
	nextConfig.Services = map[string]composeService{"postgres": {Image: "new"}}
	if sameStorage(config, nextConfig) == nil {
		t.Fatal("storage upgrade allowed without migration")
	}
}
func TestBackupManifestRejectsTraversalAndIncompleteInventory(t *testing.T) {
	a, root, _, _ := lifecycleFixture(t)
	backup, err := a.backup(options{dir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	original := get(t, filepath.Join(backup, "backup.json"))
	for _, edit := range []func(*backupManifest){
		func(m *backupManifest) { m.Volumes["data"] = "../outside" },
		func(m *backupManifest) { delete(m.Files, "installation.tar.gz") },
		func(m *backupManifest) { m.Format = 99 },
	} {
		var m backupManifest
		json.Unmarshal([]byte(original), &m)
		edit(&m)
		data, _ := json.Marshal(m)
		put(t, filepath.Join(backup, "backup.json"), string(data))
		if _, err := validateBackup(backup); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}

func TestRestoreRejectsOccupiedVolumesAndArchitectureBeforeMutation(t *testing.T) {
	for _, scenario := range []string{"occupied", "architecture", "daemon"} {
		t.Run(scenario, func(t *testing.T) {
			a, root, _, events := lifecycleFixture(t)
			backup, err := a.backup(options{dir: root}, true)
			if err != nil {
				t.Fatal(err)
			}
			*events = nil
			output := a.output
			var project string
			a.output = func(dir string, args ...string) ([]byte, error) {
				text := strings.Join(args, " ")
				if strings.Contains(text, "config --format json") {
					env, _ := readEnv(dir)
					project = env["COMPOSE_PROJECT_NAME"]
				}
				if strings.Contains(text, "docker info") && scenario == "architecture" {
					return []byte("aarch64"), nil
				}
				if strings.Contains(text, "volume ls") {
					if scenario == "occupied" {
						return []byte(project + "_data\n"), nil
					}
					if scenario == "daemon" {
						return nil, errors.New("Docker unavailable")
					}
				}
				return output(dir, args...)
			}
			if err = a.restore(options{dir: root + "-restore", backupPath: backup, yes: true, noStart: true}); err == nil {
				t.Fatal("unsafe restore accepted")
			}
			if len(*events) != 0 {
				t.Fatalf("mutated Docker before refusal: %v", *events)
			}
		})
	}
}
func TestRestoreCleansOnlyNewVolumesOnImportFailure(t *testing.T) {
	a, root, _, events := lifecycleFixture(t)
	backup, err := a.backup(options{dir: root}, true)
	if err != nil {
		t.Fatal(err)
	}
	*events = nil
	a.stream = func(string, io.Reader, io.Writer, ...string) error { return errors.New("import failed") }
	destination := root + "-restored"
	if err = a.restore(options{dir: destination, backupPath: backup, yes: true, noStart: true}); err == nil {
		t.Fatal("failure swallowed")
	}
	all := strings.Join(*events, "\n")
	if !strings.Contains(all, "docker volume rm helpin-") || strings.Contains(all, "volume rm helpin-fixture_data") {
		t.Fatal(all)
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("partial installation exposed")
	}
}
func TestUpgradeRefusesEditedAndUnmanagedFiles(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "community/compose.yaml"), "original")
	digest, err := hashFile(filepath.Join(root, "community/compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "SHA256SUMS"), digest+"  community/compose.yaml\n")
	put(t, filepath.Join(root, "community/.env"), "SECRET=operator-choice")
	if err = verifyManagedFiles(root); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "community/compose.yaml"), "customized")
	if err = verifyManagedFiles(root); err == nil {
		t.Fatal("modified release accepted")
	}
	put(t, filepath.Join(root, "community/compose.yaml"), "original")
	put(t, filepath.Join(root, "local-file"), "keep me")
	if err = verifyManagedFiles(root); err == nil {
		t.Fatal("unmanaged file would be lost")
	}
}
func TestCommandEnvironmentIncludesLocalAppConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_RUNTIME_EXECUTION_APP_CONFIG", "wrong installation")
	put(t, filepath.Join(dir, "apps.json"), `{"trusted":"local"}`)
	var values []string
	for _, entry := range commandEnv(dir) {
		if strings.HasPrefix(entry, "AGENT_RUNTIME_EXECUTION_APP_CONFIG=") {
			values = append(values, entry)
		}
	}
	if len(values) != 1 || values[0] != `AGENT_RUNTIME_EXECUTION_APP_CONFIG={"trusted":"local"}` {
		t.Fatal(values)
	}
}
