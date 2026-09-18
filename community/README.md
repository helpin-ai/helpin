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
- [Develop and release](https://github.com/helpin-ai/helpin/blob/develop/docs/community/development.md): source builds, CI, and candidate promotion.

### Optional Ask Agent execution

The Community image includes Python, pip, and venv. One `agent-runtime-worker`
service polls every queue, so ordinary chat, coding, and `run_python` share a
single process and the `execution_workspaces` volume. A long command therefore
competes with chat for that worker; Community accepts that contention rather
than running a second service. Enable execution explicitly in an owned Ask
Agent conversation; the next message after a safe transition starts a
separately routed run. Existing permissions and publication approvals still
apply.

The worker applies a Landlock filesystem ruleset to each command when the host
kernel and Docker seccomp profile allow it, and otherwise logs one line and
runs the command without it. Nothing fails closed and there are no host
requirements.

The worker reads its app configuration inline rather than from a mounted file,
so commands cannot read it. Use `./setup.sh start` to load `apps.json` that way.
For direct Compose commands, first export it without printing it:

```sh
export AGENT_RUNTIME_EXECUTION_APP_CONFIG="$(cat apps.json)"
docker compose up -d
```

Only the API service mounts `apps.json`. Python's private venv, pip cache,
temporary files, and outputs live on `execution_workspaces`, so installation does
not write to the read-only root or shared system environment. Analysis scratch
has a monitored 512 MiB operational limit and is removed when its run ends.
There is no inactivity timer. Files and packages do not transfer to successor
runs; explicitly published private artifacts remain downloadable.

When Landlock is unavailable and best-effort mode falls back, commands can read
other runs' checkouts on the shared execution volume. Filtered environments and
process dumpability hardening reduce credential exposure but do not provide
tenant or filesystem isolation. Enable this only for users trusted to execute
in that worker container.

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
network installation, and API writes do not acquire authorization from review scores;
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
