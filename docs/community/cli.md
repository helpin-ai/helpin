# Install and manage Helpin with the CLI

This guide is for operators installing Helpin Community on a computer or public
server. The CLI downloads a verified release bundle, guides configuration, and
manages the Docker Compose services. It does not require host Go or Node.js.
The download flow requires a published Community release containing CLI assets;
source availability alone does not make the public installer usable.

## Requirements

Use Linux or macOS on amd64 or arm64, with Docker Engine or Docker Desktop,
Docker Compose v2, Bash, OpenSSL, and curl. The bootstrap also needs `sha256sum`
or `shasum`. Allow at least 8 GiB of memory available to Docker and 20 GiB free
on the installation filesystem. Ensure Docker's own data disk also has room for
images and volumes. These are evaluation starting points, not capacity promises.
Native Windows is not supported by this installer. Use a Linux environment with
a reachable Docker engine. The CLI does not install Docker or change group
membership, firewalls, DNS, or system proxy services.

## Download and install

After the website installer and a Community release are published:

```sh
curl -fsSL https://helpin.ai/install.sh | bash
"$HOME/.local/bin/helpin" install
```

The downloader installs a platform-specific executable at `~/.local/bin/helpin`.
If that directory is already on `PATH`, use `helpin` directly. Otherwise follow
the printed shell-profile instruction, or continue using the absolute path.
The downloader verifies the executable's SHA-256 before replacing it and never
starts application services. For an explicit version or destination:

```sh
curl -fsSL https://helpin.ai/install.sh | \
  HELPIN_VERSION=community-v0.1.0-rc.1 HELPIN_BIN_DIR="$HOME/.local/bin" bash
```

Use a tag listed on the releases page; the example tag is not a promise of
availability. Automatic discovery selects the first published Community tag
from GitHub's newest-first release list, including beta prereleases. It ignores
hosted-service tags. A release CLI installs its matching bundle by default.

Run `helpin` without arguments for the action menu, or `helpin install` for the
setup wizard. Choose `local` for this computer or `server` for a public host.
The default installation directory is `~/helpin`. Set `--dir` to choose another.
New CLI installations receive a distinct, persistent Compose project name so
separate installation directories do not share data volumes. Keep that name
when restoring or moving an installation; `configure` preserves it.
Local setup uses ports 8085, 8086, and 9005; the installer checks for conflicts.
It downloads the bundle, verifies its checksum, generates independent secrets,
starts services, and checks the API's expected configuration before printing
success. First startup includes downloading container images and may take
several minutes.

Open the printed dashboard URL, create your account and workspace, then follow
the [first support conversation](../../community/README.md#start-your-first-conversation).
AI provider connections and application mail remain optional.

### Public server setup

Server mode asks for three distinct DNS hostnames: the dashboard, attachments,
and public help center. The widget shares the dashboard hostname. It also asks
for the exact proxy source IP/CIDR as seen by the ingress container; a host proxy
usually appears as the Docker bridge gateway, not loopback. Do not guess this
value or trust all addresses.

The CLI writes public URL settings and `community/Caddyfile`. Install that file
in your host's Caddy service, point the three DNS names at this server, permit
80/443 at the edge, and persist certificate storage. The CLI leaves the app's
published ports on loopback. It does not install or reload your proxy. See the
[public deployment guide](deployment.md) for the network and HTTPS requirements.
Configure the help-center custom domain in Helpin after creating your workspace.

Run `helpin doctor` after configuring the proxy. A successful local startup is
reported separately from public HTTPS readiness. Doctor checks the public API
with normal certificate validation and verifies that its configuration matches
this installation. It does not replace the outside-origin widget and attachment
journey described in the deployment guide.

### Unattended setup

Use `--yes` with explicit options. Without it, piped or redirected input fails
instead of silently accepting setup choices. For a local installation:

```sh
helpin install --yes --mode local --dir "$HOME/helpin" \
  --port 8085 --help-port 8086 --storage-port 9005
```

For server mode, supply `--domain`, `--storage-domain`, `--help-domain`, and
`--proxy-cidr`. Use `--no-start` to download and prepare the configuration before
starting services with `helpin start`. Use `--version` to select another
published Community bundle for a **new** installation.

A previously downloaded bundle is supported:

```sh
helpin install --yes --mode local \
  --bundle /path/to/helpin-community-v0.1.0-rc.1.tar.gz \
  --checksum /path/to/helpin-community-v0.1.0-rc.1.tar.gz.sha256
```

Container images must still be available locally or downloadable. A checksum
verifies bytes against the downloaded checksum file; use official HTTPS release
assets or artifacts from a trusted source.

## Manage the installation

| Command | Behavior |
| --- | --- |
| `helpin` | Interactive action menu; prints help when input is not a terminal |
| `helpin start` | Starts services and waits for readiness |
| `helpin stop` | Removes service containers and networks; preserves named data volumes |
| `helpin restart` | Recreates services to apply configuration, preserving data volumes |
| `helpin status` | Shows containers, image identities, and migration information |
| `helpin logs [service...]` | Follows logs, initially showing the last 150 lines |
| `helpin configure` | Updates local/server URL and port settings; preserves secrets and optional settings |
| `helpin doctor` | Checks Docker, Compose configuration, required services, API readiness, HTTPS, and secret file permissions |
| `helpin version` | Prints the CLI version |

Every installation command accepts `--dir`. Put options before service names:
`helpin logs --dir /srv/helpin helpin-api agent-runtime-worker`.
Use `helpin install --help` for setup flags. Optional mail and server AI settings
are still managed in `community/.env`; see [configuration](configuration.md).
After changing configuration, run `helpin restart`.

Repeating `install` on an existing installation preserves it and prints the
management commands. It does not rotate keys, replace the bundle, or upgrade
data. Failed startup leaves configuration available for inspection and retry.
Concurrent modifying commands are excluded by a sibling `.NAME.helpin.lock`
file containing the process ID. After a killed command, verify that process is
no longer running before removing a stale lock. Never delete a live lock.

Back up the data and encryption keys as described in [backups](backups.md).
Never run `docker compose down -v` on data you intend to keep. There is no
`helpin upgrade` command yet: tested cross-version upgrades remain a separate
release requirement. Replacing the CLI executable does not upgrade the stack.

## Build and release the CLI

Contributors can build the standard-library-only Go module:

```sh
cd community/cli
go test -race ./...
go build -o /tmp/helpin .
```

The Community CLI workflow tests Linux amd64, Linux arm64, and macOS ARM runners.
Linux jobs also exercise the compiled CLI against an isolated real Compose
fixture, including configuration, restart, stop/start, and persistent data.
Run that check with `python3 community/cli/acceptance.py` from the repository
root after making `alpine:3.20` available locally. Set `TMPDIR` to a filesystem
with at least 20 GiB free if your default temporary filesystem is smaller.
The fixture exercises the installer; it is not full-application acceptance.
The candidate workflow tests the CLI, cross-compiles Linux/macOS amd64/arm64,
and attaches binaries, individual checksums, and the bootstrap to the candidate.
Packaging records their hashes in `release.json`; promotion re-verifies them
and publishes those exact assets without rebuilding. Container acceptance
continues through the existing native Community release gates.

The canonical bootstrap is `community/install.sh`; its identical website copy
is `website/public/install.sh`, served as `/install.sh` by the existing website
build. Tests reject drift. Publishing a candidate, promoting it, and deploying
the website are separate operations and remain subject to the existing
publication gate. No `get.helpin.ai` DNS record is required for this route.
