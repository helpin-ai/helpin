# Community configuration

Community 0.1 is a beta. The supported bundle sets explicit local defaults;
running the binaries directly retains conservative authentication defaults.

| Setting | Behavior |
| --- | --- |
| `HELPIN_ENABLED_MODULES` | Comma-separated product surfaces. Community defaults to `support,docs,agents`; EE defaults to all modules. Support requires Docs. Workspace roles remain in force. |
| `AUTH_EMAIL_VERIFICATION_REQUIRED` | Defaults to `true`. Set `false` for local Community signup without mail. EE rejects `false`. This never marks an email verified. |
| `SMTP_HOST`, `SMTP_PORT` | Optional application mail relay; port defaults to 587. SMTP takes precedence over application Postmark when configured. |
| `SMTP_FROM` | Required sender address when SMTP is enabled. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | Set both for authenticated delivery, or leave both empty for a trusted relay. |
| `SMTP_TLS_MODE` | `starttls` by default; `tls` for implicit TLS; explicit `none` only for an unauthenticated trusted local relay. Credentials require TLS and AUTH PLAIN; LOGIN-only SMTP servers are unsupported. TLS certificates are verified. |
| `POSTMARK_APP_SERVER_TOKEN`, `POSTMARK_APP_FROM_EMAIL` | Optional alternative application-mail provider. |

SMTP sends invitations, password reset, optional verification and application
notifications. It does **not** enable support reply threading or inbound mail;
those integrations still use optional Postmark. Without application mail, local
signup works when verification is disabled, while invitations and password reset
report that mail is unavailable. Google OAuth is optional.

Deployment modules are intersected with workspace access, including owners and
administrators. The Agents surface uses the existing Automation access grants,
but enabling Agents alone does not enable flows, triggers, or the Automation
product surface. Shared customer record APIs remain available to Support under
their existing resource permissions. Direct module URLs and API requests cannot
enable a module omitted from the deployment selection.

## Private Runtime callback

Runtime normally requires HTTPS for model credential callbacks (existing
localhost exceptions remain). The private Compose network can explicitly set:

```text
AGENT_RUNTIME_MODEL_CALLBACK_ALLOW_HTTP=true
AGENT_RUNTIME_MODEL_CALLBACK_HTTP_HOSTS=helpin-api:8080
```

Both settings are required for a non-localhost HTTP callback. Hosts must include
their exact port; wildcard and suffix matching are not supported. The app config
still requires `token_env` referring to a populated callback secret. Callback
redirects are never followed. Use HTTPS outside a trusted private network.
These settings do not relax the separate policy for user-added MCP servers and
do not remove Runtime's standalone/environment credential support.

The prebuilt dashboard uses a same-origin `/api` base. Set `APP_BASE_URL` to the
staff dashboard origin, `PUBLIC_WIDGET_URL` to the visitor API origin, and
`PUBLIC_SDK_URL` to its `/sdk/lib.js` URL. These are server settings; rebuilding
JavaScript or setting runtime `VITE_API_URL` is unnecessary. Add every website
origin (including a staff preview or help-center origin) to its widget installation.

Community has no default telemetry endpoint or hosted support widget. Explicitly
setting `SENTRY_DSN` opts into error reporting to that endpoint. Optional Postmark
support email also needs operator-owned `SUPPORT_EMAIL_REPLY_DOMAIN` and
`SUPPORT_EMAIL_ROUTE_DOMAIN`; SMTP application mail does not configure these
support channels.

Agent profiles configure agent runs only. Knowledge embeddings still use the
server's `OPENAI_API_KEY`, optional `OPENAI_BASE_URL` (an OpenAI-compatible API
base including `/v1`), and `OPENAI_EMBEDDING_MODEL` (default
`text-embedding-3-small`). The model must return 1,536 dimensions. Without this
configuration, semantic retrieval is unavailable and keyword search remains.
A local chat connection alone does not configure embeddings. Workspace AI
settings reports this distinction; "configured" does not mean the endpoint
has been contacted or verified. Help-center AI answers and automatic triage
also retain server-level chat provider settings and their existing model routes.

The installer provisions pgvector and pgcrypto before the Helpin migrator runs.
With external Postgres, its administrator must install pgvector on the server and
run `CREATE EXTENSION IF NOT EXISTS vector;` and
`CREATE EXTENSION IF NOT EXISTS pgcrypto;` in the Helpin database first.
The runtime uses its own database and GORM schema initialization; it has no Helpin
ledger head. Record its image digest alongside the Helpin migration head.

The core foundation migration runs only for an empty application database. It
creates the tables previously supplied by GORM, then runs the existing SQL
ledger normally. It does not rewrite historical SQL or existing checksums.
Incomplete base schemas fail explicitly rather than being treated as new installs.

When `AUTH_EMAIL_VERIFICATION_REQUIRED=true`, API startup requires a configured application mail sender. This enables verification emails and the verification UI; the current server does not reject all unverified password logins. Do not treat it as a server-enforced login restriction.

`AWS_S3_PRIVATE_BUCKET=true` selects a private ACL-free bucket (the Garage bundle
default). Do not enable Garage website hosting or anonymous reads. API and worker
must share the same storage configuration and `APP_BASE_URL`; private Docs image
URLs use the application origin. Garage has a distinct generated access key,
secret key and RPC secret. Keep them with backups of the entire `garage_data`
volume, including metadata and object data. Changing env values does not rotate
an existing Garage access key; use Garage's documented key management procedure.

The fresh-schema CI job applies the ledger to an empty PostgreSQL database and
compares the schema before and after the API's exact AutoMigrate model list.
Model additions therefore need an additive ledger migration; do not regenerate
or edit the historical foundation snapshot to bypass the guard.

## Authenticated request limits

The API and public MCP use Redis counters shared across replicas. Defaults are
1,200 requests/minute and a separate 120 expensive actions/minute. Configure
`AUTHENTICATED_RATE_LIMIT_PER_MINUTE` and `EXPENSIVE_RATE_LIMIT_PER_MINUTE`;
zero disables the corresponding ceiling. API counters use the authenticated
user across workspaces (changing a workspace header cannot bypass the limit).
MCP counters use the authenticated workspace and user/service principal, across
all their tokens. Different principals do not consume each other's allowance.

Expensive API actions include agent starts/continuations/messages, rewrites,
previews, re-indexing, generation and sync. MCP mutations and workspace search
share the expensive bucket. General requests return HTTP 429 and `Retry-After: 60`;
MCP tool throttling returns a protocol tool error with a retry instruction.
Streams are checked only at connection admission. Internal Runtime callbacks
and visitor routes retain their separate policies. Redis outages fail open with
a warning; this is an abuse ceiling, not a billing or authorization mechanism.

### Allow website crawling

In **Settings → Knowledge**, add or manage a **Website** source and expand
**Allow Helpin to crawl your website**. Publish crawler-specific rules in your site's `/robots.txt`:

```text
User-agent: Helpin-Crawler
Allow: /docs/
Disallow: /

User-agent: CloudflareBrowserRenderingCrawler
Allow: /docs/
Disallow: /
```

Replace `/docs/` with your public knowledge paths. Local crawling identifies as
`Helpin-Crawler/1.0`; optional Cloudflare crawling uses
`CloudflareBrowserRenderingCrawler/1.0`. Update existing groups rather than
adding conflicting rules. For a wholly public site use `Allow: /` without
`Disallow: /`. Keep private paths protected by authentication.

Re-sync after changing rules. Blocked URLs are reported separately from sync
errors and are removed from the index after a successful full refresh. Local
crawling checks links, sitemap URLs, and redirect destinations, caching rules
per origin for that sync. Missing robots.txt (4xx except 429) allows crawling;
429, 5xx, and network errors fail the sync without stale-index cleanup. Uploads
are unaffected. Firewalls may also need to allow the configured crawler on your
public paths; robots.txt does not bypass bot challenges.

Cloudflare enforces robots and content signals itself; see its
[crawler behavior documentation](https://developers.cloudflare.com/browser-run/quick-actions/crawl-endpoint/#robotstxt-and-bot-protection).
