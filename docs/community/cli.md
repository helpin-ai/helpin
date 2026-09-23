# Install and manage Helpin with the CLI

This guide is for operators installing Helpin Community on a computer or public
server. The CLI downloads a verified release bundle, guides configuration, and
manages the Docker Compose services. It does not require host Go or Node.js.
The download flow requires a published Community release containing CLI assets;
source availability alone does not make the public installer usable.

## Requirements

Use Linux or macOS on amd64 or arm64, with Docker Engine or Docker Desktop,
Docker Compose v2, Bash, OpenSSL, and curl. The bootstrap also needs `sha256sum`
or `shasum`. Allow at least 8 GB of RAM available to Docker and 20 GiB free
on the installation filesystem. An 8 GB server reports about 7.7 GiB usable;
`helpin doctor` and `install` accept 7.5 GiB or more, warn from 6 GiB, and fail
below that. Ensure Docker's own data disk also has room for
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

The wizard then offers two optional integrations; enter `skip` to configure
either later:

- **Application email (SMTP)**: host, port, TLS mode (`starttls`, `tls`, or
  `none` for an unauthenticated trusted relay), username, password, and sender
  address. It sends invitations and password resets.
- **AI provider**: `openrouter`, `openai`, or `anthropic`, with its API key.
  Anthropic keys do not provide knowledge embeddings.

Passwords and API keys are read without echo and are written only to the
private `community/.env` (mode 600), next to the generated secrets, which are
preserved. `helpin configure` offers the same prompts; a blank secret keeps the
stored one.

Open the printed dashboard URL, create your account and workspace, then follow
the [first support conversation](../../community/README.md#start-your-first-conversation).
AI provider connections and application mail remain optional.

### Public server setup

Server mode asks for three distinct DNS hostnames: the dashboard, attachments,
and public help center. The widget shares the dashboard hostname. It then asks
how HTTPS is handled:

- **`builtin`** (default for new servers): the bundle runs Caddy
  (`compose.proxy.yaml`) on ports 80 and 443. Caddy obtains and renews
  certificates automatically once DNS points at the server. The CLI writes
  `community/Caddyfile`, and trusts only Caddy's fixed address on a private
  `edge` network, so there is no proxy address to look up. `--acme-email` sets
  an optional address for certificate expiry notices. Ports 80 and 443 must be
  free on the host.
- **`external`**: you run your own reverse proxy. The CLI asks for its exact
  source IP/CIDR as seen by the ingress container; a host proxy usually appears
  as the Docker bridge gateway, not loopback. Do not guess this value or trust
  all addresses. The CLI writes a host `community/Caddyfile` you can adapt, keeps
  the app's published ports on loopback, and does not install or reload your
  proxy. Installations that already used their own proxy keep `external`, and
  passing `--proxy-cidr` without `--proxy` also selects it.

Point the three DNS names at this server and permit 80/443 at the edge. See the
[public deployment guide](deployment.md) for the network and HTTPS requirements.
Configure the help-center custom domain in Helpin after creating your workspace.

If `172.30.255.0/28` collides with an existing network on the host, change
`EDGE_SUBNET`, `EDGE_IP_RANGE` and `EDGE_PROXY_IP` in `.env` (the proxy address
must be inside the subnet but outside the range), then rerun `helpin configure`
and `helpin restart`.

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

For server mode, supply `--domain`, `--storage-domain` and `--help-domain`, plus
`--proxy builtin` (optionally `--acme-email`) or `--proxy external
--proxy-cidr`. Unattended runs change mail and AI settings only when their flags
are given. Secrets are never accepted as command-line values; pass a file or an
environment variable instead:

```sh
HELPIN_SMTP_PASSWORD="$(cat /run/secrets/smtp)" helpin install --yes --mode local \
  --smtp-host smtp.example.com --smtp-username helpin --smtp-from help@example.com \
  --ai-provider openrouter --ai-key-file /run/secrets/openrouter
```

`--smtp-password-file` and `HELPIN_AI_API_KEY` are the other two sources; a
file's first line is used. `--smtp-port` defaults to 587 and `--smtp-tls` to
`starttls`. Use `--no-start` to download and prepare the configuration before
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
| `helpin configure` | Updates local/server URL and port settings, and optionally application mail and the AI provider key; preserves secrets |
| `helpin doctor` | Checks Docker, Compose configuration, required services, API readiness, HTTPS, secret file permissions, and capabilities |
| `helpin backup` | Stops services, snapshots the bundle and all named volumes, then resumes previously running services |
| `helpin restore` | Verifies a backup and restores the original release/data/keys into a new directory and new volumes |
| `helpin upgrade` | Verifies a compatible target release, creates a recovery backup, applies migrations, and checks readiness |
| `helpin version` | Prints the CLI version |

Every installation command accepts `--dir`. Put options before service names:
`helpin logs --dir /srv/helpin helpin-api agent-runtime-worker`.
Use `helpin install --help` for setup flags. Other optional settings are managed
in `community/.env`; see [configuration](configuration.md). After changing
configuration, run `helpin restart`.

### Capability checks

`helpin doctor` ends with a **Capabilities** section read from the local API
(`GET /api/instance/capabilities`, authenticated with the installation's
`INTERNAL_API_SECRET` over the loopback dashboard port). Each line shows a
status and, when something is missing, the next step:

| Status | Meaning |
| --- | --- |
| `ready` | Configured and confirmed by evidence, such as a successful test or indexed knowledge |
| `needs setup` | Missing configuration, or the last check failed |
| `unable to verify` | Configured, but not yet confirmed; for example no test email has been sent |
| `unavailable` | Not offered by this edition or the enabled modules |

Only object storage and background workers are required: doctor fails when one
of them needs setup. Optional integrations never fail doctor. Older releases
without the endpoint are reported as skipped. For workspace members,
`GET /api/workspaces/{id}/capabilities` reports the same checks per workspace.

Repeating `install` on an existing installation preserves it and prints the
management commands. It does not rotate keys, replace the bundle, or upgrade
data. Failed startup leaves configuration available for inspection and retry.
Concurrent modifying commands are excluded by a sibling `.NAME.helpin.lock`
file containing the process ID. After a killed command, verify that process is
no longer running before removing a stale lock. Never delete a live lock.

Back up the data and encryption keys as described in [backups](backups.md).
Never run `docker compose down -v` on data you intend to keep. Replacing the CLI
executable does not upgrade the stack.

## Upgrade an installation

Use an explicit published release tag (replace the example with a real tag):

```sh
helpin upgrade --dir /srv/helpin --version community-v0.2.0 --backup /srv/pre-upgrade-backup
```

Without `--version`, upgrade discovers the newest published Community release,
including prereleases, independently of the CLI's own version. It never skips
an incompatible newest release silently: select a compatible tag explicitly.
Previously downloaded bundles use `--bundle` and `--checksum`, as for install.
The command asks before downtime; use `--yes` for unattended operation.

The target's `release.json` must list the installed tag in `upgrade_from` and
include an HTTPS `upgrade_evidence` record. Releases without this declaration
remain installable but cannot be upgrade targets. Maintainers must verify each
full-stack upgrade/recovery path before declaring it; fixture tests alone do not
establish production compatibility.

Upgrade requires a healthy current stack, a clean Helpin migration ledger, an
unchanged release checksum inventory, and the same volume layout and infrastructure
images. Local edits to managed bundle files or additional files must be reconciled
first. Keep operator settings in `.env`, `apps.json`, and `Caddyfile`; upgrade
preserves those files and merges newly introduced environment defaults without
rotating existing keys. Changes to PostgreSQL, Redis, NATS, Garage, or Temporal
images require a separate migration procedure.

The CLI fetches target images before downtime, creates a consistent backup, then
replaces the bundle and starts the target services. It checks migrations, service
health, and API readiness. A failure stops the target services and prints recovery
instructions using the pre-upgrade backup. Follow [backup and recovery](backups.md)
to restore into a new installation. Recovery is explicit and preserves the failed
installation for inspection; there is no automatic schema downgrade.

## Build and release the CLI

Contributors can build the standard-library-only Go module:

```sh
cd community/cli
go test -race ./...
go build -o /tmp/helpin .
```

The Community CLI workflow tests Linux amd64, Linux arm64, and macOS ARM runners.
Linux jobs also exercise the compiled CLI against an isolated real Compose
fixture, including configuration, restart, stop/start, backup/restore, compatible
upgrade, and recovery after a migration mutates data and fails.
Run that check with `python3 community/cli/acceptance.py` from the repository
root after making `alpine:3.20` available locally. Set `TMPDIR` to a filesystem
with at least 20 GiB free if your default temporary filesystem is smaller.
The fixture exercises the installer; it is not full-application acceptance.
The candidate workflow tests the CLI, cross-compiles Linux/macOS amd64/arm64,
and attaches binaries, individual checksums, and the bootstrap to the candidate.
Packaging records their hashes in `release.json`; promotion re-verifies them
and publishes those exact assets without rebuilding. It refuses a repository
that is not anonymously readable and verifies anonymous asset downloads after
publication. A post-publication access failure fails the workflow without deleting
the release; repair access before announcing it. Container acceptance
continues through the existing native Community release gates.

The canonical bootstrap is `community/install.sh`; its identical website copy
is `website/public/install.sh`, served as `/install.sh` by the existing website
build. Tests reject drift. Publishing a candidate, promoting it, and deploying
the website are separate operations and remain subject to the existing
publication gate. Candidate inputs `upgrade_from` and `upgrade_evidence` record
reviewed full-stack upgrade/recovery paths; leave them empty for install-only
releases. The public repository and container images must be downloadable without
maintainer credentials. No `get.helpin.ai` DNS record is required for this route.
