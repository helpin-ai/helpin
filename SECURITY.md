# Security policy

This policy covers the Helpin repository and its published Community bundles.
Report vulnerabilities privately, never in public issues, pull requests, or
discussions.

## Report a vulnerability

Use GitHub's private reporting flow for this repository
(**Security → Report a vulnerability**):

https://github.com/helpin-ai/helpin/security/advisories/new

Do not include customer data, tokens, or passwords in the report. Include:

- the affected release or commit and the deployment shape (Community bundle,
  source build, or Enterprise);
- reproduction steps using synthetic data;
- the expected versus observed security boundary, such as workspace, origin,
  visitor session, or identity isolation.

Do not access other people's data to demonstrate impact.

Vulnerabilities in the upstream images the bundle pulls (PostgreSQL, Redis,
NATS, Temporal, Garage) belong with those projects. Report to us if a published
bundle pins a vulnerable image.

## What happens next

Maintainers validate the report, prepare a fix, and coordinate disclosure
through the private advisory. Please allow time for a fix and a published patch
before public disclosure. We do not promise a response deadline or a bounty.

## Supported versions

| Version | Supported |
| --- | --- |
| Latest published 0.2 patch | Yes |
| Earlier 0.2 patches and unpublished builds | No |

Community 0.2 is a beta. Subscribe to release notices, apply patches promptly,
and keep backups of the databases, object store, and encryption keys.

## Operator responsibilities

The Community bundle binds to loopback by default. Before exposing an
installation publicly, follow the [deployment guide](docs/community/deployment.md),
keep Postgres, Redis, NATS, Temporal, Runtime, the storage console, and
`/api/internal/` routes private, and configure
[backups](docs/community/backups.md). The [known limitations](ROADMAP.md)
describe retained data and AI trust boundaries; they do not waive the visitor
isolation requirements.
