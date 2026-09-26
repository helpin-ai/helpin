# Helpin Website Copy Skill

A reusable writing and review skill for Helpin's website. It captures the copy direction from the homepage and product-page series while keeping release-specific facts out of permanent instructions.

**Core direction:** Customer history explains the next action. Product depth makes that action possible. The team controls what happens.

## Package contents

- `SKILL.md`: The main workflow, writing rules, preservation contract, and delivery format.
- `references/positioning-and-voice.md`: Brand direction, vocabulary, headline discipline, and before/after examples.
- `references/page-playbooks.md`: Distinct priorities for ten page types and a method for future pages.
- `references/demo-scenarios.md`: Fictional examples and rules for truthful workflow states.
- `references/claim-verification.md`: Evidence, availability, permission, and publication rules.
- `references/review-checklist.md`: Editorial, factual, structural, and implementation checks.
- `assets/`: Templates for a page brief, product-truth register, page-progress record, review report, and repository instructions.
- `tests/acceptance-cases.yaml`: Fourteen evaluation cases covering expected behavior and common regressions.

Install the complete folder, not only SKILL.md: the main instructions reference the other files.

## Installation

Choose the repository-scoped location for the coding agent you use:

| Agent | Place the complete folder here |
|---|---|
| Claude Code | `.claude/skills/helpin-website-copy/` |
| Codex | `.agents/skills/helpin-website-copy/` |

These paths follow the official documentation checked on September 21, 2026. Host capabilities and discovery behavior can change; consult the cited documentation when using another version.

Keep one canonical copy. Do not maintain two independent copies that can drift; the documented hosts support symlinked skill folders when that fits your repository policy. An agent without native skill discovery can still be told to read the complete skill folder explicitly.

No files have been installed in your repository by creating this package. If a directory already exists at the destination, review and merge it rather than blindly replacing it.

Merge the relevant snippet from `assets/AGENTS-snippet.md` into your existing repository instruction file. Do not overwrite existing project instructions.

## One-time setup

The writing rules can be used immediately. Before treating generated product claims as publication-ready, give the agent current product evidence.

Use existing repository records where available. Otherwise copy the product-truth and site-copy-state templates into a maintained repository location, such as `docs/website-copy/`, and point the agent to that location. Record actual source paths, route files, release scope, and commercial owners.

The product-truth template intentionally contains no current prices, trial duration, license assertions, or feature-availability promises. Its entries are all unverified. The page-state template records that drafts were discussed, not that they were approved or implemented.

### Initialization prompt

```text
Use the helpin-website-copy skill to initialize our website-copy context.
Inspect the current repository, routes, release documentation, and existing
product/billing sources. Reuse existing records where possible; otherwise
populate the product-truth and site-copy-state templates in docs/website-copy/.
Record evidence and scope for the claims you verify. Leave unknowns explicit.
Do not change website copy, prices, product behavior, or publish anything.
Report the material gaps that need an owner decision.
```

## Day-to-day use

In Claude Code, explicitly invoke `/helpin-website-copy`. In Codex, select the skill or mention `$helpin-website-copy`. Natural-language matching may also activate it, depending on the host; an explicit instruction removes ambiguity.

### Draft one page

```text
Use the helpin-website-copy skill to rewrite the Helpin [page name] page.
Read the current page source and our product-truth records first. Keep all
existing sections, cards, tabs, FAQs, and demo states. Give me one recommended
version of the full copy, with verification notes kept separate. Do not edit
source files or change commercial terms.
```

### Implement approved copy

```text
Use the helpin-website-copy skill to implement the approved copy for [route].
Preserve the current layout, component structure, anchors, animations, and
functional behavior. Make a copy-focused diff, check the changed states on
desktop and mobile when tools are available, and report checks actually run.
Flag unsupported claims separately. Do not deploy or change shared global
copy unless it is included in this request.
```

### Continue the sequence

```text
Use the helpin-website-copy skill for the next page in our site-copy-state
record. Confirm the current route and preserve its existing structure.
Draft the full copy and update only the draft progress status.
```

## Keep these records current

**Product truth:** Update verified capabilities, plan restrictions, prices, usage rules, availability, and release support when they change. A rewrite does not refresh evidence by itself.

**Editorial decisions:** Record explicitly approved wording and user corrections. Do not treat all earlier assistant suggestions as approved examples.

**Page progress:** Track drafted, in-review, approved, and implemented separately. Keep the actual route inventory rather than guessing what “next page” means.

**Review evidence:** Record what was read, tested, rendered, and checked. A successful build is not a visual review or confirmation of a product promise.

## Evaluation and limits

Run the acceptance cases in your actual coding-agent environment before relying on automatic invocation or enforcement. The cases are definitions, not a claim of completed agent evaluations. Review outputs and diffs against their assertions; use failures to revise the relevant instruction.

This package supplies instructions and templates. It does not implement automated UI testing, crawl the site, synchronize billing data, or guarantee factual correctness or conversion performance. The agent needs access to appropriate sources and tools, and your team retains publication approval.

## Format and installation sources

Packaging was checked against these primary sources on September 21, 2026:

1. Agent Skills specification: https://agentskills.io/specification
2. Skill-creation practices: https://agentskills.io/skill-creation/best-practices
3. Claude Code skills documentation: https://code.claude.com/docs/en/skills
4. Codex skill documentation: https://developers.openai.com/codex/skills

The Helpin positioning comes from the user's copy project. No live Helpin product, price, deployment, or license claim was reverified as part of packaging this skill.
