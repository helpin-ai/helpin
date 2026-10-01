# Develop and release Community

Operators use the [installation guide](../../community/README.md). These commands
are for a source checkout, not the downloadable operator bundle.

## Source builds and acceptance

The complete stack builds in Docker; host Go and Node installations are not
required for this path. Install Git, Docker Engine with Compose v2, Bash, and
OpenSSL. Start with at least 8 GB RAM (a nominal 8 GB server reports about
7.7 GiB, which the CLI accepts) and 20 GiB disk for evaluation, and allow
additional memory and disk for source builds and image layers.

Use a new parent directory for these checkouts. Both repositories must be
cloneable at the pinned Runtime revision for this path to work. While the Agent
Runtime repository remains private, the source-build path is available to
maintainers only and operators should use a published bundle.

```sh
git clone https://github.com/helpin-ai/helpin.git
git clone https://github.com/helpin-ai/agent-runtime.git
cd helpin
# Build the Runtime revision this Helpin checkout was packaged against.
git -C ../agent-runtime checkout --detach "$(cat community/runtime-revision.txt)"
cd community
./setup.sh install
# Edit .env if needed; the generated defaults target local evaluation.
docker compose -f compose.yaml -f compose.build.yaml build
./setup.sh start
./setup.sh status
```

The build override resolves Runtime at `../../agent-runtime` relative to
`community/`, or at `AGENT_RUNTIME_SOURCE` if explicitly configured. It builds the
Runtime image used by both its API and worker. The installer generates secrets
and copies [apps.example.json](../../community/apps.example.json) into `apps.json`,
which configures internal Helpin callbacks; no hand-written Runtime configuration
is needed for this Compose path. Keep `.env` private.

Open `http://localhost:8085`, sign up, and create an organization and workspace.
Follow the [first-chat steps](../../README.md#get-started) to allow your website
origin, install the widget, and send a message. AI is optional: configure a shared
connection and workspace profile in Settings → AI to exercise agent execution.
Public articles are available at
`http://localhost:8086/?subdomain=YOUR_HELP_CENTER_SLUG` after publishing.

```sh
# From community/:
./setup.sh logs helpin-api agent-runtime-worker
./setup.sh stop
```

Stop preserves data volumes. See [configuration](configuration.md) for mail and
AI settings, [deployment](deployment.md) for public access, and
[backup/restore](backups.md) before working with persistent data. For host-process
editing rather than container builds, use [local development](../development.md)
and the [Runtime integration guide](../agent-runtime-local-setup.md).

For tests, run from the Helpin repository root:

```sh
node --test community/tests/installer.test.mjs
python3 -m unittest discover -s community/tests -p '*_test.py'
python3 -m unittest discover -s scripts/ci -p '*_test.py'
community/tests/run.sh smoke
community/tests/run.sh full
```

Acceptance requires locally built Community images. Full acceptance also needs
workspace dependencies and Playwright Chromium. Tests create temporary credentials,
unused loopback ports and uniquely named Compose projects. They do not use your
installation's configuration or volumes. Failed runs print service state, then
remove their own resources. They do not upload raw logs or credential-bearing traces.

## CI ownership

- Main CI selects jobs through `scripts/ci/checks.py`. Mobile includes frontend
  changes; root configuration and workflow changes select all checks. Documentation
  changes run lightweight contract/link checks without rebuilding the stack.
- Frontend Vitest runs in three shards. The Community frontend job builds and
  checks the artifact in one job and runs the Community tests in three more;
  `scripts/check-community-frontend.sh [all|build|test] [vitest args]` does the
  same locally, and releases run `all`.
- Image builds pass `OS_PACKAGES_DATE`, so OS package layers refresh daily while
  application layers stay cached; release builds also pull fresh base images.
- Community jobs build and test with the `ee/` directories absent. Enterprise jobs
  own the `-tags ee` builds and tests.
- PostgreSQL 16 checks historical migrations; PostgreSQL 17/pgvector checks the
  fresh ledger against API models and verifies contact anonymization.
- PRs run relevant source/edition checks, schema parity and lightweight installer/
  packaging checks, without building or starting the Docker bundle. Scheduled
  acceptance builds and tests full amd64; manual acceptance can run both native
  architectures. Run it before merging Dockerfile or Compose changes when needed.
- Release candidates run full native amd64 and arm64 acceptance, source validation,
  scans and SBOM generation. Publishing reuses the tested image archives.
- `CI required` rejects failed, cancelled and unexpectedly skipped selected jobs
  and is the aggregate branch-protection check. The `Community bundle` workflow
  runs on schedule or manual dispatch, not on pull requests.

Fork PR jobs use GitHub-hosted runners without private credentials. Internal
jobs retain ARC runners where already configured. ARC runner-group access is
restricted to trusted workflows/refs because a contributor can edit workflow
YAML; the runner selector alone is not an authorization boundary. While the
Runtime repository is private, scheduled/manual/release acceptance uses
`COMMUNITY_RUNTIME_READ_TOKEN` with Contents: read on that repository. PR checks
do not check out Runtime or need that token; do not supply private credentials
or use `pull_request_target` for forks.

Go and Node are pinned in `.go-version` and `.node-version`; pnpm is declared in
`package.json`. CI validates the Community image toolchain pins against those
files. Dependency caches follow lockfiles, and Docker caches are scoped per
architecture and image. Fork runs cannot write the shared build caches.

JavaScript actions use Node 24 and are pinned to reviewed commit SHAs; Dependabot
proposes weekly action updates. Keep ARC runners at version 2.329.0 or newer
before using these workflows. Tokens default to read-only, with write permissions
limited to publishing jobs. Checkout credentials are retained only for Git release
operations. The Doppler and Trivy installers verify versioned archive checksums
from `scripts/ci/install-tool.sh` without running remote installer scripts.

## Candidate and promotion

1. Complete [publication review](../../community/PUBLICATION.md), including real
   DNS/HTTPS acceptance. Configure the protected `community-release` environment.
   Give Helpin’s workflow token write access to the candidate GHCR image
   repositories, including `agent-runtime`, before dispatching a candidate.
   Verify anonymous pulls of the candidate GHCR images before promotion.
2. Dispatch **Community release candidate** on the intended commit with a new
   `community-v0.x.y[-rc.n]` tag and the HTTPS acceptance record.
3. Both native jobs build/test/scan once. Protected jobs publish those exact
   image archives and assemble an immutable, digest-pinned operator bundle.
4. Review the successful run and its `community-bundle` artifact. It includes
   the archive, SHA-256 checksum and `release.json` provenance.
5. Dispatch **Promote Community candidate** with that candidate run ID. After
   environment approval, it verifies provenance/checksums and publishes the same
   files as a GitHub prerelease. There is no rebuild. Existing releases/tags are
   refused, so partial publication must be reviewed manually before retrying.
6. Verify the release assets download and the [operator instructions](../../community/README.md).

Image archive artifacts expire after seven days; completed operator bundle
artifacts expire after thirty days. Promote or build a new candidate within that
window. Candidate and promotion workflows share a publication lock and refuse
already released tags before images are rebuilt or pushed. Never regenerate an old candidate's assets under an already published tag.
