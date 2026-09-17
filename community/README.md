# Helpin Community 0.1 beta

Self-host support chat, visitor identification, a staff inbox, a public help
center, and support agents. No billing service or Helpin account is required.
See [known limitations](../ROADMAP.md). Community 0.1 is a beta.

## Install

**Pre-release:** the current candidate is local and unpublished. The bundle steps
below apply once a release is published. Contributors with repository access can
use the [source-build guide](https://github.com/helpin-ai/helpin/blob/develop/docs/community/development.md#source-builds-and-acceptance)
now. See [publication gates](https://github.com/helpin-ai/helpin/blob/develop/community/PUBLICATION.md) for remaining release requirements.

Download the bundle and checksum from the [Community releases](https://github.com/helpin-ai/helpin/releases).
Verify and extract the archive, then enter its `community/` directory. Candidate
Actions artifacts are for maintainers until a release is published. Requirements: Docker Engine,
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
Local signup does not require email. New accounts remain unverified. With working application mail, `AUTH_EMAIL_VERIFICATION_REQUIRED=true` enables
verification emails and the verification UI; it does not block every unverified login.

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

## Next steps

- [Deploy publicly](../docs/community/deployment.md): DNS, HTTPS, website origins and storage.
- [Configure the installation](../docs/community/configuration.md): SMTP, AI, embeddings and optional capabilities.
- [Back up and restore](../docs/community/backups.md): preserve data and encryption keys.
- [Develop and release](https://github.com/helpin-ai/helpin/blob/develop/docs/community/development.md): source builds, CI and candidate promotion.

Published bundles pin exact image digests. Changing a version variable does not
upgrade one; use a reviewed replacement bundle. Cross-version upgrades start in 0.2.
