# Contributing to Helpin Community

Thanks for helping improve Helpin. This guide explains how to report problems,
set up a development environment, make a change, and submit a pull request.
Community 0.2 is a beta that includes support, the help center, Docs,
Projects, CRM, automation, and agents; read the
[installation guide](community/README.md) and
[scope and known limitations](ROADMAP.md) before starting a change.

## Ways to contribute

- **Report a bug** with the
  [bug report form](https://github.com/helpin-ai/helpin/issues/new?template=community-bug.yml)
  and a reproduction that uses synthetic data.
- **Propose an improvement** with the
  [feature request form](https://github.com/helpin-ai/helpin/issues/new?template=community-feature.yml).
  For larger changes, agree on scope in the issue before implementing.
- **Improve documentation.** Corrections and clearer setup steps are welcome;
  follow the conventions below.
- **Submit a fix.** Keep changes small and include focused tests for behavior
  and security boundaries.

Do not include customer data, credentials, or private operational configuration
in issues, fixtures, screenshots, or logs. Follow the
[code of conduct](CODE_OF_CONDUCT.md) and report vulnerabilities through
[SECURITY.md](SECURITY.md), not public issues.

## Set up a development environment

Start with the [architecture overview](ARCHITECTURE.md) and the
[local development guide](docs/development.md) for host Go and frontend
processes. To build and test the complete Community stack in Docker, use the
[Community development guide](docs/community/development.md). The
[documentation index](docs/README.md) lists deeper guides; component-specific
instructions live near the code, and `AGENTS.md` describes repository
conventions.

Use the pinned Go and pnpm versions from the build files.

## Make a change

Classify the change as shared Community, Enterprise/cloud-only, or private material
and follow the [edition boundary rules](AGENTS.md#community-and-cloud-boundaries).
Ask the maintainer when ownership or publication scope is unclear. Enterprise
licensing and build exclusions do not hide source in a public repository.

- Community Go commands omit `-tags ee`; changes that touch enterprise code also
  require the `-tags ee` checks.
- Public visitor APIs must preserve workspace, origin, session, and identity
  boundaries. Agent credentials stay out of metadata and logs.
- Applied migrations are immutable. Add a new migration and test it against
  PostgreSQL; an updated GORM model alone does not update a bundled installation.
- Follow existing component patterns and keep each change focused on one problem.

## Validate

Run the affected Go tests with CGO enabled (SQLite is used in tests), the
relevant frontend or SDK tests, and TypeScript checking. The
[validation section](docs/development.md#validate-changes) of the development
guide lists the commands. The Community Compose smoke exercises the public
installation rather than a developer's existing database; see
[CI ownership](docs/community/development.md#ci-ownership).

For documentation changes, run from the repository root:

```sh
python3 scripts/docs/check_names.py
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
git diff --check
```

## Submit a pull request

1. Keep the change focused on one problem.
2. Add or update tests when behavior changes, and run the relevant checks.
3. Update documentation for changed setup, configuration, or public behavior.
4. Fill in the [pull request template](.github/PULL_REQUEST_TEMPLATE.md): the
   problem, resulting behavior, validation performed, and affected docs. Include
   screenshots for UI changes, using synthetic data.
5. If the change touches AGPL or enterprise code, post the CLA acceptance
   statement below from your own account.

## Licensing and the CLA

The Community application is AGPL-3.0-only; enterprise directories use the
Helpin Enterprise License; the public SDK and widget packages listed in
[LICENSE](LICENSE) use Apache-2.0. Preserve existing third-party licenses and
notices, and identify any third-party material you add.

Each contributor to AGPL or enterprise code must accept the
[Helpin Contributor License Agreement](CLA.md) by posting this statement in the
pull request:

> I have read and agree to the Helpin Contributor License Agreement v1.0
> in CLA.md for my contributions in this pull request, and I have the
> authority to grant the rights described in it.

Contributors retain ownership; the agreement lets Helpin include contributions
in both community and commercial editions. Maintainers verify acceptance before
merging. Contributions solely to Apache-2.0 components use Apache-2.0's
contribution terms and do not require the additional CLA.

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

Product tutorials will live in the planned hosted help center. Once it exists,
link to those articles rather than duplicating them in the repository. Keep
version-sensitive setup, architecture, and API contracts alongside code. The
[PR template](.github/PULL_REQUEST_TEMPLATE.md) asks for affected documentation.

Add new maintained technical guides to
[scripts/docs/maintained-docs.json](scripts/docs/maintained-docs.json) and read
the [checker scope](scripts/docs/README.md) when adding public documentation.

## For maintainers

The [publication checklist](community/PUBLICATION.md) governs publishing this
repository, its history, and release bundles. The
[publication review](docs/publication-review.md) records documents that need a
disposition before public export. Passing link checks does not authorize
publication.
