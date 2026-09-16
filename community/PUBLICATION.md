# Publication gate — decisions pending

This worktree is an implementation candidate, not a license grant or permission
to expose the existing repository history. Do not publish while any item below
is unresolved.

- [ ] Owner chooses and adds the Helpin Community and Runtime licenses and confirms
  Go/Python SDK licensing. Preserve all third-party notices; MinIO is separately
  AGPL-3.0-only and the bundled source must remain available.
- [ ] Owner decides whether EE source stays separately licensed in the public
  repository or is a private overlay. Community builds exclude EE either way.
- [ ] Review the tracked document inventory. Keep operator/contributor docs;
  move SaaS runbooks, billing audits, pricing strategy, private customer material,
  and obsolete plans to a private archive before export. In particular review
  docs/2026-09-15-native-release-runbook.md, docs/mattermost-integration.md,
  docs/BACKLOG-and-ideas.md and docs/pricing-strategy.md if present.
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
- [ ] Enable GitHub private vulnerability reporting and verify SECURITY.md's link.
- [ ] Native amd64 and arm64 core install/restore gates pass on exact image digests.
- [ ] Complete the outside-origin support journey on an approved real DNS/TLS host.
- [ ] Confirm release notes, beta limitations, SBOMs, third-party notices, image
  scans, bundle checksums, and the Runtime image/source pairing.

No history rewriting, remote deletion, or repository-visibility change is part
of running the local build or smoke tests. Maintainers make those changes only
after reviewing this gate.
