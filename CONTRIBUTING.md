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
[security reporting](SECURITY.md). License and contribution terms must be
confirmed in the publication checklist before accepting public contributions.
