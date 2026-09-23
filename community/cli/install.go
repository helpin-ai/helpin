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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const repository = "https://github.com/helpin-ai/helpin"

func (a *app) releaseTag(requested string) (string, error) {
	if requested == "" && tagPattern.MatchString(version) {
		requested = version
	}
	return a.latestReleaseTag(requested)
}

func (a *app) latestReleaseTag(requested string) (string, error) {
	if requested != "" {
		if !tagPattern.MatchString(requested) {
			return "", errors.New("invalid --version; expected community-v0.x.y[-rc.n]")
		}
		return requested, nil
	}
	resp, err := a.client.Get("https://api.github.com/repos/helpin-ai/helpin/releases?per_page=100")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("cannot discover a published Community release (HTTP %d); use --version", resp.StatusCode)
	}
	var releases []struct {
		Tag   string `json:"tag_name"`
		Draft bool   `json:"draft"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return "", err
	}
	for _, release := range releases {
		if !release.Draft && tagPattern.MatchString(release.Tag) {
			return release.Tag, nil
		}
	}
	return "", errors.New("no published Community release is available yet; see the Community source-build guide")
}

func download(client *http.Client, url, destination string, limit int64) error {
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("download failed (HTTP %d): %s", response.StatusCode, url)
	}
	f, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(response.Body, limit+1))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if n > limit {
		return errors.New("download exceeded the size limit")
	}
	return closeErr
}

func verify(archive, checksum string) error {
	data, err := os.ReadFile(checksum)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 || fields[1] != filepath.Base(archive) {
		return errors.New("checksum file must contain exactly the archive's SHA-256 and filename")
	}
	expected, err := hex.DecodeString(fields[0])
	if err != nil || len(expected) != sha256.Size {
		return errors.New("invalid SHA-256 checksum")
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(fields[0]) {
		return errors.New("release checksum mismatch; nothing was installed")
	}
	return nil
}

// Only ordinary files and directories under the bundle root are accepted.
// Never allow archive links, traversal, duplicate entries or unbounded expansion.
func extract(archive, destination string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	seen := map[string]bool{}
	var total int64
	for count := 0; ; count++ {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		clean := path.Clean(name)
		if clean != name || strings.Contains(name, "\\") || (name != "helpin-community" && !strings.HasPrefix(name, "helpin-community/")) {
			return errors.New("unsafe bundle path")
		}
		if seen[name] || count > 10000 {
			return errors.New("duplicate or excessive bundle entries")
		}
		seen[name] = true
		if header.Size < 0 || header.Size > (128<<20)-total {
			return errors.New("bundle exceeds extraction size limit")
		}
		total += header.Size
		target := filepath.Join(destination, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(name, "helpin-community"), "/")))
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := os.FileMode(0644)
			if header.Mode&0111 != 0 {
				mode = 0755
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, reader)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return errors.New("bundle links and special files are not allowed")
		}
	}
}

func (a *app) install(o options) error {
	if (o.bundle == "") != (o.checksum == "") {
		return errors.New("--bundle and --checksum must be supplied together")
	}
	parent := filepath.Dir(o.dir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	unlock, err := lock(o.dir)
	if err != nil {
		return err
	}
	defer unlock()
	if info, err := os.Lstat(o.dir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("installation destination must be a directory, not a symlink")
		}
		if _, err = os.Stat(filepath.Join(o.dir, "community", ".env")); err == nil {
			fmt.Fprintf(a.out, "Existing installation preserved at %s.\nUse helpin configure, start, or status with --dir %q. Use helpin upgrade for a compatible release.\n", o.dir, o.dir)
			return nil
		}
		entries, err := os.ReadDir(o.dir)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return errors.New("installation directory is not empty; choose a new --dir")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := a.configuration(&o, map[string]string{}); err != nil {
		return err
	}
	if err := a.integrations(&o, map[string]string{}); err != nil {
		return err
	}
	if err := a.prerequisites(); err != nil {
		return err
	}
	if err := checkPorts(o); err != nil {
		return err
	}
	disk, err := a.output("", "df", "-Pk", parent)
	if err != nil {
		return errors.New("cannot check free disk space with df -Pk")
	}
	lines := strings.Split(strings.TrimSpace(string(disk)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return errors.New("cannot read available disk space")
	}
	available, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || available < 20*1024*1024 {
		return errors.New("Community evaluation requires at least 20 GiB free disk")
	}
	fmt.Fprintln(a.out, "✓ Disk and port checks passed")
	stage, err := os.MkdirTemp(parent, ".helpin-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	tag := o.release
	if o.bundle == "" {
		tag, err = a.releaseTag(tag)
		if err != nil {
			return err
		}
		name := "helpin-" + tag + ".tar.gz"
		o.bundle = filepath.Join(stage, name)
		o.checksum = o.bundle + ".sha256"
		fmt.Fprintln(a.out, "Downloading", tag, "…")
		base := repository + "/releases/download/" + tag + "/"
		if err = download(a.client, base+name, o.bundle, 64<<20); err != nil {
			return err
		}
		if err = download(a.client, base+name+".sha256", o.checksum, 4096); err != nil {
			return err
		}
	}
	if err = verify(o.bundle, o.checksum); err != nil {
		return err
	}
	bundle := filepath.Join(stage, "bundle")
	if err = os.Mkdir(bundle, 0700); err != nil {
		return err
	}
	if err = extract(o.bundle, bundle); err != nil {
		return err
	}
	var metadata struct {
		Tag string `json:"tag"`
	}
	data, err := os.ReadFile(filepath.Join(bundle, "release-evidence", "release.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	if !tagPattern.MatchString(metadata.Tag) || (tag != "" && tag != metadata.Tag) {
		return errors.New("bundle release does not match requested version")
	}
	dir := filepath.Join(bundle, "community")
	for _, name := range []string{"setup.sh", "compose.yaml", ".env.example", "apps.example.json"} {
		if _, err = os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("incomplete release bundle: %s", name)
		}
	}
	fmt.Fprintln(a.out, "✓ Release downloaded and verified")
	if err = a.run(dir, "bash", "./setup.sh", "install"); err != nil {
		return err
	}
	// Distinct installation directories must never share Compose volumes.
	identity := sha256.Sum256([]byte(o.dir))
	o.project = fmt.Sprintf("helpin-%x", identity[:6])
	if err = writeConfiguration(dir, o); err != nil {
		return err
	}
	if err = writeIntegrations(dir, o); err != nil {
		return err
	}
	if _, err = a.output(dir, "docker", "compose", "--env-file", ".env", "-f", "compose.yaml", "config", "--quiet"); err != nil {
		return errors.New("generated Compose configuration is invalid")
	}
	if err = os.Rename(bundle, o.dir); err != nil {
		return err
	}
	dir = filepath.Join(o.dir, "community")
	fmt.Fprintf(a.out, "✓ Configuration and secrets saved at %s\n", o.dir)
	if o.mode == "server" {
		fmt.Fprintf(a.out, "Proxy configuration: %s\nConfigure your host Caddy service with this file and point DNS here. See docs/community/deployment.md in the installation.\n", filepath.Join(dir, "Caddyfile"))
	}
	if o.noStart {
		fmt.Fprintf(a.out, "Start with: helpin start --dir %q\n", o.dir)
		return nil
	}
	fmt.Fprintln(a.out, "Starting Helpin (first image download may take several minutes)…")
	if err = a.run(dir, "bash", "./setup.sh", "start"); err != nil {
		return fmt.Errorf("startup did not finish; configuration and data were preserved. Inspect helpin logs --dir %q, then retry helpin start --dir %q: %w", o.dir, o.dir, err)
	}
	return a.ready(dir)
}
