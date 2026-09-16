# Helpin Community 0.1 beta

Self-host support chat, visitor identification, a staff inbox, a public help
center, and support agents. No billing service or Helpin account is required.
See [known limitations](../ROADMAP.md). This candidate is not published yet;
[publication decisions](PUBLICATION.md) and release tests must pass first.

## Install

Use the versioned bundle from a reviewed release. Requirements: Docker Engine,
Docker Compose v2, Bash, OpenSSL, and a supported amd64 or arm64 host. Native
architecture tests are a release gate, not an inference from successful builds.
Allow at least 8 GiB RAM and 20 GiB free disk for evaluation; source builds need
considerably more. These are evaluation starting points, not measured capacity
promises.

```sh
./setup.sh install
# Edit .env: public origins, optional SMTP, and optional server AI configuration.
./setup.sh start
./setup.sh status
```

Open `http://localhost:8085`, sign up, and create your organization/workspace.
Local signup does not require email. New accounts remain unverified. Enable
`AUTH_EMAIL_VERIFICATION_REQUIRED=true` with working application mail when your
public deployment requires mailbox verification.

In workspace settings, add your website origin first (scheme, hostname and port).
The widget refuses visitor requests while the list is empty. Add the dashboard
origin for previews and the help-center origin if it embeds chat. Copy the
resulting support-only installation snippet to your website. The snippet uses
this installation's public URLs; it does not send analytics to Helpin.

Configure API-key connections and a shared profile in Settings → AI, then choose
a workspace default. Personal connections belong in personal AI settings. ChatGPT
is optional and requires enabling it on both Helpin and Runtime; unattended runs
remain subject to their existing policy. Knowledge embeddings and non-agent AI
use separate server configuration; see [configuration](../docs/community/configuration.md).

Create and publish your first article in Docs/Help Center. Configure its custom
domain for public hosting. Locally, open
`http://localhost:8086/?subdomain=YOUR_HELP_CENTER_SLUG`.

```sh
./setup.sh logs helpin-api agent-runtime-worker
./setup.sh stop
```

Stop preserves named volumes. Never use `docker compose down -v` on an install
you want to keep. The installer preserves existing `.env` values and never
sources the file as shell code. Back up its encryption keys with your data;
changing them only in the environment makes encrypted credentials unreadable.

## Internet-facing deployment

Use an existing HTTPS reverse proxy or adapt `Caddyfile.example`. Configure:

- `APP_BASE_URL=https://inbox.example.com`
- `PUBLIC_WIDGET_URL=https://widget.example.com`
- `PUBLIC_SDK_URL=https://widget.example.com/sdk/lib.js`
- `PUBLIC_STORAGE_URL=https://files.example.com`

Point A records at the server. Add AAAA only when IPv6 works. Open 80/443 at your
edge and persist certificate storage. A CNAME contains a hostname, not a URL.
The default published ports bind loopback; the example proxy runs on that host.
A containerized proxy needs an explicitly private connection to the ingress.
Do not expose Postgres, Redis, NATS, Temporal, Runtime, the storage console, or
`/api/internal/` routes. The public widget hostname exposes only `/widget/*` and
`/sdk/*`. The dashboard and widget can share a hostname with the same route rules.

Your proxy must preserve Host, HTTPS protocol and WebSocket upgrades and replace
untrusted forwarded headers. Set `COMMUNITY_TRUSTED_PROXY_CIDR` to its exact source
address as seen by the ingress container (typically the private bridge gateway
for a host proxy). Never set it to `0.0.0.0/0`. The ingress ignores forwarded
protocol and client IP from other peers; this protects client-IP rate limits.
Do not publish that port directly on the internet. For an equivalent nginx edge, proxy the same paths to
the same upstreams, forward Upgrade/Connection, disable buffering for streaming,
and allow at least a 3,600-second WebSocket read timeout.

Permit the widget origin in your site's `connect-src`, the loader origin in
`script-src`, and your object-storage origin for image/upload requests. Serve all
public endpoints over HTTPS to avoid mixed content. Verify the exact snippet
from a different website origin, including a reply, reconnect, and attachment.
Check a third origin receives 403 even with a copied public installation key.

If the widget is blocked, check its installation origins and browser console
first. If certificates fail, check DNS A/AAAA, port reachability and certificate
storage. If the staff app cannot connect, check `./setup.sh status`, then API and
migrator logs. Do not paste credentials or full environment files into issues.

## Services and data

One Postgres server holds separate Helpin, Runtime, Temporal and visibility
databases with separate users. NATS JetStream, Temporal and Redis are required by
this supported bundle. Garage supplies S3-compatible storage. Optional coding is
outside the support beta. No ClickHouse/event collector or automatic analytics
service runs. Runtime browser tools are disabled in `apps.json` by default.
The bundle uses Runtime's `community` image target, which omits Node, browser
automation and coding toolchains. Use Runtime's separately tested full images
when deliberately enabling those optional capabilities.

The tested pins are PostgreSQL 17 with pgvector 0.8.6, Redis 7.2.16,
NATS 2.14.7, and Temporal 1.32.0. Temporal's schema and namespace jobs are
idempotent and run before workers. Redis stays on the BSD-licensed 7.2 line.
Infrastructure uses unmodified upstream images pinned by digest. We do not
rebuild PostgreSQL, NATS, or storage to change scanner results. See
[upstream image maintenance](../docs/community/upstream-images.md) for findings and review policy.

The internal model-credential callback allows HTTP only for the explicitly
configured `helpin-api:8080` host and still requires token authentication.
User-added MCP endpoints use a separate policy: HTTP and private networks are off
unless explicitly enabled. See `.env.example` and the configuration guide.

Garage 2.3.0 provides S3-compatible storage from its upstream multi-architecture
image. Its bucket is private: attachments use signed URLs and imported Docs
images require a workspace member with Docs read access. Publishing makes a
separate public help-center image copy. Helpin exposes only help-center assets,
user avatars and workspace logos; Garage has no anonymous website listener.
The one-node bundle has no storage redundancy. Use off-host backups; deployments
requiring fault tolerance should use an external replicated S3-compatible store.
This replaces the unreleased MinIO fixture, not an automatic migration of MinIO
volumes. Export any existing fixture objects before discarding its old volume.

## Back up and restore the same release

Schedule backups and practice restoring them on an isolated host. Quiesce writers
before taking a consistent snapshot. Back up `.env`, `apps.json`, the exact bundle
and image digests, all four databases, Garage metadata/object data, and the Redis/NATS volumes.
Do not print `.env` or embed it in a public support archive. Use restrictive file
permissions and encrypt backups off-host.

The tested command sequence is in `tests/backup-restore.sh`; it runs only in its
explicitly disposable test project. For an operator backup:

```sh
umask 077
mkdir -p backups
# Stop application writers first; keep infrastructure available for database dumps.
docker compose stop helpin-frontend helpin-helpcenter helpin-worker agent-runtime-worker agent-runtime helpin-api
for database in helpin agent_runtime temporal temporal_visibility; do
  docker compose exec -T postgres pg_dump -U postgres -Fc "$database" > "backups/$database.dump"
done
cp .env apps.json backups/
# Stop infrastructure before filesystem-level volume snapshots.
docker compose stop
```

Use your host's volume backup tooling to snapshot `postgres_data`, `garage_data`,
`redis_data`, and `nats_data` while stopped, then `./setup.sh start`. Record which
method was used; do not mix logical SQL restore and old Postgres volume contents.
The automated gate tests a cold volume restore, including the whole Postgres
volume. For a logical restore, first restore the same `.env` and start only
Postgres on a fresh volume (`docker compose up -d --wait postgres`); its initializer
creates matching users and databases. Restore each dump before starting writers:

```sh
for database in helpin agent_runtime temporal temporal_visibility; do
  docker compose exec -T postgres pg_restore -U postgres --clean --if-exists \
    --exit-on-error --dbname "$database" < "backups/$database.dump"
done
```

Restore the matching object and queue/cache volumes before starting the full
stack. Keep this logical procedure separate from the tested cold-volume method
and rehearse whichever method your own backup system uses.

A failed schema change is not reversed by changing an image tag. Restore the
matching pre-change database/object backup with its keys. Cross-version upgrade
support begins in 0.2; 0.1 verifies clean installs and same-release restore only.

## Build from source

From `community/` in a source checkout, with a matching Runtime checkout next to
Helpin (the operator bundle does not include build sources):

```sh
./setup.sh install
AGENT_RUNTIME_SOURCE=/path/to/agent-runtime docker compose -f compose.yaml -f compose.build.yaml build
./setup.sh start
```

Community images are separate from EE staging/production images. The release
workflow runs the source checks, native architecture smoke, image scans and
public-host gate before attaching a digest-pinned bundle.

The pinned Runtime commit in `runtime-revision.txt` must be available remotely
before CI runs. While that repository is private, configure the read-only
`COMMUNITY_RUNTIME_READ_TOKEN` Actions secret. The separate release workflow also
requires a protected `community-release` environment, chosen licenses, publication
review, and a real HTTPS acceptance record. It produces candidate artifacts;
making repositories public and publishing the final release remain reviewed
maintainer actions.
