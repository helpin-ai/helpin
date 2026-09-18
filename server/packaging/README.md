# Temporal worker Debian package

This directory contains optional Debian packaging assets for a bare-metal Temporal
worker. The current production workflow publishes container images; it does not
build or attach this worker package to releases. Build and distribute the package
separately if your installation uses systemd.

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

Agent execution belongs to the separate agent-runtime workers. Leave `TEMPORAL_WORKER_QUEUES` unset for the Helpin worker defaults; use only registered product queues when narrowing this worker.

## Local Package Build

Build the worker binary and package it locally from `server/`:

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-w -s" -o dist/temporal-worker ./cmd/temporal-worker

HELPIN_PACKAGE_VERSION=0.88.0 # Example; choose the source release being packaged
VERSION="$HELPIN_PACKAGE_VERSION" nfpm package --packager deb \
  --target "dist/helpin-temporal-worker_${HELPIN_PACKAGE_VERSION}_amd64.deb"
```

The command above builds Community. For an EE installation, build the worker
with `-tags ee` so it matches the API edition.

Package metadata and file mappings live in [`server/nfpm.yaml`](../nfpm.yaml).

## Bare-Metal Install

Install the package you built or an explicitly verified package asset from your
release process. Do not assume the latest GitHub Release has a worker `.deb`:

```bash
sudo apt install ./dist/helpin-temporal-worker_0.88.0_amd64.deb
```

Replace the example version with the package you built.

Then configure the host:

```bash
sudo vim /etc/helpin/temporal-worker.conf
sudo systemctl edit helpin-temporal-worker
```

Default package config:

```env
DOPPLER_PROJECT=your-project
DOPPLER_CONFIG=your-config
```

Set the Doppler project and config for your own account, then provide the
service token through a `systemctl edit` override:

```ini
[Service]
Environment=HOME=/var/lib/helpin-temporal-worker
Environment=DOPPLER_TOKEN=dp.st.xxxxx
```

Notes:

- Doppler needs a valid, writable `HOME` in the service environment for the service user.
- The credential the service expects is `DOPPLER_TOKEN`.
- The override lives in `/etc/systemd/system/helpin-temporal-worker.service.d/override.conf` and survives package upgrades.
- Operators who do not use Doppler can replace the `ExecStart` wrapper with a plain environment file.

Start the worker only after Doppler and runtime binaries are ready:

```bash
sudo systemctl enable --now helpin-temporal-worker
```

## Auto-Updates

The updater checks the latest GitHub Release every 10 minutes and installs a newer `.deb` when available.

Requirements:

- `gh` installed on the host
- `GH_TOKEN` set in `/etc/helpin/temporal-worker.conf`
- a token able to read releases for the repository you package from

The current container release workflow does not supply those `.deb` assets.
Enable the updater only if a separate packaging process publishes the expected
asset for each selected release and the prerequisites above are configured:

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

The [production release workflow](../../.github/workflows/deploy-prod.yml) builds
separate API, migration, and Temporal worker container images. It contains no
`nfpm` step or Temporal worker Debian upload. These packaging assets support a
manual or separately configured release process.

## Notes

- First install does not auto-start the worker.
- First install does not auto-enable the updater timer.
- Upgrades preserve `/etc/helpin/temporal-worker.conf`.
- The post-install script restarts the worker on upgrade only if it was already running.
