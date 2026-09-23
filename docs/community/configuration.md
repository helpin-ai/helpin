# Community configuration

This reference is for operators configuring a Community installation. It lists
the environment settings the bundle supports and their defaults; use it after
installation and before public deployment. Community 0.1 is a beta. The
supported bundle sets explicit local defaults; running the binaries directly
retains conservative authentication defaults.

| Setting | Behavior |
| --- | --- |
| `HELPIN_ENABLED_MODULES` | Comma-separated product surfaces. Both editions default to all modules: `support,docs,agents,pm,crm,automation`. Support requires Docs. Workspace roles remain in force. Upgrading an installation whose value is still the Community 0.1 default `support,docs,agents` switches it to the new default; any other value is kept. |
| `CRM_ENCRYPTION_KEY`, `GIT_OAUTH_ENCRYPTION_KEY` | Generated 32-byte hex keys. The first encrypts CRM mail and calendar tokens and the SMTP password saved in **Settings → System status**, and is the fallback key for TOTP and PM import secrets; the second encrypts stored Git provider tokens. Keep them stable and back them up with the databases. `helpin upgrade` generates them when an older `.env` lacks them. |
| `SETUP_SUCCESS_ENABLED` | Optional. Controls the workspace Setup guide. Empty uses the edition default: on in Community, off in Enterprise. Set `false` to hide it. |
| `AUTH_EMAIL_VERIFICATION_REQUIRED` | Defaults to `true`. Set `false` for local Community signup without mail. Enterprise rejects `false`. This never marks an email verified. |
| `DEMO_VIEWER_EMAIL` | Optional. Email of an existing account that visitors of `/demo` are signed in as without a password. Give it the `viewer` role in one workspace only, no 2FA, not a platform admin. Every non-read API request from this account is rejected with `demo_read_only`. Empty disables `/demo`. |
| `DEMO_REQUIRE_EMAIL` | Defaults to `false`. When `true`, visitors must enter their own email before the demo session is issued. |
| `DEMO_LEAD_WEBHOOK_URL` | Optional. Visitor emails captured on `/demo` are POSTed here as `{"email","source":"demo","captured_at"}`. Failures are logged and never block the visitor. |
| `SMTP_HOST`, `SMTP_PORT` | Optional application mail relay; port defaults to 587. SMTP takes precedence over application Postmark when configured. |
| `SMTP_FROM` | Required sender address when SMTP is enabled. |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | Set both for authenticated delivery, or leave both empty for a trusted relay. |
| `SMTP_TLS_MODE` | `starttls` by default; `tls` for implicit TLS; explicit `none` only for an unauthenticated trusted local relay. Credentials require TLS and AUTH PLAIN; LOGIN-only SMTP servers are unsupported. TLS certificates are verified. |
| `POSTMARK_APP_SERVER_TOKEN`, `POSTMARK_APP_FROM_EMAIL` | Optional alternative application-mail provider. |
| `HELPIN_ADMIN_EMAILS` | Optional comma-separated addresses of [server admins](#server-administration). Matching accounts are made server admins at startup, and a new account on one of these addresses becomes one once its email is verified. Accounts are matched by address, so list only mailboxes you control. |

`helpin install` and `helpin configure` can write the SMTP settings for you
(see the [CLI guide](cli.md)). Server admins can instead enter them in
**Settings → System status → Application email**; see
[Application email in the app](#application-email-in-the-app). Workspace
administrators can send a test email to their own address with
`POST /api/workspaces/{id}/email/test`, and server admins with
`POST /api/instance/email/test` (at most one every 30 seconds and five per hour
per user). The result is recorded, and the `email_outbound` capability reports
`ready` only after a successful test of the current mail settings; changing the
host, port, username, sender, or TLS mode, or saving the settings again,
requires a new test.

SMTP sends invitations, password reset, optional verification and application
notifications. It does **not** enable support reply threading or inbound mail;
those integrations still use optional Postmark. Without application mail, local
signup works when verification is disabled, invitations are created as links
that the inviter copies and shares, and password reset reports that mail is
unavailable. Google OAuth is optional.

## Server administration

Community has server admins, who manage the settings that apply to every
workspace on the server: the signup policy, the list of server admins, and
application email. Server admins see **Settings → System status** and
**Settings → Signup & admins**; workspace owners and admins who are not server
admins do not. Server admin is separate from workspace roles and from the
hosted-operator `PLATFORM_ADMIN_EMAILS` role. Enterprise has no server admins,
and its signup and application mail are unchanged.

- **The first account is the server admin.** On a new server, the first person
  to sign up becomes the server admin. Concurrent first signups are resolved in
  the database, so exactly one account wins.
- **Upgrades.** When an existing server upgrades, the owner of its oldest
  organization (normally whoever set it up) becomes the server admin; if there
  are no organizations, the oldest account does. The API repeats this check at
  startup, so a server whose only admin account was deleted gets a new admin.
- **More admins.** A server admin can make any existing account a server admin
  by email, and remove admins. The last server admin cannot be removed, and
  admins listed in `HELPIN_ADMIN_EMAILS` can only be removed there.

### Signup policy

**Settings → Signup & admins** chooses who can create an account. Invitations
work under every policy.

| Policy | Who can sign up |
| --- | --- |
| Invite only | Nobody without an invitation. The default for new servers once the first account exists. |
| Approved email domains | Addresses on the listed domains (exact match; list subdomains separately). A password signup must confirm its email before it can sign in, even when `AUTH_EMAIL_VERIFICATION_REQUIRED=false`, because anyone can type an address on an allowed domain. This policy therefore requires working application email. Google sign-in counts as verified. |
| Anyone | Anyone who can reach the server. |

Servers that existed before server administration keep **Anyone** so nobody is
locked out by the upgrade; the page recommends switching to invite only. The
sign-in and registration pages read the policy from `GET /api/auth/config`
(`signup_mode`, `signup_allowed_domains`, and `signup_first_user`, which is
`true` until the first account exists) and hide self-signup on invite-only
servers. The API enforces the policy for password and Google signups and
rejects others with `403` and the code `signup_restricted`.

### Application email in the app

Server admins can enter SMTP settings (server, port, username, password,
sender, and security) under **Settings → System status → Application email**,
then send a test email. Saved settings apply without restarting the API; other
API replicas pick them up within 30 seconds. The password is stored encrypted
with `CRM_ENCRYPTION_KEY` and is never returned by the API. An unauthenticated
local relay needs no password.

`SMTP_*` and `POSTMARK_APP_*` variables take precedence: while they configure
application email, the page shows them read-only as set by the server's
configuration, and saved settings are ignored. Remove the variables and restart
the API to manage email in the app instead.

### Invitations without email

When application email is not configured, creating an invitation still
succeeds: the response (`POST /api/invitations`) includes the `join_url` and
`"email_sent": false`, and **Settings → Members** offers **Copy invite link** for
pending invitations. Invite links expire after seven days and use the normal
join flow, including on invite-only servers.

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

Agent profiles configure agent runs only. Knowledge embeddings use the
server's `OPENAI_API_KEY`, optional `OPENAI_BASE_URL` (an OpenAI-compatible API
base including `/v1`), and `OPENAI_EMBEDDING_MODEL` (default
`text-embedding-3-small`). When `OPENAI_API_KEY` is empty and `OPENROUTER_API_KEY`
is set, embeddings use OpenRouter's embeddings endpoint (at `OPENROUTER_BASE_URL`
when set) with `openai/text-embedding-3-small`; an `OPENAI_EMBEDDING_MODEL`
without a vendor prefix gets `openai/` added. `OPENAI_API_KEY` always takes
precedence. The model must return 1,536 dimensions. A server key serves every
workspace.

Without either server key, each workspace uses its own shared (workspace-scope)
AI connection for embeddings: a connected OpenAI connection first, then a
connected OpenRouter connection, preferring the connection Helpin created for
that provider. Both request `text-embedding-3-small`; OpenRouter serves it as
`openai/text-embedding-3-small`. The stored vectors therefore stay compatible
when a workspace switches between them. Vectors are recorded with their model
name and 1,536 dimensions, and a response with any other dimension is rejected.
Anthropic connections (no embeddings API), OpenAI-compatible endpoints (no
embedding model setting) and personal connections are never used for
embeddings. This usage is recorded in the AI action audit with the connection
and funding mode; in Community it is not billed, and Enterprise uses a
connection only when the workspace's AI connection policy admits it.
When a workspace connects OpenAI or OpenRouter, Helpin indexes the knowledge
that has no vectors yet (disabled or failed knowledge sources, unindexed help
center spaces, and content sources that were never embedded) in the background.
If neither the server nor the workspace has a key, semantic retrieval is
unavailable and keyword search remains.

Workspace AI settings and the `ai_embeddings` capability name the source in use
("Using the server's OpenAI key" or "Using this workspace's OpenRouter
connection"); "configured" does not mean the endpoint has been contacted or
verified. Help-center AI answers, automatic triage and support coverage
analysis keep server-level provider settings and their existing model routes.

A workspace's website is optional. When one is given, onboarding and Settings →
Knowledge can draft the company/product context from it. The draft runs on the
workspace's default AI profile and its connection (OpenAI, Anthropic,
OpenRouter or an approved OpenAI-compatible endpoint). When no workspace
connection can run it, the server's OpenRouter key is used if it is set. A server
`OPENAI_API_KEY`, `ANTHROPIC_API_KEY` or `OPENROUTER_API_KEY` becomes a shared
AI connection in every workspace, including new ones, so this works without
further setup; Settings reports it as untested until someone tests it. Without any AI, the
request fails at once and users write the context themselves. Helpin reads only
public addresses: it refuses private, loopback, link-local and cloud metadata
addresses, including after redirects. Sites that need JavaScript or block
automated visitors cannot be read.

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

## GitHub App

Helpin connects to GitHub through one GitHub App per installation. A workspace
owner can create it from **Settings → Git Connections**, the Setup guide,
**Settings → System status**, or the optional **Connect GitHub** onboarding step
with **Create GitHub App**. Onboarding offers that step to the workspace owner
who chooses **Plan and ship team projects** while GitHub isn't connected yet;
it can be skipped.

1. Choose who owns the App: **a GitHub organization** (recommended; enter its
   login) or **your personal GitHub account**.
2. Helpin sends a [GitHub App manifest](https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest)
   to GitHub, which asks you to confirm the new App. GitHub then returns to
   `/api/github/app-manifest/callback`, and Helpin stores the App. The API and the
   worker use it at once, without a restart.
3. Helpin sends you straight to GitHub's install page for the new App. Choose the
   repositories and install. GitHub returns to `/api/git/github/callback`, Helpin
   connects the installation to the workspace's organization, and you land back
   on the page you started from.

The App is **private**: GitHub only lets the account that owns it install it, and
shows everyone else a 404 page. Create it under the organization that owns your
repositories. An App owned by your personal account can only reach your personal
repositories. Git connections and the Setup guide name the owner when the App is
configured but not yet installed.

If you install the App, or change its repository access, from GitHub instead of
from Helpin, GitHub returns without Helpin's signed link. Helpin then opens
`/github/installed`, checks with GitHub that the installation belongs to this
App, and connects it to your workspace's organization. You must be an
organization owner or admin. An installation already connected to another Helpin
organization is refused.

- GitHub must reach this server: the manifest uses `APP_BASE_URL` for the App
  homepage, its webhook (`/api/git/webhook`), and its setup and callback URLs. Use
  an `https` URL on a public hostname. When `APP_BASE_URL` uses `http`,
  `localhost`, a `.local` or single-label hostname, or a private, loopback or
  link-local IP address, **Create GitHub App** is replaced by an explanation,
  `manifest_blocked_reason` is set in the App status, and System status and
  `helpin doctor` report the same problem for GitHub.
- The App requests repository contents (write), pull requests (write), checks
  (read), and metadata (read), with `push`, `pull_request`, `release`, and
  `check_suite` events.
- The private key, client secret, and webhook secret are encrypted with
  `GIT_OAUTH_ENCRYPTION_KEY` (or `CRM_ENCRYPTION_KEY` when it is unset). The
  button is unavailable without a valid key, and a lost key makes the stored App
  unreadable.
- Helpin refuses to replace an App that already exists. To start over, delete the
  App on GitHub and the row in `github_app_credentials`.
- Webhook deliveries must carry a valid `X-Hub-Signature-256` for the App's
  webhook secret; unsigned or wrongly signed deliveries are rejected with 401.

To pin an existing App instead, set `GITHUB_APP_ID`, `GITHUB_APP_SLUG`,
`GITHUB_APP_PRIVATE_KEY` (PEM, escaped or base64-encoded PEM), and
`GITHUB_APP_WEBHOOK_SECRET`; `GITHUB_APP_CLIENT_ID` and
`GITHUB_APP_CLIENT_SECRET` are optional. When `GITHUB_APP_ID` and
`GITHUB_APP_PRIVATE_KEY` are both set, these settings take precedence over a
stored App and the create button is hidden. Without `GITHUB_APP_WEBHOOK_SECRET`,
all GitHub webhooks are rejected. `GET /api/workspaces/{id}/github/app-status`
reports `configured`, `source` (`env`, `database`, or `none`), `slug`,
`install_url`, `webhook_configured`, `manifest_available`,
`manifest_blocked_reason`, `owner_login`, `owner_type` (`User` or
`Organization`), and `private`, without secrets. For an App set through these
settings, Helpin asks GitHub for the owner once; `private` is only reported for
an App created from Helpin. Enterprise configures the App only through these
settings and connects installations only through Helpin's install link.

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

### Retrieved knowledge and agent permissions

Support knowledge results are marked `untrusted_reference`: source text, titles,
URLs, uploaded files, and curated guidance provide evidence, not tool permissions
or instructions. The host adds this rule to saved support-agent presets as well
as new ones. Preview sends escaped JSON reference data outside the system prompt.
Original content and evidence IDs remain available for citation validation.

Keep each agent's tools and approvals scoped to its job. Trust marking reduces
prompt-injection risk but is not a guarantee of model resistance. The regression
suite checks hostile source delimiters, forged evidence/policy fields, saved
preset upgrades, and the existing server-side reply gate; it is not a live-model
security evaluation.
