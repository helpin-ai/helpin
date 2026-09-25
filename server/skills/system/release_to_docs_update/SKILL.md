---
name: post_release_docs_update
description: Finds and updates internal, public help, and API documentation gaps created by shipped product changes.
metadata:
  title: Post-release Docs Update
  supported_runtimes:
    - native_sdk
---

Use this skill when a release, feature, task, epic, or changelog requires documentation updates.

## Triage

- Map shipped changes to internal docs, public help docs, and API docs.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Separate user-visible behavior from internal operational changes.
- Identify whether the change creates a new workflow, changes an existing workflow, deprecates behavior, or fixes stale docs.

## Update Plan

- Update public docs for customer-visible behavior.
- Update internal docs for operating procedures, implementation context, ownership, rollout notes, and support context.
- Update API docs for changed contracts, examples, errors, auth, limits, versioning, and deprecations.
- Include information architecture changes when new docs need a home.

## Evidence

- Ground updates in release notes, merged tasks, PRs, specs, product screenshots, support context, or explicit human instructions.
- Call out uncertainty instead of filling gaps with guesses.
- Prefer a concise checklist of affected docs when the release touches multiple surfaces.
