# Write and name Helpin documentation

Use this guide when adding, updating, or organizing repository documentation.
Readers should be able to choose the right page from its name, understand its
purpose in the first paragraph, and follow its instructions without knowing the
conversation that led to the change.

## Choose a location

| Content | Location | Example |
| --- | --- | --- |
| Current architecture or integration guide | `docs/` | `agents-and-automation.md` |
| Community installation and operations | `docs/community/` | `troubleshooting.md` |
| Environment-specific operating procedure | `docs/ops/` or `docs/deployments/` | `native-release-runbook.md` |
| Product requirements | `docs/prds/` | `support-live-chat.md` |
| Implementation plan | `docs/plans/` | `2026-09-14-ai-settings-ux-plan.md` |
| Design specification | `docs/specs/` | `2026-08-18-support-link-security-design.md` |
| Research or assessment | `docs/research/` | `editor-list-exit-behavior.md` |
| Product tutorials | [Helpin-hosted docs](https://helpin.ai/docs) | First support conversation |

The hosted help center is the destination for product tutorials. Repository
technical guides remain versioned with the code. Link between the two rather
than creating duplicate authoritative copies. Internal strategy and operational
material still need [publication review](publication-review.md).

## Name files predictably

Use **lowercase words separated by hyphens**, with a `.md` extension.
Name the subject and, when helpful, the task: `agent-runtime-local-setup.md`,
`public-mcp-server.md`, or `widget-architecture.md`.

- Keep conventional discovery filenames such as `README.md`, `ARCHITECTURE.md`,
  `CONTRIBUTING.md`, `SUPPORT.md`, `SECURITY.md`, `AGENTS.md`, `CLAUDE.md`, and
  `SKILL.md`. Existing legal/community filenames and GitHub templates also keep
  their standard names. Within `docs/`, only directory indexes use `README.md`.
- Do not add `PRD-` inside `docs/prds/`; the directory already communicates the
  document type. Put `TODO`, `draft`, or `superseded` in a status note, not the filename.
- For a dated historical plan or assessment, use `YYYY-MM-DD-topic.md`. Preserve
  its original date. Do not invent a date for an undated document or change the
  date on every edit. Current guides generally do not need dates in their names.
- Avoid names such as `misc.md`, `new-doc.md`, `final-v2.md`, and all-caps filenames.
  Use searchable product nouns and distinguish similarly named pages by purpose.
- Keep an existing clear name stable. Renaming a file changes links; do it when
  clarity or consistency warrants the cost.

Examples of the convention in this repository:

| Page | Path |
| --- | --- |
| Set up Agent Runtime locally | [agent-runtime-local-setup.md](agent-runtime-local-setup.md) |
| CRM architecture and data model | [crm-overview.md](crm-overview.md) |
| Support live chat requirements | [prds/support-live-chat.md](prds/support-live-chat.md) |

## Make each page easy to explain

Start with one descriptive H1 in sentence case. Keep product names and acronyms
correctly capitalized. Navigation labels should describe the page, not display a
raw filename such as `crm-overview.md`.

The opening paragraph should say **who the page is for, what it explains, and
when to use it**. Explain unfamiliar terms at first use. State whether the page
describes current behavior, a proposal, or historical work; separate code status
from release availability. Preserve explicit limitations instead of making a
proposal sound shipped.

Choose the structure that matches the reader's task:

- **How-to guide:** purpose, prerequisites, numbered steps, expected result,
  troubleshooting, and relevant next steps.
- **Architecture explanation:** purpose, components and responsibilities, one
  end-to-end flow, boundaries, and code entry points. Add a diagram if it helps.
- **Reference:** scope, exact fields/options/contracts, examples, and limitations.
- **Plan or specification:** problem, proposed outcome, decisions, scope, and
  verification. Put date/status near the opening; do not present it as setup advice.

Use direct sentences and concrete actions. Explain what a command accomplishes,
which directory to run it from, and how to recognize success. Link to deeper
contracts rather than repeating them. Keep lists flat when possible. Use tables
for comparisons and mappings, and meaningful link text such as “AI configuration.”

A useful introduction is: “This guide explains how to run Helpin and Agent Runtime
locally so contributors can test agent execution. Start with Community Compose
for a configured pair; use the host-process section when debugging a service.”

## Rename without breaking navigation

Search for the old filename across the repository, including indexes, agent
instructions, source comments, CI manifests, and release packaging. Move the file
and update incoming links together. Preserve linked heading anchors or update
callers when changing titles. Check that the destination does not already exist.

Do not edit unrelated uncommitted documents to complete a bulk rename. If a guide
is publicly hosted, update its published links or redirects through the normal
publishing process; local file checks cannot verify external bookmarks.

## Validate changes

From the repository root:

```sh
python3 scripts/docs/check_names.py
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
git diff --check
```

The naming check covers tracked Markdown under `docs/` and new non-ignored docs.
The link check covers the maintained guides listed in
[maintained-docs.json](../scripts/docs/maintained-docs.json). Add maintained guides
to that list; check other edited pages explicitly when needed. Historical links
are not automatically made valid by renaming a file. For operator bundle changes,
also run the [packaging check](../scripts/docs/README.md).

The repository skill [helpin-documentation](../.agents/skills/helpin-documentation/SKILL.md)
uses this guide when writing or reorganizing documentation.
