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
| `SMTP_TLS_MODE` | `starttls` by default; `tls` for implicit TLS; explicit `none` only for a trusted local relay. TLS certificates are verified. |
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
