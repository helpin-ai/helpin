# Security reporting

Do not report exploitable vulnerabilities, customer data, tokens, or passwords
in public issues. Use this repository's **Security → Report a vulnerability**
private reporting flow:

https://github.com/helpin-ai/helpin/security/advisories/new

The release publication checklist requires maintainers to enable and verify
that private reporting flow before making the repository public. Until that
check passes, this source is not a public Community release.

Include the affected release, deployment shape, reproduction steps with synthetic
data, and the expected versus observed boundary. Avoid accessing other people's
data to prove impact. Maintainers will coordinate validation, a fix, and disclosure
through the private report. We do not promise a response deadline or bounty.

Community 0.1 is beta. Only the latest published 0.1 patch is supported. Subscribe
to release notices and keep backups of the databases, object store, and encryption
keys. See [the roadmap](ROADMAP.md) for known product limitations; these do not
waive the visitor isolation requirements.
