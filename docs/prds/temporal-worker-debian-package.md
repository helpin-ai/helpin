# Temporal worker debian package

**Status:** Draft
**Date:** 2026-03-26
**Author:** Engineering
**Feature:** Bare-metal packaging and release distribution for the Temporal worker

---

## 1. Problem Statement

The Temporal worker currently deploys through Docker/Kubernetes only. That works for the existing cluster model, but it does not support the new requirement to run the worker on bare-metal hosts where Codex is installed and managed through `systemd`.

Today there is no supported way to:

- install the Temporal worker as a native Linux service
- inject production secrets through Doppler on bare-metal hosts
- publish a versioned worker artifact from the production release workflow
- roll forward worker releases outside Kubernetes

This blocks operation of Codex-backed autonomous worker queues on dedicated machines.

## 2. Goals

1. Produce a Debian package for the Temporal worker on every `main` release.
2. Attach that package to the GitHub Release created by the production workflow.
3. Install the worker on Ubuntu/Debian hosts as a `systemd` service.
4. Load runtime secrets through Doppler at process start.
5. Preserve local config across upgrades.
6. Support optional unattended upgrades through a `systemd` timer.
7. Ship a default configuration that is safe for Codex bare-metal hosts.

## 3. Non-Goals

- Packaging the main API server or help center as `.deb` artifacts
- Replacing the existing Docker/Kubernetes deployment path
- Installing Codex, OpenCode, Doppler, or GitHub CLI binaries inside the package
- Managing `DOPPLER_TOKEN` automatically
- Enabling or starting services automatically on first install
- Supporting non-Debian package formats in v1

## 4. Design Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| What artifact format ships first? | `.deb` only | Matches the target bare-metal environment and keeps packaging scope narrow. |
| What starts the worker? | `systemd` service using Doppler-wrapped `ExecStart` | Keeps secret injection outside the binary and matches current operator workflow. |
| Should the package auto-start on install? | No | First install requires host-specific Doppler auth and GitHub token setup. |
| Should the updater timer auto-enable on install? | No | Avoids a failing timer before `GH_TOKEN` and `gh` are configured. |
| What queues run by default? | `agent-opencode-autonomous,agent-codex-autonomous` | This package is intentionally for Codex/OpenCode bare-metal workers. Hosts must provide both CLIs, or operators must override the queue list locally. |
| How aggressive should systemd hardening be? | Moderate hardening: `NoNewPrivileges=yes`, `PrivateTmp=yes`, `ProtectSystem=full`; do not set `ProtectHome=yes` in v1 | The worker creates temp workspaces and temp Codex homes during execution. `ProtectSystem=strict` and `ProtectHome=yes` risk breaking runtime behavior. |
| How is local config preserved? | `/etc/helpin/temporal-worker.conf` marked `config|noreplace` | Admin edits must survive package upgrades. |
| How does auto-update authenticate to GitHub? | `gh` CLI with `GH_TOKEN` from the env file | Simple implementation path for private release downloads. |
| Is `gh` a hard dependency? | Recommended, not required for worker startup | The updater needs it; the worker itself does not. |

## 5. User Stories

### 5.1 Operator

- As an operator, I can install a released worker package with `apt install`, so I do not need Docker to run the worker.
- As an operator, I can configure Doppler project/config values in one environment file, so runtime configuration is predictable.
- As an operator, I can add `DOPPLER_TOKEN` through a `systemd` override, so long-lived machine credentials are not baked into the package.
- As an operator, I can upgrade the installed worker without losing local config.
- As an operator, I can optionally enable a timer that keeps the worker updated from GitHub Releases.

### 5.2 Release Engineer

- As a release engineer, I get a worker `.deb` artifact on each production release automatically.
- As a release engineer, I see the package attached to the GitHub Release alongside the container image references.

### 5.3 System

- The system does not start the worker before the host is configured.
- The system restarts the worker on upgrade only if it was already running.
- The system does not break existing Docker/Kubernetes release behavior.

## 6. Current-State Constraints

The current worker process already supports graceful termination and queue selection:

- it listens for `SIGTERM` and stops workers cleanly
- it supports `TEMPORAL_WORKER_QUEUES` as a comma-separated env var

The current worker also assumes external runtime CLIs are present:

- `CODEX_PATH` defaults to `codex`
- `OPENCODE_PATH` defaults to `opencode`

Because this package enables both autonomous queues by default, target hosts must provide both runtimes unless the operator overrides `TEMPORAL_WORKER_QUEUES`.

## 7. Proposed Solution

Ship a Debian package named `helpin-temporal-worker` containing:

- the worker binary
- a `systemd` service unit
- a protected environment file under `/etc/helpin`
- a release-updater script
- a `systemd` oneshot updater service
- a `systemd` updater timer
- package lifecycle scripts for install, upgrade, and removal

The production GitHub Actions workflow will build the worker binary, package it with `nfpm`, upload the resulting `.deb` as a workflow artifact, and attach it to the GitHub Release.

## 8. Package Contents

| Source File | Installed Path | Purpose |
|-------------|----------------|---------|
| `dist/temporal-worker` | `/usr/bin/helpin-temporal-worker` | Worker executable |
| `server/packaging/helpin-temporal-worker.service` | `/lib/systemd/system/helpin-temporal-worker.service` | Main worker service |
| `server/packaging/temporal-worker.conf` | `/etc/helpin/temporal-worker.conf` | Runtime env file |
| `server/packaging/helpin-temporal-worker-update.sh` | `/usr/lib/helpin/temporal-worker-update.sh` | Auto-update script |
| `server/packaging/helpin-temporal-worker-updater.service` | `/lib/systemd/system/helpin-temporal-worker-updater.service` | Oneshot updater service |
| `server/packaging/helpin-temporal-worker-updater.timer` | `/lib/systemd/system/helpin-temporal-worker-updater.timer` | Auto-update timer |
| `server/packaging/postinstall.sh` | package script | Post-install/upgrade actions |
| `server/packaging/preremove.sh` | package script | Pre-remove actions |

## 9. Functional Requirements

### 9.1 Main Service Unit

Create `server/packaging/helpin-temporal-worker.service` with:

- `Type=simple`
- `After=network-online.target`
- `Wants=network-online.target`
- `EnvironmentFile=-/etc/helpin/temporal-worker.conf`
- `ExecStart=/usr/bin/doppler run --project ${DOPPLER_PROJECT} --config ${DOPPLER_CONFIG} -- /usr/bin/helpin-temporal-worker`
- `KillSignal=SIGTERM`
- `TimeoutStopSec=30`
- `Restart=on-failure`
- `RestartSec=5`
- `StartLimitIntervalSec=60`
- `StartLimitBurst=3`
- `NoNewPrivileges=yes`
- `PrivateTmp=yes`
- `ProtectSystem=full`

Do not set `ProtectSystem=strict` or `ProtectHome=yes` in v1.

### 9.2 Runtime Environment File

Create `server/packaging/temporal-worker.conf` as the package-installed template.

Required contents:

- `DOPPLER_PROJECT=backend`
- `DOPPLER_CONFIG=prd`
- `TEMPORAL_WORKER_QUEUES=agent-opencode-autonomous,agent-codex-autonomous`
- `GH_TOKEN=` placeholder

Recommended commented examples:

- `CODEX_PATH=codex`
- `OPENCODE_PATH=opencode`

Requirements:

- install path `/etc/helpin/temporal-worker.conf`
- permissions `0600`
- marked `config|noreplace`

### 9.3 Post-Install Script

Create `server/packaging/postinstall.sh`.

Requirements:

- run `systemctl daemon-reload`
- do not auto-start the worker on first install
- do not auto-enable the updater timer on first install
- on upgrade, restart or `try-restart` the worker only if already active
- on upgrade, restart the updater timer only if already enabled/active
- print setup instructions:
  - edit `/etc/helpin/temporal-worker.conf`
  - add `DOPPLER_TOKEN` via `systemctl edit helpin-temporal-worker`
  - enable/start the worker manually
  - enable/start the updater timer manually if desired

### 9.4 Pre-Remove Script

Create `server/packaging/preremove.sh`.

Requirements:

- stop and disable `helpin-temporal-worker.service` if present
- stop and disable `helpin-temporal-worker-updater.timer` if present
- stop the oneshot updater service if currently running

### 9.5 nfpm Configuration

Create `server/nfpm.yaml`.

Requirements:

- package name `helpin-temporal-worker`
- architecture `amd64`
- version from `VERSION` environment variable
- Debian-compliant version string with no leading `v`
- `depends` includes `systemd`
- `recommends` includes `doppler` and `gh`
- all package files listed in `contents`
- config file entry uses `type: config|noreplace`
- lifecycle scripts wired to `postinstall` and `preremove`

### 9.6 Auto-Update Script

Create `server/packaging/helpin-temporal-worker-update.sh`.

Behavior:

1. Exit successfully with a log message if `gh` is not installed.
2. Exit successfully with a log message if `GH_TOKEN` is empty.
3. Read installed version with `dpkg-query -W -f='${Version}' helpin-temporal-worker`.
4. Read latest release tag with `gh release view --repo helpin-ai/helpin --json tagName`.
5. Strip the `v` prefix from the GitHub tag before version comparison.
6. If the release is newer:
   - download the matching `.deb` to `/tmp`
   - install it with `apt` in non-interactive mode
   - remove the downloaded file
7. If the installed version is current, exit successfully.

The script will be installed to `/usr/lib/helpin/temporal-worker-update.sh`.

### 9.7 Updater Service

Create `server/packaging/helpin-temporal-worker-updater.service`.

Requirements:

- `Type=oneshot`
- `EnvironmentFile=-/etc/helpin/temporal-worker.conf`
- `ExecStart=/usr/lib/helpin/temporal-worker-update.sh`

The updater service does not need to wrap itself in Doppler unless a future version requires Doppler-resolved values for update behavior.

### 9.8 Updater Timer

Create `server/packaging/helpin-temporal-worker-updater.timer`.

Requirements:

- `OnCalendar=*:0/10`
- `Persistent=true`
- install but do not auto-enable on first install
- journal output visible through `journalctl -u helpin-temporal-worker-updater`

## 10. CI/CD Requirements

Update `.github/workflows/deploy-prod.yml`.

### 10.1 New `build-worker-deb` Job

Add a parallel job named `build-worker-deb` with:

- `needs: version`
- `if: needs.version.outputs.release_tag != ''`
- checkout
- Go 1.24 setup
- build command for `linux/amd64` with:
  - `CGO_ENABLED=0`
  - `GOOS=linux`
  - `GOARCH=amd64`
  - `-ldflags="-w -s"`
- install `nfpm`
- set `VERSION=${RELEASE_TAG#v}`
- run `nfpm package --packager deb`
- upload the `.deb` as a GitHub Actions artifact

### 10.2 Release Job Changes

Update the `release` job to:

- depend on `build-worker-deb`
- download the worker artifact
- upload the `.deb` to the GitHub Release with `gh release upload`
- add a `Packages` section to release notes

The current image-publishing behavior remains unchanged.

## 11. Bare-Metal Operator Flow

### 11.1 Install

```bash
gh release download v0.88.0 --pattern '*.deb' --repo helpin-ai/helpin
sudo apt install ./helpin-temporal-worker_0.88.0_amd64.deb
```

### 11.2 Configure

```bash
sudo vim /etc/helpin/temporal-worker.conf
sudo systemctl edit helpin-temporal-worker
```

Expected local configuration:

- set `GH_TOKEN`
- set or confirm Doppler project/config values
- add `DOPPLER_TOKEN` as a `systemd` override
- ensure both `codex` and `opencode` are installed, or narrow `TEMPORAL_WORKER_QUEUES` locally

### 11.3 Start

```bash
sudo systemctl enable --now helpin-temporal-worker
sudo systemctl enable --now helpin-temporal-worker-updater.timer
```

The updater timer remains optional.

## 12. Operational Requirements

### 12.1 Logging

- Main worker logs must flow to `journalctl -u helpin-temporal-worker`
- Updater logs must flow to `journalctl -u helpin-temporal-worker-updater`

### 12.2 Upgrade Safety

- config file changes survive upgrades
- worker restarts only when previously active
- failed updater preconditions do not break the worker service

### 12.3 Host Assumptions

v1 assumes the host provides:

- `systemd`
- `doppler`
- `gh` if auto-update is enabled
- `codex` on `PATH`, or a configured `CODEX_PATH`
- `opencode` on `PATH`, or a configured `OPENCODE_PATH`

## 13. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Package defaults enable queues for missing runtimes | Worker jobs fail at runtime | Document the requirement clearly in the config template and package instructions; operators can narrow `TEMPORAL_WORKER_QUEUES` locally. |
| Service hardening blocks temp workspace creation | Worker fails under load | Use moderate hardening in v1; avoid `ProtectHome=yes` and `ProtectSystem=strict`. |
| Updater loops with missing credentials | Noisy journals and confusion | Do not auto-enable timer; updater exits `0` with clear logs when prerequisites are missing. |
| Private release download fails | Auto-updates do not apply | Require `GH_TOKEN`; log actionable errors. |
| Release workflow produces package but does not attach it | Operators cannot discover artifacts | Make the release job explicitly download and upload the `.deb`. |

## 14. Acceptance Criteria

1. Pushing to `main` with a non-empty release tag runs `build-worker-deb`.
2. The production GitHub Release includes the worker `.deb` asset.
3. Installing the package creates:
   - `/usr/bin/helpin-temporal-worker`
   - `/lib/systemd/system/helpin-temporal-worker.service`
   - `/lib/systemd/system/helpin-temporal-worker-updater.service`
   - `/lib/systemd/system/helpin-temporal-worker-updater.timer`
   - `/etc/helpin/temporal-worker.conf`
4. First install does not automatically start the worker.
5. First install does not automatically enable the updater timer.
6. Local edits to `/etc/helpin/temporal-worker.conf` survive package upgrade.
7. Starting the worker with configured Doppler access results in successful startup logs.
8. Upgrading to a newer package restarts the worker only if it was already running.
9. Enabling the updater timer causes periodic version checks without breaking when no update is available.

## 15. Verification Plan

1. Build the worker binary locally and run `nfpm package --packager deb`.
2. Inspect package contents with `dpkg-deb -c`.
3. Install the package on a Debian/Ubuntu test host.
4. Verify file placement and permissions.
5. Configure `/etc/helpin/temporal-worker.conf` and the `systemd` override for `DOPPLER_TOKEN`.
6. Start the worker and confirm logs show successful startup.
7. Enable the updater timer only after `GH_TOKEN` and `gh` are present.
8. Install a newer package and verify:
   - config is preserved
   - service restart behavior is conditional
   - timer continues to run

## 16. Implementation Checklist

- [ ] Create `server/packaging/helpin-temporal-worker.service`
- [ ] Create `server/packaging/temporal-worker.conf`
- [ ] Create `server/packaging/helpin-temporal-worker-update.sh`
- [ ] Create `server/packaging/helpin-temporal-worker-updater.service`
- [ ] Create `server/packaging/helpin-temporal-worker-updater.timer`
- [ ] Create `server/packaging/postinstall.sh`
- [ ] Create `server/packaging/preremove.sh`
- [ ] Create `server/nfpm.yaml`
- [ ] Update `.github/workflows/deploy-prod.yml`
