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
  changes; root configuration and workflow changes select all checks.
- Community jobs own builds/tests with EE source absent. EE jobs own EE builds/tests.
- PostgreSQL 16 checks historical migrations; PostgreSQL 17/pgvector checks the
  fresh ledger against API models and verifies contact anonymization.
- Relevant PRs run amd64 smoke acceptance. Scheduled acceptance runs full amd64;
  manual acceptance can run both native architectures.
- Release candidates run full native amd64 and arm64 acceptance, source validation,
  scans and SBOM generation. Publishing reuses the tested image archives.
- `CI required` rejects failed, cancelled and unexpectedly skipped selected jobs.
  Existing leaf check names remain. Maintainers should require the new aggregate
  only after observing a passing run and reviewing existing protection rules.

Fork PR jobs use GitHub-hosted runners without private credentials. Internal
jobs retain ARC runners where already configured. While Runtime is private,
internal acceptance requires `COMMUNITY_RUNTIME_READ_TOKEN` with Contents: read
on that repository. Fork acceptance reports this dependency as blocked; do not
supply private credentials or use `pull_request_target` to bypass that boundary.
Public release requires the pinned Runtime source to be publicly available.

Go and Node are pinned in `.go-version` and `.node-version`; pnpm is declared in
`package.json`. CI validates the Community image toolchain pins against those
files. Dependency caches follow lockfiles, and Docker caches are scoped per
architecture and image. Fork runs cannot write the shared build caches.

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
window. Never regenerate an old candidate's assets under an already published tag.
