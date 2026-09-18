# Documentation publication review

Reviewed September 17, 2026 against the tracked documentation inventory. This is
a publication triage, not a secret scan, legal clearance, or authorization to make
the repository public. The follow-up cleanup below removes obsolete documents
from the working tree; Git history has not been rewritten.

## Documentation destinations

Product tutorials belong in the hosted help center, which is planned; until it
is live, repository guides are the reference.
Keep architecture, contribution guidance, and version-sensitive technical
instructions in the repository. Link to the authoritative page rather than
maintaining a second copy in the help center.

The engineering index is not a public help-center import list. A public repository
exposes tracked files whether or not they are linked from the README; moving a
file to an archive directory does not make it private or remove it from history.

## Inventory and scope

The starting tracked inventory contained 366 Markdown files under `docs/`:
34 at its root, 7 Community guides, 187 plans, 55 specifications, 40 PRDs,
9 archive documents, 10 research documents, 7 strategy documents, 5 operations
files, and 12 across customer lifecycle, notifications, RBAC, mockups, and testing.
These counts precede the troubleshooting guide and this review.

Directory indexes and representative operational, strategy, integration, and
release documents were inspected. The table below classifies publication work;
it does not assert a line-by-line review of all 366 files. Untracked work was
excluded from the inventory.

## Findings and recommended disposition

| Documents | Finding | Before public export |
| --- | --- | --- |
| Root README, architecture, contribution/support/security/license documents | Public-facing entry points and policies | Retain; verify public URLs and policy ownership |
| `docs/community/` operator guides | Installation, configuration, deployment, identity, backups, troubleshooting | Retain with release-specific validation; preserve bundle links |
| [Community implementation status](community/implementation-status.md) | Historical branch and acceptance evidence; excluded from operator bundles | Keep out of public help-center imports and operator bundles; review repository inclusion separately |
| [Local development](development.md), [local Runtime](agent-runtime-local-setup.md), [widget architecture](widget-architecture.md) | Contributor setup and build contracts | Retain; verify against a clean checkout and published dependencies |
| Staging Runtime runbook (moved to the private archive on September 18, 2026) | Deployment topology, environment-specific secret-store configuration, and shared-service operational assumptions | Move to a private operational knowledge base or replace with a sanitized generic guide before export |
| Native release runbook (moved to the private archive on September 18, 2026) | Coordinated SaaS deployment, release inventory, and canary instructions | Private operational material; retain only generic contributor instructions publicly |
| [Support email operations](ops/support-email-fallback-runbook.md) | Deployment-specific operational procedures | Review and sanitize; separate public configuration from internal response procedures |
| [Pricing strategy](strategy/pricing-strategy.md), [backlog](strategy/backlog-and-ideas.md), and other `docs/strategy/` pages | Pricing options, competitive positioning, campaign planning, and uncommitted product direction | Default to private strategy material; publish only an explicitly approved roadmap |
| [Customer lifecycle campaigns](customer-io/README.md) | SaaS lifecycle campaigns and event/data contracts | Review for private business policy and user-data fields; publish only integration contracts intentionally supported for contributors |
| Mattermost design (moved to the private archive on September 18, 2026) | Assumes a shared company deployment rather than arbitrary user installations | Rewrite as a generic integration guide if supported; otherwise retain privately as design history |
| [Billing audit](plans/2026-06-22-billing-audit-findings.md), billing plans/specifications, [AI profiles](ai-connections.md) | Mix current contracts with commercial policy and dated rollout/test evidence | Keep necessary Community configuration; review commercial material and historical evidence before public inclusion |
| `docs/plans/`, `docs/specs/`, `docs/prds/`, `docs/research/` | Large body of dated design intent and assessments, not release documentation | Review individually; extract useful current contracts and keep unapproved history out of the public export |
| `docs/mockups/`, `docs/testing/`, and non-Markdown assets | Not covered by the Markdown inventory/content sampling | Inspect screenshots, fixtures, HTML, and embedded data separately for personal/customer information |

## Maintainer follow-through

1. Assign owners to the disposition decisions above. Confirm which historical and
   commercial documents belong in the public repository.
2. Preserve material selected for private retention before removing it from any
   public export. This review does not perform that migration.
3. Update engineering indexes and incoming links after an approved move or rewrite.
4. Complete the full-history secret and personal-data review, third-party review,
   and history/export decision in [publication gates](../community/PUBLICATION.md).
5. Verify the hosted docs and repository support links anonymously before launch.
   No hosted help center exists yet; navigation says one is planned and links
   repository guides. Remove that wording after the hosted destination is
   published and verified.

Link checks establish navigation integrity, not confidentiality or publication
readiness. The document-inventory publication gate remains open until maintainers
resolve the dispositions and validate the final export.

## Obsolete-document cleanup

A follow-up review removed 15 tracked documents (including the empty archive
index), all unchanged from HEAD before removal. Their previous contents remain
in Git history. Incoming index links and retained-plan references were updated.

| Removed group | Why it was removed | Current reference |
| --- | --- | --- |
| Eight original PM phase plans and the archive index | Initial story/iteration-era implementation checklists still marked “Not Started”; the application has since moved to task-based APIs, routes, and modules | [Architecture](../ARCHITECTURE.md), [development](development.md), and [migration guide](ops/database-migrations.md) |
| Four March 23–27 in-process native/Codex/OpenCode runtime rollout plans | They target retired Helpin worker executors and local machine paths; execution now belongs to the separate Runtime service | [Coding execution](coding-agent-execution.md) and [agent architecture](agents-and-automation.md) |
| Historical product-spec planning-pipeline PRD stub | It explicitly says its orchestration no longer matches the active runtime and only redirects readers | [Agents and automation](agents-and-automation.md) |
| September 14 AI-profile implementation checkpoint | Explicitly superseded completion snapshot with old branch/version and session-specific instructions | [AI connections](ai-connections.md) and [review corrections](plans/2026-09-14-ai-profiles-review-fixes.md) |

Partially superseded inbox-performance and custom-sender requirements were kept
because they explicitly retain requirements outside their replacements' scope.
Publication-sensitive strategy and operational docs
still require the disposition review above; age or privacy concerns alone were
not treated as proof that a document is obsolete.
