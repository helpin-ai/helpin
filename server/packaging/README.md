# Temporal Worker Debian Package

This directory contains the Debian packaging assets for the bare-metal Temporal worker deployment.

The package name is `helpin-temporal-worker`. It installs the worker as a `systemd` service, loads runtime secrets through Doppler, and optionally enables unattended upgrades through a timer-driven updater script.

## What Gets Installed

The package installs:

- `/usr/bin/helpin-temporal-worker`
- `/lib/systemd/system/helpin-temporal-worker.service`
- `/lib/systemd/system/helpin-temporal-worker-updater.service`
- `/lib/systemd/system/helpin-temporal-worker-updater.timer`
- `/usr/lib/helpin/temporal-worker-update.sh`
- `/etc/helpin/temporal-worker.conf`

The environment file is shipped as `config|noreplace`, so local edits survive package upgrades.

## Runtime Assumptions

The default config enables both autonomous queues:

```env
TEMPORAL_WORKER_QUEUES=agent-opencode-autonomous,agent-codex-autonomous
```

That means the target host must provide both runtimes:

- `codex`
- `opencode`

If a host only supports one runtime, narrow `TEMPORAL_WORKER_QUEUES` in `/etc/helpin/temporal-worker.conf`.

## Local Package Build

Build the worker binary and package it locally from `server/`:

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-w -s" -o dist/temporal-worker ./cmd/temporal-worker

VERSION=0.88.0 nfpm package --packager deb \
  --target "dist/helpin-temporal-worker_${VERSION}_amd64.deb"
```

Package metadata and file mappings live in [`server/nfpm.yaml`](../nfpm.yaml).

## Bare-Metal Install

Download the package from the GitHub Release and install it:

```bash
gh release download v0.88.0 --pattern '*.deb' --repo helpin-ai/helpin
sudo apt install ./helpin-temporal-worker_0.88.0_amd64.deb
```

Then configure the host:

```bash
sudo vim /etc/helpin/temporal-worker.conf
sudo systemctl edit helpin-temporal-worker
```

Typical `systemctl edit` override:

```ini
[Service]
Environment=DOPPLER_TOKEN=dp.st.xxxxx
```

Start the worker only after Doppler and runtime binaries are ready:

```bash
sudo systemctl enable --now helpin-temporal-worker
```

## Auto-Updates

The updater checks the latest GitHub Release every 10 minutes and installs a newer `.deb` when available.

Requirements:

- `gh` installed on the host
- `GH_TOKEN` set in `/etc/helpin/temporal-worker.conf`
- sufficient token scope to read private releases for `helpin-ai/helpin`

Enable it only after those prerequisites are configured:

```bash
sudo systemctl enable --now helpin-temporal-worker-updater.timer
```

Useful commands:

```bash
journalctl -u helpin-temporal-worker -f
journalctl -u helpin-temporal-worker-updater
systemctl list-timers helpin-temporal-worker-updater.timer
```

## CI/CD

The production release workflow in [`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml):

1. builds `dist/temporal-worker`
2. packages the `.deb` with `nfpm`
3. uploads the package as a workflow artifact
4. attaches the package to the GitHub Release

## Notes

- First install does not auto-start the worker.
- First install does not auto-enable the updater timer.
- Upgrades preserve `/etc/helpin/temporal-worker.conf`.
- The post-install script restarts the worker on upgrade only if it was already running.
