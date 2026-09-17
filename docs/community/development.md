# Develop and release Community

Operators use the [installation guide](../../community/README.md). These commands
are for a source checkout, not the downloadable operator bundle.

## Source builds and acceptance

Check out Helpin and Agent Runtime as sibling repositories. Runtime must match
`community/runtime-revision.txt`. Local source builds use:

```sh
cd community
./setup.sh install
docker compose -f compose.yaml -f compose.build.yaml build
./setup.sh start
```

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
- Community jobs own builds/tests with EE source absent. EE jobs own EE builds/tests.
- PostgreSQL 16 checks historical migrations; PostgreSQL 17/pgvector checks the
  fresh ledger against API models and verifies contact anonymization.
- PRs run relevant source/edition checks, schema parity and lightweight installer/
  packaging checks, without building or starting the Docker bundle. Scheduled
  acceptance builds and tests full amd64; manual acceptance can run both native
  architectures. Run it before merging Dockerfile or Compose changes when needed.
- Release candidates run full native amd64 and arm64 acceptance, source validation,
  scans and SBOM generation. Publishing reuses the tested image archives.
- `CI required` rejects failed, cancelled and unexpectedly skipped selected jobs.
  Keep `CI required` as the aggregate protection check. Remove any separately
  required `Community bundle` leaf check from branch protection: that PR job no
  longer runs.

Fork PR jobs use GitHub-hosted runners without private credentials. Internal
jobs retain ARC runners where already configured. Before making the repository
public, restrict ARC runner-group access to trusted workflows/refs: a contributor
can edit workflow YAML, so the runner selector alone is not an authorization boundary. While Runtime is private,
scheduled/manual/release acceptance requires `COMMUNITY_RUNTIME_READ_TOKEN` with
Contents: read on that repository. PR checks do not check out Runtime or need that
token; do not supply private credentials or use `pull_request_target` for forks.
Public release requires the pinned Runtime source to be publicly available.

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
   Before public promotion, make the pinned Runtime source available publicly
   and verify anonymous pulls of the candidate GHCR images.
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

## Cleanup validation — September 16, 2026

Verified locally on Linux amd64:

- CI selection/required-status/cache policy: 5 tests passed.
- Runner, archive, image identity and promotion contracts: 14 tests passed;
  publication calls are mocked. Installer/readiness: 4 tests passed.
- Community backend build/tests with EE source absent, and full EE backend tests: passed.
- PostgreSQL 16 migration regressions and PostgreSQL 17 schema/privacy checks: passed.
- Community frontend build/artifact check: passed; 2,796 tests passed, 3 existing skips.
- EE frontend build: passed; 2,829 tests passed, 1 existing skip.
- Full isolated browser/AI/mail/restore acceptance: passed with the existing local
  Community images. Restored credentials and private attachment bytes/policy passed.
- Real SIGTERM cancellation with mocked Docker, failure cleanup and ownership
  checks passed; no acceptance containers/volumes remained after live validation.
- Compose-to-Bake definitions, workflow lint, shell checks and bundle links passed.

Acceptance-only timings with existing images were approximately 58 seconds for
smoke and 239 seconds for the final full run. These are local observations, not
an end-to-end CI benchmark: builds, registry/cache transfer and runner scheduling
are excluded. The PR path now omits full browser/mail/restore phases, host workspace
installation for those phases, and duplicate Community frontend/API builds.

Native arm64, fresh candidate image builds/scans, GitHub cache behavior and real
registry/release promotion still run through their CI/release gates. They are not
claimed by these local checks. No release, tag, repository visibility or branch
protection setting was changed during this cleanup.
