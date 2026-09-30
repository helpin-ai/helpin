# Getting help with Helpin

This page explains where to find answers, how to ask a question, and what to
expect from Community support. Technical guides are versioned in this
repository. A hosted product help center is planned and will be linked here
when it is live.

## Find an answer first

| Task | Guide |
| --- | --- |
| Install Community | [Installation guide](community/README.md) |
| Fix an installation problem | [Troubleshooting](docs/community/troubleshooting.md) |
| Configure mail, AI, embeddings, or storage | [Configuration](docs/community/configuration.md) |
| Expose the installation publicly | [Deployment](docs/community/deployment.md) |
| Back up or restore | [Backups](docs/community/backups.md) |
| Understand what the beta covers | [Scope and known limitations](ROADMAP.md) |
| Set up a development environment | [Local development](docs/development.md) and [architecture](ARCHITECTURE.md) |

## Ask a question or report a problem

Search [existing issues](https://github.com/helpin-ai/helpin/issues) before
opening a new one.

- **Setup question:** open an issue with a descriptive title and say which
  documented step failed.
- **Reproducible failure:** use the
  [bug report form](https://github.com/helpin-ai/helpin/issues/new?template=community-bug.yml).
- **Improvement:** use the
  [feature request form](https://github.com/helpin-ai/helpin/issues/new?template=community-feature.yml).
- **Documentation error:** open an issue with the page path, affected version,
  and the step that needs correction.

Include your Helpin release or commit, edition (Community or Enterprise), OS and
CPU architecture, installation method, expected and observed behavior, and
minimal reproduction steps. Include only relevant, redacted log excerpts. Remove
credentials, session tokens, signed URLs, personal information, and customer
content. Never attach `.env`, database dumps, or raw support archives.

## What to expect

Community support is best effort. No response deadline or service-level
agreement is promised. Community 0.2 is a beta: only the latest published 0.2
patch is supported, as described in the [security policy](SECURITY.md).

## Security vulnerabilities

Follow [SECURITY.md](SECURITY.md) to report privately. Do not post exploit
details or sensitive data in public issues.

## Contribute a fix

Read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting changes. Documentation
corrections are welcome as pull requests too.
