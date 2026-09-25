# Deploy Community publicly

This guide is for operators exposing a Community installation on a public
hostname. It explains the reverse proxy, URL settings, and network boundaries
required for HTTPS access; use it after a local evaluation succeeds. Run
commands below from the bundle’s `community/` directory.

The quickest path is the bundled proxy: `helpin install --mode server` (or
`helpin configure --mode server`) with the default `--proxy builtin` runs Caddy
from `compose.proxy.yaml`, writes its configuration, and trusts only its fixed
address. Point DNS at the server, allow ports 80 and 443, and run `helpin
doctor`; certificates are issued automatically. See the [CLI guide](cli.md).

To run your own proxy instead, choose `--proxy external`. The wizard can still
generate URL settings and a host Caddy configuration; it keeps services on
loopback and leaves DNS and proxy activation to the operator. Use `helpin
doctor` after configuring HTTPS.

With your own proxy, use an existing HTTPS reverse proxy or adapt
`Caddyfile.example`. Configure:

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
[upstream image maintenance](upstream-images.md) for findings and review policy.

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

For step-by-step diagnosis, see [troubleshooting](troubleshooting.md).

Next: [configuration](configuration.md), [widget identity](widget-identity.md), and [backups](backups.md).
