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

- [Troubleshoot](../docs/community/troubleshooting.md): service startup, widget, AI, mail, and storage problems.
- [Product help](https://helpin.ai/docs): forthcoming guides hosted in Helpin.
- [Deploy publicly](../docs/community/deployment.md): DNS, HTTPS, website origins and storage.
- [Configure the installation](../docs/community/configuration.md): SMTP, AI, embeddings and optional capabilities.
- [Back up and restore](../docs/community/backups.md): preserve data and encryption keys.
- [Develop and release](https://github.com/helpin-ai/helpin/blob/develop/docs/community/development.md): source builds, CI and candidate promotion.

Published bundles pin exact image digests. Changing a version variable does not
upgrade one; use a reviewed replacement bundle. Cross-version upgrades start in 0.2.

### Optional Ask Agent execution

The Community image now includes Python, pip and venv. Compose runs a separate
`agent-runtime-execution` service on the existing coding queue, with concurrency 50 per worker process.
Shared workers do not poll that queue or mount the execution volume. Enable
execution explicitly in an owned Ask Agent conversation; the next message after
a safe transition starts a separately routed run. Existing permissions and
publication approvals still apply.

Use `./setup.sh start` to load `apps.json` as inline execution-worker configuration.
For direct Compose commands, first export it without printing it:

```sh
export AGENT_RUNTIME_EXECUTION_APP_CONFIG="$(cat apps.json)"
docker compose up -d
```

The execution service does not mount `apps.json`. Python's private venv, pip cache,
temporary files and outputs live on `execution_workspaces`, so installation does
not write to the read-only root or shared system environment. Analysis scratch
has a monitored 512 MiB operational limit and is removed when its run ends.
There is no inactivity timer. Files and packages do not transfer to successor
runs; explicitly published private artifacts remain downloadable.

Commands **can read other runs' checkouts on the shared execution volume**.
Filtered environments and process dumpability hardening reduce credential
exposure but do not provide tenant or filesystem isolation. Enable this only for
users trusted to execute in that worker container.

TypeSafe review is optional operator configuration on the runtime workers:
`TYPESAFE_API_KEY`, `AGENT_RUNTIME_TYPESAFE_ENABLED=true`, and optionally
`AGENT_RUNTIME_TYPESAFE_ASKED_THRESHOLD` (default minimum 0.75),
`AGENT_RUNTIME_TYPESAFE_HAZARD_THRESHOLD` (default maximum 0.25), and
`AGENT_RUNTIME_TYPESAFE_ESCALATION_THRESHOLD` (prompt above 0.50). Enabling it sends selected
command/repository context to TypeSafe's official endpoint. Automatic approval
requires `AGENT_RUNTIME_TYPESAFE_AUTO_APPROVE=true`; keep it off until the runtime's
live command-review evaluations pass. No key/provider failure retains the existing
risk-label policy, and human-only/never approval modes remain unchanged.

Auto-approval only suppresses prompts for local-operation candidates. Pushes, PRs,
network installation and API writes do not acquire authorization from review scores;
use the existing human approval flow. The seventh `external_send_requested` score
is diagnostic only. High hazards escalate routine calls and appear in the approval
summary. The reviewer does not enforce network isolation or package-name safety.

`AGENT_RUNTIME_TYPESAFE_MODEL` defaults to `jev-latest`, but prompt suppression
requires the response to match `AGENT_RUNTIME_TYPESAFE_EVALUATED_MODEL` (default
`jev-1.13.0`). Reevaluate each version before changing that setting. The supplied
ad hoc trial reported 22/34 benign approvals and 0/44 false approvals at 0.75/0.25;
these are not guarantees or fresh measurements. The runtime release suite contains
39 reconstructed scenarios, each run twice when live evaluation is enabled.
Remove the obsolete single-choice `AGENT_RUNTIME_TYPESAFE_THRESHOLD` setting
when migrating; it is not equivalent to the new thresholds.
