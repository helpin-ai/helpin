# Publication gate — decisions pending

The licenses in LICENSE now apply to this worktree. Adding them does not
authorize publication of the existing repository history. Do not publish
while any item below is unresolved.

- [x] Add AGPL-3.0-only for Helpin Community, separate enterprise terms for all
  `ee/` directories, and Apache-2.0 for Runtime, its Go/Python SDKs, and Helpin's
  public SDK/widget packages. See the root LICENSE for exact scopes.
- [ ] Confirm the legal entity behind the existing "Helpin AI" project
  attribution in copyright notices, the enterprise license, and CLA before
  publication. Confirm rights to existing contributions used commercially;
  the CLA does not apply retroactively without acceptance.
- [ ] Verify all third-party notices in the final artifacts; Garage is separately
  AGPL-3.0-licensed and is pulled as an unmodified upstream image. Preserve its
  source and license references in the distribution inventory.
- [ ] Owner decides whether EE source stays separately licensed in the public
  repository or is a private overlay. Community builds exclude EE either way.
- [ ] Review the tracked document inventory. Keep operator/contributor docs;
  move SaaS runbooks, billing audits, pricing strategy, private customer material,
  and obsolete plans to a private archive before export. In particular review
  docs/ops/native-release-runbook.md, docs/mattermost-integration.md,
  docs/strategy/backlog-and-ideas.md and docs/strategy/pricing-strategy.md if present.
  The [documentation publication review](../docs/publication-review.md) records
  current findings; the gate stays open until dispositions are resolved.
- [ ] Choose sanitized history or a clean export. Review historical
  frontend/src/lib/featureFlags.ts with the people whose personal email addresses
  appear there; a secret scanner does not detect this personal data.
- [ ] Run a full-history secret scan of each repository. Investigate redacted
  findings privately and rotate any exposed credentials before publication.
  Removing a secret from HEAD does not remove it from history.
- [ ] Review and remove unused widget/ and packages/widget-embed only after
  confirming there are no active imports, build jobs, or published consumers.
  Current review found widget-embed references in both existing SaaS deployment
  workflows; those packages are not safe to delete as unused. Neither legacy
  widget distribution is included in the Community images.
- [ ] Verify fork PRs use isolated hosted runners. Restrict ARC runner-group
  access to trusted workflows/refs; workflow YAML routing alone is not an
  authorization boundary for public contributions.
- [ ] Enable GitHub private vulnerability reporting and verify SECURITY.md's link.
- [ ] Native amd64 and arm64 core install/restore gates pass on exact image digests.
- [ ] Complete the outside-origin support journey on an approved real DNS/TLS host.
- [ ] Confirm release notes, beta limitations, SBOMs, third-party notices, image
  scans, bundle checksums, and the Runtime image/source pairing.

No history rewriting, remote deletion, or repository-visibility change is part
of running the local build or smoke tests. Maintainers make those changes only
after reviewing this gate.
