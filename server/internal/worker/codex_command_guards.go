package worker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const codexDeliveryGuardMessage = "remote delivery is backend-managed for this run; create a local commit only"

func installCodexCommandGuards(runRoot string) (string, error) {
	runRoot = strings.TrimSpace(runRoot)
	if runRoot == "" {
		return "", fmt.Errorf("command guard root is required")
	}

	binDir := filepath.Join(runRoot, ".helpin-command-guards")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("create codex command guard dir: %w", err)
	}

	if err := installCodexGuardScript(binDir, "git", buildCodexGitGuardScript); err != nil {
		return "", err
	}
	if err := installCodexGuardScript(binDir, "gh", buildCodexGHGuardScript); err != nil {
		return "", err
	}

	return binDir, nil
}

func installCodexGuardScript(binDir, name string, builder func(realPath string) string) error {
	realPath, err := exec.LookPath(name)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil
		}
		return fmt.Errorf("resolve %s for codex command guard: %w", name, err)
	}
	script := builder(realPath)
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write codex command guard %s: %w", name, err)
	}
	return nil
}

func buildCodexGitGuardScript(realPath string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
real=%q
subcmd=""
skip_next=0
for arg in "$@"; do
  if [ "$skip_next" -eq 1 ]; then
    skip_next=0
    continue
  fi
  case "$arg" in
    -C|-c|--exec-path|--git-dir|--work-tree|--namespace|--super-prefix|--config-env)
      skip_next=1
      continue
      ;;
    --version|--help)
      exec "$real" "$@"
      ;;
    --bare|--no-pager|--paginate|--literal-pathspecs|--no-literal-pathspecs|--glob-pathspecs|--noglob-pathspecs|--icase-pathspecs)
      continue
      ;;
    --)
      break
      ;;
    -*)
      continue
      ;;
    *)
      subcmd="$arg"
      break
      ;;
  esac
done
if [ "$subcmd" = "push" ]; then
  printf '%%s\n' %q >&2
  exit 1
fi
exec "$real" "$@"
`, realPath, codexDeliveryGuardMessage)
}

func buildCodexGHGuardScript(realPath string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
real=%q
subcmd=""
skip_next=0
for arg in "$@"; do
  if [ "$skip_next" -eq 1 ]; then
    skip_next=0
    continue
  fi
  case "$arg" in
    -R|--repo)
      skip_next=1
      continue
      ;;
    --repo=*)
      continue
      ;;
    --version|--help)
      exec "$real" "$@"
      ;;
    --)
      break
      ;;
    -*)
      continue
      ;;
    *)
      subcmd="$arg"
      break
      ;;
  esac
done
if [ "$subcmd" = "pr" ]; then
  printf '%%s\n' %q >&2
  exit 1
fi
exec "$real" "$@"
`, realPath, codexDeliveryGuardMessage)
}

func prependPathEnv(env []string, dir string) []string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return env
	}
	current := os.Getenv("PATH")
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			current = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	if current == "" {
		return upsertEnv(env, "PATH", dir)
	}
	return upsertEnv(env, "PATH", dir+string(os.PathListSeparator)+current)
}

func errorsIsNotFound(err error) bool {
	var execErr *exec.Error
	return err != nil && (os.IsNotExist(err) || (errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound))
}
