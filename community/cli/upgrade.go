package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func checkUpgradeCompatibility(current releaseMetadata, target releaseMetadata) error {
	if current.Tag == target.Tag {
		return errors.New("this release is already installed")
	}
	for _, from := range target.UpgradeFrom {
		if from == current.Tag && strings.HasPrefix(target.UpgradeEvidence, "https://") && !strings.ContainsAny(target.UpgradeEvidence, " \r\n\t") {
			return nil
		}
	}
	return fmt.Errorf("release %s does not declare a tested upgrade from %s; installation was not changed", target.Tag, current.Tag)
}
// retiredDefaults lists values that earlier bundles wrote as defaults. An
// unchanged retired default follows the new release default on upgrade; any
// operator-customized value is preserved exactly.
var retiredDefaults = map[string][]string{
	// Community 0.1 enabled only support, docs and agents by default.
	"HELPIN_ENABLED_MODULES": {"support,docs,agents"},
}

func isRetiredDefault(key, value string) bool {
	for _, retired := range retiredDefaults[key] {
		if strings.TrimSpace(value) == retired {
			return true
		}
	}
	return false
}

func mergeUpgradeEnvironment(old, target string) error {
	existing, err := os.ReadFile(old)
	if err != nil {
		return err
	}
	defaults, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	targetValues := envValues(string(defaults))
	lines := strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !isRetiredDefault(key, value) {
			continue
		}
		if next, found := targetValues[key]; found && strings.TrimSpace(next) != "" {
			lines[i] = key + "=" + next
		}
	}
	values := envValues(string(existing))
	merged := strings.Join(lines, "\n") + "\n"
	for _, line := range strings.Split(string(defaults), "\n") {
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		if _, found := values[key]; !found {
			merged += line + "\n"
		}
	}
	return atomicWrite(target, []byte(merged), 0600)
}
func copyPrivateFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return atomicWrite(to, data, 0600)
}
func sameStorage(old, target composeConfig) error {
	if old.Name != target.Name || len(old.Volumes) != len(target.Volumes) {
		return errors.New("upgrade changes project or volume layout; a dedicated data migration is required")
	}
	for key, volume := range old.Volumes {
		if target.Volumes[key].Name != volume.Name {
			return errors.New("upgrade changes volume identity")
		}
	}
	// Cold snapshots cannot make arbitrary storage-engine upgrades compatible.
	// Keep these exact images until a dedicated migration path is implemented.
	for _, service := range []string{"postgres", "redis", "nats", "garage", "temporal", "temporal-schema", "temporal-namespace"} {
		if old.Services[service].Image != target.Services[service].Image {
			return fmt.Errorf("upgrade changes %s infrastructure; use a dedicated migration procedure", service)
		}
	}
	return nil
}

func (a *app) upgrade(o options) (err error) {
	if (o.bundle == "") != (o.checksum == "") {
		return errors.New("--bundle and --checksum must be supplied together")
	}
	current, err := readRelease(o.dir)
	if err != nil {
		return err
	}
	if err = verifyManagedFiles(o.dir); err != nil {
		return err
	}
	dir := filepath.Join(o.dir, "community")
	oldConfig, err := a.configurationForBackup(dir)
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(o.dir), ".helpin-upgrade-*")
	if err != nil {
		return err
	}
	preservePrevious := false
	defer func() {
		if !preservePrevious {
			os.RemoveAll(stage)
		}
	}()
	tag := o.release
	if o.bundle == "" {
		// Upgrade discovery deliberately ignores the version compiled into this CLI.
		tag, err = a.latestReleaseTag(tag)
		if err != nil {
			return err
		}
		name := "helpin-" + tag + ".tar.gz"
		o.bundle, o.checksum = filepath.Join(stage, name), filepath.Join(stage, name+".sha256")
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
	targetRoot := filepath.Join(stage, "bundle")
	if err = os.Mkdir(targetRoot, 0700); err != nil {
		return err
	}
	if err = extract(o.bundle, targetRoot); err != nil {
		return err
	}
	target, err := readRelease(targetRoot)
	if err != nil {
		return err
	}
	if tag != "" && tag != target.Tag {
		return errors.New("bundle release does not match requested version")
	}
	if err = checkUpgradeCompatibility(current, target); err != nil {
		return err
	}
	next := filepath.Join(targetRoot, "community")
	// Generate only newly introduced settings, then restore every existing value,
	// including encryption keys, credentials, optional settings and project name.
	if err = a.run(next, "bash", "./setup.sh", "install"); err != nil {
		return err
	}
	if err = mergeUpgradeEnvironment(filepath.Join(dir, ".env"), filepath.Join(next, ".env")); err != nil {
		return err
	}
	if err = copyPrivateFile(filepath.Join(dir, "apps.json"), filepath.Join(next, "apps.json")); err != nil {
		return err
	}
	if _, e := os.Stat(filepath.Join(dir, "Caddyfile")); e == nil {
		if err = copyPrivateFile(filepath.Join(dir, "Caddyfile"), filepath.Join(next, "Caddyfile")); err != nil {
			return err
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if err = os.Chmod(filepath.Join(next, "apps.json"), 0644); err != nil {
		return err
	}
	nextConfig, err := a.configurationForBackup(next)
	if err != nil {
		return err
	}
	if err = sameStorage(oldConfig, nextConfig); err != nil {
		return err
	}
	if err = a.confirm(o.yes, "Upgrade "+current.Tag+" to "+target.Tag+"? Services will stop and a recovery backup will be created."); err != nil {
		return err
	}
	if err = a.checkServices(dir); err != nil {
		return fmt.Errorf("start and repair the current installation before upgrading: %w", err)
	}
	if err = a.compose(dir, "run", "--rm", "--no-deps", "helpin-migrate", "validate"); err != nil {
		return fmt.Errorf("current migrations are not clean; upgrade refused: %w", err)
	}
	// Fetch all target images while the current installation is still available.
	if err = a.compose(next, "pull"); err != nil {
		return err
	}
	backup, err := a.backup(o, false)
	if err != nil {
		return err
	}
	previous := filepath.Join(stage, "previous")
	switched := false
	defer func() {
		if err == nil {
			return
		}
		if switched {
			stopErr := a.compose(filepath.Join(o.dir, "community"), "stop", "--timeout", "180")
			err = errors.Join(err, stopErr)
			// Never run old binaries against a possibly migrated database.
		}
		if preservePrevious {
			fmt.Fprintf(a.out, "Previous bundle retained at %s\n", previous)
		}
		err = fmt.Errorf("upgrade failed; recovery backup: %s. Restore into a NEW directory with: helpin restore --backup %q --dir %q --yes --no-start; inspect services before starting the restored copy: %w", backup, backup, o.dir+"-recovered", err)
	}()
	if err = os.Rename(o.dir, previous); err != nil {
		return err
	}
	if err = os.Rename(targetRoot, o.dir); err != nil {
		if e := os.Rename(previous, o.dir); e != nil {
			// Do not allow deferred cleanup to delete the only on-disk installation.
			preservePrevious = true
			recovery := o.dir + "-previous"
			if moveErr := os.Rename(previous, recovery); moveErr != nil {
				err = errors.Join(err, e, moveErr)
			} else {
				previous = recovery
				err = errors.Join(err, e)
			}
		}
		return err
	}
	switched = true
	if err = a.compose(dir, "up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		return err
	}
	if err = a.compose(dir, "run", "--rm", "--no-deps", "helpin-migrate", "validate"); err != nil {
		return err
	}
	if err = a.checkServices(dir); err != nil {
		return err
	}
	if err = a.ready(dir); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "✓ Upgraded %s → %s. Recovery backup: %s\n", current.Tag, target.Tag, backup)
	return nil
}

// Refuse to silently discard local modifications to release-managed files.
func verifyManagedFiles(root string) error {
	data, err := os.ReadFile(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		return fmt.Errorf("installed release checksum inventory is required for upgrade: %w", err)
	}
	expected := map[string]bool{"SHA256SUMS": true, "community/.env": true, "community/apps.json": true, "community/Caddyfile": true}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		digest, name, ok := strings.Cut(line, "  ")
		clean := filepath.Clean(filepath.FromSlash(name))
		if !ok || len(digest) != 64 || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || filepath.ToSlash(clean) != name || expected[name] {
			return errors.New("invalid installed checksum inventory")
		}
		filename := filepath.Join(root, clean)
		info, e := os.Lstat(filename)
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("release-managed files must not be links")
		}
		actual, e := hashFile(filename)
		if e != nil {
			return e
		}
		if actual != digest {
			return fmt.Errorf("release file %s was modified; reconcile local changes before upgrading", name)
		}
		expected[name] = true
	}
	return filepath.WalkDir(root, func(filename string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, filename)
		if e != nil {
			return e
		}
		if !expected[filepath.ToSlash(rel)] || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unmanaged installation file %s; move it outside the bundle before upgrading", rel)
		}
		return nil
	})
}
