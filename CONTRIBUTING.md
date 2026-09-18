# Contributing to Helpin Community

Community 0.1 is a beta focused on support conversations, visitor identification,
and the public help center. Read [the Community guide](community/README.md) and
[known limitations](ROADMAP.md) before starting a change.

Open an issue describing the problem and a concrete example. For larger changes,
agree on scope first. Keep changes small and include focused tests for behavior
and security boundaries. Do not include customer data, credentials, or private
operational configuration in issues, fixtures, screenshots, or logs.

Use the pinned Go and pnpm versions from the build files. Community Go commands
omit `-tags ee`; commercial changes also require the EE checks. Run the affected
Go tests with CGO enabled (SQLite is used in tests), the relevant frontend or SDK
tests, and TypeScript checking. The Community Compose smoke exercises the public
installation rather than a developer's existing database.

Public visitor APIs must preserve workspace, origin, session, and identity
boundaries. Agent credentials stay out of metadata and logs. Historical ledger
migrations are immutable; add a new migration and test it against Postgres.

See [the code of conduct](CODE_OF_CONDUCT.md) and
[security reporting](SECURITY.md).

## Licensing contributions

The community application is AGPL-3.0-only; enterprise directories have a
separate commercial license; public SDKs and embedded widget packages use
Apache-2.0. See [LICENSE](LICENSE) for the exact directory scopes. Preserve
existing third-party licenses and notices, and identify any third-party
material you add.

For contributions to AGPL or enterprise code, each contributor must accept
the [Helpin Contributor License Agreement](CLA.md) using its acceptance
statement in the pull request. Maintainers must verify acceptance before
merging. Contributors retain ownership; the agreement lets Helpin include
their contributions in both community and commercial editions.

Contributions solely to Apache-2.0 components use Apache-2.0's contribution
terms and do not require the additional CLA. The [publication checklist](community/PUBLICATION.md)
still applies before publishing this repository or its history.

## Development and documentation

Start with the [architecture overview](ARCHITECTURE.md),
[local development guide](docs/development.md), and the
[documentation index](docs/README.md). Component-specific instructions live near
the code; `AGENTS.md` describes repository conventions.

## Submit a pull request

1. Keep changes focused on one problem and follow the existing component patterns.
2. Add or update tests when behavior changes, and run the relevant checks in the
   [development guide](docs/development.md#validate-changes).
3. Update documentation for changed setup, configuration, or public behavior.
4. Describe the problem, resulting behavior, and validation in the pull request.
   Include screenshots for UI changes when useful.

## Documentation conventions

Follow the [documentation writing and naming guide](docs/documentation-guide.md).
Use lowercase hyphenated filenames, descriptive page titles, and a short opening
that explains the reader, purpose, and scope.

- Current behavior and setup belong in engineering guides linked from
  [docs/README.md](docs/README.md).
- Product requirements belong in `docs/prds/`; implementation plans in
  `docs/plans/`; design specifications in `docs/specs/`.
- Operational procedures belong in `docs/ops/`; research and assessments in
  `docs/research/`; product strategy in `docs/strategy/`.
- Keep historical plans only while they explain a still-relevant decision or
  requirement. Remove superseded implementation checklists once current guidance
  covers the behavior; Git history preserves them. A proposal or dated plan is
  not evidence that a feature has shipped.
- Add documents to the relevant directory index, use relative links, and update
  references when moving a file.
- Keep temporary screenshots, test output, and local scratch files out of the
  repository root. Store intentional documentation assets beside their guide or
  in `docs/mockups/`; preserve assets required by application builds.

Product tutorials will be maintained at [helpin.ai/docs](https://helpin.ai/docs)
when the hosted help center is available.
Link to those articles rather than duplicating them in the repository. Keep
version-sensitive setup, architecture, and API contracts alongside code. The
[PR template](.github/PULL_REQUEST_TEMPLATE.md) asks for affected documentation.

For documentation changes, run:

```sh
python3 scripts/docs/check_names.py
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
git diff --check
```

Add new maintained technical guides to
[scripts/docs/maintained-docs.json](scripts/docs/maintained-docs.json). Read the
[checker scope](scripts/docs/README.md) and [publication review](docs/publication-review.md)
when adding public documentation. Passing link checks does not authorize publication.
