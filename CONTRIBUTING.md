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
