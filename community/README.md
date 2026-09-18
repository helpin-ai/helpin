# Helpin Community 0.1 beta

This guide is for operators installing the self-hosted Community bundle. It
covers requirements, installation, the first support conversation, and
day-to-day commands. Community 0.1 is a beta; read the
[scope and known limitations](../ROADMAP.md) before serving production traffic.
Contributors building from a source checkout should use the
[development guide](https://github.com/helpin-ai/helpin/blob/develop/docs/community/development.md)
instead.

## Requirements

- Docker Engine and Docker Compose v2
- Bash and OpenSSL
- An amd64 or arm64 host; native architecture tests are a release gate, not an
  inference from successful builds
- At least 8 GiB RAM and 20 GiB free disk for evaluation

These are evaluation starting points, not measured capacity promises.

## Install

Download the bundle archive and its checksum from the
[releases page](https://github.com/helpin-ai/helpin/releases). If no bundle is
published yet, build from source with the development guide above. Verify the
checksum, extract the archive, and run from its `community/` directory:

```sh
./setup.sh install
# Edit .env: public origins, optional SMTP, and optional server AI configuration.
./setup.sh start
./setup.sh status
```

The installer generates secrets and writes `.env`; it never sources that file
as shell code, and re-running `install` preserves existing values. Keep `.env`
private and back up its encryption keys together with your data: changing the
keys only in the environment makes stored encrypted credentials unreadable.

Open `http://localhost:8085`, sign up, and create your organization and
workspace. Local signup does not require email, and new accounts remain
unverified. With working application mail, `AUTH_EMAIL_VERIFICATION_REQUIRED=true`
enables verification emails and the verification UI; it does not block every
unverified login.

## Start your first conversation

1. In workspace settings, add your website origin first (scheme, hostname, and
   port). The widget refuses visitor requests while the list is empty. Add the
   dashboard origin for previews and the help-center origin if it embeds chat.
2. Copy the generated support-only installation snippet into your website. The
   snippet uses this installation's public URLs and sends no analytics to Helpin.
3. Open the website, send a visitor message, and reply from the staff inbox.

## Add AI and a help center

Configure API-key connections and a shared profile under **Settings → AI**, then
choose a workspace default. Personal connections belong in personal AI settings.
ChatGPT is optional and must be enabled on both Helpin and Runtime; unattended
runs remain subject to their existing policy. Knowledge embeddings and non-agent
AI use separate server configuration; see
[configuration](../docs/community/configuration.md).

Create and publish your first article in Docs / Help Center. Configure its
custom domain for public hosting. Locally, open
`http://localhost:8086/?subdomain=YOUR_HELP_CENTER_SLUG`.

## Operate

```sh
./setup.sh logs helpin-api agent-runtime-worker
./setup.sh stop
```

`stop` preserves named volumes. Never run `docker compose down -v` on an
installation you want to keep.

Published bundles pin exact image digests. Changing a version variable does not
upgrade an installation; use a reviewed replacement bundle. Tested cross-version
upgrades start in 0.2.

## Next steps

- [Deploy publicly](../docs/community/deployment.md): DNS, HTTPS, website origins, and storage.
- [Configure the installation](../docs/community/configuration.md): SMTP, AI, embeddings, and optional capabilities.
- [Back up and restore](../docs/community/backups.md): preserve data and encryption keys.
- [Troubleshoot](../docs/community/troubleshooting.md): service startup, widget, AI, mail, and storage problems.
- [Security policy](../SECURITY.md): supported versions and private vulnerability reporting.
- [Get help](https://github.com/helpin-ai/helpin/blob/develop/SUPPORT.md): questions, bug reports, and feature requests.
