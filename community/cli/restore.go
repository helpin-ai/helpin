package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func validateBackup(directory string) (backupManifest, error) {
	var manifest backupManifest
	info, err := os.Lstat(directory)
	if err != nil {
		return manifest, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return manifest, errors.New("backup must be an ordinary directory")
	}
	data, err := os.ReadFile(filepath.Join(directory, "backup.json"))
	if err != nil {
		return manifest, err
	}
	if len(data) > 1<<20 {
		return manifest, errors.New("backup manifest is too large")
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Format != 1 || !namePattern.MatchString(manifest.Architecture) || !tagPattern.MatchString(manifest.Release) || !namePattern.MatchString(manifest.Project) || len(manifest.Volumes) == 0 || len(manifest.Volumes) > 100 || len(manifest.Files) != len(manifest.Volumes)+1 {
		return manifest, errors.New("invalid or unsupported backup manifest")
	}
	expected := map[string]bool{"installation.tar.gz": true}
	for key, file := range manifest.Volumes {
		if !namePattern.MatchString(key) || file != "volume-"+key+".tar.gz" {
			return manifest, errors.New("invalid backup volume path")
		}
		expected[file] = true
	}
	for name, digest := range manifest.Files {
		if !expected[name] || len(digest) != 64 {
			return manifest, errors.New("invalid backup file inventory")
		}
		file := filepath.Join(directory, name)
		info, err = os.Lstat(file)
		if err != nil {
			return manifest, err
		}
		if !info.Mode().IsRegular() {
			return manifest, errors.New("backup files must not be links or special files")
		}
		actual, e := hashFile(file)
		if e != nil {
			return manifest, e
		}
		if digest != actual {
			return manifest, fmt.Errorf("backup checksum mismatch: %s", name)
		}
		if name != "installation.tar.gz" {
			if e = validateVolumeArchive(file); e != nil {
				return manifest, e
			}
		}
	}
	return manifest, nil
}

func replaceEnvValue(filename, key, value string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
		}
	}
	if !found {
		return fmt.Errorf("configuration missing %s", key)
	}
	return atomicWrite(filename, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func (a *app) restore(o options) (err error) {
	if o.backupPath == "" {
		if o.yes {
			return errors.New("restore requires --backup")
		}
		o.backupPath, err = a.ask("Backup directory", "")
		if err != nil {
			return err
		}
	}
	if err = os.MkdirAll(filepath.Dir(o.dir), 0700); err != nil {
		return err
	}
	o.dir, err = canonicalDestination(o.dir)
	if err != nil {
		return err
	}
	unlock, err := lock(o.dir)
	if err != nil {
		return err
	}
	defer unlock()
	// No replacement option: recovery must not overwrite another database or keys.
	if _, err = os.Lstat(o.dir); !os.IsNotExist(err) {
		return errors.New("restore requires a new --dir; existing installations are never overwritten")
	}
	backupPath, err := filepath.Abs(o.backupPath)
	if err != nil {
		return err
	}
	manifest, err := validateBackup(backupPath)
	if err != nil {
		return err
	}
	architecture, err := a.output("", "docker", "info", "--format", "{{.Architecture}}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(architecture)) != manifest.Architecture {
		return errors.New("cold volume restore requires the same Docker architecture as the backup")
	}
	stage, err := os.MkdirTemp(filepath.Dir(o.dir), ".helpin-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err = extract(filepath.Join(backupPath, "installation.tar.gz"), stage); err != nil {
		return err
	}
	metadata, err := readRelease(stage)
	if err != nil {
		return err
	}
	if metadata.Tag != manifest.Release {
		return errors.New("backup release identity does not match its installation")
	}
	dir := filepath.Join(stage, "community")
	env, err := readEnv(dir)
	if err != nil {
		return err
	}
	if env["COMPOSE_PROJECT_NAME"] != manifest.Project {
		return errors.New("backup project identity does not match its configuration")
	}
	identity := sha256.Sum256([]byte(o.dir))
	project := fmt.Sprintf("helpin-%x", identity[:6])
	if project == manifest.Project {
		return errors.New("restore must use a different directory and project from the source installation")
	}
	if err = replaceEnvValue(filepath.Join(dir, ".env"), "COMPOSE_PROJECT_NAME", project); err != nil {
		return err
	}
	if err = os.Chmod(filepath.Join(dir, "apps.json"), 0644); err != nil {
		return err
	}
	config, err := a.configurationForBackup(dir)
	if err != nil {
		return err
	}
	if len(config.Volumes) != len(manifest.Volumes) {
		return errors.New("backup volume inventory does not match Compose")
	}
	// Query the complete volume list successfully: an arbitrary inspect failure is
	// not proof of absence (it may be a daemon or permissions error).
	existing, err := a.output("", "docker", "volume", "ls", "--format", "{{.Name}}")
	if err != nil {
		return err
	}
	occupied := map[string]bool{}
	for _, name := range strings.Fields(string(existing)) {
		occupied[name] = true
	}
	for key, volume := range config.Volumes {
		if _, ok := manifest.Volumes[key]; !ok {
			return errors.New("missing volume in backup")
		}
		if occupied[volume.Name] {
			return fmt.Errorf("restore volume %s already exists; choose a new --dir", volume.Name)
		}
	}
	containers, err := a.output("", "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(containers)) != "" {
		return errors.New("restore project already has containers; choose a new --dir")
	}
	if err = a.confirm(o.yes, "Restore "+manifest.Release+" into "+o.dir+" using new volumes?"); err != nil {
		return err
	}
	if err = a.ensureArchiveImage(); err != nil {
		return err
	}
	// Keep failed restores inspectable but never expose an incomplete installation
	// as startable. Remove only volumes this invocation created and owns.
	var created []string
	installed := false
	defer func() {
		if !installed {
			for _, key := range created {
				name := config.Volumes[key].Name
				if e := a.inspectVolume(name, project, key); e != nil {
					err = errors.Join(err, e)
					continue
				}
				if e := a.run("", "docker", "volume", "rm", name); e != nil {
					err = errors.Join(err, fmt.Errorf("remove incomplete restore volume %s manually after checking it is unused: %w", name, e))
				}
			}
		}
	}()
	for _, key := range sortedKeys(config.Volumes) {
		name := config.Volumes[key].Name
		if err = a.run("", "docker", "volume", "create", "--label", "com.docker.compose.project="+project, "--label", "com.docker.compose.volume="+key, name); err != nil {
			return err
		}
		created = append(created, key)
		if err = a.inspectVolume(name, project, key); err != nil {
			return err
		}
		file, e := os.Open(filepath.Join(backupPath, manifest.Volumes[key]))
		if e != nil {
			return e
		}
		e = a.stream(dir, file, io.Discard, helperArgs(name, true)...)
		err = errors.Join(e, file.Close())
		if err != nil {
			return err
		}
	}
	if err = os.Rename(stage, o.dir); err != nil {
		return err
	}
	installed = true
	fmt.Fprintf(a.out, "✓ Restored %s at %s. Original volumes were preserved.\n", manifest.Release, o.dir)
	if o.noStart {
		fmt.Fprintf(a.out, "Start with: helpin start --dir %q\n", o.dir)
		return nil
	}
	dir = filepath.Join(o.dir, "community")
	if err = a.compose(dir, "up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		return fmt.Errorf("data restored; startup failed (check port conflicts and image availability), retry helpin start --dir %q: %w", o.dir, err)
	}
	if err = a.ready(dir); err != nil {
		return err
	}
	return a.compose(dir, "run", "--rm", "--no-deps", "helpin-migrate", "validate")
}
