package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// An immutable, multi-platform Alpine image supplies tar; no host mounts or network
// are exposed to archive helpers. Images are fetched before services are stopped.
const archiveImage = "alpine@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc"

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
var immutableImage = regexp.MustCompile(`(^sha256:|@sha256:)[a-f0-9]{64}$`)

type releaseMetadata struct {
	Tag             string   `json:"tag"`
	UpgradeFrom     []string `json:"upgrade_from,omitempty"`
	UpgradeEvidence string   `json:"upgrade_evidence,omitempty"`
}

type composeVolume struct {
	Name       string            `json:"name"`
	External   bool              `json:"external"`
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driver_opts"`
}
type composeService struct {
	Image   string `json:"image"`
	Volumes []struct {
		Type, Source, Target string
		ReadOnly             bool `json:"read_only"`
	} `json:"volumes"`
}
type composeConfig struct {
	Name     string                    `json:"name"`
	Services map[string]composeService `json:"services"`
	Volumes  map[string]composeVolume  `json:"volumes"`
}
type backupManifest struct {
	Architecture string            `json:"architecture"`
	Format       int               `json:"format"`
	Release      string            `json:"release"`
	Project      string            `json:"project"`
	Created      string            `json:"created"`
	Volumes      map[string]string `json:"volumes"`
	Files        map[string]string `json:"files"`
}

func (a *app) confirm(yes bool, question string) error {
	if yes {
		return nil
	}
	answer, err := a.ask(question+" Type yes to continue", "no")
	if err != nil {
		return err
	}
	if answer != "yes" {
		return errors.New("operation cancelled")
	}
	return nil
}
func composeArgs(args ...string) []string {
	return append([]string{"docker", "compose", "--env-file", ".env", "-f", "compose.yaml"}, args...)
}
func (a *app) compose(dir string, args ...string) error { return a.run(dir, composeArgs(args...)...) }
func readRelease(root string) (releaseMetadata, error) {
	var metadata releaseMetadata
	data, err := os.ReadFile(filepath.Join(root, "release-evidence", "release.json"))
	if err == nil {
		err = json.Unmarshal(data, &metadata)
	}
	if err == nil && !tagPattern.MatchString(metadata.Tag) {
		err = errors.New("invalid release identity")
	}
	return metadata, err
}
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func within(root, child string) bool {
	rel, err := filepath.Rel(root, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func canonicalDestination(destination string) (string, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}
func (a *app) configurationForBackup(dir string) (composeConfig, error) {
	var config composeConfig
	data, err := a.output(dir, composeArgs("config", "--format", "json")...)
	if err != nil {
		return config, errors.New("cannot resolve Compose configuration; check .env and apps.json")
	}
	if err = json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	if !namePattern.MatchString(config.Name) || len(config.Services) == 0 || len(config.Volumes) == 0 {
		return config, errors.New("installation needs an explicit Compose project, services, and named volumes")
	}
	for key, volume := range config.Volumes {
		if !namePattern.MatchString(key) || volume.Name != config.Name+"_"+key || volume.External || (volume.Driver != "" && volume.Driver != "local") || len(volume.DriverOpts) != 0 {
			return config, fmt.Errorf("volume %s is external or customized; use your storage provider's backup procedure", key)
		}
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil {
		return config, err
	}
	for name, service := range config.Services {
		if !namePattern.MatchString(name) || !immutableImage.MatchString(service.Image) {
			return config, fmt.Errorf("service %s must use an immutable image from a release bundle", name)
		}
		for _, mount := range service.Volumes {
			switch mount.Type {
			case "volume":
				if _, ok := config.Volumes[mount.Source]; !ok {
					return config, errors.New("anonymous volumes cannot be backed up")
				}
			case "bind":
				source, e := filepath.EvalSymlinks(mount.Source)
				if e != nil || !mount.ReadOnly || !within(root, source) {
					return config, fmt.Errorf("service %s has a writable or external bind mount; only bundle-local read-only files are supported", name)
				}
			case "tmpfs":
			default:
				return config, fmt.Errorf("unsupported mount type %q", mount.Type)
			}
		}
	}
	return config, nil
}
func (a *app) ensureArchiveImage() error {
	if _, err := a.output("", "docker", "image", "inspect", archiveImage); err == nil {
		return nil
	}
	return a.run("", "docker", "pull", archiveImage)
}
func (a *app) inspectVolume(name, project, key string) error {
	data, err := a.output("", "docker", "volume", "inspect", name)
	if err != nil {
		return fmt.Errorf("volume %s is missing or Docker is unavailable: %w", name, err)
	}
	var volumes []struct {
		Name, Driver    string
		Labels, Options map[string]string
	}
	if err = json.Unmarshal(data, &volumes); err != nil {
		return err
	}
	if len(volumes) != 1 || volumes[0].Name != name || volumes[0].Driver != "local" || len(volumes[0].Options) != 0 || volumes[0].Labels["com.docker.compose.project"] != project || volumes[0].Labels["com.docker.compose.volume"] != key {
		return fmt.Errorf("volume %s is not owned by this installation", name)
	}
	return nil
}
func (a *app) noVolumeWriters(config composeConfig) error {
	for _, volume := range config.Volumes {
		data, err := a.output("", "docker", "ps", "-q", "--filter", "volume="+volume.Name)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(data)) != "" {
			return fmt.Errorf("volume %s still has a running container; backup refused", volume.Name)
		}
	}
	return nil
}
func (a *app) runningServices(dir string) ([]string, error) {
	data, err := a.output(dir, composeArgs("ps", "--status", "running", "--services")...)
	if err != nil {
		return nil, err
	}
	services := strings.Fields(string(data))
	for _, name := range services {
		if !namePattern.MatchString(name) {
			return nil, errors.New("invalid running service name")
		}
	}
	return services, nil
}
func (a *app) checkCleanStop(dir string) error {
	data, err := a.output(dir, composeArgs("ps", "--all", "--format", "json")...)
	if err != nil {
		return err
	}
	type state struct {
		Service, State string
		ExitCode       int
	}
	var states []state
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		err = json.Unmarshal(data, &states)
	} else {
		dec := json.NewDecoder(strings.NewReader(string(data)))
		for {
			var s state
			e := dec.Decode(&s)
			if e == io.EOF {
				break
			}
			if e != nil {
				err = e
				break
			}
			states = append(states, s)
		}
	}
	if err != nil {
		return err
	}
	for _, s := range states {
		if s.State == "running" || s.State == "restarting" {
			return fmt.Errorf("%s did not stop cleanly; restart and investigate before backing up", s.Service)
		}
		// A forced kill (137) can leave a data volume inconsistent, so it blocks
		// the backup for services that write volumes. Stateless services such as
		// the help center (older images ignore SIGTERM) hold nothing to snapshot.
		if s.ExitCode == 137 && volumeServices[s.Service] {
			return fmt.Errorf("%s did not stop cleanly; restart and investigate before backing up", s.Service)
		}
	}
	return nil
}

// volumeServices are the Compose services that write named volumes included in
// backups. A test keeps this in sync with compose.yaml.
var volumeServices = map[string]bool{
	"postgres":             true,
	"redis":                true,
	"nats":                 true,
	"garage":               true,
	"agent-runtime-worker": true,
}

func helperArgs(volume string, restore bool) []string {
	mount := "type=volume,src=" + volume + ",dst=/data,volume-nocopy"
	if !restore {
		mount += ",readonly"
	}
	args := []string{"docker", "run", "--rm", "--pull", "never", "--network", "none", "--read-only", "--security-opt", "no-new-privileges", "--cap-drop", "ALL", "--cap-add", "DAC_OVERRIDE"}
	if restore {
		args = append(args, "--cap-add", "CHOWN", "--cap-add", "FOWNER", "-i")
	}
	args = append(args, "--mount", mount, "--entrypoint", "tar", archiveImage)
	if restore {
		return append(args, "-xzpf", "-", "-C", "/data")
	}
	return append(args, "-czf", "-", "-C", "/data", ".")
}
func hashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func archiveInstallation(root, destination string) (err error) {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	gz := gzip.NewWriter(f)
	defer func() { err = errors.Join(err, gz.Close()) }()
	tw := tar.NewWriter(gz)
	defer func() { err = errors.Join(err, tw.Close()) }()
	var total int64
	return filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("installation contains a link or special file: %s", filename)
		}
		total += info.Size()
		if total > 128<<20 {
			return errors.New("installation files exceed 128 MiB; keep backups and user data outside the bundle")
		}
		rel, e := filepath.Rel(root, filename)
		if e != nil {
			return e
		}
		header, e := tar.FileInfoHeader(info, "")
		if e != nil {
			return e
		}
		header.Name = path.Join("helpin-community", filepath.ToSlash(rel))
		// Secrets in the archive are private even when a restore extracts it.
		if !info.IsDir() {
			header.Mode = int64(info.Mode().Perm() & 0700)
		}
		if e = tw.WriteHeader(header); e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		in, e := os.Open(filename)
		if e != nil {
			return e
		}
		_, e = io.Copy(tw, in)
		return errors.Join(e, in.Close())
	})
}

// Validate paths and link targets before handing an archive to container tar.
// Files are streamed, so database size does not determine CLI memory usage.
func validateVolumeArchive(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	links := map[string]bool{}
	var total int64
	for count := 0; ; count++ {
		h, e := tr.Next()
		if e == io.EOF {
			_, err = io.Copy(io.Discard, gz)
			return err
		}
		if e != nil {
			return e
		}
		name := strings.TrimSuffix(strings.TrimPrefix(h.Name, "./"), "/")
		if name == "" {
			name = "."
		}
		if path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || seen[name] || count > 10000000 {
			return errors.New("unsafe or duplicate volume archive path")
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if links[parent] {
				return errors.New("archive entry traverses a symlink")
			}
		}
		seen[name] = true
		total += h.Size
		if h.Size < 0 || total > 1<<40 {
			return errors.New("volume archive exceeds 1 TiB expanded size")
		}
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeDir:
		case tar.TypeSymlink:
			target := path.Clean(path.Join(path.Dir(name), h.Linkname))
			if h.Linkname == "" || path.IsAbs(h.Linkname) || strings.Contains(h.Linkname, "\\") || target == ".." || strings.HasPrefix(target, "../") {
				return errors.New("volume symlink escapes its archive")
			}
			for entry := range seen {
				if strings.HasPrefix(entry, name+"/") {
					return errors.New("symlink replaces an archive directory")
				}
			}
			links[name] = true
		default:
			return errors.New("volume archive contains hard links or special files; use a storage-specific backup")
		}
	}
}

// The caller holds the installation lock. A complete manifest is written last;
// incomplete directories are never accepted by restore.
func (a *app) backup(o options, resume bool) (destination string, err error) {
	root, err := filepath.EvalSymlinks(o.dir)
	if err != nil {
		return "", err
	}
	if root != o.dir {
		return "", errors.New("use the canonical installation path for backup and upgrade")
	}
	dir := filepath.Join(root, "community")
	metadata, err := readRelease(root)
	if err != nil {
		return "", err
	}
	config, err := a.configurationForBackup(dir)
	if err != nil {
		return "", err
	}
	for key, volume := range config.Volumes {
		if err = a.inspectVolume(volume.Name, config.Name, key); err != nil {
			return "", err
		}
	}
	if o.backupPath == "" {
		o.backupPath = root + "-backup-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	destination, err = canonicalDestination(o.backupPath)
	if err != nil {
		return "", err
	}
	if within(root, destination) {
		return "", errors.New("backup destination must be outside the installation")
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return "", fmt.Errorf("backup requires a new directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			err = errors.Join(err, os.RemoveAll(destination))
		}
	}()
	if err = a.ensureArchiveImage(); err != nil {
		return destination, err
	}
	running, err := a.runningServices(dir)
	if err != nil {
		return destination, err
	}
	// On failure attempt to resume exactly the services that were running. A
	// successful upgrade backup leaves the whole stack stopped for replacement.
	defer func() {
		if (resume || !complete) && len(running) > 0 {
			if e := a.resumeServices(dir, running, 5*time.Minute); e != nil {
				err = errors.Join(err, fmt.Errorf("backup at %s; service restart failed, run helpin start --dir %q: %w", destination, root, e))
			}
		}
	}()
	fmt.Fprintln(a.out, "Stopping services for a consistent volume snapshot…")
	if err = a.compose(dir, "stop", "--timeout", "180"); err != nil {
		return destination, err
	}
	if err = a.checkCleanStop(dir); err != nil {
		return destination, err
	}
	if err = a.noVolumeWriters(config); err != nil {
		return destination, err
	}
	architecture, err := a.output("", "docker", "info", "--format", "{{.Architecture}}")
	if err != nil || strings.TrimSpace(string(architecture)) == "" {
		return destination, errors.New("cannot determine Docker architecture")
	}
	manifest := backupManifest{Architecture: strings.TrimSpace(string(architecture)), Format: 1, Release: metadata.Tag, Project: config.Name, Created: time.Now().UTC().Format(time.RFC3339), Volumes: map[string]string{}, Files: map[string]string{}}
	if err = archiveInstallation(root, filepath.Join(destination, "installation.tar.gz")); err != nil {
		return destination, err
	}
	for _, key := range sortedKeys(config.Volumes) {
		name := "volume-" + key + ".tar.gz"
		filename := filepath.Join(destination, name)
		f, e := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return destination, e
		}
		e = a.stream(dir, nil, f, helperArgs(config.Volumes[key].Name, false)...)
		err = errors.Join(e, f.Sync(), f.Close())
		if err != nil {
			return destination, err
		}
		if err = validateVolumeArchive(filename); err != nil {
			return destination, err
		}
		manifest.Volumes[key] = name
	}
	names := []string{"installation.tar.gz"}
	for _, file := range manifest.Volumes {
		names = append(names, file)
	}
	for _, name := range names {
		digest, e := hashFile(filepath.Join(destination, name))
		if e != nil {
			return destination, e
		}
		manifest.Files[name] = digest
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return destination, err
	}
	if err = atomicWrite(filepath.Join(destination, "backup.json"), append(data, '\n'), 0600); err != nil {
		return destination, err
	}
	complete = true
	fmt.Fprintf(a.out, "✓ Backup complete: %s\nContains encryption keys and private data; keep an encrypted copy off-host.\n", destination)
	return destination, nil
}

// Older Compose v2 releases support up --wait but not start --wait. Start only
// the previously running services, then poll their state without recreating them.
func (a *app) resumeServices(dir string, services []string, timeout time.Duration) error {
	if err := a.compose(dir, append([]string{"start"}, services...)...); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		data, err := a.output(dir, composeArgs("ps", "--all", "--format", "json")...)
		if err != nil {
			return err
		}
		if err = checkServiceStates(data, services); err == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out waiting for resumed services: %w", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
